package engine

import (
	"bufio"
	"fmt"
	"math"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// Texel tuning: fit the static evaluation's parameters (the piece-square tables)
// to real game outcomes by minimizing the mean squared error between each
// position's game result and a logistic of its static eval. evaluateUnsafe is a
// pure white-perspective score (the property the symmetry test guards), so we
// score it directly against results stored from White's point of view.

// TexelSample is one labeled position: a board parsed once into memory, and the
// game result from White's perspective (1.0 = White won, 0.5 = draw, 0.0 = lost).
type TexelSample struct {
	Board  Bitboard
	Result float64
}

// texelSigmoid maps a white-perspective centipawn eval to an expected score in
// (0,1) via the standard base-10 logistic with scaling constant K.
func texelSigmoid(eval, k float64) float64 {
	return 1.0 / (1.0 + math.Pow(10, -k*eval/400.0))
}

// texelMSE is the mean squared error between game results and the sigmoid of the
// static (white-perspective) eval over the dataset, at scaling constant K. It is
// parallelized across CPUs and only READS the eval parameters, so callers must
// not mutate the PSTs concurrently (coordinate descent mutates between calls).
func texelMSE(samples []TexelSample, k float64) float64 {
	// Texel passes nil explicitly for pawn-cache ownership: samples are distinct
	// and its parallel workers must not share mutable search caches.
	n := len(samples)
	if n == 0 {
		return 0
	}
	workers := runtime.GOMAXPROCS(0)
	if workers > n {
		workers = n
	}
	chunk := (n + workers - 1) / workers
	partial := make([]float64, workers)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		lo := w * chunk
		if lo >= n {
			break
		}
		hi := lo + chunk
		if hi > n {
			hi = n
		}
		wg.Add(1)
		go func(idx, lo, hi int) {
			defer wg.Done()
			var sum float64
			for i := lo; i < hi; i++ {
				// PST tables were just mutated (RebuildPST), so the sample's
				// cached eval accumulator is stale — rebuild it before eval.
				samples[i].Board.recomputeAccumulator()
				e := float64(evaluateUnsafe(&samples[i].Board))
				d := samples[i].Result - texelSigmoid(e, k)
				sum += d * d
			}
			partial[idx] = sum
		}(w, lo, hi)
	}
	wg.Wait()
	var total float64
	for _, s := range partial {
		total += s
	}
	return total / float64(n)
}

// TexelFindK ternary-searches for the scaling constant K that minimizes MSE with
// the CURRENT eval parameters — the one-time calibration before tuning so the
// logistic is scaled to the eval's centipawn units.
func TexelFindK(samples []TexelSample) float64 {
	_ = mustAcquireHCEModelUse()
	defer releaseHCEModelUse()
	return texelFindKLeased(samples)
}

func texelFindKLeased(samples []TexelSample) float64 {
	lo, hi := 0.0, 2.0
	for iter := 0; iter < 50; iter++ {
		m1 := lo + (hi-lo)/3
		m2 := hi - (hi-lo)/3
		if texelMSE(samples, m1) < texelMSE(samples, m2) {
			hi = m2
		} else {
			lo = m1
		}
	}
	return (lo + hi) / 2
}

// TexelWeight is a named tunable evaluation weight — a pointer into one of the
// eval's package vars, with its name so a run can both optimize and print it.
type TexelWeight struct {
	Name string
	Ptr  *int
}

// TexelWeights returns the hand-set eval-term weights exposed for tuning (the
// untuned scalars complementing the already-tuned PeSTO core).
func TexelWeights() []TexelWeight {
	return []TexelWeight{
		// 10 middlegame weights (index 0..9), then their 10 endgame twins (10..19) —
		// the order the gradient trace's scalar indices assume (texelScalarBase+i
		// is MG term i, texelScalarBase+10+i its EG twin).
		{"passedPawnBonus", &passedPawnBonus},
		{"doubledPawnPenalty", &doubledPawnPenalty},
		{"isolatedPawnPenalty", &isolatedPawnPenalty},
		{"backwardPawnPenalty", &backwardPawnPenalty},
		{"pawnChainWeight", &pawnChainWeight},
		{"mobilityWeight", &mobilityWeight},
		{"rookOpenWeight", &rookOpenWeight},
		{"outpostWeight", &outpostWeight},
		{"kingSafetyWeight", &kingSafetyWeight},
		{"kingActivityWeight", &kingActivityWeight},
		{"passedPawnBonusEG", &passedPawnBonusEG},
		{"doubledPawnPenaltyEG", &doubledPawnPenaltyEG},
		{"isolatedPawnPenaltyEG", &isolatedPawnPenaltyEG},
		{"backwardPawnPenaltyEG", &backwardPawnPenaltyEG},
		{"pawnChainWeightEG", &pawnChainWeightEG},
		{"mobilityWeightEG", &mobilityWeightEG},
		{"rookOpenWeightEG", &rookOpenWeightEG},
		{"outpostWeightEG", &outpostWeightEG},
		{"kingSafetyWeightEG", &kingSafetyWeightEG},
		{"kingActivityWeightEG", &kingActivityWeightEG},
	}
}

