package engine

import "testing"

// TestSEECorrectness checks staticExchangeEvaluation against hand-computed
// exchange values on crafted positions. Piece weights: P=100 N=320 B=330 R=525
// Q=1000. SEE feeds capture ordering AND pruning (SEE-prune, probcut, qsearch
// delta), so a bug here corrupts the search quietly across the board.
func TestSEECorrectness(t *testing.T) {
	cases := []struct {
		name string
		fen  string
		uci  string // move in UCI (matches Move.ToString())
		want int
	}{
		{
			name: "pawn takes undefended knight",
			fen:  "4k3/8/8/3n4/4P3/8/8/4K3 w - - 0 1",
			uci:  "e4d5", want: 320,
		},
		{
			name: "pawn takes pawn defended by pawn (equal)",
			fen:  "4k3/8/2p5/3p4/4P3/8/8/4K3 w - - 0 1",
			uci:  "e4d5", want: 0,
		},
		{
			name: "queen takes pawn defended by pawn (lose queen for pawn)",
			fen:  "4k3/8/2p5/3p4/8/8/8/3QK3 w - - 0 1",
			uci:  "d1d5", want: 100 - 1000,
		},
		{
			name: "knight takes pawn defended by pawn",
			fen:  "4k3/8/4p3/3p4/8/2N5/8/4K3 w - - 0 1",
			uci:  "c3d5", want: 100 - 320,
		},
		{
			name: "rook battery x-ray recapture (win pawn, trade rooks)",
			fen:  "4r1k1/8/8/4p3/8/4R3/8/4R1K1 w - - 0 1",
			uci:  "e3e5", want: 100,
		},
		{
			name: "pawn promotes capturing undefended rook",
			fen:  "r5k1/1P6/8/8/8/8/8/6K1 w - - 0 1",
			uci:  "b7a8q", want: 525 + (1000 - 100),
		},
		{
			name: "en passant captures undefended pawn",
			fen:  "4k3/8/8/3pP3/8/8/8/4K3 w - d6 0 1",
			uci:  "e5d6", want: 100,
		},
		// Edge-file regression cases (off-by-file SEE bug fixed 2026-05-30): the
		// pawn-attacker file guards were swapped, so a recapturing pawn whose target
		// sits on the A/H file was missed (SEE over-pessimistic) or a wrap-around
		// attacker was invented. All three FAILED before the fix.
		{
			name: "rook takes h-file pawn defended by g-pawn (h-file recapture)",
			fen:  "4k3/8/6p1/7p/8/8/8/4K2R w - - 0 1",
			uci:  "h1h5", want: 100 - 525,
		},
		{
			name: "rook takes a-file pawn defended by b-pawn (a-file recapture)",
			fen:  "4k3/8/1p6/p7/8/8/8/R3K3 w - - 0 1",
			uci:  "a1a5", want: 100 - 525,
		},
		{
			name: "kiwipete Qxh3: g2 pawn recaptures the rook (h-file, was -900)",
			fen:  "r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
			uci:  "f3h3", want: 100 - 1000 + 525,
		},
	}

	for _, c := range cases {
		pos, err := ParseFEN(c.fen)
		if err != nil {
			t.Errorf("%s: bad FEN %q: %v", c.name, c.fen, err)
			continue
		}
		var move Move = EmptyMove
		for _, m := range GenerateLegalMoves(pos) {
			if m.ToString() == c.uci {
				move = m
				break
			}
		}
		if move == EmptyMove {
			t.Errorf("%s: move %s not legal in %q", c.name, c.uci, c.fen)
			continue
		}
		got := staticExchangeEvaluation(pos, move)
		if got != c.want {
			t.Errorf("%s: SEE(%s)=%d, want %d   [%s]", c.name, c.uci, got, c.want, c.fen)
		}
	}
}

