package engine

import (
	"testing"
)

func TestAlgebraicMoveParser(t *testing.T) {
	// Start with the starting position
	pos := newUCIStartingPosition()

	tests := []struct {
		name         string
		moveStr      string
		expectError  bool
		expectedFrom Square
		expectedTo   Square
	}{
		{
			name:         "Pawn move e2e4",
			moveStr:      "e2e4",
			expectedFrom: E2,
			expectedTo:   E4,
		},
		{
			name:         "Pawn move d2d3",
			moveStr:      "d2d3",
			expectedFrom: D2,
			expectedTo:   D3,
		},
		{
			name:         "Knight move g1f3",
			moveStr:      "g1f3",
			expectedFrom: G1,
			expectedTo:   F3,
		},
		{
			name:         "Uppercase move E2E4",
			moveStr:      "E2E4",
			expectedFrom: E2,
			expectedTo:   E4,
		},
		{
			name:        "Invalid length",
			moveStr:     "e2",
			expectError: true,
		},
		{
			name:        "Invalid square",
			moveStr:     "e2i9",
			expectError: true,
		},
		{
			name:        "No piece on square",
			moveStr:     "e3e4",
			expectError: true,
		},
		{
			name:        "Illegal move",
			moveStr:     "e2e5",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			move, err := ParseAlgebraicMove(tt.moveStr, pos)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for move: %s", tt.moveStr)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if move.Source() != tt.expectedFrom {
				t.Errorf("Expected from %v, got %v", tt.expectedFrom, move.Source())
			}

			if move.Destination() != tt.expectedTo {
				t.Errorf("Expected to %v, got %v", tt.expectedTo, move.Destination())
			}
		})
	}
}

func TestAlgebraicMoveCapture(t *testing.T) {
	// Create a position with a capture available
	pos := &Position{}
	pos.Board = Bitboard{}
	pos.Board.UpdateSquare(E1, WhiteKing, NoPiece)
	pos.Board.UpdateSquare(E8, BlackKing, NoPiece)
	pos.Board.UpdateSquare(E4, WhitePawn, NoPiece)
	pos.Board.UpdateSquare(D5, BlackPawn, NoPiece)
	pos.SetTag(WhiteToMove)
	pos.EnPassant = NoSquare

	// Test pawn capture
	move, err := ParseAlgebraicMove("e4d5", pos)
	if err != nil {
		t.Fatalf("Error parsing capture move: %v", err)
	}

	if !move.IsCapture() {
		t.Error("Move should be marked as capture")
	}

	if move.CapturedPiece() != BlackPawn {
		t.Error("Should capture black pawn")
	}
}

func TestAlgebraicMovePromotion(t *testing.T) {
	// Create a position where white pawn can promote
	pos := &Position{}
	pos.Board = Bitboard{}
	pos.Board.UpdateSquare(A1, WhiteKing, NoPiece) // Move king away from e1
	pos.Board.UpdateSquare(A8, BlackKing, NoPiece) // Move black king away from e8
	pos.Board.UpdateSquare(E7, WhitePawn, NoPiece) // White pawn ready to promote
	pos.SetTag(WhiteToMove)
	pos.EnPassant = NoSquare

	tests := []struct {
		name          string
		moveStr       string
		expectedPromo PieceType
	}{
		{"Promote to queen", "e7e8q", Queen},
		{"Promote to rook", "e7e8r", Rook},
		{"Promote to bishop", "e7e8b", Bishop},
		{"Promote to knight", "e7e8n", Knight},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			move, err := ParseAlgebraicMove(tt.moveStr, pos)
			if err != nil {
				t.Errorf("Error parsing promotion move: %v", err)
				return
			}

			if move.PromoType() != tt.expectedPromo {
				t.Errorf("Expected promotion to %v, got %v", tt.expectedPromo, move.PromoType())
			}
		})
	}
}