// init applies eval aux-weight overrides from the NGN_EVAL_W environment variable
// (comma-separated name=value pairs, names from TexelWeights plus the blocked-passer
// knobs). This lets the ACPL tuner vary weights per trial WITHOUT rebuilding the
// binary: the engine subprocess inherits the parent env (uci.Start sets no cmd.Env),
// and the 20 aux weights are read at eval runtime (not baked into any precomputed
// table), so an init override takes effect for every search. Unset/empty = no
// override (the bit-for-bit default). A malformed pair or unknown name aborts — a
// silent mis-tune would corrupt every downstream ACPL measurement.
func init() {
	spec := os.Getenv("NGN_EVAL_W")
	if spec == "" {
		return
	}
	byName := make(map[string]*int)
	for _, w := range TexelWeights() {
		byName[w.Name] = w.Ptr
	}
	byName["blockedPasserPenalty"] = &blockedPasserPenalty
	byName["blockedPasserPenaltyEG"] = &blockedPasserPenaltyEG
	for _, kv := range strings.Split(spec, ",") {
		kv = strings.TrimSpace(kv)
		if kv == "" {
			continue
		}
		eq := strings.IndexByte(kv, '=')
		if eq < 0 {
			fmt.Fprintf(os.Stderr, "NGN_EVAL_W: bad pair %q (want name=value)\n", kv)
			os.Exit(1)
		}
		name := strings.TrimSpace(kv[:eq])
		val, err := strconv.Atoi(strings.TrimSpace(kv[eq+1:]))
		if err != nil {
			fmt.Fprintf(os.Stderr, "NGN_EVAL_W: bad value in %q: %v\n", kv, err)
			os.Exit(1)
		}
		ptr, ok := byName[name]
		if !ok {
			fmt.Fprintf(os.Stderr, "NGN_EVAL_W: unknown weight %q\n", name)
			os.Exit(1)
		}
		*ptr = val
	}
}

// FormatTexelWeights renders the given weights as Go var assignments, ready to
// paste back into eval.go's tunable-weights block after a tuning run.
func FormatTexelWeights(weights []TexelWeight) string {
	var b strings.Builder
	for _, w := range weights {
		fmt.Fprintf(&b, "\t%s = %d\n", w.Name, *w.Ptr)
	}
	return b.String()
}

// FormatPSTTables renders the 12 raw piece-square tables as Go var declarations
// matching eval_pesto.go's layout (8 values per row), so a tuned set can be
// pasted back in to replace the originals.
func FormatPSTTables() string {
	named := []struct {
		name string
		tbl  *[64]int
	}{
		{"mgPawnTable", &mgPawnTable}, {"mgKnightTable", &mgKnightTable}, {"mgBishopTable", &mgBishopTable},
		{"mgRookTable", &mgRookTable}, {"mgQueenTable", &mgQueenTable}, {"mgKingTable", &mgKingTable},
		{"egPawnTable", &egPawnTable}, {"egKnightTable", &egKnightTable}, {"egBishopTable", &egBishopTable},
		{"egRookTable", &egRookTable}, {"egQueenTable", &egQueenTable}, {"egKingTable", &egKingTable},
	}
	var b strings.Builder
	for _, t := range named {
		fmt.Fprintf(&b, "var %s = [64]int{\n", t.name)
		for sq := 0; sq < 64; sq++ {
			if sq%8 == 0 {
				b.WriteByte('\t')
			}
			fmt.Fprintf(&b, "%4d,", t.tbl[sq])
			if sq%8 == 7 {
				b.WriteByte('\n')
			} else {
				b.WriteByte(' ')
			}
		}
		b.WriteString("}\n\n")
	}
	return b.String()
}

