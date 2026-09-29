package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ehrlich-b/ngn/engine"
	"github.com/ehrlich-b/ngn/internal/uci"
)

// startPosition returns a fresh game-start Position (mirrors cmd/pgn_to_fens).
func startPosition() *engine.Position {
	return &engine.Position{
		Board: engine.StartingBoard(),
		Tag: engine.WhiteCanCastleKingSide | engine.WhiteCanCastleQueenSide |
			engine.BlackCanCastleKingSide | engine.BlackCanCastleQueenSide | engine.WhiteToMove,
		EnPassant: engine.NoSquare,
	}
}

// applyUCI parses uciMove in pos and makes it; false on illegal/parse error.
func applyUCI(pos *engine.Position, uciMove string) bool {
	mv, err := engine.ParseUCIMove(pos, uciMove)
	if err != nil {
		return false
	}
	pos.MakeMove(mv)
	return true
}

func badMove(m string) bool { return m == "" || m == "(none)" || m == "0000" }

// readLines reads non-empty, non-'#' lines.
func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		ln := strings.TrimSpace(sc.Text())
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		out = append(out, ln)
	}
	return out, sc.Err()
}

// ---------- gencorpus: self-play a diverse, realistic position set ----------

func runGenCorpus(engPath, openingsPath, outPath string, nOpenings, plies, every int, goCmd string, timeout time.Duration, lowPower bool) {
	openings, err := uci.LoadOpenings(openingsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "oracle gencorpus: load openings: %v\n", err)
		os.Exit(1)
	}
	if nOpenings > 0 && nOpenings < len(openings) {
		openings = openings[:nOpenings]
	}
	e := uci.Start(engPath, "gen", lowPower)
	if e == nil {
		fmt.Fprintf(os.Stderr, "oracle gencorpus: cannot start %q\n", engPath)
		os.Exit(1)
	}
	defer uci.Stop(e)
	out, err := os.Create(outPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "oracle gencorpus: create %s: %v\n", outPath, err)
		os.Exit(1)
	}
	defer out.Close()
	w := bufio.NewWriter(out)
	defer w.Flush()

	count := 0
	for oi, om := range openings {
		pos := startPosition()
		ok := true
		for _, m := range om {
			if !applyUCI(pos, m) {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		for p := 0; p < plies; p++ {
			if len(engine.GenerateLegalMoves(pos)) == 0 {
				break // checkmate / stalemate
			}
			uci.Send(e, "position fen "+engine.GenerateFEN(pos))
			mv := uci.GetMove(e, goCmd, timeout, 0)
			if badMove(mv) || !applyUCI(pos, mv) {
				break
			}
			if p%every == 0 {
				fmt.Fprintln(w, engine.GenerateFEN(pos))
				count++
			}
		}
		if (oi+1)%25 == 0 {
			w.Flush()
			fmt.Printf("  ... %d openings, %d positions\n", oi+1, count)
		}
	}
	fmt.Printf("gencorpus: wrote %d positions to %s\n", count, outPath)
}

// ---------- label: Stockfish MultiPV eval-per-move ----------

type moveEval struct {
	move string
	cp   int // from side-to-move's perspective; mate mapped to +-(100000-|n|)
}

// labelPosition asks SF for the eval of each of its top moves (deepest wins).
func labelPosition(sf *uci.Engine, fen string, depth int) []moveEval {
	uci.Send(sf, "position fen "+fen)
	lines := uci.Analyze(sf, fmt.Sprintf("go depth %d", depth), 60*time.Second)
	byRank := map[int]moveEval{}
	for _, ln := range lines {
		if !strings.Contains(ln, " multipv ") {
			continue
		}
		f := strings.Fields(ln)
		rank, cp, mv, haveCp := 0, 0, "", false
		for i := 0; i+1 < len(f); i++ {
			switch f[i] {
			case "multipv":
				rank, _ = strconv.Atoi(f[i+1])
			case "cp":
				cp, _ = strconv.Atoi(f[i+1])
				haveCp = true
			case "mate":
				m, _ := strconv.Atoi(f[i+1])
				sign := 1
				if m < 0 {
					sign, m = -1, -m
				}
				cp, haveCp = sign*(100000-m), true
			case "pv":
				mv = f[i+1]
			}
		}
		if rank > 0 && mv != "" && haveCp {
			byRank[rank] = moveEval{mv, cp}
		}
	}
	if len(byRank) == 0 {
		return nil
	}
	ranks := make([]int, 0, len(byRank))
	for r := range byRank {
		ranks = append(ranks, r)
	}
	sort.Ints(ranks)
	out := make([]moveEval, 0, len(ranks))
	for _, r := range ranks {
		out = append(out, byRank[r])
	}
	return out
}

func runLabel(sfPath, fensPath, outPath string, multipv, depth int, lowPower bool) {
	fens, err := readLines(fensPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "oracle label: read fens: %v\n", err)
		os.Exit(1)
	}
	sf := uci.Start(sfPath, "stockfish", lowPower)
	if sf == nil {
		fmt.Fprintf(os.Stderr, "oracle label: cannot start stockfish %q\n", sfPath)
		os.Exit(1)
	}
	defer uci.Stop(sf)
	uci.Send(sf, fmt.Sprintf("setoption name MultiPV value %d", multipv))
	uci.Send(sf, "isready")
	uci.WaitFor(sf, "readyok", 5*time.Second)

	out, err := os.Create(outPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "oracle label: create %s: %v\n", outPath, err)
		os.Exit(1)
	}
	defer out.Close()
	w := bufio.NewWriter(out)
	defer w.Flush()

	labeled := 0
	for i, fen := range fens {
		evs := labelPosition(sf, fen, depth)
		if len(evs) == 0 {
			continue
		}
		var sb strings.Builder
		sb.WriteString(fen)
		sb.WriteString(" |")
		for _, e := range evs {
			fmt.Fprintf(&sb, " %s:%d", e.move, e.cp)
		}
		fmt.Fprintln(w, sb.String())
		labeled++
		if (i+1)%50 == 0 {
			w.Flush()
			fmt.Printf("  ... labeled %d/%d\n", labeled, len(fens))
		}
	}
	fmt.Printf("label: wrote %d labeled positions to %s (multipv %d, depth %d)\n", labeled, outPath, multipv, depth)
}

