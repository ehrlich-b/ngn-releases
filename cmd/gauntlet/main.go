// Command gauntlet measures NGN's ABSOLUTE rating by playing it against a ladder
// of opponents with PUBLISHED CCRL ratings (Blunder versions in opponents/), then
// pooling the per-anchor performance ratings by inverse variance. This replaces
// Stockfish's UCI_Elo slider as the absolute yardstick — we measured the slider
// ~+400 inflated vs CCRL (NGN scored 70% vs "SF2400" but ~7% vs Blunder 7.4.0,
// CCRL 2532). It runs serially at equal movetime through the hardened internal/uci
// harness (handshake retry, warmup probe, stop-on-overshoot), so a slow or glitchy
// opponent can't forfeit its way into a bogus result.
//
//	scripts/build-anchors.sh                     # build the ladder once
//	./build/gauntlet -games 24 -movetime 1000    # ~real ballpark
//	./build/gauntlet -games 60 -movetime 2500    # tighter, closer to CCRL Blitz
//	./build/gauntlet -only v7.4.0 -ngn opponents/blunder_740   # calibration sanity
//
// CAVEAT printed on every run: this is a movetime proxy for CCRL Blitz (2'+1");
// it under-times the anchors vs their rating-list TC, biasing NGN's number
// optimistic by ~30-80 ELO. A true CCRL-conditions run needs NGN to play a real
// clock, currently blocked by the tournament time-management bug (top TODO).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ehrlich-b/ngn/internal/history"
	"github.com/ehrlich-b/ngn/internal/rating"
	"github.com/ehrlich-b/ngn/internal/uci"
)

// Anchor is one rung of the CCRL-rated ladder (from opponents/ratings.json).
type Anchor struct {
	Version string `json:"version"`
	Path    string `json:"path"`
	CCRL    int    `json:"ccrl"`
	Source  string `json:"source"`
}

// anchorTally is one anchor's W-D-L in a persisted gauntlet record.
type anchorTally struct {
	Version string `json:"version"`
	CCRL    int    `json:"ccrl"`
	W       int    `json:"w"`
	D       int    `json:"d"`
	L       int    `json:"l"`
}

// gauntletRecord is one JSONL row (mirrors smoke's runRecord; ts+sha drive the
// commit-decayed rolling trend).
type gauntletRecord struct {
	Timestamp  time.Time     `json:"ts"`
	SHA        string        `json:"sha"`
	MovetimeMs int           `json:"movetime_ms"`
	GamesPer   int           `json:"games_per_anchor"`
	Anchors    []anchorTally `json:"anchors"`
	Pooled     float64       `json:"pooled_elo"`
	PooledLo   float64       `json:"pooled_lo"`
	PooledHi   float64       `json:"pooled_hi"`
}

// result holds one anchor's outcome after its games are played (NGN's POV).
type result struct {
	anchor  Anchor
	w, d, l int
	reasons map[string]int
	skipped bool // excluded from pooling: opponent ignored movetime/stop (TC distorted)
}

// forfeitReasons are game-end reasons that signal a harness/process glitch rather
// than real chess; too many of these taints the run (the anti-bogus-40-0 rule).
var forfeitReasons = map[string]bool{
	"no-move": true, "illegal-move": true,
	"illegal-opening": true, "bad-opening": true, "fen-error": true,
}

// gamesPerProcess caps how many games an engine plays before the harness restarts
// it. A long-running opponent process degrades and starts flagging on time — the
// 2026-06-04 storm where Blunder 7.2.0 died at G33 / 7.4.0 at G61 and every later
// game was banked as a free NGN win (89.5% / 62%). ucinewgame (PlayGame->Reset) is
// sent each game but doesn't stop the rot, so we cycle the processes well below the
// observed onset. Set by the -restart-every flag; 0 = never (the old behavior).
var gamesPerProcess int

