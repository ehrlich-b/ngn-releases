package engine

import (
	"github.com/ehrlich-b/ngn/internal/sliderray"
	"testing"
)

func TestMagicEveryRelevantOccupancy(t *testing.T) {
	count := 0
	for square := 0; square < 64; square++ {
		for _, diagonal := range []bool{false, true} {
			directions := oracleRookDirections
			lookup, fast, ray := rookSliders[square], GetRookAttacks, sliderray.Rook
			if diagonal {
				directions = oracleBishopDirections
				lookup, fast, ray = bishopSliders[square], GetBishopAttacks, sliderray.Bishop
			}
			// A relevant bit changes the empty-board oracle's attack set.
			// This derives the mask independently from the runtime geometry.
			var mask uint64
			empty := oracleSlider(square, 0, directions)
			for bit := 0; bit < 64; bit++ {
				occupancy := uint64(1) << bit
				if oracleSlider(square, occupancy, directions) != empty {
					mask |= occupancy
				}
			}
			if lookup.mask != mask {
				t.Fatalf("square=%d diagonal=%v mask=%x want=%x", square, diagonal, lookup.mask, mask)
			}
			subset := uint64(0)
			for {
				for _, occupied := range []uint64{subset, subset | ^mask} {
					want := oracleSlider(square, occupied, directions)
					if fast(square, occupied) != want || ray(square, occupied) != want {
						t.Fatalf("square=%d diagonal=%v occupancy=%x", square, diagonal, occupied)
					}
				}
				count++
				subset = (subset - mask) & mask
				if subset == 0 {
					break
				}
			}
		}
	}
	if count != 107648 {
		t.Fatalf("tested %d subsets, want 107648", count)
	}
	t.Logf("verified %d relevant subsets with irrelevant bits both absent and present", count)
}
