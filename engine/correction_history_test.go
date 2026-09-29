package engine

import "testing"

// TestPawnCorrectionIndex verifies the structure hash is deterministic, in range, and
// sensitive to the pawn skeleton (distinct structures should not all collide to one slot).
func TestPawnCorrectionIndex(t *testing.T) {
	wp := uint64(0x000000000000FF00) // white pawns on rank 2
	bp := uint64(0x00FF000000000000) // black pawns on rank 7

	i1 := pawnCorrectionIndex(wp, bp)
	if i1 < 0 || i1 >= corrHistSize {
		t.Fatalf("index out of range: %d", i1)
	}
	if pawnCorrectionIndex(wp, bp) != i1 {
		t.Fatal("index not deterministic for identical structure")
	}

	wpE4 := (wp &^ (uint64(1) << 12)) | (uint64(1) << 28) // e2->e4
	wpD4 := (wp &^ (uint64(1) << 11)) | (uint64(1) << 27) // d2->d4
	bpE5 := (bp &^ (uint64(1) << 52)) | (uint64(1) << 36) // e7->e5

	seen := map[int]bool{}
	for _, s := range [][2]uint64{{wp, bp}, {wpE4, bp}, {wpD4, bp}, {wp, bpE5}} {
		seen[pawnCorrectionIndex(s[0], s[1])] = true
	}
	if len(seen) < 2 {
		t.Errorf("index insensitive to pawn structure: all %d structures mapped to one slot", len(seen))
	}
}

// TestCorrectionEMA verifies the gravity EMA: repeated same-sign updates push the applied
// correction in that direction, stay within the designed +-49cp bound, and reset on clear.
func TestCorrectionEMA(t *testing.T) {
	ClearHistoryTable()
	idx := 123
	if correctionValue(White, idx) != 0 {
		t.Fatal("correction should start at zero")
	}

	// Search keeps finding this node ~120cp better than static eval at depth 8.
	for i := 0; i < 64; i++ {
		updatePawnCorrection(White, idx, 120, 8)
	}
	if pos := correctionValue(White, idx); pos <= 0 {
		t.Errorf("positive gap should yield positive correction, got %d", pos)
	} else if pos > 49 {
		t.Errorf("correction exceeded designed +-49cp bound: %d", pos)
	}

	ClearHistoryTable()
	for i := 0; i < 64; i++ {
		updatePawnCorrection(White, idx, -120, 8)
	}
	if neg := correctionValue(White, idx); neg >= 0 {
		t.Errorf("negative gap should yield negative correction, got %d", neg)
	} else if neg < -49 {
		t.Errorf("correction exceeded designed +-49cp bound: %d", neg)
	}

	// Colors index independently.
	ClearHistoryTable()
	updatePawnCorrection(White, idx, 200, 8)
	if correctionValue(Black, idx) != 0 {
		t.Error("update bled across side-to-move")
	}
}

// TestCorrectionGate verifies maybeUpdatePawnCorrection only learns from clean, directional
// signals: never in check, never from a capture best move, and only when the search result
// disagrees with static eval consistently.
func TestCorrectionGate(t *testing.T) {
	pos, err := ParseFEN("rnbqkbnr/ppp1pppp/8/3p4/4P3/8/PPPP1PPP/RNBQKBNR w KQkq - 0 2")
	if err != nil {
		t.Fatal(err)
	}
	var quiet, capture Move
	var buf [256]Move
	n := GenerateMovesIntoBuffer(pos, buf[:])
	for i := 0; i < n; i++ {
		if buf[i].IsCapture() {
			if capture == EmptyMove {
				capture = buf[i]
			}
		} else if quiet == EmptyMove {
			quiet = buf[i]
		}
	}
	if quiet == EmptyMove || capture == EmptyMove {
		t.Fatalf("test position needs both a quiet and a capture move (quiet=%v capture=%v)", quiet, capture)
	}

	idx := 77

	ClearHistoryTable()
	maybeUpdatePawnCorrection(White, idx, 0, 200, 50, 8, quiet, true) // in check
	if correctionValue(White, idx) != 0 {
		t.Error("updated correction while in check")
	}

	maybeUpdatePawnCorrection(White, idx, 0, 200, 50, 8, capture, false) // capture best move
	if correctionValue(White, idx) != 0 {
		t.Error("learned correction from a capture best move")
	}

	// Fail-high quiet: bestScore(200) > static(0) with a best move => positive correction.
	maybeUpdatePawnCorrection(White, idx, 0, 200, 50, 8, quiet, false)
	if correctionValue(White, idx) <= 0 {
		t.Error("no positive correction on a quiet fail-high")
	}

	// Fail-low: bestScore(-200) < static(0) and < beta => negative correction.
	ClearHistoryTable()
	maybeUpdatePawnCorrection(White, idx, 0, -200, 50, 8, quiet, false)
	if correctionValue(White, idx) >= 0 {
		t.Error("no negative correction on a fail-low")
	}
}

// TestCorrectionPopulatedByRealSearch is the integration check: a real iterative-deepening
// search must run cleanly with correction history active and must actually fire updates
// (proving the table is wired into the search, not a silent no-op).
func TestCorrectionPopulatedByRealSearch(t *testing.T) {
	ClearHistoryTable()
	pos, err := ParseFEN("r1bqkb1r/pppp1ppp/2n2n2/4p3/2B1P3/5N2/PPPP1PPP/RNBQK2R w KQkq - 4 4")
	if err != nil {
		t.Fatal(err)
	}

	info := Search(pos, 10)
	if info.BestMove == EmptyMove {
		t.Fatal("search returned no best move")
	}
	if info.Nodes < 10000 {
		t.Fatalf("search suspiciously shallow (%d nodes) — correction may be breaking the search", info.Nodes)
	}

	nonzero := 0
	for c := range defaultSearchEngine.worker.history.pawnCorrectionHistory {
		for i := range defaultSearchEngine.worker.history.pawnCorrectionHistory[c] {
			if defaultSearchEngine.worker.history.pawnCorrectionHistory[c][i] != 0 {
				nonzero++
			}
		}
	}
	if nonzero == 0 {
		t.Error("correction history never updated during a real search — not wired into the search")
	}
	t.Logf("correction slots populated after depth-10 search: %d", nonzero)
}
