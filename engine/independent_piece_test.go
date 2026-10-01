package engine

import "testing"

func TestIndependentPrimitivePieceValuesAndNames(t *testing.T) {
	wantPieces := []Piece{
		WhitePawn, WhiteKnight, WhiteBishop, WhiteRook, WhiteQueen, WhiteKing,
		BlackPawn, BlackKnight, BlackBishop, BlackRook, BlackQueen, BlackKing,
	}
	if len(Pieces) != len(wantPieces) {
		t.Fatalf("len(Pieces) = %d, want %d", len(Pieces), len(wantPieces))
	}
	for i, want := range wantPieces {
		if Pieces[i] != want {
			t.Errorf("Pieces[%d] = %d, want %d", i, Pieces[i], want)
		}
	}

	wantNames := []string{"P", "N", "B", "R", "Q", "K", "p", "n", "b", "r", "q", "k"}
	for i, want := range wantNames {
		piece := Pieces[i]
		if piece.Name() != want {
			t.Errorf("Piece(%d).Name() = %q, want %q", piece, piece.Name(), want)
		}
		if pieceFromName([]rune(want)[0]) != piece {
			t.Errorf("pieceFromName(%q) did not return %d", want, piece)
		}
	}
	for _, piece := range []Piece{NoPiece, Piece(13), Piece(-1)} {
		if piece.Name() != " " {
			t.Errorf("Piece(%d).Name() = %q, want space", piece, piece.Name())
		}
	}
	for _, r := range []rune{' ', 'x', 'X', '\x00'} {
		if pieceFromName(r) != NoPiece {
			t.Errorf("pieceFromName(%q) = %d, want NoPiece", r, pieceFromName(r))
		}
	}
}

func TestIndependentPrimitivePieceClassificationUsesLowNibble(t *testing.T) {
	tests := []struct {
		piece  Piece
		type_  PieceType
		color  Color
		weight int16
	}{
		{Piece(0), NoType, NoColor, 0},
		{Piece(1), Pawn, White, 100},
		{Piece(2), Knight, White, 320},
		{Piece(3), Bishop, White, 330},
		{Piece(4), Rook, White, 525},
		{Piece(5), Queen, White, 1000},
		{Piece(6), King, White, MAX_INT},
		{Piece(7), Pawn, Black, 100},
		{Piece(8), Knight, Black, 320},
		{Piece(9), Bishop, Black, 330},
		{Piece(10), Rook, Black, 525},
		{Piece(11), Queen, Black, 1000},
		{Piece(12), King, Black, MAX_INT},
		{Piece(13), NoType, Black, 0},
		{Piece(14), NoType, Black, 0},
		{Piece(15), NoType, Black, 0},
		{Piece(0x21), Pawn, White, 100},
		{Piece(-1), NoType, Black, 0},
	}
	for _, test := range tests {
		if got := test.piece.Type(); got != test.type_ {
			t.Errorf("Piece(%d).Type() = %d, want %d", test.piece, got, test.type_)
		}
		if got := test.piece.Color(); got != test.color {
			t.Errorf("Piece(%d).Color() = %d, want %d", test.piece, got, test.color)
		}
		if got := test.piece.Weight(); got != test.weight {
			t.Errorf("Piece(%d).Weight() = %d, want %d", test.piece, got, test.weight)
		}
		if got := test.piece.seeWeight(); got != test.weight {
			t.Errorf("Piece(%d).seeWeight() = %d, want %d", test.piece, got, test.weight)
		}
	}
}

func TestIndependentPrimitivePieceTypeNamesAndColors(t *testing.T) {
	for i, want := range []string{" ", "P", "N", "B", "R", "Q", "K", " "} {
		if got := PieceType(i).Name(); got != want {
			t.Errorf("PieceType(%d).Name() = %q, want %q", i, got, want)
		}
	}
	if Black.Other() != White || White.Other() != Black || NoColor.Other() != NoColor || Color(2).Other() != NoColor {
		t.Fatal("Color.Other returned an unexpected color")
	}
}

func TestIndependentPrimitiveGetPiecePreservesNonzeroInt8Inputs(t *testing.T) {
	for pieceType := Pawn; pieceType <= King; pieceType++ {
		if got := GetPiece(pieceType, White); got != Piece(pieceType) {
			t.Errorf("white %d -> %d, want %d", pieceType, got, Piece(pieceType))
		}
		if got := GetPiece(pieceType, Black); got != Piece(pieceType+6) {
			t.Errorf("black %d -> %d, want %d", pieceType, got, Piece(pieceType+6))
		}
	}
	if GetPiece(NoType, White) != NoPiece || GetPiece(Pawn, NoColor) != NoPiece || GetPiece(Pawn, Color(2)) != NoPiece {
		t.Fatal("unsupported type or color did not produce NoPiece")
	}

	unknown := PieceType(127)
	if got := GetPiece(unknown, Black); got != Piece(unknown+6) {
		t.Errorf("black %d -> %d, want int8-wrapped %d", unknown, got, Piece(unknown+6))
	}
	unknown = PieceType(-1)
	if got := GetPiece(unknown, White); got != Piece(unknown) {
		t.Errorf("white %d -> %d, want %d", unknown, got, Piece(unknown))
	}
}
