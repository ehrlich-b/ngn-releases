package engine

import (
	"errors"
	"reflect"
	"sync"
	"testing"
)

func mustHCEPosition(t *testing.T, fen string) *Position {
	t.Helper()
	pos, err := ParseFEN(fen)
	if err != nil {
		t.Fatalf("ParseFEN(%q): %v", fen, err)
	}
	return pos
}

func changedQueenModel() (saved, changed TexelModelExport) {
	saved = ExportTexelModel()
	changed = saved
	changed.Values = append([]int(nil), saved.Values...)
	changed.Values[texelMatMGIndex(Queen)] += 113
	changed.Values[texelMatEGIndex(Queen)] += 113
	return saved, changed
}

func restoreHCEModelAndTT(t *testing.T, saved TexelModelExport, savedTT *SearchEngine) {
	t.Helper()
	if err := ApplyTexelModel(saved); err != nil {
		t.Errorf("restore HCE model: %v", err)
	}
	defaultSearchEngine = savedTT
}

func requireBusyPanic(t *testing.T, fn func()) {
	t.Helper()
	var recovered any
	func() {
		defer func() { recovered = recover() }()
		fn()
	}()
	if recovered == nil {
		t.Fatal("operation returned instead of failing fast")
	}
	err, ok := recovered.(error)
	if !ok || !errors.Is(err, ErrHCEModelBusy) {
		t.Fatalf("panic = %#v, want ErrHCEModelBusy", recovered)
	}
}

func TestHCEEvaluatorPreservesSearchAndLegacyScorePolicies(t *testing.T) {
	generation := mustAcquireHCEModelUse()
	defer releaseHCEModelUse()

	const placement = "7k/8/8/8/3Q4/8/8/K7"
	for _, tc := range []struct {
		name string
		turn string
		pov  int
	}{{"white", "w", 1}, {"black", "b", -1}} {
		t.Run(tc.name, func(t *testing.T) {
			pos := mustHCEPosition(t, placement+" "+tc.turn+" - - 128 1")
			pos.Board.recomputeAccumulator()
			rawWhite := evaluateWhiteLeased(&pos.Board, nil)
			evaluator := &hceEvaluator{}
			evaluator.clearForGeneration(generation)

			wantSearch := tc.pov*(rawWhite*(FiftyMoveDampBudget-128)/FiftyMoveDampBudget) + TempoBonus
			if got := evaluator.SearchSTM(pos); got != wantSearch {
				t.Fatalf("SearchSTM = %d, want %d", got, wantSearch)
			}
			wantLegacy := tc.pov*rawWhite + TempoBonus
			if got := evaluator.LegacyUndampedSTM(pos); got != wantLegacy {
				t.Fatalf("LegacyUndampedSTM = %d, want %d", got, wantLegacy)
			}
		})
	}
}

func TestAlphaBetaSafetyReturnsRemainFreshAndUndamped(t *testing.T) {
	generation := mustAcquireHCEModelUse()
	defer releaseHCEModelUse()

	pos := mustHCEPosition(t, "7k/8/8/8/3Q4/8/8/K7 w - - 255 1")
	pos.Board.recomputeAccumulator()
	evaluator := &hceEvaluator{}
	evaluator.clearForGeneration(generation)
	h := pos.Hash()
	evaluator.full[h&(1<<evalCacheBits-1)] = evalCacheEntry{key: h, val: -12345}
	want := evaluator.LegacyUndampedSTM(pos)

	cases := []struct {
		name  string
		depth int
		ply   int
	}{{"depth above maximum", MaximumDepth + 1, 0}, {"negative depth", -1, 0}, {"ply at maximum", 1, MaximumDepth}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info := &SearchInfo{control: &SearchControl{}, evaluator: testHCEWorkerEvaluator(evaluator), tt: NewCache(1)}
			if got := alphaBetaPV(pos, tc.depth, tc.ply, -INFINITY, INFINITY, true, true, false, info); got != want {
				t.Fatalf("alphaBetaPV = %d, want fresh undamped %d", got, want)
			}
		})
	}
}

