package engine

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// SearchControl owns cancellation and the configured node limit for one
// SearchEngine. It is deliberately independent of clocks, TT, histories, and
// evaluator state; those remain separate ownership stages.
type SearchControl struct {
	stopRequested atomic.Bool
	maxNodes      atomic.Uint64
}

func (c *SearchControl) RequestStop()         { c.stopRequested.Store(true) }
func (c *SearchControl) ClearStop()           { c.stopRequested.Store(false) }
func (c *SearchControl) StopRequested() bool  { return c.stopRequested.Load() }
func (c *SearchControl) SetMaxNodes(n uint64) { c.maxNodes.Store(n) }
func (c *SearchControl) MaxNodes() uint64     { return c.maxNodes.Load() }

// searchWorker owns every mutable state family used by one search lane. Pool
// configuration preserves these objects across idle width changes. Iterative
// search may run each object on a private root while sharing only the engine TT.
type searchWorker struct {
	control           SearchControl
	history           workerHistory
	hce               *hceEvaluator
	evaluator         *workerEvaluator
	evaluatorIdentity evaluatorModelIdentity
}

// searchWorkerPool is an immutable publication snapshot. The slice and its
// worker pointers never change after publication; the pointed-to worker state is
// private to that worker. Stop and limit publication can therefore load one
// stable snapshot without taking the search session lock.
type searchWorkerPool struct {
	workers []*searchWorker
}

func newSearchWorkerPool(workers []*searchWorker) *searchWorkerPool {
	snapshot := append([]*searchWorker(nil), workers...)
	return &searchWorkerPool{workers: snapshot}
}

// SearchEngine owns a persistent worker pool, the cold evaluator selection, and
// one total transposition-table allocation. It must not be copied after first
// use. Receiver lifecycle methods serialize through the final callback and are
// intentionally non-reentrant there. Different receivers own independent
// mutable state and may search concurrently under one immutable HCE generation.
type SearchEngine struct {
	sessionMu          sync.Mutex
	worker             searchWorker
	workerPool         atomic.Pointer[searchWorkerPool]
	evaluatorModel     evaluatorModel
	k4EvalScalePercent int64
	tt                 *Cache
	hashMB             int
	ttMode             TTMode
	ttGeneration       uint64
	ttIdentity         evaluatorModelIdentity
	smpHooks           *smpSearchHooks
}

// K4EvalScale reports this receiver's configured owned-network score scale.
// A zero-value SearchEngine preserves the historical 100 percent default.
func (e *SearchEngine) K4EvalScale() int {
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	return int(e.k4EvalScale())
}

func (e *SearchEngine) k4EvalScale() int64 {
	if e.k4EvalScalePercent == 0 {
		return 100
	}
	return e.k4EvalScalePercent
}

// SetK4EvalScale publishes a receiver-local scale while idle. A changed K4
// identity invalidates TT scores and worker histories through the existing
// evaluator lifecycle before any subsequent search or diagnostic uses them.
func (e *SearchEngine) SetK4EvalScale(percent int) error {
	if percent < minK4EvalScalePercent || percent > maxK4EvalScalePercent {
		return fmt.Errorf("K4EvalScale must be between %d and %d", minK4EvalScalePercent, maxK4EvalScalePercent)
	}
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	e.k4EvalScalePercent = int64(percent)
	if e.evaluatorModel.identity.backend == evaluatorBackendNGNK4 {
		e.evaluatorModel.identity.ngnK4ScalePercent = int64(percent)
	}
	return nil
}

const (
	MinHashMB  = 1
	MaxHashMB  = 1024
	MinThreads = 1
	MaxThreads = 64
)

func NewSearchEngine() *SearchEngine {
	e, _ := NewSearchEngineWithHash(DEFAULT_CACHE_SIZE)
	return e
}

// NewSearchEngineWithHash validates the UCI-supported Hash range. Allocation is
// lazy so an unused compatibility receiver does not reserve a second large TT.
func NewSearchEngineWithHash(megabytes int) (*SearchEngine, error) {
	if err := validateHashMB(megabytes); err != nil {
		return nil, err
	}
	engine := &SearchEngine{hashMB: megabytes, ttMode: TTDirect}
	engine.workerPool.Store(newSearchWorkerPool([]*searchWorker{&engine.worker}))
	return engine, nil
}

