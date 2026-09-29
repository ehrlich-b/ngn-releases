package engine

import (
	"testing"
)

// TestEvaluationBreakdown tests each component of evaluation to find the bug
func TestEvaluationBreakdown(t *testing.T) {
	fen := "rnbqkb1r/pppp1ppp/5n2/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R w KQkq - 0 1"

	pos, err := ParseFEN(fen)
	if err != nil {
		t.Fatal("Failed to parse FEN:", err)
	}

	t.Logf("=== EVALUATION BREAKDOWN ===")
	t.Logf("Position: %s", fen)

	// Test before capture
	beforeEval := Evaluate(&pos.Board)
	t.Logf("\nBefore capture: %d", beforeEval)

	// Make the capture f3xe5
	move := NewMove(F3, E5, WhiteKnight, BlackPawn, NoType, Capture)
	ep, tag, hc, _ := pos.MakeMove(move)

	afterEval := Evaluate(&pos.Board) // Black's perspective
	whiteAfterEval := -afterEval

	t.Logf("After capture: %d (white perspective)", whiteAfterEval)
	t.Logf("Change: %d", whiteAfterEval-beforeEval)

	pos.UnMakeMove(move, tag, ep, hc)

	// Now let's manually test what happens with just material changes
	t.Logf("\n=== MATERIAL TEST ===")

	// Create a simplified test by manually changing piece counts
	// Get material balance before
	materialBefore := calculateMaterialBalance(&pos.Board)
	t.Logf("Material before: %d", materialBefore)

	// Make the move again to test material after
	pos.MakeMove(move)
	materialAfter := calculateMaterialBalance(&pos.Board)
	t.Logf("Material after: %d", materialAfter)
	t.Logf("Material change: %d (should be +100)", materialAfter-materialBefore)

	pos.UnMakeMove(move, tag, ep, hc)

	// Let's test if the issue is with piece-square tables
	t.Logf("\n=== PIECE-SQUARE TABLE TEST ===")

	// Test knight on f3
	knightF3Value := getPieceSquareValue(WhiteKnight, F3, false)
	t.Logf("Knight on f3 PST value: %d", knightF3Value)

	// Test knight on e5
	knightE5Value := getPieceSquareValue(WhiteKnight, E5, false)
	t.Logf("Knight on e5 PST value: %d", knightE5Value)
	t.Logf("PST difference: %d", knightE5Value-knightF3Value)

	// Test black pawn on e5
	blackPawnE5Value := getPieceSquareValue(BlackPawn, E5, false)
	t.Logf("Black pawn on e5 PST value: %d", blackPawnE5Value)

	// Expected change should be: +100 (material) + PST difference
	expectedChange := 100 + (knightE5Value - knightF3Value) - blackPawnE5Value
	t.Logf("Expected total change: 100 + (%d - %d) - %d = %d",
		knightE5Value, knightF3Value, blackPawnE5Value, expectedChange)

	actualChange := whiteAfterEval - beforeEval
	unexplainedDifference := actualChange - expectedChange
	t.Logf("Actual change: %d", actualChange)
	t.Logf("Unexplained difference: %d", unexplainedDifference)

	if unexplainedDifference < -200 || unexplainedDifference > 200 {
		t.Logf("❌ MASSIVE UNEXPLAINED DIFFERENCE: %d points", unexplainedDifference)
		t.Logf("This suggests a major bug in evaluation components beyond material/PST")
	}
}

// calculateMaterialBalance returns the material balance from white's perspective
func calculateMaterialBalance(board *Bitboard) int {
	whiteMaterial := PopCount(board.GetBitboardOf(WhitePawn))*100 +
		PopCount(board.GetBitboardOf(WhiteKnight))*320 +
		PopCount(board.GetBitboardOf(WhiteBishop))*330 +
		PopCount(board.GetBitboardOf(WhiteRook))*500 +
		PopCount(board.GetBitboardOf(WhiteQueen))*900

	blackMaterial := PopCount(board.GetBitboardOf(BlackPawn))*100 +
		PopCount(board.GetBitboardOf(BlackKnight))*320 +
		PopCount(board.GetBitboardOf(BlackBishop))*330 +
		PopCount(board.GetBitboardOf(BlackRook))*500 +
		PopCount(board.GetBitboardOf(BlackQueen))*900

	return whiteMaterial - blackMaterial
}

// TestPieceWeights tests if the piece values are correctly applied
func TestPieceWeights(t *testing.T) {
	t.Logf("=== PIECE VALUE TEST ===")

	for _, piece := range []Piece{WhitePawn, WhiteKnight, WhiteBishop, WhiteRook, WhiteQueen} {
		t.Logf("%v weight: %d", piece, piece.Weight())
	}

	for _, piece := range []Piece{BlackPawn, BlackKnight, BlackBishop, BlackRook, BlackQueen} {
		t.Logf("%v weight: %d", piece, piece.Weight())
	}
}
