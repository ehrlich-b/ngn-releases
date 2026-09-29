package nnue_test

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"
	"unsafe"

	"github.com/ehrlich-b/ngn/engine"
	"github.com/ehrlich-b/ngn/nnue"
)

func n4LoadTensors(t testing.TB, tensors *nnue.Tensors) *nnue.Model {
	t.Helper()
	container, err := nnue.Marshal(tensors)
	if err != nil {
		t.Fatal(err)
	}
	model, err := nnue.Load(bytes.NewReader(container))
	if err != nil {
		t.Fatal(err)
	}
	return model
}

func requireInt32ContextMatches(
	t *testing.T,
	model *nnue.Model,
	reference *nnue.Context,
	narrow *nnue.Int32Context,
	position nnue.Position,
) int64 {
	t.Helper()
	wantLanes := referenceAccumulators(position)
	wideLanes := contextSnapshot(t, reference)
	narrowLanes, err := nnue.Int32AccumulatorSnapshotForTest(narrow)
	if err != nil {
		t.Fatalf("Int32AccumulatorSnapshotForTest: %v", err)
	}
	for perspective := 0; perspective < nnue.PerspectiveCount; perspective++ {
		for hidden := 0; hidden < nnue.HiddenSize; hidden++ {
			want := wantLanes[perspective][hidden]
			if wideLanes[perspective][hidden] != want {
				t.Fatalf("int64 accumulator[%d][%d] = %d, independent oracle = %d", perspective, hidden, wideLanes[perspective][hidden], want)
			}
			if int64(narrowLanes[perspective][hidden]) != want {
				t.Fatalf("int32 accumulator[%d][%d] = %d, independent oracle = %d", perspective, hidden, narrowLanes[perspective][hidden], want)
			}
			if narrowLanes[perspective][hidden] < nnue.MinAccumulatorValue ||
				narrowLanes[perspective][hidden] > nnue.MaxAccumulatorValue {
				t.Fatalf("int32 accumulator[%d][%d] = %d outside proved bounds", perspective, hidden, narrowLanes[perspective][hidden])
			}
		}
	}
	wideScore, err := reference.Evaluate()
	if err != nil {
		t.Fatal(err)
	}
	narrowScore, err := narrow.Evaluate()
	if err != nil {
		t.Fatal(err)
	}
	fullScore, err := model.Evaluate(position)
	if err != nil {
		t.Fatal(err)
	}
	if wideScore != fullScore || narrowScore != fullScore {
		t.Fatalf("scores int64=%d int32=%d full=%d", wideScore, narrowScore, fullScore)
	}
	wideFacts, _ := reference.Facts()
	narrowFacts, _ := narrow.Facts()
	if wideFacts != narrowFacts || wideFacts != mustN4Facts(t, position) {
		t.Fatalf("facts int64=%+v int32=%+v", wideFacts, narrowFacts)
	}
	if reference.Depth() != narrow.Depth() {
		t.Fatalf("depth int64=%d int32=%d", reference.Depth(), narrow.Depth())
	}
	return fullScore
}

func mustN4Facts(t *testing.T, position nnue.Position) nnue.PositionFacts {
	t.Helper()
	facts, err := nnue.Facts(position)
	if err != nil {
		t.Fatal(err)
	}
	return facts
}

func TestInt32ContextLifecycleErrorsAreExplicitAndTransactional(t *testing.T) {
	if _, err := nnue.NewInt32Context(nil); !errors.Is(err, nnue.ErrContext) {
		t.Fatalf("NewInt32Context(nil) error = %v", err)
	}
	model := contextModel(t)
	context, err := nnue.NewInt32Context(model)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := context.Facts(); !errors.Is(err, nnue.ErrContext) {
		t.Fatalf("Facts before Reset error = %v", err)
	}
	if _, err := context.Evaluate(); !errors.Is(err, nnue.ErrContext) {
		t.Fatalf("Evaluate before Reset error = %v", err)
	}
	if err := context.Pop(); !errors.Is(err, nnue.ErrContext) {
		t.Fatalf("Pop before Reset error = %v", err)
	}
	root := pos(nnue.White,
		piece(nnue.King, nnue.White, 4),
		piece(nnue.King, nnue.Black, 60),
	)
	if err := context.Reset(root); err != nil {
		t.Fatal(err)
	}
	snapshot := *context
	if err := context.Pop(); !errors.Is(err, nnue.ErrContext) {
		t.Fatalf("stack underflow error = %v", err)
	}
	if !reflect.DeepEqual(*context, snapshot) {
		t.Fatal("stack underflow changed context bytes")
	}
}

