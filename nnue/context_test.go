package nnue_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/ehrlich-b/ngn/nnue"
)

func contextFeatureWeight(feature, hidden int) int16 {
	return int16((feature*131+hidden*977)%60001 - 30000)
}

func contextFeatureBias(hidden int) int16 {
	return int16((hidden*7919)%60001 - 30000)
}

func contextModel(t *testing.T) *nnue.Model {
	t.Helper()
	payload := make([]byte, manualPayloadSize)
	for feature := 0; feature < 768; feature++ {
		for hidden := 0; hidden < 128; hidden++ {
			setFeatureWeight(payload, feature, hidden, contextFeatureWeight(feature, hidden))
		}
	}
	for hidden := 0; hidden < 128; hidden++ {
		setFeatureBias(payload, hidden, contextFeatureBias(hidden))
		setOutputWeight(payload, hidden, int16((hidden*43)%127-63))
		setOutputWeight(payload, 128+hidden, int16((hidden*71)%127-63))
	}
	setOutputBias(payload, -41)
	return loadManual(t, payload)
}

func referenceFeatureIndex(piece nnue.PieceOnSquare, perspective nnue.Color) int {
	relativeColor := 1
	if piece.Color == perspective {
		relativeColor = 0
	}
	square := int(piece.Square)
	if perspective == nnue.Black {
		square ^= 56
	}
	return relativeColor*384 + int(piece.Piece)*64 + square
}

func referenceAccumulators(position nnue.Position) [2][128]int64 {
	var accumulators [2][128]int64
	for perspective := nnue.White; perspective <= nnue.Black; perspective++ {
		for hidden := 0; hidden < 128; hidden++ {
			accumulators[perspective][hidden] = int64(contextFeatureBias(hidden))
		}
		for _, piece := range position.Pieces {
			feature := referenceFeatureIndex(piece, perspective)
			for hidden := 0; hidden < 128; hidden++ {
				accumulators[perspective][hidden] += int64(contextFeatureWeight(feature, hidden))
			}
		}
	}
	return accumulators
}

func pos(side nnue.Color, pieces ...nnue.PieceOnSquare) nnue.Position {
	return nnue.Position{SideToMove: side, Pieces: pieces}
}

func piece(kind nnue.PieceType, color nnue.Color, square nnue.Square) nnue.PieceOnSquare {
	return nnue.PieceOnSquare{Piece: kind, Color: color, Square: square}
}

func transition(t *testing.T, kind nnue.MoveKind, before, after nnue.Position, removed, added []nnue.PieceOnSquare) nnue.Delta {
	t.Helper()
	beforeFacts, err := nnue.Facts(before)
	if err != nil {
		t.Fatalf("before Facts: %v", err)
	}
	afterFacts, err := nnue.Facts(after)
	if err != nil {
		t.Fatalf("after Facts: %v", err)
	}
	var delta nnue.Delta
	delta.Kind = kind
	delta.RemovedCount = uint8(len(removed))
	delta.AddedCount = uint8(len(added))
	copy(delta.Removed[:], removed)
	copy(delta.Added[:], added)
	delta.Before = beforeFacts
	delta.After = afterFacts
	return delta
}

func requireContextEqualsRefresh(t *testing.T, model *nnue.Model, context *nnue.Context, position nnue.Position) int64 {
	t.Helper()
	got, err := context.Evaluate()
	if err != nil {
		t.Fatalf("Context.Evaluate: %v", err)
	}
	want, err := model.Evaluate(position)
	if err != nil {
		t.Fatalf("Model.Evaluate: %v", err)
	}
	if got != want {
		t.Fatalf("incremental score = %d, full-refresh score = %d", got, want)
	}
	return got
}

func contextSnapshot(t *testing.T, context *nnue.Context) [2][128]int64 {
	t.Helper()
	snapshot, err := nnue.AccumulatorSnapshotForTest(context)
	if err != nil {
		t.Fatalf("AccumulatorSnapshotForTest: %v", err)
	}
	return snapshot
}