func TestAlgebraicMoveCastling(t *testing.T) {
	// Create castling position
	pos := &Position{}
	pos.Board = Bitboard{}
	pos.Board.UpdateSquare(E1, WhiteKing, NoPiece)
	pos.Board.UpdateSquare(A1, WhiteRook, NoPiece)
	pos.Board.UpdateSquare(H1, WhiteRook, NoPiece)
	pos.Board.UpdateSquare(E8, BlackKing, NoPiece)
	pos.SetTag(WhiteToMove)
	pos.SetTag(WhiteCanCastleKingSide)
	pos.SetTag(WhiteCanCastleQueenSide)
	pos.EnPassant = NoSquare

	// Test kingside castling
	move, err := ParseAlgebraicMove("e1g1", pos)
	if err != nil {
		t.Errorf("Error parsing kingside castle: %v", err)
		return
	}

	if !move.IsKingSideCastle() {
		t.Error("Move should be marked as kingside castle")
	}

	// Test queenside castling
	move, err = ParseAlgebraicMove("e1c1", pos)
	if err != nil {
		t.Errorf("Error parsing queenside castle: %v", err)
		return
	}

	if !move.IsQueenSideCastle() {
		t.Error("Move should be marked as queenside castle")
	}
}

func TestAlgebraicMoveEnPassant(t *testing.T) {
	// Create en passant position
	pos := &Position{}
	pos.Board = Bitboard{}
	pos.Board.UpdateSquare(E1, WhiteKing, NoPiece)
	pos.Board.UpdateSquare(E8, BlackKing, NoPiece)
	pos.Board.UpdateSquare(E5, WhitePawn, NoPiece)
	pos.Board.UpdateSquare(D5, BlackPawn, NoPiece) // This pawn just moved d7-d5
	pos.SetTag(WhiteToMove)
	pos.EnPassant = D6 // En passant square behind the pawn

	move, err := ParseAlgebraicMove("e5d6", pos)
	if err != nil {
		t.Errorf("Error parsing en passant move: %v", err)
		return
	}

	if !move.IsEnPassant() {
		t.Error("Move should be marked as en passant")
	}

	if !move.IsCapture() {
		t.Error("En passant should also be marked as capture")
	}

	if move.CapturedPiece() != BlackPawn {
		t.Error("Should capture black pawn in en passant")
	}
}

func TestMoveToAlgebraic(t *testing.T) {
	tests := []struct {
		name     string
		from     Square
		to       Square
		piece    Piece
		captured Piece
		promo    PieceType
		tag      MoveTag
		expected string
	}{
		{
			name:     "Regular move",
			from:     E2,
			to:       E4,
			piece:    WhitePawn,
			expected: "e2e4",
		},
		{
			name:     "Capture",
			from:     E4,
			to:       D5,
			piece:    WhitePawn,
			captured: BlackPawn,
			tag:      Capture,
			expected: "e4d5",
		},
		{
			name:     "Promotion to queen",
			from:     E7,
			to:       E8,
			piece:    WhitePawn,
			promo:    Queen,
			expected: "e7e8q",
		},
		{
			name:     "Promotion with capture",
			from:     E7,
			to:       D8,
			piece:    WhitePawn,
			captured: BlackRook,
			promo:    Knight,
			tag:      Capture,
			expected: "e7d8n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			move := NewMove(tt.from, tt.to, tt.piece, tt.captured, tt.promo, tt.tag)
			result := MoveToAlgebraic(move)

			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestValidateAlgebraicMove(t *testing.T) {
	pos := newUCIStartingPosition()

	// Valid move
	err := ValidateAlgebraicMove("e2e4", pos)
	if err != nil {
		t.Errorf("Valid move should not return error: %v", err)
	}

	// Invalid move
	err = ValidateAlgebraicMove("e2e5", pos)
	if err == nil {
		t.Error("Invalid move should return error")
	}
}

func TestAlgebraicMoveSequence(t *testing.T) {
	// Test a sequence of moves
	pos := newUCIStartingPosition()

	moves := []string{
		"e2e4", "e7e5", "g1f3", "b8c6", "f1c4", "f8c5",
	}

	for i, moveStr := range moves {
		move, err := ParseAlgebraicMove(moveStr, pos)
		if err != nil {
			t.Errorf("Error parsing move %d (%s): %v", i+1, moveStr, err)
			break
		}

		// Make the move
		_, _, _, _ = pos.MakeMove(move)

		// Verify the move was legal by checking we can generate it
		legalMoves := GenerateLegalMoves(pos)
		if len(legalMoves) == 0 {
			t.Errorf("No legal moves after move %d", i+1)
			break
		}
	}
}

func BenchmarkParseAlgebraicMove(b *testing.B) {
	pos := newUCIStartingPosition()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseAlgebraicMove("e2e4", pos)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMoveToAlgebraic(b *testing.B) {
	move := NewMove(E2, E4, WhitePawn, NoPiece, NoType, 0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MoveToAlgebraic(move)
	}
}
