package engine

import (
	"testing"
)

// TestEvaluationPerspective tests if evaluation perspective is handled correctly
func TestEvaluationPerspective(t *testing.T) {
	fen := "rnbqkb1r/pppp1ppp/5n2/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R w KQkq - 0 1"

	pos, err := ParseFEN(fen)
	if err != nil {
		t.Fatal("Failed to parse FEN:", err)
	}

	t.Logf("=== EVALUATION PERSPECTIVE TEST ===")

	// Before: White to move
	beforeEval := Evaluate(&pos.Board)
	t.Logf("Before (white to move): %d", beforeEval)
	t.Logf("This means: position is worth %d centipawns for white", beforeEval)

	// Make the capture f3xe5
	move := NewMove(F3, E5, WhiteKnight, BlackPawn, NoType, Capture)
	ep, tag, hc, _ := pos.MakeMove(move)

	// After: Black to move
	afterEval := Evaluate(&pos.Board)
	t.Logf("After (black to move): %d", afterEval)
	t.Logf("This means: position is worth %d centipawns for black", afterEval)
	t.Logf("From white's perspective: %d", -afterEval)

	// The CORRECT way to compare:
	// Before: position worth +0 for white (white to move)
	// After: position worth +233 for black, so -233 for white (black to move)
	// Improvement for white: -233 - 0 = -233

	// But this is WRONG logic! We captured a pawn, so white should be better off

	// Let me test this by looking at how search does it:
	t.Logf("\n--- TESTING HOW SEARCH HANDLES THIS ---")

	// Simulate what search does:
	// 1. Evaluate current position (white perspective)
	currentEval := beforeEval
	t.Logf("Current position (white's turn): %d", currentEval)

	// 2. Make move
	pos.MakeMove(move)

	// 3. Evaluate after move from white's perspective
	moveEval := EvaluateForPlayer(&pos.Board, White)
	t.Logf("After move f3xe5, negated eval: %d", moveEval)
	t.Logf("This represents the value of this move for white")

	pos.UnMakeMove(move, tag, ep, hc)

	if moveEval > currentEval {
		t.Logf("✅ Move improves position by %d", moveEval-currentEval)
	} else {
		t.Logf("❌ Move worsens position by %d", currentEval-moveEval)
	}

	// Now let's test a different move for comparison
	t.Logf("\n--- TESTING NON-CAPTURE MOVE ---")

	nonCaptureMove := NewMove(F3, G5, WhiteKnight, NoPiece, NoType, 0) // f3 to g5

	pos.MakeMove(nonCaptureMove)
	nonCaptureMoveEval := EvaluateForPlayer(&pos.Board, White)
	t.Logf("After move f3g5, white eval: %d", nonCaptureMoveEval)
	pos.UnMakeMove(nonCaptureMove, tag, ep, hc)

	t.Logf("Capture move value: %d", moveEval)
	t.Logf("Non-capture move value: %d", nonCaptureMoveEval)

	if moveEval > nonCaptureMoveEval {
		t.Logf("✅ Capture is better by %d", moveEval-nonCaptureMoveEval)
	} else {
		t.Logf("❌ BUG: Capture is worse by %d", nonCaptureMoveEval-moveEval)
	}
}