func requireContextMatchesReference(t *testing.T, model *nnue.Model, context *nnue.Context, position nnue.Position) int64 {
	t.Helper()
	score := requireContextEqualsRefresh(t, model, context, position)
	got := contextSnapshot(t, context)
	if want := referenceAccumulators(position); got != want {
		for perspective := 0; perspective < 2; perspective++ {
			for hidden := 0; hidden < 128; hidden++ {
				if got[perspective][hidden] != want[perspective][hidden] {
					t.Fatalf("accumulator[%d][%d] = %d, independent refresh = %d", perspective, hidden, got[perspective][hidden], want[perspective][hidden])
				}
			}
		}
		t.Fatal("accumulator mismatch")
	}
	return score
}

func TestContextHandcraftedTransitionsMatchFullRefresh(t *testing.T) {
	wk := piece(nnue.King, nnue.White, 4)
	bk := piece(nnue.King, nnue.Black, 60)
	wpE2 := piece(nnue.Pawn, nnue.White, 12)
	wpE4 := piece(nnue.Pawn, nnue.White, 28)
	bpD5 := piece(nnue.Pawn, nnue.Black, 35)
	wpD5 := piece(nnue.Pawn, nnue.White, 35)
	wpE5 := piece(nnue.Pawn, nnue.White, 36)
	wpD6 := piece(nnue.Pawn, nnue.White, 43)
	bpD4 := piece(nnue.Pawn, nnue.Black, 27)
	bpE3 := piece(nnue.Pawn, nnue.Black, 20)
	wpE4b := piece(nnue.Pawn, nnue.White, 28)
	wrH1 := piece(nnue.Rook, nnue.White, 7)
	wkG1 := piece(nnue.King, nnue.White, 6)
	wrF1 := piece(nnue.Rook, nnue.White, 5)
	brA8 := piece(nnue.Rook, nnue.Black, 56)
	bkC8 := piece(nnue.King, nnue.Black, 58)
	brD8 := piece(nnue.Rook, nnue.Black, 59)
	wpA7 := piece(nnue.Pawn, nnue.White, 48)
	wqA8 := piece(nnue.Queen, nnue.White, 56)
	bpA2 := piece(nnue.Pawn, nnue.Black, 8)
	bnA1 := piece(nnue.Knight, nnue.Black, 0)
	brB8 := piece(nnue.Rook, nnue.Black, 57)
	wqB8 := piece(nnue.Queen, nnue.White, 57)

	tests := []struct {
		name          string
		kind          nnue.MoveKind
		before, after nnue.Position
		removed       []nnue.PieceOnSquare
		added         []nnue.PieceOnSquare
	}{
		{"white normal", nnue.MoveNormal, pos(nnue.White, wk, bk, wpE2), pos(nnue.Black, wk, bk, wpE4), []nnue.PieceOnSquare{wpE2}, []nnue.PieceOnSquare{wpE4}},
		{"white capture", nnue.MoveCapture, pos(nnue.White, wk, bk, wpE4, bpD5), pos(nnue.Black, wk, bk, wpD5), []nnue.PieceOnSquare{wpE4, bpD5}, []nnue.PieceOnSquare{wpD5}},
		{"white en passant", nnue.MoveEnPassant, pos(nnue.White, wk, bk, wpE5, bpD5), pos(nnue.Black, wk, bk, wpD6), []nnue.PieceOnSquare{wpE5, bpD5}, []nnue.PieceOnSquare{wpD6}},
		{"black en passant", nnue.MoveEnPassant, pos(nnue.Black, wk, bk, bpD4, wpE4b), pos(nnue.White, wk, bk, bpE3), []nnue.PieceOnSquare{bpD4, wpE4b}, []nnue.PieceOnSquare{bpE3}},
		{"white castle", nnue.MoveCastle, pos(nnue.White, wk, bk, wrH1), pos(nnue.Black, wkG1, bk, wrF1), []nnue.PieceOnSquare{wk, wrH1}, []nnue.PieceOnSquare{wkG1, wrF1}},
		{"black castle", nnue.MoveCastle, pos(nnue.Black, wk, bk, brA8), pos(nnue.White, wk, bkC8, brD8), []nnue.PieceOnSquare{bk, brA8}, []nnue.PieceOnSquare{bkC8, brD8}},
		{"white promotion", nnue.MovePromotion, pos(nnue.White, wk, bk, wpA7), pos(nnue.Black, wk, bk, wqA8), []nnue.PieceOnSquare{wpA7}, []nnue.PieceOnSquare{wqA8}},
		{"black promotion", nnue.MovePromotion, pos(nnue.Black, wk, bk, bpA2), pos(nnue.White, wk, bk, bnA1), []nnue.PieceOnSquare{bpA2}, []nnue.PieceOnSquare{bnA1}},
		{"white promotion capture", nnue.MovePromotionCapture, pos(nnue.White, wk, bk, wpA7, brB8), pos(nnue.Black, wk, bk, wqB8), []nnue.PieceOnSquare{wpA7, brB8}, []nnue.PieceOnSquare{wqB8}},
	}

	model := contextModel(t)
	apis := []struct {
		name string
		push func(*nnue.Context, nnue.Delta, nnue.Position) error
	}{
		{"full-position", func(context *nnue.Context, delta nnue.Delta, after nnue.Position) error {
			return context.Push(delta, after)
		}},
		{"compact-delta", func(context *nnue.Context, delta nnue.Delta, _ nnue.Position) error {
			return context.PushDelta(delta)
		}},
	}
	for _, test := range tests {
		for _, api := range apis {
			t.Run(test.name+"/"+api.name, func(t *testing.T) {
				context, err := nnue.NewContext(model)
				if err != nil {
					t.Fatalf("NewContext: %v", err)
				}
				if err := context.Reset(test.before); err != nil {
					t.Fatalf("Reset: %v", err)
				}
				rootScore := requireContextMatchesReference(t, model, context, test.before)
				delta := transition(t, test.kind, test.before, test.after, test.removed, test.added)
				if err := api.push(context, delta, test.after); err != nil {
					t.Fatalf("Push: %v", err)
				}
				requireContextMatchesReference(t, model, context, test.after)
				if context.Depth() != 1 {
					t.Fatalf("depth = %d, want 1", context.Depth())
				}
				if err := context.Pop(); err != nil {
					t.Fatalf("Pop: %v", err)
				}
				if got := requireContextMatchesReference(t, model, context, test.before); got != rootScore {
					t.Fatalf("score after undo = %d, want root %d", got, rootScore)
				}
			})
		}
	}
}

