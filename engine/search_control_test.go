package engine

import (
	"sync"
	"testing"
)

func controlTestPosition(t *testing.T) *Position {
	t.Helper()
	pos, err := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	return pos
}

// This proves that real recursive searches read their receiver's control rather
// than the package compatibility control.
func TestSearchEngineControlsAreIsolatedInRealSearches(t *testing.T) {
	stoppedEngine, _ := NewSearchEngineWithHash(1)
	stopped := stoppedEngine.SearchIterativeDeepeningWithCallback(controlTestPosition(t), 4, nil, func(info *SearchInfo) {
		if info.Depth == 1 {
			stoppedEngine.RequestStop()
		}
	})
	if !stopped.Stopped || stopped.Depth != 1 {
		t.Fatalf("receiver stop was not observed: stopped=%v depth=%d", stopped.Stopped, stopped.Depth)
	}

	unlimitedEngine, _ := NewSearchEngineWithHash(1)
	completed := unlimitedEngine.Search(controlTestPosition(t), 2)
	if completed.Stopped || completed.Depth != 2 {
		t.Fatalf("another receiver inherited stop: stopped=%v depth=%d", completed.Stopped, completed.Depth)
	}

	limitedEngine, _ := NewSearchEngineWithHash(1)
	limitedEngine.SetMaxNodes(64)
	limited := limitedEngine.Search(controlTestPosition(t), MaximumDepth)
	if !limited.Stopped || limited.Nodes < 64 {
		t.Fatalf("receiver node limit was not observed: stopped=%v nodes=%d", limited.Stopped, limited.Nodes)
	}
	if unlimitedEngine.MaxNodes() != 0 || unlimitedEngine.StopRequested() {
		t.Fatal("node-limited receiver changed another receiver's control")
	}
}

func TestSearchEngineHoldsOneSessionLockThroughCallback(t *testing.T) {
	searcher, _ := NewSearchEngineWithHash(1)
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	var once sync.Once
	pos := controlTestPosition(t)
	go func() {
		searcher.SearchIterativeDeepeningWithCallback(pos, 1, nil, func(*SearchInfo) {
			once.Do(func() { close(entered) })
			<-release
		})
		close(done)
	}()
	<-entered
	if searcher.sessionMu.TryLock() {
		searcher.sessionMu.Unlock()
		close(release)
		<-done
		t.Fatal("session lock was not held through the callback")
	}
	close(release)
	<-done
}

func TestUCIInstancesOwnDistinctSearchControls(t *testing.T) {
	a := NewUCIEngine()
	b := NewUCIEngine()
	a.searcher.SetMaxNodes(17)
	a.searcher.RequestStop()
	if b.searcher.MaxNodes() != 0 || b.searcher.StopRequested() {
		t.Fatal("UCI instances share stop/node control")
	}
}
