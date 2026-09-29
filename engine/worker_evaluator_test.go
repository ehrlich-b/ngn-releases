package engine

import (
	"bytes"
	"errors"
	"math"
	"math/big"
	"reflect"
	"testing"

	"github.com/ehrlich-b/ngn/nnue"
)

func loadEvaluatorTensors(t *testing.T, tensors *nnue.Tensors) *nnue.Model {
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

func fullPositionForWorkerTest(t *testing.T, pos *Position) nnue.Position {
	t.Helper()
	var buffer nnueRootBuffer
	full, err := buffer.position(pos)
	if err != nil {
		t.Fatal(err)
	}
	return full
}

func rawWorkerScore(t *testing.T, worker *workerEvaluator) int64 {
	t.Helper()
	raw, err := worker.evaluateNNUE()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func requireWorkerContextLanes(t *testing.T, worker *workerEvaluator, tensors *nnue.Tensors, position nnue.Position) {
	t.Helper()
	var value reflect.Value
	switch {
	case worker.context != nil && worker.portableContext == nil:
		value = reflect.ValueOf(worker.context).Elem()
	case worker.portableContext != nil && worker.context == nil:
		value = reflect.ValueOf(worker.portableContext).Elem()
	default:
		t.Fatal("worker must own exactly one NNUE context")
	}
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

func requireWorkerRawMatchesFull(t *testing.T, worker *workerEvaluator, pos *Position) int64 {
	t.Helper()
	raw := rawWorkerScore(t, worker)
	fullRaw, err := worker.model.ngnV1.Evaluate(fullPositionForWorkerTest(t, pos))
	if err != nil {
		t.Fatal(err)
	}
	if raw != fullRaw {
		t.Fatalf("incremental raw = %d, full-model raw = %d", raw, fullRaw)
	}
	return raw
}

func requirePanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected fail-fast panic")
		}
	}()
	fn()
}

