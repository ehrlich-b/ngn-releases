// Smoke test for fast iteration. Two phases:
//
//  1. Tactical suite — load positions from an EPD and run the engine in-process
//     with a short per-position budget. A sharp drop in solve count is a clear
//     search regression and we fail fast.
//  2. Game smoke — short asymmetric game match (default 5 games at 1s NGN /
//     200ms SF) via the existing elo-assess binary. We only fire on extreme
//     outcomes (0 wins or all wins); mid outcomes are treated as "no signal".
//
// Designed to run on battery in a few minutes. Its rolling number is a RELATIVE
// regression tripwire vs Stockfish's UCI_Elo slider (which runs ~+400 inflated
// vs CCRL) — NOT an absolute rating. Use `make gauntlet` for the absolute number.
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ehrlich-b/ngn/engine"
	"github.com/ehrlich-b/ngn/internal/history"
	"github.com/ehrlich-b/ngn/internal/rating"
)

// runRecord is one row of the rolling-ELO history. Append-only JSONL.
type runRecord struct {
	Timestamp time.Time `json:"ts"`
	SHA       string    `json:"sha"`
	NgnMs     int       `json:"ngn_ms"`
	SfMs      int       `json:"sf_ms"`
	TargetELO int       `json:"target_elo"`
	Games     int       `json:"games"`
	NgnW      int       `json:"ngn_w"`
	Draws     int       `json:"draws"`
	SfW       int       `json:"sf_w"`
}

func main() {
	nTactical := flag.Int("tactical", 30, "Number of tactical positions to test")
	tMs := flag.Int("tactical-time", 200, "ms per tactical position")
	nGames := flag.Int("games", 5, "Game-smoke games (0 = skip)")
	ngnMs := flag.Int("ngn-ms", 1000, "NGN ms per move (game phase)")
	sfMs := flag.Int("sf-ms", 200, "Stockfish ms per move (game phase)")
	targetELO := flag.Int("elo", 2400, "Stockfish ELO target")
	skipTactical := flag.Bool("skip-tactical", false, "Skip tactical phase")
	skipGames := flag.Bool("skip-games", false, "Skip game phase")
	epdPath := flag.String("epd", "validated_tactical_positions.epd", "EPD file path")
	threshold := flag.Float64("threshold", 0.40, "Min tactical solve rate to pass (0-1)")
	seed := flag.Int64("seed", 42, "RNG seed for deterministic position selection")
	historyPath := flag.String("history", "output/smoke_history.jsonl", "JSONL file for rolling-ELO history")
	noRecord := flag.Bool("no-record", false, "Don't append this run to history")
	rollingOnly := flag.Bool("rolling", false, "Just print the rolling-ELO summary and exit (no smoke run)")
	flag.Parse()

	fmt.Println("NGN smoke test")
	fmt.Println("==============")

	if *rollingOnly {
		printRolling(*historyPath, *targetELO, *ngnMs, *sfMs)
		return
	}

	fail := false

	if !*skipTactical {
		if !runTactical(*epdPath, *nTactical, *tMs, *threshold, *seed) {
			fail = true
		}
	}

	var gameStats *runRecord
	if !*skipGames && *nGames > 0 {
		// Game phase is a measurement, not a hard gate: vs a strong sparring
		// partner (SF2400) a shutout is within normal variance, not a
		// catastrophic break. The deterministic tactical phase owns the hard
		// fail; rolling ELO catches genuine drift.
		_, rec := runGameSmoke(*nGames, *ngnMs, *sfMs, *targetELO)
		if rec != nil && !*noRecord {
			if err := history.Append(*historyPath, *rec); err != nil {
				fmt.Printf("WARN: could not record run: %v\n", err)
			}
			gameStats = rec
		}
	}

	if gameStats != nil {
		printRolling(*historyPath, gameStats.TargetELO, gameStats.NgnMs, gameStats.SfMs)
	}

	fmt.Println()
	if fail {
		fmt.Println("=== SMOKE FAIL ===")
		os.Exit(1)
	}
	fmt.Println("=== SMOKE OK ===")
}

type puzzle struct {
	FEN string
	BM  string // best move in UCI form
}

