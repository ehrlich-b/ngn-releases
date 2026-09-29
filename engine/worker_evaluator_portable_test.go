package engine

import (
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/ehrlich-b/ngn/nnue"
)

func n4pEvaluatorSearchers(t *testing.T, model *nnue.Model) (*SearchEngine, *SearchEngine) {
	t.Helper()
	portable := NewSearchEngine()
	reference := NewSearchEngine()
	if err := portable.SelectNGNV1Evaluator(model); err != nil {
		t.Fatal(err)
	}
	if err := reference.SelectNGNV1Evaluator(model); err != nil {
		t.Fatal(err)
	}
	forced, err := reference.evaluatorModel.newWorkerWithReferenceContext(nil, true)
	if err != nil {
		t.Fatal(err)
	}
	reference.worker.evaluator = forced

	if portable.worker.evaluator.portableContext == nil || portable.worker.evaluator.context != nil {
		t.Fatal("production worker did not select the portable int32 context")
	}
	if reference.worker.evaluator.context == nil || reference.worker.evaluator.portableContext != nil {
		t.Fatal("reference test worker did not select the retained int64 context")
	}
	if portable.worker.evaluator.Identity() != reference.worker.evaluator.Identity() {
		t.Fatal("context implementation changed evaluator identity")
	}
	return portable, reference
}

func TestN4PPortableAndReferenceSearchFixedColdWarmExact(t *testing.T) {
	model, tensors := adapterContextModel(t)
	portable, reference := n4pEvaluatorSearchers(t, model)
	const fen = "r3k2r/p1ppqpb1/bn2pnp1/2pP4/1p2P3/2N2N2/PPQBBPPP/R3K2R w KQkq - 37 1"

	portableFrames := 0
	referenceFrames := 0
	portable.worker.evaluator.transitionObserver = func(pos *Position, worker *workerEvaluator) {
		portableFrames++
		requireWorkerContextLanes(t, worker, tensors, fullPositionForWorkerTest(t, pos))
	}
	reference.worker.evaluator.transitionObserver = func(pos *Position, worker *workerEvaluator) {
		referenceFrames++
		requireWorkerContextLanes(t, worker, tensors, fullPositionForWorkerTest(t, pos))
	}

	for _, lifetime := range []string{"cold", "warm"} {
		t.Run(lifetime, func(t *testing.T) {
			portablePos := n3cPosition(t, fen)
			referencePos := n3cPosition(t, fen)
			got := comparableN3CSearchInfo(portable.SearchFixed(portablePos, 3, nil))
			want := comparableN3CSearchInfo(reference.SearchFixed(referencePos, 3, nil))
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("portable SearchFixed differs from int64 reference:\nportable=%+v\nreference=%+v", got, want)
			}
			requireWorkerRawMatchesFull(t, portable.worker.evaluator, portablePos)
			requireWorkerRawMatchesFull(t, reference.worker.evaluator, referencePos)
		})
	}
	if portableFrames < 4 || referenceFrames < 4 {
		t.Fatalf("insufficient committed-frame coverage: portable=%d reference=%d", portableFrames, referenceFrames)
	}
	if portable.worker.evaluator.nnueDepth() != 0 || reference.worker.evaluator.nnueDepth() != 0 {
		t.Fatalf("context depths portable=%d reference=%d", portable.worker.evaluator.nnueDepth(), reference.worker.evaluator.nnueDepth())
	}
}

func TestN4PPortableAndReferenceIterativeColdWarmCallbacksExact(t *testing.T) {
	model, tensors := adapterContextModel(t)
	portable, reference := n4pEvaluatorSearchers(t, model)
	const fen = "r3k2r/p1ppqpb1/bn2pnp1/2pP4/1p2P3/2N2N2/PPQBBPPP/R3K2R w KQkq - 37 1"

	portableFrames := 0
	portable.worker.evaluator.transitionObserver = func(pos *Position, worker *workerEvaluator) {
		portableFrames++
		requireWorkerContextLanes(t, worker, tensors, fullPositionForWorkerTest(t, pos))
	}

	for _, lifetime := range []string{"cold", "warm"} {
		t.Run(lifetime, func(t *testing.T) {
			gotFinal, gotCallbacks := runN3CIterativeSnapshot(t, portable, fen)
			wantFinal, wantCallbacks := runN3CIterativeSnapshot(t, reference, fen)
			if !reflect.DeepEqual(gotCallbacks, wantCallbacks) {
				t.Fatalf("portable callback trace differs from int64 reference:\nportable=%+v\nreference=%+v", gotCallbacks, wantCallbacks)
			}
			if !reflect.DeepEqual(gotFinal, wantFinal) {
				t.Fatalf("portable iterative result differs from int64 reference:\nportable=%+v\nreference=%+v", gotFinal, wantFinal)
			}
			if len(gotCallbacks) == 0 ||
				gotCallbacks[len(gotCallbacks)-1].PV != gotFinal.PV ||
				gotCallbacks[len(gotCallbacks)-1].PVLength != gotFinal.PVLength ||
				gotCallbacks[len(gotCallbacks)-1].BestMove != gotFinal.BestMove ||
				gotCallbacks[len(gotCallbacks)-1].BestScore != gotFinal.BestScore {
				t.Fatal("portable final result did not preserve its completed callback PV/result")
			}
		})
	}
	if portableFrames < 4 {
		t.Fatalf("portable lane observer ran %d times, want roots and committed children", portableFrames)
	}
}

