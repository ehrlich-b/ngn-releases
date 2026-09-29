package engine

import "testing"

func h1TestQuiet(from, to Square, piece Piece) Move {
	return NewMove(from, to, piece, NoPiece, NoType, 0)
}

func TestH1HistoryResponseRateAndBounds(t *testing.T) {
	if got := h1HistoryRate(30); got != 400 {
		t.Fatalf("capped rate=%d want=400", got)
	}
	if got := h1HistoryResponse(0, historyMax, 400); got != 6400 {
		t.Fatalf("first response=%d want=6400", got)
	}
	if got := h1HistoryResponse(6400, historyMax, 400); got != 7800 {
		t.Fatalf("second response=%d want=7800", got)
	}
	if got := h1HistoryResponse(1, -historyMax, 1); got != -15 {
		t.Fatalf("negative truncation=%d want=-15", got)
	}
	entry := 0
	for i := 0; i < 1000; i++ {
		entry = h1HistoryResponse(entry, historyMax, 400)
		if entry > historyMax {
			t.Fatalf("positive bound escaped: %d", entry)
		}
	}
	for i := 0; i < 1000; i++ {
		entry = h1HistoryResponse(entry, -historyMax, 400)
		if entry < -historyMax {
			t.Fatalf("negative bound escaped: %d", entry)
		}
	}
}

func TestH1CompletedNodeLearnsOnlyQuietWinnerPrefix(t *testing.T) {
	q1 := h1TestQuiet(E2, E4, WhitePawn)
	q2 := h1TestQuiet(D2, D4, WhitePawn)
	q3 := h1TestQuiet(G1, F3, WhiteKnight)
	prev := h1TestQuiet(E7, E5, BlackPawn)
	prev2 := h1TestQuiet(B1, C3, WhiteKnight)
	var history workerHistory

	history.h1UpdateCompletedNode([]Move{q1, q2, q3}, q2, prev, prev2, 10, 0, 20)
	const want = 1600
	if got := history.historyTable[q1.MovingPiece()][q1.Destination()]; got != -want {
		t.Fatalf("prefix loser=%d want=%d", got, -want)
	}
	if got := history.historyTable[q2.MovingPiece()][q2.Destination()]; got != want {
		t.Fatalf("winner=%d want=%d", got, want)
	}
	if got := history.historyTable[q3.MovingPiece()][q3.Destination()]; got != 0 {
		t.Fatalf("later alternative changed: %d", got)
	}
	if got := history.continuationHistory[prev.MovingPiece()][prev.Destination()][q2.MovingPiece()][q2.Destination()]; got != want {
		t.Fatalf("winner continuation=%d want=%d", got, want)
	}
	if got := history.followupHistory[prev2.MovingPiece()][prev2.Destination()][q2.MovingPiece()][q2.Destination()]; got != want {
		t.Fatalf("winner followup=%d want=%d", got, want)
	}
}

func TestH1CompletedNodeRejectsFailLowNoisyAndMissingWinner(t *testing.T) {
	quiet := h1TestQuiet(E2, E4, WhitePawn)
	other := h1TestQuiet(D2, D4, WhitePawn)
	capture := NewMove(E4, D5, WhitePawn, BlackPawn, NoType, Capture)
	tests := []struct {
		name      string
		searched  []Move
		best      Move
		origAlpha int
		bestScore int
	}{
		{"fail low", []Move{quiet}, quiet, 10, 10},
		{"noisy winner", []Move{quiet}, capture, 0, 20},
		{"winner missing", []Move{other}, quiet, 0, 20},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var history workerHistory
			history.h1UpdateCompletedNode(test.searched, test.best, EmptyMove, EmptyMove, 10, test.origAlpha, test.bestScore)
			if got := history.GetHistoryScore(quiet, EmptyMove, EmptyMove); got != 0 {
				t.Fatalf("rejected label changed quiet history: %d", got)
			}
		})
	}
}

func TestH1NormalizedLMRTerm(t *testing.T) {
	tests := []struct {
		score int
		want  int
	}{
		{-9000, 2}, {-5000, 2}, {-2499, 0}, {0, 0}, {2499, 0}, {2500, -1}, {5000, -2}, {9000, -2},
	}
	for _, test := range tests {
		if got := h1NormalizedLMRTerm(test.score); got != test.want {
			t.Errorf("score %d: term=%d want=%d", test.score, got, test.want)
		}
	}
}

func TestH1DoesNotChangeCaptureHistoryArithmetic(t *testing.T) {
	capture := NewMove(E4, D5, WhitePawn, BlackPawn, NoType, Capture)
	var history workerHistory
	history.UpdateCaptureHistory(capture, 8)
	first := history.captureHistoryScore(capture)
	if first != 64 {
		t.Fatalf("first capture-history update=%d want=64", first)
	}
	history.UpdateCaptureHistory(capture, 8)
	want := first
	gravityUpdate(&want, 64)
	if got := history.captureHistoryScore(capture); got != want {
		t.Fatalf("second capture-history update=%d want=%d", got, want)
	}
	before := history.captureHistoryScore(capture)
	history.h1UpdateCompletedNode(nil, capture, EmptyMove, EmptyMove, 12, 0, 100)
	if got := history.captureHistoryScore(capture); got != before {
		t.Fatalf("completed noisy winner changed capture history: %d -> %d", before, got)
	}
}
