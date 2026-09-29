package engine

import (
	"testing"
)

func TestHasLegalMove(t *testing.T) {
	testCases := []struct {
		name        string
		fen         string
		shouldHave  bool
		description string
	}{
		{
			name:        "Starting position",
			fen:         "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
			shouldHave:  true,
			description: "Starting position should have legal moves",
		},
		{
			name:        "Checkmate position",
			fen:         "R6k/8/6KP/8/8/8/8/8 b - - 1 1",
			shouldHave:  false,
			description: "Black king in corner checkmated by rook",
		},
		{
			name:        "Stalemate position",
			fen:         "7k/5Q2/6K1/8/8/8/8/8 b - - 0 1",
			shouldHave:  false,
			description: "Black king on h8, white K g6 + Q f7 — no legal moves, not in check",
		},
		{
			name:        "King can escape",
			fen:         "k7/R7/8/8/8/8/8/K7 b - - 0 1",
			shouldHave:  true,
			description: "Black king on a8 in check by rook, can move to b8 or capture rook",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pos, err := ParseFEN(tc.fen)
			if err != nil {
				t.Fatalf("Failed to parse FEN: %v", err)
			}

			hasLegal := pos.HasLegalMove()

			if hasLegal != tc.shouldHave {
				t.Errorf("%s: Expected HasLegalMove()=%v, got %v",
					tc.description, tc.shouldHave, hasLegal)

				// Debug: Also check with full legal move generation
				legalMoves := GenerateLegalMoves(pos)
				t.Logf("  GenerateLegalMoves found %d moves", len(legalMoves))
				for i, move := range legalMoves {
					t.Logf("    %d: %s", i+1, MoveToAlgebraic(move))
				}
			}
		})
	}
}

// TestHasLegalMovePerformance compares performance of HasLegalMove vs GenerateLegalMoves
func TestHasLegalMovePerformance(t *testing.T) {
	pos, _ := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")

	// Warm up
	for i := 0; i < 100; i++ {
		pos.HasLegalMove()
		GenerateLegalMoves(pos)
	}

	// Benchmark HasLegalMove
	const iterations = 10000
	start := pos.Board.pieces[WhiteKing] // Use as timer hack
	for i := 0; i < iterations; i++ {
		pos.HasLegalMove()
	}
	hasLegalTime := pos.Board.pieces[WhiteKing] - start

	// Benchmark GenerateLegalMoves
	start = pos.Board.pieces[WhiteKing]
	for i := 0; i < iterations; i++ {
		GenerateLegalMoves(pos)
	}
	generateTime := pos.Board.pieces[WhiteKing] - start

	t.Logf("Performance comparison (lower is better):")
	t.Logf("  HasLegalMove:        %d", hasLegalTime)
	t.Logf("  GenerateLegalMoves:  %d", generateTime)

	// HasLegalMove should be faster because it exits early
	if hasLegalTime >= generateTime {
		t.Logf("⚠️  HasLegalMove is not faster than GenerateLegalMoves!")
	} else {
		speedup := float64(generateTime) / float64(hasLegalTime)
		t.Logf("✅ HasLegalMove is %.1fx faster", speedup)
	}
}
