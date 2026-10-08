package engine

import (
	"testing"
	"time"
)

func newSearchInfoForTest(tm *TimeManager, rootDepth int) *SearchInfo {
	var moveBuffer [256]Move
	var captureBuffer [64]Move
	var orderedBuffer [256]Move
	var seeGains [32]int
	return &SearchInfo{
		control:       &defaultSearchEngine.worker.control,
		history:       &defaultSearchEngine.worker.history,
		evaluator:     testHCEWorkerEvaluator(nil),
		tt:            defaultTTForDirectTest(),
		Depth:         rootDepth,
		RootDepth:     rootDepth,
		BestMove:      EmptyMove,
		BestScore:     -INFINITY,
		TimeManager:   tm,
		MoveBuffer:    &moveBuffer,
		CaptureBuffer: &captureBuffer,
		OrderedBuffer: &orderedBuffer,
		SEEGains:      &seeGains,
		Frames:        make([]searchFrame, searchFramePoolSize),
	}
}

func TestStoppedChildDoesNotStoreParentTT(t *testing.T) {
	ClearStop()
	ClearHistoryTable()
	ClearKillerMoves()
	ClearCounterMoves()
	defaultSearchEngine, _ = NewSearchEngineWithHash(1)

	pos, err := ParseFEN("rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	rootHash := pos.Hash()

	tm := NewTimeManager()
	tm.timeControl = TimePerMove
	tm.allocatedTime = time.Nanosecond
	tm.startTime = time.Now().Add(-time.Second)
	tm.checkCounter = 1021 // parent checks miss; first child check trips the clock.

	info := newSearchInfoForTest(tm, 1)
	score := alphaBetaPV(pos, 1, 0, -INFINITY, INFINITY, true, true, false, info)
	if !info.Stopped {
		t.Fatal("search did not stop in the child as expected")
	}
	if score != stoppedSearchScore {
		t.Fatalf("stopped search returned %d, want stoppedSearchScore", score)
	}
	if _, _, _, _, hit, _ := info.tt.Get(rootHash); hit {
		t.Fatal("stopped parent wrote a transposition-table entry")
	}
}

// Every clock-check site may be the one that trips, including the parent's own
// move-loop check after earlier moves were searched; none may store the parent.
func TestStoppedSearchNeverStoresRootTTAtAnyCheckSite(t *testing.T) {
	pos, err := ParseFEN("r1bqkbnr/pppp1ppp/2n5/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R w KQkq - 2 3")
	if err != nil {
		t.Fatal(err)
	}
	rootHash := pos.Hash()
	stops := 0
	for offset := 1; offset <= 1024; offset++ {
		ClearStop()
		ClearHistoryTable()
		ClearKillerMoves()
		ClearCounterMoves()
		defaultSearchEngine, _ = NewSearchEngineWithHash(1)
		tm := NewTimeManager()
		tm.timeControl = TimePerMove
		tm.allocatedTime = time.Nanosecond
		tm.startTime = time.Now().Add(-time.Second)
		tm.checkCounter = uint64(1024 - offset)
		info := newSearchInfoForTest(tm, 2)
		alphaBetaPV(pos, 2, 0, -INFINITY, INFINITY, true, true, false, info)
		if !info.Stopped {
			break
		}
		stops++
		if _, _, _, _, hit, _ := info.tt.Get(rootHash); hit {
			t.Fatalf("search stopped at check %d stored the root transposition-table entry", offset)
		}
	}
	ClearStop()
	if stops < 30 {
		t.Fatalf("only %d check sites exercised", stops)
	}
}

func TestQuiescenceHonorsTimeLimit(t *testing.T) {
	ClearStop()
	t.Cleanup(ClearStop)
	defaultSearchEngine, _ = NewSearchEngineWithHash(1)
	pos, err := ParseFEN("4k3/8/8/3q4/4P3/8/8/4K3 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	hash := pos.Hash()
	tm := NewTimeManager()
	tm.timeControl = TimePerMove
	tm.allocatedTime = time.Nanosecond
	tm.startTime = time.Now().Add(-time.Second)
	tm.checkCounter = 1023 // The next time check must observe the expired clock.
	info := newSearchInfoForTest(tm, 1)
	score := quiescence(pos, -INFINITY, INFINITY, 1, info)
	if !info.Stopped || score != stoppedSearchScore {
		t.Fatalf("expired qsearch returned score=%d stopped=%v", score, info.Stopped)
	}
	if info.Nodes != 0 || pos.Hash() != hash {
		t.Fatal("expired qsearch searched moves or changed the position")
	}
}

// Long-clock and infinite searches have no hidden node ceiling. Explicit node,
// time, and external-stop limits remain responsible for cancellation.
func TestAlphaBetaHasNoImplicitFiftyMillionNodeCeiling(t *testing.T) {
	pos, err := ParseFEN("7k/8/8/8/3Q4/8/8/K7 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	ClearStop()
	t.Cleanup(ClearStop)

	info := newSearchInfoForTest(nil, MaximumDepth+1)
	info.Nodes = 50_000_001
	want := info.evaluator.LegacyUndampedSTM(pos)
	got := alphaBetaPV(pos, MaximumDepth+1, 0, -INFINITY, INFINITY, true, true, false, info)
	if info.Stopped || got != want || info.Nodes != 50_000_001 {
		t.Fatalf("implicit ceiling survived: score=%d want=%d stopped=%v nodes=%d", got, want, info.Stopped, info.Nodes)
	}

	limited := newSearchInfoForTest(nil, MaximumDepth+1)
	limited.Nodes = 50_000_001
	limited.maxNodes = limited.Nodes
	if got := alphaBetaPV(pos, MaximumDepth+1, 0, -INFINITY, INFINITY, true, true, false, limited); got != stoppedSearchScore || !limited.Stopped {
		t.Fatalf("explicit node limit lost: score=%d stopped=%v", got, limited.Stopped)
	}
}
