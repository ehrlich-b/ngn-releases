package engine

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"github.com/ehrlich-b/ngn/nnue"
)

func adapterLegalMove(t *testing.T, pos *Position, notation string) Move {
	t.Helper()
	for _, move := range GenerateLegalMoves(pos) {
		if move.ToString() == notation {
			return move
		}
	}
	t.Fatalf("legal move %s not found", notation)
	return EmptyMove
}

func adapterPieceMap(position nnue.Position) ([64]nnue.PieceOnSquare, [64]bool) {
	var pieces [64]nnue.PieceOnSquare
	var present [64]bool
	for _, piece := range position.Pieces {
		pieces[piece.Square] = piece
		present[piece.Square] = true
	}
	return pieces, present
}

func adapterSamePiece(a, b nnue.PieceOnSquare) bool {
	return a.Piece == b.Piece && a.Color == b.Color && a.Square == b.Square
}

func adapterDeltaContains(entries [nnue.MaxDeltaPieces]nnue.PieceOnSquare, count uint8, want nnue.PieceOnSquare) bool {
	for i := 0; i < int(count); i++ {
		if adapterSamePiece(entries[i], want) {
			return true
		}
	}
	return false
}

func requireAdapterDeltaMatchesBoardDiff(t *testing.T, delta nnue.Delta, before, after nnue.Position) {
	t.Helper()
	beforeFacts, err := nnue.Facts(before)
	if err != nil {
		t.Fatal(err)
	}
	afterFacts, err := nnue.Facts(after)
	if err != nil {
		t.Fatal(err)
	}
	if delta.Before != beforeFacts || delta.After != afterFacts {
		t.Fatalf("delta facts = {%+v %+v}, full facts = {%+v %+v}", delta.Before, delta.After, beforeFacts, afterFacts)
	}

	beforePieces, beforePresent := adapterPieceMap(before)
	afterPieces, afterPresent := adapterPieceMap(after)
	removed := 0
	added := 0
	for square := nnue.Square(0); square < 64; square++ {
		if beforePresent[square] && (!afterPresent[square] ||
			beforePieces[square].Piece != afterPieces[square].Piece ||
			beforePieces[square].Color != afterPieces[square].Color) {
			removed++
			if !adapterDeltaContains(delta.Removed, delta.RemovedCount, beforePieces[square]) {
				t.Fatalf("removed board piece %+v missing from delta %+v", beforePieces[square], delta)
			}
		}
		if afterPresent[square] && (!beforePresent[square] ||
			beforePieces[square].Piece != afterPieces[square].Piece ||
			beforePieces[square].Color != afterPieces[square].Color) {
			added++
			if !adapterDeltaContains(delta.Added, delta.AddedCount, afterPieces[square]) {
				t.Fatalf("added board piece %+v missing from delta %+v", afterPieces[square], delta)
			}
		}
	}
	if removed != int(delta.RemovedCount) || added != int(delta.AddedCount) {
		t.Fatalf("board diff counts removed=%d added=%d, delta removed=%d added=%d", removed, added, delta.RemovedCount, delta.AddedCount)
	}
}

func adapterContextModel(t *testing.T) (*nnue.Model, *nnue.Tensors) {
	t.Helper()
	tensors := new(nnue.Tensors)
	for feature := 0; feature < nnue.InputSize; feature++ {
		for hidden := 0; hidden < nnue.HiddenSize; hidden++ {
			tensors.FeatureWeights[feature][hidden] = int16((feature*131+hidden*977)%60001 - 30000)
		}
	}
	for hidden := 0; hidden < nnue.HiddenSize; hidden++ {
		tensors.FeatureBias[hidden] = int16((hidden*7919)%60001 - 30000)
		tensors.OutputWeights[hidden] = int16((hidden*43)%127 - 63)
		tensors.OutputWeights[nnue.HiddenSize+hidden] = int16((hidden*71)%127 - 63)
	}
	tensors.OutputBias = -41
	container, err := nnue.Marshal(tensors)
	if err != nil {
		t.Fatal(err)
	}
	model, err := nnue.Load(bytes.NewReader(container))
	if err != nil {
		t.Fatal(err)
	}
	return model, tensors
}

