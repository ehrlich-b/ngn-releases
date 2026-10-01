package engine

// The evaluator deliberately keeps its numbers small and hand-designed.  A
// table entry is already White-relative: a white entry is positive and the
// corresponding black entry on the vertically reflected square is its
// negative.  This makes colour/board reflection an exact algebraic property,
// rather than something dependent on an opening or training corpus.

var mgPST [13][64]int
var egPST [13][64]int
var piecePhaseInc [13]int

// The phase increments are an intentionally coarse description of how much
// non-pawn material remains.  Eight initial minor pieces contribute sixteen units, four rooks contribute
// twelve, and two queens contribute ten: 16+12+10 = 38.
const totalPhase int = 38

func init() {
	RebuildPST()
}

// RebuildPST deterministically regenerates every evaluation entry.  The
// values are formulas over file/rank coordinates, not a copied square table.
// It is safe to call this before constructing a board; it does not touch any
// board value.
func RebuildPST() {
	mgPST = [13][64]int{}
	egPST = [13][64]int{}
	piecePhaseInc = [13]int{}

	for piece := Piece(WhitePawn); piece <= BlackKing; piece++ {
		typeOfPiece := piece.Type()
		piecePhaseInc[piece] = phaseIncrement(typeOfPiece)
		sign := 1
		if piece.Color() == Black {
			sign = -1
		}

		for square := 0; square < 64; square++ {
			file := square & 7
			rank := square >> 3
			forward := rank
			if piece.Color() == Black {
				forward = 7 - rank
			}

			mgPST[piece][square] = sign * analyticSquareValue(typeOfPiece, file, rank, forward, false)
			egPST[piece][square] = sign * analyticSquareValue(typeOfPiece, file, rank, forward, true)
		}
	}
}