func validateThreadCount(count int) error {
	if count < MinThreads || count > MaxThreads {
		return fmt.Errorf("Threads must be between %d and %d", MinThreads, MaxThreads)
	}
	return nil
}

func (e *SearchEngine) workerSnapshot() *searchWorkerPool {
	return e.workerPool.Load()
}

func (e *SearchEngine) configuredWorkers() []*searchWorker {
	if pool := e.workerSnapshot(); pool != nil {
		return pool.workers
	}
	// Preserve useful zero-value SearchEngine behavior without publishing from
	// a read-only operation. Constructors always install the normal snapshot.
	return []*searchWorker{&e.worker}
}

// ThreadCount reports the atomically published pool width without taking the
// session lock. Pool configuration itself is serialized as an idle operation.
func (e *SearchEngine) ThreadCount() int {
	if pool := e.workerSnapshot(); pool != nil {
		return len(pool.workers)
	}
	return 1
}

// ConfigureThreads transactionally prepares a persistent worker pool while the
// receiver is idle. It does not launch helper searches. Existing prefix workers
// retain their evaluator caches and histories; the total Hash size remains one
// engine-owned allocation. Crossing the one-worker boundary replaces an already
// allocated TT so direct mode is used for one worker and synchronized mode for
// multiple workers. RequestStop and SetMaxNodes may run lock-free; callers must
// otherwise configure only while idle. Search admission treats a stop retained
// by the primary from an older pool snapshot as authoritative for every worker.
func (e *SearchEngine) ConfigureThreads(count int) error {
	if err := validateThreadCount(count); err != nil {
		return err
	}
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	generation, err := acquireHCEModelUse()
	if err != nil {
		return err
	}
	defer releaseHCEModelUse()

	current := e.configuredWorkers()
	desiredMode := TTDirect
	if count > 1 {
		desiredMode = TTSynchronized
	}
	if len(current) == count && e.ttMode == desiredMode {
		return nil
	}

	model := e.selectedEvaluatorModel(generation)
	nextWorkers := make([]*searchWorker, count)
	retained := len(current)
	if retained > count {
		retained = count
	}
	copy(nextWorkers, current[:retained])
	for i := retained; i < count; i++ {
		candidate := &searchWorker{}
		candidate.control.SetMaxNodes(e.worker.control.MaxNodes())
		if e.worker.control.StopRequested() {
			candidate.control.RequestStop()
		}
		if _, err := prepareWorkerEvaluator(candidate, model, nil, generation); err != nil {
			return err
		}
		nextWorkers[i] = candidate
	}

	modeChanged := e.ttMode != desiredMode
	nextTT := e.tt
	hashMB := e.hashMB
	if hashMB == 0 {
		hashMB = DEFAULT_CACHE_SIZE
	}
	if nextTT != nil && modeChanged {
		nextTT = newCacheWithMode(hashMB, desiredMode)
	}
	nextPool := newSearchWorkerPool(nextWorkers)

	e.hashMB = hashMB
	e.tt = nextTT
	e.ttMode = desiredMode
	if nextTT != nil && modeChanged {
		e.ttGeneration = generation
		e.ttIdentity = model.identity
	}
	e.workerPool.Store(nextPool)
	return nil
}

func validateHashMB(megabytes int) error {
	if megabytes < MinHashMB || megabytes > MaxHashMB {
		return fmt.Errorf("Hash must be between %d and %d MB", MinHashMB, MaxHashMB)
	}
	return nil
}

func validTTMode(mode TTMode) bool { return mode == TTDirect || mode == TTSynchronized }

// prepareTTGeneration runs under sessionMu and an HCE model-use lease. A model
// publication invalidates this receiver lazily before any probe, store, or
// diagnostic can observe stale evaluator-dependent entries.
func (e *SearchEngine) prepareTTGeneration(generation uint64) *Cache {
	identity := e.selectedEvaluatorModel(generation).identity
	if e.hashMB == 0 {
		e.hashMB = DEFAULT_CACHE_SIZE
	}
	if e.tt == nil {
		e.tt = newCacheWithMode(e.hashMB, e.ttMode)
		e.ttGeneration = generation
		e.ttIdentity = identity
	} else if e.ttIdentity != identity {
		e.tt.Clear()
		e.ttGeneration = generation
		e.ttIdentity = identity
	}
	return e.tt
}

