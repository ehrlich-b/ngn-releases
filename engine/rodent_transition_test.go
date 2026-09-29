package engine

import (
	"reflect"
	"testing"

	"github.com/ehrlich-b/ngn/rodenteval"
)

func TestRodentPositionMappingUsesExplicitPlaneOrderAndSide(t *testing.T) {
	pos := n3cPosition(t, "4k3/2p2n2/8/8/8/8/1P2N3/4K3 b - - 0 1")
	got, err := rodentPositionFromPosition(pos)
	if err != nil {
		t.Fatal(err)
	}
	if got.SideToMove != rodenteval.Black ||
		got.Board[rodenteval.WhitePawn] != uint64(1)<<B2 ||
		got.Board[rodenteval.WhiteKnight] != uint64(1)<<E2 ||
		got.Board[rodenteval.WhiteKing] != uint64(1)<<E1 ||
		got.Board[rodenteval.BlackPawn] != uint64(1)<<C7 ||
		got.Board[rodenteval.BlackKnight] != uint64(1)<<F7 ||
		got.Board[rodenteval.BlackKing] != uint64(1)<<E8 {
		t.Fatalf("Rodent mapping = %+v", got)
	}
	if _, err := rodentPositionFromPosition(nil); err == nil {
		t.Fatal("nil position accepted")
	}
}

func TestRodentMoveDeltaCoversPackedMoveSemantics(t *testing.T) {
	const startFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
	tests := []struct {
		name string
		fen  string
		move string
		want rodenteval.MoveDelta
	}{
		{"white quiet", startFEN, "e2e4", rodenteval.MoveDelta{MovingPlane: rodenteval.WhitePawn, From: uint8(E2), To: uint8(E4)}},
		{"black quiet", "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq - 0 1", "e7e5", rodenteval.MoveDelta{MovingPlane: rodenteval.BlackPawn, From: uint8(E7), To: uint8(E5)}},
		{"capture", "4k3/8/8/3p4/4P3/8/8/4K3 w - - 0 1", "e4d5", rodenteval.MoveDelta{MovingPlane: rodenteval.WhitePawn, From: uint8(E4), To: uint8(D5), HasCapture: true, CapturedPlane: rodenteval.BlackPawn, CaptureSquare: uint8(D5)}},
		{"en passant", "4k3/8/8/3pP3/8/8/8/4K3 w - d6 0 1", "e5d6", rodenteval.MoveDelta{MovingPlane: rodenteval.WhitePawn, From: uint8(E5), To: uint8(D6), HasCapture: true, CapturedPlane: rodenteval.BlackPawn, CaptureSquare: uint8(D5)}},
		{"castle", "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1", "e1g1", rodenteval.MoveDelta{MovingPlane: rodenteval.WhiteKing, From: uint8(E1), To: uint8(G1), HasCastleRook: true, CastleRookFrom: uint8(H1), CastleRookTo: uint8(F1)}},
		{"white promotion capture", "r3k3/1P6/8/8/8/8/8/4K3 w - - 0 1", "b7a8q", rodenteval.MoveDelta{MovingPlane: rodenteval.WhitePawn, From: uint8(B7), To: uint8(A8), HasCapture: true, CapturedPlane: rodenteval.BlackRook, CaptureSquare: uint8(A8), HasPromotion: true, PromotionPlane: rodenteval.WhiteQueen}},
		{"black promotion", "4k3/8/8/8/8/8/p7/4K3 b - - 0 1", "a2a1n", rodenteval.MoveDelta{MovingPlane: rodenteval.BlackPawn, From: uint8(A2), To: uint8(A1), HasPromotion: true, PromotionPlane: rodenteval.BlackKnight}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pos := n3cPosition(t, test.fen)
			before := snapshotPVPosition(pos)
			move := adapterLegalMove(t, pos, test.move)
			got, err := rodentMoveDelta(pos, move)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("delta = %+v, want %+v", got, test.want)
			}
			if !reflect.DeepEqual(snapshotPVPosition(pos), before) {
				t.Fatal("delta preparation mutated position")
			}
			undoEP, undoTag, undoClock, _ := pos.MakeMove(move)
			post, err := validateRodentMoveAfter(pos, got)
			if err != nil {
				t.Fatal(err)
			}
			wantPost, err := rodentPositionFromPosition(pos)
			if err != nil || post != wantPost {
				t.Fatalf("post = %+v, want %+v, err=%v", post, wantPost, err)
			}
			pos.UnMakeMove(move, undoTag, undoEP, undoClock)
			if !reflect.DeepEqual(snapshotPVPosition(pos), before) {
				t.Fatal("move round trip changed position")
			}
		})
	}
}

func TestRodentMoveDeltaRejectsMalformedStateWithoutMutation(t *testing.T) {
	pos := n3cPosition(t, "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	before := snapshotPVPosition(pos)
	malformed := []Move{
		EmptyMove,
		NewMove(E2, E4, WhiteKnight, NoPiece, NoType, 0),
		NewMove(E2, E2, WhitePawn, NoPiece, NoType, 0),
		NewMove(E2, E4, BlackPawn, NoPiece, NoType, 0),
	}
	for _, move := range malformed {
		if _, err := rodentMoveDelta(pos, move); err == nil {
			t.Fatalf("malformed move %v accepted", move)
		}
		if !reflect.DeepEqual(snapshotPVPosition(pos), before) {
			t.Fatal("rejected delta mutated position")
		}
	}
}
