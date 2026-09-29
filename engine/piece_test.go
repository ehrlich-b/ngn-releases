package engine

import (
	"testing"
)

func TestPieceCreation(t *testing.T) {
	tests := []struct {
		pieceType PieceType
		color     Color
		expected  Piece
		name      string
	}{
		{Pawn, White, WhitePawn, "White Pawn"},
		{Knight, White, WhiteKnight, "White Knight"},
		{Bishop, White, WhiteBishop, "White Bishop"},
		{Rook, White, WhiteRook, "White Rook"},
		{Queen, White, WhiteQueen, "White Queen"},
		{King, White, WhiteKing, "White King"},
		{Pawn, Black, BlackPawn, "Black Pawn"},
		{Knight, Black, BlackKnight, "Black Knight"},
		{Bishop, Black, BlackBishop, "Black Bishop"},
		{Rook, Black, BlackRook, "Black Rook"},
		{Queen, Black, BlackQueen, "Black Queen"},
		{King, Black, BlackKing, "Black King"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			piece := GetPiece(tt.pieceType, tt.color)
			if piece != tt.expected {
				t.Errorf("GetPiece(%v, %v) = %v; want %v", tt.pieceType, tt.color, piece, tt.expected)
			}
		})
	}
}

// TestPieceEnumLayout locks the contiguous White-then-Black Piece layout that several
// hot-path speed optimizations encode arithmetically: GetPiece collapses to Piece(type)
// for White and +6 for Black; isInCheck and findLeastValuableAttacker index enemy pieces
// as WhiteX+offset; Clear/Move/UpdateSquare pick the color aggregate with `piece < BlackPawn`.
// If the enum is ever reordered this fails loudly here instead of silently corrupting
// movegen/eval (which only the deterministic node-count check would otherwise catch).
func TestPieceEnumLayout(t *testing.T) {
	// White pieces are Pawn..King = 1..6, Black pieces are Pawn..King = 7..12.
	if WhitePawn != 1 || WhiteKing != 6 || BlackPawn != 7 || BlackKing != 12 {
		t.Fatalf("Piece enum bands moved: WhitePawn=%d WhiteKing=%d BlackPawn=%d BlackKing=%d; "+
			"want 1/6/7/12 (GetPiece/isInCheck/SEE/make-unmake arithmetic depends on this)",
			WhitePawn, WhiteKing, BlackPawn, BlackKing)
	}
	for pt := Pawn; pt <= King; pt++ {
		if Piece(pt) != GetPiece(pt, White) {
			t.Errorf("Piece(%v)=%d != GetPiece(%v,White)=%d (white piece must equal its type)",
				pt, Piece(pt), pt, GetPiece(pt, White))
		}
		if Piece(pt)+6 != GetPiece(pt, Black) {
			t.Errorf("Piece(%v)+6=%d != GetPiece(%v,Black)=%d (black piece must be white+6)",
				pt, Piece(pt)+6, pt, GetPiece(pt, Black))
		}
		// The `piece < BlackPawn` color test (Clear/Move/UpdateSquare) must split colors.
		if GetPiece(pt, White) >= BlackPawn {
			t.Errorf("white %v >= BlackPawn breaks the `piece < BlackPawn` aggregate test", pt)
		}
		if GetPiece(pt, Black) < BlackPawn {
			t.Errorf("black %v < BlackPawn breaks the `piece < BlackPawn` aggregate test", pt)
		}
	}
}

func TestPieceProperties(t *testing.T) {
	tests := []struct {
		piece         Piece
		expectedType  PieceType
		expectedColor Color
		name          string
	}{
		{WhitePawn, Pawn, White, "White Pawn Properties"},
		{BlackQueen, Queen, Black, "Black Queen Properties"},
		{WhiteKing, King, White, "White King Properties"},
		{NoPiece, NoType, NoColor, "No Piece Properties"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.piece.Type() != tt.expectedType {
				t.Errorf("%s Type() = %v; want %v", tt.name, tt.piece.Type(), tt.expectedType)
			}
			if tt.piece.Color() != tt.expectedColor {
				t.Errorf("%s Color() = %v; want %v", tt.name, tt.piece.Color(), tt.expectedColor)
			}
		})
	}
}

func TestPieceValues(t *testing.T) {
	tests := []struct {
		piece    Piece
		minValue int16
		name     string
	}{
		{WhitePawn, 80, "White Pawn Value"},
		{BlackPawn, 80, "Black Pawn Value"},
		{WhiteKnight, 300, "White Knight Value"},
		{BlackBishop, 300, "Black Bishop Value"},
		{WhiteRook, 450, "White Rook Value"},
		{BlackQueen, 900, "Black Queen Value"},
		{WhiteKing, 0, "White King Value"},
		{NoPiece, 0, "No Piece Value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := tt.piece.Weight()
			if value < tt.minValue {
				t.Errorf("%s Weight() = %v; want at least %v", tt.name, value, tt.minValue)
			}
		})
	}
}

func TestColorOther(t *testing.T) {
	tests := []struct {
		color    Color
		expected Color
		name     string
	}{
		{White, Black, "White.Other()"},
		{Black, White, "Black.Other()"},
		{NoColor, NoColor, "NoColor.Other()"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.color.Other()
			if result != tt.expected {
				t.Errorf("%s = %v; want %v", tt.name, result, tt.expected)
			}
		})
	}
}

func TestPieceNames(t *testing.T) {
	tests := []struct {
		piece    Piece
		expected string
		name     string
	}{
		{WhitePawn, "P", "White Pawn Name"},
		{BlackPawn, "p", "Black Pawn Name"},
		{WhiteKnight, "N", "White Knight Name"},
		{BlackKnight, "n", "Black Knight Name"},
		{WhiteBishop, "B", "White Bishop Name"},
		{BlackBishop, "b", "Black Bishop Name"},
		{WhiteRook, "R", "White Rook Name"},
		{BlackRook, "r", "Black Rook Name"},
		{WhiteQueen, "Q", "White Queen Name"},
		{BlackQueen, "q", "Black Queen Name"},
		{WhiteKing, "K", "White King Name"},
		{BlackKing, "k", "Black King Name"},
		{NoPiece, " ", "No Piece Name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.piece.Name()
			if result != tt.expected {
				t.Errorf("%s Name() = %q; want %q", tt.name, result, tt.expected)
			}
		})
	}
}

func BenchmarkGetPiece(b *testing.B) {
	BenchmarkFunction(b, func() {
		GetPiece(Pawn, White)
	})
}

func BenchmarkPieceColor(b *testing.B) {
	piece := WhiteQueen
	BenchmarkFunction(b, func() {
		_ = piece.Color()
	})
}

func BenchmarkPieceType(b *testing.B) {
	piece := BlackRook
	BenchmarkFunction(b, func() {
		_ = piece.Type()
	})
}

func BenchmarkPieceWeight(b *testing.B) {
	piece := WhiteQueen
	BenchmarkFunction(b, func() {
		_ = piece.Weight()
	})
}
