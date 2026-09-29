package engine

import (
	"testing"
)

func TestMoveCreation(t *testing.T) {
	tests := []struct {
		from          Square
		to            Square
		movingPiece   Piece
		capturedPiece Piece
		promoType     PieceType
		tag           MoveTag
		name          string
	}{
		{E2, E4, WhitePawn, NoPiece, NoType, 0, "Pawn Move"},
		{E1, G1, WhiteKing, NoPiece, NoType, KingSideCastle, "King Side Castle"},
		{E1, C1, WhiteKing, NoPiece, NoType, QueenSideCastle, "Queen Side Castle"},
		{E4, D5, WhitePawn, BlackPawn, NoType, Capture, "Pawn Capture"},
		{E5, D6, WhitePawn, BlackPawn, NoType, EnPassant | Capture, "En Passant"},
		{E7, E8, WhitePawn, NoPiece, Queen, 0, "Pawn Promotion"},
		{E7, D8, WhitePawn, BlackRook, Knight, Capture, "Pawn Promotion Capture"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			move := NewMove(tt.from, tt.to, tt.movingPiece, tt.capturedPiece, tt.promoType, tt.tag)

			if move.Source() != tt.from {
				t.Errorf("%s Source() = %v; want %v", tt.name, move.Source(), tt.from)
			}
			if move.Destination() != tt.to {
				t.Errorf("%s Destination() = %v; want %v", tt.name, move.Destination(), tt.to)
			}
			if move.MovingPiece() != tt.movingPiece {
				t.Errorf("%s MovingPiece() = %v; want %v", tt.name, move.MovingPiece(), tt.movingPiece)
			}
			if move.CapturedPiece() != tt.capturedPiece {
				t.Errorf("%s CapturedPiece() = %v; want %v", tt.name, move.CapturedPiece(), tt.capturedPiece)
			}
			if move.PromoType() != tt.promoType {
				t.Errorf("%s PromoType() = %v; want %v", tt.name, move.PromoType(), tt.promoType)
			}
			if move.Tag() != tt.tag {
				t.Errorf("%s Tag() = %v; want %v", tt.name, move.Tag(), tt.tag)
			}
		})
	}
}

func TestMoveFlags(t *testing.T) {
	tests := []struct {
		move          Move
		isKingCastle  bool
		isQueenCastle bool
		isCastle      bool
		isCapture     bool
		isEnPassant   bool
		name          string
	}{
		{
			NewMove(E2, E4, WhitePawn, NoPiece, NoType, 0),
			false, false, false, false, false,
			"Regular Pawn Move",
		},
		{
			NewMove(E1, G1, WhiteKing, NoPiece, NoType, KingSideCastle),
			true, false, true, false, false,
			"King Side Castle",
		},
		{
			NewMove(E1, C1, WhiteKing, NoPiece, NoType, QueenSideCastle),
			false, true, true, false, false,
			"Queen Side Castle",
		},
		{
			NewMove(E4, D5, WhitePawn, BlackPawn, NoType, Capture),
			false, false, false, true, false,
			"Regular Capture",
		},
		{
			NewMove(E5, D6, WhitePawn, BlackPawn, NoType, EnPassant|Capture),
			false, false, false, true, true,
			"En Passant Capture",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.move.IsKingSideCastle() != tt.isKingCastle {
				t.Errorf("%s IsKingSideCastle() = %v; want %v", tt.name, tt.move.IsKingSideCastle(), tt.isKingCastle)
			}
			if tt.move.IsQueenSideCastle() != tt.isQueenCastle {
				t.Errorf("%s IsQueenSideCastle() = %v; want %v", tt.name, tt.move.IsQueenSideCastle(), tt.isQueenCastle)
			}
			if tt.move.IsCastle() != tt.isCastle {
				t.Errorf("%s IsCastle() = %v; want %v", tt.name, tt.move.IsCastle(), tt.isCastle)
			}
			if tt.move.IsCapture() != tt.isCapture {
				t.Errorf("%s IsCapture() = %v; want %v", tt.name, tt.move.IsCapture(), tt.isCapture)
			}
			if tt.move.IsEnPassant() != tt.isEnPassant {
				t.Errorf("%s IsEnPassant() = %v; want %v", tt.name, tt.move.IsEnPassant(), tt.isEnPassant)
			}
		})
	}
}

