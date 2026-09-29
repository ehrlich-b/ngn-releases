package engine

import (
	"fmt"
	"runtime/debug"
)

// smpSearchHooks are deterministic package-test observation points. They are
// configured only while the receiver is idle and run under its search session.
type smpSearchHooks struct {
	afterRootCopy  func(worker int)
	afterReset     func(worker int)
	beforeLaunch   func(worker int)
	workerStarted  func(worker int)
	workerFinished func(worker int, info *SearchInfo)
}

type searchWorkerPanic struct {
	worker int
	value  any
	stack  []byte
}

func (p *searchWorkerPanic) Error() string {
	return fmt.Sprintf("search helper %d panicked: %v", p.worker, p.value)
}

func (p *searchWorkerPanic) Unwrap() error {
	err, _ := p.value.(error)
	return err
}

type searchWorkerResult struct {
	worker int
	info   *SearchInfo
	panic  *searchWorkerPanic
}

func (e *SearchEngine) setupCancelled(pool *searchWorkerPool, timeManager *TimeManager) bool {
	if timeManager != nil && timeManager.SetupDeadlineExceeded() {
		e.RequestStop()
		return true
	}
	for _, worker := range pool.workers {
		if worker.control.StopRequested() {
			e.RequestStop()
			return true
		}
	}
	return false
}

func requestHelperStops(workers []*searchWorker) {
	for _, worker := range workers {
		worker.control.RequestStop()
	}
}

func collectSearchWorkerResults(results <-chan searchWorkerResult, launched, workerCount int) ([]*SearchInfo, *searchWorkerPanic) {
	infos := make([]*SearchInfo, workerCount)
	panics := make([]*searchWorkerPanic, workerCount)
	for i := 0; i < launched; i++ {
		result := <-results
		infos[result.worker] = result.info
		panics[result.worker] = result.panic
	}
	for worker := 1; worker < len(panics); worker++ {
		if panics[worker] != nil {
			return infos, panics[worker]
		}
	}
	return infos, nil
}

func (e *SearchEngine) searchIterativeDeepeningLocked(pos *Position, maxDepth int, timeManager *TimeManager, callback DepthCallback, onWidth func(int), generation uint64, tt *Cache) *SearchInfo {
	pool := e.workerSnapshot()
	if pool == nil || len(pool.workers) == 1 || pool.workers[0].control.MaxNodes() != 0 {
		evaluator := e.mustPreparePrimaryEvaluator(pos, generation)
		if onWidth != nil {
			onWidth(1)
		}
		return searchIterativeDeepeningUnsafe(pos, maxDepth, timeManager, callback, &e.worker.control, &e.worker.history, evaluator, tt)
	}
	return e.searchLazySMPLocked(pos, maxDepth, timeManager, callback, onWidth, generation, tt, pool)
}

