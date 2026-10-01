package engine

import (
	"reflect"
	"testing"
)

func TestEnPassantSecondCapturerCanBeLegal(t *testing.T) {
	tests := []struct {
		name, start, push, noEP, legalMove string
	}{
		{"white", "2r3k1/3p4/8/2P1P3/8/8/8/2K5 b - - 0 1", "d7d5", "2r3k1/8/8/2PpP3/8/8/8/2K5 w - - 0 2", "e5d6"},
		{"black", "2k5/8/8/8/2p1p3/8/3P4/2R3K1 w - - 0 1", "d2d4", "2k5/8/8/8/2pPp3/8/8/2R3K1 b - - 0 1", "e4d3"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pos := mustParseEnPassantPosition(t, test.start)
			mustPlayEnPassantMove(t, pos, test.push, false)
			before := snapshotEnPassantPosition(pos)
			var moves []string
			for _, move := range GenerateLegalMoves(pos) {
				if move.IsEnPassant() {
					moves = append(moves, move.ToString())
				}
			}
			if !reflect.DeepEqual(moves, []string{test.legalMove}) {
				t.Fatalf("legal EP moves = %v, want only %s", moves, test.legalMove)
			}
			var legal bool
			allocations := testing.AllocsPerRun(100, func() { legal = hasLegalEnPassant(pos) })
			if !legal || allocations != 0 {
				t.Fatalf("second EP capturer legal=%v allocations=%v, want true/0", legal, allocations)
			}
			assertEnPassantPositionRestored(t, pos, before)
			if pos.Hash() == mustParseEnPassantPosition(t, test.noEP).Hash() {
				t.Fatal("legal second EP capturer did not distinguish the ordinary key")
			}
		})
	}
}

func TestEnPassantNullDescendantRestoresKey(t *testing.T) {
	for _, fixture := range enPassantRepetitionFixtures {
		for _, cold := range []bool{false, true} {
			name := fixture.name + "/warm"
			if cold {
				name = fixture.name + "/cold_child"
			}
			t.Run(name, func(t *testing.T) {
				pos := mustParseEnPassantPosition(t, fixture.startFEN)
				mustPlayEnPassantMove(t, pos, fixture.push, true)
				before := snapshotEnPassantPosition(pos)
				nullEP := pos.MakeNullMove()
				nullBefore := snapshotEnPassantPosition(pos)
				var child Move
				found := false
				for _, move := range GenerateLegalMoves(pos) {
					piece := move.MovingPiece()
					if piece != WhitePawn && piece != BlackPawn && !move.IsCapture() && !move.IsCastle() {
						child, found = move, true
						break
					}
				}
				if !found {
					t.Fatal("fixture has no legal quiet non-pawn null descendant")
				}
				if cold {
					pos.hash = 0
				}
				ep, tag, clock, ok := pos.MakeMove(child)
				if !ok {
					t.Fatal("legal quiet descendant rejected")
				}
				if pos.Hash() != mustParseEnPassantPosition(t, GenerateFEN(pos)).Hash() {
					t.Fatal("null descendant incremental key differs from rebuilt full key")
				}
				pos.UnMakeMove(child, tag, ep, clock)
				assertEnPassantPositionRestored(t, pos, nullBefore)
				pos.UnMakeNullMove(nullEP)
				assertEnPassantPositionRestored(t, pos, before)
			})
		}
	}
}
