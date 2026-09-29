package engine

import "testing"

func TestParseUCIMoveRejectsIllegalMoves(t *testing.T) {
	pos, err := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	for _, move := range []string{
		"a1a8",                       // blocked rook
		"e7e5",                       // opponent's pawn
		"a3a4",                       // empty source
		"e2e5",                       // impossible pawn move
		"e1g1",                       // blocked castling
		"e2e4q", "e2e4x", "e2e4junk", // invalid promotion/suffix
		"0000", "", // no move
	} {
		t.Run(move, func(t *testing.T) {
			if parsed, err := ParseUCIMove(pos, move); err == nil {
				t.Fatalf("accepted illegal UCI move %q as %v", move, parsed)
			}
		})
	}
	if _, err := ParseUCIMove(pos, "e2e4"); err != nil {
		t.Fatal(err)
	}
}
