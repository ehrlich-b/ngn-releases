package engine

import (
	"testing"
	"time"
)

type terminationObserverClock struct {
	now time.Time
}

func (clock *terminationObserverClock) Now() time.Time { return clock.now }
func (clock *terminationObserverClock) Advance(delta time.Duration) {
	clock.now = clock.now.Add(delta)
}

func configuredObserverTournament(clock *terminationObserverClock, observer *searchTerminationObserver) *TimeManager {
	tm := newTimeManager(clock.Now)
	tm.timeControl = Tournament
	tm.startTime = clock.now
	tm.baseTime = 10 * time.Second
	tm.softTime = time.Second
	tm.hardTime = 4 * time.Second
	tm.emergencyTime = 100 * time.Millisecond
	tm.lastIterationTime = 200 * time.Millisecond
	tm.stableIters = 3
	tm.bestMoveNodeFraction = 0
	tm.checkCounter = 1023
	tm.setSearchTerminationObserver(observer)
	return tm
}

func TestTerminationObserverClassifiesTimeCausesAndLatchesFirst(t *testing.T) {
	start := time.Unix(123, 0)
	t.Run("soft admission", func(t *testing.T) {
		clock := &terminationObserverClock{now: start.Add(801 * time.Millisecond)}
		var observer searchTerminationObserver
		tm := configuredObserverTournament(clock, &observer)
		tm.startTime = start
		if !tm.shouldStopSearchAt(7, timeCheckIterationAdmission) {
			t.Fatal("soft admission did not stop")
		}
		if observer.FirstReason != searchTerminationSoftAdmission || observer.FirstDepth != 7 || observer.FirstElapsed != 801*time.Millisecond {
			t.Fatalf("soft admission observation = %+v", observer)
		}

		clock.now = start.Add(4 * time.Second)
		if !tm.ShouldStopSearch(2) {
			t.Fatal("cached stop was lost")
		}
		if observer.FirstReason != searchTerminationSoftAdmission || observer.FirstElapsed != 801*time.Millisecond {
			t.Fatalf("soft stop was relabeled during unwind: %+v", observer)
		}
	})

	t.Run("soft active abort", func(t *testing.T) {
		clock := &terminationObserverClock{now: start.Add(801 * time.Millisecond)}
		var observer searchTerminationObserver
		tm := configuredObserverTournament(clock, &observer)
		tm.startTime = start
		if !tm.ShouldStopSearch(7) || observer.FirstReason != searchTerminationSoftAbort {
			t.Fatalf("active soft observation = %+v", observer)
		}
	})

	t.Run("hard precedes emergency", func(t *testing.T) {
		clock := &terminationObserverClock{now: start.Add(4 * time.Second)}
		var observer searchTerminationObserver
		tm := configuredObserverTournament(clock, &observer)
		tm.startTime = start
		tm.baseTime = 4050 * time.Millisecond
		if !tm.ShouldStopSearch(7) || observer.FirstReason != searchTerminationHardDeadline {
			t.Fatalf("coincident hard/emergency observation = %+v", observer)
		}
	})

	t.Run("emergency", func(t *testing.T) {
		clock := &terminationObserverClock{now: start.Add(901 * time.Millisecond)}
		var observer searchTerminationObserver
		tm := configuredObserverTournament(clock, &observer)
		tm.startTime = start
		tm.baseTime = time.Second
		tm.hardTime = 2 * time.Second
		tm.softTime = 2 * time.Second
		if !tm.ShouldStopSearch(0) || observer.FirstReason != searchTerminationEmergencyReserve {
			t.Fatalf("emergency observation = %+v", observer)
		}
	})

	t.Run("fixed deadline", func(t *testing.T) {
		clock := &terminationObserverClock{now: start.Add(time.Second)}
		var observer searchTerminationObserver
		tm := newTimeManager(clock.Now)
		tm.timeControl = TimePerMove
		tm.startTime = start
		tm.allocatedTime = time.Second
		tm.checkCounter = 1023
		tm.setSearchTerminationObserver(&observer)
		if !tm.ShouldStopSearch(7) || observer.FirstReason != searchTerminationHardDeadline {
			t.Fatalf("fixed deadline observation = %+v", observer)
		}
	})

	t.Run("setup deadline", func(t *testing.T) {
		clock := &terminationObserverClock{now: start.Add(4 * time.Second)}
		var observer searchTerminationObserver
		tm := configuredObserverTournament(clock, &observer)
		tm.startTime = start
		if !tm.SetupDeadlineExceeded() || observer.FirstReason != searchTerminationHardDeadline || observer.FirstDepth != 0 {
			t.Fatalf("setup deadline observation = %+v", observer)
		}
	})
}

func TestTerminationObserverDistinguishesNodeAndExternalStops(t *testing.T) {
	newInfo := func(tm *TimeManager, control *SearchControl) *SearchInfo {
		return &SearchInfo{TimeManager: tm, control: control, maxNodes: 10, Nodes: 10, RootDepth: 6}
	}

	t.Run("node cap stays node cap", func(t *testing.T) {
		clock := &terminationObserverClock{now: time.Unix(456, 0)}
		var observer searchTerminationObserver
		tm := newTimeManager(clock.Now)
		tm.startTime = clock.now
		tm.setSearchTerminationObserver(&observer)
		control := &SearchControl{}
		info := newInfo(tm, control)
		if !info.shouldStopForControl() || observer.FirstReason != searchTerminationNodeCap {
			t.Fatalf("node observation = %+v", observer)
		}
		if !info.shouldStopForControl() || observer.FirstReason != searchTerminationNodeCap {
			t.Fatalf("node cap was relabeled external: %+v", observer)
		}
	})

	t.Run("preexisting external request wins", func(t *testing.T) {
		clock := &terminationObserverClock{now: time.Unix(789, 0)}
		var observer searchTerminationObserver
		tm := newTimeManager(clock.Now)
		tm.startTime = clock.now
		tm.setSearchTerminationObserver(&observer)
		control := &SearchControl{}
		control.RequestStop()
		if !newInfo(tm, control).shouldStopForControl() || observer.FirstReason != searchTerminationExternalStop {
			t.Fatalf("external observation = %+v", observer)
		}
	})
}

