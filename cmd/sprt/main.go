// Command sprt runs an NGN-vs-NGN self-play SPRT — the fine-grained measurement
// instrument the 5-game vs-Stockfish smoke cannot be (an identical binary swung
// 80%->20% there). It plays a new binary against a base binary over a set of
// balanced openings (each played twice, colors reversed, for variance
// reduction), detects game ends authoritatively via the engine package
// (checkmate/stalemate/threefold/50-move), and runs a Generalized SPRT on the
// trinomial score, stopping as soon as the log-likelihood ratio crosses a bound.
//
// It defaults to FIXED-DEPTH games (go depth N), not time-based. That removes
// time noise entirely and asks the question we actually care about for tuning:
// "at an equal depth budget, does the new eval/ordering pick better moves?" Use
// -movetime for a final time-control validation once a change looks good.
//
// The UCI subprocess harness and Elo math live in internal/uci and
// internal/rating (shared with cmd/smoke, cmd/elo-assess, and cmd/gauntlet).
//
//	# build a baseline to test against, then a candidate
//	cp build/ngn build/ngn_base   # baseline = current HEAD
//	# ...make a change, rebuild build/ngn...
//	./build/sprt -depth 9 -elo1 10            # is new stronger by >=10 ELO?
//	./build/sprt -genbook 500 -out book.txt   # generate openings for finer runs
//	./build/sprt -openings book.txt -depth 9 -elo1 5
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ehrlich-b/ngn/engine"
	"github.com/ehrlich-b/ngn/internal/rating"
	"github.com/ehrlich-b/ngn/internal/uci"
)

type spec struct {
	opening    []string
	newIsWhite bool
	pairID     int // the two reversed-color games of an opening share a pairID
}

// gameOutcome carries a finished game's result back with its pair identity so the
// consumer can score the pentanomial (pair-outcome) distribution.
type gameOutcome struct {
	pairID int
	res    uci.GameResult
}

// pairAcc accumulates the two games of a reversed-color opening pair.
type pairAcc struct {
	points float64 // new-POV points so far (each game 1 / 0.5 / 0)
	games  int
}

// optionFlags keeps UCI options in command-line order, including paths with spaces.
type optionFlags []string

func (o *optionFlags) String() string { return strings.Join(*o, "; ") }

func (o *optionFlags) Set(value string) error {
	name, setting, ok := strings.Cut(value, "=")
	name = strings.TrimSpace(name)
	if !ok || name == "" || strings.ContainsAny(value, "\r\n") || strings.Contains(name, " value ") {
		return fmt.Errorf("UCI option must be NAME=VALUE on one line")
	}
	*o = append(*o, "setoption name "+name+" value "+setting)
	return nil
}

