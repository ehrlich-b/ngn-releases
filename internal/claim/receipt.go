package claim

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ehrlich-b/ngn/engine"
)

type Host struct {
	Hostname, OS, Kernel, CPUStatus, Nice, Load, Scheduling string
	AtUTC                                                   string
}
type ClockProbe struct {
	Lane                      int
	Role                      string
	UsedNS, BeforeNS, AfterNS int64
	Move                      string
	Session                   SessionRecord
}
type Receipt struct {
	Schema, CreatedUTC                              string
	AttemptID                                       string
	Plan                                            Plan
	ToolSourceRoot, ToolSourceCommit, ToolGoVersion string
	ToolBuildArgv                                   []string
	SourceClean, CandidateSourceClean               bool
	NetFormat                                       string
	Hidden, Buckets, KingBuckets                    int
	FrozenOpenings                                  Artifact
	OpeningCount                                    int
	Probes                                          map[string]SessionRecord
	Selected                                        map[string]map[string]string
	ClockCheck                                      []ClockProbe
	Host, AfterClockHost                            Host
	ScreenReceipt, ScreenAnalysis                   Artifact
	Questions                                       []string
}
type RecheckReceipt struct {
	Schema, AtUTC string
	Receipt       Artifact
	Artifacts     []Artifact
	Host          Host
	Unchanged     bool
}

func command(name string, args ...string) (string, error) {
	b, e := exec.CommandContext(ProcessContext, name, args...).CombinedOutput()
	return strings.TrimSpace(string(b)), e
}
func cleanSource(root, commit string) error {
	if !filepath.IsAbs(root) || len(commit) != 40 {
		return fmt.Errorf("unresolved source identity")
	}
	got, e := command("git", "-C", root, "rev-parse", "HEAD")
	if e != nil || got != commit {
		return fmt.Errorf("source HEAD mismatch: %s", got)
	}
	status, e := command("git", "-C", root, "status", "--porcelain", "--untracked-files=all")
	if e != nil {
		return e
	}
	if status != "" {
		return fmt.Errorf("source tree is dirty: %s", status)
	}
	return nil
}
func buildIdentity(a Artifact, commit, version string, argv []string) error {
	if len(argv) < 7 || argv[0] != "/usr/local/go/bin/go" || argv[1] != "build" {
		return fmt.Errorf("unresolved build argv")
	}
	p2, out := false, false
	for i := 2; i < len(argv)-1; i++ {
		if argv[i] == "-p" && argv[i+1] == "2" {
			p2 = true
		}
		if argv[i] == "-o" && argv[i+1] == a.Path {
			out = true
		}
	}
	if !p2 || !out {
		return fmt.Errorf("build argv must bind -p 2 and exact output path")
	}
	info, e := buildinfo.ReadFile(a.Path)
	if e != nil {
		return e
	}
	if info.GoVersion != version || version != "go1.25.5" {
		return fmt.Errorf("Go version mismatch %s", info.GoVersion)
	}
	settings := map[string]string{}
	for _, s := range info.Settings {
		settings[s.Key] = s.Value
	}
	if settings["vcs.revision"] != commit || settings["vcs.modified"] != "false" || settings["GOAMD64"] != "v3" || settings["GOOS"] != "linux" || settings["GOARCH"] != "amd64" {
		return fmt.Errorf("binary source/build metadata mismatch: %v", settings)
	}
	if len(info.Deps) != 0 {
		return fmt.Errorf("claim binaries must use NGN and stdlib only")
	}
	return Check(a)
}
func HostNow() (Host, error) {
	h := Host{OS: runtime.GOOS, AtUTC: time.Now().UTC().Format(time.RFC3339Nano)}
	h.Hostname, _ = os.Hostname()
	h.Kernel, _ = command("uname", "-a")
	b, e := os.ReadFile("/proc/self/status")
	if e != nil {
		return h, e
	}
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, "Cpus_allowed_list:") {
			h.CPUStatus = strings.TrimSpace(strings.TrimPrefix(l, "Cpus_allowed_list:"))
		}
	}
	b, e = os.ReadFile("/proc/self/stat")
	if e != nil {
		return h, e
	}
	s := strings.Fields(string(b)[strings.LastIndex(string(b), ")")+1:])
	if len(s) < 17 {
		return h, fmt.Errorf("bad process stat")
	}
	h.Nice = s[16]
	b, e = os.ReadFile("/proc/loadavg")
	if e != nil {
		return h, e
	}
	h.Load = strings.TrimSpace(string(b))
	h.Scheduling = "taskset " + h.CPUStatus + " nice 19; engines GOMAXPROCS=1; Windows anchors via pinned Idle bridge"
	// The restricted mask admits editing/build/dry-run preparation only.
	// A live receipt requires the exclusive rig below.
	if h.OS != "linux" || (h.CPUStatus != "13,15" && h.CPUStatus != "0-15") || h.Nice != "19" {
		return h, fmt.Errorf("need Linux CPU 13,15 (preparation) or 0-15 (exclusive rig), nice 19, got %s/%s/%s", h.OS, h.CPUStatus, h.Nice)
	}
	return h, nil
}
func NoOtherMatch() error {
	b, e := exec.Command("pgrep", "-af", "sprt|cutechess|fastchess|trainer\\.train|datagen|claimtool (run|prepare)").Output()
	if e != nil {
		if x, ok := e.(*exec.ExitError); ok && x.ExitCode() == 1 {
			return nil
		}
		return e
	}
	// A timeout/taskset wrapper repeats our own command in its argv. Exclude
	// our process ancestry, while retaining every other matching process.
	ours := map[string]bool{}
	for pid := os.Getpid(); pid > 0; {
		ours[strconv.Itoa(pid)] = true
		stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if err != nil {
			break
		}
		fields := strings.Fields(string(stat)[strings.LastIndex(string(stat), ")")+1:])
		if len(fields) < 2 {
			break
		}
		parent, err := strconv.Atoi(fields[1])
		if err != nil || parent == pid {
			break
		}
		pid = parent
	}
	return foreignMatch(string(b), ours)
}