func TestN4PWorkerHotTransitionsAndEvaluationHaveNoAllocations(t *testing.T) {
	model, _ := adapterContextModel(t)
	cold, err := ngnV1EvaluatorModel(model, 1)
	if err != nil {
		t.Fatal(err)
	}
	worker, err := cold.newWorker(nil)
	if err != nil {
		t.Fatal(err)
	}
	pos := n3cPosition(t, "4k3/8/8/3p4/4P3/8/8/4K3 w - - 0 1")
	if err := worker.Reset(pos); err != nil {
		t.Fatal(err)
	}
	move := adapterLegalMove(t, pos, "e4d5")
	moveTransition, err := worker.PrepareMove(pos, move)
	if err != nil {
		t.Fatal(err)
	}
	nullTransition, err := worker.PrepareNull(pos)
	if err != nil {
		t.Fatal(err)
	}
	var sink int64
	allocations := testing.AllocsPerRun(100, func() {
		undoEP, undoTag, undoClock, _ := pos.MakeMove(move)
		if err := worker.PushMove(pos, moveTransition); err != nil {
			panic(err)
		}
		value, err := worker.evaluateNNUE()
		if err != nil {
			panic(err)
		}
		sink ^= value
		if err := worker.Pop(); err != nil {
			panic(err)
		}
		pos.UnMakeMove(move, undoTag, undoEP, undoClock)

		oldEP := pos.MakeNullMove()
		if err := worker.PushNull(pos, nullTransition); err != nil {
			panic(err)
		}
		value, err = worker.evaluateNNUE()
		if err != nil {
			panic(err)
		}
		sink ^= value
		if err := worker.Pop(); err != nil {
			panic(err)
		}
		pos.UnMakeNullMove(oldEP)
	})
	if allocations != 0 {
		t.Fatalf("worker portable transition/evaluation allocations = %g, want 0", allocations)
	}
	if worker.nnueDepth() != 0 {
		t.Fatalf("worker depth after allocation probe = %d", worker.nnueDepth())
	}
	_ = sink
}

func TestN4PWorkersRemainPrivateUnderConcurrentTransitions(t *testing.T) {
	model, _ := adapterContextModel(t)
	cold, err := ngnV1EvaluatorModel(model, 1)
	if err != nil {
		t.Fatal(err)
	}
	workers := make([]*workerEvaluator, 2)
	positions := make([]*Position, 2)
	moves := make([]Move, 2)
	transitions := make([]workerEvalMove, 2)
	for i := range workers {
		workers[i], err = cold.newWorker(nil)
		if err != nil {
			t.Fatal(err)
		}
		positions[i] = n3cPosition(t, "4k3/8/8/3p4/4P3/8/8/4K3 w - - 0 1")
		if err := workers[i].Reset(positions[i]); err != nil {
			t.Fatal(err)
		}
		moves[i] = adapterLegalMove(t, positions[i], "e4d5")
		transitions[i], err = workers[i].PrepareMove(positions[i], moves[i])
		if err != nil {
			t.Fatal(err)
		}
	}
	if workers[0].portableContext == workers[1].portableContext {
		t.Fatal("workers share mutable portable context")
	}

	errs := make(chan error, len(workers))
	var wg sync.WaitGroup
	for i := range workers {
		wg.Add(1)
		go func(worker *workerEvaluator, pos *Position, move Move, transition workerEvalMove) {
			defer wg.Done()
			for iteration := 0; iteration < 64; iteration++ {
				undoEP, undoTag, undoClock, _ := pos.MakeMove(move)
				if err := worker.PushMove(pos, transition); err != nil {
					errs <- err
					return
				}
				if _, err := worker.evaluateNNUE(); err != nil {
					errs <- err
					return
				}
				if err := worker.Pop(); err != nil {
					errs <- err
					return
				}
				pos.UnMakeMove(move, undoTag, undoEP, undoClock)
			}
			if worker.nnueDepth() != 0 {
				errs <- fmt.Errorf("worker depth = %d", worker.nnueDepth())
			}
		}(workers[i], positions[i], moves[i], transitions[i])
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}
