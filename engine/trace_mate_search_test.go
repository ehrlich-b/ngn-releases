package engine

import (
	"testing"
)

// TestTraceMateSearch traces exactly what happens during mate search
func TestTraceMateSearch(t *testing.T) {
	// Same position as mate detection test
	pos, err := ParseFEN("7k/R7/6KP/8/8/8/8/8 w - - 0 1")
	if err != nil {
		t.Fatalf("Failed to create test position: %v", err)
	}

	t.Log("🔍 TRACING: Search evaluation at depth 1")
	t.Logf("Starting position: %s", GenerateFEN(pos))

	// Generate all legal moves to see what options are available
	legalMoves := GenerateLegalMoves(pos)
	t.Logf("Legal moves available: %d", len(legalMoves))

	for i, move := range legalMoves {
		moveStr := MoveToAlgebraic(move)
		t.Logf("  %d: %s", i+1, moveStr)

		// Make the move and evaluate resulting position
		oldEP, oldTag, oldHalfClock, _ := pos.MakeMove(move)

		// Check what the evaluation would be from this position at depth 0
		eval := Evaluate(&pos.Board)
		t.Logf("     Static eval after move: %d cp", eval)

		// Check if opponent has legal moves (to detect mate)
		opponentMoves := GenerateLegalMoves(pos)
		inCheck := pos.IsInCheck()

		t.Logf("     Opponent in check: %t", inCheck)
		t.Logf("     Opponent legal moves: %d", len(opponentMoves))

		if inCheck && len(opponentMoves) == 0 {
			t.Logf("     ✅ THIS IS CHECKMATE! Should score near +MATE_VALUE")
			expectedScore := MATE_VALUE - 1 // Mate in 1
			t.Logf("     Expected score: +%d (mate in 1)", expectedScore)
		} else if len(opponentMoves) == 0 {
			t.Logf("     ⚠️  This is stalemate (score: 0)")
		} else {
			t.Logf("     Regular position")
		}

		// Unmake the move
		pos.UnMakeMove(move, oldTag, oldEP, oldHalfClock)

		if moveStr == "a7a8" {
			t.Logf("     ^^^^ THIS IS THE MATE MOVE THAT ENGINE SHOULD CHOOSE! ^^^^")
		}
		t.Logf("")
	}

	t.Log("🤔 The question is: Why doesn't search prioritize the mate move?")
	t.Log("   Answer: The search function needs to return +MATE_VALUE when opponent has no legal moves in check")
}
