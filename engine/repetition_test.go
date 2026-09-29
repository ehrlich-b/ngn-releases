package engine

import "testing"

// TestRootExplorationDoesNotInflateRepetitionCount guards the root-repetition
// fix. The game-history map (pos.Positions) must count only positions ACTUALLY
// PLAYED; the root search loop must therefore explore candidate moves with
// MakeMove (the search make), not GameMakeMove (the game make).
//
// The old code used GameMakeMove at the root, which incremented Positions[child]
// for the very position the ply-1 child then read. So a SECOND occurrence (true
// game count 1) looked like count 2, and the child's "count >= 2 == threefold"
// check (search.go) declared a bogus draw one occurrence early.
//
// Setup: White is in check with exactly one legal, reversible reply (Kh1-h2)
// into a position where White is down a rook and a knight. We seed the
// game-history count of the post-move position and run a depth-1 search so the
// ply-1 child's draw detection is exercised directly:
//   - seeded count 1 (this is the 2nd occurrence): NOT a draw. The search must
//     return the real, losing eval. Pre-fix the root inflated this to 2 and the
//     child returned 0.
//   - seeded count 2 (this move makes the 3rd occurrence): a real threefold. The
//     search must return 0.
func TestRootExplorationDoesNotInflateRepetitionCount(t *testing.T) {
	// Black: Kh8, Rg3, Nf2. White: Kh1 (in check from Nf2). HMC=10 so the only
	// reply Kh1-h2 stays reversible and the child's HalfMoveClock>=ply gate holds.
	const fen = "7k/8/8/8/8/6r1/5n2/7K w - - 10 1"

	// Hash of the position reached after the single legal root move.
	childHash := func() uint64 {
		pos, err := ParseFEN(fen)
		if err != nil {
			t.Fatal(err)
		}
		moves := GenerateLegalMoves(pos)
		if len(moves) != 1 {
			t.Fatalf("setup: expected exactly one legal move (Kh1-h2), got %d", len(moves))
		}
		ep, tag, hc, _ := pos.MakeMove(moves[0])
		h := pos.Hash()
		pos.UnMakeMove(moves[0], tag, ep, hc)
		return h
	}()

	search := func(seededCount int) int {
		defaultSearchEngine, _ = NewSearchEngineWithHash(DEFAULT_CACHE_SIZE)
		ClearHistoryTable()
		ClearKillerMoves()
		ClearCounterMoves()
		pos, err := ParseFEN(fen)
		if err != nil {
			t.Fatal(err)
		}
		pos.Positions[childHash] = seededCount
		return SearchIterativeDeepening(pos, 1, nil).BestScore
	}

	// 2nd occurrence (one prior real occurrence): must NOT be scored as a draw.
	if got := search(1); got > -300 {
		t.Errorf("second occurrence scored %d; expected the real losing eval (<= -300), "+
			"not a bogus draw — root exploration is inflating the repetition count", got)
	}
	// 3rd occurrence (two prior real occurrences): a genuine threefold draw.
	if got := search(2); got != 0 {
		t.Errorf("third occurrence scored %d; expected a threefold draw (0)", got)
	}
}
