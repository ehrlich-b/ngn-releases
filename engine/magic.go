package engine

// Magic bitboards for O(1) sliding piece attack generation
// Instead of looping through each direction, we use a precomputed table lookup

// Magic numbers - these specific values produce perfect hashing for occupancy -> attacks
// These are well-known magic numbers used by many chess engines

var RookMagics = [64]uint64{
	0x0080001020400080, 0x0040001000200040, 0x0080081000200080, 0x0080040800100080,
	0x0080020400080080, 0x0080010200040080, 0x0080008001000200, 0x0080002040800100,
	0x0000800020400080, 0x0000400020005000, 0x0000801000200080, 0x0000800800100080,
	0x0000800400080080, 0x0000800200040080, 0x0000800100020080, 0x0000800040800100,
	0x0000208000400080, 0x0000404000201000, 0x0000808010002000, 0x0000808008001000,
	0x0000808004000800, 0x0000808002000400, 0x0000010100020004, 0x0000020000408104,
	0x0000208080004000, 0x0000200040005000, 0x0000100080200080, 0x0000080080100080,
	0x0000040080080080, 0x0000020080040080, 0x0000010080800200, 0x0000800080004100,
	0x0000204000800080, 0x0000200040401000, 0x0000100080802000, 0x0000080080801000,
	0x0000040080800800, 0x0000020080800400, 0x0000020001010004, 0x0000800040800100,
	0x0000204000808000, 0x0000200040008080, 0x0000100020008080, 0x0000080010008080,
	0x0000040008008080, 0x0000020004008080, 0x0000010002008080, 0x0000004081020004,
	0x0000204000800080, 0x0000200040008080, 0x0000100020008080, 0x0000080010008080,
	0x0000040008008080, 0x0000020004008080, 0x0000800100020080, 0x0000800041000080,
	0x00FFFCDDFCED714A, 0x007FFCDDFCED714A, 0x003FFFCDFFD88096, 0x0000040810002101,
	0x0001000204080011, 0x0001000204000801, 0x0001000082000401, 0x0001FFFAABFAD1A2,
}

var BishopMagics = [64]uint64{
	0x0002020202020200, 0x0002020202020000, 0x0004010202000000, 0x0004040080000000,
	0x0001104000000000, 0x0000821040000000, 0x0000410410400000, 0x0000104104104000,
	0x0000040404040400, 0x0000020202020200, 0x0000040102020000, 0x0000040400800000,
	0x0000011040000000, 0x0000008210400000, 0x0000004104104000, 0x0000002082082000,
	0x0004000808080800, 0x0002000404040400, 0x0001000202020200, 0x0000800802004000,
	0x0000800400A00000, 0x0000200100884000, 0x0000400082082000, 0x0000200041041000,
	0x0002080010101000, 0x0001040008080800, 0x0000208004010400, 0x0000404004010200,
	0x0000840000802000, 0x0000404002011000, 0x0000808001041000, 0x0000404000820800,
	0x0001041000202000, 0x0000820800101000, 0x0000104400080800, 0x0000020080080080,
	0x0000404040040100, 0x0000808100020100, 0x0001010100020800, 0x0000808080010400,
	0x0000820820004000, 0x0000410410002000, 0x0000082088001000, 0x0000002011000800,
	0x0000080100400400, 0x0001010101000200, 0x0002020202000400, 0x0001010101000200,
	0x0000410410400000, 0x0000208208200000, 0x0000002084100000, 0x0000000020880000,
	0x0000001002020000, 0x0000040408020000, 0x0004040404040000, 0x0002020202020000,
	0x0000104104104000, 0x0000002082082000, 0x0000000020841000, 0x0000000000208800,
	0x0000000010020200, 0x0000000404080200, 0x0000040404040400, 0x0002020202020200,
}

// Shift amounts for indexing into attack tables
var RookShifts = [64]int{
	52, 53, 53, 53, 53, 53, 53, 52,
	53, 54, 54, 54, 54, 54, 54, 53,
	53, 54, 54, 54, 54, 54, 54, 53,
	53, 54, 54, 54, 54, 54, 54, 53,
	53, 54, 54, 54, 54, 54, 54, 53,
	53, 54, 54, 54, 54, 54, 54, 53,
	53, 54, 54, 54, 54, 54, 54, 53,
	52, 53, 53, 53, 53, 53, 53, 52,
}

var BishopShifts = [64]int{
	58, 59, 59, 59, 59, 59, 59, 58,
	59, 59, 59, 59, 59, 59, 59, 59,
	59, 59, 57, 57, 57, 57, 59, 59,
	59, 59, 57, 55, 55, 57, 59, 59,
	59, 59, 57, 55, 55, 57, 59, 59,
	59, 59, 57, 57, 57, 57, 59, 59,
	59, 59, 59, 59, 59, 59, 59, 59,
	58, 59, 59, 59, 59, 59, 59, 58,
}

// Masks for relevant occupancy bits (excludes edges for sliding pieces)
var RookMasks [64]uint64
var BishopMasks [64]uint64

// Attack tables - indexed by [square][magic_index]. Fixed arrays (not slices) so a
// lookup is a single indexed load with no slice-header deref and no dynamic bounds
// check — matches Blunder's [64][4096]/[64][512] layout. Sizes = 2^(max relevant
// occupancy bits): rook 12, bishop 9. ~2.3MB static, the hottest tables in the engine.
var RookAttacks [64][4096]uint64
var BishopAttacks [64][512]uint64

func init() {
	initMagicBitboards()
}