func TestContextNullMoveAndUndo(t *testing.T) {
	model := contextModel(t)
	before := pos(nnue.White,
		piece(nnue.King, nnue.White, 4),
		piece(nnue.King, nnue.Black, 60),
		piece(nnue.Queen, nnue.White, 3),
	)
	after := before
	after.SideToMove = nnue.Black
	afterFacts, err := nnue.Facts(after)
	if err != nil {
		t.Fatal(err)
	}
	apis := []struct {
		name string
		push func(*nnue.Context) error
	}{
		{"full-position", func(context *nnue.Context) error { return context.PushNull(after) }},
		{"compact-facts", func(context *nnue.Context) error { return context.PushNullFacts(afterFacts) }},
	}
	for _, api := range apis {
		t.Run(api.name, func(t *testing.T) {
			context, err := nnue.NewContext(model)
			if err != nil {
				t.Fatal(err)
			}
			if err := context.Reset(before); err != nil {
				t.Fatal(err)
			}
			root := requireContextMatchesReference(t, model, context, before)
			if err := api.push(context); err != nil {
				t.Fatalf("push null: %v", err)
			}
			child := requireContextMatchesReference(t, model, context, after)
			if child == root {
				t.Fatalf("asymmetric fixture did not distinguish STM ordering: both %d", child)
			}
			if err := context.Pop(); err != nil {
				t.Fatal(err)
			}
			if got := requireContextMatchesReference(t, model, context, before); got != root {
				t.Fatalf("after null undo = %d, want %d", got, root)
			}
		})
	}
}

