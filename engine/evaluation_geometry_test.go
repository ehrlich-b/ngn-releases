package engine

import (
	"math/rand"
	"testing"
)

// Enumerating square pairs independently checks edge files, extreme ranks,
// both directions, and the distinction between a passed span and a shield.
func TestEvaluationPawnGeometryAgainstCoordinates(t *testing.T) {
	for color := 0; color < 2; color++ {
		for from := 0; from < 64; from++ {
			for to := 0; to < 64; to++ {
				df, dr := (to&7)-(from&7), (to>>3)-(from>>3)
				if color == int(Black) {
					dr = -dr
				}
				forward := df >= -1 && df <= 1 && dr > 0
				shield := forward && dr <= 2
				mask := uint64(1) << uint(to)
				if (pawnForwardArea[color][from]&mask != 0) != forward || (kingShieldArea[color][from]&mask != 0) != shield {
					t.Fatalf("geometry color=%d from=%d to=%d", color, from, to)
				}
			}
		}
	}
}

func TestEvaluationDynamicTermsAgainstCoordinates(t *testing.T) {
	rng := rand.New(rand.NewSource(20261005))
	for trial := 0; trial < 3000; trial++ {
		var pawns, kings [2]uint64
		var files [2][8]int
		for sq := 0; sq < 64; sq++ {
			switch rng.Intn(8) {
			case 0, 1:
				color := rng.Intn(2)
				pawns[color] |= uint64(1) << uint(sq)
				files[color][sq&7]++
			case 2:
				kings[rng.Intn(2)] |= uint64(1) << uint(sq)
			}
		}
		mg, eg, wantMG, wantEG := 0, 0, 0, 0
		addPawnTerms(&mg, &eg, pawns, files)
		addKingTerms(&mg, &eg, pawns, kings)
		for color := 0; color < 2; color++ {
			sign, step := -1, -1
			if color == int(White) {
				sign, step = 1, 1
			}
			for from := 0; from < 64; from++ {
				if pawns[color]&(uint64(1)<<uint(from)) != 0 {
					file, rank := from&7, from>>3
					adjacent := (file > 0 && files[color][file-1] > 0) || (file < 7 && files[color][file+1] > 0)
					if !adjacent {
						wantMG -= sign * 6
						wantEG -= sign * 4
					}
					wantMG -= sign * 4 * (files[color][file] - 1)
					wantEG -= sign * 3 * (files[color][file] - 1)
					passed := true
					for to := 0; to < 64; to++ {
						df, dr := (to&7)-file, ((to>>3)-rank)*step
						if df >= -1 && df <= 1 && dr > 0 && pawns[1-color]&(uint64(1)<<uint(to)) != 0 {
							passed = false
						}
						if (df == -1 || df == 1) && dr == -1 && pawns[color]&(uint64(1)<<uint(to)) != 0 {
							wantMG += sign * 3
							wantEG += sign * 4
						}
					}
					if passed {
						forward := rank
						if color == int(Black) {
							forward = 7 - rank
						}
						wantMG += sign * (4 + 2*forward)
						wantEG += sign * (8 + 3*forward)
					}
				}
				if kings[color]&(uint64(1)<<uint(from)) != 0 {
					for to := 0; to < 64; to++ {
						df, dr := (to&7)-(from&7), ((to>>3)-(from>>3))*step
						if df >= -1 && df <= 1 && (dr == 1 || dr == 2) && pawns[color]&(uint64(1)<<uint(to)) != 0 {
							wantMG += sign * 7
							wantEG += sign * 2
						}
					}
				}
			}
		}
		if mg != wantMG || eg != wantEG {
			t.Fatalf("trial %d terms=%d/%d want=%d/%d", trial, mg, eg, wantMG, wantEG)
		}
	}
}