func requireAdapterContextLanes(t *testing.T, context *nnue.Context, tensors *nnue.Tensors, position nnue.Position) {
	t.Helper()
	value := reflect.ValueOf(context).Elem()
	depth := int(value.FieldByName("depth").Uint())
	accumulators := value.FieldByName("states").Index(depth).FieldByName("accumulators")
	for perspective := nnue.White; perspective <= nnue.Black; perspective++ {
		for hidden := 0; hidden < nnue.HiddenSize; hidden++ {
			want := int64(tensors.FeatureBias[hidden])
			for _, piece := range position.Pieces {
				feature, err := nnue.FeatureIndex(piece, perspective)
				if err != nil {
					t.Fatal(err)
				}
				want += int64(tensors.FeatureWeights[feature][hidden])
			}
			got := accumulators.Index(int(perspective)).Index(hidden).Int()
			if got != want {
				t.Fatalf("accumulator[%d][%d] = %d, independent full refresh = %d", perspective, hidden, got, want)
			}
		}
	}
}

func TestNNUETransitionAdapterSpecialMoves(t *testing.T) {
	tests := []struct {
		name, fen, notation string
		kind                nnue.MoveKind
	}{
		{"white normal", "4k3/8/8/8/8/8/4P3/4K3 w - - 0 1", "e2e4", nnue.MoveNormal},
		{"black normal", "4k3/4p3/8/8/8/8/8/4K3 b - - 0 1", "e7e5", nnue.MoveNormal},
		{"white capture", "4k3/8/8/3p4/4P3/8/8/4K3 w - - 0 1", "e4d5", nnue.MoveCapture},
		{"black capture", "4k3/8/8/3p4/4P3/8/8/4K3 b - - 0 1", "d5e4", nnue.MoveCapture},
		{"white en passant", "4k3/8/8/3pP3/8/8/8/4K3 w - d6 0 1", "e5d6", nnue.MoveEnPassant},
		{"black en passant", "4k3/8/8/8/3pP3/8/8/4K3 b - e3 0 1", "d4e3", nnue.MoveEnPassant},
		{"white king castle", "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1", "e1g1", nnue.MoveCastle},
		{"white queen castle", "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1", "e1c1", nnue.MoveCastle},
		{"black king castle", "r3k2r/8/8/8/8/8/8/R3K2R b KQkq - 0 1", "e8g8", nnue.MoveCastle},
		{"black queen castle", "r3k2r/8/8/8/8/8/8/R3K2R b KQkq - 0 1", "e8c8", nnue.MoveCastle},
		{"white promotes queen", "4k3/P7/8/8/8/8/8/4K3 w - - 0 1", "a7a8q", nnue.MovePromotion},
		{"white promotes rook", "4k3/P7/8/8/8/8/8/4K3 w - - 0 1", "a7a8r", nnue.MovePromotion},
		{"white promotes bishop", "4k3/P7/8/8/8/8/8/4K3 w - - 0 1", "a7a8b", nnue.MovePromotion},
		{"white promotes knight", "4k3/P7/8/8/8/8/8/4K3 w - - 0 1", "a7a8n", nnue.MovePromotion},
		{"black promotes queen", "4k3/8/8/8/8/8/p7/4K3 b - - 0 1", "a2a1q", nnue.MovePromotion},
		{"white promotion capture", "1r2k3/P7/8/8/8/8/8/4K3 w - - 0 1", "a7b8q", nnue.MovePromotionCapture},
		{"black promotion capture", "4k3/8/8/8/8/8/p7/1R2K3 b - - 0 1", "a2b1q", nnue.MovePromotionCapture},
	}

	model, tensors := adapterContextModel(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pos, err := ParseFEN(test.fen)
			if err != nil {
				t.Fatal(err)
			}
			move := adapterLegalMove(t, pos, test.notation)
			var beforeBuffer nnueRootBuffer
			before, err := beforeBuffer.position(pos)
			if err != nil {
				t.Fatal(err)
			}
			boardBefore := pos.Board
			tagBefore := pos.Tag
			epBefore := pos.EnPassant
			clockBefore := pos.HalfMoveClock
			delta, err := nnueMoveDelta(pos, move)
			if err != nil {
				t.Fatalf("nnueMoveDelta: %v", err)
			}
			if pos.Board != boardBefore || pos.Tag != tagBefore || pos.EnPassant != epBefore || pos.HalfMoveClock != clockBefore {
				t.Fatal("delta derivation mutated engine position")
			}
			if delta.Kind != test.kind {
				t.Fatalf("kind = %d, want %d", delta.Kind, test.kind)
			}
			undoEP, undoTag, undoClock, legal := pos.MakeMove(move)
			if !legal {
				t.Fatal("generated move rejected")
			}
			var afterBuffer nnueRootBuffer
			after, err := afterBuffer.position(pos)
			if err != nil {
				t.Fatal(err)
			}
			if err := validateNNUEMoveAfter(pos, delta); err != nil {
				t.Fatalf("validateNNUEMoveAfter: %v", err)
			}
			requireAdapterDeltaMatchesBoardDiff(t, delta, before, after)
			context, err := nnue.NewContext(model)
			if err != nil {
				t.Fatal(err)
			}
			if err := context.Reset(before); err != nil {
				t.Fatal(err)
			}
			requireAdapterContextLanes(t, context, tensors, before)
			if err := context.PushDelta(delta); err != nil {
				t.Fatalf("PushDelta: %v", err)
			}
			requireAdapterContextLanes(t, context, tensors, after)
			if err := context.Pop(); err != nil {
				t.Fatal(err)
			}
			requireAdapterContextLanes(t, context, tensors, before)
			pos.UnMakeMove(move, undoTag, undoEP, undoClock)
			if pos.Board != boardBefore || pos.Tag != tagBefore || pos.EnPassant != epBefore || pos.HalfMoveClock != clockBefore {
				t.Fatal("make/unmake did not restore full engine state")
			}
		})
	}
}

