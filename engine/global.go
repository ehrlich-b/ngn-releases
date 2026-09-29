package engine

const MaximumDepth = 100

// Precomputed attack tables
var KnightAttacks [64]uint64
var KingAttacks [64]uint64
var RankMasks [8]uint64
var FileMasks [8]uint64

// Pawn attack tables: squares from which pawns of each color can attack a given square
// WhitePawnAttackers[sq] = bitboard of squares where white pawns attack sq
// BlackPawnAttackers[sq] = bitboard of squares where black pawns attack sq
var WhitePawnAttackers [64]uint64
var BlackPawnAttackers [64]uint64

// Ray masks for sliding piece attacks - [square][direction]
// Directions: 0=N, 1=NE, 2=E, 3=SE, 4=S, 5=SW, 6=W, 7=NW
var RayMasks [64][8]uint64

func init() {
	initKnightAttacks()
	initKingAttacks()
	initPawnAttackers()
	initRankMasks()
	initFileMasks()
	initRayMasks()
}

// initKnightAttacks precomputes knight attack bitboards for all squares
func initKnightAttacks() {
	knightMoves := []int{-17, -15, -10, -6, 6, 10, 15, 17}

	for sq := 0; sq < 64; sq++ {
		attacks := uint64(0)
		file := sq % 8
		rank := sq / 8

		for _, move := range knightMoves {
			to := sq + move
			if to < 0 || to > 63 {
				continue
			}

			toFile := to % 8
			toRank := to / 8

			// Check if move is valid (not wrapping around board)
			fileDiff := abs(file - toFile)
			rankDiff := abs(rank - toRank)

			if (fileDiff == 2 && rankDiff == 1) || (fileDiff == 1 && rankDiff == 2) {
				attacks |= SquareMask[to]
			}
		}

		KnightAttacks[sq] = attacks
	}
}

// initKingAttacks precomputes king attack bitboards for all squares
func initKingAttacks() {
	kingMoves := []int{-9, -8, -7, -1, 1, 7, 8, 9}

	for sq := 0; sq < 64; sq++ {
		attacks := uint64(0)
		file := sq % 8

		for _, move := range kingMoves {
			to := sq + move
			if to < 0 || to > 63 {
				continue
			}

			toFile := to % 8

			// Check if move wraps around the board horizontally
			if abs(file-toFile) > 1 {
				continue
			}

			attacks |= SquareMask[to]
		}

		KingAttacks[sq] = attacks
	}
}

// initPawnAttackers precomputes squares from which pawns can attack each square
// This allows O(1) lookup instead of iterating through all pawns
func initPawnAttackers() {
	for sq := 0; sq < 64; sq++ {
		file := sq % 8
		rank := sq / 8

		// White pawns attack from one rank below (rank-1), adjacent files
		// A white pawn on sq-7 attacks sq (if sq is not on file A)
		// A white pawn on sq-9 attacks sq (if sq is not on file H)
		whiteMask := uint64(0)
		if rank > 0 { // Can be attacked by white pawn
			if file > 0 && sq-9 >= 0 { // Not on file A - can be attacked from left-below
				whiteMask |= SquareMask[sq-9]
			}
			if file < 7 && sq-7 >= 0 { // Not on file H - can be attacked from right-below
				whiteMask |= SquareMask[sq-7]
			}
		}
		WhitePawnAttackers[sq] = whiteMask

		// Black pawns attack from one rank above (rank+1), adjacent files
		// A black pawn on sq+7 attacks sq (if sq is not on file H)
		// A black pawn on sq+9 attacks sq (if sq is not on file A)
		blackMask := uint64(0)
		if rank < 7 { // Can be attacked by black pawn
			if file > 0 && sq+7 < 64 { // sq+7 = (rank+1, file-1): black pawn from left-above (needs not file A)
				blackMask |= SquareMask[sq+7]
			}
			if file < 7 && sq+9 < 64 { // sq+9 = (rank+1, file+1): black pawn from right-above (needs not file H)
				blackMask |= SquareMask[sq+9]
			}
		}
		BlackPawnAttackers[sq] = blackMask
	}
}

// initRankMasks precomputes rank bitboards
func initRankMasks() {
	for rank := 0; rank < 8; rank++ {
		mask := uint64(0)
		for file := 0; file < 8; file++ {
			sq := rank*8 + file
			mask |= SquareMask[sq]
		}
		RankMasks[rank] = mask
	}
}

// initFileMasks precomputes file bitboards
func initFileMasks() {
	for file := 0; file < 8; file++ {
		mask := uint64(0)
		for rank := 0; rank < 8; rank++ {
			sq := rank*8 + file
			mask |= SquareMask[sq]
		}
		FileMasks[file] = mask
	}
}

// initRayMasks precomputes ray bitboards for sliding attacks
func initRayMasks() {
	// Direction offsets: N, NE, E, SE, S, SW, W, NW
	directions := []int{-8, -7, 1, 9, 8, 7, -1, -9}

	for sq := 0; sq < 64; sq++ {
		file := sq % 8
		rank := sq / 8

		for dir := 0; dir < 8; dir++ {
			mask := uint64(0)
			offset := directions[dir]

			for i := 1; i < 8; i++ {
				to := sq + offset*i

				if to < 0 || to > 63 {
					break
				}

				toFile := to % 8
				toRank := to / 8

				// Check for board wrap-around
				fileDiff := abs(file - toFile)
				rankDiff := abs(rank - toRank)

				// For diagonal moves, file and rank differences must be equal
				if dir == 1 || dir == 3 || dir == 5 || dir == 7 { // diagonal
					if fileDiff != rankDiff {
						break
					}
				}
				// For horizontal moves, rank must stay same
				if dir == 2 || dir == 6 { // horizontal
					if rankDiff != 0 || fileDiff != i {
						break
					}
				}
				// For vertical moves, file must stay same
				if dir == 0 || dir == 4 { // vertical
					if fileDiff != 0 {
						break
					}
				}

				mask |= SquareMask[to]
			}

			RayMasks[sq][dir] = mask
		}
	}
}
