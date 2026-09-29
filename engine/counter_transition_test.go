package engine

import (
	"fmt"
	"os"
	"testing"

	"github.com/ehrlich-b/ngn/countereval"
	"github.com/ehrlich-b/ngn/nnue"
)

func counterMoveDeltaSemanticReference(pos *Position, move Move) (countereval.MoveDelta, error) {
	source, err := nnueMoveDelta(pos, move)
	if err != nil {
		return countereval.MoveDelta{}, err
	}
	plane := func(piece nnue.PieceOnSquare) (int, error) {
		if piece.Color > nnue.Black || piece.Piece > nnue.King || piece.Square >= 64 {
			return 0, fmt.Errorf("invalid reference piece %+v", piece)
		}
		return int(piece.Color)*6 + int(piece.Piece), nil
	}
	movingPlane, err := plane(source.Removed[0])
	if err != nil {
		return countereval.MoveDelta{}, err
	}
	result := countereval.MoveDelta{
		MovingPlane: uint8(movingPlane),
		From:        uint8(source.Removed[0].Square),
		To:          uint8(source.Added[0].Square),
	}
	switch source.Kind {
	case nnue.MoveNormal:
	case nnue.MoveCapture, nnue.MoveEnPassant:
		capturedPlane, err := plane(source.Removed[1])
		if err != nil {
			return countereval.MoveDelta{}, err
		}
		result.HasCapture = true
		result.CapturedPlane = uint8(capturedPlane)
		result.CaptureSquare = uint8(source.Removed[1].Square)
	case nnue.MovePromotion, nnue.MovePromotionCapture:
		promotionPlane, err := plane(source.Added[0])
		if err != nil {
			return countereval.MoveDelta{}, err
		}
		result.HasPromotion = true
		result.PromotionPlane = uint8(promotionPlane)
		if source.Kind == nnue.MovePromotionCapture {
			capturedPlane, err := plane(source.Removed[1])
			if err != nil {
				return countereval.MoveDelta{}, err
			}
			result.HasCapture = true
			result.CapturedPlane = uint8(capturedPlane)
			result.CaptureSquare = uint8(source.Removed[1].Square)
		}
	case nnue.MoveCastle:
		rookBefore, err := plane(source.Removed[1])
		if err != nil {
			return countereval.MoveDelta{}, err
		}
		rookAfter, err := plane(source.Added[1])
		if err != nil || rookAfter != rookBefore {
			return countereval.MoveDelta{}, fmt.Errorf("malformed reference castle rook")
		}
		result.HasCastleRook = true
		result.CastleRookFrom = uint8(source.Removed[1].Square)
		result.CastleRookTo = uint8(source.Added[1].Square)
	default:
		return countereval.MoveDelta{}, fmt.Errorf("unknown reference move kind %d", source.Kind)
	}
	return result, nil
}

func requireCounterDecoderEquivalent(t *testing.T, pos *Position, move Move) countereval.MoveDelta {
	t.Helper()
	direct, directErr := counterMoveDelta(pos, move)
	reference, referenceErr := counterMoveDeltaSemanticReference(pos, move)
	if (directErr == nil) != (referenceErr == nil) {
		t.Fatalf("direct/reference errors differ: direct=%v reference=%v", directErr, referenceErr)
	}
	if directErr != nil {
		t.Fatalf("equivalence fixture was rejected by both decoders: direct=%v reference=%v", directErr, referenceErr)
	}
	if direct != reference {
		t.Fatalf("direct delta %+v differs from semantic reference %+v", direct, reference)
	}
	return direct
}

func openCounterFixture(t *testing.T, outputBias float32) *os.File {
	t.Helper()
	path := writeUCICounterFixture(t, "counter-fixture.nn", outputBias)
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	return file
}