func TestContextFailedOperationsPreserveState(t *testing.T) {
	model := contextModel(t)
	before := pos(nnue.White, piece(nnue.King, nnue.White, 4), piece(nnue.King, nnue.Black, 60), piece(nnue.Pawn, nnue.White, 12))
	after := pos(nnue.Black, piece(nnue.King, nnue.White, 4), piece(nnue.King, nnue.Black, 60), piece(nnue.Pawn, nnue.White, 28))
	valid := transition(t, nnue.MoveNormal, before, after, []nnue.PieceOnSquare{before.Pieces[2]}, []nnue.PieceOnSquare{after.Pieces[2]})
	context, _ := nnue.NewContext(model)
	if err := context.Reset(before); err != nil {
		t.Fatal(err)
	}
	rootScore := requireContextMatchesReference(t, model, context, before)
	rootFacts, _ := context.Facts()
	rootSnapshot := contextSnapshot(t, context)

	badBefore := valid
	badBefore.Before.Occupied ^= 1
	badReconstruction := valid
	badReconstruction.Removed[0].Square = 11
	badKind := valid
	badKind.Kind = nnue.MoveCapture
	badSidePiece := valid
	badSidePiece.Removed[0].Color = nnue.Black
	badSidePiece.Added[0].Color = nnue.Black
	wrongSide := after
	wrongSide.SideToMove = nnue.White
	changedNull := after
	changedNull.Pieces = append([]nnue.PieceOnSquare(nil), after.Pieces...)

	for name, operation := range map[string]func() error{
		"before facts":   func() error { return context.Push(badBefore, after) },
		"reconstruction": func() error { return context.Push(badReconstruction, after) },
		"semantic kind":  func() error { return context.Push(badKind, after) },
		"mover side":     func() error { return context.Push(badSidePiece, after) },
		"side toggle":    func() error { return context.Push(valid, wrongSide) },
		"changed null":   func() error { return context.PushNull(changedNull) },
	} {
		t.Run(name, func(t *testing.T) {
			if err := operation(); !errors.Is(err, nnue.ErrContext) {
				t.Fatalf("error = %v, want ErrContext", err)
			}
			if context.Depth() != 0 {
				t.Fatalf("failed operation changed depth to %d", context.Depth())
			}
			facts, err := context.Facts()
			if err != nil || facts != rootFacts {
				t.Fatalf("facts after failure = %+v, %v; want %+v", facts, err, rootFacts)
			}
			if score := requireContextMatchesReference(t, model, context, before); score != rootScore {
				t.Fatalf("score after failure = %d, want %d", score, rootScore)
			}
			if snapshot := contextSnapshot(t, context); snapshot != rootSnapshot {
				t.Fatal("failed operation changed accumulator snapshot")
			}
		})
	}

	invalidReset := nnue.Position{SideToMove: nnue.Color(9)}
	if err := context.Reset(invalidReset); err == nil {
		t.Fatal("invalid Reset succeeded")
	}
	if context.Depth() != 0 || requireContextMatchesReference(t, model, context, before) != rootScore {
		t.Fatal("failed Reset changed live state")
	}
	if snapshot := contextSnapshot(t, context); snapshot != rootSnapshot {
		t.Fatal("failed Reset changed accumulator snapshot")
	}
}

