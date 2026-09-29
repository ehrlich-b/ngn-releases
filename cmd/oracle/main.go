// cmd/oracle — fast deterministic engine-quality pre-filters over a position
// corpus, sitting in FRONT of the SPRT lane (kill bad changes in seconds; spend
// the multi-hour SPRT only on survivors). Modes:
//
//	score     best-move agreement vs a labeled EPD (UCI `bm`). Clean signal on
//	          TACTICAL positions (one forcing best move) but blind to quiet-search
//	          changes — a tactic is never futility-pruned, so e.g. an over-pruning
//	          change shows 0 flips here yet can be -145 ELO (use acpl for those).
//	gencorpus self-play an engine from opening lines and dump a diverse, realistic
//	          set of middlegame FENs (the distribution the engine actually reaches).
//	label     Stockfish MultiPV: record the eval of each top move per position.
//	acpl      average centipawn loss vs the SF labels — robust on QUIET positions
//	          (picking a different-but-equal move costs ~0; a real mistake costs its
//	          eval), continuous/low-variance, sees both tactical and quiet errors.
//
// None of these is a strength VERDICT — they filter and rank; the SPRT confirms.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ehrlich-b/ngn/internal/uci"
)

type oraclePos struct {
	FEN string
	BM  []string // acceptable best moves, UCI long-algebraic
	ID  string
}

// parseEPD reads "FEN bm MOVE[ MOVE...]; comment" lines (UCI moves).
func parseEPD(path string) ([]oraclePos, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []oraclePos
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, " bm ")
		if idx < 0 {
			continue
		}
		fen := strings.TrimSpace(line[:idx])
		rest := line[idx+len(" bm "):]
		bmField := rest
		id := ""
		if s := strings.Index(rest, ";"); s >= 0 {
			bmField = rest[:s]
			id = strings.TrimSpace(rest[s+1:])
		}
		bms := strings.Fields(bmField)
		if len(bms) == 0 {
			continue
		}
		out = append(out, oraclePos{FEN: fen, BM: bms, ID: id})
	}
	return out, sc.Err()
}

func isHit(mv string, bm []string) bool {
	for _, b := range bm {
		if mv == b {
			return true
		}
	}
	return false
}

// scoreEngine searches every position at the fixed budget and records the move.
// Reset() (ucinewgame) clears the TT/history between positions for order-
// independent, reproducible results.
func scoreEngine(path string, positions []oraclePos, goCmd string, timeout time.Duration, lowPower bool) (hits int, moves []string) {
	e := uci.Start(path, "oracle", lowPower)
	if e == nil {
		fmt.Fprintf(os.Stderr, "oracle: failed to start engine %q\n", path)
		os.Exit(1)
	}
	defer uci.Stop(e)
	moves = make([]string, len(positions))
	for i, p := range positions {
		uci.Reset(e)
		uci.Send(e, "position fen "+p.FEN)
		mv := uci.GetMove(e, goCmd, timeout, 0) // stopAfter=0: fixed nodes/depth self-terminates
		moves[i] = mv
		if isHit(mv, p.BM) {
			hits++
		}
	}
	return hits, moves
}

// goCmdFrom builds the fixed-budget search command (depth overrides nodes).
func goCmdFrom(nodes, depth int) (goCmd, label string) {
	if depth > 0 {
		return fmt.Sprintf("go depth %d", depth), fmt.Sprintf("depth %d", depth)
	}
	return fmt.Sprintf("go nodes %d", nodes), fmt.Sprintf("nodes %d", nodes)
}

func runScore(corpus, newBin, baseBin, goCmd, budget string, limit int, timeout time.Duration, lowPower, verbose bool) {
	positions, err := parseEPD(corpus)
	if err != nil {
		fmt.Fprintf(os.Stderr, "oracle: load corpus: %v\n", err)
		os.Exit(1)
	}
	if limit > 0 && limit < len(positions) {
		positions = positions[:limit]
	}
	if len(positions) == 0 {
		fmt.Fprintln(os.Stderr, "oracle: no labeled positions in corpus")
		os.Exit(1)
	}
	pct := func(h int) float64 { return 100 * float64(h) / float64(len(positions)) }
	fmt.Printf("oracle score: %s  (%d positions, %s)%s\n", corpus, len(positions), budget, uci.PowerNote(lowPower))

	newHits, newMoves := scoreEngine(newBin, positions, goCmd, timeout, lowPower)
	fmt.Printf("  new   %-24s %5.1f%%  (%d/%d)\n", newBin, pct(newHits), newHits, len(positions))
	if baseBin == "" {
		if verbose {
			fmt.Println("  misses (bm | got):")
			for i := range positions {
				if !isHit(newMoves[i], positions[i].BM) {
					fmt.Printf("    #%-3d %s  bm=%-6s got=%-6s  %s\n",
						i+1, positions[i].FEN, strings.Join(positions[i].BM, "/"), newMoves[i], positions[i].ID)
				}
			}
		}
		return
	}
	baseHits, baseMoves := scoreEngine(baseBin, positions, goCmd, timeout, lowPower)
	fmt.Printf("  base  %-24s %5.1f%%  (%d/%d)\n", baseBin, pct(baseHits), baseHits, len(positions))

	gained, lost := 0, 0
	for i := range positions {
		nh, bh := isHit(newMoves[i], positions[i].BM), isHit(baseMoves[i], positions[i].BM)
		switch {
		case nh && !bh:
			gained++
		case bh && !nh:
			lost++
		}
	}
	fmt.Printf("  Δ %+.1f%%  (%+d solved; +%d gained / -%d lost)\n",
		pct(newHits)-pct(baseHits), newHits-baseHits, gained, lost)

	if verbose {
		fmt.Println("  flips (bm | base -> new):")
		for i := range positions {
			nh, bh := isHit(newMoves[i], positions[i].BM), isHit(baseMoves[i], positions[i].BM)
			if nh == bh {
				continue
			}
			tag := "GAINED"
			if bh {
				tag = "LOST  "
			}
			fmt.Printf("    %s #%d  bm=%s  base=%s new=%s  %s\n",
				tag, i+1, strings.Join(positions[i].BM, "/"), baseMoves[i], newMoves[i], positions[i].ID)
		}
	}
}