func initMagicBitboards() {
	// Initialize masks
	for sq := 0; sq < 64; sq++ {
		RookMasks[sq] = rookMask(sq)
		BishopMasks[sq] = bishopMask(sq)
	}

	// Initialize attack tables
	for sq := 0; sq < 64; sq++ {
		// Rook attacks (fixed [4096] array — indices 2^bits..4095 stay zero, never read)
		for occ := uint64(0); ; {
			idx := (occ * RookMagics[sq]) >> RookShifts[sq]
			RookAttacks[sq][idx] = rookAttacksSlow(sq, occ)
			occ = (occ - RookMasks[sq]) & RookMasks[sq]
			if occ == 0 {
				break
			}
		}

		// Bishop attacks (fixed [512] array — indices 2^bits..511 stay zero, never read)
		for occ := uint64(0); ; {
			idx := (occ * BishopMagics[sq]) >> BishopShifts[sq]
			BishopAttacks[sq][idx] = bishopAttacksSlow(sq, occ)
			occ = (occ - BishopMasks[sq]) & BishopMasks[sq]
			if occ == 0 {
				break
			}
		}
	}
}

// rookMask generates the relevant occupancy mask for a rook on sq
// Excludes edge squares since they don't affect attack generation
func rookMask(sq int) uint64 {
	result := uint64(0)
	rank := sq / 8
	file := sq % 8

	// North (excluding edge)
	for r := rank + 1; r < 7; r++ {
		result |= 1 << (r*8 + file)
	}
	// South (excluding edge)
	for r := rank - 1; r > 0; r-- {
		result |= 1 << (r*8 + file)
	}
	// East (excluding edge)
	for f := file + 1; f < 7; f++ {
		result |= 1 << (rank*8 + f)
	}
	// West (excluding edge)
	for f := file - 1; f > 0; f-- {
		result |= 1 << (rank*8 + f)
	}
	return result
}

// bishopMask generates the relevant occupancy mask for a bishop on sq
func bishopMask(sq int) uint64 {
	result := uint64(0)
	rank := sq / 8
	file := sq % 8

	// NE
	for r, f := rank+1, file+1; r < 7 && f < 7; r, f = r+1, f+1 {
		result |= 1 << (r*8 + f)
	}
	// NW
	for r, f := rank+1, file-1; r < 7 && f > 0; r, f = r+1, f-1 {
		result |= 1 << (r*8 + f)
	}
	// SE
	for r, f := rank-1, file+1; r > 0 && f < 7; r, f = r-1, f+1 {
		result |= 1 << (r*8 + f)
	}
	// SW
	for r, f := rank-1, file-1; r > 0 && f > 0; r, f = r-1, f-1 {
		result |= 1 << (r*8 + f)
	}
	return result
}

// rookAttacksSlow computes rook attacks given occupancy (used for table init)
func rookAttacksSlow(sq int, occ uint64) uint64 {
	result := uint64(0)
	rank := sq / 8
	file := sq % 8

	// North
	for r := rank + 1; r <= 7; r++ {
		bit := uint64(1) << (r*8 + file)
		result |= bit
		if occ&bit != 0 {
			break
		}
	}
	// South
	for r := rank - 1; r >= 0; r-- {
		bit := uint64(1) << (r*8 + file)
		result |= bit
		if occ&bit != 0 {
			break
		}
	}
	// East
	for f := file + 1; f <= 7; f++ {
		bit := uint64(1) << (rank*8 + f)
		result |= bit
		if occ&bit != 0 {
			break
		}
	}
	// West
	for f := file - 1; f >= 0; f-- {
		bit := uint64(1) << (rank*8 + f)
		result |= bit
		if occ&bit != 0 {
			break
		}
	}
	return result
}

// bishopAttacksSlow computes bishop attacks given occupancy (used for table init)
func bishopAttacksSlow(sq int, occ uint64) uint64 {
	result := uint64(0)
	rank := sq / 8
	file := sq % 8

	// NE
	for r, f := rank+1, file+1; r <= 7 && f <= 7; r, f = r+1, f+1 {
		bit := uint64(1) << (r*8 + f)
		result |= bit
		if occ&bit != 0 {
			break
		}
	}
	// NW
	for r, f := rank+1, file-1; r <= 7 && f >= 0; r, f = r+1, f-1 {
		bit := uint64(1) << (r*8 + f)
		result |= bit
		if occ&bit != 0 {
			break
		}
	}
	// SE
	for r, f := rank-1, file+1; r >= 0 && f <= 7; r, f = r-1, f+1 {
		bit := uint64(1) << (r*8 + f)
		result |= bit
		if occ&bit != 0 {
			break
		}
	}
	// SW
	for r, f := rank-1, file-1; r >= 0 && f >= 0; r, f = r-1, f-1 {
		bit := uint64(1) << (r*8 + f)
		result |= bit
		if occ&bit != 0 {
			break
		}
	}
	return result
}

// GetRookAttacks returns rook attacks using magic bitboards - O(1)
func GetRookAttacks(sq int, occupied uint64) uint64 {
	occ := occupied & RookMasks[sq]
	idx := (occ * RookMagics[sq]) >> RookShifts[sq]
	return RookAttacks[sq][idx&4095] // mask is a no-op (idx < 2^bits <= 4096); elides the bounds check
}

// GetBishopAttacks returns bishop attacks using magic bitboards - O(1)
func GetBishopAttacks(sq int, occupied uint64) uint64 {
	occ := occupied & BishopMasks[sq]
	idx := (occ * BishopMagics[sq]) >> BishopShifts[sq]
	return BishopAttacks[sq][idx&511] // mask is a no-op (idx < 2^bits <= 512); elides the bounds check
}

// GetQueenAttacks returns queen attacks (combination of rook + bishop)
func GetQueenAttacks(sq int, occupied uint64) uint64 {
	return GetRookAttacks(sq, occupied) | GetBishopAttacks(sq, occupied)
}