func main() {
	var newOptions, baseOptions optionFlags
	flag.Var(&newOptions, "newoption", "candidate UCI NAME=VALUE (repeatable, ordered)")
	flag.Var(&baseOptions, "baseoption", "baseline UCI NAME=VALUE (repeatable, ordered)")
	newPath := flag.String("new", "./build/ngn", "candidate engine binary")
	basePath := flag.String("base", "./build/ngn_base", "baseline engine binary")
	depth := flag.Int("depth", 9, "fixed search depth per move (0 = use -movetime)")
	movetime := flag.Int("movetime", 0, "ms per move (overrides -depth when > 0)")
	tc := flag.String("tc", "", "real tournament clock SECONDS[+INC] (e.g. 60+1 = 60s + 1s/move); the I4 flag-detection instrument, concurrency 1 unless -concurrency is set explicitly")
	nodes := flag.Int("nodes", 0, "fixed node budget per move (overrides -depth; deterministic, credits ordering+eval, cool at any concurrency)")
	baseDepth := flag.Int("basedepth", 0, "base engine depth (0 = same as -depth; set lower as a positive control / depth ladder)")
	elo0 := flag.Float64("elo0", -3, "SPRT H0 elo bound (mill regime: symmetric sign test [-3,3])")
	elo1 := flag.Float64("elo1", 3, "SPRT H1 elo bound (mill regime: symmetric sign test [-3,3])")
	alpha := flag.Float64("alpha", 0.05, "SPRT type-I error")
	beta := flag.Float64("beta", 0.05, "SPRT type-II error")
	maxGames := flag.Int("maxgames", 20000, "hard cap on games (mill: high so the SPRT reaches an LLR bound, not the cap)")
	minGames := flag.Int("mingames", 200, "minimum games before the SPRT stop bound can fire")
	maxMoves := flag.Int("maxmoves", 200, "ply cap per game (then adjudicated draw)")
	openingsFile := flag.String("openings", "", "file of opening move-sequences (one per line); default = built-in set")
	concurrency := flag.Int("concurrency", 2, "parallel games")
	lowPower := flag.Bool("lowpower", true, "route engines to macOS E-cores (taskpolicy -b)")
	genBook := flag.Int("genbook", 0, "generate N balanced openings and exit (writes to -out)")
	genPlies := flag.Int("genplies", 6, "random plies per generated opening")
	balanceCp := flag.Int("balancecp", 100, "|eval| ceiling for a generated opening to count as balanced")
	balanceDepth := flag.Int("balancedepth", 8, "search depth used to score opening balance")
	outFile := flag.String("out", "", "output file for -genbook")
	seed := flag.Int64("seed", 1, "RNG seed for -genbook")
	gameTimeout := flag.Int("gametimeout", 0, "per-game wall-clock watchdog deadline in seconds (0 = auto from mode); a hung game is dumped+killed+voided so the run survives the cmd/sprt I/O deadlock")
	resignScore := flag.Int("resignscore", 0, "adjudicate a decisive game when |white-POV eval| >= this cp for -resignplies consecutive plies (0 = off); mill-throughput lever, needs an A/A before a run trusts it for a verdict")
	resignPlies := flag.Int("resignplies", 4, "consecutive searched plies required for -resignscore (>=2 = two-sided)")
	drawScore := flag.Int("drawscore", 0, "adjudicate a draw when |white-POV eval| <= this cp for -drawplies consecutive plies past -drawminplies (0 = off)")
	drawPlies := flag.Int("drawplies", 8, "consecutive searched plies required for -drawscore")
	drawMinPlies := flag.Int("drawminplies", 80, "no draw adjudication before this many plies (default 80 = move 40)")
	flag.Parse()

	if *genBook > 0 {
		runGenBook(*genBook, *genPlies, *balanceCp, *balanceDepth, *seed, *outFile)
		return
	}

	openings, err := uci.LoadOpenings(*openingsFile)
	if err != nil {
		log.Fatalf("loading openings: %v", err)
	}
	if len(openings) == 0 {
		log.Fatal("no usable openings")
	}

	for _, p := range []string{*newPath, *basePath} {
		if _, err := os.Stat(p); err != nil {
			log.Fatalf("engine binary not found: %s (build it first)", p)
		}
	}

	newDepth := *depth
	baseD := *depth
	if *baseDepth > 0 {
		baseD = *baseDepth
	}
	mode := fmt.Sprintf("depth %d", *depth)
	if baseD != *depth {
		mode = fmt.Sprintf("new depth %d vs base depth %d", *depth, baseD)
	}
	if *movetime > 0 {
		mode = fmt.Sprintf("movetime %dms", *movetime)
		if *concurrency > 1 {
			fmt.Printf("WARNING: movetime mode with concurrency %d — core contention makes timing (and thus results) load-dependent. Use -concurrency 1 for clean measurement.\n", *concurrency)
		}
	}
	if *nodes > 0 {
		mode = fmt.Sprintf("nodes %d/move", *nodes)
	}
	tcTimeMs, tcIncMs := 0, 0
	if *tc != "" {
		parts := strings.SplitN(*tc, "+", 2)
		base, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		if err != nil || base <= 0 {
			log.Fatalf("bad -tc %q (want SECONDS or SECONDS+INC, e.g. 60+1)", *tc)
		}
		tcTimeMs = int(base * 1000)
		if len(parts) == 2 {
			inc, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
			if err != nil || inc < 0 {
				log.Fatalf("bad -tc increment in %q", *tc)
			}
			tcIncMs = int(inc * 1000)
		}
		// Real-clock results are load-dependent (worse than movetime); force serial
		// UNLESS the operator explicitly opts into a concurrency for a null-tested rig.
		concExplicit := false
		flag.Visit(func(f *flag.Flag) {
			if f.Name == "concurrency" {
				concExplicit = true
			}
		})
		if concExplicit && *concurrency > 1 {
			fmt.Printf("WARNING: real-clock mode at concurrency %d — timing is load-dependent; only valid on a concurrency-null-tested rig. Watch the flag-out count.\n", *concurrency)
		} else {
			*concurrency = 1
		}
		mode = fmt.Sprintf("real clock %.0fs+%.1fs (concurrency %d)", base, float64(tcIncMs)/1000, *concurrency)
	}

	lower := math.Log(*beta / (1 - *alpha))
	upper := math.Log((1 - *beta) / *alpha)

	fmt.Printf("NGN self-play SPRT\n")
	fmt.Printf("==================\n")
	fmt.Printf("new=%s  base=%s\n", *newPath, *basePath)
	fmt.Printf("new options: %v\nbase options: %v\n", []string(newOptions), []string(baseOptions))
	fmt.Printf("mode: %s | openings: %d (x2 colors) | concurrency: %d%s\n",
		mode, len(openings), *concurrency, uci.PowerNote(*lowPower))
	fmt.Printf("H0: elo<=%.1f   H1: elo>=%.1f   (alpha=%.2f beta=%.2f -> LLR bounds [%.2f, %.2f])\n\n",
		*elo0, *elo1, *alpha, *beta, lower, upper)

	adj := uci.AdjConfig{
		ResignScore: *resignScore, ResignPlies: max(1, *resignPlies),
		DrawScore: *drawScore, DrawPlies: max(1, *drawPlies), DrawMinPlies: *drawMinPlies,
	}
	if adj.ResignScore > 0 || adj.DrawScore > 0 {
		fmt.Printf("adjudication: resign>=%dcp/%dp  draw<=%dcp/%dp>=%dp  (opt-in; A/A-gate before trusting a verdict)\n\n",
			adj.ResignScore, adj.ResignPlies, adj.DrawScore, adj.DrawPlies, adj.DrawMinPlies)
	}

	runSPRT(runConfig{
		newPath: *newPath, basePath: *basePath,
		newOptions: newOptions, baseOptions: baseOptions,
		newDepth: newDepth, baseDepth: baseD, movetime: *movetime, nodes: *nodes, maxMoves: *maxMoves,
		tcTimeMs: tcTimeMs, tcIncMs: tcIncMs, gameTimeout: *gameTimeout,
		elo0: *elo0, elo1: *elo1, lower: lower, upper: upper,
		maxGames: *maxGames, minGames: *minGames, concurrency: *concurrency, lowPower: *lowPower,
		adj: adj,
	}, openings)
}

