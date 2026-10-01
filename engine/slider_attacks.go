package engine

import "math/bits"

// trailingZeros returns the index of the least-significant set bit.  An
// empty bitboard has the conventional one-past-the-end result, 64.
func trailingZeros(bb uint64) int {
	return bits.TrailingZeros64(bb)
}

// GetRookAttacks returns the squares visible from sq along the four
// orthogonal rays.  A blocker belongs to the attack set, but squares beyond
// it do not.  The origin is never inspected as an occupant.
func GetRookAttacks(sq int, occupied uint64) uint64 {
	return getRookAttacksBB(sq, occupied)
}

// GetBishopAttacks returns the squares visible from sq along the four
// diagonal rays.
func GetBishopAttacks(sq int, occupied uint64) uint64 {
	return getBishopAttacksBB(sq, occupied)
}

// GetQueenAttacks is the union of the rook and bishop rays.
func GetQueenAttacks(sq int, occupied uint64) uint64 {
	return getRookAttacksBB(sq, occupied) | getBishopAttacksBB(sq, occupied)
}

// getRookAttacksBB is the bitboard-named form kept for callers that use the
// lower-level attack helper.
func getRookAttacksBB(sq int, occupied uint64) uint64 {
	if sq < 0 || sq >= 64 {
		return 0
	}

	file := sq & 7
	rank := sq >> 3
	var attacks uint64

	for nextRank := rank + 1; nextRank < 8; nextRank++ {
		next := nextRank*8 + file
		mask := uint64(1) << uint(next)
		attacks |= mask
		if occupied&mask != 0 {
			break
		}
	}
	for nextRank := rank - 1; nextRank >= 0; nextRank-- {
		next := nextRank*8 + file
		mask := uint64(1) << uint(next)
		attacks |= mask
		if occupied&mask != 0 {
			break
		}
	}
	for nextFile := file + 1; nextFile < 8; nextFile++ {
		next := rank*8 + nextFile
		mask := uint64(1) << uint(next)
		attacks |= mask
		if occupied&mask != 0 {
			break
		}
	}
	for nextFile := file - 1; nextFile >= 0; nextFile-- {
		next := rank*8 + nextFile
		mask := uint64(1) << uint(next)
		attacks |= mask
		if occupied&mask != 0 {
			break
		}
	}

	return attacks
}

// getBishopAttacksBB is the direct coordinate walk for diagonal rays.
func getBishopAttacksBB(sq int, occupied uint64) uint64 {
	if sq < 0 || sq >= 64 {
		return 0
	}

	file := sq & 7
	rank := sq >> 3
	var attacks uint64

	for nextFile, nextRank := file+1, rank+1; nextFile < 8 && nextRank < 8; nextFile, nextRank = nextFile+1, nextRank+1 {
		next := nextRank*8 + nextFile
		mask := uint64(1) << uint(next)
		attacks |= mask
		if occupied&mask != 0 {
			break
		}
	}
	for nextFile, nextRank := file-1, rank+1; nextFile >= 0 && nextRank < 8; nextFile, nextRank = nextFile-1, nextRank+1 {
		next := nextRank*8 + nextFile
		mask := uint64(1) << uint(next)
		attacks |= mask
		if occupied&mask != 0 {
			break
		}
	}
	for nextFile, nextRank := file+1, rank-1; nextFile < 8 && nextRank >= 0; nextFile, nextRank = nextFile+1, nextRank-1 {
		next := nextRank*8 + nextFile
		mask := uint64(1) << uint(next)
		attacks |= mask
		if occupied&mask != 0 {
			break
		}
	}
	for nextFile, nextRank := file-1, rank-1; nextFile >= 0 && nextRank >= 0; nextFile, nextRank = nextFile-1, nextRank-1 {
		next := nextRank*8 + nextFile
		mask := uint64(1) << uint(next)
		attacks |= mask
		if occupied&mask != 0 {
			break
		}
	}

	return attacks
}
