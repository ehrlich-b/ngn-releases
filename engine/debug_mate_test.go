package engine

import (
	"testing"
)

// TestDebugMateEvaluation analyzes what happens when engine evaluates Ra8#
func TestDebugMateEvaluation(t *testing.T) {
	// Same position as the mate detection test
	pos, err := ParseFEN("7k/R7/6KP/8/8/8/8/8 w - - 0 1")
	if err != nil {
		t.Fatalf("Failed to create test position: %v", err)
	}

	t.Log("🔍 DEBUGGING: What happens when engine evaluates Ra8#?")
	t.Logf("Starting position: %s", GenerateFEN(pos))

	// Create the Ra8# move manually
	ra8Move, err := ParseAlgebraicMove("a7a8", pos)
	if err != nil || ra8Move == EmptyMove {
		t.Fatalf("Failed to parse Ra8 move: %v", err)
	}

	t.Logf("Evaluating move: %s", MoveToAlgebraic(ra8Move))

	// Make the move and see what position we get
	oldEP, oldTag, oldHalfClock, _ := pos.MakeMove(ra8Move)

	t.Logf("Position after Ra8: %s", GenerateFEN(pos))

	// Check if black is in check
	inCheck := pos.IsInCheck()
	t.Logf("Black in check: %t", inCheck)

	// Generate legal moves for black
	legalMoves := GenerateLegalMoves(pos)
	t.Logf("Legal moves for black: %d", len(legalMoves))

	for i, move := range legalMoves {
		t.Logf("  %d: %s", i+1, MoveToAlgebraic(move))
	}

	// This should be checkmate: black in check with 0 legal moves
	if inCheck && len(legalMoves) == 0 {
		t.Logf("✅ CONFIRMED: This is checkmate position!")
		t.Logf("   Black is in check with no legal moves")
	} else if inCheck {
		t.Errorf("❌ Black is in check but has %d legal moves (not mate)", len(legalMoves))
	} else {
		t.Errorf("❌ Black is not even in check (invalid test position)")
	}

	// Now let's see what the static evaluation says about this position
	eval := Evaluate(&pos.Board)
	t.Logf("Static evaluation of checkmate position: %d cp", eval)

	// The problem: static evaluation doesn't recognize checkmate!
	// It just sees material advantage instead of detecting mate

	// Let's also test what a depth-1 search would return from this position
	if len(legalMoves) == 0 && inCheck {
		t.Logf("✅ This position should return -MATE_VALUE in search")
		t.Logf("   Expected search result: %d (checkmate)", -MATE_VALUE)
	}

	// Unmake the move to restore original position
	pos.UnMakeMove(ra8Move, oldTag, oldEP, oldHalfClock)
}