func TestHCEPawnCacheAndNilPathAreNumericallyIdentical(t *testing.T) {
	_ = mustAcquireHCEModelUse()
	defer releaseHCEModelUse()

	pos := mustHCEPosition(t, "4k3/2p5/8/3P4/8/2P1P3/8/4K3 w - - 0 1")
	pos.Board.recomputeAccumulator()
	var cache pawnCache
	uncached := evaluateUnsafeWithPawnCache(&pos.Board, nil)
	first := evaluateUnsafeWithPawnCache(&pos.Board, &cache)
	second := evaluateUnsafeWithPawnCache(&pos.Board, &cache)
	if first != uncached || second != uncached {
		t.Fatalf("pawn cache changed evaluation: nil=%d miss=%d hit=%d", uncached, first, second)
	}
}

func TestHCEEvaluatorCachesAreIndependentUnderConcurrentUse(t *testing.T) {
	generation := mustAcquireHCEModelUse()
	defer releaseHCEModelUse()

	base := mustHCEPosition(t, "4k3/2p5/8/3P4/8/2P1P3/3Q4/4K3 w - - 0 1")
	base.Board.recomputeAccumulator()
	clean := &hceEvaluator{}
	poisoned := &hceEvaluator{}
	clean.clearForGeneration(generation)
	poisoned.clearForGeneration(generation)

	h := base.Hash()
	poisoned.full[h&(1<<evalCacheBits-1)] = evalCacheEntry{key: h, val: -12345}
	want := clean.SearchSTM(base.Copy())
	if got := poisoned.SearchSTM(base.Copy()); got == want {
		t.Fatalf("full-cache poison did not create an observable witness: %d", got)
	}

	wp := base.Board.GetBitboardOf(WhitePawn)
	bp := base.Board.GetBitboardOf(BlackPawn)
	poisoned.pawns[pawnCacheIndex(wp, bp)] = pawnCacheEntry{
		wp: wp, bp: bp, valid: true, wPassedW: 200, bPassedW: -200,
	}
	freshLegacy := (&hceEvaluator{}).LegacyUndampedSTM(base.Copy())
	if got := poisoned.LegacyUndampedSTM(base.Copy()); got == freshLegacy {
		t.Fatalf("pawn-cache poison did not create an observable witness: %d", got)
	}

	var wg sync.WaitGroup
	results := [2]int{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 128; i++ {
			results[0] = poisoned.SearchSTM(base.Copy())
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 128; i++ {
			results[1] = clean.SearchSTM(base.Copy())
		}
	}()
	wg.Wait()
	if results[0] == results[1] || results[1] != want {
		t.Fatalf("private cache results crossed: poisoned=%d clean=%d want=%d", results[0], results[1], want)
	}
}

func TestApplyTexelModelFailureAndBusyAreAtomic(t *testing.T) {
	saved, changed := changedQueenModel()
	savedTT := defaultSearchEngine
	defaultSearchEngine, _ = NewSearchEngineWithHash(1)
	t.Cleanup(func() { restoreHCEModelAndTT(t, saved, savedTT) })

	const key = uint64(0x123456789)
	defaultSearchEngine.TTStore(key, EmptyMove, 77, 4, Exact, false)
	hceModelLifecycle.mu.Lock()
	beforeGeneration := hceModelLifecycle.generation
	hceModelLifecycle.mu.Unlock()

	invalid := changed
	invalid.Names = append([]string(nil), changed.Names...)
	invalid.Names[0] += "_invalid"
	if err := ApplyTexelModel(invalid); err == nil || errors.Is(err, ErrHCEModelBusy) {
		t.Fatalf("invalid Apply error = %v", err)
	}
	if got := ExportTexelModel(); !reflect.DeepEqual(got, saved) {
		t.Fatal("failed validation changed the live HCE model")
	}
	if _, _, _, _, hit, _ := defaultSearchEngine.TTProbe(key); !hit {
		t.Fatal("failed validation cleared the TT")
	}

	_, err := acquireHCEModelUse()
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyTexelModel(changed); !errors.Is(err, ErrHCEModelBusy) {
		t.Fatalf("busy Apply error = %v, want ErrHCEModelBusy", err)
	}
	if _, _, err := TryTexelGradientTune(nil, nil, TexelGradientConfig{}); !errors.Is(err, ErrHCEModelBusy) {
		t.Fatalf("busy gradient error = %v, want ErrHCEModelBusy", err)
	}
	if _, err := TryTexelTune(nil, nil, 1, 1, 0, nil, nil); !errors.Is(err, ErrHCEModelBusy) {
		t.Fatalf("busy coordinate error = %v, want ErrHCEModelBusy", err)
	}
	releaseHCEModelUse()

	if got := ExportTexelModel(); !reflect.DeepEqual(got, saved) {
		t.Fatal("busy mutation changed the live HCE model")
	}
	hceModelLifecycle.mu.Lock()
	afterGeneration := hceModelLifecycle.generation
	hceModelLifecycle.mu.Unlock()
	if afterGeneration != beforeGeneration {
		t.Fatalf("failed/busy Apply changed generation: before=%d after=%d", beforeGeneration, afterGeneration)
	}
	if _, _, _, _, hit, _ := defaultSearchEngine.TTProbe(key); !hit {
		t.Fatal("busy Apply cleared the TT")
	}
}

func TestHCEReadEntriesFailFastInsideTunerProgress(t *testing.T) {
	pos := mustHCEPosition(t, "7k/6Q1/6K1/8/8/8/8/8 b - - 0 1")

	requireBusyPanic(t, func() {
		_, _ = TryTexelTune(nil, nil, 1, 1, 1, nil, func(int, float64, int) {
			_ = Evaluate(&pos.Board)
		})
	})
	requireBusyPanic(t, func() {
		_, _ = TryTexelTune(nil, nil, 1, 1, 1, nil, func(int, float64, int) {
			NewSearchEngine().SearchFixed(pos.Copy(), 1, nil)
		})
	})
	requireBusyPanic(t, func() {
		_, _ = TryTexelTune(nil, nil, 1, 1, 1, nil, func(int, float64, int) {
			_ = ExportTexelModel()
		})
	})

	generation, err := acquireHCEModelUse()
	if err != nil {
		t.Fatalf("tuner panic left mutation active: %v", err)
	}
	if generation == 0 {
		t.Fatal("published generation is zero")
	}
	releaseHCEModelUse()
}

func TestApplyFromSearchCallbackReturnsBusy(t *testing.T) {
	saved, changed := changedQueenModel()
	savedTT := defaultSearchEngine
	defaultSearchEngine, _ = NewSearchEngineWithHash(1)
	t.Cleanup(func() { restoreHCEModelAndTT(t, saved, savedTT) })

	var callbackErr error
	searcher, _ := NewSearchEngineWithHash(1)
	info := searcher.SearchIterativeDeepeningWithCallback(
		mustHCEPosition(t, "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"),
		1, nil, func(*SearchInfo) { callbackErr = ApplyTexelModel(changed) },
	)
	if info.Depth != 1 || info.Stopped {
		t.Fatalf("search did not finish: depth=%d stopped=%v", info.Depth, info.Stopped)
	}
	if !errors.Is(callbackErr, ErrHCEModelBusy) {
		t.Fatalf("callback Apply error = %v, want ErrHCEModelBusy", callbackErr)
	}
	if got := ExportTexelModel(); !reflect.DeepEqual(got, saved) {
		t.Fatal("reentrant Apply changed the live HCE model")
	}
}

func TestModelSwapRebuildsOldBoardsWithoutMutatingDirectCaller(t *testing.T) {
	saved, changed := changedQueenModel()
	savedTT := defaultSearchEngine
	defaultSearchEngine, _ = NewSearchEngineWithHash(1)
	t.Cleanup(func() { restoreHCEModelAndTT(t, saved, savedTT) })

	const directFEN = "7k/8/8/8/3Q4/8/8/K7 w - - 0 1"
	oldDirect := mustHCEPosition(t, directFEN)
	oldBoard := oldDirect.Board
	before := Evaluate(&oldDirect.Board)

	const mateFEN = "7k/6Q1/6K1/8/8/8/8/8 b - - 0 1"
	oldFirst := mustHCEPosition(t, mateFEN)
	oldSecond := mustHCEPosition(t, mateFEN)
	searcher, _ := NewSearchEngineWithHash(1)
	searcher.SearchFixed(oldFirst, 1, nil)

	if err := ApplyTexelModel(changed); err != nil {
		t.Fatal(err)
	}
	freshFirst := mustHCEPosition(t, mateFEN)
	freshSecond := mustHCEPosition(t, mateFEN)
	searcher.SearchFixed(freshFirst, 1, nil)
	searcher.SearchFixed(oldSecond, 1, nil)
	if oldSecond.Board != freshSecond.Board {
		t.Fatalf("same-generation old root accumulator was not rebuilt: old=%+v fresh=%+v", oldSecond.Board, freshSecond.Board)
	}

	gotOld := Evaluate(&oldDirect.Board)
	freshDirect := mustHCEPosition(t, directFEN)
	gotFresh := Evaluate(&freshDirect.Board)
	if gotOld != gotFresh || gotOld == before {
		t.Fatalf("direct old-board evaluation: before=%d old=%d fresh=%d", before, gotOld, gotFresh)
	}
	if oldDirect.Board != oldBoard {
		t.Fatal("public Evaluate mutated caller Bitboard")
	}
}

func TestGenerationLazilyInvalidatesEveryReceiver(t *testing.T) {
	saved, changed := changedQueenModel()
	savedTT := defaultSearchEngine
	defaultSearchEngine, _ = NewSearchEngineWithHash(1)
	t.Cleanup(func() { restoreHCEModelAndTT(t, saved, savedTT) })

	searcher, _ := NewSearchEngineWithHash(1)
	searcher.SearchFixed(mustHCEPosition(t, "7k/6Q1/6K1/8/8/8/8/8 b - - 0 1"), 1, nil)
	move := NewMove(A1, A2, WhiteKing, NoPiece, NoType, 0)
	searcher.worker.history.historyTable[WhiteKing][A2] = 1
	searcher.worker.history.continuationHistory[WhiteKing][A1][WhiteKing][A2] = 2
	searcher.worker.history.followupHistory[WhiteKing][A1][WhiteKing][A2] = 3
	searcher.worker.history.pawnCorrectionHistory[White][1] = 4
	searcher.worker.history.nonPawnCorrectionHistory[Black][2] = 5
	searcher.worker.history.minorCorrectionHistory[White][3] = 6
	searcher.worker.history.captureHistory[WhiteKing][A2][BlackPawn] = 7
	searcher.worker.history.killerMoves[0][0] = move
	searcher.worker.history.counterMoves[A1][A2] = move
	searcher.worker.history.lastMovePlayed = move
	searcher.worker.hce.full[0] = evalCacheEntry{key: 1, val: 2}
	searcher.worker.hce.pawns[0] = pawnCacheEntry{valid: true, wp: 1}

	const oldTTKey = uint64(0x11111111)
	defaultSearchEngine.TTStore(oldTTKey, move, 12, 3, Exact, false)
	oldTable := defaultSearchEngine
	oldSize := oldTable.HashSize()
	if err := ApplyTexelModel(changed); err != nil {
		t.Fatal(err)
	}
	if defaultSearchEngine != oldTable || defaultSearchEngine.HashSize() != oldSize {
		t.Fatal("Apply replaced or resized the configured TT")
	}
	if _, _, _, _, hit, _ := defaultSearchEngine.TTProbe(oldTTKey); hit {
		t.Fatal("first receiver access did not clear stale-generation TT state")
	}

	const newTTKey = uint64(0x22222222)
	defaultSearchEngine.TTStore(newTTKey, move, 13, 3, Exact, false)
	searcher.SearchFixed(mustHCEPosition(t, "7k/6Q1/6K1/8/8/8/8/8 b - - 0 1"), 1, nil)
	if _, _, _, _, hit, _ := defaultSearchEngine.TTProbe(newTTKey); !hit {
		t.Fatal("per-engine generation mismatch cleared the already-published TT")
	}
	if searcher.worker.history.historyTable[WhiteKing][A2] != 0 ||
		searcher.worker.history.continuationHistory[WhiteKing][A1][WhiteKing][A2] != 0 ||
		searcher.worker.history.followupHistory[WhiteKing][A1][WhiteKing][A2] != 0 ||
		searcher.worker.history.pawnCorrectionHistory[White][1] != 0 ||
		searcher.worker.history.nonPawnCorrectionHistory[Black][2] != 0 ||
		searcher.worker.history.minorCorrectionHistory[White][3] != 0 ||
		searcher.worker.history.captureHistory[WhiteKing][A2][BlackPawn] != 0 ||
		searcher.worker.history.killerMoves[0][0] != EmptyMove ||
		searcher.worker.history.counterMoves[A1][A2] != EmptyMove ||
		searcher.worker.history.lastMovePlayed != EmptyMove {
		t.Fatal("generation mismatch did not clear the complete worker-history family")
	}
	if searcher.worker.hce.full[0] != (evalCacheEntry{}) || searcher.worker.hce.pawns[0] != (pawnCacheEntry{}) {
		t.Fatal("generation mismatch did not clear private evaluator caches")
	}
}

func TestSameGenerationPreservesWarmPrivateState(t *testing.T) {
	searcher := NewSearchEngine()
	terminal := mustHCEPosition(t, "7k/6Q1/6K1/8/8/8/8/8 b - - 0 1")
	searcher.SearchFixed(terminal, 1, nil)
	move := NewMove(A1, A2, WhiteKing, NoPiece, NoType, 0)
	searcher.worker.history.lastMovePlayed = move
	searcher.worker.history.pawnCorrectionHistory[White][7] = 99
	searcher.worker.hce.full[0] = evalCacheEntry{key: 1, val: 2}
	searcher.worker.hce.pawns[0] = pawnCacheEntry{valid: true, wp: 1}

	searcher.SearchFixed(terminal.Copy(), 1, nil)
	if searcher.worker.history.lastMovePlayed != move || searcher.worker.history.pawnCorrectionHistory[White][7] != 99 {
		t.Fatal("same-generation search discarded warm worker history")
	}
	if searcher.worker.hce.full[0] != (evalCacheEntry{key: 1, val: 2}) ||
		searcher.worker.hce.pawns[0] != (pawnCacheEntry{valid: true, wp: 1}) {
		t.Fatal("same-generation search discarded warm evaluator caches")
	}
}

func TestTexelTunePanicRollsBackCoherentModel(t *testing.T) {
	saved := ExportTexelModel()
	savedTT := defaultSearchEngine
	defaultSearchEngine, _ = NewSearchEngineWithHash(1)
	t.Cleanup(func() { restoreHCEModelAndTT(t, saved, savedTT) })

	const fen = "7k/8/8/8/3Q4/8/8/K7 w - - 0 1"
	oldDirect := mustHCEPosition(t, fen)
	oldSearch := mustHCEPosition(t, "7k/6Q1/6K1/8/8/8/8/8 b - - 0 1")
	beforeDirect := Evaluate(&oldDirect.Board)
	sentinel := errors.New("injected rebuild panic")
	var recovered any
	func() {
		defer func() { recovered = recover() }()
		_, _ = TryTexelTune(nil, []*int{&pestoMGMaterial[Queen]}, 1, 1, 1, func() {
			panic(sentinel)
		}, nil)
	}()
	panicErr, ok := recovered.(error)
	if !ok || !errors.Is(panicErr, sentinel) {
		t.Fatalf("panic = %#v, want injected rebuild panic", recovered)
	}
	if got := ExportTexelModel(); !reflect.DeepEqual(got, saved) {
		t.Fatal("tuner panic did not roll back the complete HCE model")
	}
	freshDirect := mustHCEPosition(t, fen)
	if gotOld, gotFresh := Evaluate(&oldDirect.Board), Evaluate(&freshDirect.Board); gotOld != beforeDirect || gotOld != gotFresh {
		t.Fatalf("post-rollback direct eval mismatch: before=%d old=%d fresh=%d", beforeDirect, gotOld, gotFresh)
	}

	searcher, _ := NewSearchEngineWithHash(1)
	freshSearch := mustHCEPosition(t, "7k/6Q1/6K1/8/8/8/8/8 b - - 0 1")
	searcher.SearchFixed(oldSearch, 1, nil)
	searcher.SearchFixed(freshSearch, 1, nil)
	if oldSearch.Board != freshSearch.Board {
		t.Fatal("post-rollback search admitted incoherent old-position accumulators")
	}
}
