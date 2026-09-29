package sf18big

import (
	"errors"
	"fmt"
	"math/bits"
	"slices"

	base "github.com/ehrlich-b/ngn/nnue"
)

const maxActiveThreats = 128

var ErrEvaluation = errors.New("invalid Stockfish 18 BIG evaluation input")

// ThreatIndexList is the allocation-free logical-order FullThreats feature set
// for one perspective. Only Indices[:Count] is live.
type ThreatIndexList struct {
	Count   uint8
	Indices [maxActiveThreats]uint32
}

// ThreatDiff is the simple sorted-set reference transition used by the N1b
// feasibility gate. Production fused/double-update machinery is deliberately
// outside this contract.
type ThreatDiff struct {
	Removed         ThreatIndexList
	Added           ThreatIndexList
	RequiresRefresh bool
}

type boardPiece struct {
	piece base.PieceType
	color base.Color
	set   bool
}

var threatTargetMap = [6][6]int8{
	{0, 1, -1, 2, -1, -1},
	{0, 1, 2, 3, 4, 5},
	{0, 1, 2, 3, -1, 4},
	{0, 1, 2, 3, -1, 4},
	{0, 1, 2, 3, 4, 5},
	{0, 1, 2, 3, -1, -1},
}

var threatValidTargets = [6]int{6, 12, 10, 10, 12, 8}

type threatPieceOffsets struct {
	perTarget int
	base      int
	from      [64]int
}

var threatOffsets = buildThreatOffsets()

const invalidThreatIndexBase = int32(threatInputFeatures)

type threatIndexTables struct {
	base [12][12][2]int32
	rank [12][64][64]uint8
}

var (
	pseudoThreatTable = buildPseudoThreatTable()
	threatIndexTable  = buildThreatIndexTables()
	threatRayTable    = buildThreatRayTable()
)

func buildPseudoThreatTable() [2][6][64]uint64 {
	var table [2][6][64]uint64
	for color := base.White; color <= base.Black; color++ {
		for piece := base.Pawn; piece <= base.King; piece++ {
			for square := base.Square(0); square < 64; square++ {
				table[color][piece][square] = pseudoThreatAttacks(piece, color, square)
			}
		}
	}
	return table
}

func buildThreatIndexTables() threatIndexTables {
	var table threatIndexTables
	for attackerColor := base.White; attackerColor <= base.Black; attackerColor++ {
		for attackerPiece := base.Pawn; attackerPiece <= base.King; attackerPiece++ {
			attackerID := int(attackerColor)*6 + int(attackerPiece)
			offsets := threatOffsets[attackerColor][attackerPiece]
			for attackedColor := base.White; attackedColor <= base.Black; attackedColor++ {
				for attackedPiece := base.Pawn; attackedPiece <= base.King; attackedPiece++ {
					attackedID := int(attackedColor)*6 + int(attackedPiece)
					group := int(threatTargetMap[attackerPiece][attackedPiece])
					for fromLessThanTo := 0; fromLessThanTo < 2; fromLessThanTo++ {
						value := invalidThreatIndexBase
						enemy := attackerColor != attackedColor
						semiExcluded := attackerPiece == attackedPiece && (enemy || attackerPiece != base.Pawn)
						if group >= 0 && !(semiExcluded && fromLessThanTo != 0) {
							targetGroup := group + int(attackedColor)*(threatValidTargets[attackerPiece]/2)
							value = int32(offsets.base + targetGroup*offsets.perTarget)
						}
						table.base[attackerID][attackedID][fromLessThanTo] = value
					}
				}
			}
			for from := base.Square(0); from < 64; from++ {
				attacks := pseudoThreatTable[attackerColor][attackerPiece][from]
				for to := base.Square(0); to < 64; to++ {
					table.rank[attackerID][from][to] = 255
					toBit := uint64(1) << to
					if attacks&toBit != 0 {
						table.rank[attackerID][from][to] = uint8(bits.OnesCount64(attacks & (toBit - 1)))
					}
				}
			}
		}
	}
	return table
}