// TexelMaterialParams returns pointers to the tunable PeSTO material values
// (pawn..queen, MG then EG — 10 values; the king and unused index 0 stay fixed).
// Exposed so coordinate descent can move a piece's material level DIRECTLY in one
// step: the PST tuner can only shift a piece's value by stepping its 64 squares
// one at a time across many passes, so it reaches the material direction
// inefficiently. These feed mgPST/egPST, so a RebuildPST is required after any
// mutation (TexelTune's rebuild callback handles it).
// TexelMaterialParams exposes offline-only pointers to the process HCE model.
// Callers must not retain or mutate them across any search/evaluation lifetime;
// use ApplyTexelModel or a checked TryTexelTune entry point for runtime changes.
func TexelMaterialParams() []*int {
	ptrs := make([]*int, 0, 10)
	for pt := Pawn; pt <= Queen; pt++ {
		ptrs = append(ptrs, &pestoMGMaterial[pt])
	}
	for pt := Pawn; pt <= Queen; pt++ {
		ptrs = append(ptrs, &pestoEGMaterial[pt])
	}
	return ptrs
}

// FormatMaterialValues renders the tuned PeSTO material arrays as Go var
// declarations for paste-back into eval_pesto.go.
func FormatMaterialValues() string {
	var b strings.Builder
	fmt.Fprintf(&b, "var pestoMGMaterial = [7]int{%d, %d, %d, %d, %d, %d, %d}\n",
		pestoMGMaterial[0], pestoMGMaterial[1], pestoMGMaterial[2], pestoMGMaterial[3], pestoMGMaterial[4], pestoMGMaterial[5], pestoMGMaterial[6])
	fmt.Fprintf(&b, "var pestoEGMaterial = [7]int{%d, %d, %d, %d, %d, %d, %d}\n",
		pestoEGMaterial[0], pestoEGMaterial[1], pestoEGMaterial[2], pestoEGMaterial[3], pestoEGMaterial[4], pestoEGMaterial[5], pestoEGMaterial[6])
	return b.String()
}

// TexelTune runs coordinate descent on the piece-square tables to minimize MSE
// over the dataset at scaling K. Each pass line-searches every parameter (steps
// in the improving direction while MSE keeps falling), keeping all gains; it
// repeats until a pass yields no improvement or maxPasses is reached. progress
// (may be nil) is called once per pass with the pass index, current MSE, and how
// many parameters moved. The PST package vars are left holding the tuned values
// (black mirrors rebuilt). Returns the final MSE.
// rebuild (may be nil) is invoked after every parameter mutation — used when a
// tuned param feeds a derived table (e.g. the PeSTO mgPST/egPST) that must be
// recomputed before the next eval. For directly-used weights, pass nil.
func TryTexelTune(samples []TexelSample, params []*int, k float64, step, maxPasses int, rebuild func(), progress func(pass int, mse float64, moved int)) (float64, error) {
	initial, err := beginHCETuning()
	if err != nil {
		return 0, err
	}
	defer finishHCETuning(initial)
	return texelTuneLeased(samples, params, k, step, maxPasses, rebuild, progress), nil
}

// TexelTune preserves the legacy signature and fails fast rather than waiting
// if model state is busy. New callers should use TryTexelTune.
func TexelTune(samples []TexelSample, params []*int, k float64, step, maxPasses int, rebuild func(), progress func(pass int, mse float64, moved int)) float64 {
	result, err := TryTexelTune(samples, params, k, step, maxPasses, rebuild, progress)
	if err != nil {
		panic(err)
	}
	return result
}

func texelTuneLeased(samples []TexelSample, params []*int, k float64, step, maxPasses int, rebuild func(), progress func(pass int, mse float64, moved int)) float64 {
	best := texelMSE(samples, k)

	// lineSearch steps *p by delta while MSE strictly improves, leaving *p at the
	// best value found; returns true if it moved at all.
	lineSearch := func(p *int, delta int) bool {
		moved := false
		for {
			*p += delta
			if rebuild != nil {
				rebuild()
			}
			m := texelMSE(samples, k)
			if m < best {
				best = m
				moved = true
				continue
			}
			*p -= delta // overshot: undo the last (non-improving) step
			if rebuild != nil {
				rebuild()
			}
			return moved
		}
	}

	for pass := 0; pass < maxPasses; pass++ {
		moved := 0
		for _, p := range params {
			if lineSearch(p, step) {
				moved++
				continue
			}
			if lineSearch(p, -step) {
				moved++
			}
		}
		if progress != nil {
			progress(pass, best, moved)
		}
		if moved == 0 {
			break
		}
	}
	return best
}