func TestEvaluatorModelIdentityIsCompleteAndWorkerContextsArePrivate(t *testing.T) {
	model, tensors := adapterContextModel(t)
	firstModel, err := ngnV1EvaluatorModel(model, 11)
	if err != nil {
		t.Fatal(err)
	}
	sameModel, err := ngnV1EvaluatorModel(model, 11)
	if err != nil {
		t.Fatal(err)
	}
	if firstModel.identity != sameModel.identity {
		t.Fatal("same validated model and HCE generation produced different identity")
	}
	nextHCEGeneration, err := ngnV1EvaluatorModel(model, 12)
	if err != nil {
		t.Fatal(err)
	}
	if firstModel.identity == nextHCEGeneration.identity {
		t.Fatal("HCE generation change did not change NNUE identity")
	}

	changedTensors := *tensors
	changedTensors.OutputBias++
	changed := loadEvaluatorTensors(t, &changedTensors)
	changedModel, err := ngnV1EvaluatorModel(changed, 11)
	if err != nil {
		t.Fatal(err)
	}
	if firstModel.identity == changedModel.identity {
		t.Fatal("file/payload change did not change NNUE identity")
	}
	if firstModel.identity == hceEvaluatorModel(11).identity {
		t.Fatal("backend kind is absent from model identity")
	}
	if hceEvaluatorModel(11).identity == hceEvaluatorModel(12).identity {
		t.Fatal("HCE generation is absent from HCE identity")
	}
	if _, err := ngnV1EvaluatorModel(nil, 11); !errors.Is(err, errWorkerEvaluator) {
		t.Fatalf("nil model error = %v", err)
	}

	first, err := firstModel.newWorker(nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := firstModel.newWorker(nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.context != nil || second.context != nil ||
		first.portableContext == nil || second.portableContext == nil {
		t.Fatal("validated NGN-v1 workers did not select portable int32 contexts")
	}
	if first.portableContext == second.portableContext {
		t.Fatal("workers share mutable NNUE Int32Context")
	}
	root, err := ParseFEN("4k3/8/8/8/8/8/4P3/4K3 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Reset(root); err != nil {
		t.Fatal(err)
	}
	if err := second.Reset(root); err != nil {
		t.Fatal(err)
	}
	secondRaw := rawWorkerScore(t, second)
	move := adapterLegalMove(t, root, "e2e4")
	transition, err := first.PrepareMove(root, move)
	if err != nil {
		t.Fatal(err)
	}
	undoEP, undoTag, undoClock, _ := root.MakeMove(move)
	if err := first.PushMove(root, transition); err != nil {
		t.Fatal(err)
	}
	requireWorkerRawMatchesFull(t, first, root)
	if got := rawWorkerScore(t, second); got != secondRaw {
		t.Fatalf("first worker transition changed second worker raw score: got %d want %d", got, secondRaw)
	}
	if err := first.Pop(); err != nil {
		t.Fatal(err)
	}
	root.UnMakeMove(move, undoTag, undoEP, undoClock)
}

func TestNNUEStaticScorePolicyAllInt64AndCorrectionBound(t *testing.T) {
	rawScores := []int64{
		math.MinInt64,
		math.MinInt64 + 1,
		-25001,
		-25000,
		-24999,
		-257,
		-256,
		-255,
		-1,
		0,
		1,
		255,
		256,
		257,
		24999,
		25000,
		25001,
		math.MaxInt64 - 1,
		math.MaxInt64,
	}
	halfMoves := []uint8{0, 1, 99, 100, 254, 255}
	budget := big.NewInt(FiftyMoveDampBudget)
	for _, raw := range rawScores {
		for _, halfMove := range halfMoves {
			product := new(big.Int).Mul(big.NewInt(raw), big.NewInt(int64(FiftyMoveDampBudget)-int64(halfMove)))
			wantBig := new(big.Int).Quo(product, budget)
			limit := big.NewInt(nnueStaticEvalLimit)
			negativeLimit := new(big.Int).Neg(new(big.Int).Set(limit))
			if wantBig.Cmp(limit) > 0 {
				wantBig.Set(limit)
			} else if wantBig.Cmp(negativeLimit) < 0 {
				wantBig.Set(negativeLimit)
			}
			if got, want := nnueSearchScore(raw, halfMove), int(wantBig.Int64()); got != want {
				t.Fatalf("nnueSearchScore(%d, %d) = %d, exact integer oracle = %d", raw, halfMove, got, want)
			}
		}
		if got := clampNNUEStatic(raw); got < -int(nnueStaticEvalLimit) || got > int(nnueStaticEvalLimit) {
			t.Fatalf("emergency clamp(%d) escaped non-mate band: %d", raw, got)
		}
	}

	maxCorrectionTerm := corrHistLimit * 6245 / 131072
	maxAllCorrections := 3 * maxCorrectionTerm
	if maxCorrectionTerm != 48 || maxAllCorrections != 144 {
		t.Fatalf("correction bound changed: per-term=%d total=%d", maxCorrectionTerm, maxAllCorrections)
	}
	if int(nnueStaticEvalLimit)+maxAllCorrections >= MATE_IN_MAX {
		t.Fatalf("static + correction bound %d enters mate band %d", int(nnueStaticEvalLimit)+maxAllCorrections, MATE_IN_MAX)
	}
}

func TestNGNV1WorkerEvaluatorRawSearchEmergencyAndTransitions(t *testing.T) {
	model, _ := adapterContextModel(t)
	cold, err := ngnV1EvaluatorModel(model, 3)
	if err != nil {
		t.Fatal(err)
	}
	worker, err := cold.newWorker(nil)
	if err != nil {
		t.Fatal(err)
	}
	pos, err := ParseFEN("r3k2r/p1ppqpb1/bn2pnp1/2pP4/1p2P3/2N2N2/PPQBBPPP/R3K2R w KQkq - 73 1")
	if err != nil {
		t.Fatal(err)
	}
	if err := worker.Reset(pos); err != nil {
		t.Fatal(err)
	}
	raw := requireWorkerRawMatchesFull(t, worker, pos)
	if got, want := worker.SearchSTM(pos), nnueSearchScore(raw, pos.HalfMoveClock); got != want {
		t.Fatalf("SearchSTM = %d, want %d", got, want)
	}
	if got, want := worker.LegacyUndampedSTM(pos), clampNNUEStatic(raw); got != want {
		t.Fatalf("LegacyUndampedSTM = %d, want %d", got, want)
	}

	move := adapterLegalMove(t, pos, "e2a6")
	transition, err := worker.PrepareMove(pos, move)
	if err != nil {
		t.Fatal(err)
	}
	undoEP, undoTag, undoClock, _ := pos.MakeMove(move)
	if err := worker.PushMove(pos, transition); err != nil {
		t.Fatal(err)
	}
	requireWorkerRawMatchesFull(t, worker, pos)
	if err := worker.Pop(); err != nil {
		t.Fatal(err)
	}
	pos.UnMakeMove(move, undoTag, undoEP, undoClock)
	if got := requireWorkerRawMatchesFull(t, worker, pos); got != raw {
		t.Fatalf("raw after move unwind = %d, root raw = %d", got, raw)
	}

	nullTransition, err := worker.PrepareNull(pos)
	if err != nil {
		t.Fatal(err)
	}
	oldEP := pos.MakeNullMove()
	if err := worker.PushNull(pos, nullTransition); err != nil {
		t.Fatal(err)
	}
	nullRaw := requireWorkerRawMatchesFull(t, worker, pos)
	if nullRaw == raw {
		t.Fatalf("asymmetric model did not distinguish null STM: both %d", raw)
	}
	if err := worker.Pop(); err != nil {
		t.Fatal(err)
	}
	pos.UnMakeNullMove(oldEP)
	if got := requireWorkerRawMatchesFull(t, worker, pos); got != raw {
		t.Fatalf("raw after null unwind = %d, root raw = %d", got, raw)
	}
}

func TestNGNV1WorkerEvaluatorRejectsUnbridgedPostMoveTransactionally(t *testing.T) {
	model, _ := adapterContextModel(t)
	cold, err := ngnV1EvaluatorModel(model, 1)
	if err != nil {
		t.Fatal(err)
	}
	worker, err := cold.newWorker(nil)
	if err != nil {
		t.Fatal(err)
	}
	pos, err := ParseFEN("4k3/8/8/8/8/8/3PP3/4K3 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	if err := worker.Reset(pos); err != nil {
		t.Fatal(err)
	}
	raw := rawWorkerScore(t, worker)
	prepared := adapterLegalMove(t, pos, "e2e4")
	transition, err := worker.PrepareMove(pos, prepared)
	if err != nil {
		t.Fatal(err)
	}
	actual := adapterLegalMove(t, pos, "d2d4")
	undoEP, undoTag, undoClock, _ := pos.MakeMove(actual)
	if err := worker.PushMove(pos, transition); !errors.Is(err, errNNUETransition) {
		t.Fatalf("mismatched post-move error = %v", err)
	}
	if worker.nnueDepth() != 0 || rawWorkerScore(t, worker) != raw {
		t.Fatal("failed post-move validation changed Context")
	}
	pos.UnMakeMove(actual, undoTag, undoEP, undoClock)

	nullTransition, err := worker.PrepareNull(pos)
	if err != nil {
		t.Fatal(err)
	}
	if err := worker.PushNull(pos, nullTransition); !errors.Is(err, errNNUETransition) {
		t.Fatalf("missing null side flip error = %v", err)
	}
	if worker.nnueDepth() != 0 || rawWorkerScore(t, worker) != raw {
		t.Fatal("failed post-null validation changed Context")
	}
}

func TestWorkerEvaluatorHCEDelegatesExactM2Routes(t *testing.T) {
	generation := mustAcquireHCEModelUse()
	defer releaseHCEModelUse()

	pos, err := ParseFEN("4k3/8/8/8/8/8/4P3/4K3 w - - 83 1")
	if err != nil {
		t.Fatal(err)
	}
	pos.Board.recomputeAccumulator()
	hce := new(hceEvaluator)
	worker, err := hceEvaluatorModel(generation).newWorker(hce)
	if err != nil {
		t.Fatal(err)
	}
	if err := worker.Reset(pos); err != nil {
		t.Fatal(err)
	}
	if got, want := worker.SearchSTM(pos), hce.SearchSTM(pos); got != want {
		t.Fatalf("HCE SearchSTM delegation = %d, direct = %d", got, want)
	}
	if got, want := worker.LegacyUndampedSTM(pos), hce.LegacyUndampedSTM(pos); got != want {
		t.Fatalf("HCE LegacyUndampedSTM delegation = %d, direct = %d", got, want)
	}
	move := adapterLegalMove(t, pos, "e2e4")
	if transition, err := worker.PrepareMove(pos, move); err != nil || transition.active {
		t.Fatalf("HCE PrepareMove = %+v, %v", transition, err)
	}
	if transition, err := worker.PrepareNull(pos); err != nil || transition.active {
		t.Fatalf("HCE PrepareNull = %+v, %v", transition, err)
	}
	if err := worker.Pop(); err != nil {
		t.Fatalf("HCE Pop: %v", err)
	}
}

func TestNGNV1WorkerEvaluatorClampsExtremeLegalModelsAndFailsFastUnreset(t *testing.T) {
	for _, outputWeight := range []int16{math.MaxInt16, math.MinInt16} {
		tensors := new(nnue.Tensors)
		for hidden := 0; hidden < nnue.HiddenSize; hidden++ {
			tensors.FeatureBias[hidden] = math.MaxInt16
			tensors.OutputWeights[hidden] = outputWeight
			tensors.OutputWeights[nnue.HiddenSize+hidden] = outputWeight
		}
		model := loadEvaluatorTensors(t, tensors)
		cold, err := ngnV1EvaluatorModel(model, 9)
		if err != nil {
			t.Fatal(err)
		}
		worker, err := cold.newWorker(nil)
		if err != nil {
			t.Fatal(err)
		}
		pos, err := ParseFEN("4k3/8/8/8/8/8/8/4K3 w - - 0 1")
		if err != nil {
			t.Fatal(err)
		}
		requirePanic(t, func() { worker.SearchSTM(pos) })
		if err := worker.Reset(pos); err != nil {
			t.Fatal(err)
		}
		raw := requireWorkerRawMatchesFull(t, worker, pos)
		if raw >= -nnueStaticEvalLimit && raw <= nnueStaticEvalLimit {
			t.Fatalf("extreme legal model raw %d did not exceed static band", raw)
		}
		want := int(nnueStaticEvalLimit)
		if raw < 0 {
			want = -want
		}
		if got := worker.SearchSTM(pos); got != want {
			t.Fatalf("extreme SearchSTM = %d, want %d (raw %d)", got, want, raw)
		}
		if got := worker.LegacyUndampedSTM(pos); got != want {
			t.Fatalf("extreme emergency = %d, want %d (raw %d)", got, want, raw)
		}
	}
}
