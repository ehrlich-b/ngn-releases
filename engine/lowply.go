package engine

const lowPlyCount = 5

// Low-ply history specializes the ordinary history idea to distance from the
// root. It uses NGN's existing bounded depth-squared update; the table is read
// only by quiet move ordering, never by pruning, reductions or evaluation.
func (h *workerHistory) updateLowPly(move Move, ply, depth int) {
	if T1104LowPly == 0 || ply < 0 || ply >= lowPlyCount || depth <= 0 ||
		move == EmptyMove || move.IsCapture() || move.IsEnPassant() ||
		move.PromoType() != NoType || move.IsCastle() {
		return
	}
	gravityUpdate(&h.lowPlyHistory[ply][move.Source()][move.Destination()], depth*depth)
}
