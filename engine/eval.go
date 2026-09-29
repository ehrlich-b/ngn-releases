package engine

import (
	"math/bits"
)

// Note: getSetBits function removed - replaced with zero-allocation iteration patterns

// Helper function for trailing zeros
func trailingZeros(bitboard uint64) int {
	return bits.TrailingZeros64(bitboard)
}

// Branchless absolute value - avoids branch prediction penalty
// Uses sign extension: mask is -1 if x < 0, else 0
func evalAbs(x int) int {
	mask := x >> 63
	return (x ^ mask) - mask
}

// Piece-Square Tables for positional evaluation
// Values from white's perspective, mirrored for black

// Pawn piece-square table (encourages central pawns and pawn advancement)
// Enhanced with opening nudges: e4/d4 get +15 bonus
var pawnPST = [64]int{
	0, 0, 0, 0, 0, 0, 0, 0,
	50, 50, 50, 50, 50, 50, 50, 50,
	10, 10, 20, 30, 30, 20, 10, 10,
	5, 5, 10, 25, 25, 10, 5, 5,
	0, 0, 0, 35, 35, 0, 0, 0, // d4/e4 get +15 opening bonus
	5, -5, -10, 0, 0, -10, -5, 5,
	5, 10, 10, -20, -20, 10, 10, 5,
	0, 0, 0, 0, 0, 0, 0, 0,
}

// Knight piece-square table (discourages premature development, encourages proper knight placement)
var knightPST = [64]int{
	-50, -40, -30, -30, -30, -30, -40, -50,
	-40, -20, -10, -10, -10, -10, -20, -40,
	-30, -10, 0, 5, 5, 0, -10, -30,
	-30, -5, 5, 10, 10, 5, -5, -30,
	-30, -10, 5, 10, 10, 5, -10, -30,
	-30, -15, -5, 0, 0, -5, -15, -30, // Early development squares get penalty
	-40, -25, -15, -10, -10, -15, -25, -40, // Stronger penalty for very early development
	-50, -40, -30, -30, -30, -30, -40, -50,
}

// Bishop piece-square table (encourages central placement and long diagonals)
var bishopPST = [64]int{
	-20, -10, -10, -10, -10, -10, -10, -20,
	-10, 0, 0, 0, 0, 0, 0, -10,
	-10, 0, 5, 10, 10, 5, 0, -10,
	-10, 5, 5, 10, 10, 5, 5, -10,
	-10, 0, 10, 10, 10, 10, 0, -10,
	-10, 10, 10, 10, 10, 10, 10, -10,
	-10, 5, 0, 0, 0, 0, 5, -10,
	-20, -10, -10, -10, -10, -10, -10, -20,
}

// Rook piece-square table (encourages central files and 7th rank)
var rookPST = [64]int{
	0, 0, 0, 0, 0, 0, 0, 0,
	5, 10, 10, 10, 10, 10, 10, 5,
	-5, 0, 0, 0, 0, 0, 0, -5,
	-5, 0, 0, 0, 0, 0, 0, -5,
	-5, 0, 0, 0, 0, 0, 0, -5,
	-5, 0, 0, 0, 0, 0, 0, -5,
	-5, 0, 0, 0, 0, 0, 0, -5,
	0, 0, 0, 5, 5, 0, 0, 0,
}

// Queen piece-square table (discourages early queen development)
// Enhanced with opening nudges: early queen squares get penalty
var queenPST = [64]int{
	-20, -10, -10, -5, -5, -10, -10, -20,
	-35, -25, -25, -20, -20, -25, -25, -35, // Queen early development penalty
	-10, 0, 5, 5, 5, 5, 0, -10,
	-5, 0, 5, 5, 5, 5, 0, -5,
	0, 0, 5, 5, 5, 5, 0, -5,
	-10, 5, 5, 5, 5, 5, 0, -10,
	-10, 0, 5, 0, 0, 0, 0, -10,
	-20, -10, -10, -5, -5, -10, -10, -20,
}

// King piece-square table for middlegame (safety first)
var kingMiddlegamePST = [64]int{
	-30, -40, -40, -50, -50, -40, -40, -30,
	-30, -40, -40, -50, -50, -40, -40, -30,
	-30, -40, -40, -50, -50, -40, -40, -30,
	-30, -40, -40, -50, -50, -40, -40, -30,
	-20, -30, -30, -40, -40, -30, -30, -20,
	-10, -20, -20, -20, -20, -20, -20, -10,
	20, 20, 0, 0, 0, 0, 20, 20,
	20, 30, 10, 0, 0, 10, 30, 20,
}

// King piece-square table for endgame (centralization becomes important)
var kingEndgamePST = [64]int{
	-50, -40, -30, -20, -20, -30, -40, -50,
	-30, -20, -10, 0, 0, -10, -20, -30,
	-30, -10, 20, 30, 30, 20, -10, -30,
	-30, -10, 30, 40, 40, 30, -10, -30,
	-30, -10, 30, 40, 40, 30, -10, -30,
	-30, -10, 20, 30, 30, 20, -10, -30,
	-30, -30, 0, 0, 0, 0, -30, -30,
	-50, -30, -30, -30, -30, -30, -30, -50,
}

// Pre-computed PSTs for black pieces (flipped and negated for zero-cost lookups)
var blackPawnPST, blackKnightPST, blackBishopPST, blackRookPST, blackQueenPST, blackKingMiddlegamePST, blackKingEndgamePST [64]int

func init() {
	// Pre-compute all black piece-square tables (flipped and negated)
	for i := 0; i < 64; i++ {
		flippedIndex := i ^ 56 // Flip rank for black pieces
		blackPawnPST[i] = -pawnPST[flippedIndex]
		blackKnightPST[i] = -knightPST[flippedIndex]
		blackBishopPST[i] = -bishopPST[flippedIndex]
		blackRookPST[i] = -rookPST[flippedIndex]
		blackQueenPST[i] = -queenPST[flippedIndex]
		blackKingMiddlegamePST[i] = -kingMiddlegamePST[flippedIndex]
		blackKingEndgamePST[i] = -kingEndgamePST[flippedIndex]
	}
}

// Optimized PST lookup - zero-cost runtime lookups with pre-computed tables
func getPieceSquareValue(piece Piece, square Square, isEndgame bool) int {
	sq := int(square)

	switch piece {
	case WhitePawn:
		return pawnPST[sq]
	case BlackPawn:
		return blackPawnPST[sq]
	case WhiteKnight:
		return knightPST[sq]
	case BlackKnight:
		return blackKnightPST[sq]
	case WhiteBishop:
		return bishopPST[sq]
	case BlackBishop:
		return blackBishopPST[sq]
	case WhiteRook:
		return rookPST[sq]
	case BlackRook:
		return blackRookPST[sq]
	case WhiteQueen:
		return queenPST[sq]
	case BlackQueen:
		return blackQueenPST[sq]
	case WhiteKing:
		if isEndgame {
			return kingEndgamePST[sq]
		} else {
			return kingMiddlegamePST[sq]
		}
	case BlackKing:
		if isEndgame {
			return blackKingEndgamePST[sq]
		} else {
			return blackKingMiddlegamePST[sq]
		}
	}
	return 0
}

// Phase values for tapered evaluation
// Total phase = 24 (4 queens worth 4 each + 4 rooks worth 2 + 4 bishops worth 1 + 4 knights worth 1... per side)
// Actually: 2 sides * (1 queen * 4 + 2 rooks * 2 + 2 bishops * 1 + 2 knights * 1) = 2 * (4 + 4 + 2 + 2) = 24
const (
	queenPhase  = 4
	rookPhase   = 2
	bishopPhase = 1
	knightPhase = 1
	totalPhase  = 24 // Maximum phase (full material)
)

// Helper function to check if we're in the endgame
// Simple heuristic: endgame when both sides have <= 13 points of material (excluding pawns and kings)
func isEndgame(board *Bitboard) bool {
	whiteMaterial := PopCount(board.GetBitboardOf(WhiteQueen))*1000 +
		PopCount(board.GetBitboardOf(WhiteRook))*525 +
		PopCount(board.GetBitboardOf(WhiteBishop))*330 +
		PopCount(board.GetBitboardOf(WhiteKnight))*320

	blackMaterial := PopCount(board.GetBitboardOf(BlackQueen))*1000 +
		PopCount(board.GetBitboardOf(BlackRook))*525 +
		PopCount(board.GetBitboardOf(BlackBishop))*330 +
		PopCount(board.GetBitboardOf(BlackKnight))*320

	return whiteMaterial <= 1300 && blackMaterial <= 1300
}

// taperScore interpolates between middlegame and endgame scores based on game phase
// phase: 0 = pure endgame, 24 = full material
func taperScore(mgScore, egScore, phase int) int {
	// Linear interpolation: (mg * phase + eg * (totalPhase - phase)) / totalPhase
	return (mgScore*phase + egScore*(totalPhase-phase)) / totalPhase
}

// Helper function to count doubled pawns on a file
func countDoubledPawns(board *Bitboard, color Color, file int) int {
	var pawnBitboard uint64
	if color == White {
		pawnBitboard = board.GetBitboardOf(WhitePawn)
	} else {
		pawnBitboard = board.GetBitboardOf(BlackPawn)
	}

	fileMask := uint64(0x0101010101010101) << file // File A to H
	pawnsOnFile := PopCount(pawnBitboard & fileMask)

	if pawnsOnFile > 1 {
		return pawnsOnFile - 1 // Return number of "extra" pawns (doubled)
	}
	return 0
}

// Helper function to count isolated pawns
func countIsolatedPawns(board *Bitboard, color Color) int {
	var pawnBitboard uint64
	if color == White {
		pawnBitboard = board.GetBitboardOf(WhitePawn)
	} else {
		pawnBitboard = board.GetBitboardOf(BlackPawn)
	}

	isolated := 0
	for file := 0; file < 8; file++ {
		fileMask := uint64(0x0101010101010101) << file
		if (pawnBitboard & fileMask) == 0 {
			continue // No pawns on this file
		}

		// Check adjacent files for supporting pawns
		hasSupport := false
		if file > 0 {
			leftFileMask := uint64(0x0101010101010101) << (file - 1)
			if (pawnBitboard & leftFileMask) != 0 {
				hasSupport = true
			}
		}
		if file < 7 {
			rightFileMask := uint64(0x0101010101010101) << (file + 1)
			if (pawnBitboard & rightFileMask) != 0 {
				hasSupport = true
			}
		}

		if !hasSupport {
			isolated += PopCount(pawnBitboard & fileMask)
		}
	}

	return isolated
}

// Helper function to count passed pawns (simplified version)
func countPassedPawns(board *Bitboard, color Color) int {
	var pawnBitboard uint64
	var opponentPawnBitboard uint64

	if color == White {
		pawnBitboard = board.GetBitboardOf(WhitePawn)
		opponentPawnBitboard = board.GetBitboardOf(BlackPawn)
	} else {
		pawnBitboard = board.GetBitboardOf(BlackPawn)
		opponentPawnBitboard = board.GetBitboardOf(WhitePawn)
	}

	passed := 0

	// Zero-allocation pawn iteration
	bb := pawnBitboard
	for sq := nextSetBit(&bb); sq != 64; sq = nextSetBit(&bb) {
		square := sq
		file := int(square % 8)
		rank := int(square / 8)

		// Define the "passed pawn" area based on color
		var blockedByOpponent bool

		// Check if any opponent pawns are in front of this pawn or on adjacent files
		for obb := opponentPawnBitboard; obb != 0; obb &= obb - 1 {
			opponentSquare := bits.TrailingZeros64(obb)
			opponentFile := int(opponentSquare % 8)
			opponentRank := int(opponentSquare / 8)

			// Check if opponent pawn is in the path or adjacent files ahead
			if evalAbs(opponentFile-file) <= 1 {
				if color == White && opponentRank > rank {
					blockedByOpponent = true
					break
				} else if color == Black && opponentRank < rank {
					blockedByOpponent = true
					break
				}
			}
		}

		if !blockedByOpponent {
			passed++
		}
	}

	return passed
}

// passedPawnFrontSpan[color][sq] is the set of squares ahead of `sq` (from
// `color`'s perspective) on its own and adjacent files. A pawn at `sq` is
// passed iff this mask AND the opponent's pawn bitboard is zero.
// Index: 0 = Black, 1 = White (matches the Color enum: Black=0, White=1).
var passedPawnFrontSpan [2][64]uint64

func init() {
	for sq := 0; sq < 64; sq++ {
		rank := sq / 8
		file := sq % 8
		var wMask, bMask uint64
		for r := 0; r < 8; r++ {
			for fOff := -1; fOff <= 1; fOff++ {
				f := file + fOff
				if f < 0 || f > 7 {
					continue
				}
				bit := uint64(1) << uint(r*8+f)
				if r > rank {
					wMask |= bit
				}
				if r < rank {
					bMask |= bit
				}
			}
		}
		passedPawnFrontSpan[White][sq] = wMask
		passedPawnFrontSpan[Black][sq] = bMask
	}
}

// countPassedPawnsFast: pawn is passed iff no opponent pawn lies on its file
// or adjacent files at any rank ahead (from the pawn's perspective).
func countPassedPawnsFast(pawnBitboard, opponentPawnBitboard uint64, color Color) int {
	if pawnBitboard == 0 {
		return 0
	}
	passed := 0
	masks := &passedPawnFrontSpan[color]
	for bb := pawnBitboard; bb != 0; bb &= bb - 1 {
		sq := bits.TrailingZeros64(bb)
		if masks[sq]&opponentPawnBitboard == 0 {
			passed++
		}
	}
	return passed
}

// passedRankMul scales a passed pawn's bonus by how far it has advanced toward
// promotion (index = rank relative to the pawn's own side: 1 = just off the start
// square ... 6 = one push from queening; 0 and 7 are unreachable for a pawn). A
// passer on the 7th rank is far more dangerous than one on the 4th — the flat
// per-passer bonus under-credits advanced passers, the mg->eg-transition blind
// spot in the loss analysis. The MG/EG magnitude split stays in passedPawnBonus(EG);
// this only reshapes by rank. <=5th keep the flat 1x weight, only 6th/7th step up.
var passedRankMul = [8]int{0, 0, 1, 1, 1, 2, 4, 0}

// passedRankWeightedSum sums passedRankMul over the side's passed pawns — the
// rank-weighted replacement for the raw passed-pawn count. Mirrors
// countPassedPawnsFast's passer test exactly so the two agree on which pawns pass.
func passedRankWeightedSum(pawnBitboard, opponentPawnBitboard uint64, color Color) int {
	if pawnBitboard == 0 {
		return 0
	}
	sum := 0
	masks := &passedPawnFrontSpan[color]
	for bb := pawnBitboard; bb != 0; bb &= bb - 1 {
		sq := bits.TrailingZeros64(bb)
		if masks[sq]&opponentPawnBitboard == 0 {
			r := sq >> 3
			if color == Black {
				r = 7 - r
			}
			sum += passedRankMul[r]
		}
	}
	return sum
}

// passersOf returns the bitboard of `color`'s passed pawns (no enemy pawn anywhere
// on the front span). The three per-eval passer terms below (blocked / king-discount
// / rook-behind) all share this single detection instead of each re-testing every
// pawn — node-identical, just computed once per color rather than once per term.
func passersOf(ownPawns, oppPawns uint64, color Color) uint64 {
	if ownPawns == 0 {
		return 0
	}
	masks := &passedPawnFrontSpan[color]
	var passers uint64
	for bb := ownPawns; bb != 0; bb &= bb - 1 {
		sq := bits.TrailingZeros64(bb)
		if masks[sq]&oppPawns == 0 {
			passers |= SquareMask[sq]
		}
	}
	return passers
}

