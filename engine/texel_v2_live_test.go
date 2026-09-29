package engine

import (
	"math/bits"
	"strconv"
	"strings"
	"testing"
)

// These are evaluator-coverage boards, not a corpus of reachable chess games.
// Own pawn blockers can occupy a back rank. Their purpose is to exercise every
// live mobility bin and its integer arithmetic, including sparse high bins.
func mobilityCoverageFEN(piece byte, count int) string {
	var board [64]byte
	board[8], board[23], board[27] = 'K', 'k', piece // Ka2, Kh3, piece d4
	if piece == 'N' {
		for attacks := KnightAttacks[27]; attacks != 0; attacks &= attacks - 1 {
			sq := bits.TrailingZeros64(attacks)
			if count > 0 {
				count--
			} else {
				board[sq] = 'P'
			}
		}
	} else {
		var dirs [][2]int
		if piece == 'B' || piece == 'Q' {
			dirs = append(dirs, [2]int{1, 1}, [2]int{1, -1}, [2]int{-1, 1}, [2]int{-1, -1})
		}
		if piece == 'R' || piece == 'Q' {
			dirs = append(dirs, [2]int{1, 0}, [2]int{-1, 0}, [2]int{0, 1}, [2]int{0, -1})
		}
		for _, d := range dirs {
			for f, r := 3+d[0], 3+d[1]; f >= 0 && f < 8 && r >= 0 && r < 8; f, r = f+d[0], r+d[1] {
				if count == 0 {
					board[r*8+f] = 'P'
					break
				}
				count--
			}
		}
	}
	var fen strings.Builder
	for rank := 7; rank >= 0; rank-- {
		empty := 0
		for file := 0; file < 8; file++ {
			if p := board[rank*8+file]; p != 0 {
				if empty != 0 {
					fen.WriteString(strconv.Itoa(empty))
					empty = 0
				}
				fen.WriteByte(p)
			} else {
				empty++
			}
		}
		if empty != 0 {
			fen.WriteString(strconv.Itoa(empty))
		}
		if rank != 0 {
			fen.WriteByte('/')
		}
	}
	return fen.String() + " w - - 0 1"
}

func TestTexelV2EveryAppendedParameterAffectsLiveEvaluation(t *testing.T) {
	ptrs := TexelFullParams()
	seen := make([]bool, texelNumParams-texelLegacyParams)
	check := func(fen string, index int) {
		t.Helper()
		pos, err := ParseFEN(fen)
		if err != nil {
			t.Fatal(err)
		}
		trace := buildEvalTrace(&pos.Board)
		before := evaluateUnsafe(&pos.Board)
		if got := reconstructEvalInt(&trace); got != before {
			t.Fatalf("initial parity: %d != %d: %s", got, before, fen)
		}
		saved := *ptrs[index]
		*ptrs[index] += 240
		got, want := reconstructEvalInt(&trace), evaluateUnsafe(&pos.Board)
		*ptrs[index] = saved
		if got != want || want == before {
			t.Fatalf("parameter %d (%s) not live or not faithful: before=%d trace=%d live=%d FEN=%s", index, TexelParameterNames()[index], before, got, want, fen)
		}
		seen[index-texelLegacyParams] = true
	}
	offset := 0
	for _, c := range []struct {
		piece byte
		bins  int
	}{{'N', 9}, {'B', 14}, {'R', 15}, {'Q', 28}} {
		for count := 0; count < c.bins; count++ {
			fen := mobilityCoverageFEN(c.piece, count)
			check(fen, texelMobilityMGBase+offset+count)
			check(fen, texelMobilityEGBase+offset+count)
		}
		offset += c.bins
	}
	for i, fen := range []string{
		"7k/8/8/3n4/4P3/8/8/K7 w - - 0 1",
		"7k/8/8/8/3r4/1N6/8/K7 w - - 0 1",
		"7k/8/q7/8/8/8/R7/7K w - - 0 1",
		"7k/8/8/3n4/8/8/8/K2Q4 w - - 0 1",
		"7k/8/8/3r4/8/8/8/K2Q4 w - - 0 1",
		"7k/8/8/5q2/3N4/8/8/K7 w - - 0 1",
	} {
		check(fen, texelThreatBase+i)
	}
	for i, active := range seen {
		if !active {
			t.Fatalf("appended parameter %d was not exercised", texelLegacyParams+i)
		}
	}
	t.Logf("all %d appended parameters changed the live evaluation and matched frozen-trace reconstruction", len(seen))
}

func TestTexelV2ImportInvalidatesCachedEvaluation(t *testing.T) {
	saved := ExportTexelModel()
	t.Cleanup(func() {
		if err := ApplyTexelModel(saved); err != nil {
			t.Error(err)
		}
	})
	fen := mobilityCoverageFEN('Q', 27)
	pos, err := ParseFEN(fen)
	if err != nil {
		t.Fatal(err)
	}
	before := EvaluateForPlayerCached(pos)
	model := ExportTexelModel()
	model.Values[texelMobilityEGBase+mobilityCellCount-1] += 240
	if err := ApplyTexelModel(model); err != nil {
		t.Fatal(err)
	}
	// A fresh position remains numerically identical to the default cached wrapper.
	pos, err = ParseFEN(fen)
	if err != nil {
		t.Fatal(err)
	}
	got, want := EvaluateForPlayerCached(pos), EvaluateForPlayer(&pos.Board, pos.Turn())
	if got != want || got == before {
		t.Fatalf("cached pre-import value survived: before=%d got=%d want=%d", before, got, want)
	}
}
