package engine

import (
	"encoding/binary"
	"errors"
	"math"
	"os"
	"reflect"
	"testing"

	"github.com/ehrlich-b/ngn/countereval"
)

func loadSparseCounterModel(t *testing.T, values map[int]float32) *countereval.Model {
	t.Helper()
	path := writeUCICounterFixture(t, "sparse-counter.nn", 0)
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	var encoded [4]byte
	for index, value := range values {
		binary.LittleEndian.PutUint32(encoded[:], math.Float32bits(value))
		if _, err := file.WriteAt(encoded[:], int64(countereval.LegacyHeaderSize+index*4)); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	input, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	model, _, err := countereval.LoadCounter55Legacy(input)
	if err != nil {
		t.Fatal(err)
	}
	return model
}

func counterHiddenWeightIndex(feature, hidden int) int {
	return feature*countereval.HiddenSize + hidden
}

func counterOutputWeightIndex(hidden int) int {
	return countereval.InputSize*countereval.HiddenSize + countereval.HiddenSize + hidden
}

func loadVaryingCounterModel(t *testing.T) *countereval.Model {
	t.Helper()
	values := make(map[int]float32, countereval.InputSize*3+3)
	for feature := 0; feature < countereval.InputSize; feature++ {
		// Small positive integers keep every accumulator addition exact in float32.
		// The lanes independently vary full feature, square, and color/piece plane.
		values[counterHiddenWeightIndex(feature, 0)] = float32(feature + 1)
		values[counterHiddenWeightIndex(feature, 1)] = float32(feature%64 + 1)
		values[counterHiddenWeightIndex(feature, 2)] = float32(feature/64 + 1)
	}
	values[counterOutputWeightIndex(0)] = 1
	values[counterOutputWeightIndex(1)] = 2
	values[counterOutputWeightIndex(2)] = 3
	return loadSparseCounterModel(t, values)
}

func requireCounterRawMatchesFull(t *testing.T, worker *workerEvaluator, pos *Position) float32 {
	t.Helper()
	if worker.counterContext == nil || worker.model.counter55 == nil {
		t.Fatal("missing Counter context or model")
	}
	board, err := counterBoardFromPosition(pos)
	if err != nil {
		t.Fatal(err)
	}
	if worker.counterContext.Board() != board {
		t.Fatal("incremental Counter board differs from engine bitboards")
	}
	got := worker.counterContext.EvaluateRaw()
	want, err := worker.model.counter55.EvaluateFullRefresh(board)
	if err != nil {
		t.Fatal(err)
	}
	if math.Float32bits(got) != math.Float32bits(want) {
		t.Fatalf("incremental raw bits=%08x full-refresh=%08x", math.Float32bits(got), math.Float32bits(want))
	}
	return got
}

func TestCounterWorkerIdentityPrivateContextsAndTransitions(t *testing.T) {
	const (
		whitePawnE2 = 12
		whitePawnE4 = 28
	)
	model := loadSparseCounterModel(t, map[int]float32{
		counterHiddenWeightIndex(whitePawnE2, 0): 1,
		counterHiddenWeightIndex(whitePawnE4, 0): 5,
		counterOutputWeightIndex(0):              2,
	})
	cold, err := counter55EvaluatorModel(model, 17)
	if err != nil {
		t.Fatal(err)
	}
	same, err := counter55EvaluatorModel(model, 17)
	if err != nil {
		t.Fatal(err)
	}
	if cold.identity != same.identity || cold.identity.backend != evaluatorBackendCounter55 || cold.identity.counterMetadata.SHA256 == "" {
		t.Fatal("Counter immutable identity is incomplete or unstable")
	}
	if cold.identity == hceEvaluatorModel(17).identity {
		t.Fatal("Counter identity aliases HCE")
	}
	if _, err := counter55EvaluatorModel(nil, 17); !errors.Is(err, errWorkerEvaluator) {
		t.Fatalf("nil Counter model error=%v", err)
	}

	first, err := cold.newWorker(nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := cold.newWorker(nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.counterContext == nil || second.counterContext == nil || first.counterContext == second.counterContext {
		t.Fatal("Counter workers do not own distinct contexts")
	}
	pos, err := ParseFEN("4k3/8/8/8/8/8/4P3/4K3 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Reset(pos); err != nil {
		t.Fatal(err)
	}
	if err := second.Reset(pos); err != nil {
		t.Fatal(err)
	}
	rootRaw := requireCounterRawMatchesFull(t, first, pos)
	secondRaw := requireCounterRawMatchesFull(t, second, pos)
	if rootRaw != 2 || secondRaw != rootRaw || first.SearchSTM(pos) != 2 || first.LegacyUndampedSTM(pos) != 2 {
		t.Fatalf("root raw=%v second=%v search=%d emergency=%d", rootRaw, secondRaw, first.SearchSTM(pos), first.LegacyUndampedSTM(pos))
	}

	move := adapterLegalMove(t, pos, "e2e4")
	transition, err := first.PrepareMove(pos, move)
	if err != nil {
		t.Fatal(err)
	}
	undoEP, undoTag, undoClock, legal := pos.MakeMove(move)
	if !legal {
		t.Fatal("generated move rejected")
	}
	if err := first.PushMove(pos, transition); err != nil {
		t.Fatal(err)
	}
	if raw := requireCounterRawMatchesFull(t, first, pos); raw != 10 {
		t.Fatalf("post-move raw=%v", raw)
	}
	if first.SearchSTM(pos) != -10 || first.LegacyUndampedSTM(pos) != -10 {
		t.Fatalf("post-move search=%d emergency=%d", first.SearchSTM(pos), first.LegacyUndampedSTM(pos))
	}
	if got := requireCounterRawMatchesFull(t, second, mustParseCounterPosition(t, "4k3/8/8/8/8/8/4P3/4K3 w - - 0 1")); got != secondRaw {
		t.Fatalf("first worker changed second raw=%v want=%v", got, secondRaw)
	}
	if err := first.Pop(); err != nil {
		t.Fatal(err)
	}
	pos.UnMakeMove(move, undoTag, undoEP, undoClock)
	if raw := requireCounterRawMatchesFull(t, first, pos); raw != rootRaw {
		t.Fatalf("move unwind raw=%v root=%v", raw, rootRaw)
	}

	nullTransition, err := first.PrepareNull(pos)
	if err != nil {
		t.Fatal(err)
	}
	oldEP := pos.MakeNullMove()
	if err := first.PushNull(pos, nullTransition); err != nil {
		t.Fatal(err)
	}
	if raw := requireCounterRawMatchesFull(t, first, pos); raw != rootRaw {
		t.Fatalf("null changed white-perspective raw=%v root=%v", raw, rootRaw)
	}
	if got := first.SearchSTM(pos); got != -1 {
		t.Fatalf("black-to-move damped null score=%d want=-1", got)
	}
	if got := first.LegacyUndampedSTM(pos); got != -2 {
		t.Fatalf("black-to-move undamped null score=%d want=-2", got)
	}
	if err := first.Pop(); err != nil {
		t.Fatal(err)
	}
	pos.UnMakeNullMove(oldEP)
}

func mustParseCounterPosition(t *testing.T, fen string) *Position {
	t.Helper()
	pos, err := ParseFEN(fen)
	if err != nil {
		t.Fatal(err)
	}
	return pos
}

func TestCounterSearchScoreHistoricalOrderAndEmergencyRoute(t *testing.T) {
	cases := []struct {
		name string
		raw  float32
		fen  string
		damp bool
		want int
	}{
		{"white no material", 160, "4k3/8/8/8/8/8/8/4K3 w - - 0 1", true, 160},
		{"white rook material", 160, "4k3/8/8/8/8/8/8/R3K3 w - - 0 1", true, 166},
		{"black rook material", 160, "4k3/8/8/8/8/8/8/R3K3 b - - 0 1", true, -166},
		{"rule50 after material", 160, "4k3/8/8/8/8/8/8/R3K3 w - - 73 1", true, 105},
		{"undamped emergency", 160, "4k3/8/8/8/8/8/8/R3K3 w - - 73 1", false, 166},
		{"truncate raw before scaling", 160.99, "4k3/8/8/8/8/8/8/R3K3 w - - 0 1", true, 166},
		{"negative truncation", -160.99, "4k3/8/8/8/8/8/8/R3K3 w - - 0 1", true, -166},
		{"clip before material", 20000, "4k3/8/8/8/8/8/8/R3K3 w - - 0 1", true, 15562},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := counterSearchScore(test.raw, mustParseCounterPosition(t, test.fen), test.damp); got != test.want {
				t.Fatalf("score=%d want=%d", got, test.want)
			}
		})
	}
	requirePanic(t, func() { counterSearchScore(float32(math.NaN()), mustParseCounterPosition(t, cases[0].fen), true) })
	requirePanic(t, func() { counterSearchScore(0, nil, true) })
}

func TestCounterWorkerRejectsMismatchedMoveAndNullTransactionally(t *testing.T) {
	model := loadSparseCounterModel(t, map[int]float32{counterOutputWeightIndex(0): 1})
	cold, err := counter55EvaluatorModel(model, 3)
	if err != nil {
		t.Fatal(err)
	}
	worker, err := cold.newWorker(nil)
	if err != nil {
		t.Fatal(err)
	}
	pos := mustParseCounterPosition(t, "4k3/8/8/8/8/8/3PP3/4K3 w - - 0 1")
	if err := worker.Reset(pos); err != nil {
		t.Fatal(err)
	}
	beforeBoard := worker.counterContext.Board()
	beforeDepth := worker.counterContext.Depth()
	prepared := adapterLegalMove(t, pos, "e2e4")
	transition, err := worker.PrepareMove(pos, prepared)
	if err != nil {
		t.Fatal(err)
	}
	actual := adapterLegalMove(t, pos, "d2d4")
	undoEP, undoTag, undoClock, _ := pos.MakeMove(actual)
	if err := worker.PushMove(pos, transition); err == nil {
		t.Fatal("mismatched post-move accepted")
	}
	if worker.counterContext.Depth() != beforeDepth || worker.counterContext.Board() != beforeBoard {
		t.Fatal("rejected move changed Counter context")
	}
	pos.UnMakeMove(actual, undoTag, undoEP, undoClock)

	nullTransition, err := worker.PrepareNull(pos)
	if err != nil {
		t.Fatal(err)
	}
	if err := worker.PushNull(pos, nullTransition); err == nil {
		t.Fatal("missing null side flip accepted")
	}
	if worker.counterContext.Depth() != beforeDepth || worker.counterContext.Board() != beforeBoard {
		t.Fatal("rejected null changed Counter context")
	}
}

func TestCounterSearchPushFailureRestoresAdvancedEngineBoard(t *testing.T) {
	model := loadSparseCounterModel(t, map[int]float32{counterHiddenWeightIndex(0, 0): 1e35})
	cold, err := counter55EvaluatorModel(model, 5)
	if err != nil {
		t.Fatal(err)
	}
	worker, err := cold.newWorker(nil)
	if err != nil {
		t.Fatal(err)
	}
	pos := mustParseCounterPosition(t, "4k3/8/8/8/8/8/4P3/4K3 w - - 0 1")
	if err := worker.Reset(pos); err != nil {
		t.Fatal(err)
	}
	for worker.counterContext.Depth() < countereval.CompatibilityFrameCount-1 {
		if err := worker.counterContext.PushNull(); err != nil {
			t.Fatal(err)
		}
	}
	before := snapshotPVPosition(pos)
	beforeDepth := worker.counterContext.Depth()
	beforeCapacity := worker.counterContext.FrameCapacity()
	beforeBoard := worker.counterContext.Board()

	move := adapterLegalMove(t, pos, "e2e4")
	transition, err := worker.PrepareMove(pos, move)
	if err != nil {
		t.Fatal(err)
	}
	undoEP, undoTag, undoClock, legal := pos.MakeMove(move)
	if !legal {
		t.Fatal("generated move rejected")
	}
	requirePanic(t, func() {
		worker.mustPushMadeMove(pos, transition, move, undoTag, undoEP, undoClock)
	})
	if !reflect.DeepEqual(snapshotPVPosition(pos), before) || worker.counterContext.Depth() != beforeDepth ||
		worker.counterContext.FrameCapacity() != beforeCapacity || worker.counterContext.Board() != beforeBoard {
		t.Fatal("failed Counter move push did not restore engine/context transaction")
	}

	nullTransition, err := worker.PrepareNull(pos)
	if err != nil {
		t.Fatal(err)
	}
	oldEP := pos.MakeNullMove()
	requirePanic(t, func() { worker.mustPushMadeNull(pos, nullTransition, oldEP) })
	if !reflect.DeepEqual(snapshotPVPosition(pos), before) || worker.counterContext.Depth() != beforeDepth ||
		worker.counterContext.FrameCapacity() != beforeCapacity || worker.counterContext.Board() != beforeBoard {
		t.Fatal("failed Counter null push did not restore engine/context transaction")
	}
}

func TestCounterWorkerDynamicNullGrowthAndRootRestore(t *testing.T) {
	model := loadVaryingCounterModel(t)
	cold, err := counter55EvaluatorModel(model, 23)
	if err != nil {
		t.Fatal(err)
	}
	worker, err := cold.newWorker(nil)
	if err != nil {
		t.Fatal(err)
	}
	pos := mustParseCounterPosition(t, "4k3/8/8/8/8/8/8/4K3 w - - 0 1")
	before := snapshotPVPosition(pos)
	if err := worker.Reset(pos); err != nil {
		t.Fatal(err)
	}
	rootRaw := requireCounterRawMatchesFull(t, worker, pos)
	oldEP := make([]Square, 129)
	for index := range oldEP {
		transition, err := worker.PrepareNull(pos)
		if err != nil {
			t.Fatalf("prepare null %d: %v", index, err)
		}
		oldEP[index] = pos.MakeNullMove()
		if err := worker.PushNull(pos, transition); err != nil {
			t.Fatalf("push null %d: %v", index, err)
		}
		if raw := requireCounterRawMatchesFull(t, worker, pos); raw != rootRaw {
			t.Fatalf("null %d changed raw=%v root=%v", index, raw, rootRaw)
		}
	}
	if worker.counterContext.Depth() != 129 || worker.counterContext.FrameCapacity() != 256 {
		t.Fatalf("depth/capacity=%d/%d want=129/256", worker.counterContext.Depth(), worker.counterContext.FrameCapacity())
	}
	for index := len(oldEP) - 1; index >= 0; index-- {
		if err := worker.Pop(); err != nil {
			t.Fatalf("pop null %d: %v", index, err)
		}
		pos.UnMakeNullMove(oldEP[index])
	}
	if worker.counterContext.Depth() != 0 || worker.counterContext.FrameCapacity() != 256 {
		t.Fatalf("restored depth/capacity=%d/%d want=0/256", worker.counterContext.Depth(), worker.counterContext.FrameCapacity())
	}
	if !reflect.DeepEqual(snapshotPVPosition(pos), before) {
		t.Fatal("129 null transitions did not restore root position")
	}
	requireCounterRawMatchesFull(t, worker, pos)
}
