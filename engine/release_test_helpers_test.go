package engine

import (
	"testing"
	"time"
)

func adapterLegalMove(t *testing.T, pos *Position, notation string) Move {
	t.Helper()
	for _, move := range GenerateLegalMoves(pos) {
		if move.ToString() == notation {
			return move
		}
	}
	t.Fatalf("legal move %s not found", notation)
	return EmptyMove
}

func n3cPosition(t *testing.T, fen string) *Position {
	t.Helper()
	pos, err := ParseFEN(fen)
	if err != nil {
		t.Fatal(err)
	}
	return pos
}

func newN3CDirectSearchInfo(searcher *SearchEngine, evaluator *workerEvaluator, tt *Cache, depth int) *SearchInfo {
	var moveBuffer [256]Move
	var captureBuffer [64]Move
	var orderedBuffer [256]Move
	var seeGains [32]int
	return &SearchInfo{
		Depth:         depth,
		RootDepth:     depth,
		BestMove:      EmptyMove,
		BestScore:     -INFINITY,
		MoveBuffer:    &moveBuffer,
		CaptureBuffer: &captureBuffer,
		OrderedBuffer: &orderedBuffer,
		SEEGains:      &seeGains,
		Frames:        make([]searchFrame, searchFramePoolSize),
		control:       &searcher.worker.control,
		maxNodes:      searcher.worker.control.MaxNodes(),
		history:       &searcher.worker.history,
		evaluator:     evaluator,
		tt:            tt,
	}
}

func waitUCIRunning(t *testing.T, uci *UCIEngine) *uciSearchSession {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		uci.lifecycleMu.Lock()
		session, state := uci.activeSearch, uci.searchState
		uci.lifecycleMu.Unlock()
		if session != nil && state == uciSearchRunning {
			return session
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for running UCI search")
	return nil
}
