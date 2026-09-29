package engine

import (
	"testing"
)

// TestSimpleBug - minimal test to reproduce the evaluation bug
func TestSimpleBug(t *testing.T) {
	fen := "rnbqkb1r/pppp1ppp/5n2/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R w KQkq - 0 1"

	pos, err := ParseFEN(fen)
	if err != nil {
		t.Fatal("Failed to parse FEN:", err)
	}

	t.Logf("=== SIMPLE BUG TEST ===")

	// Evaluation before
	evalBefore := EvaluateForPlayer(&pos.Board, White)
	t.Logf("Eval before: %d", evalBefore)

	// Make the f3xe5 capture manually
	move := NewMove(F3, E5, WhiteKnight, BlackPawn, NoType, Capture)
	ep, tag, hc, _ := pos.MakeMove(move)

	// Evaluation after (from white's perspective)
	evalAfterWhite := EvaluateForPlayer(&pos.Board, White)
	t.Logf("Eval after (white perspective): %d", evalAfterWhite)

	change := evalAfterWhite - evalBefore
	t.Logf("Change: %d", change)

	// The bug: change is -233 when it should be roughly +100
	if change < -200 {
		t.Logf("❌ BUG CONFIRMED: Change is %d, expected around +100", change)
	} else {
		t.Logf("✅ No bug found")
	}

	pos.UnMakeMove(move, tag, ep, hc)

	// Now let's test if the issue is specifically with capturing on e5
	// Try moving the knight somewhere else first
	t.Logf("\n--- Testing knight to different square ---")

	alternativeMove := NewMove(F3, G5, WhiteKnight, NoPiece, NoType, 0) // f3 to g5
	ep2, tag2, hc2, _ := pos.MakeMove(alternativeMove)

	evalAfterWhite2 := EvaluateForPlayer(&pos.Board, White)
	change2 := evalAfterWhite2 - evalBefore

	t.Logf("Knight to g5 change: %d", change2)

	pos.UnMakeMove(alternativeMove, tag2, ep2, hc2)

	// Test if there's something specifically wrong with e5
	t.Logf("\n--- Testing other captures ---")

	// See if we can find any other captures to test
	allMoves := GenerateLegalMoves(pos)
	captureCount := 0

	for _, testMove := range allMoves {
		if testMove.IsCapture() {
			captureCount++
			t.Logf("Testing capture: %s", testMove.ToString())

			ep3, tag3, hc3, _ := pos.MakeMove(testMove)
			evalAfter3 := EvaluateForPlayer(&pos.Board, White)
			change3 := evalAfter3 - evalBefore

			t.Logf("  Change: %d", change3)

			pos.UnMakeMove(testMove, tag3, ep3, hc3)
		}
	}

	if captureCount == 0 {
		t.Logf("No other captures available to test")
	}
}
