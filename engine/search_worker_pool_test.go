package engine

import (
	"errors"
	"testing"
)

func TestConfigureThreadsPublishesPersistentPoolAndOneTotalHash(t *testing.T) {
	searcher, err := NewSearchEngineWithHash(1)
	if err != nil {
		t.Fatal(err)
	}
	pos, err := ParseFEN("7k/8/8/8/3Q4/8/8/K7 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	searcher.SearchFixed(pos, 1, nil)

	poison := NewMove(A1, A2, WhiteKing, NoPiece, NoType, 0)
	searcher.worker.history.lastMovePlayed = poison
	searcher.worker.hce.full[7] = evalCacheEntry{key: 11, val: 13}
	primaryEvaluator := searcher.worker.evaluator
	primaryHCE := searcher.worker.hce
	primary := &searcher.worker
	directTT := searcher.tt
	initialPool := searcher.workerSnapshot()
	const key = uint64(0x4d3462)
	searcher.TTStore(key, poison, 21, 4, Exact, false)

	if err := searcher.ConfigureThreads(4); err != nil {
		t.Fatal(err)
	}
	pool4 := searcher.workerSnapshot()
	if got := searcher.ThreadCount(); got != 4 {
		t.Fatalf("ThreadCount = %d, want 4", got)
	}
	if initialPool == pool4 || len(initialPool.workers) != 1 {
		t.Fatal("published pool mutated the prior immutable snapshot")
	}
	if pool4.workers[0] != primary || searcher.worker.evaluator != primaryEvaluator || searcher.worker.hce != primaryHCE {
		t.Fatal("pool growth replaced primary worker evaluator state")
	}
	if searcher.worker.history.lastMovePlayed != poison || searcher.worker.hce.full[7] != (evalCacheEntry{key: 11, val: 13}) {
		t.Fatal("pool growth discarded primary worker history or HCE cache")
	}
	for i, worker := range pool4.workers {
		if worker == nil || worker.evaluator == nil || worker.hce == nil {
			t.Fatalf("worker %d was not fully prepared", i)
		}
		for j := 0; j < i; j++ {
			if worker == pool4.workers[j] || worker.evaluator == pool4.workers[j].evaluator || worker.hce == pool4.workers[j].hce {
				t.Fatalf("worker %d aliases worker %d mutable state", i, j)
			}
		}
	}
	if searcher.hashMB != 1 || searcher.tt == nil || searcher.tt == directTT || searcher.ttMode != TTSynchronized || searcher.tt.mode != TTSynchronized {
		t.Fatal("pool growth did not preserve one total Hash allocation in synchronized mode")
	}
	if _, _, _, _, hit, _ := searcher.TTProbe(key); hit {
		t.Fatal("direct-to-synchronized mode transition retained an old TT entry")
	}

	synchronizedTT := searcher.tt
	retained := append([]*searchWorker(nil), pool4.workers[:3]...)
	if err := searcher.ConfigureThreads(3); err != nil {
		t.Fatal(err)
	}
	pool3 := searcher.workerSnapshot()
	if searcher.tt != synchronizedTT || searcher.ttMode != TTSynchronized {
		t.Fatal("same-mode resize replaced the shared TT")
	}
	for i, worker := range retained {
		if pool3.workers[i] != worker {
			t.Fatalf("same-mode resize replaced retained worker %d", i)
		}
	}
	if len(pool4.workers) != 4 || pool4.workers[3] == nil {
		t.Fatal("pool shrink mutated the previously published snapshot")
	}

	primaryEvaluator = searcher.worker.evaluator
	primaryHCE = searcher.worker.hce
	if err := searcher.ConfigureThreads(1); err != nil {
		t.Fatal(err)
	}
	pool1 := searcher.workerSnapshot()
	if len(pool1.workers) != 1 || pool1.workers[0] != primary || searcher.worker.evaluator != primaryEvaluator || searcher.worker.hce != primaryHCE {
		t.Fatal("pool shrink replaced primary worker state")
	}
	if searcher.hashMB != 1 || searcher.tt == synchronizedTT || searcher.ttMode != TTDirect || searcher.tt.mode != TTDirect {
		t.Fatal("one-worker pool did not restore one total direct-mode Hash")
	}
	directAgain := searcher.tt
	samePool := searcher.workerSnapshot()
	if err := searcher.ConfigureThreads(1); err != nil {
		t.Fatal(err)
	}
	if searcher.workerSnapshot() != samePool || searcher.tt != directAgain {
		t.Fatal("same-count configuration was not a no-op")
	}
}

func TestWorkerPoolControlPublicationAndNewGameState(t *testing.T) {
	searcher, err := NewSearchEngineWithHash(1)
	if err != nil {
		t.Fatal(err)
	}
	searcher.SetMaxNodes(73)
	searcher.RequestStop()
	if err := searcher.ConfigureThreads(3); err != nil {
		t.Fatal(err)
	}
	pool := searcher.workerSnapshot()
	if len(pool.workers) != 3 {
		t.Fatalf("pool size = %d, want 3", len(pool.workers))
	}
	for i, worker := range pool.workers {
		if !worker.control.StopRequested() || worker.control.MaxNodes() != 73 {
			t.Fatalf("worker %d control was not inherited", i)
		}
	}

	searcher.sessionMu.Lock()
	generation := mustAcquireHCEModelUse()
	model := searcher.selectedEvaluatorModel(generation)
	for i, worker := range pool.workers {
		if _, err := prepareWorkerEvaluator(worker, model, nil, generation); err != nil {
			releaseHCEModelUse()
			searcher.sessionMu.Unlock()
			t.Fatalf("prepare worker %d: %v", i, err)
		}
	}
	releaseHCEModelUse()
	searcher.sessionMu.Unlock()
	for i, worker := range pool.workers {
		if !worker.control.StopRequested() {
			t.Fatalf("worker %d preparation cleared published cancellation", i)
		}
		worker.history.lastMovePlayed = NewMove(A1, A2, WhiteKing, NoPiece, NoType, 0)
		worker.hce.full[i] = evalCacheEntry{key: uint64(i + 1), val: int32(i + 2)}
	}

	searcher.ClearStop()
	for i, worker := range pool.workers {
		if worker.control.StopRequested() {
			t.Fatalf("worker %d did not observe ClearStop", i)
		}
	}
	searcher.NewGame()
	for i, worker := range pool.workers {
		if worker.history.lastMovePlayed != EmptyMove {
			t.Fatalf("worker %d game history survived NewGame", i)
		}
		if worker.hce.full[i] != (evalCacheEntry{key: uint64(i + 1), val: int32(i + 2)}) {
			t.Fatalf("worker %d same-generation HCE cache was cleared by NewGame", i)
		}
	}
}

func TestConfigureThreadsRejectsInvalidAndFailedCandidateTransactionally(t *testing.T) {
	searcher, err := NewSearchEngineWithHash(1)
	if err != nil {
		t.Fatal(err)
	}
	searcher.TTStore(91, EmptyMove, 0, 1, Exact, false)
	beforePool := searcher.workerSnapshot()
	beforeTT := searcher.tt
	beforeMode := searcher.ttMode
	beforeHash := searcher.hashMB
	for _, count := range []int{MinThreads - 1, MaxThreads + 1} {
		if err := searcher.ConfigureThreads(count); err == nil {
			t.Fatalf("ConfigureThreads(%d) succeeded", count)
		}
		if searcher.workerSnapshot() != beforePool || searcher.tt != beforeTT || searcher.ttMode != beforeMode || searcher.hashMB != beforeHash {
			t.Fatalf("ConfigureThreads(%d) changed published state", count)
		}
	}

	priorModel := searcher.evaluatorModel
	searcher.evaluatorModel = evaluatorModel{identity: evaluatorModelIdentity{
		backend: evaluatorBackend(99), adapterRevision: evaluatorAdapterRevision,
	}}
	if err := searcher.ConfigureThreads(2); !errors.Is(err, errWorkerEvaluator) {
		t.Fatalf("invalid candidate error = %v", err)
	}
	if searcher.workerSnapshot() != beforePool || searcher.tt != beforeTT || searcher.ttMode != beforeMode || searcher.hashMB != beforeHash {
		t.Fatal("failed candidate construction changed published pool or TT")
	}
	searcher.evaluatorModel = priorModel
}

func TestConfigureThreadsPreparesPrivateSelectedEvaluatorForNewWorkers(t *testing.T) {
	model, _ := adapterContextModel(t)
	searcher, err := NewSearchEngineWithHash(1)
	if err != nil {
		t.Fatal(err)
	}
	if err := searcher.SelectNGNV1Evaluator(model); err != nil {
		t.Fatal(err)
	}
	primaryEvaluator := searcher.worker.evaluator
	if err := searcher.ConfigureThreads(3); err != nil {
		t.Fatal(err)
	}
	pool := searcher.workerSnapshot()
	if pool.workers[0].evaluator != primaryEvaluator {
		t.Fatal("NNUE pool growth replaced the selected primary evaluator")
	}
	for i, worker := range pool.workers {
		if worker.evaluator == nil || worker.evaluator.Identity().backend != evaluatorBackendNGNV1 || worker.evaluator.context != nil || worker.evaluator.portableContext == nil {
			t.Fatalf("worker %d did not receive selected NNUE backend", i)
		}
		if worker.hce != nil {
			t.Fatalf("worker %d allocated unused HCE cache for NNUE", i)
		}
		for j := 0; j < i; j++ {
			if worker.evaluator.portableContext == pool.workers[j].evaluator.portableContext {
				t.Fatalf("workers %d and %d share NNUE Context", i, j)
			}
		}
	}
	observed := make([]int, len(pool.workers))
	for i, worker := range pool.workers {
		index := i
		worker.evaluator.transitionObserver = func(*Position, *workerEvaluator) {
			observed[index]++
		}
	}
	pos, err := ParseFEN("4k3/8/8/8/8/8/4P3/4K3 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	if _, backend, err := searcher.EvaluateSelected(pos); err != nil || backend != EvaluatorBackendNGNV1Name {
		t.Fatalf("EvaluateSelected after pool configuration = backend %q, err %v", backend, err)
	}
	beforeSearch := observed[0]
	searcher.SearchFixed(pos, 2, nil)
	if observed[0] <= beforeSearch {
		t.Fatal("primary worker did not execute the one-thread search path")
	}
	for i := 1; i < len(observed); i++ {
		if observed[i] != 0 {
			t.Fatalf("configured helper %d executed before M4c", i)
		}
	}

}
