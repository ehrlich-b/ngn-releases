package engine

const (
	h1HistoryTarget  = historyMax
	h1HistoryDivisor = 512
	h1HistoryRateCap = 400
)

// HistoryPolicyName identifies the accepted policy-11 implementation. It is
// retained in production as a diagnostic and regression-test witness.
func HistoryPolicyName() string {
	switch {
	case h1NewHistoryProducer && h1NormalizedLMRConsumer:
		return "11"
	case h1NewHistoryProducer:
		return "10"
	case h1NormalizedLMRConsumer:
		return "01"
	default:
		return "00"
	}
}

func h1QuietMove(move Move) bool {
	return move != EmptyMove && !move.IsCapture() && move.PromoType() == NoType
}

func h1HistoryRate(depth int) int {
	if depth < 0 {
		depth = -depth
	}
	rate := depth * depth
	if rate > h1HistoryRateCap {
		rate = h1HistoryRateCap
	}
	return rate
}

func h1HistoryResponse(entry, target, rate int) int {
	return entry + int((int64(target)-int64(entry))*int64(rate)/h1HistoryDivisor)
}

func h1HistoryResponseUpdate(entry *int, target, rate int) {
	*entry = h1HistoryResponse(*entry, target, rate)
}

func (h *workerHistory) h1UpdateQuiet(move, prev, prev2 Move, rate, target int) {
	if !h1QuietMove(move) {
		return
	}
	piece := move.MovingPiece()
	to := move.Destination()
	h1HistoryResponseUpdate(&h.historyTable[piece][to], target, rate)
	if prev != EmptyMove {
		h1HistoryResponseUpdate(&h.continuationHistory[prev.MovingPiece()][prev.Destination()][piece][to], target, rate)
	}
	if prev2 != EmptyMove {
		h1HistoryResponseUpdate(&h.followupHistory[prev2.MovingPiece()][prev2.Destination()][piece][to], target, rate)
	}
}

// h1UpdateCompletedNode learns exactly once from a completed node's final
// quiet winner. Only searched quiets before that winner are negative examples.
func (h *workerHistory) h1UpdateCompletedNode(searchedQuiets []Move, bestMove Move, prev, prev2 Move, depth, origAlpha, bestScore int) {
	if bestScore <= origAlpha || !h1QuietMove(bestMove) {
		return
	}
	winnerIndex := -1
	for index, move := range searchedQuiets {
		if move == bestMove {
			winnerIndex = index
			break
		}
	}
	if winnerIndex < 0 {
		return
	}
	rate := h1HistoryRate(depth)
	h.h1UpdateQuiet(bestMove, prev, prev2, rate, h1HistoryTarget)
	for _, move := range searchedQuiets[:winnerIndex] {
		h.h1UpdateQuiet(move, prev, prev2, rate, -h1HistoryTarget)
	}
}

func h1NormalizedLMRTerm(score int) int {
	value := score / 2500
	if value < -2 {
		value = -2
	} else if value > 2 {
		value = 2
	}
	return -value
}