type runConfig struct {
	newPath, basePath               string
	newOptions, baseOptions         []string
	newDepth, baseDepth, movetime   int
	nodes                           int
	tcTimeMs, tcIncMs               int
	maxMoves, gameTimeout           int
	elo0, elo1, lower, upper        float64
	maxGames, minGames, concurrency int
	lowPower                        bool
	adj                             uci.AdjConfig
}

func runSPRT(cfg runConfig, openings [][]string) {
	// Build the work list: each opening twice with reversed colors. At fixed
	// depth OR fixed nodes a repeated (opening,color) is a deterministic replay
	// carrying no new information, so cap distinct work at 2*len(openings); only
	// movetime mode (timing variance) justifies cycling beyond that.
	// Each opening is played twice with reversed colors; the two games form a PAIR
	// (shared pairID) so the consumer can score the pentanomial distribution. At
	// fixed depth/nodes a repeated (opening,color) is a deterministic replay
	// carrying no new info, so distinct work is 2*len(openings); only movetime
	// (timing variance) cycles beyond that, re-adding whole pairs under fresh IDs.
	var specs []spec
	pid := 0
	addPair := func(op []string) {
		specs = append(specs, spec{op, true, pid}, spec{op, false, pid})
		pid++
	}
	for _, op := range openings {
		addPair(op)
	}
	if cfg.movetime > 0 || cfg.tcTimeMs > 0 {
		for i := 0; len(specs) < cfg.maxGames; i++ {
			addPair(openings[i%len(openings)])
		}
	}
	total := min(len(specs), cfg.maxGames)
	specs = specs[:total]

	specCh := make(chan spec, total)
	for _, s := range specs {
		specCh <- s
	}
	close(specCh)

	gameCfg := uci.GameConfig{
		NewDepth: cfg.newDepth, BaseDepth: cfg.baseDepth,
		Movetime: cfg.movetime, Nodes: cfg.nodes, MaxMoves: cfg.maxMoves,
		TCTimeMs: cfg.tcTimeMs, TCIncMs: cfg.tcIncMs,
		Adj: cfg.adj,
	}

	results := make(chan gameOutcome, cfg.concurrency)
	stop := make(chan struct{})
	var wg sync.WaitGroup
	deadline := watchdogDeadline(cfg)
	for w := 0; w < cfg.concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			newEng := uci.StartWithOptions(cfg.newPath, "new", cfg.lowPower, cfg.newOptions)
			baseEng := uci.StartWithOptions(cfg.basePath, "base", cfg.lowPower, cfg.baseOptions)
			if newEng == nil || baseEng == nil {
				log.Fatalf("engine failed UCI handshake after retries (new ok=%v, base ok=%v); aborting run instead of forfeiting every game as no-move", newEng != nil, baseEng != nil)
			}
			// Closure (not `defer uci.Stop(newEng)`) so that if a wedge forces an engine
			// RESTART below, the fresh handles are the ones stopped at exit.
			defer func() { uci.Stop(newEng); uci.Stop(baseEng) }()
			for s := range specCh {
				select {
				case <-stop:
					return
				default:
				}
				res, ok := playWatch(newEng, baseEng, s.opening, s.newIsWhite, gameCfg, deadline)
				if !ok {
					// The watchdog killed both engines to break a wedge; bring up fresh
					// ones and void this game — the run survives on the remaining games.
					newEng = uci.StartWithOptions(cfg.newPath, "new", cfg.lowPower, cfg.newOptions)
					baseEng = uci.StartWithOptions(cfg.basePath, "base", cfg.lowPower, cfg.baseOptions)
					if newEng == nil || baseEng == nil {
						log.Printf("WATCHDOG: engine restart failed after a wedge; this worker exits, others continue")
						return
					}
					continue
				}
				results <- gameOutcome{pairID: s.pairID, res: res}
			}
		}()
	}
	go func() { wg.Wait(); close(results) }()

	var w, d, l int
	var newFlags, baseFlags int    // real-clock flag-outs (time-forfeit) by side
	var adjWins, adjDraws int      // games ended early by score adjudication
	var penta [5]int               // pair-outcome counts; idx = 2*(pair points), 0..4
	pairProg := map[int]*pairAcc{} // pairID -> the pair's first game, awaiting its partner
	start := time.Now()
	stopped := false
	verdict := "INCONCLUSIVE (ran out of games — add openings or raise -maxgames)"
	for ro := range results {
		r := ro.res
		switch r.Res {
		case uci.Win:
			w++
		case uci.Draw:
			d++
		case uci.Loss:
			l++
		}
		if r.Reason == "time-forfeit" {
			if r.Res == uci.Loss {
				newFlags++
			} else {
				baseFlags++
			}
		}
		switch r.Reason {
		case "adj-win":
			adjWins++
		case "adj-draw":
			adjDraws++
		}
		// Pentanomial: accumulate the two games of a pair, then bucket the pair.
		pa := pairProg[ro.pairID]
		if pa == nil {
			pa = &pairAcc{}
			pairProg[ro.pairID] = pa
		}
		pa.points += gamePoints(r.Res)
		pa.games++
		if pa.games == 2 {
			penta[int(pa.points*2+0.5)]++
			delete(pairProg, ro.pairID)
		}
		n := w + d + l
		llr := sprtLLR(w, d, l, cfg.elo0, cfg.elo1)
		est, lo, hi := rating.RelativeElo(w, d, l)
		pLLR, _, _, _, _ := pentaStats(penta, cfg.elo0, cfg.elo1)
		fmt.Printf("G%-4d %2dW %2dD %2dL  %5.1f%%  elo %+6.1f [%+.0f,%+.0f]  LLR %+6.2f  pLLR %+6.2f  %-12s\n",
			n, w, d, l, 100*rating.Score(w, d)/float64(n), est, lo, hi, llr, pLLR, r.Reason)

		// Pentanomial pLLR is the stop stat (pair-variance reduction ~= 2x fewer
		// games than the trinomial llr; both share the same alpha/beta bounds).
		if !stopped && n >= cfg.minGames && (pLLR >= cfg.upper || pLLR <= cfg.lower) {
			if pLLR >= cfg.upper {
				verdict = fmt.Sprintf("H1 ACCEPTED: new is stronger (>= %.0f ELO)", cfg.elo1)
			} else {
				verdict = fmt.Sprintf("H0 ACCEPTED: new is NOT better (<= %.0f ELO)", cfg.elo0)
			}
			close(stop)
			stopped = true
		}
	}

	w0, d0, l0 := w, d, l
	n := w0 + d0 + l0
	est, lo, hi := rating.RelativeElo(w0, d0, l0)
	fmt.Printf("\n=== RESULT (%v) ===\n", time.Since(start).Truncate(time.Second))
	fmt.Printf("Games: %d   W-D-L: %d-%d-%d   score: %.1f%%\n", n, w0, d0, l0, 100*rating.Score(w0, d0)/math.Max(1, float64(n)))
	fmt.Printf("Elo(new - base): %+.1f   95%% CI [%+.0f, %+.0f]\n", est, lo, hi)
	fmt.Printf("LLR: %+.2f   bounds [%.2f, %.2f]\n", sprtLLR(w0, d0, l0, cfg.elo0, cfg.elo1), cfg.lower, cfg.upper)
	pLLR, pairs, pEst, pLo, pHi := pentaStats(penta, cfg.elo0, cfg.elo1)
	fmt.Printf("Pentanomial [LL %d  LD %d  {LW,DD} %d  WD %d  WW %d] over %d pairs\n",
		penta[0], penta[1], penta[2], penta[3], penta[4], pairs)
	fmt.Printf("Penta Elo: %+.1f   95%% CI [%+.0f, %+.0f]   pLLR %+.2f  (THE decision stat; trinomial above is secondary)\n",
		pEst, pLo, pHi, pLLR)
	if cfg.tcTimeMs > 0 {
		fmt.Printf("Flag-outs (lost on time): new %d, base %d  of %d games  (goal: 0)\n", newFlags, baseFlags, n)
	}
	if adjWins+adjDraws > 0 {
		fmt.Printf("Adjudicated early: %d decisive, %d draw  of %d games\n", adjWins, adjDraws, n)
	}
	fmt.Printf("Verdict: %s\n", verdict)
}