// ResizeHash changes this receiver's table while it is idle. The old table is
// discarded only after the replacement allocation succeeds.
func (e *SearchEngine) ResizeHash(megabytes int) error {
	if err := validateHashMB(megabytes); err != nil {
		return err
	}
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	generation := mustAcquireHCEModelUse()
	defer releaseHCEModelUse()
	replacement := newCacheWithMode(megabytes, e.ttMode)
	e.tt = replacement
	e.hashMB = megabytes
	e.ttGeneration = generation
	e.ttIdentity = e.selectedEvaluatorModel(generation).identity
	return nil
}

func (e *SearchEngine) HashSize() int {
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	if e.hashMB == 0 {
		return DEFAULT_CACHE_SIZE
	}
	return e.hashMB
}

// SetTTMode is an idle configuration operation. Changing mode starts with an
// empty table; no helper search is started by this ownership slice.
func (e *SearchEngine) SetTTMode(mode TTMode) error {
	if !validTTMode(mode) {
		return fmt.Errorf("invalid TT mode %d", mode)
	}
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	generation := mustAcquireHCEModelUse()
	defer releaseHCEModelUse()
	if e.ttMode == mode {
		e.prepareTTGeneration(generation)
		return nil
	}
	e.ttMode = mode
	if e.hashMB == 0 {
		e.hashMB = DEFAULT_CACHE_SIZE
	}
	e.tt = newCacheWithMode(e.hashMB, mode)
	e.ttGeneration = generation
	e.ttIdentity = e.selectedEvaluatorModel(generation).identity
	return nil
}

type TTDiagnostics struct {
	SizeMB     int
	Hashfull   int
	Generation uint64
	Mode       TTMode
}

func (e *SearchEngine) TTDiagnostics() TTDiagnostics {
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	generation := mustAcquireHCEModelUse()
	defer releaseHCEModelUse()
	tt := e.prepareTTGeneration(generation)
	return TTDiagnostics{SizeMB: e.hashMB, Hashfull: tt.Consumed(), Generation: generation, Mode: e.ttMode}
}

func (e *SearchEngine) AdvanceHashAge() {
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	generation := mustAcquireHCEModelUse()
	defer releaseHCEModelUse()
	e.prepareTTGeneration(generation).AdvanceAge()
}

func (e *SearchEngine) ClearHash() {
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	generation := mustAcquireHCEModelUse()
	defer releaseHCEModelUse()
	e.prepareTTGeneration(generation).Clear()
}

func (e *SearchEngine) TTProbe(hash uint64) (Move, int16, int8, NodeType, bool, bool) {
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	generation := mustAcquireHCEModelUse()
	defer releaseHCEModelUse()
	return e.prepareTTGeneration(generation).Get(hash)
}

func (e *SearchEngine) TTStore(hash uint64, move Move, eval int16, depth int8, nodeType NodeType, ttPv bool) {
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	generation := mustAcquireHCEModelUse()
	defer releaseHCEModelUse()
	e.prepareTTGeneration(generation).Set(hash, move, eval, depth, nodeType, ttPv)
}

// NewGame clears game-scoped search state for every configured worker while
// retaining same-generation HCE full and pawn caches. A model mismatch still
// invalidates evaluator and heuristic state during worker preparation.
func (e *SearchEngine) NewGame() {
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	generation := mustAcquireHCEModelUse()
	defer releaseHCEModelUse()
	e.prepareTTGeneration(generation).Clear()
	model := e.selectedEvaluatorModel(generation)
	for _, worker := range e.configuredWorkers() {
		if _, err := prepareWorkerEvaluator(worker, model, nil, generation); err != nil {
			panic(err)
		}
		worker.history.ClearHistoryTable()
		worker.history.ClearKillerMoves()
		worker.history.ClearCounterMoves()
		worker.history.SetLastMovePlayed(EmptyMove)
	}
}

func (e *SearchEngine) RequestStop() {
	if pool := e.workerSnapshot(); pool != nil {
		for _, worker := range pool.workers {
			worker.control.RequestStop()
		}
		return
	}
	e.worker.control.RequestStop()
}

