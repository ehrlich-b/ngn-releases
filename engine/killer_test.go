package engine

import (
	"testing"
)

func TestKillerMoveHeuristic(t *testing.T) {
	// Clear killer moves before test
	ClearKillerMoves()

	// Create some test moves
	move1 := NewMove(E2, E4, WhitePawn, NoPiece, NoType, 0)
	move2 := NewMove(D2, D4, WhitePawn, NoPiece, NoType, 0)
	move3 := NewMove(G1, F3, WhiteKnight, NoPiece, NoType, 0)

	depth := 5

	// Initially, no moves should be killer moves
	if IsKillerMove(move1, depth) {
		t.Error("Move should not be killer move initially")
	}

	// Update killer moves
	UpdateKillerMoves(move1, depth)

	// move1 should now be a killer move at depth 5
	if !IsKillerMove(move1, depth) {
		t.Error("Move1 should be a killer move after update")
	}

	// move1 should not be a killer move at different depth
	if IsKillerMove(move1, depth+1) {
		t.Error("Move1 should not be killer move at different depth")
	}

	// Update with move2
	UpdateKillerMoves(move2, depth)

	// Both moves should be killer moves
	if !IsKillerMove(move1, depth) {
		t.Error("Move1 should still be a killer move")
	}
	if !IsKillerMove(move2, depth) {
		t.Error("Move2 should be a killer move")
	}

	// Add third move - should push out the first one
	UpdateKillerMoves(move3, depth)

	// move3 should be first killer, move2 should be second, move1 should be removed
	if IsKillerMove(move1, depth) {
		t.Error("Move1 should no longer be a killer move (pushed out)")
	}
	if !IsKillerMove(move2, depth) {
		t.Error("Move2 should still be a killer move")
	}
	if !IsKillerMove(move3, depth) {
		t.Error("Move3 should be a killer move")
	}

	// Test that captures don't become killer moves
	captureMove := NewMove(E4, D5, WhitePawn, BlackPawn, NoType, Capture)
	UpdateKillerMoves(captureMove, depth)
	if IsKillerMove(captureMove, depth) {
		t.Error("Capture moves should not become killer moves")
	}

	t.Log("✓ Killer move heuristic working")
}

func TestKillerMoveClear(t *testing.T) {
	// Set up some killer moves
	move := NewMove(E2, E4, WhitePawn, NoPiece, NoType, 0)
	depth := 3

	UpdateKillerMoves(move, depth)

	// Verify it's a killer move
	if !IsKillerMove(move, depth) {
		t.Error("Move should be a killer move before clear")
	}

	// Clear killer moves
	ClearKillerMoves()

	// Should no longer be a killer move
	if IsKillerMove(move, depth) {
		t.Error("Move should not be killer move after clear")
	}

	t.Log("✓ Killer move clearing working")
}

func TestKillerMoveInOrdering(t *testing.T) {
	// Clear killer moves
	ClearKillerMoves()

	// Create a simple test position
	pos := &Position{}
	pos.Board = Bitboard{}
	pos.Board.UpdateSquare(E1, WhiteKing, NoPiece)
	pos.Board.UpdateSquare(E8, BlackKing, NoPiece)
	pos.SetTag(WhiteToMove)
	pos.EnPassant = NoSquare
	pos.HalfMoveClock = 0

	// Create two quiet moves
	move1 := NewMove(E2, E4, WhitePawn, NoPiece, NoType, 0)
	move2 := NewMove(D2, D4, WhitePawn, NoPiece, NoType, 0)

	depth := 4

	// Make move1 a killer move
	UpdateKillerMoves(move1, depth)

	// Create a list with both moves (move2 first)
	moves := []Move{move2, move1}

	// Order the moves with depth
	orderedMoves := orderMovesWithDepth(moves, depth, pos)

	// move1 should come first due to being a killer move
	if len(orderedMoves) != 2 {
		t.Fatalf("Expected 2 moves, got %d", len(orderedMoves))
	}

	// The ordered list should have killer move first
	if orderedMoves[0] != move1 {
		t.Errorf("Expected killer move %s to be first, got %s",
			move1.ToString(), orderedMoves[0].ToString())
	}

	t.Log("✓ Killer moves affect ordering")
}

func TestKillerMoveNegativeDepth(t *testing.T) {
	// Test that negative depths don't cause panics
	move := NewMove(E2, E4, WhitePawn, NoPiece, NoType, 0)

	// These should not panic and should return false
	result1 := IsKillerMove(move, -1)
	result2 := IsKillerMove(move, -10)

	if result1 || result2 {
		t.Error("Negative depths should return false for killer moves")
	}

	// Updating with negative depth should not panic
	UpdateKillerMoves(move, -1)
	UpdateKillerMoves(move, -5)

	// Should still not be killer moves
	if IsKillerMove(move, -1) || IsKillerMove(move, 3) {
		t.Error("Negative depth updates should not affect killer move status")
	}

	t.Log("✓ Negative depth safety checks working")
}
