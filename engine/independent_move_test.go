package engine

import "testing"

func TestIndependentPrimitiveNewMovePacksFieldBoundaries(t *testing.T) {
	got := NewMove(Square(63), Square(0), Piece(15), Piece(15), PieceType(7), MoveTag(255))
	want := Move(63) | Move(0)<<6 | Move(15)<<12 | Move(15)<<16 | Move(7)<<20 | Move(255)<<23
	if got != want {
		t.Fatalf("NewMove boundary value = %#x, want %#x", got, want)
	}
	if got.Source() != 63 || got.Destination() != 0 || got.MovingPiece() != 15 || got.CapturedPiece() != 15 || got.PromoType() != 7 || got.Tag() != 255 {
		t.Fatal("boundary fields did not round-trip")
	}
}

func TestIndependentPrimitiveMoveRoundTripAndFlags(t *testing.T) {
	tag := KingSideCastle | QueenSideCastle | Capture | EnPassant
	move := NewMove(A2, H7, WhiteKnight, BlackBishop, Queen, tag)
	if move.Source() != A2 || move.Destination() != H7 || move.MovingPiece() != WhiteKnight || move.CapturedPiece() != BlackBishop || move.PromoType() != Queen || move.Tag() != tag {
		t.Fatal("valid move fields did not round-trip")
	}
	if !move.IsKingSideCastle() || !move.IsQueenSideCastle() || !move.IsCastle() || !move.IsCapture() || !move.IsEnPassant() {
		t.Fatal("move flags were not detected")
	}
	if NewMove(A1, A1, WhiteKing, NoPiece, NoType, 0).IsCastle() || NewMove(A1, A1, WhiteKing, NoPiece, NoType, 0).IsCapture() || NewMove(A1, A1, WhiteKing, NoPiece, NoType, 0).IsEnPassant() {
		t.Fatal("empty flags were detected")
	}
}

func TestIndependentPrimitiveOpaqueMoveExtractionAndUnusedBit(t *testing.T) {
	all := Move(0xffffffff)
	if all.Source() != 63 || all.Destination() != 63 || all.MovingPiece() != 15 || all.CapturedPiece() != 15 || all.PromoType() != 7 || all.Tag() != 255 {
		t.Fatal("opaque all-bit move did not extract masked fields")
	}
	if Move(1<<31).Tag() != 0 {
		t.Fatal("bit 31 was not discarded from Tag")
	}
	if move := Move(0x7f << 23); move.Tag() != 127 {
		t.Fatalf("opaque tag = %d, want 127", move.Tag())
	}
}

func TestIndependentPrimitiveMoveToString(t *testing.T) {
	if EmptyMove.ToString() != "a1a1" {
		t.Fatalf("EmptyMove.ToString() = %q, want a1a1", EmptyMove.ToString())
	}
	want := []string{"a1a1", "a1a1p", "a1a1n", "a1a1b", "a1a1r", "a1a1q", "a1a1k", "a1a1 "}
	for promotion, expected := range want {
		move := NewMove(A1, A1, NoPiece, NoPiece, PieceType(promotion), 0)
		if got := move.ToString(); got != expected {
			t.Errorf("promotion %d formats as %q, want %q", promotion, got, expected)
		}
	}
	if got := NewMove(B2, G7, WhitePawn, NoPiece, NoType, 0).ToString(); got != "b2g7" {
		t.Fatalf("ordinary move formats as %q, want b2g7", got)
	}
}
