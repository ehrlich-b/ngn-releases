package engine

// canCaptureEnPassant retains raw FEN targets adjacent to a pawn. Legality,
// including king exposure, is checked separately for the repetition hash.
func canCaptureEnPassant(pos *Position) bool {
	target := pos.EnPassant
	if target < A1 || target > H8 {
		return false
	}
	if pos.Turn() == White && target.Rank() == Rank6 {
		return WhitePawnAttackers[target]&pos.Board.pieces[WhitePawn] != 0
	}
	if pos.Turn() == Black && target.Rank() == Rank3 {
		return BlackPawnAttackers[target]&pos.Board.pieces[BlackPawn] != 0
	}
	return false
}