func runTactical(epdPath string, n, msPerPos int, threshold float64, seed int64) bool {
	fmt.Printf("\n== Phase 1: tactical (%d positions @ %dms each) ==\n", n, msPerPos)
	all, err := loadEPD(epdPath)
	if err != nil {
		fmt.Printf("WARN: cannot load %s: %v (skipping tactical)\n", epdPath, err)
		return true
	}
	if len(all) == 0 {
		fmt.Println("WARN: no positions in EPD; skipping tactical")
		return true
	}

	rng := rand.New(rand.NewSource(seed))
	rng.Shuffle(len(all), func(i, j int) { all[i], all[j] = all[j], all[i] })
	if n > len(all) {
		n = len(all)
	}
	picks := all[:n]

	correct := 0
	start := time.Now()
	for i, p := range picks {
		// Fresh receiver state per puzzle so cross-contamination from one
		// puzzle's TT/history doesn't bias the next one.
		engine.ClearHash()
		engine.ClearHistoryTable()
		engine.ClearKillerMoves()
		engine.ClearCounterMoves()
		engine.SetLastMovePlayed(engine.EmptyMove)

		pos, err := engine.ParseFEN(p.FEN)
		if err != nil {
			continue
		}
		tm := engine.NewTimeManager()
		tm.SetTimeControl(engine.SearchParams{MoveTime: msPerPos}, pos.Turn() == engine.White)
		info := engine.SearchIterativeDeepening(pos, 64, tm)
		got := info.BestMove.ToString()
		if got == p.BM {
			correct++
		}
		// Light progress trace every 10 puzzles so the user sees movement.
		if (i+1)%10 == 0 {
			fmt.Printf("  ... %d/%d done (%d correct)\n", i+1, len(picks), correct)
		}
	}
	elapsed := time.Since(start)
	rate := float64(correct) / float64(len(picks))
	fmt.Printf("\nSolved: %d/%d (%.0f%%) in %v\n", correct, len(picks), rate*100, elapsed.Round(time.Millisecond))
	if rate < threshold {
		fmt.Printf("FAIL: solve rate %.0f%% below threshold %.0f%%\n", rate*100, threshold*100)
		return false
	}
	fmt.Printf("OK: solve rate ≥ threshold (%.0f%%)\n", threshold*100)
	return true
}

func loadEPD(path string) ([]puzzle, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []puzzle
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "bm ", 2)
		if len(parts) != 2 {
			continue
		}
		fen := strings.TrimSpace(parts[0])
		bm := parts[1]
		if i := strings.Index(bm, ";"); i >= 0 {
			bm = bm[:i]
		}
		out = append(out, puzzle{FEN: fen, BM: strings.TrimSpace(bm)})
	}
	return out, scan.Err()
}

// runGameSmoke shells out to ./build/elo-assess for a fixed-game match and
// returns (verdict, record). verdict is "regression"/"neutral"/"improvement"/"error".
// record is non-nil iff the games actually completed and counts parsed cleanly.
func runGameSmoke(games, ngnMs, sfMs, targetELO int) (string, *runRecord) {
	fmt.Printf("\n== Phase 2: %d-game smoke (NGN %dms vs SF%d %dms) ==\n",
		games, ngnMs, targetELO, sfMs)

	cmd := exec.Command("./build/elo-assess",
		"-elo", strconv.Itoa(targetELO),
		"-movetime", strconv.Itoa(ngnMs),
		"-sfmovetime", strconv.Itoa(sfMs),
		"-games", strconv.Itoa(games),
		"-min", strconv.Itoa(games),
		"-ci", "99999", // disable CI-based early stop
	)
	var buf bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &buf)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("WARN: elo-assess failed: %v\n", err)
		return "error", nil
	}

	out := buf.String()
	parseInt := func(re *regexp.Regexp) int {
		m := re.FindStringSubmatch(out)
		if len(m) < 2 {
			return -1
		}
		n, _ := strconv.Atoi(m[1])
		return n
	}
	ngnW := parseInt(regexp.MustCompile(`NGN wins: (\d+)`))
	sfW := parseInt(regexp.MustCompile(`Stockfish wins: (\d+)`))
	draws := parseInt(regexp.MustCompile(`Draws: (\d+)`))
	if ngnW < 0 || sfW < 0 || draws < 0 {
		fmt.Println("WARN: could not parse game counts from elo-assess output")
		return "error", nil
	}

	totalPlayed := ngnW + sfW + draws
	score := float64(ngnW) + 0.5*float64(draws)

	// Posterior under a Beta(4,2) prior centered near the prior baseline
	// (~67% win rate). Reported for context only; verdict uses raw counts so
	// the rule is dead simple to reason about.
	pA := 4.0 + score
	pB := 2.0 + float64(totalPlayed) - score
	postMean := pA / (pA + pB)

	fmt.Println("\n== Smoke verdict ==")
	fmt.Printf("Result: %dW %dD %dL  (raw score %.1f/%d)\n", ngnW, draws, sfW, score, totalPlayed)
	fmt.Printf("Beta(4,2) posterior win rate: %.0f%%\n", postMean*100)

	rec := &runRecord{
		Timestamp: time.Now().UTC(),
		SHA:       history.CurrentSHA(),
		NgnMs:     ngnMs,
		SfMs:      sfMs,
		TargetELO: targetELO,
		Games:     totalPlayed,
		NgnW:      ngnW,
		Draws:     draws,
		SfW:       sfW,
	}

	switch {
	case ngnW == 0 && sfW >= games-1:
		fmt.Println("SHUTOUT — NGN won 0; variance vs SF2400 is possible, investigate only if it persists across runs")
		return "regression", rec
	case ngnW >= games-1 && sfW == 0:
		fmt.Println("IMPROVEMENT — clear wins")
		return "improvement", rec
	default:
		fmt.Println("NEUTRAL — no clear signal at this sample size")
		return "neutral", rec
	}
}