func TestInt32ContextAllSpecialMovesMatchInt64AndIndependentLanes(t *testing.T) {
	tests := []struct {
		name, fen, move string
	}{
		{"white normal", "4k3/8/8/8/8/8/4P3/4K3 w - - 0 1", "e2e4"},
		{"black normal", "4k3/4p3/8/8/8/8/8/4K3 b - - 0 1", "e7e5"},
		{"white capture", "4k3/8/8/3p4/4P3/8/8/4K3 w - - 0 1", "e4d5"},
		{"black capture", "4k3/8/8/3p4/4P3/8/8/4K3 b - - 0 1", "d5e4"},
		{"white en passant", "4k3/8/8/3pP3/8/8/8/4K3 w - d6 0 1", "e5d6"},
		{"black en passant", "4k3/8/8/8/3pP3/8/8/4K3 b - e3 0 1", "d4e3"},
		{"white king castle", "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1", "e1g1"},
		{"white queen castle", "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1", "e1c1"},
		{"black king castle", "r3k2r/8/8/8/8/8/8/R3K2R b KQkq - 0 1", "e8g8"},
		{"black queen castle", "r3k2r/8/8/8/8/8/8/R3K2R b KQkq - 0 1", "e8c8"},
		{"white queen promotion", "4k3/P7/8/8/8/8/8/4K3 w - - 0 1", "a7a8q"},
		{"white rook promotion", "4k3/P7/8/8/8/8/8/4K3 w - - 0 1", "a7a8r"},
		{"white bishop promotion", "4k3/P7/8/8/8/8/8/4K3 w - - 0 1", "a7a8b"},
		{"white knight promotion", "4k3/P7/8/8/8/8/8/4K3 w - - 0 1", "a7a8n"},
		{"black queen promotion", "4k3/8/8/8/8/8/p7/4K3 b - - 0 1", "a2a1q"},
		{"white promotion capture", "1r2k3/P7/8/8/8/8/8/4K3 w - - 0 1", "a7b8q"},
		{"black promotion capture", "4k3/8/8/8/8/8/p7/1R2K3 b - - 0 1", "a2b1q"},
	}
	model := contextModel(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			enginePosition, err := engine.ParseFEN(test.fen)
			if err != nil {
				t.Fatal(err)
			}
			before := fromEnginePosition(enginePosition)
			wide, _ := nnue.NewContext(model)
			narrowFull, _ := nnue.NewInt32Context(model)
			narrowCompact, _ := nnue.NewInt32Context(model)
			if err := wide.Reset(before); err != nil {
				t.Fatal(err)
			}
			if err := narrowFull.Reset(before); err != nil {
				t.Fatal(err)
			}
			if err := narrowCompact.Reset(before); err != nil {
				t.Fatal(err)
			}
			requireInt32ContextMatches(t, model, wide, narrowFull, before)
			requireInt32ContextMatches(t, model, wide, narrowCompact, before)

			move := findLegalMove(t, enginePosition, test.move)
			undoEP, undoTag, undoClock, legal := enginePosition.MakeMove(move)
			if !legal {
				t.Fatal("generated move rejected")
			}
			after := fromEnginePosition(enginePosition)
			delta := deltaFromEngineMove(t, move, before, after)
			if err := wide.PushDelta(delta); err != nil {
				t.Fatal(err)
			}
			if err := narrowFull.Push(delta, after); err != nil {
				t.Fatal(err)
			}
			if err := narrowCompact.PushDelta(delta); err != nil {
				t.Fatal(err)
			}
			requireInt32ContextMatches(t, model, wide, narrowFull, after)
			requireInt32ContextMatches(t, model, wide, narrowCompact, after)

			if err := wide.Pop(); err != nil {
				t.Fatal(err)
			}
			if err := narrowFull.Pop(); err != nil {
				t.Fatal(err)
			}
			if err := narrowCompact.Pop(); err != nil {
				t.Fatal(err)
			}
			enginePosition.UnMakeMove(move, undoTag, undoEP, undoClock)
			restored := fromEnginePosition(enginePosition)
			requireInt32ContextMatches(t, model, wide, narrowFull, restored)
			requireInt32ContextMatches(t, model, wide, narrowCompact, restored)
		})
	}
}

