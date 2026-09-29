package engine

import (
	"testing"
)

// TestPawnAttackBug tests the reported issue of pawns taking rooks incorrectly
func TestPawnAttackBug(t *testing.T) {
	// Test case: White pawn should NOT be able to capture a rook that's not on a diagonal
	t.Run("PawnCannotCaptureNonDiagonal", func(t *testing.T) {
		// Position: White pawn on e4, Black rook on e5 (directly in front)
		pos, err := ParseFEN("rnbqkbnr/pppp1ppp/8/4r3/4P3/8/PPPP1PPP/RNBQKBNR w KQkq - 0 1")
		if err != nil {
			t.Fatalf("Failed to parse FEN: %v", err)
		}

		moves := GenerateLegalMoves(pos)

		// Check if any move allows pawn to capture rook directly forward
		for _, move := range moves {
			if move.Source() == E4 && move.Destination() == E5 {
				t.Errorf("❌ BUG FOUND: Pawn on e4 can illegally capture rook on e5 (not diagonal!)")
				t.Errorf("Move details: %+v", move)
			}
		}

		t.Logf("✅ Correctly: White pawn cannot capture rook directly in front")
	})

	t.Run("PawnCanCaptureOnDiagonal", func(t *testing.T) {
		// Position: White pawn on e4, Black rook on d5 (diagonal)
		pos, err := ParseFEN("rnbqkbnr/pppp1ppp/8/3r4/4P3/8/PPPP1PPP/RNBQKBNR w KQkq - 0 1")
		if err != nil {
			t.Fatalf("Failed to parse FEN: %v", err)
		}

		moves := GenerateLegalMoves(pos)

		// Check if pawn can capture rook on diagonal (should be legal)
		foundCapture := false
		for _, move := range moves {
			if move.Source() == E4 && move.Destination() == D5 && move.IsCapture() {
				foundCapture = true
				t.Logf("✅ Correctly: White pawn can capture rook on diagonal d5")
				break
			}
		}

		if !foundCapture {
			t.Errorf("❌ BUG: Pawn should be able to capture rook on diagonal!")
		}
	})

	t.Run("PawnCannotCaptureOwnPieces", func(t *testing.T) {
		// Position: White pawn on e4, White rook on d5 (diagonal)
		pos, err := ParseFEN("rnbqkbnr/pppp1ppp/8/3R4/4P3/8/PPPP1PPP/RNBQKBNR w KQkq - 0 1")
		if err != nil {
			t.Fatalf("Failed to parse FEN: %v", err)
		}

		moves := GenerateLegalMoves(pos)

		// Check if pawn tries to capture own rook
		for _, move := range moves {
			if move.Source() == E4 && move.Destination() == D5 {
				t.Errorf("❌ BUG: Pawn trying to capture own rook!")
				t.Errorf("Move details: %+v", move)
			}
		}

		t.Logf("✅ Correctly: White pawn cannot capture own rook")
	})
}

// TestSpecificPawnCaptureBugs tests edge cases in pawn capturing
func TestSpecificPawnCaptureBugs(t *testing.T) {
	t.Run("TestAllPawnCaptureDirections", func(t *testing.T) {
		// Position with pawn surrounded by enemy pieces
		pos, err := ParseFEN("rnbqkbnr/ppp2ppp/8/3rpb2/4P3/8/PPPP1PPP/RNBQKBNR w KQkq - 0 1")
		if err != nil {
			t.Fatalf("Failed to parse FEN: %v", err)
		}

		moves := GenerateLegalMoves(pos)

		// Analyze all pawn moves from e4
		pawnMoves := []Move{}
		for _, move := range moves {
			if move.Source() == E4 {
				pawnMoves = append(pawnMoves, move)
			}
		}

		t.Logf("Pawn on e4 has %d possible moves:", len(pawnMoves))
		for _, move := range pawnMoves {
			t.Logf("  %s -> %s, capture: %t, moving piece: %v, captured: %v",
				move.Source(), move.Destination(), move.IsCapture(),
				move.MovingPiece(), move.CapturedPiece())
		}

		// Check specific expectations
		validCaptures := 0
		invalidMoves := 0

		for _, move := range pawnMoves {
			dest := move.Destination()

			switch dest {
			case D5: // Should be able to capture rook diagonally
				if move.IsCapture() && move.CapturedPiece() == BlackRook {
					validCaptures++
					t.Logf("✅ Valid diagonal capture: e4xd5 rook")
				} else {
					t.Errorf("❌ Expected capture of rook on d5, got: %+v", move)
					invalidMoves++
				}
			case F5: // Should be able to capture bishop diagonally
				if move.IsCapture() && move.CapturedPiece() == BlackBishop {
					validCaptures++
					t.Logf("✅ Valid diagonal capture: e4xf5 bishop")
				} else {
					t.Errorf("❌ Expected capture of bishop on f5, got: %+v", move)
					invalidMoves++
				}
			case E5: // Should NOT be able to move forward (blocked)
				t.Errorf("❌ Pawn should not be able to move to e5 (blocked by pawn)")
				invalidMoves++
			case E6: // Should NOT be able to double push (blocked)
				t.Errorf("❌ Pawn should not be able to double push to e6 (blocked)")
				invalidMoves++
			default:
				t.Errorf("❌ Unexpected pawn move to %s", dest)
				invalidMoves++
			}
		}

		if validCaptures != 2 {
			t.Errorf("❌ Expected 2 valid captures, got %d", validCaptures)
		}

		if invalidMoves > 0 {
			t.Errorf("❌ Found %d invalid pawn moves", invalidMoves)
		}
	})
}