func phaseIncrement(typeOfPiece PieceType) int {
	switch typeOfPiece {
	case Knight, Bishop:
		return 2
	case Rook:
		return 3
	case Queen:
		return 5
	default:
		return 0
	}
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// centerScore is zero at a corner and largest near the four central squares.
// The doubled coordinates avoid fractional distances while preserving
// reflection symmetry on both axes.
func centerScore(file, rank int) int {
	return 14 - absInt(2*file-7) - absInt(2*rank-7)
}

// analyticSquareValue contains the complete fixed positional/material part
// of the evaluator.  The round values are chosen as a compact scale: a pawn
// is the 100-unit baseline, a minor is about three baselines, a rook about
// five, and a queen about ten.  The small coordinate terms are intentionally
// much less important than material.
func analyticSquareValue(typeOfPiece PieceType, file, rank, forward int, endgame bool) int {
	center := centerScore(file, rank)
	if endgame {
		switch typeOfPiece {
		case Pawn:
			return 100 + 8*forward + center/2
		case Knight:
			return 330 + 2*center + forward
		case Bishop:
			return 350 + 2*center + forward
		case Rook:
			return 540 + forward + 2*center
		case Queen:
			return 985 + 2*center
		case King:
			return 2 + 3*center
		default:
			return 0
		}
	}

	switch typeOfPiece {
	case Pawn:
		return 100 + 4*forward + center/3
	case Knight:
		return 315 + 4*center
	case Bishop:
		return 335 + 2*center + forward
	case Rook:
		return 510 + 2*forward + center
	case Queen:
		return 975 + center
	case King:
		// A middlegame king prefers a less central square.  The dynamic
		// pawn-shield term below supplies the small safety adjustment.
		return 10 - 3*center
	default:
		return 0
	}
}

// evaluateRaw returns a White-positive, tempo-free score.  It rebuilds its
// additive part from the mailbox instead of trusting cached accumulators, so
// a stale cache cannot change the result and the input is never repaired.
func evaluateRaw(board *Bitboard) int {
	if board == nil {
		return 0
	}

	var pawnBits [2]uint64
	var pawnFiles [2][8]int
	var kingBits [2]uint64
	var mg, eg int
	phase := 0
	pawnCount := 0
	minorCount := 0
	heavyCount := 0

	for square := 0; square < 64; square++ {
		piece := board.mailbox[square]
		if !validPiece(piece) {
			continue
		}

		mg += mgPST[piece][square]
		eg += egPST[piece][square]
		phase += piecePhaseInc[piece]

		colorIndex := int(piece.Color())
		mask := uint64(1) << uint(square)
		switch piece.Type() {
		case Pawn:
			pawnBits[colorIndex] |= mask
			pawnFiles[colorIndex][square&7]++
			pawnCount++
		case Knight, Bishop:
			minorCount++
		case Rook, Queen:
			heavyCount++
		case King:
			kingBits[colorIndex] |= mask
		}
	}

	// These are the standard insufficient-material shapes requested by the
	// contract.  Their location should not manufacture a numerical advantage.
	if pawnCount == 0 && heavyCount == 0 && minorCount <= 1 {
		return 0
	}

	if phase < 0 {
		phase = 0
	} else if phase > totalPhase {
		phase = totalPhase
	}

	addPawnTerms(&mg, &eg, pawnBits, pawnFiles)
	addKingTerms(&mg, &eg, pawnBits, kingBits)

	// A linear blend is enough for this deliberately small evaluator.  Both
	// endpoints are signed together, so negation remains exact at every phase.
	score := (mg*phase + eg*(totalPhase-phase)) / totalPhase
	if score >= 25000 {
		return 24999
	}
	if score <= -25000 {
		return -24999
	}
	return score
}

func addPawnTerms(mg, eg *int, pawnBits [2]uint64, pawnFiles [2][8]int) {
	for square := 0; square < 64; square++ {
		piece := NoPiece
		if pawnBits[0]&(uint64(1)<<uint(square)) != 0 {
			piece = BlackPawn
		} else if pawnBits[1]&(uint64(1)<<uint(square)) != 0 {
			piece = WhitePawn
		}
		if piece == NoPiece {
			continue
		}

		colorIndex := int(piece.Color())
		sign := -1
		if piece.Color() == White {
			sign = 1
		}
		file := square & 7
		rank := square >> 3
		forward := rank
		if piece.Color() == Black {
			forward = 7 - rank
		}

		// A pawn sharing neither neighbouring file is isolated.  A second
		// pawn on its file receives a small doubled-pawn deduction.
		adjacent := false
		if file > 0 && pawnFiles[colorIndex][file-1] != 0 {
			adjacent = true
		}
		if file < 7 && pawnFiles[colorIndex][file+1] != 0 {
			adjacent = true
		}
		if !adjacent {
			*mg += sign * -6
			*eg += sign * -4
		}
		if pawnFiles[colorIndex][file] > 1 {
			doubled := pawnFiles[colorIndex][file] - 1
			*mg += sign * (-4 * doubled)
			*eg += sign * (-3 * doubled)
		}

		// A pawn immediately supported from behind is a small structural
		// asset.  The rank check prevents a file edge from wrapping.
		behindRank := rank - 1
		if piece.Color() == Black {
			behindRank = rank + 1
		}
		if behindRank >= 0 && behindRank < 8 {
			if file > 0 && pawnBits[colorIndex]&(uint64(1)<<uint(behindRank*8+file-1)) != 0 {
				*mg += sign * 3
				*eg += sign * 4
			}
			if file < 7 && pawnBits[colorIndex]&(uint64(1)<<uint(behindRank*8+file+1)) != 0 {
				*mg += sign * 3
				*eg += sign * 4
			}
		}

		// Passed-pawn detection considers the same and adjacent files only
		// in the pawn's forward half of the board.
		passed := true
		firstRank, lastRank, step := rank+1, 8, 1
		if piece.Color() == Black {
			firstRank, lastRank, step = rank-1, -1, -1
		}
		for scanRank := firstRank; scanRank != lastRank && passed; scanRank += step {
			for scanFile := file - 1; scanFile <= file+1; scanFile++ {
				if scanFile < 0 || scanFile >= 8 {
					continue
				}
				if pawnBits[1-colorIndex]&(uint64(1)<<uint(scanRank*8+scanFile)) != 0 {
					passed = false
					break
				}
			}
		}
		if passed {
			*mg += sign * (4 + 2*forward)
			*eg += sign * (8 + 3*forward)
		}
	}
}

func addKingTerms(mg, eg *int, pawnBits [2]uint64, kingBits [2]uint64) {
	for colorIndex := 0; colorIndex < 2; colorIndex++ {
		sign := -1
		if colorIndex == int(White) {
			sign = 1
		}
		for kingSquare := 0; kingSquare < 64; kingSquare++ {
			if kingBits[colorIndex]&(uint64(1)<<uint(kingSquare)) == 0 {
				continue
			}
			kingFile := kingSquare & 7
			kingRank := kingSquare >> 3
			shield := 0
			for pawnSquare := 0; pawnSquare < 64; pawnSquare++ {
				if pawnBits[colorIndex]&(uint64(1)<<uint(pawnSquare)) == 0 {
					continue
				}
				pawnFile := pawnSquare & 7
				if absInt(pawnFile-kingFile) > 1 {
					continue
				}
				pawnRank := pawnSquare >> 3
				relativeRank := pawnRank - kingRank
				if colorIndex == int(Black) {
					relativeRank = kingRank - pawnRank
				}
				if relativeRank == 1 || relativeRank == 2 {
					shield++
				}
			}
			*mg += sign * 7 * shield
			*eg += sign * 2 * shield
		}
	}
}

// gamePhase returns the mailbox-rebuilt phase, clipped to the declared
// initial maximum.  Pawns and kings intentionally contribute no phase.
func gamePhase(board *Bitboard) int {
	if board == nil {
		return 0
	}
	phase := 0
	for square := 0; square < 64; square++ {
		piece := board.mailbox[square]
		if validPiece(piece) {
			phase += piecePhaseInc[piece]
		}
	}
	if phase < 0 {
		return 0
	}
	if phase > totalPhase {
		return totalPhase
	}
	return phase
}

// isEndgame uses an explicit material description rather than a tuned score:
// no queens and no more than two rooks leaves only sparse heavy material or
// minor pieces and is treated as an endgame.
func isEndgame(board *Bitboard) bool {
	if board == nil {
		return true
	}
	queens, rooks := 0, 0
	for square := 0; square < 64; square++ {
		piece := board.mailbox[square]
		if !validPiece(piece) {
			continue
		}
		switch piece.Type() {
		case Queen:
			queens++
		case Rook:
			rooks++
		}
	}
	return queens == 0 && rooks <= 2
}

// hasNonPawnMaterial reports whether color owns a knight, bishop, rook or
// queen.  Pawns and kings are deliberately excluded.
func hasNonPawnMaterial(board *Bitboard, color Color) bool {
	if board == nil || (color != White && color != Black) {
		return false
	}
	for square := 0; square < 64; square++ {
		piece := board.mailbox[square]
		if !validPiece(piece) || piece.Color() != color {
			continue
		}
		switch piece.Type() {
		case Knight, Bishop, Rook, Queen:
			return true
		}
	}
	return false
}