// passedBlockedCount counts, over the side's already-detected passers, those whose
// stop square (one rank ahead) is occupied by an enemy piece. A blockaded passer
// can't advance until the blocker is removed, so it is worth far less than the
// rank-weighted bonus credits — the eval-optimism bias the L1 loss analysis found
// (NGN steers into lost positions over-valuing its own stalled passers). The blocker
// is necessarily a non-pawn (a passer has no enemy pawn ahead on its file), so
// oppNonPawns excludes pawns.
func passedBlockedCount(passers, oppNonPawns uint64, color Color) int {
	count := 0
	for bb := passers; bb != 0; bb &= bb - 1 {
		sq := bits.TrailingZeros64(bb)
		stop := sq + 8
		if color == Black {
			stop = sq - 8
		}
		if stop < 0 || stop > 63 {
			continue
		}
		if oppNonPawns&SquareMask[stop] != 0 {
			count++
		}
	}
	return count
}

// kingPasserMaxDist is the chebyshev (king-move) distance from a passer's stop square
// at or beyond which the enemy king is too far to discount the passer; kingPasserBlockEG
// is the endgame centipawn discount per square the enemy king is INSIDE that radius.
// Non-tunable structural constants (the texel param vector does not include them);
// captured in the trace's Fixed lump so reconstructEvalInt stays bit-exact.
const (
	kingPasserMaxDist  = 5
	kingPasserBlockEG  = 6
	rookBehindPasserEG = 10 // EG discount for a passer with an enemy rook behind it (Tarrasch restraint)
)

// passedKingDiscount sums, over the side's passed pawns, a proximity discount for how
// close the ENEMY king sits to each passer's stop square. A passer the defending king
// can reach is worth far less than the rank-weighted bonus credits — the endgame half
// of the L1 over-optimism (NGN over-values its own passers; an enemy king racing to
// blockade them is exactly when that bias bites). The discount ramps linearly from
// kingPasserMaxDist (enemy king on the stop square) to 0 (king kingPasserMaxDist+ away).
// Returns the raw ramp sum (EG taper applied by the caller); mirrors the
// passedPawnFrontSpan passer test so it agrees with the other passed-pawn terms.
func passedKingDiscount(passers uint64, enemyKingSq int, color Color) int {
	if passers == 0 {
		return 0
	}
	ekFile := enemyKingSq & 7
	ekRank := enemyKingSq >> 3
	discount := 0
	for bb := passers; bb != 0; bb &= bb - 1 {
		sq := bits.TrailingZeros64(bb)
		stop := sq + 8
		if color == Black {
			stop = sq - 8
		}
		if stop < 0 || stop > 63 {
			continue
		}
		df := (stop & 7) - ekFile
		if df < 0 {
			df = -df
		}
		dr := (stop >> 3) - ekRank
		if dr < 0 {
			dr = -dr
		}
		d := df
		if dr > d {
			d = dr // chebyshev = king-move distance
		}
		if d < kingPasserMaxDist {
			discount += kingPasserMaxDist - d
		}
	}
	return discount
}

// passedRookBehindCount counts the side's passed pawns that have an ENEMY rook behind
// them on the same file (the defender's Tarrasch position). A rook behind a passer
// restrains its advance and follows it down the board, so the passer is worth less than
// its rank bonus credits — another slice of the L1 own-passer optimism. "Behind" is
// toward the pawn's own side: lower ranks for White, higher for Black. Mirrors the
// passedPawnFrontSpan passer test so it agrees with the other passed-pawn terms.
func passedRookBehindCount(passers, oppRooks uint64, color Color) int {
	if oppRooks == 0 {
		return 0
	}
	count := 0
	for bb := passers; bb != 0; bb &= bb - 1 {
		sq := bits.TrailingZeros64(bb)
		fileBB := uint64(0x0101010101010101) << (sq & 7)
		var behind uint64
		if color == White {
			behind = fileBB & (SquareMask[sq] - 1) // same-file squares below the pawn
		} else {
			behind = fileBB &^ (SquareMask[sq] | (SquareMask[sq] - 1)) // same-file squares above the pawn
		}
		if oppRooks&behind != 0 {
			count++
		}
	}
	return count
}

// Helper function to evaluate pawn chains (connected pawns that support each other)
func evaluatePawnChains(board *Bitboard, color Color) int {
	var pawnBitboard uint64
	if color == White {
		pawnBitboard = board.GetBitboardOf(WhitePawn)
	} else {
		pawnBitboard = board.GetBitboardOf(BlackPawn)
	}

	if pawnBitboard == 0 {
		return 0
	}

	chainBonus := 0

	for pbb := pawnBitboard; pbb != 0; pbb &= pbb - 1 {
		square := bits.TrailingZeros64(pbb)
		file := int(square % 8)
		rank := int(square / 8)

		// Count supporting pawns (pawns that can defend this pawn)
		supportingPawns := 0

		// Check for supporting pawns on adjacent files, one rank behind
		for fileOffset := -1; fileOffset <= 1; fileOffset += 2 { // -1 and +1
			supportFile := file + fileOffset
			if supportFile < 0 || supportFile > 7 {
				continue
			}

			var supportRank int
			if color == White {
				supportRank = rank - 1 // White pawns advance up, so support comes from behind
			} else {
				supportRank = rank + 1 // Black pawns advance down, so support comes from behind
			}

			if supportRank < 0 || supportRank > 7 {
				continue
			}

			supportSquare := supportRank*8 + supportFile
			if (pawnBitboard & (1 << supportSquare)) != 0 {
				supportingPawns++
			}
		}

		// Bonus for pawns in chains (more support = higher bonus)
		switch supportingPawns {
		case 1:
			chainBonus += 8 // Single support
		case 2:
			chainBonus += 12 // Double support (very strong)
		}

		// Additional bonus for advanced pawns in chains
		var advancement int
		if color == White {
			advancement = rank - 1 // Rank 2 is base rank for white pawns
		} else {
			advancement = 6 - rank // Rank 7 is base rank for black pawns
		}

		if supportingPawns > 0 && advancement > 2 {
			chainBonus += advancement * 2 // Advanced supported pawns get extra bonus
		}
	}

	return chainBonus
}

// Helper function to evaluate weak squares around pawns
// PERFORMANCE: Disabled for Phase 3 optimization - was consuming 5.31% CPU time
func evaluateWeakSquares(board *Bitboard, color Color) int {
	// Fast path - return 0 for performance optimization
	return 0

	// Original complex implementation commented out for performance:
	/*
		var pawnBitboard uint64

		if color == White {
			pawnBitboard = board.GetBitboardOf(WhitePawn)
		} else {
			pawnBitboard = board.GetBitboardOf(BlackPawn)
		}

		if pawnBitboard == 0 {
			return 0 // No pawns, no weak squares to evaluate
		}

		weaknessBonus := 0 // Positive bonus for opponent having weak squares

		// Generate pawn attack coverage (squares controlled by our pawns)
		pawnAttackMask := uint64(0)

		for pbb := pawnBitboard; pbb != 0; pbb &= pbb - 1 {
			square := bits.TrailingZeros64(pbb)
			file := int(square % 8)
			rank := int(square / 8)

			// Generate pawn attacks (diagonal captures)
			var attackRank int
			if color == White {
				attackRank = rank + 1 // White pawns attack forward
			} else {
				attackRank = rank - 1 // Black pawns attack backward
			}

			if attackRank >= 0 && attackRank <= 7 {
				// Left attack
				if file > 0 {
					attackSquare := attackRank*8 + (file - 1)
					pawnAttackMask |= (1 << attackSquare)
				}
				// Right attack
				if file < 7 {
					attackSquare := attackRank*8 + (file + 1)
					pawnAttackMask |= (1 << attackSquare)
				}
			}
		}

		// Check for enemy pieces on weak squares (not controlled by our pawns)
		// Focus on central files (c, d, e, f) and important squares
		centralFiles := []int{2, 3, 4, 5} // Files c, d, e, f

		for _, file := range centralFiles {
			for rank := 2; rank <= 5; rank++ { // Focus on central area
				square := rank*8 + file
				squareMask := uint64(1 << square)

				// If this square is not controlled by our pawns, it's potentially weak
				if (pawnAttackMask & squareMask) == 0 {
					// Check if opponent has pieces that could benefit from this weak square
					// Look for enemy knights, bishops, or queens nearby

					// Simple penalty for uncontrolled central squares
					if rank >= 3 && rank <= 4 { // Ranks 4-5 (0-indexed)
						weaknessBonus -= 3 // Small penalty for weak central squares
					}

					// Additional penalty if opponent has pawns that create holes
					// Check if this square is a "hole" (can't be defended by pawns)
					canBeDefended := false

					// Check if we could potentially put a pawn to defend this square
					for fileOffset := -1; fileOffset <= 1; fileOffset += 2 {
						defendFile := file + fileOffset
						if defendFile < 0 || defendFile > 7 {
							continue
						}

						var defendRank int
						if color == White {
							defendRank = rank - 1 // Would need a pawn behind to defend
						} else {
							defendRank = rank + 1 // Would need a pawn behind to defend
						}

						if defendRank >= 0 && defendRank <= 7 {
							defendSquare := defendRank*8 + defendFile
							if (pawnBitboard & (1 << defendSquare)) != 0 {
								canBeDefended = true
								break
							}
						}
					}

					if !canBeDefended {
						// This is a permanent weak square - bigger penalty
						if rank >= 3 && rank <= 4 {
							weaknessBonus -= 8 // Larger penalty for holes in our position
						}
					}
				}
			}
		}

		return weaknessBonus // This will be negative for our weak squares
	*/
}

// Helper function to evaluate pawn storms
func evaluatePawnStorms(board *Bitboard, color Color) int {
	var pawnBitboard uint64
	var opponentKingBitboard uint64

	if color == White {
		pawnBitboard = board.GetBitboardOf(WhitePawn)
		opponentKingBitboard = board.GetBitboardOf(BlackKing)
	} else {
		pawnBitboard = board.GetBitboardOf(BlackPawn)
		opponentKingBitboard = board.GetBitboardOf(WhiteKing)
	}

	if pawnBitboard == 0 || opponentKingBitboard == 0 {
		return 0
	}

	// Find opponent king position
	opponentKingSquare := trailingZeros(opponentKingBitboard)
	opponentKingFile := int(opponentKingSquare % 8)
	opponentKingRank := int(opponentKingSquare / 8)

	stormBonus := 0

	// Track most advanced pawn per file without allocations
	// Use -1 to indicate no pawn on that file
	var mostAdvancedByFile [8]int
	var pawnCountByFile [8]int

	// Initialize with invalid values
	for i := 0; i < 8; i++ {
		if color == White {
			mostAdvancedByFile[i] = -1 // No pawn yet
		} else {
			mostAdvancedByFile[i] = 8 // No pawn yet (8 is invalid rank)
		}
	}

	// Find most advanced pawn on each file
	for pbb := pawnBitboard; pbb != 0; pbb &= pbb - 1 {
		square := bits.TrailingZeros64(pbb)
		file := square % 8
		rank := square / 8
		pawnCountByFile[file]++

		if color == White {
			if mostAdvancedByFile[file] == -1 || rank > mostAdvancedByFile[file] {
				mostAdvancedByFile[file] = rank
			}
		} else {
			if mostAdvancedByFile[file] == 8 || rank < mostAdvancedByFile[file] {
				mostAdvancedByFile[file] = rank
			}
		}
	}

	// Evaluate storm potential near enemy king
	for fileOffset := -1; fileOffset <= 1; fileOffset++ {
		kingFile := opponentKingFile + fileOffset
		if kingFile < 0 || kingFile > 7 {
			continue
		}

		mostAdvancedRank := mostAdvancedByFile[kingFile]
		pawnCount := pawnCountByFile[kingFile]

		// Skip if no pawns on this file
		if pawnCount == 0 {
			continue
		}

		// Calculate storm advancement
		var advancement int
		if color == White {
			advancement = mostAdvancedRank - 1 // Starting from rank 2
			// Bonus increases as pawn advances toward enemy king
			if mostAdvancedRank >= 4 { // Pawn has crossed the halfway point
				stormBonus += advancement * 4

				// Extra bonus if pawn is close to enemy king rank
				if mostAdvancedRank >= opponentKingRank-2 {
					stormBonus += 10
				}
			}
		} else {
			advancement = 6 - mostAdvancedRank // Starting from rank 7
			if mostAdvancedRank <= 3 {         // Pawn has crossed the halfway point
				stormBonus += advancement * 4

				// Extra bonus if pawn is close to enemy king rank
				if mostAdvancedRank <= opponentKingRank+2 {
					stormBonus += 10
				}
			}
		}

		// Multiple pawns on same file create stronger storms
		if pawnCount > 1 {
			stormBonus += pawnCount * 2 // Bonus for pawn cooperation
		}
	}

	// Additional bonus for connected pawn storms (pawns on adjacent files)
	for file := 0; file < 7; file++ {
		leftCount := pawnCountByFile[file]
		rightCount := pawnCountByFile[file+1]

		if leftCount > 0 && rightCount > 0 {
			// Check if both files have advanced pawns near enemy king
			kingDistance := evalAbs(file-opponentKingFile) + evalAbs((file+1)-opponentKingFile)
			if kingDistance <= 3 { // Within striking distance of king
				stormBonus += 8 // Bonus for connected storm
			}
		}
	}

	return stormBonus
}

// kingSafetyFullAttackMaterial is the enemy attacking-material total (Q=4 R=2
// B=1 N=1) at which the POSITIONAL king-safety terms (pawn shield, central-king
// exposure, open files near the king) apply at full weight; below it they scale
// down linearly toward 0. An exposed king is only weak if the enemy has force to
// exploit it — without it those terms over-credit a "safe" king, the source of a
// measured ~+145cp own-side eval over-optimism in phase 6-11 (queens traded).
const kingSafetyFullAttackMaterial = 8

// Enhanced king safety evaluation with improved pawn shield and attack patterns
func evaluateKingSafety(board *Bitboard, color Color, sliderAtt *[64]uint64) int {
	var kingBitboard uint64
	var pawnBitboard uint64
	var opponentQueenBitboard uint64
	var opponentRookBitboard uint64
	var opponentBishopBitboard uint64
	var opponentKnightBitboard uint64

	if color == White {
		kingBitboard = board.GetBitboardOf(WhiteKing)
		pawnBitboard = board.GetBitboardOf(WhitePawn)
		opponentQueenBitboard = board.GetBitboardOf(BlackQueen)
		opponentRookBitboard = board.GetBitboardOf(BlackRook)
		opponentBishopBitboard = board.GetBitboardOf(BlackBishop)
		opponentKnightBitboard = board.GetBitboardOf(BlackKnight)
	} else {
		kingBitboard = board.GetBitboardOf(BlackKing)
		pawnBitboard = board.GetBitboardOf(BlackPawn)
		opponentQueenBitboard = board.GetBitboardOf(WhiteQueen)
		opponentRookBitboard = board.GetBitboardOf(WhiteRook)
		opponentBishopBitboard = board.GetBitboardOf(WhiteBishop)
		opponentKnightBitboard = board.GetBitboardOf(WhiteKnight)
	}

	if kingBitboard == 0 {
		return 0
	}

	kingSquare := trailingZeros(kingBitboard)
	kingFile := int(kingSquare % 8)
	kingRank := int(kingSquare / 8)

	// Positional ("how exposed is the king") terms — shield, central-king, open
	// files. Accumulated into nonAttack, then scaled by enemy attacking material
	// below: these only matter if the enemy has force to attack the king.
	nonAttack := 0

	// 1. Enhanced Pawn Shield Evaluation
	nonAttack += evaluatePawnShield(pawnBitboard, kingFile, kingRank, color)

	// 3. King position penalty (being in center during middlegame is dangerous)
	if kingFile >= 2 && kingFile <= 5 { // Central files
		nonAttack -= 15 // Penalty for exposed king
	}

	// 4. Open file penalty near king
	whitePawnsAll := board.GetBitboardOf(WhitePawn)
	blackPawnsAll := board.GetBitboardOf(BlackPawn)
	for fileOffset := -1; fileOffset <= 1; fileOffset++ {
		file := kingFile + fileOffset
		if file < 0 || file > 7 {
			continue
		}

		// Check if this file is open (no pawns from either side)
		fileMask := uint64(0x0101010101010101) << file
		whitePawns := whitePawnsAll & fileMask
		blackPawns := blackPawnsAll & fileMask
		if whitePawns == 0 && blackPawns == 0 {
			nonAttack -= 20 // Open file near king is dangerous
		} else if color == White && whitePawns == 0 {
			nonAttack -= 10 // Semi-open file is also risky
		} else if color == Black && blackPawns == 0 {
			nonAttack -= 10
		}
	}

	// Scale the positional terms by enemy attacking material (full weight at
	// >=8 ~ Q+2R; collapses toward 0 once queens/heavies trade off).
	attackerMaterial := 4*PopCount(opponentQueenBitboard) + 2*PopCount(opponentRookBitboard) +
		PopCount(opponentBishopBitboard) + PopCount(opponentKnightBitboard)
	if attackerMaterial > kingSafetyFullAttackMaterial {
		attackerMaterial = kingSafetyFullAttackMaterial
	}
	safety := nonAttack * attackerMaterial / kingSafetyFullAttackMaterial

	// 2. Attack Pattern Evaluation (penalties for enemy pieces actually attacking
	// the king zone, not just sitting near it). Already self-scaled by the number
	// and weight of real attackers, so it is NOT material-gated above.
	safety -= evaluateKingAttackPatterns(sliderAtt,
		opponentQueenBitboard, opponentRookBitboard,
		opponentBishopBitboard, opponentKnightBitboard,
		int(kingSquare), color)

	return safety
}

