package engine

import "math/bits"

var passedPawnFrontSpan [2][64]uint64

func init() {
	for color := Black; color <= White; color++ {
		for origin := 0; origin < 64; origin++ {
			for target := 0; target < 64; target++ {
				distance := target%8 - origin%8
				if distance < -1 || distance > 1 {
					continue
				}
				ahead := target/8 < origin/8
				if color == White {
					ahead = target/8 > origin/8
				}
				if ahead {
					passedPawnFrontSpan[color][origin] |= uint64(1) << target
				}
			}
		}
	}
}

func countPassedPawnsFast(own, enemy uint64, color Color) int {
	count := 0
	for own != 0 {
		square := bits.TrailingZeros64(own)
		if passedPawnFrontSpan[color][square]&enemy == 0 {
			count++
		}
		own &= own - 1
	}
	return count
}

func countPassedPawns(board *Bitboard, color Color) int {
	own := board.GetBitboardOf(GetPiece(Pawn, color))
	enemy := board.GetBitboardOf(GetPiece(Pawn, Color(1-int(color))))
	count := 0
	for square := 0; square < 64; square++ {
		if own&(uint64(1)<<square) == 0 {
			continue
		}
		blocked := false
		for other := 0; other < 64; other++ {
			if enemy&(uint64(1)<<other) == 0 {
				continue
			}
			distance := other%8 - square%8
			ahead := other/8 < square/8
			if color == White {
				ahead = other/8 > square/8
			}
			if distance >= -1 && distance <= 1 && ahead {
				blocked = true
				break
			}
		}
		if !blocked {
			count++
		}
	}
	return count
}

func countDoubledPawns(board *Bitboard, color Color, file int) int {
	if file < 0 || file >= 8 {
		return 0
	}
	pawns := board.GetBitboardOf(GetPiece(Pawn, color))
	count := bits.OnesCount64(pawns & (uint64(0x0101010101010101) << file))
	if count > 1 {
		return count - 1
	}
	return 0
}

func countIsolatedPawns(board *Bitboard, color Color) int {
	pawns := board.GetBitboardOf(GetPiece(Pawn, color))
	count := 0
	for square := 0; square < 64; square++ {
		if pawns&(uint64(1)<<square) == 0 {
			continue
		}
		file := square % 8
		var neighbours uint64
		if file > 0 {
			neighbours |= uint64(0x0101010101010101) << (file - 1)
		}
		if file < 7 {
			neighbours |= uint64(0x0101010101010101) << (file + 1)
		}
		if pawns&neighbours == 0 {
			count++
		}
	}
	return count
}