// TestPawnCaptureValidation tests the complete move generation and validation pipeline
func TestPawnCaptureValidation(t *testing.T) {
	pos, err := ParseFEN("rnbqkbnr/pppp1ppp/8/4r3/4P3/8/PPPP1PPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatalf("Failed to parse FEN: %v", err)
	}

	// Generate all legal moves - should not include straight-ahead pawn captures
	legalMoves := GenerateLegalMoves(pos)

	// Check that no pawn move captures straight ahead
	for _, move := range legalMoves {
		if move.MovingPiece().Type() == Pawn && move.IsCapture() {
			from := move.Source()
			to := move.Destination()

			// Check if it's a straight-ahead capture (same file)
			if from.File() == to.File() {
				t.Errorf("❌ CRITICAL BUG: Found illegal straight-ahead pawn capture: %s", move.ToString())
			}
		}
	}

	t.Logf("✅ Move generation correctly excludes straight-ahead pawn captures")

	// Test valid diagonal capture (set up position for it)
	pos2, err := ParseFEN("rnbqkbnr/pppp1ppp/8/3r4/4P3/8/PPPP1PPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatalf("Failed to parse FEN: %v", err)
	}

	legalMoves2 := GenerateLegalMoves(pos2)

	// Check that valid diagonal capture is included
	foundDiagonalCapture := false
	for _, move := range legalMoves2 {
		if move.MovingPiece().Type() == Pawn && move.IsCapture() &&
			move.Source() == E4 && move.Destination() == D5 {
			foundDiagonalCapture = true
			break
		}
	}

	if !foundDiagonalCapture {
		t.Errorf("❌ BUG: Legal diagonal pawn capture not generated!")
	} else {
		t.Logf("✅ Move generation correctly includes diagonal pawn captures")
	}
}

// TestPseudoLegalVsLegal tests the difference between pseudo-legal and legal move generation
func TestPseudoLegalVsLegal(t *testing.T) {
	pos, err := ParseFEN("rnbqkbnr/pppp1ppp/8/4r3/4P3/8/PPPP1PPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatalf("Failed to parse FEN: %v", err)
	}

	// Generate pseudo-legal moves (should include invalid pawn captures)
	pseudoLegalMoves := generateMovesUnsafe(pos)

	// Generate legal moves (should filter out invalid moves)
	legalMoves := GenerateLegalMoves(pos)

	t.Logf("Pseudo-legal moves: %d", len(pseudoLegalMoves))
	t.Logf("Legal moves: %d", len(legalMoves))

	// Check for the problematic pawn move in pseudo-legal
	foundInPseudoLegal := false
	foundInLegal := false

	for _, move := range pseudoLegalMoves {
		if move.Source() == E4 && move.Destination() == E5 {
			foundInPseudoLegal = true
			t.Logf("Found e4-e5 in pseudo-legal moves: %+v", move)
		}
	}

	for _, move := range legalMoves {
		if move.Source() == E4 && move.Destination() == E5 {
			foundInLegal = true
			t.Errorf("❌ CRITICAL BUG: Found illegal pawn move e4-e5 in legal moves!")
		}
	}

	if foundInPseudoLegal && !foundInLegal {
		t.Logf("✅ Good: Illegal move filtered out between pseudo-legal and legal")
	} else if !foundInPseudoLegal && !foundInLegal {
		t.Logf("✅ Good: Illegal move not generated at all")
	} else if foundInLegal {
		t.Errorf("❌ CRITICAL: Illegal move made it to legal moves!")
	}
}