const (
	threatNorth = iota
	threatSouth
	threatEast
	threatWest
	threatNorthEast
	threatNorthWest
	threatSouthEast
	threatSouthWest
	threatRayDirections
)

func buildThreatRayTable() [threatRayDirections][64]uint64 {
	deltas := [threatRayDirections][2]int{
		{0, 1}, {0, -1}, {1, 0}, {-1, 0},
		{1, 1}, {-1, 1}, {1, -1}, {-1, -1},
	}
	var table [threatRayDirections][64]uint64
	for direction := range deltas {
		for from := base.Square(0); from < 64; from++ {
			table[direction][from] = slidingThreatAttacks(from, 0, deltas[direction:direction+1])
		}
	}
	return table
}

func buildThreatOffsets() [2][6]threatPieceOffsets {
	var result [2][6]threatPieceOffsets
	baseOffset := 0
	for color := base.White; color <= base.Black; color++ {
		for piece := base.Pawn; piece <= base.King; piece++ {
			perTarget := 0
			var fromOffsets [64]int
			for square := base.Square(0); square < 64; square++ {
				fromOffsets[square] = perTarget
				if piece != base.Pawn || square >= 8 && square < 56 {
					perTarget += popcount64(pseudoThreatAttacks(piece, color, square))
				}
			}
			result[color][piece] = threatPieceOffsets{
				perTarget: perTarget,
				base:      baseOffset,
				from:      fromOffsets,
			}
			baseOffset += threatValidTargets[piece] * perTarget
		}
	}
	if baseOffset != threatInputFeatures {
		panic(fmt.Sprintf("sf18big: FullThreats dimensions %d, expected %d", baseOffset, threatInputFeatures))
	}
	return result
}

// ActiveThreats returns Stockfish 18 FullThreats indices for one perspective.
// The returned live prefix is sorted to make before/after transition diffs
// explicit and deterministic.
func ActiveThreats(position base.Position, perspective base.Color) (ThreatIndexList, error) {
	board, kings, occupied, _, err := validatePosition(position)
	if err != nil {
		return ThreatIndexList{}, err
	}
	if perspective > base.Black {
		return ThreatIndexList{}, fmt.Errorf("%w: perspective %d", ErrEvaluation, perspective)
	}
	return activeThreatsBoard(&board, kings[perspective], occupied, perspective)
}

func activeThreatsBoard(
	board *[64]boardPiece,
	king base.Square,
	occupied uint64,
	perspective base.Color,
) (ThreatIndexList, error) {
	var active ThreatIndexList
	pieces := boardPieceBitboards(board)
	orientation := threatOrientation(king, perspective)
	for relativeColor := base.White; relativeColor <= base.Black; relativeColor++ {
		color := perspective ^ relativeColor
		for piece := base.Pawn; piece <= base.King; piece++ {
			attackers := pieces[color][piece]
			for attackers != 0 {
				from := base.Square(bits.TrailingZeros64(attackers))
				attackers &= attackers - 1
				if err := appendAttackerThreats(&active, board, occupied, perspective, orientation, from); err != nil {
					return ThreatIndexList{}, err
				}
			}
		}
	}
	slices.Sort(active.Indices[:active.Count])
	return active, nil
}

func boardPieceBitboards(board *[64]boardPiece) [2][6]uint64 {
	var pieces [2][6]uint64
	for square, piece := range board {
		if piece.set {
			pieces[piece.color][piece.piece] |= uint64(1) << square
		}
	}
	return pieces
}

func threatOrientation(king base.Square, perspective base.Color) base.Square {
	orientation := base.Square(56 * perspective)
	if king&7 >= 4 {
		orientation ^= 7
	}
	return orientation
}

