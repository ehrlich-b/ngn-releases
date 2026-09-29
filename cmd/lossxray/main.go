// Command lossxray re-classifies NGN's losses using STOCKFISH as the eval ruler
// (not NGN's own eval, which the cmd/oracle "classify" tool uses and which is
// circular: an NGN eval blind-spot reads "fine" until material falls, then
// craters in one move and is mislabeled TACTICAL/depth).
//
// For each NGN loss in a gauntlet -pgn capture it walks the game, getting SF's
// NGN-POV eval (fixed depth) at every ply AND NGN's own eval (movetime) over a
// pre-decisive window, then classifies the decisive blunder by whether SF saw
// NGN losing LONG before NGN did (EVAL blind-spot) or only sharply at the end
// (DEPTH/tactical miss). Read-only on engine source; lives in its own cmd.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ehrlich-b/ngn/engine"
	"github.com/ehrlich-b/ngn/internal/uci"
)

const (
	evalClamp = 1500 // bound SF eval before drop computation (mate scores mask the origin)
	egPhase   = 6    // PeSTO phase <= this = endgame (matches cmd/oracle)

	losingCp = -150 // SF NGN-POV <= this means SF judges NGN clearly worse
	// sustained: SF had NGN losing for this many plies BEFORE the decisive move
	sustainedPlies = 12 // 6 full moves
	// NGN's own eval median in the pre-decisive window above this = NGN thought it was fine
	ngnFineCp     = -75
	tacticalDrop  = 200 // single-move SF drop that = a sharp/tactical swing
	preWindow     = 30  // plies before the decisive move we examine NGN's own eval over
	ngnSampleEach = 3   // sample NGN's own eval every N plies in the window (efficiency)
)

type gameRec struct {
	anchor, result, ngnColor, reason string
	moves                            []string
}

func startPosition() *engine.Position {
	return &engine.Position{
		Board: engine.StartingBoard(),
		Tag: engine.WhiteCanCastleKingSide | engine.WhiteCanCastleQueenSide |
			engine.BlackCanCastleKingSide | engine.BlackCanCastleQueenSide | engine.WhiteToMove,
		EnPassant: engine.NoSquare,
	}
}

func applyUCI(pos *engine.Position, uciMove string) bool {
	mv, err := engine.ParseUCIMove(pos, uciMove)
	if err != nil {
		return false
	}
	pos.MakeMove(mv)
	return true
}

func parseGames(path string) ([]gameRec, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []gameRec
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<22)
	for sc.Scan() {
		ln := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(ln, "GAME ") {
			continue
		}
		head, moves := ln, []string(nil)
		if i := strings.Index(ln, " | "); i >= 0 {
			moves = strings.Fields(ln[i+3:])
			head = ln[:i]
		}
		fld := strings.Fields(head)
		if len(fld) < 5 {
			continue
		}
		out = append(out, gameRec{fld[1], fld[2], fld[3], strings.Join(fld[4:], " "), moves})
	}
	return out, sc.Err()
}

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

// sfGoCmd is the go command used for every SF eval (set in main: depth or movetime).
var sfGoCmd = "go depth 16"

// skipOpeningPlies: don't SF-eval before this ply (opening positions are the
// slowest at fixed depth and never contain a game's decisive blunder). Set in main.
var skipOpeningPlies = 0

// sfEval returns SF's eval from the side-to-move POV (cp; mate mapped) at sfGoCmd.
func sfEval(sf *uci.Engine, fen string, depth int) (int, bool) {
	uci.Send(sf, "position fen "+fen)
	lines := uci.Analyze(sf, sfGoCmd, 30*time.Second)
	cp, have := 0, false
	for _, ln := range lines {
		if !strings.Contains(ln, " score ") {
			continue
		}
		f := strings.Fields(ln)
		for i := 0; i+1 < len(f); i++ {
			switch f[i] {
			case "cp":
				cp, _ = strconv.Atoi(f[i+1])
				have = true
			case "mate":
				m, _ := strconv.Atoi(f[i+1])
				sign := 1
				if m < 0 {
					sign, m = -1, -m
				}
				cp, have = sign*(100000-m), true
			}
		}
	}
	return cp, have
}