func main() {
	mode := flag.String("mode", "score", "score | gencorpus | label | distill | lichess | acpl | classify")
	corpus := flag.String("corpus", "validated_tactical_positions.epd", "score: labeled EPD (UCI bm)")
	labelsF := flag.String("labels", "", "acpl: SF MultiPV label file (from -mode label)")
	fensF := flag.String("fens", "", "label: input FEN file (from -mode gencorpus)")
	newBin := flag.String("new", "build/ngn", "engine to score (also the self-play engine for gencorpus)")
	baseBin := flag.String("base", "", "second binary for A/B (score, acpl)")
	nodes := flag.Int("nodes", 200000, "fixed node budget per position (score, acpl, gencorpus)")
	depth := flag.Int("depth", 0, "fixed depth per position (overrides -nodes when >0)")
	limit := flag.Int("n", 0, "score: cap positions (0 = all)")
	lowPower := flag.Bool("lowpower", true, "route engines to E-cores (taskpolicy -b)")
	verbose := flag.Bool("v", false, "score: list flipped positions")
	openings := flag.String("openings", "output/sprt_openings.txt", "gencorpus: opening lines")
	opens := flag.Int("opens", 100, "gencorpus: number of openings to play out")
	plies := flag.Int("plies", 24, "gencorpus: self-play plies past the opening")
	every := flag.Int("every", 4, "gencorpus: dump a FEN every N plies")
	sfPath := flag.String("sf", "stockfish", "label/distill: stockfish path")
	multipv := flag.Int("multipv", 8, "label: SF MultiPV width")
	sfDepth := flag.Int("sfdepth", 16, "label/distill: SF search depth")
	workers := flag.Int("workers", 6, "distill: parallel Stockfish processes")
	cpScale := flag.Float64("cpscale", 400.0, "distill/lichess: cp->win-prob logistic scale")
	mateCp := flag.Int("matecp", 10000, "lichess: cp magnitude a mate score maps to")
	lQuiet := flag.Bool("quiet", true, "lichess: keep only quiet positions (not in check, best move not a capture)")
	out := flag.String("out", "", "gencorpus/label/distill/lichess: output file")
	capCp := flag.Int("cap", 1000, "acpl: per-position cp-loss cap")
	acplConc := flag.Int("concurrency", 1, "acpl: parallel engine processes (deterministic; fixed-node, so result == serial)")
	gamesF := flag.String("games", "", "classify: gauntlet -pgn capture file (M1 loss classification)")
	flag.Parse()

	goCmd, budget := goCmdFrom(*nodes, *depth)
	timeout := 60 * time.Second

	switch *mode {
	case "score":
		runScore(*corpus, *newBin, *baseBin, goCmd, budget, *limit, timeout, *lowPower, *verbose)
	case "gencorpus":
		o := *out
		if o == "" {
			o = "output/oracle_corpus.fens"
		}
		runGenCorpus(*newBin, *openings, o, *opens, *plies, *every, goCmd, timeout, *lowPower)
	case "label":
		if *fensF == "" || *out == "" {
			fmt.Fprintln(os.Stderr, "label: need -fens <input> and -out <output>")
			os.Exit(1)
		}
		runLabel(*sfPath, *fensF, *out, *multipv, *sfDepth, *lowPower)
	case "distill":
		if *fensF == "" || *out == "" {
			fmt.Fprintln(os.Stderr, "distill: need -fens <input FENs/dataset> and -out <output>")
			os.Exit(1)
		}
		runDistill(*sfPath, *fensF, *out, *sfDepth, *workers, *cpScale, *lowPower)
	case "lichess":
		if *fensF == "" || *out == "" {
			fmt.Fprintln(os.Stderr, "lichess: need -fens <lichess eval jsonl> and -out <output>")
			os.Exit(1)
		}
		runLichess(*fensF, *out, *cpScale, *mateCp, *lQuiet)
	case "acpl":
		if *labelsF == "" {
			fmt.Fprintln(os.Stderr, "acpl: need -labels <file>")
			os.Exit(1)
		}
		runACPL(*newBin, *baseBin, *labelsF, goCmd, budget, timeout, *lowPower, *capCp, *acplConc)
	case "classify":
		if *gamesF == "" {
			fmt.Fprintln(os.Stderr, "classify: need -games <gauntlet -pgn file>")
			os.Exit(1)
		}
		runClassify(*gamesF, *sfPath, *sfDepth, *lowPower)
	default:
		fmt.Fprintf(os.Stderr, "unknown -mode %q (want score|gencorpus|label|acpl|classify)\n", *mode)
		os.Exit(1)
	}
}
