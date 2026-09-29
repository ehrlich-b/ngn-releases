package engine

import "testing"

// firstCapture returns a legal-ish capture move from a position (pseudo-legal is fine;
// we only need a real captured-piece encoding for indexing).
func firstCapture(t *testing.T, fen string) Move {
	t.Helper()
	pos, err := ParseFEN(fen)
	if err != nil {
		t.Fatal(err)
	}
	var buf [256]Move
	n := GenerateMovesIntoBuffer(pos, buf[:])
	for i := 0; i < n; i++ {
		if buf[i].IsCapture() {
			return buf[i]
		}
	}
	t.Fatalf("no capture move in %q", fen)
	return EmptyMove
}

// TestCaptureHistoryUpdate verifies reward/penalty direction, the gravity bound, and that
// distinct (piece,to,captured) slots are independent.
func TestCaptureHistoryUpdate(t *testing.T) {
	ClearHistoryTable()
	cap := firstCapture(t, "rnbqkbnr/ppp1pppp/8/3p4/4P3/8/PPPP1PPP/RNBQKBNR w KQkq - 0 2") // exd5

	if captureHistoryScore(cap) != 0 {
		t.Fatal("capture history should start at zero")
	}

	for i := 0; i < 200; i++ {
		UpdateCaptureHistory(cap, 8)
	}
	if s := captureHistoryScore(cap); s <= 0 {
		t.Errorf("reward should make score positive, got %d", s)
	} else if s > 8192 {
		t.Errorf("score exceeded gravity bound 8192: %d", s)
	}

	// An unrelated slot must be untouched.
	if defaultSearchEngine.worker.history.captureHistory[WhiteKnight][0][BlackQueen] != 0 {
		t.Error("update bled into an unrelated capture-history slot")
	}

	ClearHistoryTable()
	for i := 0; i < 200; i++ {
		PenalizeCaptureHistory(cap, 8)
	}
	if s := captureHistoryScore(cap); s >= 0 {
		t.Errorf("penalty should make score negative, got %d", s)
	} else if s < -8192 {
		t.Errorf("score exceeded gravity floor -8192: %d", s)
	}
}

// TestCaptureHistoryPopulatedByRealSearch is the integration check: a capture-rich search
// must run cleanly and actually fire capture-history updates (proving it is wired in).
func TestCaptureHistoryPopulatedByRealSearch(t *testing.T) {
	ClearHistoryTable()
	pos, err := ParseFEN("r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1") // Kiwipete
	if err != nil {
		t.Fatal(err)
	}

	info := Search(pos, 10)
	if info.BestMove == EmptyMove {
		t.Fatal("search returned no best move")
	}
	if info.Nodes < 10000 {
		t.Fatalf("search suspiciously shallow (%d nodes)", info.Nodes)
	}

	nonzero := 0
	for i := range defaultSearchEngine.worker.history.captureHistory {
		for j := range defaultSearchEngine.worker.history.captureHistory[i] {
			for k := range defaultSearchEngine.worker.history.captureHistory[i][j] {
				if defaultSearchEngine.worker.history.captureHistory[i][j][k] != 0 {
					nonzero++
				}
			}
		}
	}
	if nonzero == 0 {
		t.Error("capture history never updated during a real search — not wired into the search")
	}
	t.Logf("capture-history slots populated after depth-10 search: %d", nonzero)
}
