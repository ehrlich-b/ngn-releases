package engine

// wave2GivesCheck reports whether move attacks the opposing king after all
// occupancy changes. The input must be a pseudo-legal, correctly encoded NGN
// move on board (including its capture, castling and promotion fields).
// It does not validate the move or test the mover's own king safety; search
// must perform that legality check separately. No board, position metadata,
// hash or evaluator state is changed.
func wave2GivesCheck(board *Bitboard, move Move) bool {
	moving := move.MovingPiece()
	color := moving.Color()
	king := board.PieceOccupancy(GetPiece(King, color.Other()))
	if king == 0 {
		return false
	}
	kingSquare := bitScanForward(king)
	from, to := SquareMask[move.Source()], SquareMask[move.Destination()]
	occupied := (board.WhiteOccupancy()|board.BlackOccupancy())&^from | to

	// Only the mover's piece masks are needed for attacks. A normal capture
	// leaves the destination occupied, while en passant also vacates its victim.
	pieces := board.pieces
	pieces[moving] &^= from
	placed := moving
	if move.PromoType() != NoType {
		placed = GetPiece(move.PromoType(), color)
	}
	pieces[placed] |= to
	if move.IsEnPassant() {
		occupied &^= SquareMask[enPassantVictimSquare(move)]
	}
	if move.IsCastle() {
		rookFrom, rookTo := H1, F1
		if move.IsQueenSideCastle() {
			rookFrom, rookTo = A1, D1
		}
		if color == Black {
			rookFrom ^= 56
			rookTo ^= 56
		}
		remove, add := SquareMask[rookFrom], SquareMask[rookTo]
		rook := GetPiece(Rook, color)
		pieces[rook] = pieces[rook]&^remove | add
		occupied = occupied&^remove | add
	}

	pawnAttackers := WhitePawnAttackers[kingSquare]
	if color == Black {
		pawnAttackers = BlackPawnAttackers[kingSquare]
	}
	return pawnAttackers&pieces[GetPiece(Pawn, color)] != 0 ||
		KnightAttacks[kingSquare]&pieces[GetPiece(Knight, color)] != 0 ||
		KingAttacks[kingSquare]&pieces[GetPiece(King, color)] != 0 ||
		GetBishopAttacks(int(kingSquare), occupied)&(pieces[GetPiece(Bishop, color)]|pieces[GetPiece(Queen, color)]) != 0 ||
		GetRookAttacks(int(kingSquare), occupied)&(pieces[GetPiece(Rook, color)]|pieces[GetPiece(Queen, color)]) != 0
}