func appendAttackerThreats(
	active *ThreatIndexList,
	board *[64]boardPiece,
	occupied uint64,
	perspective base.Color,
	orientation base.Square,
	from base.Square,
) error {
	attacker := (*board)[from]
	if !attacker.set {
		return nil
	}
	attackerColor := attacker.color ^ perspective
	attackerID := int(attackerColor)*6 + int(attacker.piece)
	fromOriented := from ^ orientation
	fromOffset := threatOffsets[attackerColor][attacker.piece].from[fromOriented]
	attacks := occupiedThreatAttacks(attacker.piece, attacker.color, from, occupied)
	for attacks != 0 {
		to := base.Square(bits.TrailingZeros64(attacks))
		attacks &= attacks - 1
		attacked := (*board)[to]
		attackedColor := attacked.color ^ perspective
		attackedID := int(attackedColor)*6 + int(attacked.piece)
		toOriented := to ^ orientation
		fromLessThanTo := 0
		if fromOriented < toOriented {
			fromLessThanTo = 1
		}
		indexBase := threatIndexTable.base[attackerID][attackedID][fromLessThanTo]
		rank := threatIndexTable.rank[attackerID][fromOriented][toOriented]
		if indexBase == invalidThreatIndexBase || rank == 255 {
			continue
		}
		if int(active.Count) >= len(active.Indices) {
			return fmt.Errorf("%w: more than %d active threats", ErrEvaluation, maxActiveThreats)
		}
		active.Indices[active.Count] = uint32(int(indexBase) + fromOffset + int(rank))
		active.Count++
	}
	return nil
}

func dirtyThreatIndexDiff(
	before, after *[64]boardPiece,
	beforePieces, afterPieces [2][6]uint64,
	beforeOccupied, afterOccupied uint64,
	changedSquares uint64,
	king base.Square,
	perspective base.Color,
) (ThreatIndexList, ThreatIndexList, error) {
	if changedSquares == 0 {
		return ThreatIndexList{}, ThreatIndexList{}, nil
	}
	affected := changedSquares |
		threatAttackersToSquares(beforePieces, beforeOccupied, changedSquares) |
		threatAttackersToSquares(afterPieces, afterOccupied, changedSquares)
	orientation := threatOrientation(king, perspective)
	var removedCandidates, addedCandidates ThreatIndexList
	for affected != 0 {
		from := base.Square(bits.TrailingZeros64(affected))
		affected &= affected - 1
		beforeAttacker := (*before)[from]
		var beforeAttacks uint64
		if beforeAttacker.set {
			beforeAttacks = occupiedThreatAttacks(
				beforeAttacker.piece,
				beforeAttacker.color,
				from,
				beforeOccupied,
			)
		}
		afterAttacker := (*after)[from]
		var afterAttacks uint64
		if afterAttacker.set {
			afterAttacks = occupiedThreatAttacks(
				afterAttacker.piece,
				afterAttacker.color,
				from,
				afterOccupied,
			)
		}
		beforeOrigin := indexedThreatOrigin(beforeAttacker, perspective, orientation, from)
		afterOrigin := indexedThreatOrigin(afterAttacker, perspective, orientation, from)
		for targets := beforeAttacks | afterAttacks; targets != 0; targets &= targets - 1 {
			to := base.Square(bits.TrailingZeros64(targets))
			toBit := uint64(1) << to
			toOriented := to ^ orientation
			var beforeIndex, afterIndex uint32
			var beforeActive, afterActive bool
			if beforeAttacks&toBit != 0 {
				beforeIndex, beforeActive = indexedThreatTarget(
					beforeOrigin, (*before)[to], perspective, toOriented,
				)
			}
			if afterAttacks&toBit != 0 {
				afterIndex, afterActive = indexedThreatTarget(
					afterOrigin, (*after)[to], perspective, toOriented,
				)
			}
			if beforeActive && afterActive && beforeIndex == afterIndex {
				continue
			}
			if beforeActive {
				if err := appendThreatIndex(&removedCandidates, beforeIndex); err != nil {
					return ThreatIndexList{}, ThreatIndexList{}, err
				}
			}
			if afterActive {
				if err := appendThreatIndex(&addedCandidates, afterIndex); err != nil {
					return ThreatIndexList{}, ThreatIndexList{}, err
				}
			}
		}
	}
	slices.Sort(removedCandidates.Indices[:removedCandidates.Count])
	slices.Sort(addedCandidates.Indices[:addedCandidates.Count])
	removed, added := diffThreatIndexLists(removedCandidates, addedCandidates)
	return removed, added, nil
}

