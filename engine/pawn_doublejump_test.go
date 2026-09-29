package engine

import (
	"testing"
)

// TestPawnDoubleJumpBlocked verifies pawns cannot jump over pieces
func TestPawnDoubleJumpBlocked(t *testing.T) {
	// FEN: White pawn on e2, Black pawn on e3 blocking double jump, white to move
	// 4k3/8/8/8/8/4p3/4P3/4K3 w - - 0 1
	pos, err := ParseFEN("4k3/8/8/8/8/4p3/4P3/4K3 w - - 0 1")
	if err != nil {
		t.Fatalf("Failed to parse FEN: %v", err)
	}

	// Generate moves
	moves := generateWhitePawnMoves(pos)

	// Verify that e2-e4 double jump is NOT generated
	doubleJumpFound := false
	for _, move := range moves {
		if move.Source() == E2 && move.Destination() == E4 {
			doubleJumpFound = true
			break
		}
	}

	if doubleJumpFound {
		t.Error("ERROR: Pawn double jump e2-e4 was generated despite e3 being blocked!")
		t.Error("Generated moves:")
		for _, move := range moves {
			if move.Source() == E2 {
				t.Errorf("  %s-%s", move.Source().String(), move.Destination().String())
			}
		}
	}

	// Verify that single push e2-e3 is also NOT generated (square occupied)
	singlePushFound := false
	for _, move := range moves {
		if move.Source() == E2 && move.Destination() == E3 {
			singlePushFound = true
			break
		}
	}

	if singlePushFound {
		t.Error("ERROR: Single pawn push e2-e3 was generated despite e3 being occupied!")
	}
}

// TestPawnDoubleJumpAllowed verifies legal double jumps still work
func TestPawnDoubleJumpAllowed(t *testing.T) {
	// FEN: White pawn on e2, clear path to e4, white to move
	// 4k3/8/8/8/8/8/4P3/4K3 w - - 0 1
	pos, err := ParseFEN("4k3/8/8/8/8/8/4P3/4K3 w - - 0 1")
	if err != nil {
		t.Fatalf("Failed to parse FEN: %v", err)
	}

	// Generate moves
	moves := generateWhitePawnMoves(pos)

	// Verify that e2-e4 double jump IS generated
	doubleJumpFound := false
	singlePushFound := false

	for _, move := range moves {
		if move.Source() == E2 && move.Destination() == E4 {
			doubleJumpFound = true
		}
		if move.Source() == E2 && move.Destination() == E3 {
			singlePushFound = true
		}
	}

	if !doubleJumpFound {
		t.Error("ERROR: Legal pawn double jump e2-e4 was NOT generated!")
		t.Error("Generated moves:")
		for _, move := range moves {
			if move.Source() == E2 {
				t.Errorf("  %s-%s", move.Source().String(), move.Destination().String())
			}
		}
	}

	if !singlePushFound {
		t.Error("ERROR: Legal single pawn push e2-e3 was NOT generated!")
	}
}

// TestBlackPawnDoubleJumpBlocked verifies black pawns also can't jump over pieces
func TestBlackPawnDoubleJumpBlocked(t *testing.T) {
	// FEN: Black pawn on e7, White pawn on e6 blocking double jump, black to move
	// 4k3/4p3/4P3/8/8/8/8/4K3 b - - 0 1
	pos, err := ParseFEN("4k3/4p3/4P3/8/8/8/8/4K3 b - - 0 1")
	if err != nil {
		t.Fatalf("Failed to parse FEN: %v", err)
	}

	// Generate moves
	moves := generateBlackPawnMoves(pos)

	// Verify that e7-e5 double jump is NOT generated
	doubleJumpFound := false
	for _, move := range moves {
		if move.Source() == E7 && move.Destination() == E5 {
			doubleJumpFound = true
			break
		}
	}

	if doubleJumpFound {
		t.Error("ERROR: Black pawn double jump e7-e5 was generated despite e6 being blocked!")
	}
}