// ---------- acpl: average centipawn loss vs SF labels ----------

type labeledPos struct {
	fen   string
	evals []moveEval // best-first
}

func parseLabels(path string) ([]labeledPos, error) {
	lines, err := readLines(path)
	if err != nil {
		return nil, err
	}
	var out []labeledPos
	for _, ln := range lines {
		i := strings.Index(ln, " |")
		if i < 0 {
			continue
		}
		fen := strings.TrimSpace(ln[:i])
		var evs []moveEval
		for _, tok := range strings.Fields(ln[i+2:]) {
			c := strings.LastIndex(tok, ":")
			if c < 0 {
				continue
			}
			cp, err := strconv.Atoi(tok[c+1:])
			if err != nil {
				continue
			}
			evs = append(evs, moveEval{tok[:c], cp})
		}
		if len(evs) > 0 {
			out = append(out, labeledPos{fen, evs})
		}
	}
	return out, nil
}

// acplLoss is one position's capped centipawn loss for the move mv vs the SF
// labels: bestCp minus the labeled eval of mv (floored at the worst labeled eval
// when mv is outside SF's top-N — a conservative LOWER bound), clamped to [0,cap].
func acplLoss(lp labeledPos, mv string, capCp int) float64 {
	bestCp := lp.evals[0].cp
	moveCp := lp.evals[len(lp.evals)-1].cp
	for _, me := range lp.evals {
		if me.move == mv {
			moveCp = me.cp
			break
		}
	}
	loss := float64(bestCp - moveCp)
	if loss < 0 {
		loss = 0
	}
	if loss > float64(capCp) {
		loss = float64(capCp)
	}
	return loss
}

// acplEngine returns one binary's average centipawn loss vs the SF labels.
// concurrency>1 fans the positions across that many independent engine processes;
// because each position is searched at a FIXED node/depth budget, a position's move
// (and thus its loss) is identical regardless of which worker runs it, so the mean
// is deterministic and equal to the serial result — concurrency only buys wall time.
func acplEngine(engPath string, labels []labeledPos, goCmd string, timeout time.Duration, lowPower bool, capCp, concurrency int) float64 {
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > len(labels) {
		concurrency = len(labels)
	}
	losses := make([]float64, len(labels)) // worker w writes only its own indices — no race
	jobs := make(chan int, len(labels))
	for i := range labels {
		jobs <- i
	}
	close(jobs)
	var wg sync.WaitGroup
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e := uci.Start(engPath, "acpl", lowPower)
			if e == nil {
				fmt.Fprintf(os.Stderr, "oracle acpl: cannot start %q\n", engPath)
				os.Exit(1)
			}
			defer uci.Stop(e)
			for idx := range jobs {
				lp := labels[idx]
				uci.Reset(e)
				uci.Send(e, "position fen "+lp.fen)
				mv := uci.GetMove(e, goCmd, timeout, 0)
				losses[idx] = acplLoss(lp, mv, capCp)
			}
		}()
	}
	wg.Wait()
	var total float64
	for _, l := range losses {
		total += l
	}
	return total / float64(len(labels))
}

