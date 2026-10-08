package claim

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ehrlich-b/ngn/engine"
)

// Fixtures are synthetic NGN-written records, with no engine/oracle subprocess.
func fixtureSession(name string) SessionRecord {
	s := SessionRecord{Engine: Artifact{Path: name, SHA256: strings.Repeat("a", 64)}, PID: 1, ExitCode: 0, Options: map[string]string{}, Selected: map[string]string{}}
	for _, e := range []Event{{0, "stdin", "uci"}, {1, "stdout", "id name " + name}, {2, "stdout", "uciok"}, {3, "stdin", "isready"}, {4, "stdout", "readyok"}, {5, "stdin", "ucinewgame"}, {6, "stdin", "isready"}, {7, "stdout", "readyok"}} {
		s.Events = append(s.Events, e)
	}
	return s
}
func fixtureGame(t *testing.T, opening, played []string) (Game, Receipt, Opening) {
	t.Helper()
	o := Opening{Index: 0, FEN: StartFEN, History: append([]string{}, opening...), Line: strings.Join(opening, " ")}
	r := Receipt{AttemptID: strings.Repeat("c", 32), CreatedUTC: "2026-10-06T00:00:00Z", Plan: Plan{BaseNS: 120e9, IncrementNS: 1e9, Cap: 400}, Probes: map[string]SessionRecord{}, Selected: map[string]map[string]string{}}
	g := Game{AttemptID: r.AttemptID, StartedUTC: "2026-10-06T00:00:01Z", EndedUTC: "2026-10-06T00:00:02Z", Schema: Schema, ID: "fixture-00-white", PairID: "fixture-00", Anchor: "fixture", CandidateColor: "white", Opening: o, BaseNS: r.Plan.BaseNS, IncrementNS: r.Plan.IncrementNS, Cap: 400, Candidate: fixtureSession("candidate"), Opponent: fixtureSession("fixture"), Plies: []Ply{}}
	r.Probes["candidate"] = g.Candidate
	r.Probes["fixture"] = g.Opponent
	r.Selected["candidate"] = g.Candidate.Selected
	r.Selected["fixture"] = g.Opponent.Selected
	p, e := engine.ParseFEN(StartFEN)
	if e != nil {
		t.Fatal(e)
	}
	seen := map[string]int{PositionKey(p): 1}
	half := 0
	history := append([]string{}, opening...)
	for _, m := range opening {
		if e = advanceAudit(p, m, seen, &half); e != nil {
			t.Fatal(e)
		}
	}
	clocks := [2]int64{r.Plan.BaseNS, r.Plan.BaseNS}
	for _, m := range played {
		stm := Color(p)
		role := "candidate"
		s := &g.Candidate
		side := 0
		if stm == "black" {
			role = "opponent"
			s = &g.Opponent
			side = 1
		}
		now := s.Events[len(s.Events)-1].AtNS + 1e6
		s.Events = append(s.Events, Event{now, "stdin", PositionCommand(StartFEN, history)})
		gi := len(s.Events)
		s.Events = append(s.Events, Event{now + 1, "stdin", GoCommand(clocks, r.Plan.IncrementNS)})
		bi := len(s.Events)
		used := int64(50e6)
		s.Events = append(s.Events, Event{now + 1 + used, "stdout", "bestmove " + m})
		after := clocks
		after[side] += r.Plan.IncrementNS - used
		g.Plies = append(g.Plies, Ply{m, stm, role, clocks, after, used, gi, bi})
		clocks = after
		if e = advanceAudit(p, m, seen, &half); e != nil {
			t.Fatal(e)
		}
		history = append(history, m)
	}
	for _, s := range []*SessionRecord{&g.Candidate, &g.Opponent} {
		s.Events = append(s.Events, Event{s.Events[len(s.Events)-1].AtNS + 1, "stdin", "quit"})
	}
	g.Result, g.Reason = AuditTerminal(p, seen, half, len(history), 400)
	return g, r, o
}
func TestAuditMislabelledMateAsDraw(t *testing.T) {
	opening := []string{"f2f3", "e7e5", "g2g4"}
	g, r, o := fixtureGame(t, opening, []string{"d8h4"})
	if g.Result != "0-1" || g.Reason != "checkmate" {
		t.Fatalf("fixture terminal %s/%s", g.Result, g.Reason)
	}
	if e := AuditGame(g, r, o); e != nil {
		t.Fatalf("valid mate rejected: %v", e)
	}
	g.Result = "1/2-1/2"
	g.Reason = "ply-cap"
	if e := AuditGame(g, r, o); e == nil || !strings.Contains(e.Error(), "terminal mismatch") {
		t.Fatalf("mislabelled mate admitted/error: %v", e)
	}
}
func TestAuditRepetitionUsesOpeningHistory(t *testing.T) {
	loop := []string{"g1f3", "g8f6", "f3g1", "f6g8"}
	g, r, o := fixtureGame(t, loop, loop)
	if g.Reason != "repetition" {
		t.Fatalf("reason=%s", g.Reason)
	}
	if e := AuditGame(g, r, o); e != nil {
		t.Fatal(e)
	}
	g.Reason = "ply-cap"
	if e := AuditGame(g, r, o); e == nil {
		t.Fatal("repetition relabel admitted")
	}
}
func TestAuditClock120Plus1AndTranscriptFailures(t *testing.T) {
	mutations := []struct {
		name   string
		change func(*Game)
	}{
		{"lost-increment", func(g *Game) { g.Plies[0].After[0] -= 1e9 }},
		{"wrong-stm", func(g *Game) { g.Plies[0].STM = "black" }},
		{"wrong-time-used", func(g *Game) { g.Plies[0].UsedNS++ }},
		{"missing-move", func(g *Game) { g.Plies = g.Plies[1:] }},
		{"missing-transcript", func(g *Game) { g.Candidate.Events = nil }},
		{"missing-bestmove", func(g *Game) { i := g.Plies[0].BestEvent; g.Candidate.Events[i].Line = "info depth 1" }},
		{"extra-bestmove", func(g *Game) {
			i := len(g.Candidate.Events) - 1
			g.Candidate.Events = append(g.Candidate.Events[:i], Event{g.Candidate.Events[i].AtNS, "stdout", "bestmove e2e4"}, g.Candidate.Events[i])
		}},
		{"extra-position", func(g *Game) {
			i := len(g.Candidate.Events) - 1
			g.Candidate.Events = append(g.Candidate.Events[:i], Event{g.Candidate.Events[i].AtNS, "stdin", PositionCommand(StartFEN, nil)}, g.Candidate.Events[i])
		}},
		{"missing-readyok", func(g *Game) { g.Candidate.Events[7].Line = "info string waiting" }},
		{"engine-error", func(g *Game) {
			i := len(g.Candidate.Events) - 1
			g.Candidate.Events = append(g.Candidate.Events[:i], Event{g.Candidate.Events[i].AtNS, "stderr", "panic: fixture"}, g.Candidate.Events[i])
		}},
		{"forfeit", func(g *Game) { g.Reason = "no-move"; g.Result = "0-1" }},
		{"watchdog", func(g *Game) { g.Error = "watchdog" }},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			g, r, o := fixtureGame(t, nil, []string{"f2f3", "e7e5", "g2g4", "d8h4"})
			if e := AuditGame(g, r, o); e != nil {
				t.Fatalf("positive fixture: %v", e)
			}
			if g.Plies[0].After[0] != 120950000000 {
				t.Fatalf("120+1 arithmetic=%d", g.Plies[0].After[0])
			}
			tc.change(&g)
			if e := AuditGame(g, r, o); e == nil {
				t.Fatal("corruption admitted")
			}
		})
	}
}
func TestRawClockLossRejected(t *testing.T) {
	g, r, o := fixtureGame(t, nil, []string{"f2f3", "e7e5", "g2g4", "d8h4"})
	m := &g.Plies[0]
	m.UsedNS = r.Plan.BaseNS
	g.Candidate.Events[m.BestEvent].AtNS = g.Candidate.Events[m.GoEvent].AtNS + m.UsedNS
	// Shift all later candidate events together to preserve timestamp ordering.
	for i := m.BestEvent + 1; i < len(g.Candidate.Events); i++ {
		g.Candidate.Events[i].AtNS += r.Plan.BaseNS
	}
	m.After[0] = r.Plan.IncrementNS
	if e := AuditGame(g, r, o); e == nil || !strings.Contains(e.Error(), "clock loss") {
		t.Fatalf("clock loss admitted/error: %v", e)
	}
}
func TestAuditSessionAdvertisedOptionsAndOrder(t *testing.T) {
	s := fixtureSession("candidate")
	s.Events = append(s.Events, Event{8, "stdin", "quit"})
	id := s
	if e := validateSession(s, id); e != nil {
		t.Fatal(e)
	}
	s.Options = map[string]string{"Threads": "spin default 1 min 1 max 1"}
	id.Options = s.Options
	if e := validateSession(s, id); e == nil {
		t.Fatal("forged advertised option admitted")
	}
}
func TestPositionCommandRetainsFullHistory(t *testing.T) {
	got := PositionCommand(StartFEN, []string{"g1f3", "g8f6", "f3g1", "f6g8"})
	want := fmt.Sprintf("position fen %s moves g1f3 g8f6 f3g1 f6g8", StartFEN)
	if got != want {
		t.Fatal(got)
	}
}
func TestAuditWindowsSchedulingReceipt(t *testing.T) {
	s := fixtureSession("counter.exe")
	s.Events = append(s.Events, Event{8, "stdin", "quit"})
	identity := s
	if e := validateSession(s, identity); e == nil {
		t.Fatal("Windows session without actual affinity receipt admitted")
	}
	s.Events = append(s.Events[:8], Event{8, "stderr", "NGN claimbridge affinity 0xffff priority 0x40 pid 42"}, Event{9, "stdin", "quit"})
	if e := validateSession(s, identity); e != nil {
		t.Fatalf("valid Windows affinity receipt: %v", e)
	}
	s.Events[8].Line = "NGN claimbridge affinity 0xa000 priority 0x40 pid 42"
	if e := validateSession(s, identity); e == nil {
		t.Fatal("wrong Windows affinity admitted")
	}
}