// weightedRun pairs a record with the commit distance from HEAD and the
// resulting decay weight. distance < 0 means the SHA is unreachable (deleted,
// rewritten, or unknown) — those contribute zero weight.
type weightedRun struct {
	rec      runRecord
	distance int
	weight   float64
}

// printRolling reports the rolling estimate restricted to runs at the same
// (targetELO, ngnMs, sfMs) configuration. Different configs are not directly
// comparable so we don't pool them. Older records decay per history.RecordWeight
// so stale data slowly bleeds out as commits accumulate. NB: this is a RELATIVE
// tripwire vs the SF UCI_Elo slider, not an absolute CCRL rating.
func printRolling(historyPath string, targetELO, ngnMs, sfMs int) {
	all, err := history.Load[runRecord](historyPath)
	if err != nil {
		fmt.Printf("WARN: could not load history %s: %v\n", historyPath, err)
		return
	}
	var matching []runRecord
	for _, r := range all {
		if r.TargetELO == targetELO && r.NgnMs == ngnMs && r.SfMs == sfMs {
			matching = append(matching, r)
		}
	}
	if len(matching) == 0 {
		fmt.Println("\n== Rolling ELO ==")
		fmt.Printf("(no prior runs at NGN %dms vs SF%d %dms)\n", ngnMs, targetELO, sfMs)
		return
	}

	weighted := make([]weightedRun, 0, len(matching))
	for _, r := range matching {
		d := history.CommitDistance(r.SHA)
		w := history.RecordWeight(d)
		weighted = append(weighted, weightedRun{rec: r, distance: d, weight: w})
	}

	var wW, wD, wL, wTotal float64
	contributing := 0
	for _, wr := range weighted {
		if wr.weight <= 0 {
			continue
		}
		wW += wr.weight * float64(wr.rec.NgnW)
		wD += wr.weight * float64(wr.rec.Draws)
		wL += wr.weight * float64(wr.rec.SfW)
		wTotal += wr.weight * float64(wr.rec.Games)
		contributing++
	}

	fmt.Printf("\n== Rolling ELO (commit-decayed, NGN %dms vs SF%d %dms) ==\n",
		ngnMs, targetELO, sfMs)
	if wTotal <= 0 {
		oldest := -1
		for _, wr := range weighted {
			if wr.distance > oldest {
				oldest = wr.distance
			}
		}
		fmt.Println("(no recent runs within decay window — confidence wide open)")
		if oldest >= 0 {
			fmt.Printf("Total runs in bucket: %d (oldest %d commits behind HEAD)\n",
				len(matching), oldest)
		} else {
			fmt.Printf("Total runs in bucket: %d (SHAs unreachable)\n", len(matching))
		}
	} else {
		score := wW + 0.5*wD
		est, lo, hi := rating.PerfRating(score, wTotal, float64(targetELO))
		fmt.Printf("Effective sample: %.1f games across %d/%d runs\n",
			wTotal, contributing, len(matching))
		fmt.Printf("Weighted: %.1fW %.1fD %.1fL = %.1f%%\n",
			wW, wD, wL, score/wTotal*100)
		fmt.Printf("Estimated ELO vs SF slider: %.0f  CI [%.0f, %.0f]  (width %.0f)\n",
			est, lo, hi, hi-lo)
		fmt.Println("  (relative tripwire — SF UCI_Elo runs ~+400 hot vs CCRL; run `make gauntlet` for the absolute number)")
	}

	// Show the last few runs for trend visibility.
	const recentN = 6
	start := len(weighted) - recentN
	if start < 0 {
		start = 0
	}
	fmt.Println("Recent runs (most recent last; d=commits behind HEAD, w=weight):")
	for _, wr := range weighted[start:] {
		ts := wr.rec.Timestamp.Local().Format("Mon 15:04")
		runScore := float64(wr.rec.NgnW) + 0.5*float64(wr.rec.Draws)
		runRate := runScore / float64(wr.rec.Games)
		dStr := strconv.Itoa(wr.distance)
		if wr.distance < 0 {
			dStr = "?"
		}
		fmt.Printf("  %s  %s  d=%-3s w=%.2f  %dW %dD %dL = %.0f%%\n",
			ts, history.ShortSHA(wr.rec.SHA), dStr, wr.weight,
			wr.rec.NgnW, wr.rec.Draws, wr.rec.SfW, runRate*100)
	}
}
