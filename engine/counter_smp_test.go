package engine

import (
	"sync"
	"testing"
	"time"

	"github.com/ehrlich-b/ngn/countereval"
)

func TestLazySMPCounterContextsStayPrivateBalancedAndReusable(t *testing.T) {
	requireLazySMPCounterContexts(t, loadVaryingCounterModel(t), 3)
}

func requireLazySMPCounterContexts(
	t *testing.T,
	model *countereval.Model,
	workerCount int,
) (*SearchEngine, []*searchWorker, []*workerEvaluator, []*countereval.SearchContext) {
	t.Helper()
	searcher := newSMPTestEngine(t, workerCount)
	if err := searcher.SelectCounter55Evaluator(model); err != nil {
		t.Fatal(err)
	}
	pos := controlTestPosition(t)
	workers := searcher.configuredWorkers()
	transitioned := make(chan int, len(workers))
	release := make(chan struct{})
	var releaseOnce sync.Once
	firstPush := make([]sync.Once, len(workers))
	evaluators := make([]*workerEvaluator, len(workers))
	contexts := make([]*countereval.SearchContext, len(workers))
	identities := make([]evaluatorModelIdentity, len(workers))
	frameBoards := make([]countereval.Board, len(workers))
	positionBoards := make([]countereval.Board, len(workers))
	frameErrors := make([]error, len(workers))
	searcher.smpHooks = &smpSearchHooks{afterReset: func(index int) {
		evaluator := workers[index].evaluator
		evaluators[index] = evaluator
		contexts[index] = evaluator.counterContext
		identities[index] = evaluator.Identity()
		evaluator.transitionObserver = func(observedPos *Position, observed *workerEvaluator) {
			if observed.nnueDepth() <= 0 {
				return
			}
			firstPush[index].Do(func() {
				frameBoards[index] = observed.counterContext.Board()
				positionBoards[index], frameErrors[index] = counterBoardFromPosition(observedPos)
				transitioned <- index
				<-release
			})
		}
	}}

	done := make(chan *SearchInfo, 1)
	finished := make(chan struct{})
	t.Cleanup(func() {
		searcher.RequestStop()
		releaseOnce.Do(func() { close(release) })
		select {
		case <-finished:
		case <-time.After(10 * time.Second):
			t.Errorf("Counter helper session survived cleanup")
		}
	})
	go func() {
		defer close(finished)
		done <- searcher.Search(pos, 2)
	}()
	seen := make([]bool, len(workers))
	for range workers {
		seen[waitSMPIndex(t, transitioned, "Counter pushed frames")] = true
	}
	for index, worker := range workers {
		evaluator := worker.evaluator
		if !seen[index] || evaluator == nil || evaluator != evaluators[index] {
			t.Fatalf("Counter worker %d did not enter its captured evaluator", index)
		}
		if identities[index].backend != evaluatorBackendCounter55 || evaluator.model.counter55 != model {
			t.Fatalf("Counter worker %d model/identity mismatch", index)
		}
		if contexts[index] == nil || evaluator.counterContext != contexts[index] || evaluator.nnueDepth() <= 0 {
			t.Fatalf("Counter worker %d missing or unpushed context", index)
		}
		if frameErrors[index] != nil || frameBoards[index] != positionBoards[index] {
			t.Fatalf("Counter worker %d committed-frame board mismatch: err=%v", index, frameErrors[index])
		}
		for prior := 0; prior < index; prior++ {
			if evaluator == evaluators[prior] || contexts[index] == contexts[prior] {
				t.Fatalf("Counter workers %d and %d share evaluator or context", index, prior)
			}
		}
	}

	searcher.RequestStop()
	releaseOnce.Do(func() { close(release) })
	var result *SearchInfo
	select {
	case result = <-done:
	case <-time.After(10 * time.Second):
		searcher.RequestStop()
		t.Fatal("Counter helper session did not join")
	}
	if !result.Stopped || result.EffectiveThreads != workerCount {
		t.Fatalf("Counter stopped/effective=%v/%d want=true/%d", result.Stopped, result.EffectiveThreads, workerCount)
	}
	for index, worker := range workers {
		worker.evaluator.transitionObserver = nil
		if worker.evaluator != evaluators[index] || worker.evaluator.nnueDepth() != 0 {
			t.Fatalf("Counter worker %d evaluator/context not balanced after join", index)
		}
		requireCounterRawMatchesFull(t, worker.evaluator, pos)
	}

	searcher.smpHooks = nil
	next := mustParseCounterPosition(t, "4k3/8/8/8/8/8/4P3/4K3 w - - 0 1")
	warm := searcher.Search(next, 1)
	if warm.EffectiveThreads != workerCount {
		t.Fatalf("Counter warm effective=%d want=%d", warm.EffectiveThreads, workerCount)
	}
	for index, worker := range workers {
		if worker.evaluator != evaluators[index] || worker.evaluator.counterContext != contexts[index] || worker.evaluator.nnueDepth() != 0 {
			t.Fatalf("Counter warm root reset did not reuse balanced worker %d", index)
		}
		requireCounterRawMatchesFull(t, worker.evaluator, next)
	}

	poisonMove := NewMove(A1, A2, WhiteKing, NoPiece, NoType, 0)
	const newGameKey = uint64(0xc0550003)
	for _, worker := range workers {
		poisonN3CWorkerHistory(&worker.history, poisonMove)
	}
	searcher.TTStore(newGameKey, poisonMove, 17, 3, Exact, false)
	searcher.NewGame()
	if _, _, _, _, hit, _ := searcher.TTProbe(newGameKey); hit {
		t.Fatal("Counter NewGame retained stale TT entry")
	}
	for index, worker := range workers {
		if worker.evaluator != evaluators[index] || worker.evaluator.counterContext != contexts[index] || worker.history != (workerHistory{}) {
			t.Fatalf("Counter NewGame changed context or retained history for worker %d", index)
		}
	}
	postNewGame := searcher.Search(next.Copy(), 1)
	if postNewGame.EffectiveThreads != workerCount {
		t.Fatalf("Counter post-NewGame effective=%d want=%d", postNewGame.EffectiveThreads, workerCount)
	}
	for index, worker := range workers {
		if worker.evaluator != evaluators[index] || worker.evaluator.counterContext != contexts[index] || worker.evaluator.nnueDepth() != 0 {
			t.Fatalf("Counter post-NewGame root reset did not reuse balanced worker %d", index)
		}
		requireCounterRawMatchesFull(t, worker.evaluator, next)
	}
	return searcher, workers, evaluators, contexts
}