func TestContextPushDeltaFailuresAreTransactional(t *testing.T) {
	model := contextModel(t)
	before := pos(nnue.White,
		piece(nnue.King, nnue.White, 4),
		piece(nnue.King, nnue.Black, 60),
		piece(nnue.Pawn, nnue.White, 12),
		piece(nnue.Knight, nnue.Black, 28),
	)
	after := pos(nnue.Black,
		piece(nnue.King, nnue.White, 4),
		piece(nnue.King, nnue.Black, 60),
		piece(nnue.Pawn, nnue.White, 28),
	)
	valid := transition(t, nnue.MoveCapture, before, after,
		[]nnue.PieceOnSquare{before.Pieces[2], before.Pieces[3]},
		[]nnue.PieceOnSquare{after.Pieces[2]},
	)
	context, _ := nnue.NewContext(model)
	if err := context.Reset(before); err != nil {
		t.Fatal(err)
	}

	badBefore := valid
	badBefore.Before.Occupied ^= 1
	missingMover := valid
	missingMover.Removed[0].Square = 11
	wrongVictim := valid
	wrongVictim.Removed[1].Piece = nnue.Bishop
	collision := valid
	collision.Added[0].Square = 4
	badAfter := valid
	badAfter.After.Pawns[nnue.White] ^= uint64(1) << valid.Added[0].Square
	wrongSide := valid
	wrongSide.Removed[0].Color = nnue.Black
	wrongSide.Added[0].Color = nnue.Black

	for name, delta := range map[string]nnue.Delta{
		"before facts":  badBefore,
		"missing mover": missingMover,
		"wrong victim":  wrongVictim,
		"collision":     collision,
		"after facts":   badAfter,
		"mover side":    wrongSide,
	} {
		t.Run(name, func(t *testing.T) {
			snapshot := *context
			if err := context.PushDelta(delta); !errors.Is(err, nnue.ErrContext) {
				t.Fatalf("PushDelta error = %v, want ErrContext", err)
			}
			if !reflect.DeepEqual(*context, snapshot) {
				t.Fatal("failed PushDelta changed Context state")
			}
			requireContextMatchesReference(t, model, context, before)
		})
	}

	facts, err := context.Facts()
	if err != nil {
		t.Fatal(err)
	}
	changed := facts
	changed.Occupied ^= 1
	snapshot := *context
	if err := context.PushNullFacts(changed); !errors.Is(err, nnue.ErrContext) {
		t.Fatalf("PushNullFacts error = %v, want ErrContext", err)
	}
	if !reflect.DeepEqual(*context, snapshot) {
		t.Fatal("failed PushNullFacts changed Context state")
	}
}

func TestContextPushDeltaSingularReplay(t *testing.T) {
	model := contextModel(t)
	root := pos(nnue.White,
		piece(nnue.King, nnue.White, 4),
		piece(nnue.King, nnue.Black, 60),
		piece(nnue.Pawn, nnue.White, 12),
	)
	child := pos(nnue.Black,
		piece(nnue.King, nnue.White, 4),
		piece(nnue.King, nnue.Black, 60),
		piece(nnue.Pawn, nnue.White, 28),
	)
	alternative := pos(nnue.Black,
		piece(nnue.King, nnue.White, 3),
		piece(nnue.King, nnue.Black, 60),
		piece(nnue.Pawn, nnue.White, 12),
	)
	move := transition(t, nnue.MoveNormal, root, child,
		[]nnue.PieceOnSquare{root.Pieces[2]},
		[]nnue.PieceOnSquare{child.Pieces[2]},
	)
	verify := transition(t, nnue.MoveNormal, root, alternative,
		[]nnue.PieceOnSquare{root.Pieces[0]},
		[]nnue.PieceOnSquare{alternative.Pieces[0]},
	)
	context, _ := nnue.NewContext(model)
	if err := context.Reset(root); err != nil {
		t.Fatal(err)
	}

	if err := context.PushDelta(move); err != nil {
		t.Fatal(err)
	}
	requireContextMatchesReference(t, model, context, child)
	if err := context.Pop(); err != nil {
		t.Fatal(err)
	}
	requireContextMatchesReference(t, model, context, root)

	if err := context.PushDelta(verify); err != nil {
		t.Fatal(err)
	}
	requireContextMatchesReference(t, model, context, alternative)
	if err := context.Pop(); err != nil {
		t.Fatal(err)
	}
	requireContextMatchesReference(t, model, context, root)

	if err := context.PushDelta(move); err != nil {
		t.Fatal(err)
	}
	requireContextMatchesReference(t, model, context, child)
	if err := context.Pop(); err != nil {
		t.Fatal(err)
	}
	requireContextMatchesReference(t, model, context, root)
}

