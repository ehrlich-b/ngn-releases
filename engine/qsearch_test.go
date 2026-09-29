package engine

import "testing"

func newQSearchInfo() *SearchInfo {
	var moveBuffer [256]Move
	var captureBuffer [64]Move
	var orderedBuffer [256]Move
	var seeGains [32]int
	return &SearchInfo{
		control:       &defaultSearchEngine.worker.control,
		history:       &defaultSearchEngine.worker.history,
		evaluator:     testHCEWorkerEvaluator(nil),
		tt:            defaultTTForDirectTest(),
		MoveBuffer:    &moveBuffer,
		CaptureBuffer: &captureBuffer,
		OrderedBuffer: &orderedBuffer,
		SEEGains:      &seeGains,
	}
}

func qsearchOf(t *testing.T, fen string) (got, standPat int) {
	t.Helper()
	pos, err := ParseFEN(fen)
	if err != nil {
		t.Fatalf("bad FEN %q: %v", fen, err)
	}
	ClearStop()
	ClearHistoryTable() // T4: qsearch stand-pat now carries the pawn correction; clear it so quiet positions read == raw
	defaultSearchEngine, _ = NewSearchEngineWithHash(DEFAULT_CACHE_SIZE)
	standPat = EvaluateForPlayer(&pos.Board, pos.Turn())
	got = quiescence(pos, -INFINITY, INFINITY, 0, newQSearchInfo())
	return
}

// TestQuiescenceInvariants checks core qsearch correctness on crafted positions.
// qsearch produces every leaf evaluation in the search, so a bug here corrupts
// everything above it.
func TestQuiescenceInvariants(t *testing.T) {
	// A) Quiet positions (no captures, not in check): qsearch must return EXACTLY
	// the stand-pat eval.
	quiet := []string{
		"4k3/8/8/8/8/8/4P3/4K3 w - - 0 1",
		"4k3/pp6/8/8/8/8/6PP/4K3 w - - 0 1",
		"r3k3/8/8/8/8/8/8/4K2R w Kq - 0 1",
	}
	for _, fen := range quiet {
		got, standPat := qsearchOf(t, fen)
		if got != standPat {
			t.Errorf("quiet qsearch(%s)=%d, want stand-pat %d", fen, got, standPat)
		}
	}

	// B) A free hanging queen (pawn attacks an undefended queen): qsearch must
	// realize the win, i.e. score far above stand-pat. If delta/SEE pruning wrongly
	// skipped the capture, qsearch would collapse to stand-pat and fail here.
	got, standPat := qsearchOf(t, "4k3/8/8/3q4/4P3/8/8/4K3 w - - 0 1")
	if got-standPat < 800 {
		t.Errorf("hanging-queen qsearch=%d, stand-pat=%d (gain %d); expected to win ~a queen",
			got, standPat, got-standPat)
	}

	// C) Only a losing capture available (QxP defended by a pawn, SEE -900):
	// qsearch must DECLINE it and return stand-pat, not blunder the queen.
	got, standPat = qsearchOf(t, "4k3/8/2p5/3p4/8/8/8/3QK3 w - - 0 1")
	if got != standPat {
		t.Errorf("losing-capture qsearch=%d, want stand-pat %d (should decline QxP)", got, standPat)
	}
}

func TestQuiescenceDepthCapStillHandlesCheckmate(t *testing.T) {
	pos, err := ParseFEN("7k/6Q1/6K1/8/8/8/8/8 b - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	if !pos.IsInCheck() {
		t.Fatal("test position must have side to move in check")
	}
	ClearStop()
	defaultSearchEngine, _ = NewSearchEngineWithHash(DEFAULT_CACHE_SIZE)

	const ply = 6
	got := quiescenceWithDepth(pos, -INFINITY, INFINITY, ply, newQSearchInfo(), 6)
	want := -MATE_VALUE + ply
	if got != want {
		t.Fatalf("qsearch at cap while in check returned %d, want mate score %d", got, want)
	}
}