// Evaluate castling bonus for king safety (simplified version for bitboard-only evaluation)
func evaluateCastling(board *Bitboard, color Color) int {
	bonus := 0

	// Check if king has already castled by looking at king position
	kingBitboard := board.GetBitboardOf(WhiteKing)
	if color == Black {
		kingBitboard = board.GetBitboardOf(BlackKing)
	}

	if kingBitboard == 0 {
		return 0
	}

	kingSquare := trailingZeros(kingBitboard)
	kingFile := int(kingSquare % 8)
	kingRank := int(kingSquare / 8)

	// Detect if castling has already occurred based on king position
	hasCastled := false
	if color == White {
		// White king starts on e1 (file 4, rank 0)
		// After castling: g1 (file 6) for kingside, c1 (file 2) for queenside
		if kingRank == 0 && (kingFile == 6 || kingFile == 2) {
			hasCastled = true
		}
	} else {
		// Black king starts on e8 (file 4, rank 7)
		// After castling: g8 (file 6) for kingside, c8 (file 2) for queenside
		if kingRank == 7 && (kingFile == 6 || kingFile == 2) {
			hasCastled = true
		}
	}

	if hasCastled {
		// Bonus for having castled (king safety achieved)
		bonus += 40

		// Additional bonus based on rook position to confirm valid castling
		var expectedRookSquare int
		var rookBitboard uint64

		if color == White {
			rookBitboard = board.GetBitboardOf(WhiteRook)
			if kingFile == 6 { // Kingside castling
				expectedRookSquare = 5 // f1
			} else { // Queenside castling
				expectedRookSquare = 3 // d1
			}
		} else {
			rookBitboard = board.GetBitboardOf(BlackRook)
			if kingFile == 6 { // Kingside castling
				expectedRookSquare = 61 // f8
			} else { // Queenside castling
				expectedRookSquare = 59 // d8
			}
		}

		// Extra bonus if rook is in the typical castled position
		if (rookBitboard & (1 << expectedRookSquare)) != 0 {
			bonus += 15 // Confirmed proper castling structure
		}
	}

	return bonus
}

// Fast optimized castling evaluation - minimal CPU overhead
func evaluateCastlingFast(board *Bitboard, color Color) int {
	var kingBitboard uint64
	if color == White {
		kingBitboard = board.GetBitboardOf(WhiteKing)
	} else {
		kingBitboard = board.GetBitboardOf(BlackKing)
	}

	if kingBitboard == 0 {
		return 0
	}

	kingSquare := trailingZeros(kingBitboard)
	kingFile := kingSquare % 8
	kingRank := kingSquare / 8

	// Simple castling detection - just check if king is in castled position
	if color == White {
		if kingRank == 0 && (kingFile == 6 || kingFile == 2) {
			return 40 // Castled bonus
		}
	} else {
		if kingRank == 7 && (kingFile == 6 || kingFile == 2) {
			return 40 // Castled bonus
		}
	}

	return 0
}

// Evaluate center control bonus - reward occupation and control of central squares
func evaluateCenterControl(board *Bitboard, color Color) int {
	bonus := 0

	// Central squares: e4, d4, e5, d5 (squares 28, 27, 36, 35)
	centralSquares := []int{27, 28, 35, 36}                                        // d4, e4, d5, e5
	extendedCenterSquares := []int{18, 19, 20, 21, 26, 29, 34, 37, 42, 43, 44, 45} // c3-f3, c4-f4, c6-f6

	var ownPawns, ownKnights, ownBishops, ownQueens uint64

	if color == White {
		ownPawns = board.GetBitboardOf(WhitePawn)
		ownKnights = board.GetBitboardOf(WhiteKnight)
		ownBishops = board.GetBitboardOf(WhiteBishop)
		ownQueens = board.GetBitboardOf(WhiteQueen)
	} else {
		ownPawns = board.GetBitboardOf(BlackPawn)
		ownKnights = board.GetBitboardOf(BlackKnight)
		ownBishops = board.GetBitboardOf(BlackBishop)
		ownQueens = board.GetBitboardOf(BlackQueen)
	}

	// 1. Piece occupation bonuses for central squares
	for _, square := range centralSquares {
		squareMask := uint64(1) << square

		if (ownPawns & squareMask) != 0 {
			bonus += 25 // Pawn in center is very valuable
		} else if (ownKnights & squareMask) != 0 {
			bonus += 20 // Knight in center is strong
		} else if (ownBishops & squareMask) != 0 {
			bonus += 15 // Bishop in center is good
		} else if (ownQueens & squareMask) != 0 {
			bonus += 10 // Queen in center can be powerful but also vulnerable
		}
		// Rooks and kings in center are usually not beneficial
	}

	// 2. Extended center occupation (smaller bonuses)
	for _, square := range extendedCenterSquares {
		squareMask := uint64(1) << square

		if (ownPawns & squareMask) != 0 {
			bonus += 8 // Extended center pawn
		} else if (ownKnights & squareMask) != 0 {
			bonus += 10 // Knight in extended center
		} else if (ownBishops & squareMask) != 0 {
			bonus += 6 // Bishop in extended center
		}
	}

	// 3. Pawn control of central squares (pawn attacks center even if not occupying)
	for _, square := range centralSquares {
		file := square % 8
		rank := square / 8

		// Check if our pawns control this square
		controlledByPawn := false

		if color == White {
			// White pawns attack diagonally upward
			if rank > 0 { // Can be attacked from rank below
				leftAttack := (rank-1)*8 + (file - 1)
				rightAttack := (rank-1)*8 + (file + 1)

				if file > 0 && (ownPawns&(1<<leftAttack)) != 0 {
					controlledByPawn = true
				}
				if file < 7 && (ownPawns&(1<<rightAttack)) != 0 {
					controlledByPawn = true
				}
			}
		} else {
			// Black pawns attack diagonally downward
			if rank < 7 { // Can be attacked from rank above
				leftAttack := (rank+1)*8 + (file - 1)
				rightAttack := (rank+1)*8 + (file + 1)

				if file > 0 && (ownPawns&(1<<leftAttack)) != 0 {
					controlledByPawn = true
				}
				if file < 7 && (ownPawns&(1<<rightAttack)) != 0 {
					controlledByPawn = true
				}
			}
		}

		if controlledByPawn {
			bonus += 15 // Bonus for controlling center with pawns
		}
	}

	return bonus
}

// Fast optimized center control - minimal overhead version
func evaluateCenterControlFast(board *Bitboard, color Color) int {
	bonus := 0

	// Just check piece occupation of core central squares (d4, e4, d5, e5)
	centralSquares := [4]int{27, 28, 35, 36} // d4, e4, d5, e5

	var ownPawns, ownKnights uint64
	if color == White {
		ownPawns = board.GetBitboardOf(WhitePawn)
		ownKnights = board.GetBitboardOf(WhiteKnight)
	} else {
		ownPawns = board.GetBitboardOf(BlackPawn)
		ownKnights = board.GetBitboardOf(BlackKnight)
	}

	// Quick check for occupation of central squares
	for _, square := range centralSquares {
		squareMask := uint64(1) << square
		if (ownPawns & squareMask) != 0 {
			bonus += 25 // Pawn in center
		} else if (ownKnights & squareMask) != 0 {
			bonus += 20 // Knight in center
		}
	}

	return bonus
}

// Evaluate piece development - reward proper opening principles
func evaluatePieceDevelopment(board *Bitboard, color Color) int {
	bonus := 0

	var ownKnights, ownBishops, ownRooks, ownQueen, ownKing uint64
	var startingRank int

	if color == White {
		ownKnights = board.GetBitboardOf(WhiteKnight)
		ownBishops = board.GetBitboardOf(WhiteBishop)
		ownRooks = board.GetBitboardOf(WhiteRook)
		ownQueen = board.GetBitboardOf(WhiteQueen)
		ownKing = board.GetBitboardOf(WhiteKing)
		startingRank = 0 // White pieces start on rank 1 (0-indexed)
	} else {
		ownKnights = board.GetBitboardOf(BlackKnight)
		ownBishops = board.GetBitboardOf(BlackBishop)
		ownRooks = board.GetBitboardOf(BlackRook)
		ownQueen = board.GetBitboardOf(BlackQueen)
		ownKing = board.GetBitboardOf(BlackKing)
		startingRank = 7 // Black pieces start on rank 8 (7-indexed)
	}

	// 1. Knight development bonus
	knightsDeveloped := 0
	for knightBB := ownKnights; knightBB != 0; knightBB &= knightBB - 1 {
		square := trailingZeros(knightBB)
		rank := square / 8

		if color == White && rank > startingRank {
			knightsDeveloped++
			bonus += 20 // Bonus for developed knight

			// Extra bonus for knights in good squares (f3, c3, f6, c6 area)
			file := square % 8
			if (rank == 2 || rank == 5) && (file == 2 || file == 5) { // c3, f3, c6, f6
				bonus += 10 // Extra bonus for knights in ideal squares
			}
		} else if color == Black && rank < startingRank {
			knightsDeveloped++
			bonus += 20 // Bonus for developed knight

			// Extra bonus for knights in good squares
			file := square % 8
			if (rank == 2 || rank == 5) && (file == 2 || file == 5) {
				bonus += 10 // Extra bonus for knights in ideal squares
			}
		}
	}

	// 2. Bishop development bonus
	bishopsDeveloped := 0
	for bishopBB := ownBishops; bishopBB != 0; bishopBB &= bishopBB - 1 {
		square := trailingZeros(bishopBB)
		rank := square / 8

		if color == White && rank > startingRank {
			bishopsDeveloped++
			bonus += 15 // Bonus for developed bishop
		} else if color == Black && rank < startingRank {
			bishopsDeveloped++
			bonus += 15 // Bonus for developed bishop
		}
	}

	// 3. Early queen penalty (queen should not be developed too early)
	queenDevelopedEarly := false
	if ownQueen != 0 {
		queenSquare := trailingZeros(ownQueen)
		queenRank := queenSquare / 8

		if color == White && queenRank > startingRank {
			queenDevelopedEarly = true
		} else if color == Black && queenRank < startingRank {
			queenDevelopedEarly = true
		}

		// Penalty increases if queen is out but minor pieces aren't developed
		if queenDevelopedEarly {
			totalMinorPiecesDeveloped := knightsDeveloped + bishopsDeveloped

			if totalMinorPiecesDeveloped == 0 {
				bonus -= 30 // Heavy penalty for queen out with no minor pieces developed
			} else if totalMinorPiecesDeveloped == 1 {
				bonus -= 20 // Medium penalty
			} else if totalMinorPiecesDeveloped == 2 {
				bonus -= 10 // Light penalty
			}
			// No penalty if 3+ minor pieces developed (queen out is fine then)
		}
	}

	// 4. Castling preparation bonus (king and rook coordination)
	if ownKing != 0 {
		kingSquare := trailingZeros(ownKing)
		kingRank := kingSquare / 8
		kingFile := kingSquare % 8

		// Check if king is still on starting square (good for castling preparation)
		if color == White && kingRank == 0 && kingFile == 4 { // e1
			// Bonus for keeping king on starting square while developing
			if knightsDeveloped >= 1 || bishopsDeveloped >= 1 {
				bonus += 8 // Small bonus for development while keeping king safe
			}
		} else if color == Black && kingRank == 7 && kingFile == 4 { // e8
			if knightsDeveloped >= 1 || bishopsDeveloped >= 1 {
				bonus += 8 // Small bonus for development while keeping king safe
			}
		}
	}

	// 5. Rook development penalty (rooks should generally stay on back rank early)
	for rookBB := ownRooks; rookBB != 0; rookBB &= rookBB - 1 {
		square := trailingZeros(rookBB)
		rank := square / 8

		// Penalty for rooks leaving back rank too early
		if color == White && rank > startingRank {
			// Only penalty if minor pieces aren't well developed
			if knightsDeveloped+bishopsDeveloped < 3 {
				bonus -= 15 // Penalty for premature rook development
			}
		} else if color == Black && rank < startingRank {
			if knightsDeveloped+bishopsDeveloped < 3 {
				bonus -= 15 // Penalty for premature rook development
			}
		}
	}

	return bonus
}

// Helper function for detailed pawn shield evaluation
func evaluatePawnShield(pawnBitboard uint64, kingFile int, kingRank int, color Color) int {
	shieldBonus := 0

	// Check pawn shield in three files around king
	for fileOffset := -1; fileOffset <= 1; fileOffset++ {
		file := kingFile + fileOffset
		if file < 0 || file > 7 {
			continue
		}

		// Closest own pawn in front of the king on this file, via a single bit scan
		// instead of a rank-by-rank loop. Legal positions keep pawns on ranks 2-7, so
		// the old loop's rankOffset<=6 cap never excluded a reachable pawn — this is
		// bit-identical to that scan (the node-identity gate enforces it).
		fileBB := uint64(0x0101010101010101) << uint(file)
		var inFront uint64
		if color == White {
			inFront = pawnBitboard & fileBB & (^uint64(0) << uint((kingRank+1)*8)) // ranks above the king
		} else {
			inFront = pawnBitboard & fileBB & ((uint64(1) << uint(kingRank*8)) - 1) // ranks below the king
		}

		if inFront != 0 {
			var closestPawnRank int
			if color == White {
				closestPawnRank = bits.TrailingZeros64(inFront) >> 3 // nearest rank above the king
			} else {
				closestPawnRank = (63 - bits.LeadingZeros64(inFront)) >> 3 // nearest rank below the king
			}

			var distance int
			if color == White {
				distance = closestPawnRank - kingRank
			} else {
				distance = kingRank - closestPawnRank
			}

			// Shelter is scored as a penalty-from-ideal, not a reward: an ideally
			// placed (adjacent, unmoved) pawn nets ~0, and only distance, advancement,
			// or a missing pawn create signal. The old form awarded a large positive
			// (+20..+30/file) for the NORMAL case of simply having a shield, which made
			// the term positive-sum and inflated king-safety by a measured +13cp
			// own-side in lost positions — NGN rating its own superficially-intact
			// shelter as "safe" while it drifted positionally into a mated game.
			baseBonus := -distance * 3 // closer is better; ideal adjacent pawn = 0
			if fileOffset == 0 {
				baseBonus += 3 // center file matters slightly more
			}

			// Bonus varies by how advanced the pawn is
			var pawnAdvancement int
			if color == White {
				pawnAdvancement = closestPawnRank - 1 // White pawns start on rank 2
			} else {
				pawnAdvancement = 6 - closestPawnRank // Black pawns start on rank 7
			}

			if pawnAdvancement == 0 {
				baseBonus += 3 // Unmoved pawn is a slightly stronger shield
			} else if pawnAdvancement >= 3 {
				baseBonus -= 5 // Advanced pawn is weaker shield
			}

			shieldBonus += baseBonus
		} else {
			// Heavy penalty for missing pawn shield
			if fileOffset == 0 {
				shieldBonus -= 25 // Center file most critical
			} else {
				shieldBonus -= 15 // Side files also important
			}
		}
	}

	return shieldBonus
}

