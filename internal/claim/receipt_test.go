package claim

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Synthetic receipt fixtures exercise admission without launching any engine,
// reading an opponent, or invoking git/Go. Real metadata checks remain preflight.
func receiptFixtureSession(a Artifact, id *Identity, candidate bool, format string) SessionRecord {
	options := map[string]string{"Hash": "spin default 128 min 1 max 1024", "Threads": "spin default 1 min 1 max 64", "Move Overhead": "spin default 100 min 0 max 5000"}
	if candidate {
		options["EvalFile"] = "string default <empty>"
		options["UseNNUE"] = "check default false"
		options["OwnBook"] = "check default false"
	}
	s := SessionRecord{Engine: a, PID: 123, ExitCode: 0, Options: options, Selected: FrozenOptions(options, id, 100), LaunchArgv: sessionLaunch(a), Scheduling: schedulingFor(a)}
	now := int64(1)
	add := func(stream, line string) { s.Events = append(s.Events, Event{now, stream, line}); now++ }
	add("stdin", "uci")
	add("stdout", "id name owned receipt fixture")
	add("stdout", "id author NGN")
	keys := []string{}
	for k := range options {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		add("stdout", "option name "+k+" type "+options[k])
	}
	add("stdout", "uciok")
	ordered := []string{}
	for _, k := range []string{"EvalFile", "UseNNUE"} {
		if _, ok := s.Selected[k]; ok {
			ordered = append(ordered, k)
		}
	}
	for _, k := range keys {
		if k != "EvalFile" && k != "UseNNUE" {
			ordered = append(ordered, k)
		}
	}
	for _, k := range ordered {
		add("stdin", "setoption name "+k+" value "+s.Selected[k])
		if k == "Threads" && candidate {
			add("stdout", "info string threads configured 1 effective 1")
		}
	}
	add("stdin", "isready")
	add("stdout", "readyok")
	if candidate {
		backend := "hce"
		if format != "HCE" {
			backend = strings.ToLower(format)
		}
		for _, fen := range []string{StartFEN, "4k3/8/8/8/8/8/4P3/4K3 w - - 0 1"} {
			add("stdin", PositionCommand(fen, nil))
			add("stdin", "eval")
			add("stdout", "info string eval backend "+backend+" score_cp 0")
			add("stdin", "isready")
			add("stdout", "readyok")
		}
	}
	add("stdin", "quit")
	return s
}
func receiptFixture(t *testing.T) Receipt {
	t.Helper()
	sha := strings.Repeat("a", 64)
	id := Identity{Engine: Artifact{Path: "/owned/fixture-ngn", SHA256: sha}, SourceRoot: "/owned/source", SourceCommit: strings.Repeat("b", 40), GoVersion: "go1.25.5", BuildArgv: []string{"/usr/local/go/bin/go", "build", "-p", "2", "-o", "/owned/fixture-ngn", "."}, Provenance: map[string]Artifact{}}
	for _, key := range []string{"manifest", "data", "training", "export", "parity"} {
		id.Provenance[key] = Artifact{Path: "/owned/" + key, SHA256: sha}
	}
	p, e := MakePlan("tooling-check", id, Artifact{Path: "/owned/claimtool", SHA256: sha}, "/owned/openings.txt")
	if e != nil {
		t.Fatal(e)
	}
	host := Host{Hostname: "fixture", OS: "linux", Kernel: "fixture kernel", CPUStatus: "0-15", Nice: "19", Load: "0.01 0.02 0.03 1/20 1", Scheduling: "fixture scheduling", AtUTC: "2026-10-06T00:00:00Z"}
	r := Receipt{AttemptID: strings.Repeat("c", 32), Schema: Schema, CreatedUTC: "2026-10-06T00:00:00Z", Plan: p, ToolSourceRoot: "/owned/source", ToolSourceCommit: strings.Repeat("b", 40), ToolGoVersion: "go1.25.5", ToolBuildArgv: []string{"/usr/local/go/bin/go", "build", "-p", "2", "-o", "/owned/claimtool", "./cmd/claimtool"}, SourceClean: true, CandidateSourceClean: true, NetFormat: "HCE", OpeningCount: 80, FrozenOpenings: Artifact{Path: "/owned/first-80-openings.txt", SHA256: sha}, Host: host, AfterClockHost: host, Probes: map[string]SessionRecord{}, Selected: map[string]map[string]string{}}
	r.Probes["candidate"] = receiptFixtureSession(id.Engine, &id, true, r.NetFormat)
	r.Selected["candidate"] = r.Probes["candidate"].Selected
	for _, a := range p.Anchors {
		r.Probes[a.Name] = receiptFixtureSession(a.Engine, nil, false, r.NetFormat)
		r.Selected[a.Name] = r.Probes[a.Name].Selected
	}
	for _, role := range []string{"A", "A-copy"} {
		// Clock probes use the candidate options, but omit static evaluator probes.
		s := receiptFixtureSession(id.Engine, &id, true, r.NetFormat)
		cut := 0
		for i, ev := range s.Events {
			if ev.Stream == "stdin" && strings.HasPrefix(ev.Line, "position ") {
				cut = i
				break
			}
		}
		s.Events = s.Events[:cut]
		now := s.Events[len(s.Events)-1].AtNS + 1
		s.Events = append(s.Events, Event{now, "stdin", PositionCommand(StartFEN, nil)}, Event{now + 1, "stdin", GoCommand([2]int64{p.BaseNS, p.BaseNS}, p.IncrementNS)}, Event{now + 1 + 50e6, "stdin", "stop"}, Event{now + 1 + 50e6 + 10, "stdout", "bestmove e2e4"}, Event{now + 1 + 50e6 + 11, "stdin", "quit"})
		used := int64(50e6 + 10)
		r.ClockCheck = append(r.ClockCheck, ClockProbe{Lane: 0, Role: role, UsedNS: used, BeforeNS: p.BaseNS, AfterNS: p.BaseNS - used + p.IncrementNS, Move: "e2e4", Session: s})
	}
	if e := ValidateReceipt(r); e != nil {
		t.Fatalf("valid receipt fixture: %v", e)
	}
	return r
}
func TestReceiptRejectsChangedClockCoverageAndIdentity(t *testing.T) {
	cases := []struct {
		name   string
		change func(*Receipt)
	}{
		{"missing-attempt", func(r *Receipt) { r.AttemptID = "" }},
		{"wrong-increment", func(r *Receipt) { r.ClockCheck[0].AfterNS -= r.Plan.IncrementNS }},
		{"missing-role", func(r *Receipt) { r.ClockCheck = r.ClockCheck[:1] }},
		{"duplicate-role", func(r *Receipt) { r.ClockCheck[1].Role = "A" }},
		{"wrong-lane", func(r *Receipt) { r.ClockCheck[0].Lane = 1 }},
		{"time-used-vs-transcript", func(r *Receipt) { r.ClockCheck[0].UsedNS++; r.ClockCheck[0].AfterNS-- }},
		{"clock-loss", func(r *Receipt) { r.ClockCheck[0].UsedNS = r.Plan.BaseNS; r.ClockCheck[0].AfterNS = r.Plan.IncrementNS }},
		{"zero-time", func(r *Receipt) {
			r.ClockCheck[0].UsedNS = 0
			r.ClockCheck[0].AfterNS = r.Plan.BaseNS + r.Plan.IncrementNS
		}},
		{"selected-options", func(r *Receipt) { r.ClockCheck[0].Session.Selected = map[string]string{"Threads": "2"} }},
		{"dirty-tool-source", func(r *Receipt) { r.SourceClean = false }},
		{"dirty-candidate-source", func(r *Receipt) { r.CandidateSourceClean = false }},
		{"changed-label", func(r *Receipt) { r.Plan.Anchors[0].Label = 3360 }},
		{"changed-count", func(r *Receipt) { r.Plan.Count++ }},
		{"changed-cap", func(r *Receipt) { r.Plan.Cap = 399 }},
		{"changed-base", func(r *Receipt) { r.Plan.BaseNS = 120e9 }},
		{"unresolved-provenance", func(r *Receipt) { r.Plan.Candidate.Provenance["export"] = Artifact{} }},
		{"wrong-engine-probe", func(r *Receipt) {
			s := r.Probes["candidate"]
			s.Engine.SHA256 = strings.Repeat("f", 64)
			r.Probes["candidate"] = s
		}},
		{"missing-ready", func(r *Receipt) {
			s := r.Probes["candidate"]
			for i := range s.Events {
				if s.Events[i].Line == "readyok" {
					s.Events[i].Line = "info string wait"
					break
				}
			}
			r.Probes["candidate"] = s
		}},
		{"wrong-evaluator", func(r *Receipt) {
			s := r.Probes["candidate"]
			for i := range s.Events {
				if strings.HasPrefix(s.Events[i].Line, "info string eval backend ") {
					s.Events[i].Line = "info string eval backend ngnn1 score_cp 0"
				}
			}
			r.Probes["candidate"] = s
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := receiptFixture(t)
			tc.change(&r)
			if e := ValidateReceipt(r); e == nil {
				t.Fatal("changed receipt admitted")
			}
		})
	}
}
func TestReceiptSealTamperAndExclusiveCreation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "receipt.json")
	r := receiptFixture(t)
	if e := WriteJSON(path, r); e != nil {
		t.Fatal(e)
	}
	seal, e := Bind(path)
	if e != nil {
		t.Fatal(e)
	}
	sealPath := filepath.Join(dir, "receipt-seal.json")
	if e = WriteJSON(sealPath, seal); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadReceipt(path); e != nil {
		t.Fatal(e)
	}
	if e = WriteJSON(path, r); e == nil {
		t.Fatal("immutable receipt overwritten")
	}
	if e = os.Chmod(path, 0600); e != nil {
		t.Fatal(e)
	}
	r.Plan.Cap = 399
	b, e := json.Marshal(r)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, b, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadReceipt(path); e == nil || !strings.Contains(e.Error(), "hash mismatch") {
		t.Fatalf("tamper missed/error: %v", e)
	}
}
func TestReceiptSealRequiresExactPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "receipt.json")
	if e := WriteJSON(path, receiptFixture(t)); e != nil {
		t.Fatal(e)
	}
	seal, e := Bind(path)
	if e != nil {
		t.Fatal(e)
	}
	seal.Path = filepath.Join(dir, "another.json")
	if e = WriteJSON(filepath.Join(dir, "receipt-seal.json"), seal); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadReceipt(path); e == nil || !strings.Contains(e.Error(), "seal path") {
		t.Fatalf("wrong path seal admitted/error: %v", e)
	}
}
func TestRecheckBindsEveryArtifactBeforeAndAfter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "artifact")
	if e := os.WriteFile(path, []byte("owned identity fixture\n"), 0600); e != nil {
		t.Fatal(e)
	}
	a, e := Bind(path)
	if e != nil {
		t.Fatal(e)
	}
	r := Receipt{Plan: Plan{Candidate: Identity{Engine: a, Net: a, Provenance: map[string]Artifact{"manifest": a}}, Runner: a, Openings: a, Anchors: []Anchor{{Engine: a, Dependencies: []Artifact{a}}}}, FrozenOpenings: a, ScreenReceipt: a, ScreenAnalysis: a}
	if e = Recheck(r); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, []byte("changed identity fixture\n"), 0600); e != nil {
		t.Fatal(e)
	}
	if e = Recheck(r); e == nil {
		t.Fatal("changed post-run artifact admitted")
	}
}

