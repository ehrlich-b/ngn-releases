package claim

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/ehrlich-b/ngn/engine"
)

// Auditor material policy is computed from the FEN mailbox, independently of
// the runner's bitboard material helper. No external oracle/toolchain is used.
func auditDead(p *engine.Position) bool {
	f := strings.Fields(engine.GenerateFEN(p))[0]
	rank, file, minors, knights := 7, 0, 0, 0
	color := -1
	same := true
	for _, ch := range f {
		switch {
		case ch == '/':
			rank--
			file = 0
			continue
		case ch >= '1' && ch <= '8':
			file += int(ch - '0')
			continue
		case ch == 'k' || ch == 'K':
		case ch == 'n' || ch == 'N':
			minors++
			knights++
		case ch == 'b' || ch == 'B':
			minors++
			c := (rank + file) % 2
			if color < 0 {
				color = c
			} else if color != c {
				same = false
			}
		default:
			return false
		}
		file++
	}
	return minors <= 1 || (knights == 0 && same)
}
func AuditTerminal(p *engine.Position, seen map[string]int, half, total, cap int) (string, string) {
	legal := engine.GenerateLegalMoves(p)
	if len(legal) == 0 {
		if p.IsInCheck() {
			winner := "1-0"
			if p.Turn() == engine.White {
				winner = "0-1"
			}
			return winner, "checkmate"
		}
		return "1/2-1/2", "stalemate"
	}
	if seen[PositionKey(p)] >= 3 {
		return "1/2-1/2", "repetition"
	}
	if half >= 100 {
		return "1/2-1/2", "fifty-move"
	}
	if auditDead(p) {
		return "1/2-1/2", "insufficient-material"
	}
	if total >= cap {
		return "1/2-1/2", "ply-cap"
	}
	return "", ""
}
func advanceAudit(p *engine.Position, token string, seen map[string]int, half *int) error {
	m, e := LegalMove(p, token)
	if e != nil {
		return e
	}
	if m.MovingPiece().Type() == engine.Pawn || m.CapturedPiece() != engine.NoPiece {
		*half = 0
	} else {
		*half++
	}
	if _, _, _, ok := p.GameMakeMove(m); !ok {
		return fmt.Errorf("legal move refused: %s", token)
	}
	if int(p.HalfMoveClock) != *half {
		return fmt.Errorf("halfmove arithmetic mismatch")
	}
	seen[PositionKey(p)]++
	return nil
}
func validateSession(s SessionRecord, identity SessionRecord) error {
	if s.Engine != identity.Engine || s.PID <= 0 || s.ExitCode != 0 || !reflect.DeepEqual(s.Options, identity.Options) || !reflect.DeepEqual(s.Selected, identity.Selected) || !reflect.DeepEqual(s.LaunchArgv, identity.LaunchArgv) || s.Scheduling != identity.Scheduling {
		return fmt.Errorf("session identity/options/exit/launch mismatch")
	}
	if len(s.Events) == 0 {
		return fmt.Errorf("missing transcript")
	}
	last := int64(-1)
	phase := 0
	pendingReady := false
	bridgeReceipts := 0
	sets := map[string]string{}
	options := map[string]string{}
	var ids, expectedIDs []string
	for _, e := range identity.Events {
		if e.Stream == "stdout" && strings.HasPrefix(e.Line, "id ") {
			expectedIDs = append(expectedIDs, e.Line)
		}
	}
	if len(expectedIDs) == 0 {
		return fmt.Errorf("missing identity handshake")
	}
	for _, e := range s.Events {
		if e.AtNS < last {
			return fmt.Errorf("nonmonotonic transcript event")
		}
		last = e.AtNS
		if e.Stream != "stdin" && engineError(e.Line) {
			return fmt.Errorf("unexpected engine error")
		}
		switch e.Stream {
		case "stdin":
			switch {
			case e.Line == "uci":
				if phase != 0 {
					return fmt.Errorf("duplicate/out-of-order uci")
				}
				phase = 1
			case strings.HasPrefix(e.Line, "setoption name "):
				if phase != 2 || pendingReady {
					return fmt.Errorf("setoption outside configuration")
				}
				f := strings.SplitN(strings.TrimPrefix(e.Line, "setoption name "), " value ", 2)
				if len(f) != 2 {
					return fmt.Errorf("bad setoption")
				}
				if _, ok := options[f[0]]; !ok {
					return fmt.Errorf("unadvertised option")
				}
				if _, dup := sets[f[0]]; dup {
					return fmt.Errorf("duplicate setoption")
				}
				if f[0] == "UseNNUE" {
					if _, yes := s.Selected["EvalFile"]; yes {
						if _, done := sets["EvalFile"]; !done {
							return fmt.Errorf("UseNNUE before EvalFile")
						}
					}
				}
				sets[f[0]] = f[1]
			case e.Line == "isready":
				if pendingReady || (phase != 2 && phase != 4) {
					return fmt.Errorf("out-of-order isready")
				}
				pendingReady = true
			case e.Line == "ucinewgame":
				if phase != 3 || pendingReady {
					return fmt.Errorf("out-of-order ucinewgame")
				}
				phase = 4
			case strings.HasPrefix(e.Line, "position ") || strings.HasPrefix(e.Line, "go "):
				if phase != 5 || pendingReady {
					return fmt.Errorf("search before game readiness")
				}
			case e.Line == "quit":
				if phase != 5 || pendingReady {
					return fmt.Errorf("out-of-order quit")
				}
				phase = 6
			default:
				return fmt.Errorf("unexpected game command: %s", e.Line)
			}
		case "stdout":
			switch {
			case e.Line == "uciok":
				if phase != 1 {
					return fmt.Errorf("out-of-order uciok")
				}
				phase = 2
			case e.Line == "readyok":
				if !pendingReady {
					return fmt.Errorf("unsolicited readyok")
				}
				pendingReady = false
				if phase == 2 {
					phase = 3
				} else if phase == 4 {
					phase = 5
				} else {
					return fmt.Errorf("out-of-order readyok")
				}
			case strings.HasPrefix(e.Line, "id "):
				if phase != 1 {
					return fmt.Errorf("identity after handshake")
				}
				ids = append(ids, e.Line)
			case strings.HasPrefix(e.Line, "option name "):
				if phase != 1 {
					return fmt.Errorf("option after handshake")
				}
				f := strings.SplitN(strings.TrimPrefix(e.Line, "option name "), " type ", 2)
				if len(f) != 2 || f[0] == "" {
					return fmt.Errorf("bad advertised option")
				}
				if _, dup := options[f[0]]; dup {
					return fmt.Errorf("duplicate advertised option")
				}
				options[f[0]] = f[1]
			}
		case "stderr":
			const prefix = "NGN claimbridge affinity 0xffff priority 0x40 pid "
			if strings.HasPrefix(e.Line, prefix) {
				pid, err := strconv.Atoi(strings.TrimPrefix(e.Line, prefix))
				if err != nil || pid <= 0 {
					return fmt.Errorf("malformed Windows scheduling receipt")
				}
				bridgeReceipts++
			}
		default:
			return fmt.Errorf("unknown transcript stream")
		}
	}
	if phase != 6 || pendingReady || !reflect.DeepEqual(ids, expectedIDs) || !reflect.DeepEqual(options, s.Options) || !reflect.DeepEqual(sets, s.Selected) {
		return fmt.Errorf("incomplete UCI lifecycle/option receipt")
	}
	if strings.HasSuffix(strings.ToLower(s.Engine.Path), ".exe") && bridgeReceipts != 1 {
		return fmt.Errorf("missing/duplicate Windows affinity/priority receipt")
	}
	return nil
}