// LoadTexelSamples reads a dataset of "<FEN> <result>" lines (result one of 1.0 /
// 0.5 / 0.0, White's perspective), parsing each FEN once into an in-memory board.
// Blank lines and lines starting with '#' are skipped; unparseable lines error.
//
// Positions are deduped by full Zobrist key (pieces, side, castling and EP),
// keeping the first outcome and returning the duplicate count. This reduces
// repeated full positions; it does not establish game-disjoint validation or
// board-only model-input separation. Outcome labels must be finite and in [0,1],
// including observations of positions already seen.
func LoadTexelSamples(filename string) (samples []TexelSample, nDup int, err error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	seen := make(map[uint64]bool)
	// keep appends a sample unless its position was already seen; counts duplicates.
	keep := func(pos *Position, result float64) {
		key := pos.Hash()
		if seen[key] {
			nDup++
			return
		}
		seen[key] = true
		samples = append(samples, TexelSample{Board: pos.Board, Result: result})
	}

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// EPD datasets (e.g. Zurichess quiet-labeled.epd) label the result with a
		// `c9 "1-0"` opcode rather than a trailing float, and give a 4-field FEN
		// (no move counters). Translate those to a board + White-POV result.
		if i := strings.Index(line, `c9 "`); i >= 0 {
			fields := strings.Fields(line)
			if len(fields) < 4 {
				return nil, 0, fmt.Errorf("line %d: short EPD: %q", lineNo, line)
			}
			fen := strings.Join(fields[:4], " ") + " 0 1"
			r := line[i+len(`c9 "`):]
			j := strings.IndexByte(r, '"')
			if j < 0 {
				return nil, 0, fmt.Errorf("line %d: unterminated c9 opcode: %q", lineNo, line)
			}
			var result float64
			switch r[:j] {
			case "1-0":
				result = 1.0
			case "0-1":
				result = 0.0
			case "1/2-1/2":
				result = 0.5
			default:
				return nil, 0, fmt.Errorf("line %d: unknown c9 result %q", lineNo, r[:j])
			}
			pos, err := ParseFEN(fen)
			if err != nil {
				return nil, 0, fmt.Errorf("line %d: bad FEN %q: %w", lineNo, fen, err)
			}
			keep(pos, result)
			continue
		}
		// Result is the final whitespace-separated field; the FEN is the rest.
		idx := strings.LastIndexByte(line, ' ')
		if idx < 0 {
			return nil, 0, fmt.Errorf("line %d: no result field: %q", lineNo, line)
		}
		fen := strings.TrimSpace(line[:idx])
		result, perr := strconv.ParseFloat(strings.TrimSpace(line[idx+1:]), 64)
		if perr != nil {
			return nil, 0, fmt.Errorf("line %d: bad result: %w", lineNo, perr)
		}
		if math.IsNaN(result) || math.IsInf(result, 0) || result < 0 || result > 1 {
			return nil, 0, fmt.Errorf("line %d: result must be finite and in [0,1], got %q", lineNo, strings.TrimSpace(line[idx+1:]))
		}
		pos, err := ParseFEN(fen)
		if err != nil {
			return nil, 0, fmt.Errorf("line %d: bad FEN %q: %w", lineNo, fen, err)
		}
		keep(pos, result)
	}
	return samples, nDup, sc.Err()
}

// SplitTrainTest shuffles samples (seeded for reproducibility) and splits off a
// test fraction for early stopping. This legacy flat-data split has no game or
// opening identities and cannot guarantee related-game or board-input separation.
// Shuffles in place; use grouped data for an experiment requiring those guarantees.
func SplitTrainTest(samples []TexelSample, testFrac float64, seed int64) (train, test []TexelSample) {
	rng := rand.New(rand.NewSource(seed))
	rng.Shuffle(len(samples), func(i, j int) { samples[i], samples[j] = samples[j], samples[i] })
	nTest := int(float64(len(samples)) * testFrac)
	if nTest >= len(samples) {
		nTest = len(samples) - 1
	}
	if nTest < 0 {
		nTest = 0
	}
	return samples[nTest:], samples[:nTest]
}