func TestMatchGuardExcludesOwnWrappersAndRejectsForeignJobs(t *testing.T) {
	ours := map[string]bool{"10": true, "11": true}
	if e := foreignMatch("10 timeout 120s claimtool prepare\n11 claimtool prepare\n", ours); e != nil {
		t.Fatal(e)
	}
	for _, job := range []string{"sprt", "cutechess-cli", "fastchess", "trainer.train", "datagen", "claimtool run", "timeout 120s claimtool prepare"} {
		if e := foreignMatch("12 "+job, ours); e == nil {
			t.Fatalf("foreign %s admitted", job)
		}
	}
}

func TestHostLoadRequiresFiniteNonnegativeBound(t *testing.T) {
	for _, load := range []string{"", "NaN", "+Inf", "-1", "16.01"} {
		if e := checkLoad(Host{Load: load}, 16); e == nil {
			t.Fatalf("load %q admitted", load)
		}
	}
	if e := checkLoad(Host{Load: "16.00 1 1"}, 16); e != nil {
		t.Fatal(e)
	}
}

func TestReceiptStaticProbeDistinguishesNNUEFormats(t *testing.T) {
	for _, format := range []string{"NGNN1", "NGNN2", "NGNN3"} {
		id := Identity{Net: Artifact{Path: "/owned/fixture.nnue"}}
		s := receiptFixtureSession(Artifact{Path: "/owned/ngn"}, &id, true, format)
		if e := validateProbe(s, true, format); e != nil {
			t.Fatalf("%s: %v", format, e)
		}
		for i := range s.Events {
			if strings.HasPrefix(s.Events[i].Line, "info string eval backend ") {
				s.Events[i].Line = "info string eval backend hce score_cp 0"
			}
		}
		if e := validateProbe(s, true, format); e == nil {
			t.Fatalf("%s silently admitted HCE", format)
		}
	}
}