func runACPL(newBin, baseBin, labelFile, goCmd, budget string, timeout time.Duration, lowPower bool, capCp, concurrency int) {
	labels, err := parseLabels(labelFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "oracle acpl: %v\n", err)
		os.Exit(1)
	}
	if len(labels) == 0 {
		fmt.Fprintln(os.Stderr, "oracle acpl: no labeled positions")
		os.Exit(1)
	}
	fmt.Printf("acpl: %s  (%d positions, %s, cap %dcp, conc %d)%s\n", labelFile, len(labels), budget, capCp, concurrency, uci.PowerNote(lowPower))
	newACPL := acplEngine(newBin, labels, goCmd, timeout, lowPower, capCp, concurrency)
	fmt.Printf("  new   %-24s ACPL %6.1f\n", newBin, newACPL)
	if baseBin == "" {
		return
	}
	baseACPL := acplEngine(baseBin, labels, goCmd, timeout, lowPower, capCp, concurrency)
	fmt.Printf("  base  %-24s ACPL %6.1f\n", baseBin, baseACPL)
	fmt.Printf("  Δ %+.1f cp/move  (lower ACPL = better; positive Δ means NEW gives up more eval = WORSE)\n", newACPL-baseACPL)
}

// ---------- classify: where did NGN lose? (M1 loss classification) ----------

// tacticalDropCp: an NGN move that drops SF's NGN-POV eval by >= this in ONE move
// is a sudden blunder (a missed tactic / hung material) = a SEARCH/depth failure.
// Below it, a loss with no single big drop is gradual = a POSITIONAL/eval failure.
const tacticalDropCp = 200

// egPhase: PeSTO game phase <= this (from non-pawn material) counts as endgame.
const egPhase = 6

// evalClamp bounds SF's NGN-POV eval before drop computation. A position past
// this is already decisively lost, so without the clamp the single biggest
// "drop" is always the forced-mate score (~100000cp) at the END of the game,
// masking the ORIGINATING error. Clamping surfaces the move where NGN actually
// went from ~playable to ~lost — the decisive moment we want to classify.
const evalClamp = 1500

type gameRec struct {
	anchor, result, ngnColor, reason string
	moves                            []string
}

// parseGames reads the gauntlet -pgn capture: "GAME <anchor> <R|D|L> <w|b> <reason> | m1 m2 ...".
func parseGames(path string) ([]gameRec, error) {
	lines, err := readLines(path)
	if err != nil {
		return nil, err
	}
	var out []gameRec
	for _, ln := range lines {
		if !strings.HasPrefix(ln, "GAME ") {
			continue
		}
		head, moves := ln, []string(nil)
		if i := strings.Index(ln, " | "); i >= 0 {
			moves = strings.Fields(ln[i+3:])
			head = ln[:i]
		}
		f := strings.Fields(head) // GAME anchor result color reason...
		if len(f) < 5 {
			continue
		}
		out = append(out, gameRec{f[1], f[2], f[3], strings.Join(f[4:], " "), moves})
	}
	return out, nil
}

// fenPhase is a PeSTO-style game phase from the FEN's non-pawn material
// (N/B=1, R=2, Q=4; 24=opening, 0=bare kings). evaluatePeSTO is unexported, so
// we recompute it here for the endgame tag.
func fenPhase(fen string) int {
	w := map[rune]int{'N': 1, 'B': 1, 'R': 2, 'Q': 4, 'n': 1, 'b': 1, 'r': 2, 'q': 4}
	p := 0
	for _, c := range strings.Fields(fen)[0] {
		p += w[c]
	}
	if p > 24 {
		p = 24
	}
	return p
}