// evaluateKingAttackPatterns computes a king-safety penalty based on real
// attack bitboards rather than just piece proximity. Each opponent piece type
// contributes attack-units proportional to the number of king-zone squares it
// actually attacks (knight=2, bishop=2, rook=3, queen=5 per attacked square).
// With 2+ distinct attackers, the total is fed through a quadratic curve so
// multi-piece attacks compound super-linearly; with 0-1 attackers, a small
// linear penalty captures harassment without overcommitting. Capped at 500cp.
func evaluateKingAttackPatterns(sliderAtt *[64]uint64, queenBB, rookBB, bishopBB, knightBB uint64,
	kingSquare int, color Color) int {

	// King zone: the king's square, its 8 neighbors, plus a one-rank
	// extension toward the enemy half (where pieces stage attacks from).
	kingZone := KingAttacks[kingSquare] | (uint64(1) << kingSquare)
	if color == White {
		kingZone |= kingZone << 8
	} else {
		kingZone |= kingZone >> 8
	}

	attackUnits := 0
	attackerCount := 0

	for bb := knightBB; bb != 0; bb &= bb - 1 {
		sq := bits.TrailingZeros64(bb)
		hits := KnightAttacks[sq] & kingZone
		if hits != 0 {
			attackerCount++
			attackUnits += 2 * PopCount(hits)
		}
	}
	// Sliders read their raw attack set from the shared sliderAtt table (filled
	// once per eval against the full occupancy) — bit-identical to recomputing the
	// magic lookup here, but mobility already paid for it this eval.
	for bb := bishopBB; bb != 0; bb &= bb - 1 {
		sq := bits.TrailingZeros64(bb)
		hits := sliderAtt[sq] & kingZone
		if hits != 0 {
			attackerCount++
			attackUnits += 2 * PopCount(hits)
		}
	}
	for bb := rookBB; bb != 0; bb &= bb - 1 {
		sq := bits.TrailingZeros64(bb)
		hits := sliderAtt[sq] & kingZone
		if hits != 0 {
			attackerCount++
			attackUnits += 3 * PopCount(hits)
		}
	}
	for bb := queenBB; bb != 0; bb &= bb - 1 {
		sq := bits.TrailingZeros64(bb)
		hits := sliderAtt[sq] & kingZone
		if hits != 0 {
			attackerCount++
			attackUnits += 5 * PopCount(hits)
		}
	}

	if attackerCount < 2 {
		return attackUnits / 2
	}

	penalty := attackUnits * attackUnits / 8
	if penalty > 500 {
		penalty = 500
	}
	return penalty
}

// Basic endgame knowledge evaluation
func evaluateBasicEndgames(board *Bitboard) int {
	endgameBonus := 0

	// Count material to identify endgame types
	whitePawns := PopCount(board.GetBitboardOf(WhitePawn))
	blackPawns := PopCount(board.GetBitboardOf(BlackPawn))
	whiteKnights := PopCount(board.GetBitboardOf(WhiteKnight))
	blackKnights := PopCount(board.GetBitboardOf(BlackKnight))
	whiteBishops := PopCount(board.GetBitboardOf(WhiteBishop))
	blackBishops := PopCount(board.GetBitboardOf(BlackBishop))
	whiteRooks := PopCount(board.GetBitboardOf(WhiteRook))
	blackRooks := PopCount(board.GetBitboardOf(BlackRook))
	whiteQueens := PopCount(board.GetBitboardOf(WhiteQueen))
	blackQueens := PopCount(board.GetBitboardOf(BlackQueen))

	// Total pieces (excluding kings)
	whitePieces := whitePawns + whiteKnights + whiteBishops + whiteRooks + whiteQueens
	blackPieces := blackPawns + blackKnights + blackBishops + blackRooks + blackQueens

	// 1. King and pawn endgames - evaluate opposition
	if whitePieces <= 1 && blackPieces <= 1 && (whitePawns > 0 || blackPawns > 0) {
		oppositionBonus := evaluateKingPawnOpposition(board)
		endgameBonus += oppositionBonus
	}

	// 2. Basic mate patterns
	if whitePieces <= 2 && blackPieces == 0 {
		// White has mating material vs lone king
		mateBonus := evaluateBasicMatePattern(board, White)
		endgameBonus += mateBonus
	}

	if blackPieces <= 2 && whitePieces == 0 {
		// Black has mating material vs lone king
		mateBonus := evaluateBasicMatePattern(board, Black)
		endgameBonus -= mateBonus
	}

	// 3. King activity in endgame
	if whitePieces+blackPieces <= 4 {
		kingActivityBonus := evaluateKingActivity(board)
		endgameBonus += kingActivityBonus
	}

	return endgameBonus
}

// Evaluate opposition in king and pawn endgames
func evaluateKingPawnOpposition(board *Bitboard) int {
	whiteKingBB := board.GetBitboardOf(WhiteKing)
	blackKingBB := board.GetBitboardOf(BlackKing)

	if whiteKingBB == 0 || blackKingBB == 0 {
		return 0
	}

	whiteKingSquare := trailingZeros(whiteKingBB)
	blackKingSquare := trailingZeros(blackKingBB)

	whiteKingFile := int(whiteKingSquare % 8)
	whiteKingRank := int(whiteKingSquare / 8)
	blackKingFile := int(blackKingSquare % 8)
	blackKingRank := int(blackKingSquare / 8)

	// Calculate distance between kings
	fileDistance := evalAbs(whiteKingFile - blackKingFile)
	rankDistance := evalAbs(whiteKingRank - blackKingRank)

	oppositionBonus := 0

	// Direct opposition - kings face each other with one square between
	if (fileDistance == 0 && rankDistance == 2) ||
		(fileDistance == 2 && rankDistance == 0) ||
		(fileDistance == 2 && rankDistance == 2) {

		// Determine who has the opposition (who moves next loses the opposition)
		// This is a simplified implementation - in real games, this depends on whose turn it is
		// For now, we'll give a small bonus to the side that can potentially gain opposition

		whitePawnsBB := board.GetBitboardOf(WhitePawn)
		blackPawnsBB := board.GetBitboardOf(BlackPawn)

		// If one side has pawns and the other doesn't, give opposition bonus to pawn side
		if PopCount(whitePawnsBB) > 0 && PopCount(blackPawnsBB) == 0 {
			oppositionBonus += 25 // White has pawns, wants opposition
		} else if PopCount(blackPawnsBB) > 0 && PopCount(whitePawnsBB) == 0 {
			oppositionBonus -= 25 // Black has pawns, wants opposition
		}
	}

	// Distant opposition - same principle but with more squares between
	if fileDistance%2 == 0 && rankDistance%2 == 0 &&
		(fileDistance+rankDistance) >= 4 && (fileDistance+rankDistance) <= 6 {

		whitePawnsBB := board.GetBitboardOf(WhitePawn)
		blackPawnsBB := board.GetBitboardOf(BlackPawn)

		if PopCount(whitePawnsBB) > 0 && PopCount(blackPawnsBB) == 0 {
			oppositionBonus += 15 // Smaller bonus for distant opposition
		} else if PopCount(blackPawnsBB) > 0 && PopCount(whitePawnsBB) == 0 {
			oppositionBonus -= 15
		}
	}

	return oppositionBonus
}

// Evaluate basic mate patterns (KQ vs K, KR vs K)
func evaluateBasicMatePattern(board *Bitboard, color Color) int {
	var kingBB, opponentKingBB uint64
	var queenBB, rookBB uint64

	if color == White {
		kingBB = board.GetBitboardOf(WhiteKing)
		opponentKingBB = board.GetBitboardOf(BlackKing)
		queenBB = board.GetBitboardOf(WhiteQueen)
		rookBB = board.GetBitboardOf(WhiteRook)
	} else {
		kingBB = board.GetBitboardOf(BlackKing)
		opponentKingBB = board.GetBitboardOf(WhiteKing)
		queenBB = board.GetBitboardOf(BlackQueen)
		rookBB = board.GetBitboardOf(BlackRook)
	}

	if kingBB == 0 || opponentKingBB == 0 {
		return 0
	}

	mateBonus := 0
	opponentKingSquare := trailingZeros(opponentKingBB)
	opponentKingFile := int(opponentKingSquare % 8)
	opponentKingRank := int(opponentKingSquare / 8)

	// Distance from center (enemy king should be driven to edge for mate)
	centerDistance := evalAbs(opponentKingFile-3) + evalAbs(opponentKingFile-4) +
		evalAbs(opponentKingRank-3) + evalAbs(opponentKingRank-4)
	centerDistance = centerDistance / 2 // Average distance from center files/ranks

	// Bonus for driving enemy king to edge
	edgeBonus := centerDistance * 10

	// KQ vs K mate
	if PopCount(queenBB) > 0 {
		mateBonus += 800 + edgeBonus // Massive bonus for having queen vs bare king

		// Additional bonus for restricting enemy king
		if opponentKingFile <= 1 || opponentKingFile >= 6 {
			mateBonus += 100 // King near edge
		}
		if opponentKingRank <= 1 || opponentKingRank >= 6 {
			mateBonus += 100 // King near edge
		}

		// Bonus for king proximity (supporting the mate)
		kingSquare := trailingZeros(kingBB)
		kingFile := int(kingSquare % 8)
		kingRank := int(kingSquare / 8)
		kingDistance := evalAbs(kingFile-opponentKingFile) + evalAbs(kingRank-opponentKingRank)
		if kingDistance <= 3 {
			mateBonus += (4 - kingDistance) * 50 // Closer king is better
		}
	}

	// KR vs K mate (harder than KQ vs K)
	if PopCount(rookBB) > 0 && PopCount(queenBB) == 0 {
		mateBonus += 600 + edgeBonus // Large bonus for rook vs bare king

		// Additional bonus for cutting off enemy king
		if opponentKingFile <= 1 || opponentKingFile >= 6 {
			mateBonus += 80
		}
		if opponentKingRank <= 1 || opponentKingRank >= 6 {
			mateBonus += 80
		}

		// Bonus for king proximity (crucial for rook mates)
		kingSquare := trailingZeros(kingBB)
		kingFile := int(kingSquare % 8)
		kingRank := int(kingSquare / 8)
		kingDistance := evalAbs(kingFile-opponentKingFile) + evalAbs(kingRank-opponentKingRank)
		if kingDistance <= 3 {
			mateBonus += (4 - kingDistance) * 60 // Closer king is even more important for rook mates
		}

		// Extra bonus for rook cutting off ranks/files
		rookSquare := trailingZeros(rookBB)
		rookFile := int(rookSquare % 8)
		rookRank := int(rookSquare / 8)
		if rookFile == opponentKingFile || rookRank == opponentKingRank {
			mateBonus += 50 // Rook on same rank/file as enemy king
		}
	}

	return mateBonus
}

// Evaluate king activity in endgames
func evaluateKingActivity(board *Bitboard) int {
	whiteKingBB := board.GetBitboardOf(WhiteKing)
	blackKingBB := board.GetBitboardOf(BlackKing)

	if whiteKingBB == 0 || blackKingBB == 0 {
		return 0
	}

	whiteKingSquare := trailingZeros(whiteKingBB)
	blackKingSquare := trailingZeros(blackKingBB)

	whiteKingFile := int(whiteKingSquare % 8)
	whiteKingRank := int(whiteKingSquare / 8)
	blackKingFile := int(blackKingSquare % 8)
	blackKingRank := int(blackKingSquare / 8)

	activityBonus := 0

	// Bonus for centralized kings in endgame
	whiteCentrality := 0
	blackCentrality := 0

	// Distance from center (closer to center is better)
	whiteCenterDistance := evalAbs(whiteKingFile-3) + evalAbs(whiteKingFile-4) +
		evalAbs(whiteKingRank-3) + evalAbs(whiteKingRank-4)
	whiteCenterDistance = whiteCenterDistance / 2
	whiteCentrality = 6 - whiteCenterDistance // Convert to bonus

	blackCenterDistance := evalAbs(blackKingFile-3) + evalAbs(blackKingFile-4) +
		evalAbs(blackKingRank-3) + evalAbs(blackKingRank-4)
	blackCenterDistance = blackCenterDistance / 2
	blackCentrality = 6 - blackCenterDistance

	activityBonus += (whiteCentrality - blackCentrality) * 3

	// Bonus for kings supporting their own pawns or attacking enemy pawns
	whitePawnsBB := board.GetBitboardOf(WhitePawn)
	blackPawnsBB := board.GetBitboardOf(BlackPawn)

	// Check if king is supporting own pawns
	for wpbb := whitePawnsBB; wpbb != 0; wpbb &= wpbb - 1 {
		pawnSquare := bits.TrailingZeros64(wpbb)
		pawnFile := int(pawnSquare % 8)
		pawnRank := int(pawnSquare / 8)
		distance := evalAbs(whiteKingFile-pawnFile) + evalAbs(whiteKingRank-pawnRank)
		if distance <= 2 {
			activityBonus += 8 // Bonus for king supporting pawn
		}
	}

	for bpbb := blackPawnsBB; bpbb != 0; bpbb &= bpbb - 1 {
		pawnSquare := bits.TrailingZeros64(bpbb)
		pawnFile := int(pawnSquare % 8)
		pawnRank := int(pawnSquare / 8)
		distance := evalAbs(blackKingFile-pawnFile) + evalAbs(blackKingRank-pawnRank)
		if distance <= 2 {
			activityBonus -= 8 // Bonus for black king supporting pawn
		}
	}

	return activityBonus
}

// Piece coordination and harmony evaluation
func evaluatePieceCoordination(board *Bitboard, color Color) int {
	coordinationBonus := 0

	// Get piece bitboards for the given color
	var kingBB, queenBB, rookBB, bishopBB, knightBB, pawnBB uint64
	var opponentPawnBB uint64

	if color == White {
		kingBB = board.GetBitboardOf(WhiteKing)
		queenBB = board.GetBitboardOf(WhiteQueen)
		rookBB = board.GetBitboardOf(WhiteRook)
		bishopBB = board.GetBitboardOf(WhiteBishop)
		knightBB = board.GetBitboardOf(WhiteKnight)
		pawnBB = board.GetBitboardOf(WhitePawn)
		opponentPawnBB = board.GetBitboardOf(BlackPawn)
	} else {
		kingBB = board.GetBitboardOf(BlackKing)
		queenBB = board.GetBitboardOf(BlackQueen)
		rookBB = board.GetBitboardOf(BlackRook)
		bishopBB = board.GetBitboardOf(BlackBishop)
		knightBB = board.GetBitboardOf(BlackKnight)
		pawnBB = board.GetBitboardOf(BlackPawn)
		opponentPawnBB = board.GetBitboardOf(WhitePawn)
	}

	// 1. Evaluate mutual piece protection
	coordinationBonus += evaluatePieceProtection(kingBB, queenBB, rookBB, bishopBB, knightBB, pawnBB, color)

	// 2. Evaluate outpost squares for knights and bishops
	coordinationBonus += evaluateOutpostSquares(knightBB, bishopBB, pawnBB, opponentPawnBB, color)

	// 3. Evaluate piece batteries (rook behind queen, doubled rooks)
	coordinationBonus += evaluatePieceBatteries(queenBB, rookBB, bishopBB)

	// 4. Evaluate piece cooperation in attacks
	coordinationBonus += evaluatePieceCooperation(queenBB, rookBB, bishopBB, knightBB, color)

	// 5. Evaluate centralization coordination
	coordinationBonus += evaluateCentralization(queenBB, rookBB, bishopBB, knightBB, pawnBB)

	return coordinationBonus
}