// Legacy callers may enter qsearch with a non-zero qDepth.  That value must
// not shrink the capture tree: a TT bound written on the same position has to
// cover a normal qDepth-0 call as well.  Before the uncapped search this exact
// Kiwipete window stored a qDepth-5 lower bound of 100, then incorrectly cut a
// qDepth-0 call whose cold result was 51.
func TestQSearchTTBoundsDoNotDependOnLegacyQDepth(t *testing.T) {
	for _, fen := range []string{
		"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
		"r3k2r/pppbbppp/2n2q1P/1P2p3/3pn3/BN2PNP1/P1PPQPB1/R3K2R b KQkq - 0 1",
	} {
		for _, legacyQDepth := range []int{1, 5, 6, 9} {
			pos, err := ParseFEN(fen)
			if err != nil {
				t.Fatal(err)
			}
			before := pos.Copy()
			history := make(map[uint64]int, len(pos.Positions))
			for key, count := range pos.Positions {
				history[key] = count
			}
			assertPreserved := func(stage string) {
				t.Helper()
				if !PositionEqual(before, pos) {
					t.Fatalf("%s changed caller state at legacy qDepth %d", stage, legacyQDepth)
				}
				if len(pos.Positions) != len(history) {
					t.Fatalf("%s changed caller history size at legacy qDepth %d", stage, legacyQDepth)
				}
				for key, want := range history {
					if got := pos.Positions[key]; got != want {
						t.Fatalf("%s changed caller history at legacy qDepth %d", stage, legacyQDepth)
					}
				}
			}
			ClearStop()
			ClearHistoryTable()
			defaultSearchEngine, _ = NewSearchEngineWithHash(DEFAULT_CACHE_SIZE)
			_ = quiescenceWithDepth(pos, -INFINITY, 100, 5, newQSearchInfo(), legacyQDepth)
			assertPreserved("legacy search")
			warm := quiescenceWithDepth(pos, -INFINITY, 100, 5, newQSearchInfo(), 0)
			assertPreserved("warm search")
			ClearStop()
			ClearHistoryTable()
			defaultSearchEngine, _ = NewSearchEngineWithHash(DEFAULT_CACHE_SIZE)
			cold := quiescenceWithDepth(pos, -INFINITY, 100, 5, newQSearchInfo(), 0)
			assertPreserved("cold search")
			if warm != cold {
				t.Fatalf("legacy qDepth %d poisoned qDepth0: warm=%d cold=%d FEN=%s", legacyQDepth, warm, cold, fen)
			}
		}
	}
}

