package engine

import "testing"

func h0TestQuiet(from, to Square, piece Piece) Move {
	return NewMove(from, to, piece, NoPiece, NoType, 0)
}

func TestH0HistoryShadowResponse(t *testing.T) {
	if got := h0HistoryRate(30); got != 400 {
		t.Fatalf("capped rate=%d want=400", got)
	}
	if got := h0HistoryResponse(0, historyMax, 400); got != 6400 {
		t.Fatalf("first positive response=%d want=6400", got)
	}
	if got := h0HistoryResponse(6400, historyMax, 400); got != 7800 {
		t.Fatalf("second positive response=%d want=7800", got)
	}
	if got := h0HistoryResponse(1, -historyMax, 1); got != -15 {
		t.Fatalf("negative truncation response=%d want=-15", got)
	}

	entry := 0
	for i := 0; i < 1000; i++ {
		entry = h0HistoryResponse(entry, historyMax, 400)
		if entry > historyMax {
			t.Fatalf("positive response escaped bound: %d", entry)
		}
	}
	for i := 0; i < 1000; i++ {
		entry = h0HistoryResponse(entry, -historyMax, 400)
		if entry < -historyMax {
			t.Fatalf("negative response escaped bound: %d", entry)
		}
	}
}

func TestH0HistoryShadowCompletedNodeUsesWinnerPrefix(t *testing.T) {
	q1 := h0TestQuiet(E2, E4, WhitePawn)
	q2 := h0TestQuiet(D2, D4, WhitePawn)
	q3 := h0TestQuiet(G1, F3, WhiteKnight)
	prev := h0TestQuiet(E7, E5, BlackPawn)
	prev2 := h0TestQuiet(G1, F3, WhiteKnight)

	var live workerHistory
	observer := new(h0HistoryShadowObserver)
	var node h0HistoryShadowNode
	observer.captureNode(&node, []Move{q1, q2, q3}, &live, prev, prev2)
	observer.observeCompletedNode(&node, []Move{q1, q2, q3}, q2, 10, 0, 20, 100, 3, true, false, false)

	const want = 1600 // 8192 * depth^2 / 512
	if got := observer.main[q1.MovingPiece()][q1.Destination()]; got != -want {
		t.Fatalf("prefix loser main=%d want=%d", got, -want)
	}
	if got := observer.main[q2.MovingPiece()][q2.Destination()]; got != want {
		t.Fatalf("winner main=%d want=%d", got, want)
	}
	if got := observer.main[q3.MovingPiece()][q3.Destination()]; got != 0 {
		t.Fatalf("post-winner move changed: %d", got)
	}
	if got := observer.continuation[prev.MovingPiece()][prev.Destination()][q2.MovingPiece()][q2.Destination()]; got != want {
		t.Fatalf("winner continuation=%d want=%d", got, want)
	}
	if got := observer.followup[prev2.MovingPiece()][prev2.Destination()][q2.MovingPiece()][q2.Destination()]; got != want {
		t.Fatalf("winner followup=%d want=%d", got, want)
	}
	if got := live.GetHistoryScore(q2, prev, prev2); got != 0 {
		t.Fatalf("passive observer mutated live history: %d", got)
	}
	c := observer.counters
	if c.QuietWinnerLabels != 1 || c.QuietExactLabels != 1 || c.MissedPVQuietLabels != 1 || c.NewOnlyLabels != 1 {
		t.Fatalf("label counters=%+v", c)
	}
	if c.PositiveMoves != 1 || c.NegativePrefixMoves != 1 || c.ShadowEntryUpdates != 6 {
		t.Fatalf("update counters=%+v", c)
	}

	updatesBefore := c.ShadowEntryUpdates
	capture := NewMove(E4, D5, WhitePawn, BlackPawn, NoType, Capture)
	observer.observeCompletedNode(&node, []Move{q1, q2}, capture, 10, 0, 100, 50, 3, false, false, false)
	if observer.counters.ShadowEntryUpdates != updatesBefore {
		t.Fatal("noisy winner changed shadow history")
	}
	if observer.counters.NoisyCutoffNodes != 1 || observer.counters.SuppressedNoisyNegativeMoves != 2 {
		t.Fatalf("noisy suppression counters=%+v", observer.counters)
	}
}