// IsQuietPosition reports whether pos has no pending tactic — the property a Texel
// training sample needs for its static eval to be meaningful against the game
// result. Quiet ⇔ not in check AND a full-window quiescence search returns the
// static stand-pat (no capture sequence changes the score). A hanging piece or a
// winning capture makes qsearch diverge from the static eval, so training on such a
// position fits the eval to a number the static function structurally cannot output
// — the prior gendata filter (not-in-check only) let that noise poison the gradient.
//
// The filter deliberately does not call production qsearch. Its stand-pat includes
// halfmove-clock damping and correction histories, probes and populates the TT, and
// obeys the process-wide stop request. None of those are sample properties.
func IsQuietPosition(pos *Position) bool {
	_ = mustAcquireHCEModelUse()
	defer releaseHCEModelUse()
	if pos.IsInCheck() {
		return false
	}
	work := pos.Copy()
	static := texelStaticEval(work)
	q, resolved := texelQuietSearch(work, -INFINITY, INFINITY, 0)
	return resolved && q == static
}

// texelStaticEval is independent of search-only clock damping, correction history,
// the TT and the stop state.
func texelStaticEval(pos *Position) int {
	white := evaluatePublicWhiteLeased(&pos.Board)
	if pos.Turn() == White {
		return white + TempoBonus
	}
	return -white + TempoBonus
}

// hasLegalQuietPromotion catches the material-changing horizon move omitted by
// GenerateCapturesIntoBuffer. Capture-promotions are searched as captures.
func hasLegalQuietPromotion(pos *Position) bool {
	var moves [256]Move
	n := GenerateMovesIntoBuffer(pos, moves[:])
	for i := 0; i < n; i++ {
		move := moves[i]
		if move.PromoType() == NoType || move.IsCapture() {
			continue
		}
		ep, tag, hc, _ := pos.MakeMove(move)
		legal := !isInCheck(pos, move.MovingPiece().Color())
		pos.UnMakeMove(move, tag, ep, hc)
		if legal {
			return true
		}
	}
	return false
}

// texelQuietSearch is a bounded, capture-only negamax used solely to decide if a
// static-evaluation sample is tactically contaminated. It has no shared search
// state. Evasions are searched after a capture gives check so legal recaptures are
// classified correctly.
func texelQuietSearch(pos *Position, alpha, beta, depth int) (int, bool) {
	if depth >= 6 {
		// A capture chain at the horizon is not evidence that its stand-pat is
		// quiet. Reject it instead of classifying an unresolved exchange as calm.
		return 0, false
	}
	// Quiet promotions are not in the capture generator. Check every visited node,
	// not just the root: a capture sequence can uncover a later promotion.
	if hasLegalQuietPromotion(pos) {
		return 0, false
	}
	if pos.IsInCheck() {
		var moves [256]Move
		n := GenerateMovesIntoBuffer(pos, moves[:])
		best, legalMoves := -INFINITY, 0
		for i := 0; i < n; i++ {
			move := moves[i]
			if move.IsCastle() && !isLegalCastle(pos, move) {
				continue
			}
			ep, tag, hc, _ := pos.MakeMove(move)
			legal := !isInCheck(pos, move.MovingPiece().Color())
			if legal {
				legalMoves++
				score, resolved := texelQuietSearch(pos, -beta, -alpha, depth+1)
				if !resolved {
					pos.UnMakeMove(move, tag, ep, hc)
					return 0, false
				}
				score = -score
				if score > best {
					best = score
				}
				if score > alpha {
					alpha = score
				}
			}
			pos.UnMakeMove(move, tag, ep, hc)
			if alpha >= beta {
				return beta, true
			}
		}
		if legalMoves == 0 {
			return -MATE_VALUE + depth, true
		}
		return best, true
	}

	standPat := texelStaticEval(pos)
	if standPat >= beta {
		return beta, true
	}
	if standPat > alpha {
		alpha = standPat
	}
	var captures [64]Move
	n := GenerateCapturesIntoBuffer(pos, captures[:])
	for i := 0; i < n; i++ {
		move := captures[i]
		ep, tag, hc, _ := pos.MakeMove(move)
		legal := !isInCheck(pos, move.MovingPiece().Color())
		if legal {
			score, resolved := texelQuietSearch(pos, -beta, -alpha, depth+1)
			if !resolved {
				pos.UnMakeMove(move, tag, ep, hc)
				return 0, false
			}
			score = -score
			if score > alpha {
				alpha = score
			}
		}
		pos.UnMakeMove(move, tag, ep, hc)
		if alpha >= beta {
			return beta, true
		}
	}
	return alpha, true
}