func (e *SearchEngine) ClearStop() {
	if pool := e.workerSnapshot(); pool != nil {
		for _, worker := range pool.workers {
			worker.control.ClearStop()
		}
		return
	}
	e.worker.control.ClearStop()
}

func (e *SearchEngine) StopRequested() bool {
	if pool := e.workerSnapshot(); pool != nil {
		for _, worker := range pool.workers {
			if worker.control.StopRequested() {
				return true
			}
		}
		return false
	}
	return e.worker.control.StopRequested()
}

func (e *SearchEngine) SetMaxNodes(n uint64) {
	if pool := e.workerSnapshot(); pool != nil {
		for _, worker := range pool.workers {
			worker.control.SetMaxNodes(n)
		}
		return
	}
	e.worker.control.SetMaxNodes(n)
}

func (e *SearchEngine) MaxNodes() uint64 { return e.worker.control.MaxNodes() }

// prepareHCEGeneration runs only while sessionMu and a process-model use lease
// are held. A changed generation invalidates every worker-private heuristic and
// cache. Root accumulators are rebuilt unconditionally because any Position may
// have been created under an older process-wide PST generation.
func (w *searchWorker) prepareHCEGeneration(pos *Position, generation uint64) *hceEvaluator {
	if w.hce == nil {
		// A new worker has no stale private state to clear. Recording the current
		// generation avoids touching its large zeroed tables on first admission.
		w.hce = &hceEvaluator{seenGeneration: generation}
	} else if w.hce.seenGeneration != generation {
		w.hce.clearForGeneration(generation)
		w.history = workerHistory{}
	}
	if pos != nil {
		pos.Board.recomputeAccumulator()
	}
	return w.hce
}

func (e *SearchEngine) ClearHistoryTable() {
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	generation := mustAcquireHCEModelUse()
	defer releaseHCEModelUse()
	e.mustPreparePrimaryEvaluator(nil, generation)
	e.worker.history.ClearHistoryTable()
}
func (e *SearchEngine) ClearKillerMoves() {
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	generation := mustAcquireHCEModelUse()
	defer releaseHCEModelUse()
	e.mustPreparePrimaryEvaluator(nil, generation)
	e.worker.history.ClearKillerMoves()
}
func (e *SearchEngine) ClearCounterMoves() {
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	generation := mustAcquireHCEModelUse()
	defer releaseHCEModelUse()
	e.mustPreparePrimaryEvaluator(nil, generation)
	e.worker.history.ClearCounterMoves()
}
func (e *SearchEngine) SetLastMovePlayed(move Move) {
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	generation := mustAcquireHCEModelUse()
	defer releaseHCEModelUse()
	e.mustPreparePrimaryEvaluator(nil, generation)
	e.worker.history.SetLastMovePlayed(move)
}
func (e *SearchEngine) GetLastMovePlayed() Move {
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	generation := mustAcquireHCEModelUse()
	defer releaseHCEModelUse()
	e.mustPreparePrimaryEvaluator(nil, generation)
	return e.worker.history.GetLastMovePlayed()
}

func (e *SearchEngine) evaluateForPlayerCached(pos *Position) int {
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	generation := mustAcquireHCEModelUse()
	defer releaseHCEModelUse()
	evaluator := e.worker.prepareHCEGeneration(pos, generation)
	return evaluator.SearchSTM(pos)
}

func (e *SearchEngine) Search(pos *Position, depth int) *SearchInfo {
	return e.SearchIterativeDeepening(pos, depth, nil)
}

func (e *SearchEngine) SearchIterativeDeepening(pos *Position, maxDepth int, timeManager *TimeManager) *SearchInfo {
	return e.SearchIterativeDeepeningWithCallback(pos, maxDepth, timeManager, nil)
}

func (e *SearchEngine) SearchIterativeDeepeningWithCallback(pos *Position, maxDepth int, timeManager *TimeManager, callback DepthCallback) *SearchInfo {
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	e.ClearStop()
	generation := mustAcquireHCEModelUse()
	defer releaseHCEModelUse()
	tt := e.prepareTTGeneration(generation)
	return e.searchIterativeDeepeningLocked(pos, maxDepth, timeManager, callback, nil, generation, tt)
}