func TestAuditRejectsOwnRecoveredPanicAndUCIErrors(t *testing.T) {
	for _, line := range []string{"NGN ENGINE PANIC: search panic recovered", "PANIC RECOVERED: using fallback", "info string warning malformed position", "info string invalid move e2e5", "info string eval loading failed", "info string unknown option Threads", "ERROR loading network"} {
		t.Run(line, func(t *testing.T) {
			g, r, o := fixtureGame(t, nil, []string{"f2f3", "e7e5", "g2g4", "d8h4"})
			i := len(g.Candidate.Events) - 1
			at := g.Candidate.Events[i].AtNS
			quit := g.Candidate.Events[i]
			g.Candidate.Events = append(g.Candidate.Events[:i], Event{at, "stderr", line}, quit)
			if e := AuditGame(g, r, o); e == nil {
				t.Fatal("recovered crash/engine error admitted")
			}
		})
	}
	for _, line := range []string{"", "info string eval backend ngnn1 score_cp 0", "info string threads configured 1 effective 1", "NGN claimbridge affinity 0xffff priority 0x40 pid 42"} {
		if engineError(line) {
			t.Fatalf("successful startup/probe considered error: %q", line)
		}
	}
}

func TestAuditRejectsCrossAttemptAndPriorGameRecords(t *testing.T) {
	for _, change := range []func(*Game){
		func(g *Game) { g.AttemptID = strings.Repeat("d", 32) },
		func(g *Game) { g.AttemptID = "" },
		func(g *Game) { g.StartedUTC = "2026-10-05T23:59:59Z" },
		func(g *Game) { g.EndedUTC = "" },
		func(g *Game) { g.EndedUTC = "2026-10-06T00:00:00Z" },
	} {
		g, r, o := fixtureGame(t, nil, []string{"f2f3", "e7e5", "g2g4", "d8h4"})
		if e := AuditGame(g, r, o); e != nil {
			t.Fatal(e)
		}
		change(&g)
		if e := AuditGame(g, r, o); e == nil {
			t.Fatal("mixed/older/incomplete attempt record admitted")
		}
	}
}