type rawSearch struct {
	position, goCmd, move string
	goEvent, bestEvent    int
}

func rawSearches(s SessionRecord) ([]rawSearch, error) {
	var out []rawSearch
	pos := ""
	var active *rawSearch
	for i, e := range s.Events {
		if e.Stream == "stdin" {
			switch {
			case strings.HasPrefix(e.Line, "position "):
				if active != nil || pos != "" {
					return nil, fmt.Errorf("extra position/during search")
				}
				pos = e.Line
			case strings.HasPrefix(e.Line, "go "):
				if active != nil || pos == "" {
					return nil, fmt.Errorf("unpaired go")
				}
				active = &rawSearch{position: pos, goCmd: e.Line, goEvent: i}
				pos = ""
			case e.Line == "stop":
				return nil, fmt.Errorf("unexpected stop in game")
			case e.Line == "quit":
				if active != nil || pos != "" {
					return nil, fmt.Errorf("unfinished search at quit")
				}
			}
		} else if e.Stream == "stdout" && strings.HasPrefix(e.Line, "bestmove") {
			if active == nil {
				return nil, fmt.Errorf("unpaired bestmove")
			}
			f := strings.Fields(e.Line)
			if len(f) != 2 && !(len(f) == 4 && f[2] == "ponder") {
				return nil, fmt.Errorf("malformed bestmove")
			}
			active.move = f[1]
			active.bestEvent = i
			out = append(out, *active)
			active = nil
		}
	}
	if active != nil || pos != "" {
		return nil, fmt.Errorf("missing bestmove/go")
	}
	return out, nil
}
func AuditGame(g Game, r Receipt, o Opening) error {
	if r.AttemptID == "" || g.AttemptID != r.AttemptID {
		return fmt.Errorf("game belongs to another pre-run attempt")
	}
	created, e := time.Parse(time.RFC3339Nano, r.CreatedUTC)
	if e != nil {
		return e
	}
	started, e := time.Parse(time.RFC3339Nano, g.StartedUTC)
	if e != nil {
		return fmt.Errorf("missing/invalid game start: %w", e)
	}
	ended, e := time.Parse(time.RFC3339Nano, g.EndedUTC)
	if e != nil || started.Before(created) || ended.Before(started) {
		return fmt.Errorf("game predates receipt or has invalid end timestamp")
	}
	p := r.Plan
	if g.Schema != Schema || g.Error != "" || g.BaseNS != p.BaseNS || g.IncrementNS != p.IncrementNS || g.Cap != p.Cap || p.Cap != 400 || !reflect.DeepEqual(g.Opening, o) {
		return fmt.Errorf("game header/opening/error mismatch")
	}
	if g.CandidateColor != "white" && g.CandidateColor != "black" {
		return fmt.Errorf("bad color")
	}
	if g.PairID != fmt.Sprintf("%s-%02d", g.Anchor, o.Index) || g.ID != g.PairID+"-"+g.CandidateColor {
		return fmt.Errorf("pair/game ID mismatch")
	}
	ci, ok := r.Probes["candidate"]
	if !ok {
		return fmt.Errorf("missing candidate probe")
	}
	co, ok := r.Probes[g.Anchor]
	if !ok {
		return fmt.Errorf("missing anchor probe")
	}
	if !reflect.DeepEqual(g.Candidate.Selected, r.Selected["candidate"]) || !reflect.DeepEqual(g.Opponent.Selected, r.Selected[g.Anchor]) {
		return fmt.Errorf("receipt selected-options mismatch")
	}
	if e := validateSession(g.Candidate, ci); e != nil {
		return e
	}
	if e := validateSession(g.Opponent, co); e != nil {
		return e
	}
	cs, e := rawSearches(g.Candidate)
	if e != nil {
		return e
	}
	bs, e := rawSearches(g.Opponent)
	if e != nil {
		return e
	}
	raw := map[string][]rawSearch{"candidate": cs, "opponent": bs}
	cursor := map[string]int{}
	pos, e := engine.ParseFEN(o.FEN)
	if e != nil {
		return e
	}
	parts := strings.Fields(o.FEN)
	half, e := strconv.Atoi(parts[4])
	if e != nil {
		return e
	}
	seen := map[string]int{PositionKey(pos): 1}
	history := []string{}
	for i, t := range o.History {
		if _, reason := AuditTerminal(pos, seen, half, i, 400); reason != "" {
			return fmt.Errorf("opening continues %s", reason)
		}
		if e = advanceAudit(pos, t, seen, &half); e != nil {
			return e
		}
		history = append(history, t)
	}
	if _, reason := AuditTerminal(pos, seen, half, len(history), 400); reason != "" {
		return fmt.Errorf("opening ends terminal %s", reason)
	}
	clocks := [2]int64{p.BaseNS, p.BaseNS}
	for i, m := range g.Plies {
		if _, reason := AuditTerminal(pos, seen, half, len(history), 400); reason != "" {
			return fmt.Errorf("ply %d after terminal %s", i, reason)
		}
		stm := Color(pos)
		role := "opponent"
		session := g.Opponent
		if stm == g.CandidateColor {
			role = "candidate"
			session = g.Candidate
		}
		if m.STM != stm || m.Role != role || m.Before != clocks || m.UsedNS < 0 {
			return fmt.Errorf("ply %d STM/clock mismatch", i)
		}
		j := cursor[role]
		if j >= len(raw[role]) {
			return fmt.Errorf("missing raw search at ply %d", i)
		}
		search := raw[role][j]
		cursor[role]++
		if search.position != PositionCommand(o.FEN, history) || search.goCmd != GoCommand(clocks, p.IncrementNS) || search.move != m.Move || search.goEvent != m.GoEvent || search.bestEvent != m.BestEvent {
			return fmt.Errorf("ply %d transcript mismatch", i)
		}
		used := session.Events[m.BestEvent].AtNS - session.Events[m.GoEvent].AtNS
		if used != m.UsedNS {
			return fmt.Errorf("time-used mismatch")
		}
		side := 0
		if stm == "black" {
			side = 1
		}
		if used >= clocks[side] {
			return fmt.Errorf("clock loss")
		}
		clocks[side] += p.IncrementNS - used
		if m.After != clocks {
			return fmt.Errorf("increment/clock arithmetic mismatch")
		}
		if e = advanceAudit(pos, m.Move, seen, &half); e != nil {
			return e
		}
		history = append(history, m.Move)
	}
	for role, searches := range raw {
		if cursor[role] != len(searches) {
			return fmt.Errorf("missing recorded moves: %s", role)
		}
	}
	result, reason := AuditTerminal(pos, seen, half, len(history), 400)
	if result == "" || result != g.Result || reason != g.Reason {
		return fmt.Errorf("terminal mismatch: replay %s/%s, record %s/%s", result, reason, g.Result, g.Reason)
	}
	return nil
}
func AuditDirectory(receiptPath, dir string) (Audit, []Game, error) {
	a := Audit{Schema: Schema, Games: map[string]string{}, Counts: map[string]Counts{}}
	r, e := ReadReceipt(receiptPath)
	if e != nil {
		return a, nil, e
	}
	if e = Recheck(r); e != nil {
		return a, nil, e
	}
	h, e := Digest(receiptPath)
	if e != nil {
		return a, nil, e
	}
	a.ReceiptSHA256 = h
	ops, _, e := LoadOpenings(r.Plan.Openings.Path)
	if e != nil {
		return a, nil, e
	}
	expected := map[string]bool{}
	for _, anchor := range r.Plan.Anchors {
		for i := 0; i < r.Plan.Count; i++ {
			for _, color := range []string{"white", "black"} {
				expected[fmt.Sprintf("%s-%02d-%s.json", anchor.Name, i, color)] = true
			}
		}
	}
	files, e := os.ReadDir(dir)
	if e != nil {
		return a, nil, e
	}
	var games []Game
	pairs := map[string]map[int]uint8{}
	for _, f := range files {
		if f.Name() == "summary.json" {
			continue
		}
		if f.IsDir() || !expected[f.Name()] {
			return a, nil, fmt.Errorf("unexpected/missing-record file: %s", f.Name())
		}
		delete(expected, f.Name())
		path := filepath.Join(dir, f.Name())
		var g Game
		if e = ReadJSON(path, &g); e != nil {
			return a, nil, e
		}
		if g.Opening.Index < 0 || g.Opening.Index >= r.Plan.Count {
			return a, nil, fmt.Errorf("index out of sample")
		}
		if f.Name() != g.ID+".json" {
			return a, nil, fmt.Errorf("filename/ID mismatch")
		}
		if e = AuditGame(g, r, ops[g.Opening.Index]); e != nil {
			return a, nil, fmt.Errorf("%s: %w", g.ID, e)
		}
		h, e := Digest(path)
		if e != nil {
			return a, nil, e
		}
		a.Games[g.ID] = h
		games = append(games, g)
		mask := uint8(1)
		if g.CandidateColor == "black" {
			mask = 2
		}
		if pairs[g.Anchor] == nil {
			pairs[g.Anchor] = map[int]uint8{}
		}
		if pairs[g.Anchor][g.Opening.Index]&mask != 0 {
			return a, nil, fmt.Errorf("duplicate color in pair")
		}
		pairs[g.Anchor][g.Opening.Index] |= mask
		// Recount directly from white result/color, independently of collector.
		c := a.Counts[g.Anchor]
		c.N++
		switch g.Result {
		case "1/2-1/2":
			c.D++
		case "1-0":
			if g.CandidateColor == "white" {
				c.W++
			} else {
				c.L++
			}
		case "0-1":
			if g.CandidateColor == "black" {
				c.W++
			} else {
				c.L++
			}
		default:
			return a, nil, fmt.Errorf("unknown result")
		}
		if g.Reason == "ply-cap" {
			c.CapDraws++
		}
		a.Counts[g.Anchor] = c
	}
	if len(expected) != 0 {
		return a, nil, fmt.Errorf("missing %d games", len(expected))
	}
	for _, anchor := range r.Plan.Anchors {
		name := anchor.Name
		c := a.Counts[name]
		for i := 0; i < r.Plan.Count; i++ {
			if pairs[name][i] != 3 {
				return a, nil, fmt.Errorf("missing/incomplete pair %s/%d", name, i)
			}
			c.Pairs++
		}
		if c.N != 2*c.Pairs {
			return a, nil, fmt.Errorf("coverage mismatch")
		}
		a.Counts[name] = c
	}
	var s Summary
	if e = ReadJSON(filepath.Join(dir, "summary.json"), &s); e != nil {
		return a, nil, e
	}
	if s.Schema != Schema || s.Games != len(games) || !reflect.DeepEqual(s.Counts, a.Counts) {
		return a, nil, fmt.Errorf("collector W/D/L reconciliation failed")
	}
	a.Admitted = true
	return a, games, nil
}
