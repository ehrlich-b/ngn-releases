package engine

import "math/bits"

// Zero-allocation bitboard iteration - replaces getSetBits()
// Usage: for sq := nextSetBit(&bb); sq != 64; sq = nextSetBit(&bb) { ... }
func nextSetBit(bb *uint64) int {
	if *bb == 0 {
		return 64 // No more bits
	}
	bit := bits.TrailingZeros64(*bb)
	*bb &= *bb - 1 // Clear the lowest set bit
	return bit
}

// Fast bitboard population count - replaces len(getSetBits())
func popCount(bb uint64) int {
	return bits.OnesCount64(bb)
}

// Optimized evaluation helper - iterate without allocations
func iterateBitboard(bb uint64, fn func(square int)) {
	for bb != 0 {
		bit := bits.TrailingZeros64(bb)
		fn(bit)
		bb &= bb - 1 // Clear the lowest set bit
	}
}