// ngnEval returns NGN's own eval (side-to-move POV cp) at a movetime budget, plus
// the PV's first move. NGN must keep stdin open until bestmove (Analyze does this).
func ngnEval(ng *uci.Engine, fen string, movetimeMs int) (int, bool) {
	uci.Send(ng, "position fen "+fen)
	lines := uci.Analyze(ng, fmt.Sprintf("go movetime %d", movetimeMs), 30*time.Second)
	cp, have := 0, false
	for _, ln := range lines {
		if !strings.Contains(ln, " score ") {
			continue
		}
		f := strings.Fields(ln)
		for i := 0; i+1 < len(f); i++ {
			switch f[i] {
			case "cp":
				cp, _ = strconv.Atoi(f[i+1])
				have = true
			case "mate":
				m, _ := strconv.Atoi(f[i+1])
				sign := 1
				if m < 0 {
					sign, m = -1, -m
				}
				cp, have = sign*(100000-m), true
			}
		}
	}
	return cp, have
}

func clamp(x int) int {
	if x > evalClamp {
		return evalClamp
	}
	if x < -evalClamp {
		return -evalClamp
	}
	return x
}

func median(xs []int) int {
	if len(xs) == 0 {
		return 0
	}
	c := append([]int(nil), xs...)
	sort.Ints(c)
	return c[len(c)/2]
}

type lossAnalysis struct {
	idx                        int
	color, reason              string
	plies                      int
	decisivePly, decisivePhase int
	drop                       int
	sfBeforeDecisive           int // SF NGN-POV just before the decisive move
	sustainedBadPlies          int // consecutive plies before decisive with SF<=losingCp
	ngnMedianPre               int // NGN's own median eval over the pre-decisive window
	sfMedianPre                int // SF's median NGN-POV eval over the same window
	ponrPly                    int // point-of-no-return: SF's first ply NGN was lost-for-good
	ponrTailPlies              int // plies NGN kept playing after the point of no return
	ngnMedianAtPonr            int // NGN's own median eval from the point of no return forward
	sfMedianAtPonr             int // SF's median NGN-POV eval over the same span
	classification             string
	phaseTag                   string
	passedPawn                 bool
	fenAtDecisive              string
	note                       string
}

func main() {
	pgn := flag.String("pgn", "output/gauntlet_stack.pgn", "gauntlet -pgn capture")
	sfPath := flag.String("sf", "/opt/homebrew/bin/stockfish", "stockfish path")
	ngnPath := flag.String("ngn", "build/ngn", "ngn binary path")
	depth := flag.Int("depth", 16, "SF search depth (ignored if -sfmt > 0)")
	sfMt := flag.Int("sfmt", 0, "SF movetime ms per eval (0 = use -depth); caps per-position cost")
	ngnMt := flag.Int("ngnmt", 250, "NGN movetime ms for its own eval")
	maxLosses := flag.Int("max", 0, "cap losses analyzed (0=all)")
	skipOpen := flag.Int("skipopen", 16, "do not SF-eval before this ply (opening speedup)")
	from := flag.Int("from", 0, "analyze losses from this 1-based global index (0=start); for sharding")
	to := flag.Int("to", 0, "analyze losses up to this 1-based global index inclusive (0=end); for sharding")
	detail := flag.String("detail", "", "comma-sep loss indices: print full ply-by-ply SF+NGN trace for these and exit")
	flag.Parse()

	if *sfMt > 0 {
		sfGoCmd = fmt.Sprintf("go movetime %d", *sfMt)
	} else {
		sfGoCmd = fmt.Sprintf("go depth %d", *depth)
	}
	skipOpeningPlies = *skipOpen

	games, err := parseGames(*pgn)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var losses []gameRec
	for _, g := range games {
		if g.result == "L" && g.reason != "time-forfeit" {
			losses = append(losses, g)
		}
	}
	if *maxLosses > 0 && len(losses) > *maxLosses {
		losses = losses[:*maxLosses]
	}
	// Sharding: keep global indices, analyze only [from,to]. offset is added to the
	// per-shard loop index to recover the global 1-based loss number.
	offset := 0
	if *from > 0 || *to > 0 {
		lo, hi := 1, len(losses)
		if *from > 0 {
			lo = *from
		}
		if *to > 0 && *to < hi {
			hi = *to
		}
		if lo < 1 {
			lo = 1
		}
		offset = lo - 1
		losses = losses[lo-1 : hi]
	}
	fmt.Printf("lossxray: %s — %d games, analyzing %d NGN losses [global %d..%d] (SF '%s', NGN movetime %dms, losing<=%dcp, sustained>=%dply)\n",
		*pgn, len(games), len(losses), offset+1, offset+len(losses), sfGoCmd, *ngnMt, losingCp, sustainedPlies)

	sf := uci.Start(*sfPath, "stockfish", true)
	if sf == nil {
		fmt.Fprintln(os.Stderr, "cannot start stockfish")
		os.Exit(1)
	}
	defer uci.Stop(sf)
	uci.Send(sf, "setoption name MultiPV value 1")
	uci.Send(sf, "isready")
	uci.WaitFor(sf, "readyok", 5*time.Second)

	ng := uci.Start(*ngnPath, "ngn", true)
	if ng == nil {
		fmt.Fprintln(os.Stderr, "cannot start ngn")
		os.Exit(1)
	}
	defer uci.Stop(ng)
	uci.Send(ng, "isready")
	uci.WaitFor(ng, "readyok", 5*time.Second)

	if *detail != "" {
		for _, s := range strings.Split(*detail, ",") {
			n, err := strconv.Atoi(strings.TrimSpace(s))
			if err != nil || n < 1 || n > len(losses) {
				continue
			}
			detailLoss(n, losses[n-1], sf, ng, *depth, *ngnMt)
		}
		return
	}

	var results []lossAnalysis
	for gi, g := range losses {
		fmt.Fprintf(os.Stderr, "  [%d/%d] L%d %d plies...\n", gi+1, len(losses), offset+gi+1, len(g.moves))
		results = append(results, analyzeLoss(offset+gi+1, g, sf, ng, *depth, *ngnMt))
	}

	report(results)
}

