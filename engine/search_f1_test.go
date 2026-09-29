package engine

import "testing"

// TestF1FutilityFalseDraw is a regression guard for the F1 bug: futility,
// SEE-capture, and SEE-quiet pruning used to `continue` BEFORE the legality
// check and `legalTried++`. At a non-PV, not-in-check, shallow node whose every
// move is quiet+futile (no TT move, no surviving capture, no queen promo), all
// moves were pruned, `legalTried` stayed 0, and the node fell into the
// checkmate/stalemate branch and returned 0 — a FALSE DRAW — when the true
// score is a large fail-low. (LMP and history-pruning already guard with
// legalTried; futility/SEE did not.)
//
// Position: black is down a queen for a pawn (true score ~ -870). A non-PV scout
// with a window placed well ABOVE that (alpha=-300, beta=-299) must FAIL LOW
// (return <= alpha), never report 0 (a false fail-high claiming a draw).
func TestF1FutilityFalseDraw(t *testing.T) {
	pos, err := ParseFEN("7Q/p7/8/8/8/5k2/8/4K3 b - - 0 1")
	if err != nil {
		t.Fatal(err)
	}

	ClearStop()
	ClearHistoryTable()
	ClearKillerMoves()

	var moveBuffer [256]Move
	var captureBuffer [64]Move
	var orderedBuffer [256]Move
	var seeGains [32]int
	info := &SearchInfo{
		control:       &defaultSearchEngine.worker.control,
		history:       &defaultSearchEngine.worker.history,
		evaluator:     testHCEWorkerEvaluator(nil),
		tt:            defaultTTForDirectTest(),
		Depth:         3,
		BestMove:      EmptyMove,
		BestScore:     -INFINITY,
		MoveBuffer:    &moveBuffer,
		CaptureBuffer: &captureBuffer,
		OrderedBuffer: &orderedBuffer,
		SEEGains:      &seeGains,
		Frames:        make([]searchFrame, searchFramePoolSize),
	}

	// non-PV, canNull — depth 3 so FUTILITY_MAX_DEPTH applies and the futile-node
	// condition (staticEval+margin <= alpha) holds for the queen-down side.
	score := alphaBetaPV(pos, 3, 1, -300, -299, false, true, false, info)
	if score > -299 {
		t.Fatalf("F1: non-PV futile node returned %d (a false draw / fail-high >= beta -299); "+
			"a clean queen-down position must fail low (<= -300)", score)
	}
}