func TestQSearchLegacyQDepthStillHonorsNodeStop(t *testing.T) {
	pos, err := ParseFEN("r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	before := pos.Copy()
	historyLen := len(pos.Positions)
	history := make(map[uint64]int, historyLen)
	for key, count := range pos.Positions {
		history[key] = count
	}
	ClearStop()
	ClearHistoryTable()
	defaultSearchEngine, _ = NewSearchEngineWithHash(DEFAULT_CACHE_SIZE)
	info := newQSearchInfo()
	info.maxNodes = 1
	_ = quiescenceWithDepth(pos, -INFINITY, INFINITY, 0, info, 9)
	if !info.Stopped || info.Nodes == 0 {
		t.Fatalf("legacy qDepth capture search did not reach node stop: stopped=%v nodes=%d", info.Stopped, info.Nodes)
	}
	if !PositionEqual(before, pos) || len(pos.Positions) != historyLen {
		t.Fatal("bounded qsearch changed caller state")
	}
	for key, want := range history {
		if got := pos.Positions[key]; got != want {
			t.Fatal("bounded qsearch changed caller history")
		}
	}
	ClearStop()
}

// TestPawnCorrectionAppliedAtQsearchStandPat is the T4 red->green guard: the qsearch stand-pat must
// carry the pawn-structure correction that interior nodes apply (search.go ~1353). Pre-patch the
// qsearch returned the RAW stand-pat and discarded the correction, so this assertion (got == raw+corr)
// failed; post-patch qsearch backs up the corrected static value.
func TestPawnCorrectionAppliedAtQsearchStandPat(t *testing.T) {
	pos, err := ParseFEN("4k3/pp6/8/8/8/8/6PP/4K3 w - - 0 1") // quiet: not in check, no captures available
	if err != nil {
		t.Fatal(err)
	}
	ClearStop()
	ClearHistoryTable()
	defaultSearchEngine, _ = NewSearchEngineWithHash(DEFAULT_CACHE_SIZE)

	stm := pos.Turn()
	idx := pawnCorrectionIndex(pos.Board.pieces[WhitePawn], pos.Board.pieces[BlackPawn])
	defaultSearchEngine.worker.history.pawnCorrectionHistory[stm][idx] = corrHistLimit / 2 // poison the slot -> a clear, in-bounds non-zero correction
	corr := correctionValue(stm, idx)
	if corr == 0 {
		t.Fatal("test setup: poisoned correction must be non-zero")
	}
	raw := EvaluateForPlayer(&pos.Board, stm)

	got := quiescence(pos, -INFINITY, INFINITY, 0, newQSearchInfo())
	if got != raw+corr {
		t.Errorf("qsearch stand-pat = %d, want raw %d + correction %d = %d (correction not applied at qsearch stand-pat)",
			got, raw, corr, raw+corr)
	}
	ClearHistoryTable() // don't leak the poisoned slot into other tests
}

// TestNonPawnCorrectionAppliedAtQsearchStandPat is the T4-nonpawn red->green guard: the qsearch
// stand-pat must also carry the non-pawn-structure correction that interior nodes apply (search.go
// ~1358). It mirrors the pawn guard above but poisons the non-pawn slot (the pawn table stays zero),
// so got must equal raw + non-pawn correction. Pre-patch (correctedStandPat without the non-pawn term)
// this fails; post-patch qsearch backs up the corrected static value.
func TestNonPawnCorrectionAppliedAtQsearchStandPat(t *testing.T) {
	pos, err := ParseFEN("4k3/pp6/8/8/8/8/6PP/4K3 w - - 0 1") // quiet: not in check, no captures available
	if err != nil {
		t.Fatal(err)
	}
	ClearStop()
	ClearHistoryTable()
	defaultSearchEngine, _ = NewSearchEngineWithHash(DEFAULT_CACHE_SIZE)

	stm := pos.Turn()
	idx := nonPawnCorrectionIndex(pos.Board.GetWhitePieces()&^pos.Board.pieces[WhitePawn], pos.Board.GetBlackPieces()&^pos.Board.pieces[BlackPawn])
	defaultSearchEngine.worker.history.nonPawnCorrectionHistory[stm][idx] = corrHistLimit / 2 // poison the slot -> a clear, in-bounds non-zero correction
	corr := nonPawnCorrectionValue(stm, idx)
	if corr == 0 {
		t.Fatal("test setup: poisoned correction must be non-zero")
	}
	raw := EvaluateForPlayer(&pos.Board, stm)

	got := quiescence(pos, -INFINITY, INFINITY, 0, newQSearchInfo())
	if got != raw+corr {
		t.Errorf("qsearch stand-pat = %d, want raw %d + non-pawn correction %d = %d (non-pawn correction not applied at qsearch stand-pat)",
			got, raw, corr, raw+corr)
	}
	ClearHistoryTable() // don't leak the poisoned slot into other tests
}

// TestMinorCorrectionAppliedAtQsearchStandPat is the T4c red->green guard: the qsearch stand-pat
// must also carry the minor-piece correction that interior nodes apply (search.go ~1360). It mirrors
// the pawn/non-pawn guards but poisons the minor slot (the pawn and non-pawn tables stay zero), so got
// must equal raw + minor correction. The position holds knights so the minor key is a real hashed slot,
// not the degenerate empty-minor slot 0. Pre-patch (correctedStandPat without the minor term) this
// fails; post-patch qsearch backs up the corrected static value.
func TestMinorCorrectionAppliedAtQsearchStandPat(t *testing.T) {
	pos, err := ParseFEN("4k3/pp6/8/3n4/3N4/8/6PP/4K3 w - - 0 1") // quiet: not in check, no captures available
	if err != nil {
		t.Fatal(err)
	}
	ClearStop()
	ClearHistoryTable()
	defaultSearchEngine, _ = NewSearchEngineWithHash(DEFAULT_CACHE_SIZE)

	stm := pos.Turn()
	idx := minorCorrectionIndex(pos.Board.pieces[WhiteKnight]|pos.Board.pieces[WhiteBishop], pos.Board.pieces[BlackKnight]|pos.Board.pieces[BlackBishop])
	defaultSearchEngine.worker.history.minorCorrectionHistory[stm][idx] = corrHistLimit / 2 // poison the slot -> a clear, in-bounds non-zero correction
	corr := minorCorrectionValue(stm, idx)
	if corr == 0 {
		t.Fatal("test setup: poisoned correction must be non-zero")
	}
	raw := EvaluateForPlayer(&pos.Board, stm)

	got := quiescence(pos, -INFINITY, INFINITY, 0, newQSearchInfo())
	if got != raw+corr {
		t.Errorf("qsearch stand-pat = %d, want raw %d + minor correction %d = %d (minor correction not applied at qsearch stand-pat)",
			got, raw, corr, raw+corr)
	}
	ClearHistoryTable() // don't leak the poisoned slot into other tests
}