type threatIndexOrigin struct {
	attackerID   int
	fromOffset   int
	fromOriented base.Square
	valid        bool
}

func indexedThreatOrigin(
	attacker boardPiece,
	perspective base.Color,
	orientation base.Square,
	from base.Square,
) threatIndexOrigin {
	if !attacker.set {
		return threatIndexOrigin{}
	}
	attackerColor := attacker.color ^ perspective
	fromOriented := from ^ orientation
	return threatIndexOrigin{
		attackerID:   int(attackerColor)*6 + int(attacker.piece),
		fromOffset:   threatOffsets[attackerColor][attacker.piece].from[fromOriented],
		fromOriented: fromOriented,
		valid:        true,
	}
}

func indexedThreatTarget(
	origin threatIndexOrigin,
	attacked boardPiece,
	perspective base.Color,
	toOriented base.Square,
) (uint32, bool) {
	if !origin.valid || !attacked.set {
		return 0, false
	}
	attackedColor := attacked.color ^ perspective
	attackedID := int(attackedColor)*6 + int(attacked.piece)
	fromLessThanTo := 0
	if origin.fromOriented < toOriented {
		fromLessThanTo = 1
	}
	indexBase := threatIndexTable.base[origin.attackerID][attackedID][fromLessThanTo]
	rank := threatIndexTable.rank[origin.attackerID][origin.fromOriented][toOriented]
	if indexBase == invalidThreatIndexBase || rank == 255 {
		return 0, false
	}
	return uint32(int(indexBase) + origin.fromOffset + int(rank)), true
}

func appendThreatIndex(active *ThreatIndexList, index uint32) error {
	if int(active.Count) >= len(active.Indices) {
		return fmt.Errorf("%w: more than %d active threats", ErrEvaluation, maxActiveThreats)
	}
	active.Indices[active.Count] = index
	active.Count++
	return nil
}

func threatAttackersToSquares(pieces [2][6]uint64, occupied, targets uint64) uint64 {
	var attackers uint64
	for targets != 0 {
		to := base.Square(bits.TrailingZeros64(targets))
		targets &= targets - 1
		attackers |= pseudoThreatTable[base.Black][base.Pawn][to] & pieces[base.White][base.Pawn]
		attackers |= pseudoThreatTable[base.White][base.Pawn][to] & pieces[base.Black][base.Pawn]
		attackers |= pseudoThreatTable[base.White][base.Knight][to] &
			(pieces[base.White][base.Knight] | pieces[base.Black][base.Knight])
		attackers |= pseudoThreatTable[base.White][base.King][to] &
			(pieces[base.White][base.King] | pieces[base.Black][base.King])
		diagonal := occupiedThreatAttacks(base.Bishop, base.White, to, occupied)
		attackers |= diagonal & (pieces[base.White][base.Bishop] | pieces[base.Black][base.Bishop] |
			pieces[base.White][base.Queen] | pieces[base.Black][base.Queen])
		orthogonal := occupiedThreatAttacks(base.Rook, base.White, to, occupied)
		attackers |= orthogonal & (pieces[base.White][base.Rook] | pieces[base.Black][base.Rook] |
			pieces[base.White][base.Queen] | pieces[base.Black][base.Queen])
	}
	return attackers
}

