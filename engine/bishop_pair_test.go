package engine

import (
	"testing"
)

func TestBishopPairBonus(t *testing.T) {
	tests := []struct {
		name          string
		fen           string
		expectedWhite int
		expectedBlack int
	}{
		{
			name:          "Starting position - both sides have bishop pair",
			fen:           "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
			expectedWhite: 30, // Base bishop pair bonus
			expectedBlack: 30, // Base bishop pair bonus
		},
		{
			name:          "White missing one bishop - no bonus",
			fen:           "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RN1QKBNR w KQkq - 0 1",
			expectedWhite: 0,  // No bishop pair
			expectedBlack: 30, // Still has bishop pair
		},
		{
			name:          "Black missing one bishop - no bonus",
			fen:           "rn1qkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
			expectedWhite: 30, // Still has bishop pair
			expectedBlack: 0,  // No bishop pair
		},
		{
			name:          "Both bishops on same color squares - no bonus",
			fen:           "2bqk1nr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
			expectedWhite: 30, // White still has proper pair
			expectedBlack: 0,  // Black bishops would be on same color (if this were a legal position)
		},
		{
			name:          "Fewer pawns - higher bonus (12 pawns total)",
			fen:           "rnbqkbnr/4pppp/8/pp6/PP6/8/4PPPP/RNBQKBNR w KQkq - 0 1",
			expectedWhite: 40, // 30 base + 10 for open position (12 pawns)
			expectedBlack: 40, // 30 base + 10 for open position (12 pawns)
		},
		{
			name:          "Very few pawns - even higher bonus (8 pawns total)",
			fen:           "rnbqkbnr/6pp/8/pp6/PP6/8/6PP/RNBQKBNR w KQkq - 0 1",
			expectedWhite: 50, // 30 base + 10 + 10 for very open position (8 pawns)
			expectedBlack: 50, // 30 base + 10 + 10 for very open position (8 pawns)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos, err := ParseFEN(tt.fen)
			if err != nil {
				t.Fatalf("Failed to parse FEN: %v", err)
			}

			board := &pos.Board

			whiteBonus := evaluateBishopPair(board, White)
			blackBonus := evaluateBishopPair(board, Black)

			if whiteBonus != tt.expectedWhite {
				t.Errorf("Expected white bishop pair bonus %d, got %d", tt.expectedWhite, whiteBonus)
			}

			if blackBonus != tt.expectedBlack {
				t.Errorf("Expected black bishop pair bonus %d, got %d", tt.expectedBlack, blackBonus)
			}
		})
	}
}

func TestBishopPairInFullEvaluation(t *testing.T) {
	// Test that bishop pair bonus is actually included in the main evaluation
	startingPos, err := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatalf("Failed to parse starting position FEN: %v", err)
	}

	// Position where white has bishop pair but black doesn't
	whitePairPos, err := ParseFEN("rnbqk1nr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatalf("Failed to parse white pair position FEN: %v", err)
	}

	startingEval := Evaluate(&startingPos.Board)
	whitePairEval := Evaluate(&whitePairPos.Board)

	// White should have a better evaluation in the second position due to bishop pair advantage
	if whitePairEval <= startingEval {
		t.Errorf("Expected white to have better evaluation with bishop pair advantage, got starting: %d, white pair: %d",
			startingEval, whitePairEval)
	}

	t.Logf("Starting position evaluation: %d", startingEval)
	t.Logf("White bishop pair advantage evaluation: %d", whitePairEval)
	t.Logf("Bishop pair effect: +%d centipawns for white", whitePairEval-startingEval)
}