func TestMaelstromUsesVerifiedExecutableDigest(t *testing.T) {
	p, e := MakePlan("screen", Identity{}, Artifact{}, "/owned/openings.txt")
	if e != nil {
		t.Fatal(e)
	}
	if p.Anchors[1].Engine.SHA256 != MaelstromSHA {
		t.Fatal("Maelstrom identity unresolved")
	}
	if e = validatePlan(p); e != nil {
		t.Fatal(e)
	}
	p.Anchors[1].Engine.SHA256 = strings.Repeat("f", 64)
	if e = validatePlan(p); e == nil {
		t.Fatal("substituted Maelstrom executable admitted")
	}
}

func TestRevisedPoolPinsNewAnchorAndFixedSamples(t *testing.T) {
	for _, mode := range []string{"screen", "confirmation"} {
		p, e := MakePlan(mode, Identity{}, Artifact{}, "/owned/openings.txt")
		if e != nil || len(p.Anchors) != 4 || p.Anchors[3].Label != 3557 || p.Anchors[3].Engine.SHA256 != ViridithasSHA {
			t.Fatalf("bad v2 pool: %+v %v", p, e)
		}
		if p.Concurrency != 8 || p.Cap != 400 || (mode == "screen" && p.Count != 40) || (mode == "confirmation" && p.Count != 80) {
			t.Fatal("changed sample")
		}
		p.Anchors[3].Engine.SHA256 = strings.Repeat("f", 64)
		if validatePlan(p) == nil {
			t.Fatal("substituted new anchor admitted")
		}
	}
}