// detailLoss prints a full ply-by-ply trace of one loss: SF NGN-POV eval at every
// ply and NGN's own NGN-POV eval at each NGN move, so the eval/depth call is auditable.
func detailLoss(idx int, g gameRec, sf, ng *uci.Engine, depth, ngnMt int) {
	ngnWhite := g.ngnColor == "w"
	pos := startPosition()
	fmt.Printf("\n===== LOSS L%d  anchor=%s  NGN=%s  reason=%s  plies=%d =====\n",
		idx, g.anchor, g.ngnColor, g.reason, len(g.moves))
	fmt.Printf("  %-4s %-3s %-7s %8s %8s %8s\n", "ply", "by", "move", "sfNGN", "ngnOwn", "dropSF")
	prevSF, havePrev := 0, false
	fenBefore := "" // position before the current ply (the position NGN moved FROM)
	for i, mv := range g.moves {
		moverNGN := (pos.Turn() == engine.White) == ngnWhite
		if !applyUCI(pos, mv) {
			break
		}
		fen := engine.GenerateFEN(pos)
		cp, ok := sfEval(sf, fen, depth)
		if !ok {
			continue
		}
		sfNGN := cp
		if (pos.Turn() == engine.White) != ngnWhite {
			sfNGN = -cp
		}
		sfNGN = clamp(sfNGN)
		by := "blu"
		if moverNGN {
			by = "NGN"
		}
		ngnStr, dropStr := "", ""
		if moverNGN {
			nc, ok := ngnEval(ng, fen, ngnMt)
			if ok {
				whiteAfter := strings.Contains(fen, " w ")
				nv := nc
				if whiteAfter != ngnWhite {
					nv = -nc
				}
				ngnStr = fmt.Sprintf("%+d", clamp(nv))
			}
			if havePrev {
				dropStr = fmt.Sprintf("%+d", prevSF-sfNGN)
			}
		}
		mark := ""
		if moverNGN && havePrev && prevSF-sfNGN >= tacticalDrop {
			mark = "  <-- big SF drop  FEN_BEFORE: " + fenBefore
		}
		fmt.Printf("  %-4d %-3s %-7s %+8d %8s %8s%s\n", i+1, by, mv, sfNGN, ngnStr, dropStr, mark)
		prevSF, havePrev, fenBefore = sfNGN, true, fen
	}
	// Print the FEN at a couple of key plies for the writeup.
	fmt.Printf("  (final FEN: %s)\n", engine.GenerateFEN(pos))
}