// DiffThreats computes a reference sorted-set transition. It accepts two
// validated positions rather than a move-specific dirty-threat encoding, which
// keeps this feasibility oracle independent of Stockfish's update machinery.
func DiffThreats(before, after base.Position, perspective base.Color) (ThreatDiff, error) {
	_, beforeKings, _, _, err := validatePosition(before)
	if err != nil {
		return ThreatDiff{}, err
	}
	_, afterKings, _, _, err := validatePosition(after)
	if err != nil {
		return ThreatDiff{}, err
	}
	beforeActive, err := ActiveThreats(before, perspective)
	if err != nil {
		return ThreatDiff{}, err
	}
	afterActive, err := ActiveThreats(after, perspective)
	if err != nil {
		return ThreatDiff{}, err
	}

	diff := ThreatDiff{
		RequiresRefresh: beforeKings[perspective]&4 != afterKings[perspective]&4,
	}
	diff.Removed, diff.Added = diffThreatIndexLists(beforeActive, afterActive)
	return diff, nil
}

func diffThreatIndexLists(beforeActive, afterActive ThreatIndexList) (ThreatIndexList, ThreatIndexList) {
	var removed ThreatIndexList
	var added ThreatIndexList
	i, j := 0, 0
	for i < int(beforeActive.Count) || j < int(afterActive.Count) {
		switch {
		case i == int(beforeActive.Count):
			added.Indices[added.Count] = afterActive.Indices[j]
			added.Count++
			j++
		case j == int(afterActive.Count):
			removed.Indices[removed.Count] = beforeActive.Indices[i]
			removed.Count++
			i++
		case beforeActive.Indices[i] < afterActive.Indices[j]:
			removed.Indices[removed.Count] = beforeActive.Indices[i]
			removed.Count++
			i++
		case afterActive.Indices[j] < beforeActive.Indices[i]:
			added.Indices[added.Count] = afterActive.Indices[j]
			added.Count++
			j++
		default:
			i++
			j++
		}
	}
	return removed, added
}

func pseudoThreatAttacks(piece base.PieceType, color base.Color, from base.Square) uint64 {
	switch piece {
	case base.Pawn:
		return pawnThreatAttacks(color, from)
	case base.Knight:
		return leaperThreatAttacks(from, [][2]int{{1, 2}, {2, 1}, {2, -1}, {1, -2}, {-1, -2}, {-2, -1}, {-2, 1}, {-1, 2}})
	case base.Bishop:
		return slidingThreatAttacks(from, 0, [][2]int{{1, 1}, {1, -1}, {-1, 1}, {-1, -1}})
	case base.Rook:
		return slidingThreatAttacks(from, 0, [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}})
	case base.Queen:
		return slidingThreatAttacks(from, 0, [][2]int{{1, 1}, {1, -1}, {-1, 1}, {-1, -1}, {1, 0}, {-1, 0}, {0, 1}, {0, -1}})
	case base.King:
		return leaperThreatAttacks(from, [][2]int{{1, 1}, {1, 0}, {1, -1}, {0, 1}, {0, -1}, {-1, 1}, {-1, 0}, {-1, -1}})
	default:
		return 0
	}
}

func occupiedThreatAttacks(piece base.PieceType, color base.Color, from base.Square, occupied uint64) uint64 {
	switch piece {
	case base.Bishop:
		return (threatRayAttacks(threatNorthEast, from, occupied) |
			threatRayAttacks(threatNorthWest, from, occupied) |
			threatRayAttacks(threatSouthEast, from, occupied) |
			threatRayAttacks(threatSouthWest, from, occupied)) & occupied
	case base.Rook:
		return (threatRayAttacks(threatNorth, from, occupied) |
			threatRayAttacks(threatSouth, from, occupied) |
			threatRayAttacks(threatEast, from, occupied) |
			threatRayAttacks(threatWest, from, occupied)) & occupied
	case base.Queen:
		return (threatRayAttacks(threatNorth, from, occupied) |
			threatRayAttacks(threatSouth, from, occupied) |
			threatRayAttacks(threatEast, from, occupied) |
			threatRayAttacks(threatWest, from, occupied) |
			threatRayAttacks(threatNorthEast, from, occupied) |
			threatRayAttacks(threatNorthWest, from, occupied) |
			threatRayAttacks(threatSouthEast, from, occupied) |
			threatRayAttacks(threatSouthWest, from, occupied)) & occupied
	default:
		return pseudoThreatTable[color][piece][from] & occupied
	}
}

