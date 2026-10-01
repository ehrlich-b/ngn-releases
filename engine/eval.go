package engine

const TempoBonus = 10
const FiftyMoveDampBudget = 256

func Evaluate(board *Bitboard) int {
	_ = mustAcquireHCEModelUse()
	defer releaseHCEModelUse()
	return evaluatePublicWhiteLeased(board)
}

func evaluatePublicWhiteLeased(board *Bitboard) int {
	work := board.copy()
	work.recomputeAccumulator()
	return evaluateWhiteLeased(&work)
}

func evaluateWhiteLeased(board *Bitboard) int {
	return WrapEvaluation("Evaluate", func() int { return evaluateRaw(board) }, board)
}

func EvaluateForPlayer(board *Bitboard, player Color) int {
	_ = mustAcquireHCEModelUse()
	defer releaseHCEModelUse()
	score := evaluatePublicWhiteLeased(board)
	if player != White {
		score = -score
	}
	return score + TempoBonus
}

func EvaluateForPlayerCached(pos *Position) int {
	return defaultSearchEngine.evaluateForPlayerCached(pos)
}

func evaluateUnsafe(board *Bitboard) int { return evaluateRaw(board) }

var qsearchLazyMargin = 100000

func evaluateLazyStandPat(board *Bitboard, player Color, beta int) (int, bool) {
	score := evaluateRaw(board)
	if player != White {
		score = -score
	}
	return score + TempoBonus, false
}

const evalCacheBits = 20

type evalCacheEntry struct {
	key uint64
	val int32
	_   int32
}

type hceEvaluator struct {
	full           [1 << evalCacheBits]evalCacheEntry
	seenGeneration uint64
}

func (e *hceEvaluator) clearForGeneration(generation uint64) {
	clear(e.full[:])
	e.seenGeneration = generation
}

// SearchSTM is the ordinary search score route. Its operation order is the
// cached-white value, rule-50 attenuation, side-to-move POV, then tempo.
func (e *hceEvaluator) SearchSTM(pos *Position) int {
	h := pos.Hash()
	entry := &e.full[h&(1<<evalCacheBits-1)]
	var white int
	if entry.key == h && h != 0 {
		white = int(entry.val)
	} else {
		white = evaluateWhiteLeased(&pos.Board)
		entry.key = h
		entry.val = int32(white)
	}
	white = white * (FiftyMoveDampBudget - int(pos.HalfMoveClock)) / FiftyMoveDampBudget
	if pos.Turn() == White {
		return white + TempoBonus
	}
	return -white + TempoBonus
}

// LegacyUndampedSTM preserves the three alphaBeta safety-return semantics: a
// fresh full HCE value (the pawn-count cache may hit), draw scaling, POV, Tempo,
// and no full-cache lookup or halfmove attenuation.
func (e *hceEvaluator) LegacyUndampedSTM(pos *Position) int {
	white := evaluateWhiteLeased(&pos.Board)
	if pos.Turn() == White {
		return white + TempoBonus
	}
	return -white + TempoBonus
}