func TestCounterTransitionAdapterSpecialMovesAndDirectBoards(t *testing.T) {
	tests := []struct {
		name, fen, notation        string
		capture, promotion, castle bool
	}{
		{"white normal", "4k3/8/8/8/8/8/4P3/4K3 w - - 0 1", "e2e4", false, false, false},
		{"black normal", "4k3/4p3/8/8/8/8/8/4K3 b - - 0 1", "e7e5", false, false, false},
		{"white capture", "4k3/8/8/3p4/4P3/8/8/4K3 w - - 0 1", "e4d5", true, false, false},
		{"black en passant", "4k3/8/8/8/3pP3/8/8/4K3 b - e3 0 1", "d4e3", true, false, false},
		{"white castle", "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1", "e1g1", false, false, true},
		{"black castle", "r3k2r/8/8/8/8/8/8/R3K2R b KQkq - 0 1", "e8c8", false, false, true},
		{"white promotion", "4k3/P7/8/8/8/8/8/4K3 w - - 0 1", "a7a8n", false, true, false},
		{"black promotion capture", "4k3/8/8/8/8/8/p7/1R2K3 b - - 0 1", "a2b1q", true, true, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pos, err := ParseFEN(test.fen)
			if err != nil {
				t.Fatal(err)
			}
			beforePosition := snapshotPVPosition(pos)
			beforeBoard, err := counterBoardFromPosition(pos)
			if err != nil {
				t.Fatal(err)
			}
			move := adapterLegalMove(t, pos, test.notation)
			delta := requireCounterDecoderEquivalent(t, pos, move)
			assertPVPositionRestored(t, pos, beforePosition)
			if delta.HasCapture != test.capture || delta.HasPromotion != test.promotion || delta.HasCastleRook != test.castle {
				t.Fatalf("flags=%+v", delta)
			}
			undoEP, undoTag, undoClock, legal := pos.MakeMove(move)
			if !legal {
				t.Fatal("generated move rejected")
			}
			expectedPost, err := validateCounterMoveAfter(pos, delta)
			if err != nil {
				t.Fatal(err)
			}
			model, _, loadErr := countereval.LoadCounter55Legacy(openCounterFixture(t, 0))
			if loadErr != nil {
				t.Fatal(loadErr)
			}
			context, err := model.NewSearchContext(beforeBoard)
			if err != nil {
				t.Fatal(err)
			}
			if err := context.PushMove(delta, expectedPost); err != nil {
				t.Fatal(err)
			}
			if context.Board() != expectedPost {
				t.Fatal("Counter context board differs from direct post-position bitboards")
			}
			if err := context.Pop(); err != nil {
				t.Fatal(err)
			}
			if context.Board() != beforeBoard {
				t.Fatal("Counter pop did not restore direct pre-position bitboards")
			}
			pos.UnMakeMove(move, undoTag, undoEP, undoClock)
			assertPVPositionRestored(t, pos, beforePosition)
		})
	}
}

func TestCounterTransitionAdapterRejectsInvalidState(t *testing.T) {
	if _, err := counterBoardFromPosition(nil); err == nil {
		t.Fatal("nil position accepted")
	}
	pos, err := ParseFEN("4k3/8/8/8/8/8/4P3/4K3 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := counterMoveDelta(pos, EmptyMove); err == nil {
		t.Fatal("empty move accepted")
	}
	delta := countereval.MoveDelta{MovingPlane: 0, From: 12, To: 28}
	if _, err := validateCounterMoveAfter(pos, delta); err == nil {
		t.Fatal("unchanged side accepted as post-move position")
	}
}

func TestCounterMoveDeltaDirectMatchesSemanticReferenceForIllegalAndMalformedMoves(t *testing.T) {
	t.Run("pseudo-legal self-check", func(t *testing.T) {
		pos, err := ParseFEN("4r1k1/8/8/8/8/8/4R3/4K3 w - - 0 1")
		if err != nil {
			t.Fatal(err)
		}
		before := snapshotPVPosition(pos)
		move := NewMove(E2, D2, WhiteRook, NoPiece, NoType, 0)
		requireCounterDecoderEquivalent(t, pos, move)
		undoEP, undoTag, undoClock, _ := pos.MakeMove(move)
		if !isInCheck(pos, White) {
			t.Fatal("fixture move did not expose the white king")
		}
		pos.UnMakeMove(move, undoTag, undoEP, undoClock)
		assertPVPositionRestored(t, pos, before)
	})

	malformed := []struct {
		name string
		fen  string
		move Move
	}{
		{"empty", "4k3/8/8/8/8/8/4P3/4K3 w - - 0 1", EmptyMove},
		{"wrong mover colour", "4k3/4p3/8/8/8/8/8/4K3 w - - 0 1", NewMove(E7, E6, BlackPawn, NoPiece, NoType, 0)},
		{"missing source", "4k3/8/8/8/8/8/4P3/4K3 w - - 0 1", NewMove(D2, D4, WhitePawn, NoPiece, NoType, 0)},
		{"quiet occupied destination", "4k3/8/8/8/8/8/4P3/4K3 w - - 0 1", NewMove(E2, E8, WhitePawn, NoPiece, NoType, 0)},
		{"capture victim mismatch", "4k3/8/8/8/8/8/4P3/4K3 w - - 0 1", NewMove(E2, E8, WhitePawn, BlackQueen, NoType, Capture)},
		{"invalid promotion type", "4k3/8/8/8/8/8/4P3/4K3 w - - 0 1", NewMove(E2, E3, WhitePawn, NoPiece, King, 0)},
		{"invalid castle squares", "4k3/8/8/8/8/8/8/R3K2R w KQ - 0 1", NewMove(E1, E2, WhiteKing, NoPiece, NoType, KingSideCastle)},
		{"missing en-passant victim", "4k3/8/8/4P3/8/8/8/4K3 w - d6 0 1", NewMove(E5, D6, WhitePawn, BlackPawn, NoType, Capture|EnPassant)},
	}
	for _, test := range malformed {
		t.Run(test.name, func(t *testing.T) {
			pos, err := ParseFEN(test.fen)
			if err != nil {
				t.Fatal(err)
			}
			before := snapshotPVPosition(pos)
			direct, directErr := counterMoveDelta(pos, test.move)
			reference, referenceErr := counterMoveDeltaSemanticReference(pos, test.move)
			if directErr == nil || referenceErr == nil {
				t.Fatalf("malformed move accepted: direct=%+v/%v reference=%+v/%v", direct, directErr, reference, referenceErr)
			}
			assertPVPositionRestored(t, pos, before)
		})
	}
}