// sfEvalCp returns SF's eval of fen from the side-to-move's POV (best PV's cp).
func sfEvalCp(sf *uci.Engine, fen string, depth int) (int, bool) {
	evs := labelPosition(sf, fen, depth)
	if len(evs) == 0 {
		return 0, false
	}
	return evs[0].cp, true
}

// ---------- distill: Stockfish soft labels for Texel training ----------

// runDistill labels positions with Stockfish's evaluation for knowledge
// distillation: for each position SF searches to -sfdepth, and its white-POV cp is
// mapped through the logistic 1/(1+10^(-cp/scale)) to a win-probability target,
// written as a Texel "<FEN> <target>" line (LoadTexelSamples reads these directly).
// The sigmoid de-emphasizes already-won positions — the strength-relevant choice
// over matching SF's raw cp everywhere. Labeling is offline and embarrassingly
// parallel, so it runs a pool of independent SF processes (P-cores are fine here,
// unlike the timing-sensitive engine-vs-engine path).
func runDistill(sfPath, inPath, outPath string, depth, workers int, cpScale float64, lowPower bool) {
	lines, err := readLines(inPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "oracle distill: read %s: %v\n", inPath, err)
		os.Exit(1)
	}
	// Accept either bare FENs or Texel "<FEN> <result>" lines — take the 6 FEN fields.
	fens := make([]string, 0, len(lines))
	for _, ln := range lines {
		f := strings.Fields(ln)
		if len(f) < 6 {
			continue
		}
		fens = append(fens, strings.Join(f[:6], " "))
	}
	if len(fens) == 0 {
		fmt.Fprintln(os.Stderr, "oracle distill: no positions")
		os.Exit(1)
	}
	if workers < 1 {
		workers = 1
	}

	out, err := os.Create(outPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "oracle distill: create %s: %v\n", outPath, err)
		os.Exit(1)
	}
	defer out.Close()
	w := bufio.NewWriter(out)
	defer w.Flush()

	jobs := make(chan string, workers*2)
	results := make(chan string, workers*2)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go distillWorker(sfPath, depth, cpScale, lowPower, jobs, results, &wg)
	}
	done := make(chan int)
	go func() {
		start := time.Now()
		n := 0
		for line := range results {
			fmt.Fprintln(w, line)
			n++
			if n%500 == 0 {
				w.Flush()
				rate := float64(n) / time.Since(start).Seconds()
				fmt.Fprintf(os.Stderr, "  labeled %d/%d (%.1f pos/s)\n", n, len(fens), rate)
			}
		}
		done <- n
	}()

	for _, fen := range fens {
		jobs <- fen
	}
	close(jobs)
	wg.Wait()
	close(results)
	n := <-done
	fmt.Fprintf(os.Stderr, "distill done: %d/%d positions labeled (depth %d, %d workers, scale %.0f) -> %s\n",
		n, len(fens), depth, workers, cpScale, outPath)
}

// distillWorker owns one Stockfish process and labels positions off the jobs
// channel. cp is normalized to White's perspective (SF reports side-to-move) before
// the logistic map.
func distillWorker(sfPath string, depth int, cpScale float64, lowPower bool, jobs <-chan string, results chan<- string, wg *sync.WaitGroup) {
	defer wg.Done()
	sf := uci.Start(sfPath, "stockfish", lowPower)
	if sf == nil {
		fmt.Fprintf(os.Stderr, "oracle distill: cannot start stockfish %q\n", sfPath)
		return
	}
	defer uci.Stop(sf)
	uci.Send(sf, "setoption name MultiPV value 1")
	uci.Send(sf, "setoption name Threads value 1")
	uci.Send(sf, "isready")
	uci.WaitFor(sf, "readyok", 5*time.Second)

	for fen := range jobs {
		cp, ok := sfEvalCp(sf, fen, depth)
		if !ok {
			continue
		}
		fields := strings.Fields(fen)
		if len(fields) >= 2 && fields[1] == "b" {
			cp = -cp // SF score is side-to-move POV; flip to White POV
		}
		target := 1.0 / (1.0 + math.Pow(10, -float64(cp)/cpScale))
		results <- fmt.Sprintf("%s %.4f", fen, target)
	}
}

// ---------- lichess: import the Lichess Stockfish eval DB as distill targets ----------