// watchdogDeadline is the per-game wall-clock cap after which a game is presumed
// WEDGED (the cmd/sprt I/O deadlock seen on the cloud fleet: sprt + all engine
// processes sit at 0% CPU in a mutual read-wait and never return). It is set well
// above any legitimate game so a healthy game never trips it: ~4x the theoretical
// whole-game wall plus a minute, floored at 3 minutes. -gametimeout overrides.
func watchdogDeadline(cfg runConfig) time.Duration {
	if cfg.gameTimeout > 0 {
		return time.Duration(cfg.gameTimeout) * time.Second
	}
	var game time.Duration
	switch {
	case cfg.tcTimeMs > 0:
		perSide := cfg.tcTimeMs + cfg.tcIncMs*cfg.maxMoves/2
		game = time.Duration(2*perSide) * time.Millisecond
	case cfg.movetime > 0:
		game = time.Duration(cfg.movetime*cfg.maxMoves) * time.Millisecond
	default: // fixed depth/nodes: bounded and fast
		game = 75 * time.Second
	}
	return max(4*game+60*time.Second, 3*time.Minute)
}

// playWatch runs PlayGame under deadline. On a timeout it dumps ALL goroutine
// stacks to stderr (cloudsprt wires sprt's stderr into sprt.log -> S3, so the dump
// survives for root-causing the deadlock with no SIGQUIT needed), kills both
// engines so PlayGame's blocked Scan() returns, and reports ok=false so the worker
// voids the game and restarts engines. `done` is buffered so the abandoned
// PlayGame goroutine never leaks when it finally returns after the kill.
func playWatch(newEng, baseEng *uci.Engine, opening []string, newIsWhite bool, cfg uci.GameConfig, deadline time.Duration) (uci.GameResult, bool) {
	done := make(chan uci.GameResult, 1)
	go func() { done <- uci.PlayGame(newEng, baseEng, opening, newIsWhite, cfg) }()
	select {
	case r := <-done:
		return r, true
	case <-time.After(deadline):
		buf := make([]byte, 1<<20)
		n := runtime.Stack(buf, true)
		fmt.Fprintf(os.Stderr, "WATCHDOG: a game exceeded %v — abandoning + restarting engines (presumed I/O deadlock). Full goroutine dump:\n%s\n", deadline, buf[:n])
		uci.Stop(newEng)
		uci.Stop(baseEng)
		return uci.GameResult{Reason: "watchdog-void"}, false
	}
}

