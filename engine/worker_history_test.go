package engine

import (
	"io"
	"testing"
)

type workerSearchFingerprint struct {
	bestMove  Move
	bestScore int
	nodes     uint64
	pv        [MaximumDepth]Move
	pvLength  int
}

func runWorkerSearchWithFreshTT(t *testing.T, searcher *SearchEngine, fen string, depth int) workerSearchFingerprint {
	t.Helper()
	if err := searcher.ResizeHash(1); err != nil {
		t.Fatal(err)
	}
	pos, err := ParseFEN(fen)
	if err != nil {
		t.Fatal(err)
	}
	info := searcher.Search(pos, depth)
	return workerSearchFingerprint{
		bestMove:  info.BestMove,
		bestScore: info.BestScore,
		nodes:     info.Nodes,
		pv:        info.PV,
		pvLength:  info.PVLength,
	}
}

func fillWorkerCorrections(h *workerHistory, value int) {
	for color := range h.pawnCorrectionHistory {
		for i := range h.pawnCorrectionHistory[color] {
			h.pawnCorrectionHistory[color][i] = value
			h.nonPawnCorrectionHistory[color][i] = value
			h.minorCorrectionHistory[color][i] = value
		}
	}
}

func TestSearchEngineWorkerHistoryIsolation(t *testing.T) {
	poisoned := NewSearchEngine()
	cleanAfterPoison := NewSearchEngine()
	cleanReference := NewSearchEngine()
	fillWorkerCorrections(&poisoned.worker.history, corrHistLimit)

	const fen = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
	poisonedResult := runWorkerSearchWithFreshTT(t, poisoned, fen, 3)
	afterResult := runWorkerSearchWithFreshTT(t, cleanAfterPoison, fen, 3)
	referenceResult := runWorkerSearchWithFreshTT(t, cleanReference, fen, 3)

	if afterResult != referenceResult {
		t.Fatalf("clean receiver changed after another receiver searched: after=%+v reference=%+v", afterResult, referenceResult)
	}
	if poisonedResult == referenceResult {
		t.Fatalf("correction poison did not produce an observable search witness: %+v", poisonedResult)
	}
}

func TestUCIWorkerHistoryLifecycleIsolation(t *testing.T) {
	first := NewUCIEngine()
	second := NewUCIEngine()
	first.handlePosition([]string{"startpos", "moves", "e2e4"}, io.Discard)
	second.handlePosition([]string{"startpos", "moves", "d2d4"}, io.Discard)

	firstPrevious := first.searcher.GetLastMovePlayed()
	secondPrevious := second.searcher.GetLastMovePlayed()
	if firstPrevious.ToString() != "e2e4" || secondPrevious.ToString() != "d2d4" {
		t.Fatalf("replayed predecessors: first=%s second=%s", firstPrevious.ToString(), secondPrevious.ToString())
	}

	// Put both receivers on the same played root, then make one receiver's counter
	// state observably change its move ordering. The other receiver must retain the
	// unmodified order; checking only table fields would not prove search behavior.
	second.handlePosition([]string{"startpos", "moves", "e2e4"}, io.Discard)
	counter, err := ParseAlgebraicMove("c7c5", first.position)
	if err != nil {
		t.Fatal(err)
	}
	first.searcher.worker.history.UpdateCounterMove(counter, firstPrevious)
	firstOrdered := first.searcher.worker.history.orderMovesWithDepth(GenerateMoves(first.position), 0, first.position)
	secondOrdered := second.searcher.worker.history.orderMovesWithDepth(GenerateMoves(second.position), 0, second.position)
	if len(firstOrdered) == 0 || firstOrdered[0] != counter {
		t.Fatalf("first receiver did not prioritize its counter %s: %v", counter.ToString(), firstOrdered)
	}
	if len(secondOrdered) == 0 || secondOrdered[0] == counter {
		t.Fatalf("counter ordering leaked to second UCI receiver: %v", secondOrdered)
	}

	secondCurrent := second.searcher.GetLastMovePlayed()
	first.handleNewGame(io.Discard)
	if got := first.searcher.GetLastMovePlayed(); got != EmptyMove {
		t.Fatalf("ucinewgame predecessor = %s, want empty", got.ToString())
	}
	if got := second.searcher.GetLastMovePlayed(); got != secondCurrent {
		t.Fatalf("first ucinewgame cleared second predecessor: got %s want %s", got.ToString(), secondCurrent.ToString())
	}
}