func foreignMatch(output string, ours map[string]bool) error {
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && !ours[fields[0]] {
			return fmt.Errorf("other match running: %s", line)
		}
	}
	return nil
}
func MakePlan(mode string, id Identity, runner Artifact, openingPath string) (Plan, error) {
	p := Plan{Schema: Schema, Mode: mode, Candidate: id, Anchors: Anchors(), Openings: Artifact{openingPath, OpeningSHA}, Concurrency: 8, Cap: 400, Runner: runner, MoveOverhead: 100, MaxLoad: 16}
	switch mode {
	case "screen":
		p.Count = 40
		p.BaseNS = 10e9
		p.IncrementNS = 1e8
	case "confirmation":
		p.Count = 80
		p.BaseNS = 120e9
		p.IncrementNS = 1e9
	case "identity-check":
		p.Count = 0
		p.BaseNS = 120e9
		p.IncrementNS = 1e9
	case "tooling-check":
		p.Count = 2
		p.Concurrency = 1
		p.BaseNS = 1e9
		p.IncrementNS = 2e7
		p.Anchors = p.Anchors[2:3]
	case "stability-screen", "stability-confirmation":
		p.Count = 2
		p.BaseNS = 10e9
		p.IncrementNS = 1e8
		if mode == "stability-confirmation" {
			p.BaseNS = 120e9
			p.IncrementNS = 1e9
		}
		p.Anchors = p.Anchors[3:4]
	default:
		return p, fmt.Errorf("unknown mode")
	}
	return p, nil
}
func validatePlan(p Plan) error {
	expected, e := MakePlan(p.Mode, p.Candidate, p.Runner, p.Openings.Path)
	if e != nil {
		return e
	}
	if p.Schema != Schema || p.Count != expected.Count || p.Concurrency != expected.Concurrency || p.Cap != 400 || p.BaseNS != expected.BaseNS || p.IncrementNS != expected.IncrementNS || p.MoveOverhead != 100 || p.MaxLoad != expected.MaxLoad || p.Openings.SHA256 != OpeningSHA {
		return fmt.Errorf("protocol settings mismatch")
	}
	if len(p.Anchors) != len(expected.Anchors) {
		return fmt.Errorf("anchor pool changed")
	}
	for i, a := range p.Anchors {
		want := expected.Anchors[i]
		if a.Name != want.Name || a.Label != want.Label || a.Engine.Path != want.Engine.Path || (want.Engine.SHA256 != "" && want.Engine.SHA256 != a.Engine.SHA256) {
			return fmt.Errorf("anchor changed: %s", a.Name)
		}
	}
	return nil
}
func allArtifacts(r Receipt) []Artifact {
	a := []Artifact{r.Plan.Candidate.Engine, r.Plan.Runner, r.Plan.Openings, r.FrozenOpenings}
	if r.Plan.Candidate.Net.Path != "" {
		a = append(a, r.Plan.Candidate.Net)
	}
	if r.Plan.WindowsBridge.Path != "" {
		a = append(a, r.Plan.WindowsBridge)
	}
	keys := []string{}
	for k := range r.Plan.Candidate.Provenance {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		a = append(a, r.Plan.Candidate.Provenance[k])
	}
	for _, anchor := range r.Plan.Anchors {
		a = append(a, anchor.Engine)
		a = append(a, anchor.Dependencies...)
	}
	if r.ScreenReceipt.Path != "" {
		a = append(a, r.ScreenReceipt, r.ScreenAnalysis)
	}
	return a
}
func Recheck(r Receipt) error {
	for _, a := range allArtifacts(r) {
		if e := Check(a); e != nil {
			return e
		}
	}
	return nil
}
func ReadReceipt(path string) (Receipt, error) {
	var seal Artifact
	var r Receipt
	if e := ReadJSON(filepath.Join(filepath.Dir(path), "receipt-seal.json"), &seal); e != nil {
		return r, e
	}
	abs, e := filepath.Abs(path)
	if e != nil {
		return r, e
	}
	if seal.Path != abs {
		return r, fmt.Errorf("receipt seal path mismatch")
	}
	if e = Check(seal); e != nil {
		return r, e
	}
	if e = ReadJSON(path, &r); e != nil {
		return r, e
	}
	WindowsBridge = r.Plan.WindowsBridge
	return r, ValidateReceipt(r)
}
func ValidateReceipt(r Receipt) error {
	if b, e := hex.DecodeString(r.AttemptID); e != nil || len(b) != 16 {
		return fmt.Errorf("missing/invalid pre-run attempt identity")
	}
	if _, e := time.Parse(time.RFC3339Nano, r.CreatedUTC); e != nil {
		return fmt.Errorf("invalid receipt timestamp")
	}
	if r.Schema != Schema || r.CreatedUTC == "" || r.OpeningCount != 80 || len(r.ToolSourceCommit) != 40 || r.ToolGoVersion != "go1.25.5" || !r.SourceClean || !r.CandidateSourceClean {
		return fmt.Errorf("incomplete receipt")
	}
	if e := validatePlan(r.Plan); e != nil {
		return e
	}
	if r.NetFormat == "" || (r.Plan.Candidate.Net.Path != "" && (r.Hidden <= 0 || r.Buckets <= 0 || r.KingBuckets <= 0)) {
		return fmt.Errorf("unresolved evaluator")
	}
	for _, role := range []string{"manifest", "data", "training", "export", "parity"} {
		a, ok := r.Plan.Candidate.Provenance[role]
		if !ok || !filepath.IsAbs(a.Path) || len(a.SHA256) != 64 {
			return fmt.Errorf("missing %s provenance", role)
		}
	}
	if r.Host.CPUStatus != "0-15" || r.Host.Nice != "19" || r.AfterClockHost.CPUStatus != "0-15" || r.AfterClockHost.Nice != "19" {
		return fmt.Errorf("unproved scheduling")
	}
	for _, h := range []Host{r.Host, r.AfterClockHost} {
		if e := checkLoad(h, r.Plan.MaxLoad); e != nil {
			return e
		}
	}
	roles := []string{"candidate"}
	for _, a := range r.Plan.Anchors {
		roles = append(roles, a.Name)
		if len(a.Engine.SHA256) != 64 {
			return fmt.Errorf("unresolved anchor hash")
		}
	}
	if len(r.Probes) != len(roles) || len(r.Selected) != len(roles) {
		return fmt.Errorf("missing/extra engine probes")
	}
	for _, role := range roles {
		s, ok := r.Probes[role]
		if !ok || s.ExitCode != 0 || s.PID <= 0 || len(s.Events) == 0 || len(s.Options) == 0 || !reflect.DeepEqual(s.Selected, r.Selected[role]) {
			return fmt.Errorf("incomplete %s handshake", role)
		}
		a := r.Plan.Candidate.Engine
		if role != "candidate" {
			for _, anchor := range r.Plan.Anchors {
				if anchor.Name == role {
					a = anchor.Engine
				}
			}
		}
		if s.Engine != a || !reflect.DeepEqual(s.Selected, FrozenOptions(s.Options, identityFor(role, r.Plan.Candidate), r.Plan.MoveOverhead)) {
			return fmt.Errorf("%s identity/options mismatch", role)
		}
		if !reflect.DeepEqual(s.LaunchArgv, sessionLaunch(a)) || s.Scheduling != schedulingFor(a) {
			return fmt.Errorf("%s launch mismatch", role)
		}
		if strings.HasSuffix(strings.ToLower(a.Path), ".exe") {
			found := false
			for _, ev := range s.Events {
				if ev.Stream == "stderr" && strings.HasPrefix(ev.Line, "NGN claimbridge affinity 0xffff priority 0x40 pid ") {
					found = true
				}
			}
			if !found || r.Plan.WindowsBridge.Path == "" {
				return fmt.Errorf("unproved Windows scheduling")
			}
		}
		if e := validateProbe(s, role == "candidate", r.NetFormat); e != nil {
			return fmt.Errorf("%s: %w", role, e)
		}
	}
	if len(r.ClockCheck) != 2*r.Plan.Concurrency {
		return fmt.Errorf("missing bounded A/A clock check")
	}
	lanes := map[string]int64{}
	for _, c := range r.ClockCheck {
		key := fmt.Sprintf("%d/%s", c.Lane, c.Role)
		if _, ok := lanes[key]; ok || c.Lane < 0 || c.Lane >= r.Plan.Concurrency || (c.Role != "A" && c.Role != "A-copy") {
			return fmt.Errorf("A/A coverage mismatch")
		}
		if c.BeforeNS != r.Plan.BaseNS || c.UsedNS <= 0 || c.UsedNS >= c.BeforeNS || c.UsedNS > 500e6 || c.AfterNS != c.BeforeNS-c.UsedNS+r.Plan.IncrementNS || c.Move == "" || c.Session.ExitCode != 0 {
			return fmt.Errorf("A/A clock arithmetic/response failure")
		}
		if e := validateClockTranscript(c, r); e != nil {
			return e
		}
		lanes[key] = c.UsedNS
	}
	for lane := 0; lane < r.Plan.Concurrency; lane++ {
		d := lanes[fmt.Sprintf("%d/A", lane)] - lanes[fmt.Sprintf("%d/A-copy", lane)]
		if d < -250e6 || d > 250e6 {
			return fmt.Errorf("A/A clock asymmetry exceeds 250ms")
		}
	}
	return nil
}
func identityFor(role string, id Identity) *Identity {
	if role == "candidate" {
		return &id
	}
	return nil
}
func checkLoad(h Host, max float64) error {
	fields := strings.Fields(h.Load)
	if len(fields) == 0 {
		return fmt.Errorf("missing host load")
	}
	load, e := strconv.ParseFloat(fields[0], 64)
	if e != nil || math.IsNaN(load) || math.IsInf(load, 0) || load < 0 || load > max {
		return fmt.Errorf("host load %s exceeds bound %.2f", h.Load, max)
	}
	return nil
}

