package engine

// PeSTO evaluation: Texel-tuned tapered piece-square tables and material values
// Source: Ronald Friederich's PeSTO engine (public domain) — one of the best-known
// classical evaluations. Having correct, tuned PSTs is worth 200+ ELO over
// hand-tuned PSTs.

// Material values (middlegame and endgame)
var pestoMGMaterial = [7]int{0, 82, 337, 365, 477, 1025, 0}
var pestoEGMaterial = [7]int{0, 94, 281, 297, 512, 936, 0}

// MG = middlegame, EG = endgame. Tables are from white's perspective, with
// square 0 = A8 (display order). Index by square as (rank*8 + file), where
// rank 0 = 8th rank, rank 7 = 1st rank. This matches the natural way chess
// boards are printed.
//
// The position.Square enum in this engine is rank 0 = 1st rank; we flip
// at lookup time with `sq ^ 56` for white, and mirror for black.

var mgPawnTable = [64]int{
	0, 0, 0, 0, 0, 0, 0, 0,
	74, 108, 51, 105, 90, 118, -2, -47,
	-6, -13, 20, 19, 57, 63, 9, -22,
	-18, 3, 10, 21, 19, 16, 1, -23,
	-31, -14, -1, 8, 13, 2, -10, -33,
	-30, -16, -4, -14, -5, -1, 13, -20,
	-31, -5, -12, -19, -16, 20, 26, -22,
	0, 0, 0, 0, 0, 0, 0, 0,
}

var egPawnTable = [64]int{
	0, 0, 0, 0, 0, 0, 0, 0,
	202, 185, 170, 136, 147, 134, 185, 219,
	106, 112, 89, 69, 50, 49, 86, 96,
	36, 16, 5, -7, -10, -4, 9, 17,
	17, 1, -11, -19, -15, -16, -9, -1,
	4, -5, -14, -7, -8, -13, -17, -12,
	17, 0, 4, 0, 5, -8, -14, -11,
	0, 0, 0, 0, 0, 0, 0, 0,
}

var mgKnightTable = [64]int{
	-167, -92, -58, -34, 67, -95, -24, -102,
	-85, -49, 68, 24, 9, 50, -4, -18,
	-51, 42, 17, 45, 79, 106, 61, 38,
	-8, 11, 1, 41, 19, 58, 14, 27,
	-7, 5, 10, 11, 28, 15, 21, 2,
	-15, -1, 14, 14, 29, 23, 33, -6,
	-14, -32, -2, 15, 17, 26, 5, 7,
	-107, -7, -36, -18, 8, -8, -3, 1,
}

var egKnightTable = [64]int{
	-20, -19, 3, -23, -27, -17, -51, -73,
	-5, 8, -29, 1, -9, -25, -11, -30,
	-14, -22, 0, -7, -21, -27, -27, -36,
	-7, -3, 10, 10, 10, -5, 0, -10,
	-10, -10, 4, 17, 8, 9, 4, -6,
	-11, -3, -9, 7, 2, -11, -20, -3,
	-23, -8, -6, -5, -2, -16, -9, -34,
	4, -35, -9, 1, -8, -7, -32, -43,
}

var mgBishopTable = [64]int{
	-23, -12, -121, -89, -49, -58, -26, -2,
	-32, -2, -40, -45, 14, 35, 8, -53,
	-26, 16, 31, 18, 18, 42, 17, -14,
	-8, 5, 1, 39, 20, 27, 3, -2,
	0, 5, 7, 22, 34, -1, 4, 6,
	0, 19, 15, 15, 14, 35, 18, 12,
	14, 25, 20, 8, 19, 27, 43, 9,
	-25, 9, -2, -9, 1, -2, -39, -19,
}

var egBishopTable = [64]int{
	-2, -3, 17, 16, 13, 11, 7, -9,
	8, 5, 19, 4, 5, -1, 2, 2,
	16, -1, -2, -1, -2, -2, 4, 16,
	5, 5, 10, 2, 6, 4, 1, 10,
	2, 3, 13, 11, -1, 10, 1, 3,
	4, 5, 16, 14, 21, 3, 5, 1,
	0, -6, 1, 7, 8, -2, -7, -13,
	-3, 9, -3, 11, 8, 8, 15, 5,
}