// Evaluate how well pieces protect each other
func evaluatePieceProtection(kingBB, queenBB, rookBB, bishopBB, knightBB, pawnBB uint64, color Color) int {
	protection := 0

	// Process queens
	for qbb := queenBB; qbb != 0; qbb &= qbb - 1 {
		queenSquare := bits.TrailingZeros64(qbb)
		protectors := countProtectorsBB(queenSquare, rookBB, bishopBB, knightBB, pawnBB, color)
		protection += protectors * 8 // Reduced from 15 to prevent evaluation inflation
	}

	// Process rooks
	for rbb := rookBB; rbb != 0; rbb &= rbb - 1 {
		rookSquare := bits.TrailingZeros64(rbb)
		protectors := countProtectorsBB(rookSquare, 0, bishopBB, knightBB, pawnBB, color)
		// Add queen protection
		for qbb := queenBB; qbb != 0; qbb &= qbb - 1 {
			queenSquare := bits.TrailingZeros64(qbb)
			if canQueenProtect(queenSquare, rookSquare) {
				protectors++
			}
		}
		protection += protectors * 4 // Reduced from 8 to prevent evaluation inflation
	}

	// Process bishops
	for bbb := bishopBB; bbb != 0; bbb &= bbb - 1 {
		bishopSquare := bits.TrailingZeros64(bbb)
		protectors := countProtectorsBB(bishopSquare, rookBB, 0, knightBB, pawnBB, color)
		for qbb := queenBB; qbb != 0; qbb &= qbb - 1 {
			queenSquare := bits.TrailingZeros64(qbb)
			if canQueenProtect(queenSquare, bishopSquare) {
				protectors++
			}
		}
		protection += protectors * 3 // Reduced from 5 to prevent evaluation inflation
	}

	// Process knights
	for nbb := knightBB; nbb != 0; nbb &= nbb - 1 {
		knightSquare := bits.TrailingZeros64(nbb)
		protectors := countProtectorsBB(knightSquare, rookBB, bishopBB, 0, pawnBB, color)
		for qbb := queenBB; qbb != 0; qbb &= qbb - 1 {
			queenSquare := bits.TrailingZeros64(qbb)
			if canQueenProtect(queenSquare, knightSquare) {
				protectors++
			}
		}
		protection += protectors * 3 // Reduced from 5 to prevent evaluation inflation
	}

	return protection
}

// Count how many pieces can protect a given square (using bitboards)
// Optimized with precomputed attack tables for O(1) knight/pawn lookups
func countProtectorsBB(targetSquare int, rookBB, bishopBB, knightBB, pawnBB uint64, color Color) int {
	protectors := 0

	// Knight protection - O(1) with precomputed table
	protectors += PopCount(knightBB & KnightAttacks[targetSquare])

	// Pawn protection - O(1) with precomputed table
	if color == White {
		protectors += PopCount(pawnBB & WhitePawnAttackers[targetSquare])
	} else {
		protectors += PopCount(pawnBB & BlackPawnAttackers[targetSquare])
	}

	// Rook protection - use rank/file masks
	targetFile := targetSquare % 8
	targetRank := targetSquare / 8
	rookMask := RankMasks[targetRank] | FileMasks[targetFile]
	protectors += PopCount(rookBB & rookMask)

	// Bishop protection (still need to iterate for diagonal check)
	for bbb := bishopBB; bbb != 0; bbb &= bbb - 1 {
		bishopSquare := bits.TrailingZeros64(bbb)
		if onSameDiagonal(bishopSquare, targetSquare) {
			protectors++
		}
	}

	return protectors
}

// Keep old version for compatibility if needed
func countProtectors(targetSquare int, rooks, bishops, knights, pawns []int, color Color) int {
	protectors := 0
	targetFile := targetSquare % 8
	targetRank := targetSquare / 8

	// Check rook protection (same rank or file)
	for _, rookSquare := range rooks {
		rookFile := rookSquare % 8
		rookRank := rookSquare / 8
		if rookFile == targetFile || rookRank == targetRank {
			protectors++
		}
	}

	// Check bishop protection (same diagonal)
	for _, bishopSquare := range bishops {
		if onSameDiagonal(bishopSquare, targetSquare) {
			protectors++
		}
	}

	// Check knight protection
	for _, knightSquare := range knights {
		if canKnightReach(knightSquare, targetSquare) {
			protectors++
		}
	}

	// Check pawn protection
	for _, pawnSquare := range pawns {
		if canPawnProtect(pawnSquare, targetSquare, color) {
			protectors++
		}
	}

	return protectors
}

// Check if pieces are on the same diagonal
func onSameDiagonal(square1, square2 int) bool {
	file1, rank1 := square1%8, square1/8
	file2, rank2 := square2%8, square2/8
	return evalAbs(file1-file2) == evalAbs(rank1-rank2)
}

// Check if knight can reach target square
func canKnightReach(knightSquare, targetSquare int) bool {
	knightFile, knightRank := knightSquare%8, knightSquare/8
	targetFile, targetRank := targetSquare%8, targetSquare/8

	fileDiff := evalAbs(knightFile - targetFile)
	rankDiff := evalAbs(knightRank - targetRank)

	return (fileDiff == 2 && rankDiff == 1) || (fileDiff == 1 && rankDiff == 2)
}

// Check if pawn can protect target square
//
//go:inline
func canPawnProtect(pawnSquare, targetSquare int, color Color) bool {
	pawnFile, pawnRank := pawnSquare%8, pawnSquare/8
	targetFile, targetRank := targetSquare%8, targetSquare/8

	// Pawns protect diagonally forward
	if evalAbs(pawnFile-targetFile) != 1 {
		return false
	}

	if color == White {
		return targetRank == pawnRank+1
	} else {
		return targetRank == pawnRank-1
	}
}

// Check if queen can protect target square
func canQueenProtect(queenSquare, targetSquare int) bool {
	// Queen combines rook and bishop movement
	queenFile, queenRank := queenSquare%8, queenSquare/8
	targetFile, targetRank := targetSquare%8, targetSquare/8

	// Same rank, file, or diagonal
	return queenFile == targetFile || queenRank == targetRank || onSameDiagonal(queenSquare, targetSquare)
}

// Evaluate outpost squares for knights and bishops
func evaluateOutpostSquares(knightBB, bishopBB, pawnBB, opponentPawnBB uint64, color Color) int {
	outpostBonus := 0

	// Check knights on outpost squares
	for nbb := knightBB; nbb != 0; nbb &= nbb - 1 {
		knightSquare := bits.TrailingZeros64(nbb)
		if isOutpostSquareBB(knightSquare, pawnBB, opponentPawnBB, color) {
			// Knights on outposts are very strong
			bonus := 20

			// Extra bonus for advanced outposts
			rank := knightSquare / 8
			if color == White && rank >= 4 {
				bonus += (rank - 3) * 5 // Bonus increases with advancement
			} else if color == Black && rank <= 3 {
				bonus += (4 - rank) * 5
			}

			outpostBonus += bonus
		}
	}

	// Check bishops on outpost squares (less common but still valuable)
	for bbb := bishopBB; bbb != 0; bbb &= bbb - 1 {
		bishopSquare := bits.TrailingZeros64(bbb)
		if isOutpostSquareBB(bishopSquare, pawnBB, opponentPawnBB, color) {
			bonus := 12

			// Bishops on outposts in enemy territory
			rank := bishopSquare / 8
			if color == White && rank >= 4 {
				bonus += (rank - 3) * 3
			} else if color == Black && rank <= 3 {
				bonus += (4 - rank) * 3
			}

			outpostBonus += bonus
		}
	}

	return outpostBonus
}

// Check if a square is an outpost (supported by pawn, can't be attacked by enemy pawns)
// Optimized with precomputed pawn attack tables
func isOutpostSquareBB(square int, friendlyPawnBB, enemyPawnBB uint64, color Color) bool {
	file := square % 8
	rank := square / 8

	// 3. Must be in enemy territory (ranks 4-6 for white, ranks 3-5 for black)
	// Check this first as it's cheapest
	if color == White && (rank < 3 || rank > 5) {
		return false
	}
	if color == Black && (rank < 2 || rank > 4) {
		return false
	}

	// 1. Must be supported by our own pawns - O(1) lookup
	var supported bool
	if color == White {
		supported = (friendlyPawnBB & WhitePawnAttackers[square]) != 0
	} else {
		supported = (friendlyPawnBB & BlackPawnAttackers[square]) != 0
	}
	if !supported {
		return false
	}

	// 2. Must not be currently attackable by enemy pawns - O(1) lookup
	if color == White {
		if (enemyPawnBB & BlackPawnAttackers[square]) != 0 {
			return false
		}
	} else {
		if (enemyPawnBB & WhitePawnAttackers[square]) != 0 {
			return false
		}
	}

	// Check if enemy pawn can attack this square in the future
	// (on adjacent files, can advance to attack)
	for ebb := enemyPawnBB; ebb != 0; ebb &= ebb - 1 {
		enemyPawnSquare := bits.TrailingZeros64(ebb)
		enemyFile := enemyPawnSquare % 8
		enemyRank := enemyPawnSquare / 8

		if evalAbs(enemyFile-file) == 1 {
			if color == White && enemyRank < rank {
				return false
			} else if color == Black && enemyRank > rank {
				return false
			}
		}
	}

	return true
}

// Keep old version for compatibility
func isOutpostSquare(square int, friendlyPawns, enemyPawns []int, color Color) bool {
	file := square % 8
	rank := square / 8

	// 1. Must be supported by our own pawns
	supported := false
	for _, pawnSquare := range friendlyPawns {
		if canPawnProtect(pawnSquare, square, color) {
			supported = true
			break
		}
	}

	if !supported {
		return false
	}

	// 2. Must not be attackable by enemy pawns (now or in future)
	for _, enemyPawnSquare := range enemyPawns {
		enemyFile := enemyPawnSquare % 8
		enemyRank := enemyPawnSquare / 8

		// Check if enemy pawn can attack this square now
		enemyColor := White
		if color == White {
			enemyColor = Black
		}

		if canPawnProtect(enemyPawnSquare, square, enemyColor) {
			return false
		}

		// Check if enemy pawn could advance to attack this square
		if evalAbs(enemyFile-file) == 1 {
			if color == White {
				// Enemy pawns advance toward us (down ranks)
				if enemyRank > rank && enemyRank-1 == rank {
					return false // Pawn could advance to attack
				}
			} else {
				// Enemy pawns advance toward us (up ranks)
				if enemyRank < rank && enemyRank+1 == rank {
					return false
				}
			}
		}
	}

	return true
}

// Evaluate piece batteries (rook behind queen, doubled rooks, bishop pairs)
func evaluatePieceBatteries(queenBB, rookBB, bishopBB uint64) int {
	batteryBonus := 0

	// 1. Rook behind queen on same file or rank
	for queenBits := queenBB; queenBits != 0; queenBits &= queenBits - 1 {
		queenSquare := bits.TrailingZeros64(queenBits)
		queenFile, queenRank := queenSquare%8, queenSquare/8

		for rookBits := rookBB; rookBits != 0; rookBits &= rookBits - 1 {
			rookSquare := bits.TrailingZeros64(rookBits)
			rookFile, rookRank := rookSquare%8, rookSquare/8

			// Same file
			if queenFile == rookFile && queenRank != rookRank {
				batteryBonus += 15 // Vertical battery
			}

			// Same rank
			if queenRank == rookRank && queenFile != rookFile {
				batteryBonus += 15 // Horizontal battery
			}
		}
	}

	// 2. Doubled/tripled rooks - use arrays instead of maps for performance
	var fileRooks [8]int // file counts (files 0-7)
	var rankRooks [8]int // rank counts (ranks 0-7)

	for rookBits := rookBB; rookBits != 0; rookBits &= rookBits - 1 {
		rookSquare := bits.TrailingZeros64(rookBits)
		file, rank := rookSquare%8, rookSquare/8
		fileRooks[file]++
		rankRooks[rank]++
	}

	for _, count := range fileRooks {
		if count >= 2 {
			batteryBonus += count * 10 // Bonus for doubled rooks on file
		}
	}

	for _, count := range rankRooks {
		if count >= 2 {
			batteryBonus += count * 8 // Bonus for doubled rooks on rank
		}
	}

	// 3. Bishop pair on same diagonal
	bishopCount := bits.OnesCount64(bishopBB)
	if bishopCount >= 2 {
		// Check all pairs of bishops
		bishops := [8]int{} // Maximum 8 bishops theoretically
		idx := 0
		for bishopBits := bishopBB; bishopBits != 0; bishopBits &= bishopBits - 1 {
			bishops[idx] = bits.TrailingZeros64(bishopBits)
			idx++
		}

		for i := 0; i < bishopCount; i++ {
			for j := i + 1; j < bishopCount; j++ {
				if onSameDiagonal(bishops[i], bishops[j]) {
					batteryBonus += 12 // Bonus for bishops on same diagonal
				}
			}
		}
	}

	return batteryBonus
}

// Evaluate piece cooperation in attacks (SIMPLIFIED for performance)
func evaluatePieceCooperation(queenBB, rookBB, bishopBB, knightBB uint64, color Color) int {
	// Simplified piece cooperation: just count pieces and give a small bonus
	// The original complex version was taking 52% CPU time for ~5 Elo gain
	cooperationBonus := 0

	// Simple approach: give small bonus for having multiple piece types developed
	pieceTypes := 0
	if queenBB != 0 {
		pieceTypes++
	}
	if rookBB != 0 {
		pieceTypes++
	}
	if bishopBB != 0 {
		pieceTypes++
	}
	if knightBB != 0 {
		pieceTypes++
	}

	if pieceTypes >= 3 {
		cooperationBonus += 5 // Small bonus for piece development
	}
	if pieceTypes >= 4 {
		cooperationBonus += 5 // Additional bonus for full development
	}

	// 2. Piece coordination around enemy king (if we can find it)
	// This is a simplified version - in practice you'd get enemy king position

	return cooperationBonus
}

// Simplified attack pattern generators (these would normally be more complex)
func getQueenAttacks(square int) []int {
	// Simplified: return squares on same rank, file, and diagonals
	attacks := make([]int, 0, 27) // Max 8+8+8+3 squares
	file, rank := square%8, square/8

	// Add rank attacks
	for f := 0; f < 8; f++ {
		if f != file {
			attacks = append(attacks, rank*8+f)
		}
	}

	// Add file attacks
	for r := 0; r < 8; r++ {
		if r != rank {
			attacks = append(attacks, r*8+file)
		}
	}

	// Add diagonal attacks (simplified)
	for i := 1; i < 8; i++ {
		// Up-right diagonal
		if file+i < 8 && rank+i < 8 {
			attacks = append(attacks, (rank+i)*8+(file+i))
		}
		// Up-left diagonal
		if file-i >= 0 && rank+i < 8 {
			attacks = append(attacks, (rank+i)*8+(file-i))
		}
		// Down-right diagonal
		if file+i < 8 && rank-i >= 0 {
			attacks = append(attacks, (rank-i)*8+(file+i))
		}
		// Down-left diagonal
		if file-i >= 0 && rank-i >= 0 {
			attacks = append(attacks, (rank-i)*8+(file-i))
		}
	}

	return attacks
}