// searchIterativeDeepeningPrepared is used by UCI after it has cleared the
// control and published searching=true. It must not clear again: a stop may
// arrive after go launches the goroutine but before the goroutine acquires this
// session lock.
func (e *SearchEngine) searchIterativeDeepeningPrepared(pos *Position, maxDepth int, timeManager *TimeManager, callback DepthCallback) *SearchInfo {
	return e.searchIterativeDeepeningPreparedWithWidth(pos, maxDepth, timeManager, callback, nil)
}

func (e *SearchEngine) searchIterativeDeepeningPreparedWithWidth(pos *Position, maxDepth int, timeManager *TimeManager, callback DepthCallback, onWidth func(int)) *SearchInfo {
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	generation := mustAcquireHCEModelUse()
	defer releaseHCEModelUse()
	tt := e.prepareTTGeneration(generation)
	return e.searchIterativeDeepeningLocked(pos, maxDepth, timeManager, callback, onWidth, generation, tt)
}

func (e *SearchEngine) SearchFixed(pos *Position, depth int, timeManager *TimeManager) *SearchInfo {
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	e.ClearStop()
	generation := mustAcquireHCEModelUse()
	defer releaseHCEModelUse()
	tt := e.prepareTTGeneration(generation)
	evaluator := e.mustPreparePrimaryEvaluator(pos, generation)
	return searchFixedUnsafe(pos, depth, timeManager, &e.worker.control, &e.worker.history, evaluator, tt)
}

// ExtractPVFromTT returns a generation-coherent snapshot outside a search
// callback. Callbacks must consume SearchInfo.PV because the receiver session is
// intentionally held while callbacks run.
func (e *SearchEngine) ExtractPVFromTT(pos *Position, firstMove Move, maxLength int) []Move {
	e.sessionMu.Lock()
	defer e.sessionMu.Unlock()
	generation := mustAcquireHCEModelUse()
	defer releaseHCEModelUse()
	return extractPVFromTT(e.prepareTTGeneration(generation), pos, firstMove, maxLength)
}

// Compatibility wrappers retain the package API and its single default search
// lifetime. New code that needs independent control should own a SearchEngine.
var defaultSearchEngine = NewSearchEngine()

func RequestStop()          { defaultSearchEngine.RequestStop() }
func ClearStop()            { defaultSearchEngine.ClearStop() }
func IsStopRequested() bool { return defaultSearchEngine.StopRequested() }
func SetMaxNodes(n uint64)  { defaultSearchEngine.SetMaxNodes(n) }
func MaxNodes() uint64      { return defaultSearchEngine.MaxNodes() }
func Search(pos *Position, depth int) *SearchInfo {
	return defaultSearchEngine.Search(pos, depth)
}
func SearchIterativeDeepening(pos *Position, maxDepth int, timeManager *TimeManager) *SearchInfo {
	return defaultSearchEngine.SearchIterativeDeepening(pos, maxDepth, timeManager)
}
func SearchIterativeDeepeningWithCallback(pos *Position, maxDepth int, timeManager *TimeManager, callback DepthCallback) *SearchInfo {
	return defaultSearchEngine.SearchIterativeDeepeningWithCallback(pos, maxDepth, timeManager, callback)
}
func SearchFixed(pos *Position, depth int, timeManager *TimeManager) *SearchInfo {
	return defaultSearchEngine.SearchFixed(pos, depth, timeManager)
}

func ResizeHash(megabytes int) error          { return defaultSearchEngine.ResizeHash(megabytes) }
func ClearHash()                              { defaultSearchEngine.ClearHash() }
func AdvanceHashAge()                         { defaultSearchEngine.AdvanceHashAge() }
func HashSize() int                           { return defaultSearchEngine.HashSize() }
func TranspositionDiagnostics() TTDiagnostics { return defaultSearchEngine.TTDiagnostics() }
func StoreTransposition(hash uint64, move Move, eval int16, depth int8, nodeType NodeType, ttPv bool) {
	defaultSearchEngine.TTStore(hash, move, eval, depth, nodeType, ttPv)
}
func ProbeTransposition(hash uint64) (Move, int16, int8, NodeType, bool, bool) {
	return defaultSearchEngine.TTProbe(hash)
}
func ExtractPVFromTT(pos *Position, firstMove Move, maxLength int) []Move {
	return defaultSearchEngine.ExtractPVFromTT(pos, firstMove, maxLength)
}
