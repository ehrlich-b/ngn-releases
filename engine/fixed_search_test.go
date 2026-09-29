package engine

import (
	"testing"
)

// TestFixedSearch tests if the search fix resolved the evaluation bug
func TestFixedSearch(t *testing.T) {
	fen := "rnbqkb1r/pppp1ppp/5n2/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R w KQkq - 0 1"

	pos, err := ParseFEN(fen)
	if err != nil {
		t.Fatal("Failed to parse FEN:", err)
	}

	t.Logf("=== TESTING FIXED SEARCH ===")
	t.Logf("FEN: %s", fen)

	// Test individual moves using the search algorithm
	moves := GenerateLegalMoves(pos)

	type moveEval struct {
		move     Move
		score    int
		notation string
	}

	var moveEvals []moveEval

	// Evaluate each move using search
	for _, move := range moves {
		ep, tag, hc, _ := pos.MakeMove(move)

		// Use the search to evaluate this position
		searchInfo := SearchFixed(pos, 1, nil) // Depth 1 search
		score := -searchInfo.BestScore         // Negate because it's from opponent's perspective

		pos.UnMakeMove(move, tag, ep, hc)

		moveEvals = append(moveEvals, moveEval{
			move:     move,
			score:    score,
			notation: move.ToString(),
		})
	}

	// Find the capture move and some non-capture moves for comparison
	var captureMove, nonCaptureMove *moveEval

	for i := range moveEvals {
		if moveEvals[i].move.IsCapture() && captureMove == nil {
			captureMove = &moveEvals[i]
		}
		if !moveEvals[i].move.IsCapture() && nonCaptureMove == nil {
			nonCaptureMove = &moveEvals[i]
		}
	}

	if captureMove == nil {
		t.Fatal("No capture moves found")
	}

	if nonCaptureMove == nil {
		t.Fatal("No non-capture moves found")
	}

	t.Logf("Capture move %s: score %d", captureMove.notation, captureMove.score)
	t.Logf("Non-capture move %s: score %d", nonCaptureMove.notation, nonCaptureMove.score)

	if captureMove.score > nonCaptureMove.score {
		t.Logf("✅ FIXED! Capture move scores %d points better", captureMove.score-nonCaptureMove.score)
	} else {
		t.Logf("❌ Still broken: Capture move scores %d points worse", nonCaptureMove.score-captureMove.score)
	}

	// Test the specific f3xe5 capture
	f3e5Move := NewMove(F3, E5, WhiteKnight, BlackPawn, NoType, Capture)

	ep, tag, hc, _ := pos.MakeMove(f3e5Move)
	searchInfo := SearchFixed(pos, 1, nil)
	f3e5Score := -searchInfo.BestScore
	pos.UnMakeMove(f3e5Move, tag, ep, hc)

	t.Logf("Specific f3xe5 capture: score %d", f3e5Score)

	if f3e5Score > 0 {
		t.Logf("✅ f3xe5 capture is now evaluated positively!")
	} else {
		t.Logf("❌ f3xe5 capture still evaluated negatively: %d", f3e5Score)
	}
}