func TestContextStackBoundsAndUninitializedOperations(t *testing.T) {
	model := contextModel(t)
	context, _ := nnue.NewContext(model)
	if _, err := context.Evaluate(); !errors.Is(err, nnue.ErrContext) {
		t.Fatalf("uninitialized Evaluate error = %v", err)
	}
	if err := context.Pop(); !errors.Is(err, nnue.ErrContext) {
		t.Fatalf("uninitialized Pop error = %v", err)
	}
	white := pos(nnue.White, piece(nnue.King, nnue.White, 4), piece(nnue.King, nnue.Black, 60))
	black := white
	black.SideToMove = nnue.Black
	if err := context.Reset(white); err != nil {
		t.Fatal(err)
	}
	if err := context.Pop(); !errors.Is(err, nnue.ErrContext) {
		t.Fatalf("underflow error = %v", err)
	}
	for depth := 0; depth < nnue.MaxContextPly; depth++ {
		after := black
		if depth%2 == 1 {
			after = white
		}
		if err := context.PushNull(after); err != nil {
			t.Fatalf("PushNull at depth %d: %v", depth, err)
		}
	}
	fullScore, _ := context.Evaluate()
	fullSnapshot := contextSnapshot(t, context)
	if context.Depth() != nnue.MaxContextPly {
		t.Fatalf("depth = %d", context.Depth())
	}
	extra := black
	if nnue.MaxContextPly%2 == 1 {
		extra = white
	}
	if err := context.PushNull(extra); !errors.Is(err, nnue.ErrContext) {
		t.Fatalf("overflow error = %v", err)
	}
	if score, _ := context.Evaluate(); context.Depth() != nnue.MaxContextPly || score != fullScore {
		t.Fatal("overflow changed state")
	}
	if snapshot := contextSnapshot(t, context); snapshot != fullSnapshot {
		t.Fatal("overflow changed accumulator snapshot")
	}
}

