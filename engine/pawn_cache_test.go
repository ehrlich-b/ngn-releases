package engine

import "testing"

// TestPawnCacheMatchesDirect pins the N6 pawn-structure cache as transparent: the
// memoized passed/doubled/isolated/chain counts must equal a fresh direct
// computation for every position, on both the miss→hit path and an explicit
// nil-cache path. Node-identity proves this indirectly in search; this nails it directly
// and guards against a future term that secretly depends on non-pawn state.
func TestPawnCacheMatchesDirect(t *testing.T) {
	fens := []string{
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",             // startpos
		"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1", // Kiwipete
		"r1bq1rk1/pp2bppp/2n2n2/2pp4/3P4/2N1PN2/PPQ1BPPP/R1B2RK1 w - - 0 10",   // midgame
		"8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1",                            // endgame
		"8/pppppppp/8/8/8/8/PPPPPPPP/8 w - - 0 1",                              // pawns only
		"4k3/8/8/3pP3/8/8/8/4K3 w - - 0 1",                                     // passed pawns
		"4k3/p6p/8/8/8/8/P2PP3/4K3 w - - 0 1",                                  // isolated/doubled mix
	}

	for _, fen := range fens {
		pos, err := ParseFEN(fen)
		if err != nil {
			t.Fatalf("ParseFEN(%q): %v", fen, err)
		}
		wp := pos.Board.GetBitboardOf(WhitePawn)
		bp := pos.Board.GetBitboardOf(BlackPawn)

		// Direct computation via the underlying term functions.
		dWP := countPassedPawnsFast(wp, bp, White)
		dBP := countPassedPawnsFast(bp, wp, Black)
		dWPW := passedRankWeightedSum(wp, bp, White)
		dBPW := passedRankWeightedSum(bp, wp, Black)
		dWD, dBD := 0, 0
		for file := 0; file < 8; file++ {
			dWD += countDoubledPawns(&pos.Board, White, file)
			dBD += countDoubledPawns(&pos.Board, Black, file)
		}
		dWI := countIsolatedPawns(&pos.Board, White)
		dBI := countIsolatedPawns(&pos.Board, Black)
		dCW := evaluatePawnChains(&pos.Board, White)
		dCB := evaluatePawnChains(&pos.Board, Black)

		check := func(label string, cache *pawnCache) {
			wPassed, bPassed, wPassedW, bPassedW, wDoubled, bDoubled, wIso, bIso, cW, cB := pawnStructureCounts(cache, &pos.Board, wp, bp)
			if wPassed != dWP || bPassed != dBP || wPassedW != dWPW || bPassedW != dBPW || wDoubled != dWD || bDoubled != dBD ||
				wIso != dWI || bIso != dBI || cW != dCW || cB != dCB {
				t.Errorf("fen %q [%s]: cached (%d %d %d %d %d %d %d %d %d %d) != direct (%d %d %d %d %d %d %d %d %d %d)",
					fen, label, wPassed, bPassed, wPassedW, bPassedW, wDoubled, bDoubled, wIso, bIso, cW, cB,
					dWP, dBP, dWPW, dBPW, dWD, dBD, dWI, dBI, dCW, dCB)
			}
		}

		var cache pawnCache
		check("miss", &cache) // first call populates the entry
		check("hit", &cache)  // second call reads it back
		check("nil", nil)     // direct computation with no cache owner
	}
}
