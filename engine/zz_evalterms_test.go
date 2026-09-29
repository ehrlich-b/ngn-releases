package engine

import (
	"fmt"
	"testing"
)

// TestZZEvalTerms (throwaway diagnostic): decompose the WHITE-POV eval into its
// aux terms on L1's blind-spot FENs, by zeroing each term's MG+EG weight and
// measuring the delta. The base (all aux zeroed) is PeSTO material+PST. Goal:
// find which term(s) drive NGN's ~150-220cp optimism vs Stockfish.
func TestZZEvalTerms(t *testing.T) {

	fens := []struct{ name, fen string }{
		// name carries NGN's color and the WHITE-POV truth per SF (so a wrong-sign
		// NGN term jumps out). NGN over-favors its own side on all four.
		{"L47 NGN=black  SFwPOV=+149", "3rk2r/ppn2pp1/2p2n1p/4bB2/7P/P1N1P3/1PP2PP1/R1BK3R w k - 1 1"},
		{"L48 NGN=white  SFwPOV=-113", "5r2/1pr2pp1/p2P4/P2P1kp1/1R6/3p2P1/5P1P/5RK1 b - - 0 1"},
		{"L50 NGN=black  SFwPOV=+138", "r1b1kb1r/5ppp/n1P1pn2/p2p4/Q2N3P/B1P1PPq1/P5P1/RN1K1B1R w kq - 7 1"},
		{"L42 NGN=black  SFwPOV=+160", "r7/1bq3k1/2p2np1/1pNp1p2/rBnP3P/4P2P/4QPB1/2R1R1K1 b - - 4 1"},
	}
	terms := []struct {
		name   string
		mg, eg *int
	}{
		{"passed", &passedPawnBonus, &passedPawnBonusEG},
		{"doubled", &doubledPawnPenalty, &doubledPawnPenaltyEG},
		{"isolated", &isolatedPawnPenalty, &isolatedPawnPenaltyEG},
		{"chain", &pawnChainWeight, &pawnChainWeightEG},
		{"mobility", &mobilityWeight, &mobilityWeightEG},
		{"rookOpen", &rookOpenWeight, &rookOpenWeightEG},
		{"outpost", &outpostWeight, &outpostWeightEG},
		{"kingSafety", &kingSafetyWeight, &kingSafetyWeightEG},
		{"kingActivity", &kingActivityWeight, &kingActivityWeightEG},
	}
	for _, f := range fens {
		pos, err := ParseFEN(f.fen)
		if err != nil {
			t.Fatalf("%s: %v", f.name, err)
		}
		board := &pos.Board
		full := evaluateUnsafe(board)
		// base: all aux zeroed -> PeSTO material+PST only
		saved := make([][2]int, len(terms))
		for i, tm := range terms {
			saved[i] = [2]int{*tm.mg, *tm.eg}
			*tm.mg, *tm.eg = 0, 0
		}
		base := evaluateUnsafe(board)
		for i, tm := range terms {
			*tm.mg, *tm.eg = saved[i][0], saved[i][1]
		}
		fmt.Printf("\n=== %s ===\n  full(wPOV)=%+d   material+PST=%+d   aux-sum=%+d\n", f.name, full, base, full-base)
		for _, tm := range terms {
			smg, seg := *tm.mg, *tm.eg
			*tm.mg, *tm.eg = 0, 0
			d := full - evaluateUnsafe(board)
			*tm.mg, *tm.eg = smg, seg
			if d != 0 {
				fmt.Printf("    %-12s %+d\n", tm.name, d)
			}
		}
		// Does tapering king-safety in the EG move NGN's full eval toward SF's truth?
		sMg, sEg := kingSafetyWeight, kingSafetyWeightEG
		fmt.Printf("    [kingSafetyEG sweep] ")
		for _, eg := range []int{100, 50, 25, 0} {
			kingSafetyWeightEG = eg
			fmt.Printf("EG=%-3d:%+d   ", eg, evaluateUnsafe(board))
		}
		kingSafetyWeight, kingSafetyWeightEG = sMg, sEg
		fmt.Println()
	}
}

// TestZZRookBehindPasser pins the rook-behind-passer mask logic directly on the
// bitboards (symmetry/faithfulness can't catch a both-sided wrong-direction bug).
func TestZZRookBehindPasser(t *testing.T) {
	bit := func(sq int) uint64 { return uint64(1) << sq }
	cases := []struct {
		name                         string
		ownPawns, oppPawns, oppRooks uint64
		color                        Color
		want                         int
	}{
		{"white a5 passer, black rook a1 behind", bit(32), 0, bit(0), White, 1},
		{"white a5 passer, black rook a8 ahead", bit(32), 0, bit(56), White, 0},
		{"white a5 passer, black rook h1 wrong file", bit(32), 0, bit(7), White, 0},
		{"white a5 not passed (black pawn a6)", bit(32), bit(40), bit(0), White, 0},
		{"black a4 passer, white rook a8 behind", bit(24), 0, bit(56), Black, 1},
		{"black a4 passer, white rook a1 ahead", bit(24), 0, bit(0), Black, 0},
		{"no rooks", bit(32), 0, 0, White, 0},
	}
	for _, c := range cases {
		passers := passersOf(c.ownPawns, c.oppPawns, c.color)
		if got := passedRookBehindCount(passers, c.oppRooks, c.color); got != c.want {
			t.Errorf("%s: got %d want %d", c.name, got, c.want)
		}
	}
}