// lichessLine is the minimal slice of one Lichess eval-DB JSONL record we need:
// the FEN plus the multi-depth evals, each a MultiPV list. We decode only fen /
// depth / cp / mate / line; knodes and the rest are ignored by encoding/json.
type lichessLine struct {
	FEN   string `json:"fen"`
	Evals []struct {
		Depth int `json:"depth"`
		PVs   []struct {
			CP   *int   `json:"cp"`   // White-POV centipawns (pointer: distinguishes 0 from absent)
			Mate *int   `json:"mate"` // signed mate distance, present iff cp absent
			Line string `json:"line"`
		} `json:"pvs"`
	} `json:"evals"`
}

// runLichess converts the Lichess Stockfish eval database (one JSON object per
// line) into Texel distill samples "<FEN> <winprob>". For each position it takes
// the DEEPEST eval entry's best PV. Crucially the DB's cp is ALREADY White-POV, so
// — unlike the live-SF distillWorker — there is NO side-to-move flip. mate is mapped
// to +-mateCp. The 4-field DB FEN (no move clocks) gets " 0 1" appended. With
// -quiet (default) it keeps only positions where the static eval is meaningful:
// side-to-move not in check, and the best move not a capture/promotion — tuning a
// STATIC eval on mid-tactic positions teaches it noise (the held-out-MSE mirage).
func runLichess(inPath, outPath string, cpScale float64, mateCp int, quiet bool) {
	in, err := os.Open(inPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "oracle lichess: open %s: %v\n", inPath, err)
		os.Exit(1)
	}
	defer in.Close()
	out, err := os.Create(outPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "oracle lichess: create %s: %v\n", outPath, err)
		os.Exit(1)
	}
	defer out.Close()
	w := bufio.NewWriter(out)
	defer w.Flush()

	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<22) // deep-endgame multipv lines run large

	var read, kept, skipParse, skipNoEval, skipCheck, skipCap int
	for sc.Scan() {
		read++
		var rec lichessLine
		if json.Unmarshal(sc.Bytes(), &rec) != nil || rec.FEN == "" || len(rec.Evals) == 0 {
			skipParse++
			continue
		}
		// Deepest eval entry with a usable best PV.
		bestDepth, bi := -1, -1
		for i := range rec.Evals {
			if rec.Evals[i].Depth > bestDepth && len(rec.Evals[i].PVs) > 0 {
				bestDepth, bi = rec.Evals[i].Depth, i
			}
		}
		if bi < 0 {
			skipNoEval++
			continue
		}
		pv := rec.Evals[bi].PVs[0]
		var cp int
		switch {
		case pv.CP != nil:
			cp = *pv.CP
		case pv.Mate != nil:
			if *pv.Mate >= 0 {
				cp = mateCp
			} else {
				cp = -mateCp
			}
		default:
			skipNoEval++
			continue
		}
		// The DB FEN is 4-field (no halfmove/fullmove clocks); pad to a legal 6-field FEN.
		f := strings.Fields(rec.FEN)
		if len(f) < 4 {
			skipParse++
			continue
		}
		fen := strings.Join(f[:4], " ") + " 0 1"
		if len(f) >= 6 {
			fen = strings.Join(f[:6], " ")
		}
		pos, err := engine.ParseFEN(fen)
		if err != nil {
			skipParse++
			continue
		}
		if quiet {
			if pos.IsInCheck() {
				skipCheck++
				continue
			}
			if pv.Line != "" {
				if mvs := strings.Fields(pv.Line); len(mvs) > 0 {
					if mv, err := engine.ParseUCIMove(pos, mvs[0]); err == nil &&
						(mv.IsCapture() || mv.PromoType() != engine.NoType) {
						skipCap++
						continue
					}
				}
			}
		}
		target := 1.0 / (1.0 + math.Pow(10, -float64(cp)/cpScale))
		fmt.Fprintf(w, "%s %.4f\n", fen, target)
		kept++
		if read%200000 == 0 {
			w.Flush()
			fmt.Fprintf(os.Stderr, "  read %d, kept %d\n", read, kept)
		}
	}
	if err := sc.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "oracle lichess: scan: %v\n", err)
	}
	fmt.Fprintf(os.Stderr, "lichess: %d read -> %d kept (%.1f%%); skipped parse=%d noeval=%d check=%d capture=%d; quiet=%v scale=%.0f -> %s\n",
		read, kept, 100*float64(kept)/float64(read), skipParse, skipNoEval, skipCheck, skipCap, quiet, cpScale, outPath)
}