// --- SPRT statistics (Generalized SPRT on the trinomial score) ---

func eloToScore(elo float64) float64 { return 1.0 / (1.0 + math.Pow(10, -elo/400)) }

// sprtLLR is the GSPRT linear log-likelihood ratio for H0:elo0 vs H1:elo1,
// using the empirical mean and variance of the per-game score. Positive LLR
// favors H1 (new stronger), negative favors H0.
func sprtLLR(w, d, l int, elo0, elo1 float64) float64 {
	n := w + d + l
	if n == 0 {
		return 0
	}
	nf := float64(n)
	S := rating.Score(w, d)
	mean := S / nf
	// outcome second moment: win contributes 1, draw 0.25, loss 0
	sumSq := float64(w) + 0.25*float64(d)
	variance := sumSq/nf - mean*mean
	// Floor at a realistic minimum (~a very drawish match). Binds only in
	// degenerate cases (tiny n, all-identical results) where it keeps the LLR
	// from blowing up; for normal score variance (0.1-0.25) it never binds.
	variance = max(variance, 0.05)
	s0, s1 := eloToScore(elo0), eloToScore(elo1)
	return (s1 - s0) / variance * (S - nf*(s0+s1)/2.0)
}

func gamePoints(o uci.Outcome) float64 {
	switch o {
	case uci.Win:
		return 1
	case uci.Draw:
		return 0.5
	default:
		return 0
	}
}