func TestInt32ContextNullHistoryUndoAndStackLimit(t *testing.T) {
	model := contextModel(t)
	enginePosition, err := engine.ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	root := fromEnginePosition(enginePosition)
	wide, _ := nnue.NewContext(model)
	narrow, _ := nnue.NewInt32Context(model)
	if err := wide.Reset(root); err != nil {
		t.Fatal(err)
	}
	if err := narrow.Reset(root); err != nil {
		t.Fatal(err)
	}
	var undos []engineUndo
	for ply := 0; ply < 40; ply++ {
		moves := engine.GenerateLegalMoves(enginePosition)
		if len(moves) == 0 {
			break
		}
		move := moves[(ply*17+5)%len(moves)]
		before := fromEnginePosition(enginePosition)
		undoEP, undoTag, undoClock, _ := enginePosition.MakeMove(move)
		after := fromEnginePosition(enginePosition)
		delta := deltaFromEngineMove(t, move, before, after)
		if err := wide.PushDelta(delta); err != nil {
			t.Fatal(err)
		}
		if err := narrow.PushDelta(delta); err != nil {
			t.Fatal(err)
		}
		requireInt32ContextMatches(t, model, wide, narrow, after)
		undos = append(undos, engineUndo{move: move, tag: undoTag, enPassant: undoEP, halfClock: undoClock})
	}
	if len(undos) < 10 {
		t.Fatalf("history too short: %d", len(undos))
	}
	for i := len(undos) - 1; i >= 0; i-- {
		if err := wide.Pop(); err != nil {
			t.Fatal(err)
		}
		if err := narrow.Pop(); err != nil {
			t.Fatal(err)
		}
		undo := undos[i]
		enginePosition.UnMakeMove(undo.move, undo.tag, undo.enPassant, undo.halfClock)
		requireInt32ContextMatches(t, model, wide, narrow, fromEnginePosition(enginePosition))
	}

	narrowFull, _ := nnue.NewInt32Context(model)
	if err := narrowFull.Reset(root); err != nil {
		t.Fatal(err)
	}
	nullAfter := root
	nullAfter.SideToMove ^= 1
	nullFacts := mustN4Facts(t, nullAfter)
	if err := wide.PushNull(nullAfter); err != nil {
		t.Fatal(err)
	}
	if err := narrowFull.PushNull(nullAfter); err != nil {
		t.Fatal(err)
	}
	if err := narrow.PushNullFacts(nullFacts); err != nil {
		t.Fatal(err)
	}
	requireInt32ContextMatches(t, model, wide, narrowFull, nullAfter)
	requireInt32ContextMatches(t, model, wide, narrow, nullAfter)
	if err := wide.Pop(); err != nil {
		t.Fatal(err)
	}
	if err := narrowFull.Pop(); err != nil {
		t.Fatal(err)
	}
	if err := narrow.Pop(); err != nil {
		t.Fatal(err)
	}
	requireInt32ContextMatches(t, model, wide, narrowFull, root)
	requireInt32ContextMatches(t, model, wide, narrow, root)

	for depth := 0; depth < nnue.MaxContextPly; depth++ {
		after := nullFacts
		if err := narrow.PushNullFacts(after); err != nil {
			t.Fatalf("PushNullFacts depth %d: %v", depth, err)
		}
	}
	snapshot, err := nnue.Int32AccumulatorSnapshotForTest(narrow)
	if err != nil {
		t.Fatal(err)
	}
	if err := narrow.PushNullFacts(nullFacts); !errors.Is(err, nnue.ErrContext) {
		t.Fatalf("overflow error = %v", err)
	}
	afterOverflow, _ := nnue.Int32AccumulatorSnapshotForTest(narrow)
	if narrow.Depth() != nnue.MaxContextPly || snapshot != afterOverflow {
		t.Fatal("overflow changed narrow context")
	}
}