func runClassify(gamesPath, sfPath string, depth int, lowPower bool) {
	games, err := parseGames(gamesPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "oracle classify: %v\n", err)
		os.Exit(1)
	}
	var losses []gameRec
	for _, g := range games {
		if g.result == "L" {
			losses = append(losses, g)
		}
	}
	fmt.Printf("classify: %s — %d games, %d NGN losses (SF %s depth %d, tactical>=%dcp, eg phase<=%d)%s\n",
		gamesPath, len(games), len(losses), sfPath, depth, tacticalDropCp, egPhase, uci.PowerNote(lowPower))
	if len(losses) == 0 {
		return
	}
	sf := uci.Start(sfPath, "stockfish", lowPower)
	if sf == nil {
		fmt.Fprintf(os.Stderr, "oracle classify: cannot start stockfish %q\n", sfPath)
		os.Exit(1)
	}
	defer uci.Stop(sf)
	uci.Send(sf, "setoption name MultiPV value 1")
	uci.Send(sf, "isready")
	uci.WaitFor(sf, "readyok", 5*time.Second)

	// type x phase tally
	cnt := map[string]int{}
	fmt.Printf("  %-4s %-5s %-13s %6s %6s  %-8s %s\n", "#", "color", "reason", "plies", "finalE", "type", "decisive (drop@ply, phase)")
	for gi, g := range losses {
		// time-forfeit: classify directly, no replay needed.
		if g.reason == "time-forfeit" {
			cnt["time"]++
			fmt.Printf("  L%-3d %-5s %-13s %6d %6s  %-8s %s\n", gi+1, g.ngnColor, g.reason, len(g.moves), "-", "TIME", "(flagged on the clock)")
			continue
		}
		ngnWhite := g.ngnColor == "w"
		pos := startPosition()
		// lastNGN = SF eval (NGN POV) after the previous ply. After the opponent's
		// move it equals the eval just BEFORE NGN's next move, so an NGN move's drop
		// is lastNGN - (eval after NGN's move). One SF eval per ply.
		lastNGN, havePrev := 0, false
		biggestDrop, dropPly, dropPhase := -1<<30, -1, 24
		finalNGN := 0
		for i, mv := range g.moves {
			moverIsNGN := (pos.Turn() == engine.White) == ngnWhite
			if !applyUCI(pos, mv) {
				break
			}
			fen := engine.GenerateFEN(pos)
			cp, ok := sfEvalCp(sf, fen, depth)
			if !ok {
				continue
			}
			ngnPOV := cp // side-to-move flipped after the move; convert to NGN POV
			if (pos.Turn() == engine.White) != ngnWhite {
				ngnPOV = -cp
			}
			if ngnPOV > evalClamp { // clamp mate/decisive scores (see evalClamp)
				ngnPOV = evalClamp
			} else if ngnPOV < -evalClamp {
				ngnPOV = -evalClamp
			}
			finalNGN = ngnPOV
			if moverIsNGN && havePrev {
				if drop := lastNGN - ngnPOV; drop > biggestDrop {
					biggestDrop, dropPly, dropPhase = drop, i+1, fenPhase(fen)
				}
			}
			lastNGN, havePrev = ngnPOV, true
		}
		typ := "POSITNL"
		if biggestDrop >= tacticalDropCp {
			typ = "TACTICAL"
		}
		phaseTag := "mg"
		if dropPhase <= egPhase {
			phaseTag = "eg"
		}
		cnt[typ+"-"+phaseTag]++
		fmt.Printf("  L%-3d %-5s %-13s %6d %+6d  %-8s drop %dcp @ply%d (%s, phase %d)\n",
			gi+1, g.ngnColor, g.reason, len(g.moves), finalNGN, typ, biggestDrop, dropPly, phaseTag, dropPhase)
	}
	fmt.Println("  ----")
	fmt.Printf("  TACTICAL: mg=%d eg=%d   POSITIONAL: mg=%d eg=%d   TIME=%d\n",
		cnt["TACTICAL-mg"], cnt["TACTICAL-eg"], cnt["POSITNL-mg"], cnt["POSITNL-eg"], cnt["time"])
	fmt.Println("  TACTICAL = sudden >=200cp drop (search/depth); POSITIONAL = gradual bleed (eval); split by phase at the decisive move.")
}