func (e *SearchEngine) searchLazySMPLocked(pos *Position, maxDepth int, timeManager *TimeManager, callback DepthCallback, onWidth func(int), generation uint64, tt *Cache, pool *searchWorkerPool) *SearchInfo {
	workerCount := len(pool.workers)
	model := e.selectedEvaluatorModel(generation)
	roots := make([]*Position, workerCount)
	publishers := make([]searchNodePublisher, workerCount)
	roots[0] = pos

	// Age belongs to the admitted iterative search, not to an individual lane.
	tt.AdvanceAge()

	// The primary reset remains mandatory even for a cancellation that arrived
	// before setup; it preserves the admitted search's root/evaluator contract.
	cancelled := e.setupCancelled(pool, timeManager)
	if _, err := prepareWorkerEvaluator(pool.workers[0], model, pos, generation); err != nil {
		panic(err)
	}
	if hooks := e.smpHooks; hooks != nil && hooks.afterReset != nil {
		hooks.afterReset(0)
	}
	cancelled = e.setupCancelled(pool, timeManager) || cancelled

	for i := 1; i < workerCount && !cancelled; i++ {
		if e.setupCancelled(pool, timeManager) {
			cancelled = true
			break
		}
		roots[i] = pos.Copy()
		if hooks := e.smpHooks; hooks != nil && hooks.afterRootCopy != nil {
			hooks.afterRootCopy(i)
		}
		if e.setupCancelled(pool, timeManager) {
			cancelled = true
			break
		}
		if _, err := prepareWorkerEvaluator(pool.workers[i], model, roots[i], generation); err != nil {
			panic(err)
		}
		if hooks := e.smpHooks; hooks != nil && hooks.afterReset != nil {
			hooks.afterReset(i)
		}
		cancelled = e.setupCancelled(pool, timeManager)
	}
	masterOptions := iterativeSearchOptions{effectiveThreads: workerCount, nodePublisher: &publishers[0]}
	if cancelled || e.setupCancelled(pool, timeManager) {
		masterOptions.effectiveThreads = 1
		if onWidth != nil {
			onWidth(1)
		}
		return searchIterativeDeepeningWorkerUnsafe(pos, maxDepth, timeManager, callback,
			&pool.workers[0].control, &pool.workers[0].history, pool.workers[0].evaluator, tt, masterOptions)
	}

	results := make(chan searchWorkerResult, workerCount-1)
	launched := 0
	joined := false
	defer func() {
		if joined {
			return
		}
		recovered := recover()
		e.RequestStop()
		_, helperPanic := collectSearchWorkerResults(results, launched, workerCount)
		joined = true
		if recovered != nil {
			panic(recovered)
		}
		if helperPanic != nil {
			panic(helperPanic)
		}
	}()
	for i := 1; i < workerCount; i++ {
		if hooks := e.smpHooks; hooks != nil && hooks.beforeLaunch != nil {
			hooks.beforeLaunch(i)
		}
		if e.setupCancelled(pool, timeManager) {
			break
		}
		worker := pool.workers[i]
		root := roots[i]
		publisher := &publishers[i]
		launched++
		go func(index int) {
			result := searchWorkerResult{worker: index}
			defer func() {
				if recovered := recover(); recovered != nil {
					result.panic = &searchWorkerPanic{worker: index, value: recovered, stack: debug.Stack()}
					e.RequestStop()
				}
				results <- result
			}()
			if hooks := e.smpHooks; hooks != nil && hooks.workerStarted != nil {
				hooks.workerStarted(index)
			}
			result.info = searchIterativeDeepeningWorkerUnsafe(root, maxDepth, nil, nil,
				&worker.control, &worker.history, worker.evaluator, tt,
				iterativeSearchOptions{effectiveThreads: workerCount, nodePublisher: publisher})
			if hooks := e.smpHooks; hooks != nil && hooks.workerFinished != nil {
				hooks.workerFinished(index, result.info)
			}
		}(i)
	}

	activeCount := launched + 1
	masterOptions.effectiveThreads = activeCount
	if onWidth != nil {
		onWidth(activeCount)
	}
	masterCallback := callback
	if callback != nil {
		masterCallback = func(info *SearchInfo) {
			snapshot := *info
			for i := 1; i <= launched; i++ {
				nodes, qnodes := publishers[i].load()
				snapshot.Nodes += nodes
				snapshot.QNodes += qnodes
			}
			snapshot.EffectiveThreads = activeCount
			callback(&snapshot)
		}
	}

	var masterInfo *SearchInfo
	var masterPanic any
	func() {
		defer func() { masterPanic = recover() }()
		if hooks := e.smpHooks; hooks != nil && hooks.workerStarted != nil {
			hooks.workerStarted(0)
		}
		masterInfo = searchIterativeDeepeningWorkerUnsafe(pos, maxDepth, timeManager, masterCallback,
			&pool.workers[0].control, &pool.workers[0].history, pool.workers[0].evaluator, tt, masterOptions)
		if hooks := e.smpHooks; hooks != nil && hooks.workerFinished != nil {
			hooks.workerFinished(0, masterInfo)
		}
	}()

	if masterPanic != nil {
		e.RequestStop()
	} else {
		requestHelperStops(pool.workers[1 : launched+1])
	}

	helperResults, helperPanic := collectSearchWorkerResults(results, launched, workerCount)
	joined = true
	if masterPanic != nil {
		panic(masterPanic)
	}
	if helperPanic != nil {
		panic(helperPanic)
	}
	for i := 1; i <= launched; i++ {
		addSearchMonotonicCounters(masterInfo, helperResults[i])
	}
	masterInfo.EffectiveThreads = activeCount
	return masterInfo
}