func main() {
	anchorsPath := flag.String("anchors", "opponents/ratings.json", "JSON file: version->path->CCRL ladder")
	ngnPath := flag.String("ngn", "build/ngn", "NGN engine binary")
	movetimeMs := flag.Int("movetime", 1000, "ms per move (equal for both sides)")
	tc := flag.String("tc", "", "real tournament clock SECONDS[+INC] (e.g. 120+1 = CCRL Blitz); overrides -movetime, drops the movetime bias")
	flag.IntVar(&gamesPerProcess, "restart-every", 1, "restart both engine processes every N games to prevent the degradation forfeit-storm; 1 = every game (default; the cost is just the ~400ms warmup probe, negligible vs a multi-hour run), 0 = never")
	games := flag.Int("games", 24, "games per anchor (even = equal colors)")
	only := flag.String("only", "", "comma-separated versions to include (default: all)")
	openingsFile := flag.String("openings", "output/sprt_openings.txt", "opening move-sequences, one per line")
	historyPath := flag.String("history", "output/gauntlet_history.jsonl", "JSONL file for rolling pooled trend")
	noRecord := flag.Bool("no-record", false, "don't append this run to history")
	rollingOnly := flag.Bool("rolling", false, "print the commit-decayed pooled trend and exit (no games)")
	probe := flag.Bool("probe", false, "probe each anchor's movetime/stop compliance and exit (no games)")
	lowPower := flag.Bool("lowpower", true, "route engines to macOS E-cores (taskpolicy -b)")
	pgnPath := flag.String("pgn", "", "capture every game (moves+result+NGN color+reason) to this file for M1 loss classification")
	concurrency := flag.Int("concurrency", 1, "parallel games (1 = serial, the validated default; >1 spawns fresh engines per game and runs all anchors' games through one worker pool — for the cloud gauntlet set this to (cores-1))")
	tallyOut := flag.String("tally-out", "", "write per-anchor W/D/L as JSON ([]anchorTally) to this file for cross-box pooling (the cloud gauntlet sums these across shards)")
	poolFiles := flag.String("pool", "", "comma-separated -tally-out JSON files to SUM per-anchor and report as one pooled rating (the cloud gauntlet's fetch step; plays no games)")
	flag.Parse()

	tcTimeMs, tcIncMs := 0, 0
	if *tc != "" {
		parts := strings.SplitN(*tc, "+", 2)
		base, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		if err != nil || base <= 0 {
			log.Fatalf("bad -tc %q (want SECONDS or SECONDS+INC, e.g. 120+1)", *tc)
		}
		tcTimeMs = int(base * 1000)
		if len(parts) == 2 {
			inc, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
			if err != nil || inc < 0 {
				log.Fatalf("bad -tc increment in %q", *tc)
			}
			tcIncMs = int(inc * 1000)
		}
	}

	if *poolFiles != "" {
		report(poolTallies(*poolFiles), *movetimeMs, *games, *historyPath, *noRecord)
		return
	}

	if *rollingOnly {
		printRolling(*historyPath, *movetimeMs)
		return
	}

	anchors, err := loadAnchors(*anchorsPath, *only)
	if err != nil {
		log.Fatalf("loading anchors: %v", err)
	}
	if len(anchors) == 0 {
		log.Fatal("no anchors selected")
	}
	if _, err := os.Stat(*ngnPath); err != nil {
		log.Fatalf("NGN binary not found: %s (build it first)", *ngnPath)
	}
	for _, a := range anchors {
		if _, err := os.Stat(a.Path); err != nil {
			log.Fatalf("anchor binary not found: %s (run scripts/build-anchors.sh)", a.Path)
		}
	}
	if *games%2 != 0 {
		fmt.Printf("WARNING: -games %d is odd; colors will be unbalanced — prefer an even count\n", *games)
	}

	if *probe {
		probeAnchors(anchors, *movetimeMs, *lowPower)
		return
	}

	openings, err := uci.LoadOpenings(*openingsFile)
	if err != nil || len(openings) == 0 {
		fmt.Printf("WARN: openings %q unavailable (%v); using built-in set\n", *openingsFile, err)
		openings, _ = uci.LoadOpenings("")
	}

	fmt.Println("NGN CCRL-anchored gauntlet")
	fmt.Println("==========================")
	if tcTimeMs > 0 {
		fmt.Printf("ngn=%s | %d anchors | %d games/anchor | real clock %.0fs+%.1fs | concurrency %d%s\n",
			*ngnPath, len(anchors), *games, float64(tcTimeMs)/1000, float64(tcIncMs)/1000, *concurrency, uci.PowerNote(*lowPower))
		fmt.Println("REAL CLOCK: true tournament TC — no movetime bias; both sides budget their own bank (flag-outs counted).")
	} else {
		fmt.Printf("ngn=%s | %d anchors | %d games/anchor | movetime %dms | concurrency %d%s\n",
			*ngnPath, len(anchors), *games, *movetimeMs, *concurrency, uci.PowerNote(*lowPower))
		fmt.Println("CAVEAT: movetime proxy for CCRL Blitz (2'+1\"); under-times anchors -> NGN biased ~+30-80 optimistic.")
	}
	fmt.Println()

	var pgnW io.Writer
	if *pgnPath != "" {
		f, err := os.Create(*pgnPath)
		if err != nil {
			log.Fatalf("cannot create -pgn file %s: %v", *pgnPath, err)
		}
		defer f.Close()
		pgnW = f
		fmt.Printf("capturing all games to %s\n\n", *pgnPath)
	}

	var results []result
	if *concurrency > 1 {
		results = playConcurrent(*ngnPath, anchors, openings, *games, *movetimeMs, tcTimeMs, tcIncMs, *concurrency, *lowPower, pgnW)
	} else {
		for _, a := range anchors {
			fmt.Printf("== vs Blunder %s (CCRL %d) ==\n", a.Version, a.CCRL)
			results = append(results, playAnchor(*ngnPath, a, openings, *games, *movetimeMs, tcTimeMs, tcIncMs, *lowPower, pgnW))
			fmt.Println()
		}
	}

	report(results, *movetimeMs, *games, *historyPath, *noRecord)
	if *tallyOut != "" {
		if err := writeTally(*tallyOut, results); err != nil {
			fmt.Printf("WARN: could not write -tally-out %s: %v\n", *tallyOut, err)
		}
	}
}