// analyzeLoss walks one NGN loss: SF eval (NGN-POV) at every ply to find the
// decisive drop, then NGN's own eval over the pre-decisive window, then classifies.
func analyzeLoss(idx int, g gameRec, sf, ng *uci.Engine, depth, ngnMt int) lossAnalysis {
	ngnWhite := g.ngnColor == "w"
	pos := startPosition()

	// sfPOV[i] = SF NGN-POV eval after ply i (0-based index into g.moves). fens[i] same.
	sfPOV := make([]int, len(g.moves))
	fens := make([]string, len(g.moves))
	moverNGN := make([]bool, len(g.moves))
	have := make([]bool, len(g.moves))

	lastNGN, havePrev := 0, false
	biggestDrop, dropPly, dropPhase := -1<<30, -1, 24
	sfBeforeDecisive := 0
	for i, mv := range g.moves {
		moverNGN[i] = (pos.Turn() == engine.White) == ngnWhite
		if !applyUCI(pos, mv) {
			break
		}
		fen := engine.GenerateFEN(pos)
		fens[i] = fen
		if i < skipOpeningPlies {
			continue // opening: not decisive, slowest at fixed depth
		}
		cp, ok := sfEval(sf, fen, depth)
		if !ok {
			continue
		}
		ngnPOV := cp
		if (pos.Turn() == engine.White) != ngnWhite {
			ngnPOV = -cp
		}
		ngnPOV = clamp(ngnPOV)
		sfPOV[i] = ngnPOV
		have[i] = true
		if moverNGN[i] && havePrev {
			if drop := lastNGN - ngnPOV; drop > biggestDrop {
				biggestDrop, dropPly, dropPhase = drop, i, fenPhase(fen)
				sfBeforeDecisive = lastNGN
			}
		}
		lastNGN, havePrev = ngnPOV, true
	}

	res := lossAnalysis{
		idx: idx, color: g.ngnColor, reason: g.reason, plies: len(g.moves),
		decisivePly: dropPly + 1, decisivePhase: dropPhase, drop: biggestDrop,
		sfBeforeDecisive: sfBeforeDecisive,
	}
	if dropPly >= 0 {
		res.fenAtDecisive = fens[dropPly]
	}

	// Pre-decisive window: how long had SF judged NGN clearly losing BEFORE the
	// decisive move? Count consecutive NGN-POV evals <= losingCp ending at dropPly-1.
	sustained := 0
	for i := dropPly - 1; i >= 0; i-- {
		if !have[i] {
			continue
		}
		if sfPOV[i] <= losingCp {
			sustained++
		} else {
			break
		}
	}
	res.sustainedBadPlies = sustained

	// POINT OF NO RETURN (ply-detector-independent): the first post-opening ply
	// where SF judges NGN clearly losing (<=losingCp) AND, from there to the end,
	// SF's eval STAYS losing on balance (median of the remaining post-opening evals
	// <= losingCp). Using the median tolerates brief upward eval noise at fixed
	// depth, so this is SF's robust verdict on WHEN NGN was lost — independent of
	// the noisy single biggest-drop ply.
	ponr := -1
	for i := 0; i < len(g.moves); i++ {
		if !have[i] || sfPOV[i] > losingCp {
			continue
		}
		var rest []int
		for j := i; j < len(g.moves); j++ {
			if have[j] {
				rest = append(rest, sfPOV[j])
			}
		}
		if median(rest) <= losingCp {
			ponr = i
			break
		}
	}
	res.ponrPly = ponr + 1
	if ponr >= 0 {
		res.ponrTailPlies = len(g.moves) - ponr // how long NGN kept playing while SF-lost
		// NGN's OWN eval at/after the point of no return: did NGN still think it was
		// fine while SF had condemned it? Sample a few plies from ponr forward.
		var ngnAtPonr, sfAtPonr []int
		for k := 0; k < 8; k++ {
			i := ponr + k*ngnSampleEach
			if i >= len(g.moves) || !have[i] {
				continue
			}
			sfAtPonr = append(sfAtPonr, sfPOV[i])
			nc, ok := ngnEval(ng, fens[i], ngnMt)
			if !ok {
				continue
			}
			whiteAfter := strings.Contains(fens[i], " w ")
			nv := nc
			if whiteAfter != ngnWhite {
				nv = -nc
			}
			ngnAtPonr = append(ngnAtPonr, clamp(nv))
		}
		res.ngnMedianAtPonr = median(ngnAtPonr)
		res.sfMedianAtPonr = median(sfAtPonr)
	}

	// Phase tag: use the phase at the POINT OF NO RETURN (where the loss became
	// irreversible) when one exists; else the decisive-drop ply. This makes the
	// mg/eg split match where NGN actually lost, not just where material fell.
	phaseSrc := dropPhase
	if ponr >= 0 && fens[ponr] != "" {
		phaseSrc = fenPhase(fens[ponr])
	}
	res.phaseTag = "mg"
	if phaseSrc <= egPhase {
		res.phaseTag = "eg"
	}

	// Sample NGN's OWN eval and collect SF eval over the pre-decisive window
	// [dropPly-preWindow, dropPly-1]. Compare medians.
	lo := dropPly - preWindow
	if lo < 0 {
		lo = 0
	}
	var ngnVals, sfVals []int
	for i := lo; i < dropPly; i++ {
		if !have[i] {
			continue
		}
		sfVals = append(sfVals, sfPOV[i])
		// Only sample NGN's eval every few plies; only at positions where it is
		// NGN to move would bias it, so just take NGN's static-search eval of the
		// position regardless of side and flip to NGN-POV.
		if (i-lo)%ngnSampleEach != 0 {
			continue
		}
		nc, ok := ngnEval(ng, fens[i], ngnMt)
		if !ok {
			continue
		}
		// NGN reports side-to-move POV; flip to NGN POV.
		// Determine side to move at fens[i]: it's the side AFTER ply i.
		whiteToMoveAfter := strings.Contains(fens[i], " w ")
		ngnPOVval := nc
		if whiteToMoveAfter != ngnWhite {
			ngnPOVval = -nc
		}
		ngnVals = append(ngnVals, clamp(ngnPOVval))
	}
	res.ngnMedianPre = median(ngnVals)
	res.sfMedianPre = median(sfVals)

	// Passed-pawn culprit detection at the decisive position: a pawn on 6th/7th
	// rank (relative) for either side, OR SF's PV promotes within a few moves.
	res.passedPawn = passedPawnInvolved(res.fenAtDecisive, sf, depth)

	// CLASSIFY (SF as the ruler). Lead with the robust point-of-no-return
	// divergence (independent of the noisy biggest-drop ply):
	//   - EVAL-BLINDSPOT: SF condemned NGN well before mate AND at that moment NGN's
	//     OWN eval still read fine -> NGN mis-VALUED the position (the masquerade).
	//   - SLOW-BLEED: SF condemned NGN well before mate but NGN's own eval also said
	//     losing -> gradual loss both engines saw (not a 1-move miss, not a blind-spot).
	//   - DEPTH: a sharp single-move SF swing with SF ~ok just before -> a tactic NGN
	//     (and SF at shallow depth) only saw at the last moment; short dead-tail.
	const deadTail = 16 // plies NGN kept playing after SF's point of no return
	hasPONR := res.ponrPly > 0
	switch {
	case hasPONR && res.ponrTailPlies >= deadTail && res.ngnMedianAtPonr >= ngnFineCp:
		res.classification = "EVAL-BLINDSPOT"
		res.note = fmt.Sprintf("SF declared NGN lost @ply%d (then %d plies played); at that point NGN's own eval was %+dcp vs SF %+dcp",
			res.ponrPly, res.ponrTailPlies, res.ngnMedianAtPonr, res.sfMedianAtPonr)
	case hasPONR && res.ponrTailPlies >= deadTail:
		res.classification = "SLOW-BLEED"
		res.note = fmt.Sprintf("SF declared NGN lost @ply%d; NGN's own eval AGREED (%+dcp, SF %+dcp) — gradual loss both saw",
			res.ponrPly, res.ngnMedianAtPonr, res.sfMedianAtPonr)
	case biggestDrop >= tacticalDrop && sfBeforeDecisive >= ngnFineCp:
		res.classification = "DEPTH"
		res.note = fmt.Sprintf("sharp %dcp SF swing @ply%d; SF was %+dcp just before — ~ok until the tactic", biggestDrop, res.decisivePly, sfBeforeDecisive)
	case biggestDrop >= tacticalDrop:
		// Big drop but SF already had NGN somewhat worse before it: a tactic that
		// converted an already-inferior position. Tag DEPTH but flag the context.
		res.classification = "DEPTH"
		res.note = fmt.Sprintf("sharp %dcp SF swing @ply%d from an already-worse %+dcp", biggestDrop, res.decisivePly, sfBeforeDecisive)
	default:
		res.classification = "UNCLEAR"
		res.note = fmt.Sprintf("drop %dcp, ponr@%d tail%d, NGN@ponr %+d, SF@ponr %+d", biggestDrop, res.ponrPly, res.ponrTailPlies, res.ngnMedianAtPonr, res.sfMedianAtPonr)
	}
	return res
}

