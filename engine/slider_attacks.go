package engine

import (
	"github.com/ehrlich-b/ngn/internal/sliderray"
	"math/bits"
)

// A square's interior blockers select one row in an NGN-generated attack table.
// uint64 multiplication deliberately wraps; its high bits form the table index.
type sliderLookup struct {
	mask, multiplier uint64
	offset           uint32
	shift            uint8
}

var rookSliders [64]sliderLookup
var bishopSliders [64]sliderLookup
var rookAttackTable [102400]uint64
var bishopAttackTable [5248]uint64

func init() {
	buildSliderTables(false, rookMagicMultipliers, &rookSliders, rookAttackTable[:])
	buildSliderTables(true, bishopMagicMultipliers, &bishopSliders, bishopAttackTable[:])
}

// Construction verifies every relevant occupancy against coordinate rays.
// A destructive collision in a committed multiplier fails at startup.
func buildSliderTables(diagonal bool, multipliers [64]uint64, lookups *[64]sliderLookup, table []uint64) {
	var offset uint32
	for square, multiplier := range multipliers {
		mask := sliderray.Mask(square, diagonal)
		width := bits.OnesCount64(mask)
		entry := sliderLookup{mask: mask, multiplier: multiplier, offset: offset, shift: uint8(64 - width)}
		lookups[square] = entry
		blockers := uint64(0)
		for {
			attacks := sliderray.Rook(square, blockers)
			if diagonal {
				attacks = sliderray.Bishop(square, blockers)
			}
			index := offset + uint32((blockers*multiplier)>>entry.shift)
			if table[index] != 0 && table[index] != attacks {
				panic("NGN slider multiplier has a destructive collision")
			}
			table[index] = attacks
			blockers = (blockers - mask) & mask
			if blockers == 0 {
				break
			}
		}
		offset += uint32(1) << width
	}
	if int(offset) != len(table) {
		panic("NGN slider table size mismatch")
	}
}

func trailingZeros(bb uint64) int                     { return bits.TrailingZeros64(bb) }
func GetRookAttacks(sq int, occupied uint64) uint64   { return getRookAttacksBB(sq, occupied) }
func GetBishopAttacks(sq int, occupied uint64) uint64 { return getBishopAttacksBB(sq, occupied) }
func GetQueenAttacks(sq int, occupied uint64) uint64 {
	return getRookAttacksBB(sq, occupied) | getBishopAttacksBB(sq, occupied)
}

func getRookAttacksBB(sq int, occupied uint64) uint64 {
	if sq < 0 || sq >= 64 {
		return 0
	}
	entry := &rookSliders[sq]
	index := uint32(((occupied & entry.mask) * entry.multiplier) >> entry.shift)
	return rookAttackTable[entry.offset+index]
}

func getBishopAttacksBB(sq int, occupied uint64) uint64 {
	if sq < 0 || sq >= 64 {
		return 0
	}
	entry := &bishopSliders[sq]
	index := uint32(((occupied & entry.mask) * entry.multiplier) >> entry.shift)
	return bishopAttackTable[entry.offset+index]
}