func TestInt32ContextRejectedTransitionIsByteTransactional(t *testing.T) {
	model := contextModel(t)
	before := pos(nnue.White,
		piece(nnue.King, nnue.White, 4),
		piece(nnue.King, nnue.Black, 60),
		piece(nnue.Pawn, nnue.White, 12),
	)
	after := pos(nnue.Black,
		piece(nnue.King, nnue.White, 4),
		piece(nnue.King, nnue.Black, 60),
		piece(nnue.Pawn, nnue.White, 28),
	)
	delta := transition(t, nnue.MoveNormal, before, after,
		[]nnue.PieceOnSquare{before.Pieces[2]},
		[]nnue.PieceOnSquare{after.Pieces[2]},
	)
	narrow, _ := nnue.NewInt32Context(model)
	if err := narrow.Reset(before); err != nil {
		t.Fatal(err)
	}
	bad := delta
	bad.After.Pawns[nnue.White] ^= uint64(1) << 28
	snapshot := *narrow
	if err := narrow.PushDelta(bad); !errors.Is(err, nnue.ErrContext) {
		t.Fatalf("PushDelta error = %v", err)
	}
	if !reflect.DeepEqual(*narrow, snapshot) {
		t.Fatal("failed narrow transition changed context bytes")
	}
}

func TestInt32ContextDenseBoundaryLanesAndFullWeightFallback(t *testing.T) {
	for _, weight := range []int16{math.MaxInt16, math.MinInt16} {
		tensors := new(nnue.Tensors)
		for feature := 0; feature < nnue.InputSize; feature++ {
			for hidden := 0; hidden < nnue.HiddenSize; hidden++ {
				tensors.FeatureWeights[feature][hidden] = weight
			}
		}
		for hidden := 0; hidden < nnue.HiddenSize; hidden++ {
			tensors.FeatureBias[hidden] = weight
			tensors.OutputWeights[hidden] = math.MaxInt16
			tensors.OutputWeights[nnue.HiddenSize+hidden] = math.MinInt16
		}
		model := n4LoadTensors(t, tensors)
		capabilities := model.Capabilities()
		if !capabilities.PortableInt32Accumulator || capabilities.BoundedInt32Output {
			t.Fatalf("full-weight capabilities = %+v", capabilities)
		}
		pieces := make([]nnue.PieceOnSquare, 64)
		for square := range pieces {
			color := nnue.White
			if square&1 != 0 {
				color = nnue.Black
			}
			pieces[square] = piece(nnue.Pawn, color, nnue.Square(square))
		}
		position := nnue.Position{SideToMove: nnue.White, Pieces: pieces}
		wide, _ := nnue.NewContext(model)
		narrow, _ := nnue.NewInt32Context(model)
		if err := wide.Reset(position); err != nil {
			t.Fatal(err)
		}
		if err := narrow.Reset(position); err != nil {
			t.Fatal(err)
		}
		narrowLanes, err := nnue.Int32AccumulatorSnapshotForTest(narrow)
		if err != nil {
			t.Fatal(err)
		}
		want := nnue.MaxAccumulatorValue
		if weight < 0 {
			want = nnue.MinAccumulatorValue
		}
		for perspective := 0; perspective < nnue.PerspectiveCount; perspective++ {
			for hidden := 0; hidden < nnue.HiddenSize; hidden++ {
				if narrowLanes[perspective][hidden] != want {
					t.Fatalf("weight %d lane[%d][%d] = %d, bound = %d", weight, perspective, hidden, narrowLanes[perspective][hidden], want)
				}
			}
		}
		wideScore, _ := wide.Evaluate()
		narrowScore, _ := narrow.Evaluate()
		fullScore, _ := model.Evaluate(position)
		if wideScore != fullScore || narrowScore != fullScore {
			t.Fatalf("full-weight raw int64=%d int32=%d full=%d", wideScore, narrowScore, fullScore)
		}
	}
}