// pentaStats computes the pentanomial (pair-outcome) LLR and Elo CI. The unit is
// the PAIR: the same opening played from both sides. Pairing exploits the negative
// correlation between the two colors of one position — the single biggest variance
// reduction over scoring games independently (the trinomial). With independent
// games it reduces EXACTLY to the trinomial LLR; when pairs are correlated (e.g. a
// balanced match splits every pair 1W-1L) its variance is far lower, so it reaches
// a bound in ~15-30% fewer games. THIS is the stop rule (promoted 2026-06-14); the
// first mill null + positive-control run certifies it before real verdicts rely on it.
//
// penta index = round(2 * pair-points): 0=LL, 1=LD/DL, 2={LW,WL,DD}, 3=WD/DW, 4=WW.
func pentaStats(penta [5]int, elo0, elo1 float64) (llr float64, pairs int, est, lo, hi float64) {
	P := 0
	for _, c := range penta {
		P += c
	}
	if P == 0 {
		return 0, 0, 0, -800, 800
	}
	Pf := float64(P)
	var S, S2 float64
	for k, c := range penta {
		x := float64(k) / 4.0 // pair score on the per-game scale: 0, .25, .5, .75, 1
		S += x * float64(c)
		S2 += x * x * float64(c)
	}
	mu := S / Pf
	variance := S2/Pf - mu*mu
	// A perfectly balanced match legitimately splits every pair 1W-1L (pair
	// variance 0), so the floor sits far below the trinomial's 0.05; it binds only
	// to stop a degenerate early LLR from exploding. The minGames gate is the other
	// guard. (Floor revisit is part of promoting penta to the stop rule.)
	variance = max(variance, 0.001)
	s0, s1 := eloToScore(elo0), eloToScore(elo1)
	// GSPRT linear LLR, same form as the trinomial but with the pair as the unit.
	llr = (s1 - s0) / variance * (S - Pf*(s0+s1)/2.0)
	se := math.Sqrt(variance / Pf)
	// ScoreToEloDiff clamps the tails, so passing mu±1.96se raw is safe.
	return llr, P, rating.ScoreToEloDiff(mu), rating.ScoreToEloDiff(mu - 1.96*se), rating.ScoreToEloDiff(mu + 1.96*se)
}