func loadAnchors(path, only string) ([]Anchor, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var all []Anchor
	if err := json.Unmarshal(data, &all); err != nil {
		return nil, err
	}
	if only == "" {
		return all, nil
	}
	want := map[string]bool{}
	for _, v := range strings.Split(only, ",") {
		want[strings.TrimSpace(v)] = true
	}
	var out []Anchor
	for _, a := range all {
		if want[a.Version] {
			out = append(out, a)
		}
	}
	return out, nil
}

// probeAnchors checks each anchor's movetime/stop compliance at the given budget
// and prints a table, without playing games. An anchor that ignores `stop` (older
// Blunder) can't be measured at an honest equal time control, so it's flagged
// "NO (skip)" — exactly what a real run excludes.
func probeAnchors(anchors []Anchor, movetimeMs int, lowPower bool) {
	fmt.Printf("Movetime/stop compliance probe (budget %dms)%s\n", movetimeMs, uci.PowerNote(lowPower))
	fmt.Printf("%-8s %5s  %-12s %s\n", "Anchor", "CCRL", "complies", "probe move time")
	for _, a := range anchors {
		opp := uci.Start(a.Path, a.Version, lowPower)
		if opp == nil {
			fmt.Printf("%-8s %5d  %-12s (failed handshake)\n", a.Version, a.CCRL, "NO")
			continue
		}
		ok, took := uci.RespectsMovetime(opp, movetimeMs)
		verdict := "yes"
		if !ok {
			verdict = "NO (skip)"
		}
		fmt.Printf("%-8s %5d  %-12s %s\n", a.Version, a.CCRL, verdict, took.Round(time.Millisecond))
		uci.Stop(opp)
	}
}

