package engine

import "testing"

// TestSearchFixedMatchesProductionScore is the "SearchFixed authority" guard.
// SearchFixed is a diagnostic-only fixed-depth driver that shares alphaBetaPV with
// the production SearchIterativeDeepening. SearchFixed computes the exact FULL-WIDTH
// minimax value; production's PVS/LMR/aspiration approximate it within a few cp.
// So the two must agree on the best move and track within search noise — a large
// score gap or a different best move means the diagnostic no longer reflects
// production semantics (e.g. a repetition/extension/stop divergence in the shared
// alphaBetaPV).
func TestSearchFixedMatchesProductionScore(t *testing.T) {
	// White to move wins the undefended black queen with Rf1xf3 (the f-file is
	// clear), reaching a winning K+R vs K.
	const fen = "5k2/8/8/8/8/5q2/8/4KR2 w - - 0 1"
	const depth = 6
	// PVS/LMR/aspiration make production's score an approximation of the full-width
	// minimax that SearchFixed computes; a small cp gap is expected, a large one is
	// a real divergence.
	const scoreTol = 25

	fresh := func() *Position {
		pos, err := ParseFEN(fen)
		if err != nil {
			t.Fatal(err)
		}
		defaultSearchEngine, _ = NewSearchEngineWithHash(DEFAULT_CACHE_SIZE)
		ClearHistoryTable()
		ClearKillerMoves()
		ClearCounterMoves()
		return pos
	}

	prod := SearchIterativeDeepening(fresh(), depth, nil)
	diag := SearchFixed(fresh(), depth, nil)

	if diff := prod.BestScore - diag.BestScore; diff < -scoreTol || diff > scoreTol {
		t.Errorf("score gap too large at depth %d: production=%d SearchFixed=%d (diff %d, tol %d) — diagnostic diverged beyond the PVS/LMR approximation",
			depth, prod.BestScore, diag.BestScore, diff, scoreTol)
	}
	if prod.BestMove != diag.BestMove {
		t.Errorf("best-move mismatch at depth %d: production=%v SearchFixed=%v", depth, prod.BestMove, diag.BestMove)
	}
	if diag.BestScore < 300 {
		t.Errorf("expected a clearly winning score (Rxf3 wins the queen), got %d", diag.BestScore)
	}
}