func getRookAttacks(square int) []int {
	attacks := make([]int, 0, 14)
	file, rank := square%8, square/8

	// Add rank attacks
	for f := 0; f < 8; f++ {
		if f != file {
			attacks = append(attacks, rank*8+f)
		}
	}

	// Add file attacks
	for r := 0; r < 8; r++ {
		if r != rank {
			attacks = append(attacks, r*8+file)
		}
	}

	return attacks
}

func getBishopAttacks(square int) []int {
	attacks := make([]int, 0, 13)
	file, rank := square%8, square/8

	// Add diagonal attacks (simplified)
	for i := 1; i < 8; i++ {
		// Up-right diagonal
		if file+i < 8 && rank+i < 8 {
			attacks = append(attacks, (rank+i)*8+(file+i))
		}
		// Up-left diagonal
		if file-i >= 0 && rank+i < 8 {
			attacks = append(attacks, (rank+i)*8+(file-i))
		}
		// Down-right diagonal
		if file+i < 8 && rank-i >= 0 {
			attacks = append(attacks, (rank-i)*8+(file+i))
		}
		// Down-left diagonal
		if file-i >= 0 && rank-i >= 0 {
			attacks = append(attacks, (rank-i)*8+(file-i))
		}
	}

	return attacks
}

func getKnightAttacks(square int) []int {
	attacks := make([]int, 0, 8)
	file, rank := square%8, square/8

	// All possible knight moves
	knightMoves := [][]int{
		{-2, -1}, {-2, 1}, {-1, -2}, {-1, 2},
		{1, -2}, {1, 2}, {2, -1}, {2, 1},
	}

	for _, move := range knightMoves {
		newFile := file + move[0]
		newRank := rank + move[1]

		if newFile >= 0 && newFile < 8 && newRank >= 0 && newRank < 8 {
			attacks = append(attacks, newRank*8+newFile)
		}
	}

	return attacks
}

// Evaluate centralization coordination
func evaluateCentralization(queenBB, rookBB, bishopBB, knightBB, pawnBB uint64) int {
	centralizationBonus := 0

	// Central squares (d4, d5, e4, e5)
	centralSquares := []int{27, 28, 35, 36}                                         // d4, e4, d5, e5
	extendedCentralSquares := []int{18, 19, 20, 21, 26, 29, 34, 37, 42, 43, 44, 45} // c3-f6 area

	// Check pieces on central squares
	for _, square := range centralSquares {
		squareMask := uint64(1 << square)

		if (knightBB & squareMask) != 0 {
			centralizationBonus += 15 // Knights are excellent in center
		}
		if (bishopBB & squareMask) != 0 {
			centralizationBonus += 10 // Bishops good in center
		}
		if (pawnBB & squareMask) != 0 {
			centralizationBonus += 12 // Central pawns are strong
		}
	}

	// Check pieces on extended central squares
	for _, square := range extendedCentralSquares {
		squareMask := uint64(1 << square)

		if (knightBB & squareMask) != 0 {
			centralizationBonus += 8 // Good knight squares
		}
		if (bishopBB & squareMask) != 0 {
			centralizationBonus += 5 // Decent bishop squares
		}
		if (queenBB & squareMask) != 0 {
			centralizationBonus += 6 // Queen can be good in center but risky
		}
	}

	// Bonus for multiple pieces controlling center
	centralControl := 0
	for _, square := range centralSquares {
		squareMask := uint64(1 << square)

		// Count pieces that could move to this square (simplified)
		if (knightBB|bishopBB|rookBB|queenBB)&squareMask != 0 {
			centralControl++
		}
	}

	// Bonus for controlling multiple central squares
	if centralControl >= 2 {
		centralizationBonus += centralControl * 3
	}

	return centralizationBonus
}

// Evaluate bishop pair bonus
func evaluateBishopPair(board *Bitboard, color Color) int {
	var bishopBB uint64

	if color == White {
		bishopBB = board.GetBitboardOf(WhiteBishop)
	} else {
		bishopBB = board.GetBitboardOf(BlackBishop)
	}

	bishopCount := PopCount(bishopBB)

	// Must have exactly 2 bishops for the pair bonus
	if bishopCount != 2 {
		return 0
	}

	// Extract the two bishop positions using bitboard iteration
	bishops := [2]int{}
	idx := 0
	for bishopBits := bishopBB; bishopBits != 0 && idx < 2; bishopBits &= bishopBits - 1 {
		bishops[idx] = bits.TrailingZeros64(bishopBits)
		idx++
	}

	if idx != 2 {
		return 0 // Safety check
	}

	// Check if bishops are on opposite colored squares
	bishop1Square := bishops[0]
	bishop2Square := bishops[1]

	// Calculate square colors (light or dark)
	// Light squares: (file + rank) % 2 == 0
	// Dark squares: (file + rank) % 2 == 1
	bishop1Color := (bishop1Square%8 + bishop1Square/8) % 2
	bishop2Color := (bishop2Square%8 + bishop2Square/8) % 2

	// Bishop pair bonus only applies if bishops are on opposite colored squares
	if bishop1Color != bishop2Color {
		// Base bishop pair bonus
		bonus := 30

		// Additional bonus in open positions (fewer pawns = more open)
		totalPawns := PopCount(board.GetBitboardOf(WhitePawn)) + PopCount(board.GetBitboardOf(BlackPawn))

		// The fewer pawns, the stronger the bishop pair
		if totalPawns <= 12 {
			bonus += 10 // Extra bonus in open positions
		}
		if totalPawns <= 8 {
			bonus += 10 // Even more bonus in very open positions
		}

		// Endgame bonus - bishop pair becomes stronger in endgame
		if isEndgame(board) {
			bonus += 15 // Additional endgame bonus
		}

		return bonus
	}

	return 0
}

// evaluateRookOnOpenFile gives bonuses for rooks on open and semi-open files
// Open file: no pawns of either color
// Semi-open file: no friendly pawns, but enemy pawns present
func evaluateRookOnOpenFile(board *Bitboard, color Color) int {
	bonus := 0

	var rookBB, friendlyPawns, enemyPawns uint64
	if color == White {
		rookBB = board.GetBitboardOf(WhiteRook)
		friendlyPawns = board.GetBitboardOf(WhitePawn)
		enemyPawns = board.GetBitboardOf(BlackPawn)
	} else {
		rookBB = board.GetBitboardOf(BlackRook)
		friendlyPawns = board.GetBitboardOf(BlackPawn)
		enemyPawns = board.GetBitboardOf(WhitePawn)
	}

	// Iterate through each rook
	for bb := rookBB; bb != 0; bb &= bb - 1 {
		sq := bits.TrailingZeros64(bb)
		file := sq % 8

		// File mask for this file
		fileMask := uint64(0x0101010101010101) << file

		hasFriendlyPawn := (friendlyPawns & fileMask) != 0
		hasEnemyPawn := (enemyPawns & fileMask) != 0

		if !hasFriendlyPawn && !hasEnemyPawn {
			// Fully open file
			bonus += 25
		} else if !hasFriendlyPawn {
			// Semi-open file (no friendly pawns, enemy pawns present)
			bonus += 15
		}
	}

	return bonus
}

// evaluateMobility counts available moves for pieces and awards bonuses
// Mobility is a key positional factor - pieces that can move to more squares are more valuable
// mobilityForSide sums one side's piece mobility given the shared occupancy and the
// side's excluded squares (own pieces + enemy pawn attacks). Factored out of
// evaluateMobility so evaluateMobilityDelta can compute the color-independent setup
// (occupancy, pawn-attack spans) once instead of once per color.
// mobilityForSide reads each slider's raw attack set from the shared sliderAtt
// table (filled once per eval by fillSliderAttacks against the full occupancy),
// so mobility and king-safety no longer each recompute the same magic lookups.
// Bit-identical to recomputing getBishopAttacksBB/getRookAttacksBB inline
// (sliderAtt[sq] holds exactly getBishopAttacksBB(sq, all) for a bishop,
// getRookAttacksBB for a rook, and the union for a queen).
// Per-count mobility tables (centipawns), indexed by the number of reachable
// squares (own pieces + enemy-pawn-attacked squares excluded). These un-freeze
// the old single linear slope-per-piece into a concave, phase-split shape:
//   - concave (diminishing returns past a per-piece "knee") — the first escape
//     squares matter most; a trapped piece is the real signal, a 12-square rook
//     barely better than an 11-square one.
//   - phase-split — heavy/long-range pieces (rook, bishop, queen) weigh more in
//     the endgame where open lines decide; the knight's short range weighs
//     slightly more in the middlegame. The old code ran ONE near-flat global
//     MG/EG weight (181/180) over every piece, so no per-piece taper existed.
//
// Zero-crossings keep the old baselines (knight 3, bishop/rook 6, queen 12),
// and the cp scale folds in the old ~1.8x global weight, so values sit near the
// old behaviour through the common midrange and only bend at the extremes.
// Hand-seeded (perMove cp, halved past the knee); the texel basis does not tune
// them — mobility now rides the non-tunable Fixed lump in the gradient trace.
var (
	knightMobMG = [9]int{-22, -14, -7, 0, 7, 14, 22, 25, 29}
	knightMobEG = [9]int{-20, -13, -7, 0, 7, 13, 20, 23, 26}

	bishopMobMG = [14]int{-32, -27, -22, -16, -11, -5, 0, 5, 11, 16, 19, 22, 24, 27}
	bishopMobEG = [14]int{-36, -30, -24, -18, -12, -6, 0, 6, 12, 18, 21, 24, 27, 30}

	rookMobMG = [15]int{-18, -15, -12, -9, -6, -3, 0, 3, 6, 9, 12, 14, 15, 17, 18}
	rookMobEG = [15]int{-27, -23, -18, -14, -9, -5, 0, 5, 9, 14, 18, 20, 23, 25, 27}

	queenMobMG = [28]int{-22, -20, -18, -16, -14, -13, -11, -9, -7, -5, -4, -2, 0, 2, 4, 5, 7, 8, 9, 10, 11, 12, 13, 14, 14, 15, 16, 17}
	queenMobEG = [28]int{-26, -24, -22, -20, -18, -15, -13, -11, -9, -7, -4, -2, 0, 2, 4, 7, 9, 10, 11, 12, 13, 14, 15, 17, 18, 19, 20, 21}
)

func mobilityForSide(sliderAtt *[64]uint64, excluded, knightBB, bishopBB, rookBB, queenBB uint64) (mg, eg int) {
	for bb := knightBB; bb != 0; bb &= bb - 1 {
		sq := bits.TrailingZeros64(bb)
		mc := PopCount(KnightAttacks[sq] &^ excluded)
		mg += knightMobMG[mc]
		eg += knightMobEG[mc]
	}
	for bb := bishopBB; bb != 0; bb &= bb - 1 {
		sq := bits.TrailingZeros64(bb)
		mc := PopCount(sliderAtt[sq] &^ excluded)
		mg += bishopMobMG[mc]
		eg += bishopMobEG[mc]
	}
	for bb := rookBB; bb != 0; bb &= bb - 1 {
		sq := bits.TrailingZeros64(bb)
		mc := PopCount(sliderAtt[sq] &^ excluded)
		mg += rookMobMG[mc]
		eg += rookMobEG[mc]
	}
	for bb := queenBB; bb != 0; bb &= bb - 1 {
		sq := bits.TrailingZeros64(bb)
		mc := PopCount(sliderAtt[sq] &^ excluded)
		mg += queenMobMG[mc]
		eg += queenMobEG[mc]
	}
	return mg, eg
}

// fillSliderAttacks populates sliderAtt[sq] with the raw magic-bitboard attack
// set of every bishop/rook/queen on the board, computed once against the shared
// occupancy. mobility and king-safety both read this table instead of each
// recomputing the same per-square magic lookups (~2x slider lookups per eval
// removed). Only slider squares are written; non-slider squares are never read.
func fillSliderAttacks(board *Bitboard, sliderAtt *[64]uint64) {
	occupied := board.GetWhitePieces() | board.GetBlackPieces()
	for bb := board.GetBitboardOf(WhiteBishop) | board.GetBitboardOf(BlackBishop); bb != 0; bb &= bb - 1 {
		sq := bits.TrailingZeros64(bb)
		sliderAtt[sq] = getBishopAttacksBB(sq, occupied)
	}
	for bb := board.GetBitboardOf(WhiteRook) | board.GetBitboardOf(BlackRook); bb != 0; bb &= bb - 1 {
		sq := bits.TrailingZeros64(bb)
		sliderAtt[sq] = getRookAttacksBB(sq, occupied)
	}
	for bb := board.GetBitboardOf(WhiteQueen) | board.GetBitboardOf(BlackQueen); bb != 0; bb &= bb - 1 {
		sq := bits.TrailingZeros64(bb)
		sliderAtt[sq] = getBishopAttacksBB(sq, occupied) | getRookAttacksBB(sq, occupied)
	}
}

// mobilitySetup computes the color-independent bitboards mobility needs: full
// occupancy and each side's pawn-attack span (a piece moving onto a square a pawn
// attacks is effectively lost, so those squares don't count as mobility).
func mobilitySetup(board *Bitboard) (whitePieces, blackPieces, allPieces, whitePawnAttacks, blackPawnAttacks uint64) {
	whitePieces = board.GetWhitePieces()
	blackPieces = board.GetBlackPieces()
	allPieces = whitePieces | blackPieces
	whitePawns := board.GetBitboardOf(WhitePawn)
	blackPawns := board.GetBitboardOf(BlackPawn)
	whitePawnAttacks = ((whitePawns & ^FileMasks[FileA]) << 7) | ((whitePawns & ^FileMasks[FileH]) << 9)
	blackPawnAttacks = ((blackPawns & ^FileMasks[FileH]) >> 7) | ((blackPawns & ^FileMasks[FileA]) >> 9)
	return
}

// evaluateMobility returns one side's combined (MG+EG) mobility units. Retained
// only for the per-term symmetry unit test; the eval hot path uses the phase-split
// evaluateMobilityDelta. The MG+EG sum is a color-symmetric scalar, so the mirror
// test stays valid.
func evaluateMobility(board *Bitboard, color Color) int {
	var sliderAtt [64]uint64
	fillSliderAttacks(board, &sliderAtt)
	whitePieces, blackPieces, _, whitePawnAttacks, blackPawnAttacks := mobilitySetup(board)
	var mg, eg int
	if color == White {
		mg, eg = mobilityForSide(&sliderAtt, whitePieces|blackPawnAttacks,
			board.GetBitboardOf(WhiteKnight), board.GetBitboardOf(WhiteBishop),
			board.GetBitboardOf(WhiteRook), board.GetBitboardOf(WhiteQueen))
	} else {
		mg, eg = mobilityForSide(&sliderAtt, blackPieces|whitePawnAttacks,
			board.GetBitboardOf(BlackKnight), board.GetBitboardOf(BlackBishop),
			board.GetBitboardOf(BlackRook), board.GetBitboardOf(BlackQueen))
	}
	return mg + eg
}

// evaluateMobilityDelta returns White mobility minus Black mobility in one pass,
// computing the shared setup once (evaluateMobility was called per color, doing that
// setup twice). Bit-identical to evaluateMobility(W) - evaluateMobility(B), so search
// stays node-identical. sliderAtt is the per-eval slider-attack table (fillSliderAttacks),
// shared with king-safety so each slider's magic lookups happen once per eval.
func evaluateMobilityDelta(board *Bitboard, sliderAtt *[64]uint64) (mg, eg int) {
	whitePieces, blackPieces, _, whitePawnAttacks, blackPawnAttacks := mobilitySetup(board)
	wmg, weg := mobilityForSide(sliderAtt, whitePieces|blackPawnAttacks,
		board.GetBitboardOf(WhiteKnight), board.GetBitboardOf(WhiteBishop),
		board.GetBitboardOf(WhiteRook), board.GetBitboardOf(WhiteQueen))
	bmg, beg := mobilityForSide(sliderAtt, blackPieces|whitePawnAttacks,
		board.GetBitboardOf(BlackKnight), board.GetBitboardOf(BlackBishop),
		board.GetBitboardOf(BlackRook), board.GetBitboardOf(BlackQueen))
	return wmg - bmg, weg - beg
}

