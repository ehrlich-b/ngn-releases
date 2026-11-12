package engine

const pawnQuietBuckets = 512

func pawnQuietEligible(move Move) bool {
	return h1QuietMove(move) && !move.IsEnPassant() && !move.IsCastle()
}

// The existing board-only pawn hash supplies context, without sharing learned
// entries with evaluation correction. Colored moving pieces distinguish sides.
func pawnQuietIndex(pos *Position) int {
	return pawnCorrectionIndex(pos.Board.pieces[WhitePawn], pos.Board.pieces[BlackPawn]) & (pawnQuietBuckets - 1)
}

// Update only a completed quiet winner and the completed quiets before it.
// The parent position must be restored before this call: a pawn move's result
// is credited to the structure in which it was chosen, not its child structure.
func (h *workerHistory) updatePawnQuiets(pos *Position, searched []Move, best Move, depth, originalAlpha, score int) {
	if score <= originalAlpha || !pawnQuietEligible(best) {
		return
	}
	winner := -1
	for i, move := range searched {
		if move == best {
			winner = i
			break
		}
	}
	if winner < 0 {
		return
	}
	index := pawnQuietIndex(pos)
	rate := h1HistoryRate(depth)
	update := func(move Move, target int) {
		if !pawnQuietEligible(move) {
			return
		}
		entry := &h.pawnQuietHistory[index][move.MovingPiece()][move.Destination()]
		*entry = int16(h1HistoryResponse(int(*entry), target, rate))
	}
	update(best, historyMax)
	for _, move := range searched[:winner] {
		update(move, -historyMax)
	}
}
