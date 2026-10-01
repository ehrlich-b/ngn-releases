package engine

import (
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func newSMPTestEngine(t *testing.T, workers int) *SearchEngine {
	t.Helper()
	searcher, err := NewSearchEngineWithHash(1)
	if err != nil {
		t.Fatal(err)
	}
	if err := searcher.ConfigureThreads(workers); err != nil {
		t.Fatal(err)
	}
	return searcher
}

func waitSMPIndex(t *testing.T, ch <-chan int, what string) int {
	t.Helper()
	select {
	case index := <-ch:
		return index
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
		return -1
	}
}

func TestLazySMPStartsPrivateWorkersAndMergesExactCounters(t *testing.T) {
	searcher := newSMPTestEngine(t, 3)
	started := make(chan int, 3)
	release := make(chan struct{})
	locals := make([]SearchInfo, 3)
	var localsMu sync.Mutex
	searcher.smpHooks = &smpSearchHooks{
		workerStarted: func(worker int) {
			started <- worker
			<-release
		},
		workerFinished: func(worker int, info *SearchInfo) {
			localsMu.Lock()
			locals[worker] = *info
			localsMu.Unlock()
		},
	}

	pos := controlTestPosition(t)
	before := snapshotPVPosition(pos)
	var callbacks []SearchInfo
	done := make(chan *SearchInfo, 1)
	go func() {
		done <- searcher.SearchIterativeDeepeningWithCallback(pos, 2, nil, func(info *SearchInfo) {
			callbacks = append(callbacks, *info)
		})
	}()
	seen := [3]bool{}
	for i := 0; i < 3; i++ {
		seen[waitSMPIndex(t, started, "all lazy-SMP workers")] = true
	}
	close(release)
	var result *SearchInfo
	select {
	case result = <-done:
	case <-time.After(10 * time.Second):
		searcher.RequestStop()
		t.Fatal("timed out joining lazy-SMP search")
	}
	for worker, didStart := range seen {
		if !didStart {
			t.Fatalf("worker %d did not start", worker)
		}
	}
	if result.EffectiveThreads != 3 || result.BestMove == EmptyMove {
		t.Fatalf("result effective=%d move=%v", result.EffectiveThreads, result.BestMove)
	}
	var want SearchInfo
	for i := range locals {
		addSearchMonotonicCounters(&want, &locals[i])
	}
	if result.Nodes != want.Nodes || result.QNodes != want.QNodes || result.TTProbes != want.TTProbes || result.BetaCutoffs != want.BetaCutoffs {
		t.Fatalf("final counters nodes/q/ttp/beta=%d/%d/%d/%d, want %d/%d/%d/%d",
			result.Nodes, result.QNodes, result.TTProbes, result.BetaCutoffs,
			want.Nodes, want.QNodes, want.TTProbes, want.BetaCutoffs)
	}
	if len(callbacks) == 0 {
		t.Fatal("worker zero emitted no completed-depth callbacks")
	}
	for i, callback := range callbacks {
		if callback.EffectiveThreads != 3 {
			t.Fatalf("callback %d effective=%d", i, callback.EffectiveThreads)
		}
		if i > 0 && callback.Nodes < callbacks[i-1].Nodes {
			t.Fatalf("callback nodes regressed %d -> %d", callbacks[i-1].Nodes, callback.Nodes)
		}
	}
	if result.Nodes < callbacks[len(callbacks)-1].Nodes {
		t.Fatal("final exact nodes are below last live aggregate")
	}
	if after := snapshotPVPosition(pos); !reflect.DeepEqual(after, before) {
		t.Fatal("lazy-SMP search changed caller root")
	}
}

func TestLazySMPAdvancesAgeOnceAndNodesForcesOneWorker(t *testing.T) {
	searcher := newSMPTestEngine(t, 4)
	searcher.TTDiagnostics()
	before := searcher.tt.age
	result := searcher.Search(controlTestPosition(t), 1)
	if result.EffectiveThreads != 4 || searcher.tt.age != before+1 {
		t.Fatalf("iterative effective/age=%d/%d, want 4/%d", result.EffectiveThreads, searcher.tt.age, before+1)
	}
	age := searcher.tt.age
	searcher.SearchFixed(controlTestPosition(t), 1, nil)
	if searcher.tt.age != age {
		t.Fatalf("fixed search changed historical TT age %d -> %d", age, searcher.tt.age)
	}

	var startedMu sync.Mutex
	var started []int
	searcher.smpHooks = &smpSearchHooks{workerStarted: func(worker int) {
		startedMu.Lock()
		started = append(started, worker)
		startedMu.Unlock()
	}}
	searcher.SetMaxNodes(64)
	limited := searcher.Search(controlTestPosition(t), MaximumDepth)
	if limited.EffectiveThreads != 1 || !limited.Stopped {
		t.Fatalf("go-nodes-equivalent result effective=%d stopped=%v", limited.EffectiveThreads, limited.Stopped)
	}
	if len(started) != 0 {
		t.Fatalf("fixed-node policy launched SMP hooks: %v", started)
	}
	if searcher.tt.age != age+1 {
		t.Fatalf("fixed-node iterative age=%d, want %d", searcher.tt.age, age+1)
	}
}

func TestLazySMPSetupCancellationAfterCopyAndReset(t *testing.T) {
	for _, test := range []struct {
		name string
		hook func(*SearchEngine) *smpSearchHooks
	}{
		{"stop-after-copy", func(searcher *SearchEngine) *smpSearchHooks {
			return &smpSearchHooks{afterRootCopy: func(worker int) {
				if worker == 1 {
					searcher.RequestStop()
				}
			}}
		}},
		{"stop-after-reset", func(searcher *SearchEngine) *smpSearchHooks {
			return &smpSearchHooks{afterReset: func(worker int) {
				if worker == 1 {
					searcher.RequestStop()
				}
			}}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			searcher := newSMPTestEngine(t, 3)
			starts := 0
			searcher.smpHooks = test.hook(searcher)
			searcher.smpHooks.workerStarted = func(int) { starts++ }
			searcher.ClearStop()
			result := searcher.searchIterativeDeepeningPrepared(controlTestPosition(t), 4, nil, nil)
			if !result.Stopped || starts != 0 {
				t.Fatalf("stopped=%v worker starts=%d, want true/0", result.Stopped, starts)
			}
		})
	}
}

func TestLazySMPSetupDeadlineAfterCopyAndReset(t *testing.T) {
	for _, phase := range []string{"copy", "reset"} {
		t.Run(phase, func(t *testing.T) {
			searcher := newSMPTestEngine(t, 3)
			clock := &uciFakeClock{now: time.Date(2026, 9, 6, 13, 0, 0, 0, time.UTC)}
			tm := newTimeManager(clock.Now)
			tm.SetTimeControlAt(SearchParams{MoveTime: 500}, true, clock.Now())
			advance := func(worker int) {
				if worker == 1 {
					clock.Advance(time.Second)
				}
			}
			searcher.smpHooks = &smpSearchHooks{}
			if phase == "copy" {
				searcher.smpHooks.afterRootCopy = advance
			} else {
				searcher.smpHooks.afterReset = advance
			}
			starts := 0
			searcher.smpHooks.workerStarted = func(int) { starts++ }
			result := searcher.SearchIterativeDeepening(controlTestPosition(t), 4, tm)
			if !result.Stopped || starts != 0 || !searcher.StopRequested() {
				t.Fatalf("deadline result stopped=%v starts=%d control=%v", result.Stopped, starts, searcher.StopRequested())
			}
		})
	}
}

func TestLazySMPHelperCompletionDoesNotStopPrimary(t *testing.T) {
	searcher := newSMPTestEngine(t, 2)
	helperDone := make(chan struct{})
	var once sync.Once
	primarySawStop := false
	searcher.smpHooks = &smpSearchHooks{
		workerStarted: func(worker int) {
			if worker == 0 {
				<-helperDone
				primarySawStop = searcher.worker.control.StopRequested()
			}
		},
		workerFinished: func(worker int, _ *SearchInfo) {
			if worker == 1 {
				once.Do(func() { close(helperDone) })
			}
		},
	}
	result := searcher.Search(controlTestPosition(t), 1)
	if primarySawStop || result.Stopped {
		t.Fatalf("normal helper completion stopped primary: saw=%v result=%v", primarySawStop, result.Stopped)
	}
}

func TestLazySMPPanicCancelsAndJoins(t *testing.T) {
	t.Run("helper", func(t *testing.T) {
		searcher := newSMPTestEngine(t, 2)
		sentinel := errors.New("helper boom")
		searcher.smpHooks = &smpSearchHooks{workerStarted: func(worker int) {
			if worker == 1 {
				panic(sentinel)
			}
		}}
		var recovered any
		func() {
			defer func() { recovered = recover() }()
			searcher.Search(controlTestPosition(t), 4)
		}()
		panicInfo, ok := recovered.(*searchWorkerPanic)
		if !ok || !errors.Is(panicInfo.value.(error), sentinel) || !strings.Contains(string(panicInfo.stack), "TestLazySMPPanicCancelsAndJoins") {
			t.Fatalf("helper panic=%#v", recovered)
		}
		if !searcher.StopRequested() {
			t.Fatal("helper panic did not cancel primary and pool")
		}
	})

	t.Run("primary-callback", func(t *testing.T) {
		searcher := newSMPTestEngine(t, 2)
		sentinel := errors.New("callback boom")
		helperDone := make(chan struct{})
		var once sync.Once
		searcher.smpHooks = &smpSearchHooks{workerFinished: func(worker int, _ *SearchInfo) {
			if worker == 1 {
				once.Do(func() { close(helperDone) })
			}
		}}
		var recovered any
		func() {
			defer func() { recovered = recover() }()
			searcher.SearchIterativeDeepeningWithCallback(controlTestPosition(t), 4, nil, func(*SearchInfo) {
				panic(sentinel)
			})
		}()
		if !errors.Is(recovered.(error), sentinel) {
			t.Fatalf("primary callback panic=%#v", recovered)
		}
		select {
		case <-helperDone:
		default:
			t.Fatal("primary callback panic returned before helper joined")
		}
	})
}

func TestAddSearchMonotonicCountersRequiresExplicitPolicy(t *testing.T) {
	dst := SearchInfo{}
	src := SearchInfo{}
	dst.maxNodes = 2
	src.maxNodes = 1
	dv, sv := reflect.ValueOf(&dst).Elem(), reflect.ValueOf(&src).Elem()
	for i := 0; i < dv.NumField(); i++ {
		field := dv.Type().Field(i)
		if field.PkgPath != "" {
			continue
		}
		switch {
		case field.Type.Kind() == reflect.Uint64:
			dv.Field(i).SetUint(2)
			sv.Field(i).SetUint(1)
		case field.Name == "CutIdxHist" || field.Name == "BcutBand" || field.Name == "FmcBand" || field.Name == "CutMissByClass":
			fillUint64Tree(dv.Field(i), 2)
			fillUint64Tree(sv.Field(i), 1)
		}
	}
	addSearchMonotonicCounters(&dst, &src)
	if dst.maxNodes != 2 {
		t.Fatal("merge changed maxNodes")
	}
	for i := 0; i < dv.NumField(); i++ {
		field := dv.Type().Field(i)
		if field.PkgPath != "" {
			continue
		}
		switch {
		case field.Type.Kind() == reflect.Uint64:
			if dv.Field(i).Uint() != 3 {
				t.Fatalf("counter %s was not explicitly merged", field.Name)
			}
		case field.Name == "CutIdxHist" || field.Name == "BcutBand" || field.Name == "FmcBand" || field.Name == "CutMissByClass":
			requireUint64Tree(t, field.Name, dv.Field(i), 3)
		}
	}
}

func TestSearchNodePublisherConcurrentSnapshotsStayCoherent(t *testing.T) {
	var publisher searchNodePublisher
	done := make(chan struct{})
	go func() {
		for i := uint64(1); i <= 10000; i++ {
			publisher.publish(i+1, i)
		}
		close(done)
	}()
	lastNodes := uint64(0)
	for {
		nodes, qnodes := publisher.load()
		if qnodes > nodes || (nodes != 0 && nodes-qnodes != 1) {
			t.Fatalf("incoherent published pair nodes/qnodes=%d/%d", nodes, qnodes)
		}
		if nodes < lastNodes {
			t.Fatalf("published nodes regressed %d -> %d", lastNodes, nodes)
		}
		lastNodes = nodes
		select {
		case <-done:
			return
		default:
		}
	}
}

func fillUint64Tree(value reflect.Value, n uint64) {
	if value.Kind() == reflect.Uint64 {
		value.SetUint(n)
		return
	}
	for i := 0; i < value.Len(); i++ {
		fillUint64Tree(value.Index(i), n)
	}
}

func requireUint64Tree(t *testing.T, name string, value reflect.Value, want uint64) {
	t.Helper()
	if value.Kind() == reflect.Uint64 {
		if value.Uint() != want {
			t.Fatalf("counter %s was not explicitly merged", name)
		}
		return
	}
	for i := 0; i < value.Len(); i++ {
		requireUint64Tree(t, name, value.Index(i), want)
	}
}

func TestLazySMPPrelaunchPanicCancelsAndJoinsLaunchedPrefix(t *testing.T) {
	searcher := newSMPTestEngine(t, 3)
	sentinel := errors.New("late prelaunch boom")
	helperStarted := make(chan struct{})
	helperJoined := make(chan struct{})
	searcher.smpHooks = &smpSearchHooks{
		beforeLaunch: func(worker int) {
			if worker == 2 {
				select {
				case <-helperStarted:
				case <-time.After(5 * time.Second):
					panic("helper 1 did not start before worker 2 launch")
				}
				panic(sentinel)
			}
		},
		workerStarted: func(worker int) {
			if worker == 1 {
				close(helperStarted)
			}
		},
		workerFinished: func(worker int, _ *SearchInfo) {
			if worker == 1 {
				close(helperJoined)
			}
		},
	}
	recoveredCh := make(chan any, 1)
	go func() {
		defer func() { recoveredCh <- recover() }()
		searcher.Search(controlTestPosition(t), MaximumDepth)
	}()
	var recovered any
	select {
	case recovered = <-recoveredCh:
	case <-time.After(10 * time.Second):
		searcher.RequestStop()
		t.Fatal("prelaunch panic did not join launched prefix")
	}
	if !errors.Is(recovered.(error), sentinel) {
		t.Fatalf("prelaunch panic=%#v", recovered)
	}
	select {
	case <-helperJoined:
	default:
		t.Fatal("prelaunch panic returned before launched helper joined")
	}
	if !searcher.worker.control.StopRequested() {
		t.Fatal("prelaunch cleanup did not publish authoritative primary stop")
	}
}

func TestLazySMPCancellationAfterFirstLaunchJoinsPrefix(t *testing.T) {
	searcher := newSMPTestEngine(t, 3)
	helperStarted := make(chan struct{})
	helperJoined := make(chan struct{})
	started := make(chan int, 3)
	searcher.smpHooks = &smpSearchHooks{
		beforeLaunch: func(worker int) {
			if worker == 2 {
				<-helperStarted
				searcher.RequestStop()
			}
		},
		workerStarted: func(worker int) {
			started <- worker
			if worker == 1 {
				close(helperStarted)
			}
		},
		workerFinished: func(worker int, _ *SearchInfo) {
			if worker == 1 {
				close(helperJoined)
			}
		},
	}
	resultCh := make(chan *SearchInfo, 1)
	widthCh := make(chan int, 1)
	searcher.ClearStop()
	go func() {
		resultCh <- searcher.searchIterativeDeepeningPreparedWithWidth(controlTestPosition(t), MaximumDepth, nil, nil, func(width int) {
			widthCh <- width
		})
	}()
	var result *SearchInfo
	select {
	case result = <-resultCh:
	case <-time.After(10 * time.Second):
		searcher.RequestStop()
		t.Fatal("partial launch cancellation did not join")
	}
	if !result.Stopped || result.EffectiveThreads != 2 {
		t.Fatalf("partial launch stopped/effective=%v/%d, want true/2", result.Stopped, result.EffectiveThreads)
	}
	if width := waitSMPIndex(t, widthCh, "partial launch width"); width != 2 {
		t.Fatalf("partial launch width callback=%d, want 2", width)
	}
	close(started)
	seen := map[int]bool{}
	for worker := range started {
		seen[worker] = true
	}
	if !seen[0] || !seen[1] || seen[2] {
		t.Fatalf("started workers=%v, want primary and helper1 only", seen)
	}
	select {
	case <-helperJoined:
	default:
		t.Fatal("partial cancellation returned before helper joined")
	}
}