// --- opening book generation ---

func runGenBook(n, plies, balanceCp, balanceDepth int, seed int64, outPath string) {
	rng := rand.New(rand.NewSource(seed))
	seen := map[string]bool{}
	var out []string
	maxAttempts := n * 200
	for attempts := 0; len(out) < n && attempts < maxAttempts; attempts++ {
		pos, _ := engine.ParseFEN(uci.StartFEN)
		pos.Positions = make(map[uint64]int)
		pos.Positions[pos.Hash()]++
		moves := make([]string, 0, plies)
		ok := true
		for p := 0; p < plies; p++ {
			legal := engine.GenerateLegalMoves(pos)
			if len(legal) == 0 {
				ok = false
				break
			}
			m := legal[rng.Intn(len(legal))]
			pos.GameMakeMove(m)
			moves = append(moves, m.ToString())
		}
		if !ok || len(engine.GenerateLegalMoves(pos)) == 0 {
			continue
		}
		key := strings.Join(moves, " ")
		if seen[key] {
			continue
		}
		info := engine.SearchIterativeDeepening(pos, balanceDepth, nil)
		sc := info.BestScore
		if sc < 0 {
			sc = -sc
		}
		if sc > balanceCp { // unbalanced or mate-ish
			continue
		}
		seen[key] = true
		out = append(out, key)
		if len(out)%25 == 0 {
			fmt.Printf("  generated %d/%d (after %d attempts)\n", len(out), n, attempts+1)
		}
	}
	body := strings.Join(out, "\n") + "\n"
	if outPath == "" {
		fmt.Print(body)
	} else {
		if err := os.WriteFile(outPath, []byte(body), 0644); err != nil {
			log.Fatalf("writing %s: %v", outPath, err)
		}
		fmt.Printf("wrote %d balanced openings to %s\n", len(out), outPath)
	}
	if len(out) < n {
		fmt.Printf("NOTE: only %d/%d found; raise -balancecp or lower -genplies for a higher yield.\n", len(out), n)
	}
}
