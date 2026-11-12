package engine

// W2QCapture enables learned capture ordering in quiescence. Keep default play
// unchanged until the search experiment has completed its strength gate.
var W2QCapture = 0

func (h *workerHistory) wave2QCaptureScore(move, ttMove Move) int {
	if move == ttMove {
		return 1 << 30
	}
	score := 8*int(move.CapturedPiece().seeWeight()) - int(move.MovingPiece().seeWeight())
	if W2QCapture == 1 && move.IsCapture() {
		bonus := h.captureHistoryScore(move) / 8
		if bonus < -512 {
			bonus = -512
		} else if bonus > 512 {
			bonus = 512
		}
		score += bonus
	}
	return score
}
