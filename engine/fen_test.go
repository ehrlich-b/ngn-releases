package engine

import (
	"testing"
)

func TestFENParsing(t *testing.T) {
	tests := []struct {
		name        string
		fen         string
		expectError bool
	}{
		{
			name: "Starting position",
			fen:  "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		},
		{
			name: "After e4",
			fen:  "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1",
		},
		{
			name: "After e4 e5",
			fen:  "rnbqkbnr/pppp1ppp/8/4p3/4P3/8/PPPP1PPP/RNBQKBNR w KQkq e6 0 2",
		},
		{
			name: "No castling rights",
			fen:  "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w - - 0 1",
		},
		{
			name: "Only white kingside castling",
			fen:  "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w K - 0 1",
		},
		{
			name:        "Invalid FEN - too few parts",
			fen:         "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq",
			expectError: true,
		},
		{
			name:        "Invalid FEN - bad active color",
			fen:         "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR x KQkq - 0 1",
			expectError: true,
		},
		{
			name:        "Invalid FEN - bad piece",
			fen:         "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBXR w KQkq - 0 1",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos, err := ParseFEN(tt.fen)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for FEN: %s", tt.fen)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error parsing FEN: %v", err)
				return
			}

			if pos == nil {
				t.Error("Position should not be nil")
				return
			}

			// Test that we can generate FEN back
			generatedFEN := GenerateFEN(pos)
			if generatedFEN == "" {
				t.Error("Generated FEN should not be empty")
			}
		})
	}
}

func TestFENStartingPosition(t *testing.T) {
	startingFEN := "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
	pos, err := ParseFEN(startingFEN)

	if err != nil {
		t.Fatalf("Error parsing starting FEN: %v", err)
	}

	// Check piece placement
	if pos.Board.PieceAt(E1) != WhiteKing {
		t.Error("White king should be on e1")
	}
	if pos.Board.PieceAt(E8) != BlackKing {
		t.Error("Black king should be on e8")
	}
	if pos.Board.PieceAt(A1) != WhiteRook {
		t.Error("White rook should be on a1")
	}
	if pos.Board.PieceAt(H8) != BlackRook {
		t.Error("Black rook should be on h8")
	}

	// Check turn
	if pos.Turn() != White {
		t.Error("Should be white to move")
	}

	// Check castling rights
	if !pos.HasTag(WhiteCanCastleKingSide) {
		t.Error("White should be able to castle kingside")
	}
	if !pos.HasTag(WhiteCanCastleQueenSide) {
		t.Error("White should be able to castle queenside")
	}
	if !pos.HasTag(BlackCanCastleKingSide) {
		t.Error("Black should be able to castle kingside")
	}
	if !pos.HasTag(BlackCanCastleQueenSide) {
		t.Error("Black should be able to castle queenside")
	}

	// Check en passant
	if pos.EnPassant != NoSquare {
		t.Error("No en passant square should be set")
	}

	// Check halfmove clock
	if pos.HalfMoveClock != 0 {
		t.Error("Halfmove clock should be 0")
	}
}

func TestFENGeneration(t *testing.T) {
	// Test that parsing and generating FEN is symmetric
	testFENs := []string{
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		"rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1",
		"rnbqkbnr/pppp1ppp/8/4p3/4P3/8/PPPP1PPP/RNBQKBNR w KQkq e6 0 2",
		"r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1",
		"8/8/8/8/8/8/8/8 w - - 50 25",
	}

	for _, originalFEN := range testFENs {
		pos, err := ParseFEN(originalFEN)
		if err != nil {
			t.Errorf("Error parsing FEN %s: %v", originalFEN, err)
			continue
		}

		generatedFEN := GenerateFEN(pos)

		// Parse the generated FEN to make sure it's valid
		pos2, err := ParseFEN(generatedFEN)
		if err != nil {
			t.Errorf("Generated FEN is invalid %s: %v", generatedFEN, err)
			continue
		}

		// Check that positions are equivalent
		if pos.Turn() != pos2.Turn() {
			t.Errorf("Turn mismatch for FEN %s", originalFEN)
		}

		if pos.EnPassant != pos2.EnPassant {
			t.Errorf("En passant mismatch for FEN %s", originalFEN)
		}

		if pos.HalfMoveClock != pos2.HalfMoveClock {
			t.Errorf("Halfmove clock mismatch for FEN %s", originalFEN)
		}
	}
}

func TestParseSquare(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    Square
		expectError bool
	}{
		{"a1", "a1", A1, false},
		{"e4", "e4", E4, false},
		{"h8", "h8", H8, false},
		{"d5", "d5", D5, false},
		{"invalid length", "e", NoSquare, true},
		{"invalid file", "i4", NoSquare, true},
		{"invalid rank", "e9", NoSquare, true},
		{"invalid format", "4e", NoSquare, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseSquare(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for input: %s", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func BenchmarkParseFEN(b *testing.B) {
	fen := "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseFEN(fen)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGenerateFEN(b *testing.B) {
	fen := "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1"
	pos, err := ParseFEN(fen)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GenerateFEN(pos)
	}
}
