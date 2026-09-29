package engine

import (
	"testing"
)

// TestEvalComponents tests individual evaluation components before/after the capture
func TestEvalComponents(t *testing.T) {
	fen := "rnbqkb1r/pppp1ppp/5n2/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R w KQkq - 0 1"

	pos, err := ParseFEN(fen)
	if err != nil {
		t.Fatal("Failed to parse FEN:", err)
	}

	t.Logf("=== TESTING INDIVIDUAL EVALUATION COMPONENTS ===")

	// Before capture
	t.Logf("\n--- BEFORE CAPTURE ---")
	testEvalComponents(t, &pos.Board, "BEFORE")

	// Make the capture
	move := NewMove(F3, E5, WhiteKnight, BlackPawn, NoType, Capture)
	ep, tag, hc, _ := pos.MakeMove(move)

	t.Logf("\n--- AFTER CAPTURE (black's perspective) ---")
	testEvalComponents(t, &pos.Board, "AFTER")

	pos.UnMakeMove(move, tag, ep, hc)
}

func testEvalComponents(t *testing.T, board *Bitboard, label string) {
	endgame := isEndgame(board)
	eval := 0

	// 1. Material and PST
	materialAndPST := 0
	for _, piece := range Pieces {
		if piece == NoPiece {
			continue
		}

		pieceBitboard := board.GetBitboardOf(piece)
		color := piece.Color()
		val := piece.Weight()
		colorMul := 1
		if color == Black {
			colorMul = -1
		}

		// Material value
		if piece.Type() != King {
			materialValue := colorMul * int(val) * PopCount(pieceBitboard)
			materialAndPST += materialValue
			eval += materialValue
		}

		// PST values
		iterateBitboard(pieceBitboard, func(square int) {
			pstValue := getPieceSquareValue(piece, Square(square), endgame)
			materialAndPST += pstValue
			eval += pstValue
		})
	}

	t.Logf("%s Material+PST: %d", label, materialAndPST)

	// 2. Pawn structure
	pawnEval := 0

	whiteDoubledPawns := 0
	blackDoubledPawns := 0
	for file := 0; file < 8; file++ {
		whiteDoubledPawns += countDoubledPawns(board, White, file)
		blackDoubledPawns += countDoubledPawns(board, Black, file)
	}
	pawnDoubledPenalty := -(whiteDoubledPawns * 50) + (blackDoubledPawns * 50)
	pawnEval += pawnDoubledPenalty
	eval += pawnDoubledPenalty

	whiteIsolatedPawns := countIsolatedPawns(board, White)
	blackIsolatedPawns := countIsolatedPawns(board, Black)
	pawnIsolatedPenalty := -(whiteIsolatedPawns * 15) + (blackIsolatedPawns * 15)
	pawnEval += pawnIsolatedPenalty
	eval += pawnIsolatedPenalty

	whitePassedPawns := countPassedPawns(board, White)
	blackPassedPawns := countPassedPawns(board, Black)
	pawnPassedBonus := (whitePassedPawns * 25) - (blackPassedPawns * 25)
	pawnEval += pawnPassedBonus
	eval += pawnPassedBonus

	t.Logf("%s Pawn structure: %d (doubled: %d, isolated: %d, passed: %d)",
		label, pawnEval, pawnDoubledPenalty, pawnIsolatedPenalty, pawnPassedBonus)

	// 3. More pawn structure
	whitePawnChains := evaluatePawnChains(board, White)
	blackPawnChains := evaluatePawnChains(board, Black)
	pawnChainBonus := whitePawnChains - blackPawnChains
	pawnEval += pawnChainBonus
	eval += pawnChainBonus

	whiteWeakSquares := evaluateWeakSquares(board, White)
	blackWeakSquares := evaluateWeakSquares(board, Black)
	weakSquaresPenalty := whiteWeakSquares - blackWeakSquares
	pawnEval += weakSquaresPenalty
	eval += weakSquaresPenalty

	whitePawnStorms := evaluatePawnStorms(board, White)
	blackPawnStorms := evaluatePawnStorms(board, Black)
	pawnStormBonus := whitePawnStorms - blackPawnStorms
	pawnEval += pawnStormBonus
	eval += pawnStormBonus

	t.Logf("%s Extended pawn: %d (chains: %d, weak: %d, storms: %d)",
		label, pawnChainBonus+weakSquaresPenalty+pawnStormBonus,
		pawnChainBonus, weakSquaresPenalty, pawnStormBonus)

	// 4. King safety
	var sliderAtt [64]uint64
	fillSliderAttacks(board, &sliderAtt)
	whiteKingSafety := evaluateKingSafety(board, White, &sliderAtt)
	blackKingSafety := evaluateKingSafety(board, Black, &sliderAtt)
	kingSafetyBonus := 0
	if !endgame {
		kingSafetyBonus = whiteKingSafety - blackKingSafety
		eval += kingSafetyBonus
	}

	t.Logf("%s King safety: %d", label, kingSafetyBonus)

	// 5. Piece coordination
	whiteCoordination := evaluatePieceCoordination(board, White)
	blackCoordination := evaluatePieceCoordination(board, Black)
	coordinationBonus := whiteCoordination - blackCoordination
	eval += coordinationBonus

	t.Logf("%s Piece coordination: %d", label, coordinationBonus)

	// 6. Bishop pair
	whiteBishopPair := evaluateBishopPair(board, White)
	blackBishopPair := evaluateBishopPair(board, Black)
	bishopPairBonus := whiteBishopPair - blackBishopPair
	eval += bishopPairBonus

	t.Logf("%s Bishop pair: %d", label, bishopPairBonus)

	// 7. Endgame
	endgameBonus := 0
	if endgame {
		endgameBonus = evaluateBasicEndgames(board)
		eval += endgameBonus
	}

	t.Logf("%s Endgame: %d", label, endgameBonus)

	t.Logf("%s TOTAL CALCULATED: %d", label, eval)

	// Compare with actual Evaluate function
	actualEval := Evaluate(board)
	t.Logf("%s ACTUAL EVALUATE: %d", label, actualEval)

	if eval != actualEval {
		t.Logf("❌ MISMATCH: Calculated %d vs Actual %d (diff: %d)",
			eval, actualEval, actualEval-eval)
	}
}