// preflightLifecycle matches every input byte-line and the UCI readiness order.
func preflightLifecycle(s SessionRecord, tail []string) error {
	if s.PID <= 0 || s.ExitCode != 0 {
		return fmt.Errorf("preflight process failed")
	}
	expected := []string{"uci"}
	keys := []string{}
	for k := range s.Selected {
		if k != "EvalFile" && k != "UseNNUE" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range []string{"EvalFile", "UseNNUE"} {
		if v, ok := s.Selected[k]; ok {
			expected = append(expected, "setoption name "+k+" value "+v)
		}
	}
	for _, k := range keys {
		expected = append(expected, "setoption name "+k+" value "+s.Selected[k])
	}
	expected = append(expected, "isready")
	expected = append(expected, tail...)
	expected = append(expected, "quit")
	commands := []string{}
	options := map[string]string{}
	ids := 0
	uciSent, handshake, readyPending, configured := false, false, false, false
	last := int64(-1)
	for _, ev := range s.Events {
		if ev.AtNS < last {
			return fmt.Errorf("nonmonotonic preflight transcript")
		}
		last = ev.AtNS
		if ev.Stream != "stdin" && engineError(ev.Line) {
			return fmt.Errorf("preflight engine error")
		}
		switch ev.Stream {
		case "stdin":
			commands = append(commands, ev.Line)
			switch {
			case ev.Line == "uci":
				if uciSent {
					return fmt.Errorf("duplicate uci")
				}
				uciSent = true
			case strings.HasPrefix(ev.Line, "setoption name "):
				if !handshake || configured || readyPending {
					return fmt.Errorf("preflight option order")
				}
			case ev.Line == "isready":
				if !handshake || readyPending {
					return fmt.Errorf("preflight readiness order")
				}
				readyPending = true
			case ev.Line == "quit":
				if !configured || readyPending {
					return fmt.Errorf("preflight incomplete on quit")
				}
			default:
				if !configured || readyPending {
					return fmt.Errorf("preflight command before ready")
				}
			}
		case "stdout":
			switch {
			case ev.Line == "uciok":
				if !uciSent || handshake {
					return fmt.Errorf("unsolicited uciok")
				}
				handshake = true
			case ev.Line == "readyok":
				if !readyPending {
					return fmt.Errorf("unsolicited readyok")
				}
				readyPending = false
				configured = true
			case strings.HasPrefix(ev.Line, "id "):
				if !uciSent || handshake {
					return fmt.Errorf("preflight identity order")
				}
				ids++
			case strings.HasPrefix(ev.Line, "option name "):
				if !uciSent || handshake {
					return fmt.Errorf("preflight advertisement order")
				}
				f := strings.SplitN(strings.TrimPrefix(ev.Line, "option name "), " type ", 2)
				if len(f) != 2 || f[0] == "" {
					return fmt.Errorf("bad probe option")
				}
				if _, ok := options[f[0]]; ok {
					return fmt.Errorf("duplicate probe option")
				}
				options[f[0]] = f[1]
			}
		case "stderr":
		default:
			return fmt.Errorf("unknown preflight stream")
		}
	}
	if !reflect.DeepEqual(commands, expected) || !reflect.DeepEqual(options, s.Options) || ids == 0 || !handshake || !configured || readyPending {
		return fmt.Errorf("incomplete preflight lifecycle/options")
	}
	return nil
}
func validateProbe(s SessionRecord, candidate bool, format string) error {
	tail := []string{}
	if candidate {
		for _, fen := range []string{StartFEN, "4k3/8/8/8/8/8/4P3/4K3 w - - 0 1"} {
			tail = append(tail, PositionCommand(fen, nil), "eval", "isready")
		}
	}
	if e := preflightLifecycle(s, tail); e != nil {
		return e
	}
	evals := 0
	effective := false
	backend := "hce"
	if format != "HCE" {
		backend = strings.ToLower(format)
	}
	for _, ev := range s.Events {
		if ev.Stream == "stdout" {
			if strings.HasPrefix(ev.Line, "info string eval backend "+backend+" score_cp ") {
				evals++
			}
			if ev.Line == "info string threads configured 1 effective 1" {
				effective = true
			}
		}
	}
	if candidate && (evals != 2 || !effective) {
		return fmt.Errorf("unproved evaluator/threads: expected %s, probes %d, effective Threads=1 %t", backend, evals, effective)
	}
	return nil
}
func validateClockTranscript(c ClockProbe, r Receipt) error {
	s := c.Session
	identity := r.Probes["candidate"]
	if s.Engine != identity.Engine || !reflect.DeepEqual(s.Options, identity.Options) || !reflect.DeepEqual(s.Selected, identity.Selected) || !reflect.DeepEqual(s.LaunchArgv, identity.LaunchArgv) || s.Scheduling != identity.Scheduling {
		return fmt.Errorf("A/A identity mismatch")
	}
	goAt, bestAt := int64(-1), int64(-1)
	moves, stops := 0, 0
	tail := []string{PositionCommand(StartFEN, nil), GoCommand([2]int64{r.Plan.BaseNS, r.Plan.BaseNS}, r.Plan.IncrementNS)}
	for _, ev := range s.Events {
		if ev.Stream == "stdin" && strings.HasPrefix(ev.Line, "go ") {
			if goAt >= 0 || ev.Line != tail[1] {
				return fmt.Errorf("A/A go mismatch")
			}
			goAt = ev.AtNS
		}
		if ev.Stream == "stdin" && ev.Line == "stop" {
			if goAt < 0 || stops > 0 {
				return fmt.Errorf("A/A unexpected stop")
			}
			stops++
			tail = append(tail, "stop")
		}
		if ev.Stream == "stdout" && strings.HasPrefix(ev.Line, "bestmove ") {
			f := strings.Fields(ev.Line)
			if goAt < 0 || len(f) < 2 || f[1] != c.Move {
				return fmt.Errorf("A/A bestmove mismatch")
			}
			bestAt = ev.AtNS
			moves++
		}
	}
	if e := preflightLifecycle(s, tail); e != nil {
		return e
	}
	if goAt < 0 || bestAt < 0 || moves != 1 || bestAt-goAt != c.UsedNS {
		return fmt.Errorf("A/A transcript clock mismatch")
	}
	pos, e := engine.ParseFEN(StartFEN)
	if e != nil {
		return e
	}
	_, e = LegalMove(pos, c.Move)
	return e
}
func probe(a Artifact, id *Identity, overhead int) (SessionRecord, error) {
	s, e := StartSession(a, nil)
	if e != nil {
		if s != nil {
			return s.Snapshot(), e
		}
		return SessionRecord{}, e
	}
	options := s.Snapshot().Options
	selected := FrozenOptions(options, id, overhead)
	if e = s.Close(); e != nil {
		return s.Snapshot(), e
	}
	s, e = StartSession(a, selected)
	if e != nil {
		if s != nil {
			return s.Snapshot(), e
		}
		return SessionRecord{}, e
	}
	if id != nil {
		for _, fen := range []string{StartFEN, "4k3/8/8/8/8/8/4P3/4K3 w - - 0 1"} {
			if _, e = s.send(PositionCommand(fen, nil)); e != nil {
				break
			}
			if _, e = s.send("eval"); e != nil {
				break
			}
			if e = s.ready(); e != nil {
				break
			}
		}
	}
	if closeErr := s.Close(); e == nil {
		e = closeErr
	}
	return s.Snapshot(), e
}
func ClockCheck(p Plan, selected map[string]string) ([]ClockProbe, error) {
	// Bounded stop-response A/A probes at declared clocks/concurrency; no games.
	var mu sync.Mutex
	var wg sync.WaitGroup
	var out []ClockProbe
	var first error
	for lane := 0; lane < p.Concurrency; lane++ {
		wg.Add(1)
		go func(lane int) {
			defer wg.Done()
			for _, role := range []string{"A", "A-copy"} {
				cp := ClockProbe{Lane: lane, Role: role, BeforeNS: p.BaseNS}
				s, e := StartSession(p.Candidate.Engine, selected)
				if e == nil {
					_, e = s.send(PositionCommand(StartFEN, nil))
					gi := 0
					if e == nil {
						gi, e = s.send(GoCommand([2]int64{p.BaseNS, p.BaseNS}, p.IncrementNS))
					}
					if e == nil {
						timerDone := make(chan struct{})
						timer := time.AfterFunc(50*time.Millisecond, func() { s.send("stop"); close(timerDone) })
						ctx, cancel := context.WithTimeout(ProcessContext, 3*time.Second)
						best, searchErr := s.until(ctx, "bestmove")
						cancel()
						if !timer.Stop() {
							<-timerDone
						}
						e = searchErr
						if e == nil {
							snap := s.Snapshot()
							cp.UsedNS = snap.Events[best.index].AtNS - snap.Events[gi].AtNS
							f := strings.Fields(best.text)
							if len(f) < 2 {
								e = fmt.Errorf("missing A/A move")
							} else {
								cp.Move = f[1]
								pos, _ := engine.ParseFEN(StartFEN)
								_, e = LegalMove(pos, cp.Move)
							}
							cp.AfterNS = p.BaseNS - cp.UsedNS + p.IncrementNS
							if cp.UsedNS <= 0 || cp.UsedNS >= p.BaseNS || cp.UsedNS > 500e6 {
								e = fmt.Errorf("A/A clock response failed")
							}
						}
					}
				}
				if s != nil {
					if closeErr := s.Close(); e == nil {
						e = closeErr
					}
					cp.Session = s.Snapshot()
				}
				mu.Lock()
				if e != nil && first == nil {
					first = e
				}
				out = append(out, cp)
				mu.Unlock()
			}
		}(lane)
	}
	wg.Wait()
	sort.Slice(out, func(i, j int) bool {
		if out[i].Lane == out[j].Lane {
			return out[i].Role < out[j].Role
		}
		return out[i].Lane < out[j].Lane
	})
	return out, first
}

// PreflightPlan checks artifacts/build metadata and all 80 owned openings without
// launching an engine. It also pins adjacent/advertised Maelstrom dependencies.
func PreflightPlan(p Plan, sourceRoot string, toolBuild []string) (Plan, error) {
	if e := validatePlan(p); e != nil {
		return p, e
	}
	commit, e := command("git", "-C", sourceRoot, "rev-parse", "HEAD")
	if e != nil {
		return p, e
	}
	if e = cleanSource(sourceRoot, commit); e != nil {
		return p, e
	}
	if e = cleanSource(p.Candidate.SourceRoot, p.Candidate.SourceCommit); e != nil {
		return p, e
	}
	if e = buildIdentity(p.Runner, commit, "go1.25.5", toolBuild); e != nil {
		return p, e
	}
	if e = buildIdentity(p.Candidate.Engine, p.Candidate.SourceCommit, p.Candidate.GoVersion, p.Candidate.BuildArgv); e != nil {
		return p, e
	}
	for _, role := range []string{"manifest", "data", "training", "export", "parity"} {
		a, ok := p.Candidate.Provenance[role]
		if !ok {
			return p, fmt.Errorf("missing %s provenance", role)
		}
		if e = Check(a); e != nil {
			return p, e
		}
	}
	if p.Candidate.Net.Path != "" {
		if e = Check(p.Candidate.Net); e != nil {
			return p, e
		}
		if _, e = engine.LoadNGNN1(p.Candidate.Net.Path); e != nil {
			return p, e
		}
	} else if p.Candidate.Net.SHA256 != "" {
		return p, fmt.Errorf("HCE net hash must be empty")
	}
	if e = Check(p.Openings); e != nil {
		return p, e
	}
	if _, _, e = LoadOpenings(p.Openings.Path); e != nil {
		return p, e
	}
	for i := range p.Anchors {
		a := &p.Anchors[i]
		if a.Engine.SHA256 == "" {
			a.Engine, e = Bind(a.Engine.Path)
			if e != nil {
				return p, e
			}
		}
		if e = Check(a.Engine); e != nil {
			return p, e
		}
		if strings.HasPrefix(a.Name, "Maelstrom") {
			files, e := os.ReadDir(filepath.Dir(a.Engine.Path))
			if e != nil {
				return p, e
			}
			for _, f := range files {
				ext := strings.ToLower(filepath.Ext(f.Name()))
				if !f.IsDir() && (ext == ".nnue" || ext == ".net" || ext == ".bin") {
					dep, e := Bind(filepath.Join(filepath.Dir(a.Engine.Path), f.Name()))
					if e != nil {
						return p, e
					}
					found := false
					for _, d := range a.Dependencies {
						if d == dep {
							found = true
						}
					}
					if !found {
						a.Dependencies = append(a.Dependencies, dep)
					}
				}
			}
		}
		if strings.HasSuffix(strings.ToLower(a.Engine.Path), ".exe") {
			if e = Check(p.WindowsBridge); e != nil {
				return p, fmt.Errorf("Windows bridge: %w", e)
			}
			bi, e := buildinfo.ReadFile(p.WindowsBridge.Path)
			if e != nil {
				return p, e
			}
			settings := map[string]string{}
			for _, s := range bi.Settings {
				settings[s.Key] = s.Value
			}
			if bi.GoVersion != "go1.25.5" || settings["GOOS"] != "windows" || settings["GOARCH"] != "amd64" || settings["GOAMD64"] != "v3" || settings["vcs.revision"] != commit || settings["vcs.modified"] != "false" {
				return p, fmt.Errorf("bridge build metadata mismatch")
			}
		}
	}
	return p, nil
}
func CreateReceipt(p Plan, dir, sourceRoot string, toolBuild []string, screenReceipt, screenAnalysis string) (Receipt, error) {
	r := Receipt{Schema: Schema, CreatedUTC: time.Now().UTC().Format(time.RFC3339Nano), ToolSourceRoot: sourceRoot, ToolGoVersion: "go1.25.5", ToolBuildArgv: toolBuild, Probes: map[string]SessionRecord{}, Selected: map[string]map[string]string{}, Questions: []string{
		"The 400-ply cap counts opening and played plies from the initial FEN; true terminals are tested first.",
		"Threefold repetition and fifty-move eligibility are automatic rule draws; mate/stalemate take priority.",
		"Bounded A/A is a same-clock/concurrency 50ms stop-response control, max 500ms response and 250ms paired asymmetry, not a strength/parity match.",
		"Host load bound is 16 on this 16-logical-CPU rig; probes must also pass. This is an operational bound, not a correction to statistics.",
	}}
	var nonce [16]byte
	if _, e := rand.Read(nonce[:]); e != nil {
		return r, e
	}
	r.AttemptID = fmt.Sprintf("%x", nonce)
	var e error
	p, e = PreflightPlan(p, sourceRoot, toolBuild)
	if e != nil {
		return r, e
	}
	r.Plan = p
	WindowsBridge = p.WindowsBridge
	if e = NoOtherMatch(); e != nil {
		return r, e
	}
	r.Host, e = HostNow()
	if e != nil {
		return r, e
	}
	if r.Host.CPUStatus != "0-15" {
		return r, fmt.Errorf("live receipt requires exclusive rig CPU 0-15 after T80 worker exits")
	}
	if e = checkLoad(r.Host, p.MaxLoad); e != nil {
		return r, e
	}
	r.ToolSourceCommit, e = command("git", "-C", sourceRoot, "rev-parse", "HEAD")
	if e != nil {
		return r, e
	}
	r.SourceClean = true
	r.CandidateSourceClean = true
	version, e := command("/usr/local/go/bin/go", "version")
	if e != nil || version != "go version go1.25.5 linux/amd64" {
		return r, fmt.Errorf("host Go version mismatch")
	}
	r.NetFormat = "HCE"
	if p.Candidate.Net.Path != "" {
		b, e := os.ReadFile(p.Candidate.Net.Path)
		if e != nil {
			return r, e
		}
		v := binary.LittleEndian.Uint32(b[4:8])
		r.NetFormat = fmt.Sprintf("NGNN%d", v)
		r.Hidden = int(binary.LittleEndian.Uint32(b[8:12]))
		r.Buckets = 1
		r.KingBuckets = 1
		if v == 2 {
			r.Buckets = int(binary.LittleEndian.Uint32(b[12:16]))
		}
		if v == 3 {
			r.KingBuckets = int(binary.LittleEndian.Uint32(b[12:16]))
			r.Buckets = int(binary.LittleEndian.Uint32(b[16:20]))
		}
	}
	_, slice, e := LoadOpenings(p.Openings.Path)
	if e != nil {
		return r, e
	}
	r.OpeningCount = 80
	if e = os.Mkdir(dir, 0755); e != nil {
		return r, e
	}
	slicePath := filepath.Join(dir, "first-80-openings.txt")
	f, e := os.OpenFile(slicePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0444)
	if e != nil {
		return r, e
	}
	_, e = f.Write(slice)
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		return r, e
	}
	r.FrozenOpenings = Artifact{slicePath, fmt.Sprintf("%x", sha256.Sum256(slice))}
	for i := range r.Plan.Anchors {
		a := &r.Plan.Anchors[i]
		rec, e := probe(a.Engine, nil, p.MoveOverhead)
		if e != nil {
			WriteJSON(filepath.Join(dir, "preflight-failure.json"), map[string]any{"error": e.Error(), "session": rec})
			return r, e
		}
		r.Probes[a.Name] = rec
		r.Selected[a.Name] = rec.Selected
		// Any advertised evaluator file is an explicit dependency, including an
		// external path outside the adjacent directory. Embedded weights are bound
		// by the executable digest. No opponent source is inspected.
		for k, v := range rec.Selected {
			if strings.Contains(strings.ToLower(k), "file") && (strings.Contains(strings.ToLower(k), "eval") || strings.Contains(strings.ToLower(k), "net")) && v != "" && v != "<empty>" {
				path := v
				if !filepath.IsAbs(path) {
					path = filepath.Join(filepath.Dir(a.Engine.Path), path)
				}
				dep, e := Bind(path)
				if e != nil {
					return r, fmt.Errorf("unresolved %s dependency %s: %w", a.Name, k, e)
				}
				found := false
				for _, d := range a.Dependencies {
					if d == dep {
						found = true
					}
				}
				if !found {
					a.Dependencies = append(a.Dependencies, dep)
				}
			}
		}
	}
	pr, e := probe(p.Candidate.Engine, &p.Candidate, p.MoveOverhead)
	if e != nil {
		WriteJSON(filepath.Join(dir, "preflight-failure.json"), map[string]any{"error": e.Error(), "session": pr})
		return r, e
	}
	r.Probes["candidate"] = pr
	r.Selected["candidate"] = pr.Selected
	r.ClockCheck, e = ClockCheck(p, pr.Selected)
	if e != nil {
		WriteJSON(filepath.Join(dir, "preflight-failure.json"), map[string]any{"error": e.Error(), "probes": r.ClockCheck})
		return r, e
	}
	r.AfterClockHost, e = HostNow()
	if e != nil {
		return r, e
	}
	if p.Mode == "confirmation" {
		if screenReceipt == "" || screenAnalysis == "" {
			return r, fmt.Errorf("confirmation requires admitted screen receipt and analysis")
		}
		sr, e := ReadReceipt(screenReceipt)
		if e != nil {
			return r, e
		}
		var sa Analysis
		if e = ReadJSON(screenAnalysis, &sa); e != nil {
			return r, e
		}
		verified, e := Analyze(screenReceipt, sa.Audit.Path, sa.GameDirectory)
		if e != nil {
			return r, e
		}
		if !reflect.DeepEqual(sa, verified) || !sa.ScreenEligible || sr.Plan.Mode != "screen" || !reflect.DeepEqual(sr.Plan.Candidate, p.Candidate) || !reflect.DeepEqual(sr.Selected, r.Selected) || sr.Plan.Runner != p.Runner || sr.Plan.WindowsBridge != p.WindowsBridge || !reflect.DeepEqual(sr.Plan.Anchors, r.Plan.Anchors) {
			return r, fmt.Errorf("screen does not admit same frozen candidate/tool/options/anchors")
		}
		r.ScreenReceipt, e = Bind(screenReceipt)
		if e != nil {
			return r, e
		}
		r.ScreenAnalysis, e = Bind(screenAnalysis)
		if e != nil {
			return r, e
		}
		if sa.ReceiptSHA256 != r.ScreenReceipt.SHA256 {
			return r, fmt.Errorf("screen analysis receipt mismatch")
		}
		WindowsBridge = p.WindowsBridge
	}
	if e = ValidateReceipt(r); e != nil {
		WriteJSON(filepath.Join(dir, "preflight-failure.json"), map[string]any{"error": e.Error(), "receipt": r})
		return r, e
	}
	if e = Recheck(r); e != nil {
		return r, e
	}
	receiptPath := filepath.Join(dir, "receipt.json")
	if e = WriteJSON(receiptPath, r); e != nil {
		return r, e
	}
	seal, e := Bind(receiptPath)
	if e != nil {
		return r, e
	}
	if e = WriteJSON(filepath.Join(dir, "receipt-seal.json"), seal); e != nil {
		return r, e
	}
	return r, nil
}
func SaveRecheck(receiptPath, out string) error {
	r, e := ReadReceipt(receiptPath)
	if e != nil {
		return e
	}
	if e = Recheck(r); e != nil {
		return e
	}
	seal, e := Bind(receiptPath)
	if e != nil {
		return e
	}
	h, e := HostNow()
	if e != nil {
		return e
	}
	return WriteJSON(out, RecheckReceipt{Schema: Schema, AtUTC: time.Now().UTC().Format(time.RFC3339Nano), Receipt: seal, Artifacts: allArtifacts(r), Host: h, Unchanged: true})
}