func TestMoveToString(t *testing.T) {
	tests := []struct {
		move     Move
		expected string
		name     string
	}{
		{
			NewMove(E2, E4, WhitePawn, NoPiece, NoType, 0),
			"e2e4",
			"Regular Move",
		},
		{
			NewMove(E1, G1, WhiteKing, NoPiece, NoType, KingSideCastle),
			"e1g1",
			"Castle Move",
		},
		{
			NewMove(E7, E8, WhitePawn, NoPiece, Queen, 0),
			"e7e8q",
			"Promotion to Queen",
		},
		{
			NewMove(E7, D8, WhitePawn, BlackRook, Knight, Capture),
			"e7d8n",
			"Promotion Capture to Knight",
		},
		{
			NewMove(A7, A8, BlackPawn, NoPiece, Bishop, 0),
			"a7a8b",
			"Black Promotion to Bishop",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.move.ToString()
			if result != tt.expected {
				t.Errorf("%s ToString() = %q; want %q", tt.name, result, tt.expected)
			}
		})
	}
}

func TestEmptyMove(t *testing.T) {
	empty := EmptyMove

	if empty != Move(0) {
		t.Errorf("EmptyMove should be Move(0), got %v", empty)
	}

	// Test that empty move has all zero values
	if empty.Source() != Square(0) {
		t.Errorf("EmptyMove Source() should be 0, got %v", empty.Source())
	}
	if empty.Destination() != Square(0) {
		t.Errorf("EmptyMove Destination() should be 0, got %v", empty.Destination())
	}
	if empty.MovingPiece() != Piece(0) {
		t.Errorf("EmptyMove MovingPiece() should be 0, got %v", empty.MovingPiece())
	}
	if empty.CapturedPiece() != Piece(0) {
		t.Errorf("EmptyMove CapturedPiece() should be 0, got %v", empty.CapturedPiece())
	}
	if empty.PromoType() != PieceType(0) {
		t.Errorf("EmptyMove PromoType() should be 0, got %v", empty.PromoType())
	}
	if empty.Tag() != MoveTag(0) {
		t.Errorf("EmptyMove Tag() should be 0, got %v", empty.Tag())
	}
}

func TestMoveTagCombinations(t *testing.T) {
	// Test that multiple tags can be combined
	combinedTag := Capture | EnPassant
	move := NewMove(E5, D6, WhitePawn, BlackPawn, NoType, combinedTag)

	if !move.IsCapture() {
		t.Error("Move with Capture|EnPassant should be a capture")
	}
	if !move.IsEnPassant() {
		t.Error("Move with Capture|EnPassant should be en passant")
	}
	if move.IsCastle() {
		t.Error("Move with Capture|EnPassant should not be a castle")
	}
}

func BenchmarkNewMove(b *testing.B) {
	BenchmarkFunction(b, func() {
		NewMove(E2, E4, WhitePawn, NoPiece, NoType, 0)
	})
}

func BenchmarkMoveSource(b *testing.B) {
	move := NewMove(E2, E4, WhitePawn, NoPiece, NoType, 0)
	BenchmarkFunction(b, func() {
		_ = move.Source()
	})
}

func BenchmarkMoveDestination(b *testing.B) {
	move := NewMove(E2, E4, WhitePawn, NoPiece, NoType, 0)
	BenchmarkFunction(b, func() {
		_ = move.Destination()
	})
}

func BenchmarkMoveIsCapture(b *testing.B) {
	move := NewMove(E4, D5, WhitePawn, BlackPawn, NoType, Capture)
	BenchmarkFunction(b, func() {
		_ = move.IsCapture()
	})
}

func BenchmarkMoveToString(b *testing.B) {
	move := NewMove(E7, E8, WhitePawn, NoPiece, Queen, 0)
	BenchmarkFunction(b, func() {
		_ = move.ToString()
	})
}
