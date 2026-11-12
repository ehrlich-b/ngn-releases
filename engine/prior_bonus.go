package engine

// rewardPriorQuiet implements the fail-low prior-move idea using NGN's own
// bounded history response: when every reply fails to improve the original
// alpha, the opponent's preceding quiet move is a positive example. Piece
// encodings include color, so the update belongs to the side that played it.
func (info *SearchInfo) rewardPriorQuiet(depth, ply, origAlpha, bestScore int) {
	if T1104PriorBonus == 0 || info.Stopped || bestScore > origAlpha || ply < 1 {
		return
	}
	previous := info.MoveStack[ply-1]
	if !h1QuietMove(previous) || previous == info.priorBonusTTStack[ply-1] {
		return
	}
	predecessor := EmptyMove
	if ply >= 2 {
		predecessor = info.MoveStack[ply-2]
	}
	// Reuse policy 11's depth-squared, capped learning rate. This signal
	// rewards main and one-ply continuation history, without a follow-up label.
	info.history.h1UpdateQuiet(previous, predecessor, EmptyMove, h1HistoryRate(depth), h1HistoryTarget)
}