// getBishopAttacksBB returns bishop attack bitboard using magic bitboards - O(1)
func getBishopAttacksBB(sq int, occupied uint64) uint64 {
	return GetBishopAttacks(sq, occupied)
}

// getRookAttacksBB returns rook attack bitboard using magic bitboards - O(1)
func getRookAttacksBB(sq int, occupied uint64) uint64 {
	return GetRookAttacks(sq, occupied)
}

// --- Threats: static favorable attacks on enemy pieces ---------------------
// NGN otherwise sees tactics only through search; this prices them into the leaf
// eval. Each component fires ONLY on a favorable attack — an enemy minor/rook/
// queen hit by our pawn, an enemy rook/queen hit by our minor, an enemy queen hit
// by our rook, or any enemy piece we attack that they do not defend (hanging) — so
// the capture always gains material and there are no "bad capture" false positives.
// Values are hand-set centipawns (a threat is pressure, not realized material, so
// they sit below the piece value) and ride in the texel Fixed lump, not the tuned
// basis. The term is symmetric (White−Black) and tapered to zero in the endgame, so
// its main job is to cut NGN's middlegame over-optimism by pricing the OPPONENT's
// threats on NGN's pieces (the L1 loss analysis: 86/87 losses are eval over-optimism)
// while leaving the endgame search tree unperturbed.
var (
	threatByPawn       = 50 // any enemy minor/rook/queen attacked by our pawn
	threatMinorOnMajor = 38 // enemy rook/queen attacked by our knight/bishop
	threatRookOnQueen  = 38 // enemy queen attacked by our rook
	threatHangingMinor = 45 // enemy knight/bishop attacked by us and undefended
	threatHangingRook  = 68 // undefended attacked rook
	threatHangingQueen = 95 // undefended attacked queen
)

// sideAtt holds one side's attack spans split by attacker type for the threat term:
// pawn, minor (knight|bishop), rook, and the full attack set (all).
type sideAtt struct {
	pawn, minor, rook, all uint64
}

// computeSideAtt unions a side's per-piece attack sets. Sliders read the shared
// per-eval slider-attack table (fillSliderAttacks); knights/king use the static
// tables. Queen attacks fold into `all` only — no threat component keys off the
// queen as attacker (its only favorable target is an undefended piece, already
// covered by the hanging test over `all`).
func computeSideAtt(sliderAtt *[64]uint64, pawnAtt, knightBB, bishopBB, rookBB, queenBB, kingBB uint64) sideAtt {
	var knight, bishop, rook, queen uint64
	for bb := knightBB; bb != 0; bb &= bb - 1 {
		knight |= KnightAttacks[bits.TrailingZeros64(bb)]
	}
	for bb := bishopBB; bb != 0; bb &= bb - 1 {
		bishop |= sliderAtt[bits.TrailingZeros64(bb)]
	}
	for bb := rookBB; bb != 0; bb &= bb - 1 {
		rook |= sliderAtt[bits.TrailingZeros64(bb)]
	}
	for bb := queenBB; bb != 0; bb &= bb - 1 {
		queen |= sliderAtt[bits.TrailingZeros64(bb)]
	}
	var king uint64
	if kingBB != 0 { // malformed king-less FENs reach here via the stress tests
		king = KingAttacks[bits.TrailingZeros64(kingBB)]
	}
	minor := knight | bishop
	return sideAtt{pawn: pawnAtt, minor: minor, rook: rook, all: pawnAtt | minor | rook | queen | king}
}

// threatScore scores one side's threats against the enemy. att is the attacker's
// spans; the enemy* bitboards are the victim pieces; enemyAll is the enemy's full
// attack set (a square in it is defended, so a piece there is not hanging).
func threatScore(att sideAtt, enemyKnight, enemyBishop, enemyRook, enemyQueen, enemyAll uint64) int {
	enemyMinors := enemyKnight | enemyBishop
	enemyPieces := enemyMinors | enemyRook | enemyQueen
	score := PopCount(enemyPieces&att.pawn) * threatByPawn
	score += PopCount((enemyRook|enemyQueen)&att.minor) * threatMinorOnMajor
	score += PopCount(enemyQueen&att.rook) * threatRookOnQueen
	weak := att.all &^ enemyAll // attacked by us, not defended by them
	score += PopCount(enemyMinors&weak) * threatHangingMinor
	score += PopCount(enemyRook&weak) * threatHangingRook
	score += PopCount(enemyQueen&weak) * threatHangingQueen
	return score
}

// evaluateThreatsDelta returns White's threat pressure minus Black's, tapered
// MG-full -> EG-zero (phase/totalPhase). sliderAtt is the shared slider-attack
// table; pawn-attack spans are recomputed here with the same formula as
// mobilitySetup, so the term needs nothing precomputed beyond sliderAtt.
func evaluateThreatsDelta(board *Bitboard, sliderAtt *[64]uint64, phase int) int {
	whitePawns := board.GetBitboardOf(WhitePawn)
	blackPawns := board.GetBitboardOf(BlackPawn)
	wPawnAtt := ((whitePawns &^ FileMasks[FileA]) << 7) | ((whitePawns &^ FileMasks[FileH]) << 9)
	bPawnAtt := ((blackPawns &^ FileMasks[FileH]) >> 7) | ((blackPawns &^ FileMasks[FileA]) >> 9)
	wAtt := computeSideAtt(sliderAtt, wPawnAtt,
		board.GetBitboardOf(WhiteKnight), board.GetBitboardOf(WhiteBishop),
		board.GetBitboardOf(WhiteRook), board.GetBitboardOf(WhiteQueen), board.GetBitboardOf(WhiteKing))
	bAtt := computeSideAtt(sliderAtt, bPawnAtt,
		board.GetBitboardOf(BlackKnight), board.GetBitboardOf(BlackBishop),
		board.GetBitboardOf(BlackRook), board.GetBitboardOf(BlackQueen), board.GetBitboardOf(BlackKing))
	white := threatScore(wAtt,
		board.GetBitboardOf(BlackKnight), board.GetBitboardOf(BlackBishop),
		board.GetBitboardOf(BlackRook), board.GetBitboardOf(BlackQueen), bAtt.all)
	black := threatScore(bAtt,
		board.GetBitboardOf(WhiteKnight), board.GetBitboardOf(WhiteBishop),
		board.GetBitboardOf(WhiteRook), board.GetBitboardOf(WhiteQueen), wAtt.all)
	return (white - black) * phase / totalPhase
}

// Endgame draw-scaling — generalized from OCB-only to Counter 3.8's computeFactor family
// (CounterGo eval/evaluation.go:413). NGN previously scored dead-drawn endings (KBPvKN
// wrong-bishop, R-vs-R, single-pawn-up, KNNK, pawnless up-a-minor, ...) at FULL material and
// traded into them; this pulls them toward the draw. Modeled as Counter's DIVISOR d∈{1,2,4,8,16}
// applied to the LEADING side's material (d=1 no scale, d=16 heavy pull), expressed as the /64
// multiplier 64/d to fit the existing hook (and `raw*(64/d)/64 == raw/d` exactly for d|64). Safe
// by construction: it only ever REDUCES the magnitude of a position the leading side cannot
// realistically convert. Material weights are Counter's (minor 4 / rook 6 / queen 12) so the
// thresholds port verbatim; the OCB case (d=2) reproduces the prior ocbScale.
const (
	dfMinor = 4
	dfRook  = 6
	dfQueen = 12
)

// drawFactor = Counter's computeFactor: the draw divisor for the LEADING side (own) vs the other.
func drawFactor(ownForce, ownPawns, ownN, theirForce, theirPawns, theirN, theirB int, ocb bool) int {
	if ownForce >= dfQueen+dfRook {
		return 1
	}
	if ownPawns == 0 {
		if ownForce <= dfMinor {
			return 16
		}
		if ownForce == 2*dfMinor && ownN == 2 && theirPawns == 0 {
			return 16
		}
		if ownForce-theirForce <= dfMinor {
			return 4
		}
	} else if ownPawns == 1 {
		if ownForce <= dfMinor && theirN+theirB != 0 {
			return 8
		}
		if ownForce == theirForce && theirN+theirB != 0 {
			return 2
		}
	} else if ocb && ownPawns-theirPawns <= 2 {
		return 2
	}
	return 1
}

// drawScale returns the endgame draw-scale (out of 64; 64 = identity) for a WHITE-RELATIVE eval,
// applying drawFactor to whichever side is ahead.
func drawScale(board *Bitboard, whiteRel int) int {
	wN := PopCount(board.GetBitboardOf(WhiteKnight))
	wB := PopCount(board.GetBitboardOf(WhiteBishop))
	wR := PopCount(board.GetBitboardOf(WhiteRook))
	wQ := PopCount(board.GetBitboardOf(WhiteQueen))
	wP := PopCount(board.GetBitboardOf(WhitePawn))
	bN := PopCount(board.GetBitboardOf(BlackKnight))
	bB := PopCount(board.GetBitboardOf(BlackBishop))
	bR := PopCount(board.GetBitboardOf(BlackRook))
	bQ := PopCount(board.GetBitboardOf(BlackQueen))
	bP := PopCount(board.GetBitboardOf(BlackPawn))
	wF := dfMinor*(wN+wB) + dfRook*wR + dfQueen*wQ
	bF := dfMinor*(bN+bB) + dfRook*bR + dfQueen*bQ

	ocb := wB == 1 && bB == 1 && wN == 0 && bN == 0 && wR == 0 && bR == 0 && wQ == 0 && bQ == 0
	if ocb {
		wsq := int(bitScanForward(board.GetBitboardOf(WhiteBishop)))
		bsq := int(bitScanForward(board.GetBitboardOf(BlackBishop)))
		ocb = (wsq%8+wsq/8)%2 != (bsq%8+bsq/8)%2
	}

	var d int
	if whiteRel >= 0 {
		d = drawFactor(wF, wP, wN, bF, bP, bN, bB, ocb)
	} else {
		d = drawFactor(bF, bP, bN, wF, wP, wN, wB, ocb)
	}
	return 64 / d
}

// Enhanced evaluation function - always returns from white's perspective.
// Public direct evaluation takes a short process-model lease and evaluates a
// rebuilt board copy, so a Bitboard created before a model swap is safe and the
// caller's hidden accumulator is not mutated. It fails fast with
// ErrHCEModelBusy while an offline tuner owns the model mutation window.
func Evaluate(board *Bitboard) int {
	_ = mustAcquireHCEModelUse()
	defer releaseHCEModelUse()
	return evaluatePublicWhiteLeased(board)
}

func evaluatePublicWhiteLeased(board *Bitboard) int {
	work := board.copy()
	work.recomputeAccumulator()
	return evaluateWhiteLeased(&work, nil)
}

func evaluateWhiteLeased(board *Bitboard, pawns *pawnCache) int {
	return WrapEvaluation("Evaluate", func() int {
		raw := evaluateUnsafeWithPawnCache(board, pawns)
		return raw * drawScale(board, raw) / 64
	}, board)
}

// TempoBonus is a small advantage given to the side-to-move, reflecting
// that having the next move is itself a mild positional asset. Standard
// classical-eval value across most engines is ~10–15 cp.
const TempoBonus = 10

// EvaluateForPlayer returns evaluation from the given player's perspective,
// including the tempo bonus for being the side-to-move. It uses one direct
// model lease rather than nesting through Evaluate.
func EvaluateForPlayer(board *Bitboard, player Color) int {
	_ = mustAcquireHCEModelUse()
	defer releaseHCEModelUse()
	whiteEval := evaluatePublicWhiteLeased(board)
	if player == White {
		return whiteEval + TempoBonus
	}
	return -whiteEval + TempoBonus
}

// --- Per-engine HCE evaluation state ---
// The full cache stores the white-POV, draw-scaled value by full position hash.
// POV, TempoBonus, and rule-50 attenuation remain outside it. Each SearchEngine
// owns one evaluator so search instances cannot poison each other's eval state.
const evalCacheBits = 20

type evalCacheEntry struct {
	key uint64
	val int32
	_   int32
}

type hceEvaluator struct {
	full           [1 << evalCacheBits]evalCacheEntry
	pawns          pawnCache
	seenGeneration uint64
}

func (e *hceEvaluator) clearForGeneration(generation uint64) {
	clear(e.full[:])
	clear(e.pawns[:])
	e.seenGeneration = generation
}

// SearchSTM is the ordinary search score route. Its operation order is the
// legacy cached-white value, rule-50 attenuation, side-to-move POV, then Tempo.
func (e *hceEvaluator) SearchSTM(pos *Position) int {
	h := pos.Hash()
	entry := &e.full[h&(1<<evalCacheBits-1)]
	var white int
	if entry.key == h && h != 0 {
		white = int(entry.val)
	} else {
		white = evaluateWhiteLeased(&pos.Board, &e.pawns)
		entry.key = h
		entry.val = int32(white)
	}
	white = white * (FiftyMoveDampBudget - int(pos.HalfMoveClock)) / FiftyMoveDampBudget
	if pos.Turn() == White {
		return white + TempoBonus
	}
	return -white + TempoBonus
}

// LegacyUndampedSTM preserves the three alphaBeta safety-return semantics: a
// fresh full HCE value (the pawn-count cache may hit), draw scaling, POV, Tempo,
// and no full-cache lookup or halfmove attenuation.
func (e *hceEvaluator) LegacyUndampedSTM(pos *Position) int {
	white := evaluateWhiteLeased(&pos.Board, &e.pawns)
	if pos.Turn() == White {
		return white + TempoBonus
	}
	return -white + TempoBonus
}

// FiftyMoveDampBudget scales the static eval toward a draw as the halfmove clock
// climbs toward the 50-move rule (hmc=100): factor = (BUDGET-hmc)/BUDGET, so 1.0
// at hmc=0 and ~0.61 at hmc=100. Anti-optimism — a stale position drifting toward
// a 50-move draw should not read as a clean win (the search otherwise ignores the
// clock entirely; drawScale only knows material). Peer-standard (Caissa/SF/Ethereal).
// 256 = power-of-two divisor, and since hmc is uint8 (<=255) the factor can never go
// negative, so no clamp/branch is needed on the hot eval path. Applied OUTSIDE the
// eval cache (depends on hmc, not the board) alongside POV/Tempo, so a board-pure
// cached value is never poisoned, and it only ever sees a static eval (never a mate
// score), so damping cannot corrupt mate distances.
const FiftyMoveDampBudget = 256

// EvaluateForPlayerCached retains the package compatibility API using the
// default SearchEngine's private evaluator and serialized session lifetime.
func EvaluateForPlayerCached(pos *Position) int {
	return defaultSearchEngine.evaluateForPlayerCached(pos)
}

// Tunable eval-term weights — hand-set magnitudes (never data-tuned) exposed as
// package vars so the Texel tuner (cmd/texel) can fit them to game results. The
// three pawn-structure values are direct centipawn weights; the six term
// weights are percentage multipliers (100 = identity), so these defaults
// reproduce the original eval exactly. Tuned values are pasted back here.
//
// Each weight is TAPERED MG->EG (like the PeSTO core): the value below is the
// middlegame end, its *EG twin the endgame end, blended by game phase via taperW.
// The EG defaults equal the MG values, so a fresh checkout reproduces the old flat
// eval bit-for-bit; the tuner then pulls them apart so a passed pawn / king-safety
// unit can be worth more (or less) in the endgame than the middlegame.
var (
	passedPawnBonus      = 10
	blockedPasserPenalty = 8 // discount a passer blockaded by an enemy piece (L1 over-optimism fix)
	doubledPawnPenalty   = 10
	isolatedPawnPenalty  = 10
	backwardPawnPenalty  = 8 // pawn whose stop square is enemy-pawn-controlled and outside its own pawns' attack span
	pawnChainWeight      = 76
	mobilityWeight       = 181
	rookOpenWeight       = 87
	outpostWeight        = 84
	kingSafetyWeight     = 106
	kingActivityWeight   = 21 // ACPL coordinate-descent (2026-06-16): 29->21, NGN over-valued MG king activity

	passedPawnBonusEG      = 44 // ACPL coordinate-descent (2026-06-16): 36->44, NGN under-valued EG passers (opp-passer optimism)
	blockedPasserPenaltyEG = 12 // ACPL coordinate-descent (2026-06-16): 20->12, over-penalized blockaded EG passers
	doubledPawnPenaltyEG   = 23
	isolatedPawnPenaltyEG  = 14
	backwardPawnPenaltyEG  = 12 // backward pawns are a chronic endgame target (cannot be defended, cannot advance)
	pawnChainWeightEG      = 77
	mobilityWeightEG       = 180
	rookOpenWeightEG       = 86
	outpostWeightEG        = 85
	kingSafetyWeightEG     = 84
	kingActivityWeightEG   = 29
)