func TestNNUETransitionAdapterRejectsInvalidState(t *testing.T) {
	if _, err := new(nnueRootBuffer).position(nil); !errors.Is(err, errNNUETransition) {
		t.Fatalf("nil root error = %v", err)
	}
	if _, err := nnueMoveDelta(nil, EmptyMove); !errors.Is(err, errNNUETransition) {
		t.Fatalf("nil delta error = %v", err)
	}

	corrupt, err := ParseFEN("4k3/8/8/8/8/8/4P3/4K3 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	corrupt.Board.mailbox[E2] = NoPiece
	if _, err := new(nnueRootBuffer).position(corrupt); !errors.Is(err, errNNUETransition) {
		t.Fatalf("corrupt root error = %v", err)
	}

	start, err := ParseFEN("r3k2r/8/8/8/8/8/4P3/4K2R w Kkq - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	invalid := []Move{
		NewMove(E2, E4, BlackPawn, NoPiece, NoType, 0),
		NewMove(E1, E2, WhiteKing, NoPiece, NoType, 0),
		NewMove(E1, G1, WhiteKing, NoPiece, King, KingSideCastle),
	}
	for _, move := range invalid {
		if _, err := nnueMoveDelta(start, move); !errors.Is(err, errNNUETransition) {
			t.Fatalf("invalid move %v error = %v", move, err)
		}
	}

	capturePos, err := ParseFEN("4k3/8/8/3p4/4P3/8/8/4K3 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	wrongVictim := NewMove(E4, D5, WhitePawn, BlackBishop, NoType, Capture)
	if _, err := nnueMoveDelta(capturePos, wrongVictim); !errors.Is(err, errNNUETransition) {
		t.Fatalf("wrong victim error = %v", err)
	}

	noRook, err := ParseFEN("4k3/8/8/8/8/8/8/4K3 w K - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	castle := NewMove(E1, G1, WhiteKing, NoPiece, NoType, KingSideCastle)
	if _, err := nnueMoveDelta(noRook, castle); !errors.Is(err, errNNUETransition) {
		t.Fatalf("missing rook error = %v", err)
	}

	epPos, err := ParseFEN("4k3/8/8/3pP3/8/8/8/4K3 w - d6 0 1")
	if err != nil {
		t.Fatal(err)
	}
	badEP := NewMove(E5, D6, WhitePawn, BlackKnight, NoType, EnPassant)
	if _, err := nnueMoveDelta(epPos, badEP); !errors.Is(err, errNNUETransition) {
		t.Fatalf("bad en-passant error = %v", err)
	}
	outOfRangeEP := NewMove(E5, D6, WhitePawn, Piece(15), NoType, EnPassant)
	if _, err := nnueMoveDelta(epPos, outOfRangeEP); !errors.Is(err, errNNUETransition) {
		t.Fatalf("out-of-range en-passant error = %v", err)
	}
}

func TestNNUETransitionAdapterNullAndSingularReplay(t *testing.T) {
	pos, err := ParseFEN("4k3/8/8/8/8/8/4P3/4K3 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	move := adapterLegalMove(t, pos, "e2e4")
	first, err := nnueMoveDelta(pos, move)
	if err != nil {
		t.Fatal(err)
	}
	undoEP, undoTag, undoClock, _ := pos.MakeMove(move)
	pos.UnMakeMove(move, undoTag, undoEP, undoClock)
	second, err := nnueMoveDelta(pos, move)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("replayed delta differs: first=%+v second=%+v", first, second)
	}

	before, err := nnuePositionFacts(pos)
	if err != nil {
		t.Fatal(err)
	}
	beforePieces, err := nnuePositionPieceBitboards(pos)
	if err != nil {
		t.Fatal(err)
	}
	ep := pos.MakeNullMove()
	after, err := nnuePositionFacts(pos)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateNNUENullAfter(pos, before, beforePieces, nnue.White); err != nil {
		t.Fatalf("validateNNUENullAfter: %v", err)
	}
	if before != after {
		t.Fatalf("null move changed NNUE facts: before=%+v after=%+v", before, after)
	}
	pos.UnMakeNullMove(ep)
	restored, _ := nnuePositionFacts(pos)
	if restored != before {
		t.Fatalf("null undo facts = %+v, want %+v", restored, before)
	}
}

func TestNNUETransitionPostValidationRejectsUnbridgedState(t *testing.T) {
	pos, err := ParseFEN("4k3/8/8/8/8/8/4P3/1N2K3 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	move := adapterLegalMove(t, pos, "e2e4")
	delta, err := nnueMoveDelta(pos, move)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateNNUEMoveAfter(pos, delta); !errors.Is(err, errNNUETransition) {
		t.Fatalf("pre-move position accepted as post-move: %v", err)
	}

	before, err := nnuePositionFacts(pos)
	if err != nil {
		t.Fatal(err)
	}
	beforePieces, err := nnuePositionPieceBitboards(pos)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateNNUENullAfter(pos, before, beforePieces, nnue.White); !errors.Is(err, errNNUETransition) {
		t.Fatalf("null without side toggle accepted: %v", err)
	}
	pos.ToggleTurn()
	pos.Board.UpdateSquare(B1, WhiteBishop, WhiteKnight)
	if after, _ := nnuePositionFacts(pos); after != before {
		t.Fatal("same-occupancy non-pawn substitution changed compact facts")
	}
	if err := validateNNUENullAfter(pos, before, beforePieces, nnue.White); !errors.Is(err, errNNUETransition) {
		t.Fatalf("null non-pawn substitution accepted: %v", err)
	}

	promotion, err := ParseFEN("4k3/P7/8/8/8/8/8/4K3 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	queen := adapterLegalMove(t, promotion, "a7a8q")
	queenDelta, err := nnueMoveDelta(promotion, queen)
	if err != nil {
		t.Fatal(err)
	}
	rook := adapterLegalMove(t, promotion, "a7a8r")
	undoEP, undoTag, undoClock, _ := promotion.MakeMove(rook)
	actualFacts, _ := nnuePositionFacts(promotion)
	if actualFacts != queenDelta.After {
		t.Fatal("promotion witness does not preserve compact facts")
	}
	if err := validateNNUEMoveAfter(promotion, queenDelta); !errors.Is(err, errNNUETransition) {
		t.Fatalf("wrong promoted piece accepted: %v", err)
	}
	promotion.UnMakeMove(rook, undoTag, undoEP, undoClock)
}

func TestNNUETransitionAdapterHasNoHotAllocations(t *testing.T) {
	pos, err := ParseFEN("4k3/8/8/3p4/4P3/8/8/4K3 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	move := adapterLegalMove(t, pos, "e4d5")
	var buffer nnueRootBuffer
	if allocations := testing.AllocsPerRun(100, func() {
		if _, err := buffer.position(pos); err != nil {
			panic(err)
		}
		if _, err := nnueMoveDelta(pos, move); err != nil {
			panic(err)
		}
	}); allocations != 0 {
		t.Fatalf("adapter allocations = %g, want 0", allocations)
	}
}