func TestH0HistoryShadowRanksAndLMR(t *testing.T) {
	q1 := h0TestQuiet(E2, E4, WhitePawn)
	q2 := h0TestQuiet(D2, D4, WhitePawn)
	q3 := h0TestQuiet(G1, F3, WhiteKnight)
	node := h0HistoryShadowNode{count: 3}
	node.moves[0] = h0HistoryShadowMoveScore{move: q1, current: 100, shadow: 0}
	node.moves[1] = h0HistoryShadowMoveScore{move: q2, current: 0, shadow: 100}
	node.moves[2] = h0HistoryShadowMoveScore{move: q3, current: 50, shadow: 0}
	currentRank, shadowRank, ok := node.ranks(q2)
	if !ok || currentRank != 3 || shadowRank != 1 {
		t.Fatalf("ranks=%d/%d ok=%v want=3/1/true", currentRank, shadowRank, ok)
	}

	observer := new(h0HistoryShadowObserver)
	observer.main[q2.MovingPiece()][q2.Destination()] = 5000
	observation := observer.observeLMRDecision(&node, q2, -600, 2, 3, 10, false, false)
	if !observation.active || observation.actualReduction != 3 || observation.policy11 != 0 {
		t.Fatalf("LMR observation=%+v", observation)
	}
	if observer.counters.LMRBaselineRecomputeMismatches != 0 || observer.counters.LMRPolicy11Changed != 1 {
		t.Fatalf("LMR counters=%+v", observer.counters)
	}
	observer.observeLMROutcome(observation, 1, -1, 0, 1)
	if observer.counters.LMRActualReducedFailHighs != 1 || observer.counters.LMRVerificationRefuted != 1 {
		t.Fatalf("LMR outcome counters=%+v", observer.counters)
	}
}

func TestH0HistoryShadowPruneCounterfactualAndLifecycle(t *testing.T) {
	move := h0TestQuiet(E2, E4, WhitePawn)
	node := h0HistoryShadowNode{prev: EmptyMove, prev2: EmptyMove}
	observer := new(h0HistoryShadowObserver)
	observer.main[move.MovingPiece()][move.Destination()] = -2000

	baseline, shadowOnly := observer.observeHistoryPrune(&node, move, 0, 4)
	if baseline || !shadowOnly {
		t.Fatalf("prune decisions baseline=%v shadowOnly=%v", baseline, shadowOnly)
	}
	observer.observeShadowPruneOutcome(shadowOnly, 5, 0, 10, true)
	if observer.counters.ShadowOnlyPruneAlphaRaisers != 1 || observer.counters.ShadowOnlyPruneBestMoves != 1 {
		t.Fatalf("prune outcome counters=%+v", observer.counters)
	}

	observer.main[move.MovingPiece()][move.Destination()] = 0
	baseline, shadowOnly = observer.observeHistoryPrune(&node, move, -2000, 4)
	if !baseline || shadowOnly || observer.counters.BaselinePruneShadowRescues != 1 {
		t.Fatalf("rescue decisions baseline=%v shadowOnly=%v counters=%+v", baseline, shadowOnly, observer.counters)
	}

	var live workerHistory
	live.h0Shadow = observer
	live.historyTable[move.MovingPiece()][move.Destination()] = 123
	observer.main[move.MovingPiece()][move.Destination()] = 456
	beforeCounters := observer.counters
	live.ClearHistoryTable()
	if live.historyTable[move.MovingPiece()][move.Destination()] != 0 || observer.main[move.MovingPiece()][move.Destination()] != 0 {
		t.Fatal("NewGame lifecycle did not clear live and shadow histories")
	}
	if observer.counters != beforeCounters {
		t.Fatal("table clear unexpectedly erased observer counters")
	}
}
