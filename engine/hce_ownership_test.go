package engine

import (
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
			rawWhite := evaluateWhiteLeased(&pos.Board)
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

func TestSameGenerationPreservesWarmPrivateState(t *testing.T) {
	searcher := NewSearchEngine()
	terminal := mustHCEPosition(t, "7k/6Q1/6K1/8/8/8/8/8 b - - 0 1")
	searcher.SearchFixed(terminal, 1, nil)
	move := NewMove(A1, A2, WhiteKing, NoPiece, NoType, 0)
	searcher.worker.history.lastMovePlayed = move
	searcher.worker.history.pawnCorrectionHistory[White][7] = 99
	searcher.worker.hce.full[0] = evalCacheEntry{key: 1, val: 2}

	searcher.SearchFixed(terminal.Copy(), 1, nil)
	if searcher.worker.history.lastMovePlayed != move || searcher.worker.history.pawnCorrectionHistory[White][7] != 99 {
		t.Fatal("same-generation search discarded warm worker history")
	}
	if searcher.worker.hce.full[0] != (evalCacheEntry{key: 1, val: 2}) {
		t.Fatal("same-generation search discarded warm evaluator caches")
	}
}
