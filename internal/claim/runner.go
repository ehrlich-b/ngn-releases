package claim

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ehrlich-b/ngn/engine"
)

func Color(p *engine.Position) string {
	if p.Turn() == engine.White {
		return "white"
	}
	return "black"
}
func GoCommand(clocks [2]int64, inc int64) string {
	return fmt.Sprintf("go wtime %d btime %d winc %d binc %d", clocks[0]/1000000, clocks[1]/1000000, inc/1000000, inc/1000000)
}
func PositionCommand(fen string, history []string) string {
	s := "position fen " + fen
	if len(history) > 0 {
		s += " moves " + strings.Join(history, " ")
	}
	return s
}
func Play(p Plan, a Anchor, o Opening, color string, selected map[string]map[string]string) (g Game, err error) {
	g = Game{Schema: Schema, ID: fmt.Sprintf("%s-%02d-%s", a.Name, o.Index, color), PairID: fmt.Sprintf("%s-%02d", a.Name, o.Index), Anchor: a.Name, CandidateColor: color, Opening: o, BaseNS: p.BaseNS, IncrementNS: p.IncrementNS, Cap: p.Cap, Plies: []Ply{}}
	g.StartedUTC = time.Now().UTC().Format(time.RFC3339Nano)
	if color != "white" && color != "black" {
		return g, fmt.Errorf("invalid candidate color")
	}
	if p.Cap != 400 || p.BaseNS <= 0 || p.IncrementNS < 0 {
		return g, fmt.Errorf("invalid clock/cap")
	}
	if _, ok := selected["candidate"]; !ok {
		return g, fmt.Errorf("missing frozen candidate options")
	}
	if _, ok := selected[a.Name]; !ok {
		return g, fmt.Errorf("missing frozen anchor options")
	}
	var c, b *Session
	defer func() {
		defer func() { g.EndedUTC = time.Now().UTC().Format(time.RFC3339Nano) }()
		for _, s := range []*Session{c, b} {
			if s != nil {
				if e := s.Close(); e != nil && err == nil {
					err = e
				}
			}
		}
		if c != nil {
			g.Candidate = c.Snapshot()
		}
		if b != nil {
			g.Opponent = b.Snapshot()
		}
		if err != nil {
			g.Error = err.Error()
			g.Result = ""
			g.Reason = "operational-invalid"
		}
	}()
	c, err = StartSession(p.Candidate.Engine, selected["candidate"])
	if err != nil {
		return g, err
	}
	b, err = StartSession(a.Engine, selected[a.Name])
	if err != nil {
		return g, err
	}
	for _, s := range []*Session{c, b} {
		if _, err = s.send("ucinewgame"); err != nil {
			return
		}
		if err = s.ready(); err != nil {
			return
		}
	}
	pos, err := engine.ParseFEN(o.FEN)
	if err != nil {
		return
	}
	history := append([]string(nil), o.History...)
	for i, t := range o.History {
		if _, reason := RunnerTerminal(pos, i, p.Cap); reason != "" {
			err = fmt.Errorf("opening continues terminal %s", reason)
			return
		}
		var m engine.Move
		m, err = LegalMove(pos, t)
		if err != nil {
			return
		}
		if _, _, _, ok := pos.GameMakeMove(m); !ok {
			err = fmt.Errorf("opening legal move refused %s", t)
			return
		}
	}
	clocks := [2]int64{p.BaseNS, p.BaseNS}
	for {
		g.Result, g.Reason = RunnerTerminal(pos, len(history), p.Cap)
		if g.Reason != "" {
			return g, nil
		}
		stm := Color(pos)
		role := "opponent"
		s := b
		if stm == color {
			role = "candidate"
			s = c
		}
		side := 0
		if stm == "black" {
			side = 1
		}
		var t string
		var gi, bi int
		t, gi, bi, err = s.Search(PositionCommand(o.FEN, history), GoCommand(clocks, p.IncrementNS), clocks[side])
		if err != nil {
			return
		}
		r := s.Snapshot()
		used := r.Events[bi].AtNS - r.Events[gi].AtNS
		if used < 0 || used >= clocks[side] {
			err = fmt.Errorf("clock loss: %s used %d bank %d", role, used, clocks[side])
			return
		}
		m, e := LegalMove(pos, t)
		if e != nil {
			err = e
			return
		}
		after := clocks
		after[side] -= used
		after[side] += p.IncrementNS
		g.Plies = append(g.Plies, Ply{t, stm, role, clocks, after, used, gi, bi})
		clocks = after
		if _, _, _, ok := pos.GameMakeMove(m); !ok {
			err = fmt.Errorf("played legal move refused %s", t)
			return
		}
		history = append(history, t)
	}
}
func candidateCounts(g Game, c Counts) Counts {
	c.N++
	if g.Result == "1/2-1/2" {
		c.D++
	} else if (g.Result == "1-0") == (g.CandidateColor == "white") {
		c.W++
	} else {
		c.L++
	}
	if g.Reason == "ply-cap" {
		c.CapDraws++
	}
	return c
}
func Run(receiptPath, dir string) error {
	r, e := ReadReceipt(receiptPath)
	if e != nil {
		return e
	}
	if r.Plan.Mode == "identity-check" {
		return fmt.Errorf("identity-check receipt never authorizes games")
	}
	if e = Recheck(r); e != nil {
		return e
	}
	if e := NoOtherMatch(); e != nil {
		return e
	}
	host, e := HostNow()
	if e != nil {
		return e
	}
	if e = checkLoad(host, r.Plan.MaxLoad); e != nil {
		return e
	}
	if e := os.Mkdir(dir, 0755); e != nil {
		return e
	}
	ops, _, e := LoadOpenings(r.Plan.Openings.Path)
	if e != nil {
		return e
	}
	type job struct {
		a     Anchor
		o     Opening
		color string
	}
	jobs := make(chan job)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var first error
	summary := Summary{Schema: Schema, Counts: map[string]Counts{}}
	for i := 0; i < r.Plan.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				mu.Lock()
				failed := first != nil
				mu.Unlock()
				if failed {
					continue
				}
				g, e := Play(r.Plan, j.a, j.o, j.color, r.Selected)
				g.AttemptID = r.AttemptID
				path := filepath.Join(dir, g.ID+".json")
				if w := WriteJSON(path, g); w != nil {
					e = w
				}
				mu.Lock()
				if e != nil && first == nil {
					first = fmt.Errorf("%s: %w", g.ID, e)
				}
				if e == nil {
					summary.Games++
					summary.Counts[g.Anchor] = candidateCounts(g, summary.Counts[g.Anchor])
				}
				mu.Unlock()
			}
		}()
	}
	for _, a := range r.Plan.Anchors {
		for i := 0; i < r.Plan.Count; i++ {
			for _, color := range []string{"white", "black"} {
				jobs <- job{a, ops[i], color}
			}
		}
	}
	close(jobs)
	wg.Wait()
	if first != nil {
		WriteJSON(filepath.Join(dir, "failure.json"), map[string]string{"error": first.Error()})
		return first
	}
	for name, c := range summary.Counts {
		c.Pairs = c.N / 2
		summary.Counts[name] = c
	}
	if summary.Games != 2*r.Plan.Count*len(r.Plan.Anchors) {
		return fmt.Errorf("missing game records: got %d", summary.Games)
	}
	if e = WriteJSON(filepath.Join(dir, "summary.json"), summary); e != nil {
		return e
	}
	if e = Recheck(r); e != nil {
		return e
	}
	_, _, e = AuditDirectory(receiptPath, dir)
	return e
}