func TestTerminationObserverRecordsWholeIterationAndReset(t *testing.T) {
	start := time.Unix(321, 0)
	clock := &terminationObserverClock{now: start}
	var observer searchTerminationObserver
	tm := newTimeManager(clock.Now)
	tm.setSearchTerminationObserver(&observer)
	tm.SetTimeControlAt(SearchParams{WhiteTime: 10_000, WhiteInc: 100}, true, start)
	tm.NewIteration()
	clock.Advance(30 * time.Millisecond)
	clock.Advance(40 * time.Millisecond)
	clock.Advance(50 * time.Millisecond)
	tm.observeCompletedIteration(4, 3)

	if observer.IterationCount != 1 {
		t.Fatalf("iteration count=%d, want 1", observer.IterationCount)
	}
	record := observer.Iterations[0]
	if record.Depth != 4 || record.Duration != 120*time.Millisecond || record.CompletedAtElapsed != 120*time.Millisecond || record.AspirationAttempts != 3 {
		t.Fatalf("iteration observation = %+v", record)
	}

	tm.checkCounter = 99
	tm.shouldStop = true
	tm.SetTimeControlAt(SearchParams{WhiteTime: 10_000, WhiteInc: 100}, true, clock.now)
	if observer != (searchTerminationObserver{}) {
		t.Fatalf("time-control reset retained observer data: %+v", observer)
	}
	if tm.checkCounter != 0 || tm.shouldStop {
		t.Fatalf("time-control reset retained stop state: counter=%d stop=%v", tm.checkCounter, tm.shouldStop)
	}
}

func TestTerminationObserverNaturalCompletionCategories(t *testing.T) {
	for _, test := range []struct {
		name   string
		fen    string
		depth  int
		reason searchTerminationReason
	}{
		{"terminal", "7k/6Q1/6K1/8/8/8/8/8 b - - 0 1", 8, searchTerminationTerminal},
		{"depth", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1", 1, searchTerminationDepthComplete},
	} {
		t.Run(test.name, func(t *testing.T) {
			position, err := ParseFEN(test.fen)
			if err != nil {
				t.Fatal(err)
			}
			searcher, err := NewSearchEngineWithHash(1)
			if err != nil {
				t.Fatal(err)
			}
			tm := NewTimeManager()
			tm.SetTimeControl(SearchParams{Depth: test.depth}, position.Turn() == White)
			var observer searchTerminationObserver
			tm.setSearchTerminationObserver(&observer)
			result := searcher.SearchIterativeDeepening(position, test.depth, tm)
			if result.Stopped || observer.FirstReason != test.reason {
				t.Fatalf("result stopped=%v observation=%+v", result.Stopped, observer)
			}
		})
	}
}

func TestTerminationObserverPreservesFixedNodeSearch(t *testing.T) {
	const nodes = 50_000
	run := func(withObserver bool) (*SearchInfo, searchTerminationObserver) {
		t.Helper()
		position := newUCIStartingPosition()
		searcher, err := NewSearchEngineWithHash(1)
		if err != nil {
			t.Fatal(err)
		}
		searcher.SetMaxNodes(nodes)
		tm := NewTimeManager()
		tm.SetTimeControl(SearchParams{Depth: MaximumDepth}, true)
		var observer searchTerminationObserver
		if withObserver {
			tm.setSearchTerminationObserver(&observer)
		}
		return searcher.SearchIterativeDeepening(position, MaximumDepth, tm), observer
	}

	plain, _ := run(false)
	observed, observer := run(true)
	if !plain.Stopped || !observed.Stopped || plain.Nodes != nodes || observed.Nodes != nodes {
		t.Fatalf("fixed-node stops plain=%+v observed=%+v", plain, observed)
	}
	if plain.BestMove != observed.BestMove || plain.BestScore != observed.BestScore || plain.Depth != observed.Depth || plain.SelDepth != observed.SelDepth {
		t.Fatalf("observer changed fixed-node decision: plain move=%v score=%d depth=%d/%d; observed move=%v score=%d depth=%d/%d",
			plain.BestMove, plain.BestScore, plain.Depth, plain.SelDepth,
			observed.BestMove, observed.BestScore, observed.Depth, observed.SelDepth)
	}
	if observer.FirstReason != searchTerminationNodeCap {
		t.Fatalf("fixed-node observation = %+v", observer)
	}
}

func BenchmarkShouldStopSearchObserver(b *testing.B) {
	for _, enabled := range []bool{false, true} {
		name := "off"
		if enabled {
			name = "on"
		}
		b.Run(name, func(b *testing.B) {
			tm := NewTimeManager()
			tm.SetTimeControl(SearchParams{Infinite: true}, true)
			var observer searchTerminationObserver
			if enabled {
				tm.setSearchTerminationObserver(&observer)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tm.ShouldStopSearch(6)
			}
		})
	}
}