// passedPawnInvolved: at the decisive FEN, is there a pawn on the 6th/7th rank
// (relative to its color), or does SF's PV include a promotion within ~6 plies?
func passedPawnInvolved(fen string, sf *uci.Engine, depth int) bool {
	if fen == "" {
		return false
	}
	rows := strings.Split(strings.Fields(fen)[0], "/")
	if len(rows) == 8 {
		// rows[0]=rank8 ... rows[7]=rank1. White pawn advanced = rank7/rank8 area
		// (rows[1],rows[0]); black pawn advanced = rank2/rank1 (rows[6],rows[7]).
		for _, c := range rows[1] {
			if c == 'P' {
				return true
			}
		}
		for _, c := range rows[6] {
			if c == 'p' {
				return true
			}
		}
	}
	// SF PV promotion check.
	uci.Send(sf, "position fen "+fen)
	lines := uci.Analyze(sf, sfGoCmd, 30*time.Second)
	for _, ln := range lines {
		if i := strings.Index(ln, " pv "); i >= 0 {
			pv := strings.Fields(ln[i+4:])
			end := 6
			if len(pv) < end {
				end = len(pv)
			}
			for _, m := range pv[:end] {
				// promotion UCI move is 5 chars ending in q/r/b/n
				if len(m) == 5 {
					switch m[4] {
					case 'q', 'r', 'b', 'n':
						return true
					}
				}
			}
		}
	}
	return false
}