var mgRookTable = [64]int{
	20, 28, -2, 47, 67, -5, 26, 7,
	20, 20, 48, 62, 84, 67, 6, 33,
	-21, 5, 12, 16, -9, 36, 63, -8,
	-34, -19, 3, 22, 12, 23, -12, -32,
	-44, -30, -11, -7, 5, -7, 10, -27,
	-41, -21, -10, -15, 3, 8, 3, -23,
	-38, -10, -12, -1, 7, 16, 4, -59,
	-11, -5, 9, 17, 20, 17, -25, -10,
}

var egRookTable = [64]int{
	19, 14, 22, 7, 6, 28, 19, 21,
	17, 19, 11, 3, -11, 7, 25, 15,
	23, 19, 15, 15, 18, 9, 3, 17,
	22, 18, 23, 6, 12, 17, 16, 28,
	23, 22, 18, 12, 7, 13, 6, 11,
	14, 14, 5, 7, -3, 0, 6, 0,
	12, 6, 8, 6, -1, -1, -1, 21,
	11, 10, 7, -1, -1, -1, 16, -4,
}

var mgQueenTable = [64]int{
	-34, -27, -11, 0, 107, 92, 50, 29,
	-37, -63, -33, -11, -69, 23, -2, 28,
	-25, -33, -6, -35, -7, 37, 5, 19,
	-47, -39, -36, -41, -33, -21, -30, -25,
	-17, -46, -21, -26, -20, -18, -17, -21,
	-30, -4, -17, -10, -13, -6, 1, -9,
	-35, -10, 7, 4, 12, 19, 5, 10,
	-6, -10, -3, 10, -10, -19, -26, -60,
}

var egQueenTable = [64]int{
	44, 93, 92, 83, 37, 38, 50, 83,
	37, 82, 100, 101, 146, 89, 87, 60,
	36, 62, 55, 128, 121, 87, 91, 69,
	73, 86, 86, 113, 133, 110, 125, 95,
	38, 104, 85, 113, 95, 92, 101, 85,
	59, 21, 75, 62, 69, 75, 76, 75,
	46, 31, 26, 28, 32, 25, 12, 17,
	23, 20, 26, 13, 53, 26, 35, 20,
}

var mgKingTable = [64]int{
	-83, 191, 170, 128, -94, -16, 96, 95,
	243, 84, 48, 138, 42, 50, 1, -104,
	101, 74, 98, 31, 66, 140, 150, 16,
	23, 16, 56, -3, -1, -9, 16, -62,
	-43, 41, -15, -62, -66, -20, -19, -63,
	4, 16, -6, -29, -32, -14, 21, -19,
	17, 19, 8, -46, -23, 0, 25, 16,
	-31, 20, 20, -49, 8, -32, 16, 10,
}

var egKingTable = [64]int{
	-66, -65, -46, -44, 7, 20, -10, -27,
	-60, 2, 8, -5, 11, 32, 27, 32,
	-9, 9, 9, 11, 6, 26, 23, 9,
	-20, 18, 16, 27, 24, 33, 26, 11,
	-20, -14, 22, 33, 35, 23, 9, -3,
	-25, -7, 11, 21, 25, 16, -1, -9,
	-37, -15, 4, 16, 14, 4, -8, -20,
	-57, -38, -27, -6, -24, -6, -28, -55,
}

// Precomputed per-square PST values combined with material, indexed by square
// in internal Square ordering (rank 0 = 1st rank, so white H1 = 7, A8 = 56).
// Tables are `[piece][sq]` where piece is PieceType (1..6).
// At square lookup time we do sq^56 for white (to flip from "display order" in
// the raw tables to internal order) — actually we precompute this once here.
var mgPST [13][64]int // [piece][sq] indexed by Piece value (WhitePawn=1..BlackKing=12)
var egPST [13][64]int

// piecePhaseInc is the game-phase weight contributed by each piece, indexed by
// Piece value (both colors share a type's weight). It feeds the incremental
// phase accumulator in bitboard.go so gamePhase need not rescan the board.
var piecePhaseInc [13]int

func init() {
	typePhase := [7]int{0, 0, knightPhase, bishopPhase, rookPhase, queenPhase, 0} // by PieceType
	for pt := Pawn; pt <= King; pt++ {
		piecePhaseInc[GetPiece(pt, White)] = typePhase[pt]
		piecePhaseInc[GetPiece(pt, Black)] = typePhase[pt]
	}
	RebuildPST()
}

