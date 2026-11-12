package engine

// Separate the optional first-layer qsearch bounds from capture-only bounds.
const wave2QChecksTTKey uint64 = 0xd72c8e4a3b1965f0

// appendWave2QChecks appends only legal quiet non-promotion checks. Captures
// (including en passant) and all promotions already belong to the noisy prefix.
// Both buffers have full move capacity (256), as in the ordinary move generator;
// filtering disjoint classes cannot exceed that full pseudo-legal move count.
func appendWave2QChecks(pos *Position, moves, scratch []Move) int {
	n := GenerateMovesIntoBuffer(pos, scratch)
	count := 0
	for _, move := range scratch[:n] {
		if move.IsCapture() || move.IsEnPassant() || move.PromoType() != NoType ||
			!wave2GivesCheck(&pos.Board, move) {
			continue
		}
		if move.IsCastle() {
			if !isLegalCastle(pos, move) {
				continue
			}
		} else {
			// Board-only legality probe leaves hash, history and evaluator alone.
			pos.partialMakeMove(move)
			legal := !isInCheck(pos, move.MovingPiece().Color())
			pos.partialUnMakeMove(move)
			if !legal {
				continue
			}
		}
		moves[count] = move
		count++
	}
	return count
}