func report(rs []lossAnalysis) {
	fmt.Println()
	fmt.Printf("  %-4s %-3s %5s %5s %8s %6s %6s %8s %8s %-15s %s\n",
		"#", "col", "ply", "ponr", "tail", "drop", "dPly", "ngn@pnr", "sf@pnr", "class", "phase/pp")
	type key struct{ class, phase string }
	cnt := map[key]int{}
	ppByClass := map[string]int{}
	for _, r := range rs {
		pp := ""
		if r.passedPawn {
			pp = " PP"
			ppByClass[r.classification]++
		}
		fmt.Printf("  L%-3d %-3s %5d %5d %8d %6d %6d %+8d %+8d %-15s %s%s\n",
			r.idx, r.color, r.plies, r.ponrPly, r.ponrTailPlies, r.drop, r.decisivePly,
			r.ngnMedianAtPonr, r.sfMedianAtPonr, r.classification, r.phaseTag, pp)
		cnt[key{r.classification, r.phaseTag}]++
	}
	fmt.Println("  ----")
	classes := []string{"EVAL-BLINDSPOT", "SLOW-BLEED", "DEPTH", "UNCLEAR"}
	fmt.Printf("  %-16s %5s %5s %5s   %s\n", "class", "mg", "eg", "all", "(PP = passed-pawn involved at decisive pos)")
	for _, c := range classes {
		mg, eg := cnt[key{c, "mg"}], cnt[key{c, "eg"}]
		fmt.Printf("  %-16s %5d %5d %5d   PP=%d\n", c, mg, eg, mg+eg, ppByClass[c])
	}
	// Endgame verdict.
	var egEval, egDepth, egOther int
	for _, r := range rs {
		if r.phaseTag != "eg" {
			continue
		}
		switch r.classification {
		case "EVAL-BLINDSPOT":
			egEval++
		case "DEPTH":
			egDepth++
		default:
			egOther++
		}
	}
	fmt.Println("  ----")
	fmt.Printf("  ENDGAME losses: EVAL-BLINDSPOT=%d  DEPTH=%d  OTHER(slow-bleed/unclear)=%d\n", egEval, egDepth, egOther)
}