// RebuildPST recomputes the combined mgPST/egPST lookup from the raw piece-square
// tables + material. It is the single source of truth for that derivation (init
// calls it at startup); Texel tuning calls it after mutating raw table entries
// via TexelPSTParams so evaluatePeSTO sees the updated values.
//
// This is an offline raw hook. It does not acquire the HCE model lifecycle or
// publish a generation. Runtime model replacement must use ApplyTexelModel.
func RebuildPST() {
	// Combine material + piece-square value so one table lookup yields total
	// contribution for a single piece on a square. Signed by color: black
	// entries are negated from white (so summing all pieces gives white-perspective eval).
	rawMG := [7]*[64]int{nil, &mgPawnTable, &mgKnightTable, &mgBishopTable, &mgRookTable, &mgQueenTable, &mgKingTable}
	rawEG := [7]*[64]int{nil, &egPawnTable, &egKnightTable, &egBishopTable, &egRookTable, &egQueenTable, &egKingTable}

	for pt := Pawn; pt <= King; pt++ {
		wp := GetPiece(pt, White)
		bp := GetPiece(pt, Black)
		for sq := 0; sq < 64; sq++ {
			// White: table is in display order (rank 0 = 8th rank),
			// internal square order has rank 0 = 1st rank → flip sq^56.
			mgPST[wp][sq] = pestoMGMaterial[pt] + (*rawMG[pt])[sq^56]
			egPST[wp][sq] = pestoEGMaterial[pt] + (*rawEG[pt])[sq^56]

			// Black: use display-order table as-is (rank 0 = 8th rank, which
			// for black IS the back rank), negated for white-perspective eval.
			mgPST[bp][sq] = -(pestoMGMaterial[pt] + (*rawMG[pt])[sq])
			egPST[bp][sq] = -(pestoEGMaterial[pt] + (*rawEG[pt])[sq])
		}
	}
}

// texelPSTTables lists the 12 raw piece-square tables in a stable order (MG
// pawn..king, then EG pawn..king). Black tables are derived by mirroring in
// RebuildPST, so only these white-POV tables are exposed for tuning.
func texelPSTTables() []*[64]int {
	return []*[64]int{
		&mgPawnTable, &mgKnightTable, &mgBishopTable, &mgRookTable, &mgQueenTable, &mgKingTable,
		&egPawnTable, &egKnightTable, &egBishopTable, &egRookTable, &egQueenTable, &egKingTable,
	}
}

// TexelPSTParams returns pointers to all 768 raw PST entries for coordinate
// descent. The pointers are offline-only and carry no lifecycle protection.
// Mutating any requires RebuildPST before evaluation; runtime replacement must
// instead use ApplyTexelModel or a checked TryTexelTune entry point.
func TexelPSTParams() []*int {
	tables := texelPSTTables()
	ptrs := make([]*int, 0, len(tables)*64)
	for _, t := range tables {
		for i := range t {
			ptrs = append(ptrs, &t[i])
		}
	}
	return ptrs
}

// evaluatePeSTO returns the core tapered material + PST evaluation from
// white's perspective. This is the backbone of the evaluation.
func evaluatePeSTO(board *Bitboard) (mgScore, egScore, phase int) {
	// Read the incrementally-maintained accumulator instead of rescanning the
	// board. accMG/accEG are the exact MG/EG material+PST sums; accPhase is the
	// raw game phase, clamped here to match gamePhase's defensive cap (extra
	// queens from promotion can push it past totalPhase).
	phase = board.accPhase
	if phase > totalPhase {
		phase = totalPhase
	}
	return board.accMG, board.accEG, phase
}

// hasNonPawnMaterial reports whether the given color has any piece other than
// pawns or the king. Used to gate null-move pruning (pawn-only endgames are
// prone to zugzwang, where null-move would give unsound cutoffs).
func hasNonPawnMaterial(board *Bitboard, color Color) bool {
	if color == White {
		return (board.GetBitboardOf(WhiteKnight) |
			board.GetBitboardOf(WhiteBishop) |
			board.GetBitboardOf(WhiteRook) |
			board.GetBitboardOf(WhiteQueen)) != 0
	}
	return (board.GetBitboardOf(BlackKnight) |
		board.GetBitboardOf(BlackBishop) |
		board.GetBitboardOf(BlackRook) |
		board.GetBitboardOf(BlackQueen)) != 0
}