func TestModelArithmeticCapabilityBoundaries(t *testing.T) {
	tests := []struct {
		weight  int16
		bounded bool
	}{
		{127, true},
		{128, true},
		{-128, true},
		{129, false},
		{-129, false},
		{math.MaxInt16, false},
		{math.MinInt16, false},
	}
	for _, test := range tests {
		t.Run(fmt.Sprint(test.weight), func(t *testing.T) {
			tensors := new(nnue.Tensors)
			tensors.OutputWeights[73] = test.weight
			model := n4LoadTensors(t, tensors)
			capabilities := model.Capabilities()
			if !capabilities.PortableInt32Accumulator ||
				capabilities.AccumulatorMin != nnue.MinAccumulatorValue ||
				capabilities.AccumulatorMax != nnue.MaxAccumulatorValue {
				t.Fatalf("accumulator capabilities = %+v", capabilities)
			}
			if capabilities.BoundedInt32Output != test.bounded {
				t.Fatalf("weight %d bounded output = %v, want %v", test.weight, capabilities.BoundedInt32Output, test.bounded)
			}
		})
	}
	if got := (*nnue.Model)(nil).Capabilities(); got != (nnue.Capabilities{}) {
		t.Fatalf("nil model capabilities = %+v", got)
	}
}

func TestInt32ContextsArePrivateRaceAndAllocateNoHotMemory(t *testing.T) {
	model := contextModel(t)
	root := pos(nnue.White,
		piece(nnue.King, nnue.White, 4),
		piece(nnue.King, nnue.Black, 60),
		piece(nnue.Pawn, nnue.White, 12),
	)
	after := pos(nnue.Black,
		piece(nnue.King, nnue.White, 4),
		piece(nnue.King, nnue.Black, 60),
		piece(nnue.Pawn, nnue.White, 28),
	)
	delta := transition(t, nnue.MoveNormal, root, after,
		[]nnue.PieceOnSquare{root.Pieces[2]},
		[]nnue.PieceOnSquare{after.Pieces[2]},
	)
	first, _ := nnue.NewInt32Context(model)
	second, _ := nnue.NewInt32Context(model)
	if first == second {
		t.Fatal("workers share Int32Context")
	}
	run := func(context *nnue.Int32Context, done chan<- error) {
		for i := 0; i < 200; i++ {
			if err := context.Reset(root); err != nil {
				done <- err
				return
			}
			if err := context.PushDelta(delta); err != nil {
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
	done := make(chan error, 2)
	go run(first, done)
	go run(second, done)
	for i := 0; i < 2; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}

	allocations := testing.AllocsPerRun(100, func() {
		if err := first.Reset(root); err != nil {
			panic(err)
		}
		if err := first.PushDelta(delta); err != nil {
			panic(err)
		}
		if _, err := first.Evaluate(); err != nil {
			panic(err)
		}
		if err := first.Pop(); err != nil {
			panic(err)
		}
	})
	if allocations != 0 {
		t.Fatalf("int32 hot context allocations = %g, want 0", allocations)
	}

	wide, _ := nnue.NewContext(model)
	if unsafe.Sizeof(*first) >= unsafe.Sizeof(*wide) {
		t.Fatalf("int32 Context size %d is not smaller than int64 Context %d", unsafe.Sizeof(*first), unsafe.Sizeof(*wide))
	}
}