func TestContextExtremeTensorsAndNoHotAllocations(t *testing.T) {
	payload := make([]byte, manualPayloadSize)
	for hidden := 0; hidden < 128; hidden++ {
		setFeatureBias(payload, hidden, 32767)
	}
	for hidden := 0; hidden < 256; hidden++ {
		setOutputWeight(payload, hidden, 32767)
	}
	setOutputBias(payload, 32767)
	model := loadManual(t, payload)
	before := pos(nnue.White, piece(nnue.King, nnue.White, 4), piece(nnue.King, nnue.Black, 60), piece(nnue.Pawn, nnue.White, 12))
	after := pos(nnue.Black, piece(nnue.King, nnue.White, 4), piece(nnue.King, nnue.Black, 60), piece(nnue.Pawn, nnue.White, 28))
	delta := transition(t, nnue.MoveNormal, before, after, []nnue.PieceOnSquare{before.Pieces[2]}, []nnue.PieceOnSquare{after.Pieces[2]})
	context, _ := nnue.NewContext(model)
	if err := context.Reset(before); err != nil {
		t.Fatal(err)
	}
	if score := requireContextEqualsRefresh(t, model, context, before); score != 52428003 {
		t.Fatalf("extreme root score = %d", score)
	}
	if err := context.Push(delta, after); err != nil {
		t.Fatal(err)
	}
	if score := requireContextEqualsRefresh(t, model, context, after); score != 52428003 {
		t.Fatalf("extreme child score = %d", score)
	}
	if err := context.Pop(); err != nil {
		t.Fatal(err)
	}

	allocations := testing.AllocsPerRun(100, func() {
		if err := context.Reset(before); err != nil {
			panic(err)
		}
		if err := context.Push(delta, after); err != nil {
			panic(err)
		}
		if _, err := context.Evaluate(); err != nil {
			panic(err)
		}
		if err := context.Pop(); err != nil {
			panic(err)
		}
	})
	if allocations != 0 {
		t.Fatalf("full-position context cycle allocations = %g, want 0", allocations)
	}

	compact, _ := nnue.NewContext(model)
	compactAllocations := testing.AllocsPerRun(100, func() {
		if err := compact.Reset(before); err != nil {
			panic(err)
		}
		if err := compact.PushDelta(delta); err != nil {
			panic(err)
		}
		if _, err := compact.Evaluate(); err != nil {
			panic(err)
		}
		if err := compact.Pop(); err != nil {
			panic(err)
		}
	})
	if compactAllocations != 0 {
		t.Fatalf("compact context cycle allocations = %g, want 0", compactAllocations)
	}
}

func TestContextsShareImmutableModelButNotState(t *testing.T) {
	model := contextModel(t)
	base := pos(nnue.White, piece(nnue.King, nnue.White, 4), piece(nnue.King, nnue.Black, 60), piece(nnue.Pawn, nnue.White, 12), piece(nnue.Pawn, nnue.Black, 51))
	whiteAfter := pos(nnue.Black, piece(nnue.King, nnue.White, 4), piece(nnue.King, nnue.Black, 60), piece(nnue.Pawn, nnue.White, 28), piece(nnue.Pawn, nnue.Black, 51))
	whiteDelta := transition(t, nnue.MoveNormal, base, whiteAfter, []nnue.PieceOnSquare{base.Pieces[2]}, []nnue.PieceOnSquare{whiteAfter.Pieces[2]})
	otherRoot := base
	otherRoot.SideToMove = nnue.Black
	blackAfter := pos(nnue.White, piece(nnue.King, nnue.White, 4), piece(nnue.King, nnue.Black, 60), piece(nnue.Pawn, nnue.White, 12), piece(nnue.Pawn, nnue.Black, 35))
	blackDelta := transition(t, nnue.MoveNormal, otherRoot, blackAfter, []nnue.PieceOnSquare{otherRoot.Pieces[3]}, []nnue.PieceOnSquare{blackAfter.Pieces[3]})

	first, _ := nnue.NewContext(model)
	second, _ := nnue.NewContext(model)
	if err := first.Reset(base); err != nil {
		t.Fatal(err)
	}
	if err := second.Reset(otherRoot); err != nil {
		t.Fatal(err)
	}
	if err := first.Push(whiteDelta, whiteAfter); err != nil {
		t.Fatal(err)
	}
	if err := second.Push(blackDelta, blackAfter); err != nil {
		t.Fatal(err)
	}
	requireContextMatchesReference(t, model, first, whiteAfter)
	requireContextMatchesReference(t, model, second, blackAfter)

	done := make(chan error, 2)
	run := func(context *nnue.Context, root, after nnue.Position, delta nnue.Delta) {
		for i := 0; i < 200; i++ {
			if err := context.Reset(root); err != nil {
				done <- err
				return
			}
			if err := context.Push(delta, after); err != nil {
				done <- err
				return
			}
			if _, err := context.Evaluate(); err != nil {
				done <- err
				return
			}
			if err := context.Pop(); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}
	go run(first, base, whiteAfter, whiteDelta)
	go run(second, otherRoot, blackAfter, blackDelta)
	for i := 0; i < 2; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
}