// TestSEEQuietMoves checks staticExchangeEvaluation on QUIET (non-capture) moves —
// the basis of quiet-move SEE pruning. For a quiet move gain[0]=0 and the swap
// evaluates whether the moving piece survives on its destination square: a piece
// moving onto a square the opponent can win it on returns a negative value.
func TestSEEQuietMoves(t *testing.T) {
	cases := []struct {
		name string
		fen  string
		uci  string
		want int
	}{
		{
			// Knight to an empty, unattacked square — nothing recaptures.
			name: "quiet move to safe square",
			fen:  "4k3/8/8/8/8/2N5/8/4K3 w - - 0 1",
			uci:  "c3e4", want: 0,
		},
		{
			// Queen onto c4, attacked by the d5 pawn and undefended — loses the queen.
			name: "queen onto pawn-attacked undefended square",
			fen:  "4k3/8/8/3p4/8/8/Q7/4K3 w - - 0 1",
			uci:  "a2c4", want: -1000,
		},
		{
			// Knight onto e4, attacked by the d5 pawn but defended by the d3 pawn:
			// ...dxe4, dxe4 — lose knight (320), win back a pawn (100) = -220.
			name: "knight onto pawn-attacked pawn-defended square",
			fen:  "4k3/8/8/3p4/8/2NP4/8/4K3 w - - 0 1",
			uci:  "c3e4", want: 100 - 320,
		},
		{
			// Bishop onto e4, attacked only by the h7 queen but defended by the g3
			// knight: QxB? then NxQ — the queen declines, so the bishop is safe = 0.
			name: "bishop onto queen-attacked knight-defended square (queen declines)",
			fen:  "4k3/1B5q/8/8/8/6N1/8/4K3 w - - 0 1",
			uci:  "b7e4", want: 0,
		},
	}

	for _, c := range cases {
		pos, err := ParseFEN(c.fen)
		if err != nil {
			t.Errorf("%s: bad FEN %q: %v", c.name, c.fen, err)
			continue
		}
		var move Move = EmptyMove
		for _, m := range GenerateLegalMoves(pos) {
			if m.ToString() == c.uci {
				move = m
				break
			}
		}
		if move == EmptyMove {
			t.Errorf("%s: move %s not legal in %q", c.name, c.uci, c.fen)
			continue
		}
		if move.IsCapture() {
			t.Errorf("%s: %s is a capture, not a quiet move", c.name, c.uci)
			continue
		}
		got := staticExchangeEvaluation(pos, move)
		if got != c.want {
			t.Errorf("%s: SEE(%s)=%d, want %d   [%s]", c.name, c.uci, got, c.want, c.fen)
		}
	}
}

// TestSEEQuietPruningWired proves quiet-move SEE pruning actually fires in a real
// search and is controlled by the SEEPrune toggle (the mechanism check for the
// cool loop — a correct SEE value is useless if it isn't wired into the tree).
func TestSEEQuietPruningWired(t *testing.T) {
	// Kiwipete: a tactically dense middlegame with many quiet moves that hang a
	// piece on their destination — guaranteed soft-hangs at the depth<=4 frontier.
	const kiwipete = "r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1"

	reset := func() {
		defaultSearchEngine, _ = NewSearchEngineWithHash(DEFAULT_CACHE_SIZE)
		ClearHistoryTable()
		ClearKillerMoves()
		ClearCounterMoves()
		SetLastMovePlayed(EmptyMove)
	}

	pos, err := ParseFEN(kiwipete)
	if err != nil {
		t.Fatalf("bad FEN: %v", err)
	}

	defer func() { SearchToggles.SEEPrune = true }()

	reset()
	SearchToggles.SEEPrune = true
	on := Search(pos, 8)
	if on.SEEQuietPrunes == 0 {
		t.Error("SEEPrune ON: quiet SEE pruning never fired in a depth-8 Kiwipete search")
	}

	pos, _ = ParseFEN(kiwipete)
	reset()
	SearchToggles.SEEPrune = false
	off := Search(pos, 8)
	if off.SEEQuietPrunes != 0 {
		t.Errorf("SEEPrune OFF: quiet SEE pruning fired %d times — toggle not respected", off.SEEQuietPrunes)
	}
}

// TestQSearchTTStoresWired proves the quiescence search writes its fail-high
// results to the transposition table (S3). A correct store is a no-op if it never
// runs, so assert the counter is non-zero after a tactically dense search.
func TestQSearchTTStoresWired(t *testing.T) {
	const kiwipete = "r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1"

	defaultSearchEngine, _ = NewSearchEngineWithHash(DEFAULT_CACHE_SIZE)
	ClearHistoryTable()
	ClearKillerMoves()
	ClearCounterMoves()
	SetLastMovePlayed(EmptyMove)

	pos, err := ParseFEN(kiwipete)
	if err != nil {
		t.Fatalf("bad FEN: %v", err)
	}
	info := Search(pos, 8)
	if info.QSearchTTStores == 0 {
		t.Error("quiescence never stored a fail-high to the TT in a depth-8 Kiwipete search")
	}
}