// taperW blends a middlegame/endgame weight pair by game phase (phase=totalPhase
// all-MG, phase=0 all-EG), matching the PeSTO core's taper. When mg==eg it returns
// exactly that value, so equal MG/EG twins are behavior-neutral.
func taperW(mg, eg, phase int) int { return (mg*phase + eg*(totalPhase-phase)) / totalPhase }

// --- Pawn-structure evaluation cache (N6) ---
// passed / doubled / isolated / pawn-chain are pure functions of the two pawn
// bitboards, so memoize their RAW counts keyed by (whitePawns, blackPawns) with
// a full-key verify — collision-proof, hence node-identical. Raw counts (not the
// weighted contribution) are stored so tuning the weights never stales the cache.
// Each SearchEngine owns this cache. Texel and direct parallel evaluation pass
// nil explicitly, so they neither share nor mutate a search worker's entries.
const (
	pawnCacheSize = 1 << 16
	pawnCacheMask = pawnCacheSize - 1
)

type pawnCacheEntry struct {
	wp, bp             uint64
	wPassed, bPassed   int16
	wPassedW, bPassedW int16
	wDoubled, bDoubled int16
	wIso, bIso         int16
	chainW, chainB     int16
	valid              bool
}

type pawnCache [pawnCacheSize]pawnCacheEntry

func pawnCacheIndex(wp, bp uint64) uint64 {
	h := wp ^ (bp*0x9E3779B97F4A7C15 + 0x7F4A7C159E3779B9)
	h ^= h >> 29
	h *= 0xBF58476D1CE4E5B9
	h ^= h >> 32
	return h & pawnCacheMask
}

// pawnStructureCounts returns the raw passed/doubled/isolated/chain counts plus the
// rank-weighted passed-pawn sums (wPassedW/bPassedW) for both colors, memoized on the
// pawn bitboards. board's pawn bitboards MUST equal (wp, bp) — the caller passes them
// so the key and the compute path agree.
func pawnStructureCounts(cache *pawnCache, board *Bitboard, wp, bp uint64) (wPassed, bPassed, wPassedW, bPassedW, wDoubled, bDoubled, wIso, bIso, chainW, chainB int) {
	if cache != nil {
		e := &(*cache)[pawnCacheIndex(wp, bp)]
		if e.valid && e.wp == wp && e.bp == bp {
			return int(e.wPassed), int(e.bPassed), int(e.wPassedW), int(e.bPassedW), int(e.wDoubled), int(e.bDoubled),
				int(e.wIso), int(e.bIso), int(e.chainW), int(e.chainB)
		}
	}
	wPassed = countPassedPawnsFast(wp, bp, White)
	bPassed = countPassedPawnsFast(bp, wp, Black)
	wPassedW = passedRankWeightedSum(wp, bp, White)
	bPassedW = passedRankWeightedSum(bp, wp, Black)
	for file := 0; file < 8; file++ {
		wDoubled += countDoubledPawns(board, White, file)
		bDoubled += countDoubledPawns(board, Black, file)
	}
	wIso = countIsolatedPawns(board, White)
	bIso = countIsolatedPawns(board, Black)
	chainW = evaluatePawnChains(board, White)
	chainB = evaluatePawnChains(board, Black)
	if cache != nil {
		(*cache)[pawnCacheIndex(wp, bp)] = pawnCacheEntry{
			wp: wp, bp: bp,
			wPassed: int16(wPassed), bPassed: int16(bPassed),
			wPassedW: int16(wPassedW), bPassedW: int16(bPassedW),
			wDoubled: int16(wDoubled), bDoubled: int16(bDoubled),
			wIso: int16(wIso), bIso: int16(bIso),
			chainW: int16(chainW), chainB: int16(chainB),
			valid: true,
		}
	}
	return
}

// evaluateUnsafe = cheap CORE (material/PST + pawn structure) + expensive EXTRAS
// (mobility, rook, outposts, king). Split so qsearch can lazily skip the extras
// when the core alone already proves a stand-pat fail-high (evaluateLazyStandPat).
// The full path is bit-for-bit identical to the old monolithic version.
// northFill / southFill smear a bitboard toward higher / lower ranks — used to build
// each side's pawn attack span for backward-pawn detection.
func northFill(b uint64) uint64 { b |= b << 8; b |= b << 16; b |= b << 32; return b }
func southFill(b uint64) uint64 { b |= b >> 8; b |= b >> 16; b |= b >> 32; return b }

// backwardPawnCounts returns (white, black) backward-pawn counts. A pawn is backward
// when the square in front of it is attacked by an enemy pawn AND lies outside its own
// side's pawn attack span (no friendly pawn defends that stop square now or as it
// advances) — so it can neither advance safely nor be supported, a chronic weakness the
// pawn-structure terms otherwise miss. Set-wise (Stockfish/CPW), no per-pawn loop.
func backwardPawnCounts(wp, bp uint64) (int, int) {
	wpAtt := ((wp &^ FileMasks[FileA]) << 7) | ((wp &^ FileMasks[FileH]) << 9)
	bpAtt := ((bp &^ FileMasks[FileH]) >> 7) | ((bp &^ FileMasks[FileA]) >> 9)
	wBackward := (wp << 8) & bpAtt &^ northFill(wpAtt)
	bBackward := (bp >> 8) & wpAtt &^ southFill(bpAtt)
	return PopCount(wBackward), PopCount(bBackward)
}

func evaluateUnsafe(board *Bitboard) int {
	return evaluateUnsafeWithPawnCache(board, nil)
}

func evaluateUnsafeWithPawnCache(board *Bitboard, pawns *pawnCache) (eval int) {
	core, phase := evalCoreWhiteWithPawnCache(board, pawns)
	return core + evalExtrasWhite(board, phase)
}

func evalCoreWhite(board *Bitboard) (eval, phase int) {
	return evalCoreWhiteWithPawnCache(board, nil)
}

// evalCoreWhiteWithPawnCache is the CHEAP half (white POV): PeSTO tapered
// material+PST plus optionally memoized pawn-structure terms. It also returns
// the game phase for the extras.
func evalCoreWhiteWithPawnCache(board *Bitboard, pawns *pawnCache) (eval, phase int) {
	// Core: PeSTO tapered material + piece-square tables.
	mgScore, egScore, ph := evaluatePeSTO(board)
	phase = ph
	eval = (mgScore*phase + egScore*(totalPhase-phase)) / totalPhase

	whitePawns := board.GetBitboardOf(WhitePawn)
	blackPawns := board.GetBitboardOf(BlackPawn)

	// passed / doubled / isolated / pawn-chain are pure functions of the pawn
	// bitboards — fetch their raw counts from the memoized cache (N6) and apply
	// the weights here. Same arithmetic, same order as the inline version, so the
	// result is bit-identical (node-identical); only the recomputation is saved.
	_, _, whitePassedW, blackPassedW, whiteDoubled, blackDoubled, whiteIsolated, blackIsolated, chainWhite, chainBlack :=
		pawnStructureCounts(pawns, board, whitePawns, blackPawns)
	eval += (whitePassedW - blackPassedW) * taperW(passedPawnBonus, passedPawnBonusEG, phase) // passed pawns (rank-weighted), not captured by PST
	// Blockaded-passer discount: the rank-weighted bonus above credits a stalled passer
	// fully, but one whose stop square is held by an enemy piece is going nowhere — the
	// L1 over-optimism the loss analysis flagged. Subtract a phase-tapered penalty each.
	blackNonPawns := board.GetBlackPieces() &^ blackPawns
	whiteNonPawns := board.GetWhitePieces() &^ whitePawns
	whitePassers := passersOf(whitePawns, blackPawns, White)
	blackPassers := passersOf(blackPawns, whitePawns, Black)
	whiteBlocked := passedBlockedCount(whitePassers, blackNonPawns, White)
	blackBlocked := passedBlockedCount(blackPassers, whiteNonPawns, Black)
	eval -= (whiteBlocked - blackBlocked) * taperW(blockedPasserPenalty, blockedPasserPenaltyEG, phase)
	// King-vs-passer endgame discount: a passer the enemy king can reach is worth less
	// than its rank-weighted bonus credits (the endgame half of the same L1 own-passer
	// optimism). EG-only via taperW(0, ...) so it never nudges middlegame king walks.
	whiteKingSq := trailingZeros(board.GetBitboardOf(WhiteKing))
	blackKingSq := trailingZeros(board.GetBitboardOf(BlackKing))
	whiteKingDisc := passedKingDiscount(whitePassers, blackKingSq, White)
	blackKingDisc := passedKingDiscount(blackPassers, whiteKingSq, Black)
	eval -= (whiteKingDisc - blackKingDisc) * taperW(0, kingPasserBlockEG, phase)
	// Rook-behind-passer discount (Tarrasch): a passer with an enemy rook behind it on
	// its file is restrained, worth less than its rank bonus credits. EG-only.
	whiteRookBehind := passedRookBehindCount(whitePassers, board.GetBitboardOf(BlackRook), White)
	blackRookBehind := passedRookBehindCount(blackPassers, board.GetBitboardOf(WhiteRook), Black)
	eval -= (whiteRookBehind - blackRookBehind) * taperW(0, rookBehindPasserEG, phase)
	eval -= (whiteDoubled - blackDoubled) * taperW(doubledPawnPenalty, doubledPawnPenaltyEG, phase)     // doubled penalty
	eval -= (whiteIsolated - blackIsolated) * taperW(isolatedPawnPenalty, isolatedPawnPenaltyEG, phase) // isolated penalty
	eval += (chainWhite - chainBlack) * taperW(pawnChainWeight, pawnChainWeightEG, phase) / 100         // connected/supported pawn chains
	wBackward, bBackward := backwardPawnCounts(whitePawns, blackPawns)
	eval -= (wBackward - bBackward) * taperW(backwardPawnPenalty, backwardPawnPenaltyEG, phase) // backward-pawn penalty
	return eval, phase
}

// evalExtrasWhite is the EXPENSIVE half (white POV): mobility, rook-on-open-file,
// outposts, and king safety/activity — the ~16% of CPU the lazy path can skip.
func evalExtrasWhite(board *Bitboard, phase int) (eval int) {
	whitePawns := board.GetBitboardOf(WhitePawn)
	blackPawns := board.GetBitboardOf(BlackPawn)

	// Bishop-pair bonus REMOVED 2026-06-02: a fixed-nodes SPRT (ngn_l5 vs base, 800 games)
	// scored removal at +20.4 ELO [-4,+45] — the term was net-harmful (it double-counted the
	// PeSTO bishop values), so dropping it is both a strength and a speed win. evaluateBishopPair
	// is retained only for its unit tests.

	// Slider-attack table, filled once per eval: mobility and king-safety both
	// read it instead of each recomputing the same bishop/rook/queen magic lookups.
	var sliderAtt [64]uint64
	fillSliderAttacks(board, &sliderAtt)

	// Mobility — per-count concave MG/EG tables (centipawns), tapered by phase.
	mgMob, egMob := evaluateMobilityDelta(board, &sliderAtt)
	eval += taperW(mgMob, egMob, phase)

	// Rook on open file bonus.
	eval += (evaluateRookOnOpenFile(board, White) - evaluateRookOnOpenFile(board, Black)) * taperW(rookOpenWeight, rookOpenWeightEG, phase) / 100

	// Knight/bishop outposts (squares supported by own pawn, unattackable by
	// enemy pawns, in enemy territory). PeSTO PSTs don't know pawn structure.
	whiteKnights := board.GetBitboardOf(WhiteKnight)
	whiteBishops := board.GetBitboardOf(WhiteBishop)
	blackKnights := board.GetBitboardOf(BlackKnight)
	blackBishops := board.GetBitboardOf(BlackBishop)
	eval += (evaluateOutpostSquares(whiteKnights, whiteBishops, whitePawns, blackPawns, White) -
		evaluateOutpostSquares(blackKnights, blackBishops, blackPawns, whitePawns, Black)) * taperW(outpostWeight, outpostWeightEG, phase) / 100

	// King safety: middlegame through late-middlegame. Gated phase>=6 (C4): at
	// phase==6 (e.g. Q+2 minors / 1Q+1R) a queen can still attack, so safety is
	// the right term — the old phase>=7 gate left phase 6 with NEITHER safety nor
	// activity (a ~144cp eval cliff at the 6/7 boundary, inside the measured 7-11 hole).
	if phase >= 6 {
		eval += (evaluateKingSafety(board, White, &sliderAtt) - evaluateKingSafety(board, Black, &sliderAtt)) * taperW(kingSafetyWeight, kingSafetyWeightEG, phase) / 100
	} else if phase < 6 {
		// Deep endgame only: encourage king centralization and supporting pawns.
		// Wider phase windows interfered with tactical search where leaves drop
		// below phase=12 mid-combination.
		eval += evaluateKingActivity(board) * taperW(kingActivityWeight, kingActivityWeightEG, phase) / 100
	}

	// Threats: static favorable attacks on enemy pieces (see evaluateThreatsDelta).
	// Symmetric and MG-tapered, so it prices the opponent's threats on our pieces
	// (cutting middlegame over-optimism) without perturbing the endgame tree.
	eval += evaluateThreatsDelta(board, &sliderAtt, phase)
	return eval
}

// qsearchLazyMargin bounds the magnitude of evalExtrasWhite. In qsearch, if the
// cheap core alone clears beta by more than this, the expensive extras cannot pull
// the score back below beta, so the stand-pat fail-high is taken without computing
// them. A LARGE value makes the cut provably node-identical (it fires only when the
// extras cannot matter); shrinking it skips the expensive eval on more nodes at a
// small exactness cost — SPRT the magnitude.
// SHELVED 2026-06-02: 150 was unsound (extras reach +-199cp; a concrete wrong cut was
// found, and it tested net-neutral at fixed time, G121 elo +0.0) — the CPU win it chased
// is captured node-identically by the slider-attack dedup refactor instead. Pinned at
// 100000 = lazy-out never fires = node-identical to the full eval.
var qsearchLazyMargin = 100000

// evaluateLazyStandPat returns the side-to-move stand-pat eval, but when the cheap
// core already proves a fail-high (coreStm - margin >= beta) it returns failHigh
// without computing the expensive extras. POV+tempo mirror EvaluateForPlayer.
func evaluateLazyStandPat(board *Bitboard, player Color, beta int) (standPat int, failHigh bool) {
	whiteCore, phase := evalCoreWhite(board)
	coreStm := whiteCore
	if player != White {
		coreStm = -coreStm
	}
	coreStm += TempoBonus
	if coreStm-qsearchLazyMargin >= beta {
		return 0, true
	}
	full := whiteCore + evalExtrasWhite(board, phase)
	full = full * drawScale(board, full) / 64 // endgame draw-scaling — mirror Evaluate so main search and qsearch agree
	if player != White {
		full = -full
	}
	return full + TempoBonus, false
}