// playAnchor plays `games` games NGN-vs-anchor at equal movetime, alternating
// colors in reversed pairs over the opening list. Counts are from NGN's POV. It
// first probes the anchor for movetime/stop compliance and skips it (no games) if
// the anchor ignores `stop`, since the time control would be distorted.
func playAnchor(ngnPath string, a Anchor, openings [][]string, games, movetimeMs, tcTimeMs, tcIncMs int, lowPower bool, pgnW io.Writer) result {
	ngn := uci.Start(ngnPath, "ngn", lowPower)
	opp := uci.Start(a.Path, a.Version, lowPower)
	if ngn == nil || opp == nil {
		log.Fatalf("engine failed UCI handshake (ngn ok=%v, %s ok=%v); aborting instead of forfeiting every game",
			ngn != nil, a.Version, opp != nil)
	}
	// Stop whichever pair is live at return (the loop reassigns ngn/opp on restart),
	// so capture the variables in a closure rather than the initial pointers by value.
	defer func() { uci.Stop(ngn); uci.Stop(opp) }()

	res := result{anchor: a, reasons: map[string]int{}}
	cfg := uci.GameConfig{Movetime: movetimeMs, MaxMoves: 200}
	if tcTimeMs > 0 {
		// Real-clock mode: the anchor plays a standard UCI clock, so there's no
		// movetime/stop probe to run; a side that burns its bank loses "time-forfeit".
		cfg = uci.GameConfig{TCTimeMs: tcTimeMs, TCIncMs: tcIncMs, MaxMoves: 200}
	} else if ok, took := uci.RespectsMovetime(opp, movetimeMs); !ok {
		fmt.Printf("  SKIP %s: ignores movetime/stop (probe move %s vs %dms budget) — TC would be distorted; excluded.\n",
			a.Version, took.Round(time.Millisecond), movetimeMs)
		res.skipped = true
		return res
	}
	uci.ResetCounters(opp) // drop the warmup+probe moves; only rated games feed the guard
	uci.ResetCounters(ngn)
	var gameMoves []string
	if pgnW != nil {
		cfg.MovesOut = &gameMoves // PlayGame overwrites this each game
	}
	consecutiveForfeits := 0
	consecutiveTimeForfeits := 0
	for g := 0; g < games; g++ {
		// Restart both engines every gamesPerProcess games so neither can degrade into
		// the time-forfeit storm (see gamesPerProcess). ucinewgame alone doesn't stop it.
		if g > 0 && gamesPerProcess > 0 && g%gamesPerProcess == 0 {
			uci.Stop(ngn)
			uci.Stop(opp)
			ngn = uci.Start(ngnPath, "ngn", lowPower)
			opp = uci.Start(a.Path, a.Version, lowPower)
			if ngn == nil || opp == nil { // one retry before giving up the anchor
				uci.Stop(ngn)
				uci.Stop(opp)
				ngn = uci.Start(ngnPath, "ngn", lowPower)
				opp = uci.Start(a.Path, a.Version, lowPower)
			}
			if ngn == nil || opp == nil {
				fmt.Printf("  ABORT %s: engine restart failed at game %d — excluding anchor.\n", a.Version, g)
				res.skipped = true
				break
			}
			uci.ResetCounters(ngn)
			uci.ResetCounters(opp)
		}
		op := openings[(g/2)%len(openings)] // reversed-color pairs of the same line
		newIsWhite := g%2 == 0
		// uci.PlayGame returns the result from the FIRST engine's (NGN's) POV.
		gr := uci.PlayGame(ngn, opp, op, newIsWhite, cfg)
		if pgnW != nil {
			rc := "D"
			switch gr.Res {
			case uci.Win:
				rc = "W"
			case uci.Loss:
				rc = "L"
			}
			color := "b"
			if newIsWhite {
				color = "w"
			}
			fmt.Fprintf(pgnW, "GAME %s %s %s %s | %s\n", a.Version, rc, color, gr.Reason, strings.Join(gameMoves, " "))
		}
		res.reasons[gr.Reason]++
		switch gr.Res {
		case uci.Win:
			res.w++
		case uci.Draw:
			res.d++
		case uci.Loss:
			res.l++
		}
		n := res.w + res.d + res.l
		fmt.Printf("  G%-3d %2dW %2dD %2dL  %5.1f%%  %-12s\n",
			n, res.w, res.d, res.l, 100*rating.Score(res.w, res.d)/float64(n), gr.Reason)
		// Dead-opponent guard: a process that died/wedged mid-run returns no move
		// every game (a phantom forfeit-win that bypasses the overshoot counter, since
		// the move never arrives). Abort after a few consecutive forfeits instead of
		// grinding the rest at the full GetMove timeout — the v8.0.0 "turbo broken" run.
		if forfeitReasons[gr.Reason] {
			consecutiveForfeits++
		} else {
			consecutiveForfeits = 0
		}
		// time-forfeit isn't a forfeitReason (an occasional real flag is legitimate and
		// must not taint the run), but a long RUN of them is the degradation storm — a
		// backstop in case a process rots faster than the gamesPerProcess restart cadence.
		if gr.Reason == "time-forfeit" {
			consecutiveTimeForfeits++
		} else {
			consecutiveTimeForfeits = 0
		}
		if consecutiveForfeits >= 3 || consecutiveTimeForfeits >= 6 {
			fmt.Printf("  ABORT %s: %d consec forfeits / %d consec time-forfeits (%s) — opponent dead/degraded; excluding anchor.\n",
				a.Version, consecutiveForfeits, consecutiveTimeForfeits, gr.Reason)
			res.skipped = true
			break
		}
		// In-play guard: if the opponent starts ignoring `stop` mid-run (e.g. under
		// E-core contention) it slipped past the pre-flight probe — bail rather than
		// grind 20+ distorted games (the failure that wasted a whole overnight run).
		if tcTimeMs == 0 && opp.Moves >= 20 && float64(opp.Overshoots) > 0.10*float64(opp.Moves) {
			fmt.Printf("  ABORT %s: opponent overshot %d/%d moves (>10%%) — TC distorted; excluding anchor.\n",
				a.Version, opp.Overshoots, opp.Moves)
			res.skipped = true
			break
		}
	}
	return res
}

