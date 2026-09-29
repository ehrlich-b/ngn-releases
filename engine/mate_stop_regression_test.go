package engine

import "testing"

func TestIterativeDeepeningDoesNotStopOnShallowCachedMateWithHistory(t *testing.T) {
	// replayGame46 supplies the real played-position counts. In particular,
	// e5e8 g4g5 reaches a position already present twice in that history.
	pos := replayGame46(t)
	poisoned, err := ParseUCIMove(pos, "e5e8")
	if err != nil {
		t.Fatal(err)
	}
	witness := replayGame46(t)
	for _, text := range []string{"e5e8", "g4g5"} {
		move, err := ParseUCIMove(witness, text)
		if err != nil {
			t.Fatalf("draw witness %s: %v", text, err)
		}
		if _, _, _, ok := witness.GameMakeMove(move); !ok {
			t.Fatalf("draw witness %s is illegal", text)
		}
	}
	if !witness.IsFIDEDrawRule() {
		t.Fatal("legal e5e8 g4g5 continuation did not reach the recorded threefold")
	}

	oldTT := defaultSearchEngine
	oldMaxNodes := MaxNodes()
	oldStop := IsStopRequested()
	oldHistory := defaultSearchEngine.worker.history.historyTable[poisoned.MovingPiece()][poisoned.Destination()]
	defer func() {
		defaultSearchEngine = oldTT
		SetMaxNodes(oldMaxNodes)
		defaultSearchEngine.worker.history.historyTable[poisoned.MovingPiece()][poisoned.Destination()] = oldHistory
		if oldStop {
			RequestStop()
		} else {
			ClearStop()
		}
	}()
	defaultSearchEngine, _ = NewSearchEngineWithHash(DEFAULT_CACHE_SIZE)
	ClearStop()
	SetMaxNodes(0)

	// Root ordering does not probe the TT. Give the poisoned quiet move a test-only
	// history score above every ordinary ordering band so it is searched first.
	defaultSearchEngine.worker.history.historyTable[poisoned.MovingPiece()][poisoned.Destination()] = 100000

	ep, tag, hc, ok := pos.MakeMove(poisoned)
	if !ok {
		t.Fatal("e5e8 is illegal")
	}
	childHash := pos.Hash()
	pos.UnMakeMove(poisoned, tag, ep, hc)

	// Synthetically reproduce the archived root mate-15 score (29 plies).
	// scoreToTT normalizes the child score for storage at root ply one, and
	// scoreFromTT restores -MATE_VALUE+29 when qsearch probes this child.
	defaultSearchEngine.TTStore(childHash, EmptyMove,
		scoreToTT(-MATE_VALUE+29, 1), 0, Exact, true)

	type completed struct {
		depth int
		move  Move
		score int
	}
	var iterations []completed
	result := SearchIterativeDeepeningWithCallback(pos, 10, nil, func(info *SearchInfo) {
		iterations = append(iterations, completed{info.Depth, info.BestMove, info.BestScore})
	})

	if len(iterations) == 0 {
		t.Fatal("no completed search iteration")
	}
	first := iterations[0]
	if first.depth != 1 || first.move != poisoned || first.score != MATE_VALUE-29 {
		t.Fatalf("depth-1 cache setup failed: depth=%d move=%s score=%d",
			first.depth, first.move.ToString(), first.score)
	}
	if len(iterations) < 2 || result.Depth < 2 {
		t.Fatalf("shallow cached mate stopped iterative deepening: callbacks=%d finalDepth=%d",
			len(iterations), result.Depth)
	}
	if result.BestMove == poisoned {
		t.Fatalf("deeper history-aware search retained poisoned move %s at depth %d score %d",
			result.BestMove.ToString(), result.Depth, result.BestScore)
	}
}