func threatRayAttacks(direction int, from base.Square, occupied uint64) uint64 {
	ray := threatRayTable[direction][from]
	blockers := ray & occupied
	if blockers == 0 {
		return ray
	}
	var blocker base.Square
	switch direction {
	case threatNorth, threatEast, threatNorthEast, threatNorthWest:
		blocker = base.Square(bits.TrailingZeros64(blockers))
	default:
		blocker = base.Square(63 - bits.LeadingZeros64(blockers))
	}
	return ray ^ threatRayTable[direction][blocker]
}

func pawnThreatAttacks(color base.Color, from base.Square) uint64 {
	file, rank := int(from&7), int(from>>3)
	direction := 1
	if color == base.Black {
		direction = -1
	}
	toRank := rank + direction
	if toRank < 0 || toRank >= 8 {
		return 0
	}
	var attacks uint64
	for _, fileDelta := range []int{-1, 1} {
		toFile := file + fileDelta
		if toFile >= 0 && toFile < 8 {
			attacks |= uint64(1) << (toRank*8 + toFile)
		}
	}
	return attacks
}

func leaperThreatAttacks(from base.Square, deltas [][2]int) uint64 {
	file, rank := int(from&7), int(from>>3)
	var attacks uint64
	for _, delta := range deltas {
		toFile, toRank := file+delta[0], rank+delta[1]
		if toFile >= 0 && toFile < 8 && toRank >= 0 && toRank < 8 {
			attacks |= uint64(1) << (toRank*8 + toFile)
		}
	}
	return attacks
}

func slidingThreatAttacks(from base.Square, occupied uint64, directions [][2]int) uint64 {
	file, rank := int(from&7), int(from>>3)
	var attacks uint64
	for _, direction := range directions {
		for distance := 1; ; distance++ {
			toFile, toRank := file+direction[0]*distance, rank+direction[1]*distance
			if toFile < 0 || toFile >= 8 || toRank < 0 || toRank >= 8 {
				break
			}
			bit := uint64(1) << (toRank*8 + toFile)
			attacks |= bit
			if occupied&bit != 0 {
				break
			}
		}
	}
	return attacks
}

func validatePosition(position base.Position) ([64]boardPiece, [2]base.Square, uint64, int, error) {
	var board [64]boardPiece
	var kings [2]base.Square
	var kingCount [2]uint8
	if position.SideToMove > base.Black {
		return board, kings, 0, 0, fmt.Errorf("%w: side to move %d", ErrEvaluation, position.SideToMove)
	}
	if len(position.Pieces) > 32 {
		return board, kings, 0, 0, fmt.Errorf("%w: %d occupied squares, maximum 32", ErrEvaluation, len(position.Pieces))
	}
	var occupied uint64
	for _, piece := range position.Pieces {
		if piece.Color > base.Black || piece.Piece > base.King || piece.Square >= 64 {
			return board, kings, 0, 0, fmt.Errorf("%w: invalid piece %+v", ErrEvaluation, piece)
		}
		if board[piece.Square].set {
			return board, kings, 0, 0, fmt.Errorf("%w: duplicate square %d", ErrEvaluation, piece.Square)
		}
		board[piece.Square] = boardPiece{piece: piece.Piece, color: piece.Color, set: true}
		occupied |= uint64(1) << piece.Square
		if piece.Piece == base.King {
			kingCount[piece.Color]++
			kings[piece.Color] = piece.Square
		}
	}
	for color := base.White; color <= base.Black; color++ {
		if kingCount[color] != 1 {
			return board, kings, 0, 0, fmt.Errorf("%w: colour %d has %d kings", ErrEvaluation, color, kingCount[color])
		}
	}
	return board, kings, occupied, len(position.Pieces), nil
}

func popcount64(value uint64) int {
	return bits.OnesCount64(value)
}

func trailingZeros64(value uint64) int {
	return bits.TrailingZeros64(value)
}