// spec is one game to play in the concurrent gauntlet: which anchor (index into
// the anchors slice) and the game number (drives color + opening), so a worker
// can run it standalone.
type spec struct {
	anchorIdx, g int
}

// gameMsg routes one finished game back to the collector.
type gameMsg struct {
	anchorIdx int
	res       uci.GameResult
	pgn       string // pre-formatted PGN line (empty if -pgn disabled)
}

// playConcurrent runs every (anchor, game) pair through ONE worker pool of
// `concurrency` workers — each spawns a FRESH NGN+anchor pair per game
// (restart-every=1 semantics, so the degradation forfeit-storm can't accumulate
// and the serial path's consecutive-forfeit abort is unnecessary; report()'s 2%
// forfeit ceiling still drops a dead anchor from the pool). One game ≈ one busy
// core (both engines are single-threaded and turn-based), so set concurrency to
// (cores-1) to leave a core for the OS + driver. Returns one result per anchor,
// identical in shape to serial playAnchor, so report() is unchanged. Anchors that
// fail pre-flight (handshake, or movetime non-compliance in movetime mode) come
// back skipped.
func playConcurrent(ngnPath string, anchors []Anchor, openings [][]string, games, movetimeMs, tcTimeMs, tcIncMs, concurrency int, lowPower bool, pgnW io.Writer) []result {
	results := make([]result, len(anchors))
	for i, a := range anchors {
		results[i] = result{anchor: a, reasons: map[string]int{}}
	}

	// Pre-flight each anchor once (serial, cheap): a dead binary or a
	// movetime-ignoring anchor is excluded before we commit its games.
	var specs []spec
	for i, a := range anchors {
		ngn := uci.Start(ngnPath, "ngn", lowPower)
		opp := uci.Start(a.Path, a.Version, lowPower)
		ok := ngn != nil && opp != nil
		if ok && tcTimeMs == 0 {
			if respects, took := uci.RespectsMovetime(opp, movetimeMs); !respects {
				fmt.Printf("  SKIP %s: ignores movetime/stop (probe %s vs %dms) — excluded.\n",
					a.Version, took.Round(time.Millisecond), movetimeMs)
				ok = false
			}
		}
		uci.Stop(ngn)
		uci.Stop(opp)
		if !ok {
			results[i].skipped = true
			continue
		}
		for g := 0; g < games; g++ {
			specs = append(specs, spec{anchorIdx: i, g: g})
		}
	}
	if len(specs) == 0 {
		return results
	}

	baseCfg := uci.GameConfig{Movetime: movetimeMs, MaxMoves: 200}
	if tcTimeMs > 0 {
		baseCfg = uci.GameConfig{TCTimeMs: tcTimeMs, TCIncMs: tcIncMs, MaxMoves: 200}
	}

	specCh := make(chan spec, len(specs))
	for _, s := range specs {
		specCh <- s
	}
	close(specCh)
	msgs := make(chan gameMsg, concurrency)

	var wg sync.WaitGroup
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for s := range specCh {
				a := anchors[s.anchorIdx]
				ngn := uci.Start(ngnPath, "ngn", lowPower)
				opp := uci.Start(a.Path, a.Version, lowPower)
				if ngn == nil || opp == nil { // one retry, then count as a forfeit
					uci.Stop(ngn)
					uci.Stop(opp)
					ngn = uci.Start(ngnPath, "ngn", lowPower)
					opp = uci.Start(a.Path, a.Version, lowPower)
				}
				if ngn == nil || opp == nil {
					uci.Stop(ngn)
					uci.Stop(opp)
					msgs <- gameMsg{anchorIdx: s.anchorIdx, res: uci.GameResult{Res: uci.Draw, Reason: "no-move"}}
					continue
				}
				op := openings[(s.g/2)%len(openings)] // reversed-color pairs of the same line
				newIsWhite := s.g%2 == 0
				cfg := baseCfg
				var moves []string
				if pgnW != nil {
					cfg.MovesOut = &moves
				}
				gr := uci.PlayGame(ngn, opp, op, newIsWhite, cfg)
				uci.Stop(ngn)
				uci.Stop(opp)
				m := gameMsg{anchorIdx: s.anchorIdx, res: gr}
				if pgnW != nil {
					rc := "D"
					switch gr.Res {
					case uci.Win:
						rc = "W"
					case uci.Loss:
						rc = "L"
					}
					color := "b"
					if newIsWhite {
						color = "w"
					}
					m.pgn = fmt.Sprintf("GAME %s %s %s %s | %s\n", a.Version, rc, color, gr.Reason, strings.Join(moves, " "))
				}
				msgs <- m
			}
		}()
	}
	go func() { wg.Wait(); close(msgs) }()

	// Single collector ⇒ no locks on results/pgnW. Prints a running tally as
	// games land (interleaved across anchors, hence the per-line anchor label).
	done, total := 0, len(specs)
	for m := range msgs {
		r := &results[m.anchorIdx]
		r.reasons[m.res.Reason]++
		switch m.res.Res {
		case uci.Win:
			r.w++
		case uci.Draw:
			r.d++
		case uci.Loss:
			r.l++
		}
		if pgnW != nil && m.pgn != "" {
			io.WriteString(pgnW, m.pgn)
		}
		done++
		n := r.w + r.d + r.l
		fmt.Printf("  [%4d/%d] %-8s %2dW %2dD %2dL  %5.1f%%  %-12s\n",
			done, total, r.anchor.Version, r.w, r.d, r.l,
			100*rating.Score(r.w, r.d)/float64(n), m.res.Reason)
	}
	return results
}

