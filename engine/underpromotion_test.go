package engine

import "testing"

// TestUnderpromotionKnightForkFound guards promotion-safety in forward pruning.
// Promotions are forcing, material-changing moves and must be exempt from the
// quiet-move prunings (futility, LMP, history, LMR). Underpromotions are the
// vulnerable case: move ordering scores them 70000 (below captures), so they hit
// the count-gated prunings sooner than queen promotions, and the futility gate
// historically exempted ONLY queen promotions.
//
// White: Ke1, Rh1, Pe7. Black: Kc7, Qf6. The pawn promotes with e8=N+, forking
// the king (c7) and the queen (f6); after the king steps aside, Nxf6 wins the
// queen, leaving K+R+N vs K. The plain queen promotion e8=Q is NOT check and only
// wins a rook's worth (Black keeps the queen), so the knight UNDERpromotion is
// uniquely best.
//
// This is a BEHAVIORAL guard that underpromotions are generated, ordered, and
// preferred when winning — it does not isolate the deep-node pruning exemptions
// (the root searches every move, and the fork is too shallow to be missed by a
// reduction, so it passes with or without the gate fix). The exemptions
// themselves are mechanism-proven: a promotion is a forcing, material-changing
// move, so the quiet-move pruning premise is false — matching the already-correct
// quiet-SEE / quiet-tracking sites and standard engine practice. Non-regression is
// covered by perft (move generation is unchanged) and the full test suite.
func TestUnderpromotionKnightForkFound(t *testing.T) {
	pos, err := ParseFEN("8/2k1P3/5q2/8/8/8/8/4K2R w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	defaultSearchEngine, _ = NewSearchEngineWithHash(DEFAULT_CACHE_SIZE)
	ClearHistoryTable()
	ClearKillerMoves()
	ClearCounterMoves()

	info := SearchIterativeDeepening(pos, 10, nil)

	if info.BestMove.PromoType() != Knight {
		t.Errorf("best move = %v (promo type %v), want the knight underpromotion e8=N (the royal fork)",
			info.BestMove, info.BestMove.PromoType())
	}
	if info.BestScore < 300 {
		t.Errorf("best score = %d, want clearly winning (the fork wins the queen for K+R+N vs K)", info.BestScore)
	}
}