// writeTally persists per-anchor W/D/L as []anchorTally JSON so the cloud
// gauntlet can sum tallies across boxes and compute one pooled rating. Skipped
// anchors are omitted (no usable games).
func writeTally(path string, results []result) error {
	var tallies []anchorTally
	for _, r := range results {
		if r.skipped {
			continue
		}
		tallies = append(tallies, anchorTally{r.anchor.Version, r.anchor.CCRL, r.w, r.d, r.l})
	}
	data, err := json.MarshalIndent(tallies, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// poolTallies reads the comma-separated -tally-out JSON files from each cloud box
// and SUMS their per-anchor W/D/L into one []result, so report() computes a single
// pooled rating over the combined games. A missing/empty shard file is skipped
// (an evicted box contributes nothing rather than failing the pool).
func poolTallies(csv string) []result {
	sum := map[string]*result{}
	var order []string
	for _, path := range strings.Split(csv, ",") {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("WARN: pool skipping %s: %v\n", path, err)
			continue
		}
		var tallies []anchorTally
		if err := json.Unmarshal(data, &tallies); err != nil {
			fmt.Printf("WARN: pool skipping %s (bad JSON): %v\n", path, err)
			continue
		}
		for _, t := range tallies {
			r, ok := sum[t.Version]
			if !ok {
				r = &result{anchor: Anchor{Version: t.Version, CCRL: t.CCRL}, reasons: map[string]int{}}
				sum[t.Version] = r
				order = append(order, t.Version)
			}
			r.w += t.W
			r.d += t.D
			r.l += t.L
		}
	}
	var out []result
	for _, v := range order {
		out = append(out, *sum[v])
	}
	return out
}

// row is one anchor's computed display + pooling values.
type row struct {
	r            result
	score, n     float64
	perf, lo, hi float64
	weight       float64
}

// report computes per-anchor performance ratings, the inverse-variance pooled
// rating, runs the integrity checks, prints the crosstable, and persists the run.
func report(results []result, movetimeMs, gamesPer int, historyPath string, noRecord bool) {
	const c = 400.0 / math.Ln10 // logistic slope d(elo)/d(logit); var_i = c^2/(n*p*(1-p))

	var rows []row
	var sumW, sumWR float64
	above, below := false, false

	for _, r := range results {
		if r.skipped {
			fmt.Printf("EXCLUDED: %s (ignored movetime/stop — not measurable at an honest TC)\n", r.anchor.Version)
			continue
		}
		n := float64(r.w + r.d + r.l)
		if n == 0 {
			continue
		}
		f := 0
		for reason, cnt := range r.reasons {
			if forfeitReasons[reason] {
				f += cnt
			}
		}
		if float64(f)/n > 0.02 {
			// One bad anchor must not nuke the whole run: drop it from the pool and
			// keep the clean anchors (the dead/wedged-opponent case the v8.0.0 run hit).
			fmt.Printf("EXCLUDED (tainted): %s had %d/%d forfeit/error games (%v) — exceeds 2%% ceiling; dropped from pool.\n",
				r.anchor.Version, f, int(n), r.reasons)
			continue
		}
		score := rating.Score(r.w, r.d)
		perf, lo, hi := rating.PerfRating(score, n, float64(r.anchor.CCRL))
		// Pool on add-half-smoothed p so 0%/100% anchors get a large-but-finite
		// variance (~0 weight) instead of a div-by-zero or the ±800 clamp.
		ps := (score + 0.5) / (n + 1)
		weight := n * ps * (1 - ps) / (c * c) // = 1/var_i
		sumW += weight
		sumWR += weight * (float64(r.anchor.CCRL) + rating.ScoreToEloDiff(ps))
		if score/n > 0.5 {
			above = true
		} else {
			below = true
		}
		rows = append(rows, row{r, score, n, perf, lo, hi, weight})
	}

	if sumW == 0 {
		fmt.Println("No usable anchors — all were skipped (ignored movetime/stop) or had no games.")
		fmt.Println("Check `gauntlet -probe` for which versions honor `stop`; pick a bracketing set of those.")
		return
	}

	fmt.Println("== Crosstable ==")
	fmt.Printf("%-8s %5s  %3s %3s %3s  %6s  %5s  %-13s %7s\n",
		"Anchor", "CCRL", "W", "D", "L", "Score", "Perf", "95% CI", "weight")
	for _, rw := range rows {
		fmt.Printf("%-8s %5d  %3d %3d %3d  %5.1f%%  %5.0f  [%4.0f,%4.0f]  %6.1f%%\n",
			rw.r.anchor.Version, rw.r.anchor.CCRL, rw.r.w, rw.r.d, rw.r.l,
			100*rw.score/rw.n, rw.perf, rw.lo, rw.hi, 100*rw.weight/sumW)
	}
	fmt.Println("------------------------------------------------------------------------")

	pooled := sumWR / sumW
	se := math.Sqrt(1 / sumW)
	lo, hi := pooled-1.96*se, pooled+1.96*se

	if len(rows) >= 2 && !(above && below) {
		fmt.Println("NOTE: the anchors did not bracket 50% — the pooled rating is an EXTRAPOLATION.")
		fmt.Println("      Add a weaker anchor (NGN won all) or a stronger one (NGN lost all) and re-run.")
	}
	fmt.Printf("\nPOOLED NGN rating: %.0f   95%% CI [%.0f, %.0f]   (%d anchors, %d games each)\n",
		pooled, lo, hi, len(rows), gamesPer)
	fmt.Println("(CCRL-scale via Blunder anchors at movetime; ~+30-80 optimistic vs a true 2'+1\" clock run.)")

	if !noRecord {
		rec := gauntletRecord{
			Timestamp: time.Now().UTC(), SHA: history.CurrentSHA(),
			MovetimeMs: movetimeMs, GamesPer: gamesPer,
			Pooled: pooled, PooledLo: lo, PooledHi: hi,
		}
		for _, r := range results {
			if r.skipped {
				continue
			}
			rec.Anchors = append(rec.Anchors, anchorTally{r.anchor.Version, r.anchor.CCRL, r.w, r.d, r.l})
		}
		if err := history.Append(historyPath, rec); err != nil {
			fmt.Printf("WARN: could not record run: %v\n", err)
		}
	}
}

// printRolling shows the commit-decayed weighted mean of past pooled ratings at
// the given movetime, so a trend survives across commits like smoke's rolling-ELO.
func printRolling(historyPath string, movetimeMs int) {
	all, err := history.Load[gauntletRecord](historyPath)
	if err != nil {
		fmt.Printf("WARN: could not load history %s: %v\n", historyPath, err)
		return
	}
	var num, den float64
	n := 0
	fmt.Printf("== Rolling pooled CCRL rating (commit-decayed, movetime %dms) ==\n", movetimeMs)
	for _, r := range all {
		if r.MovetimeMs != movetimeMs {
			continue
		}
		w := history.RecordWeight(history.CommitDistance(r.SHA))
		if w <= 0 {
			continue
		}
		num += w * r.Pooled
		den += w
		n++
	}
	if den == 0 {
		fmt.Println("(no recent runs within decay window)")
		return
	}
	fmt.Printf("Estimated NGN rating: %.0f   (effective weight %.2f across %d runs)\n", num/den, den, n)
}
