package engine

import (
	"testing"
)

func TestCounterMoveHeuristic(t *testing.T) {
	// Clear counter moves before test
	ClearCounterMoves()

	// Create test moves
	previousMove := NewMove(E2, E4, WhitePawn, NoPiece, NoType, 0)
	counterMove := NewMove(E7, E5, BlackPawn, NoPiece, NoType, 0)
	otherMove := NewMove(D7, D6, BlackPawn, NoPiece, NoType, 0)

	// Initially, no move should be a counter move
	if IsCounterMove(counterMove, previousMove) {
		t.Error("Move should not be counter move initially")
	}

	// Update counter move
	UpdateCounterMove(counterMove, previousMove)

	// counterMove should now be the counter move to previousMove
	if !IsCounterMove(counterMove, previousMove) {
		t.Error("Move should be counter move after update")
	}

	// otherMove should not be a counter move
	if IsCounterMove(otherMove, previousMove) {
		t.Error("Other move should not be counter move")
	}

	// Test retrieval
	retrieved := GetCounterMove(previousMove)
	if retrieved != counterMove {
		t.Errorf("Expected counter move %s, got %s", counterMove.ToString(), retrieved.ToString())
	}

	// Test that captures don't become counter moves
	captureMove := NewMove(D4, E5, WhitePawn, BlackPawn, NoType, Capture)
	UpdateCounterMove(captureMove, previousMove)
	// Should still be the original counter move, not the capture
	if GetCounterMove(previousMove) != counterMove {
		t.Error("Capture moves should not become counter moves")
	}

	t.Log("✓ Counter move heuristic working")
}

func TestCounterMoveWithLastMove(t *testing.T) {
	// Clear counter moves
	ClearCounterMoves()

	// Set up a sequence
	move1 := NewMove(E2, E4, WhitePawn, NoPiece, NoType, 0)
	move2 := NewMove(E7, E5, BlackPawn, NoPiece, NoType, 0)

	// Set last move played
	SetLastMovePlayed(move1)

	// Verify we can get it back
	if GetLastMovePlayed() != move1 {
		t.Errorf("Expected last move %s, got %s", move1.ToString(), GetLastMovePlayed().ToString())
	}

	// Update counter move based on last move
	UpdateCounterMove(move2, GetLastMovePlayed())

	// Check if move2 is now a counter to move1
	if !IsCounterMove(move2, move1) {
		t.Error("Move2 should be counter to move1")
	}

	t.Log("✓ Counter move with last move tracking working")
}

func TestCounterMoveClear(t *testing.T) {
	// Set up counter moves
	move1 := NewMove(E2, E4, WhitePawn, NoPiece, NoType, 0)
	move2 := NewMove(E7, E5, BlackPawn, NoPiece, NoType, 0)

	SetLastMovePlayed(move1)
	UpdateCounterMove(move2, move1)

	// Verify it's set
	if !IsCounterMove(move2, move1) {
		t.Error("Counter move should be set before clear")
	}

	// Clear counter moves
	ClearCounterMoves()

	// Should no longer be a counter move
	if IsCounterMove(move2, move1) {
		t.Error("Counter move should be cleared")
	}

	// Last move should also be cleared
	if GetLastMovePlayed() != EmptyMove {
		t.Error("Last move should be cleared")
	}

	t.Log("✓ Counter move clearing working")
}

func TestCounterMoveInOrdering(t *testing.T) {
	// Clear everything
	ClearCounterMoves()

	// Create a simple test position
	pos := &Position{}
	pos.Board = Bitboard{}
	pos.Board.UpdateSquare(E1, WhiteKing, NoPiece)
	pos.Board.UpdateSquare(E8, BlackKing, NoPiece)
	pos.SetTag(WhiteToMove)
	pos.EnPassant = NoSquare
	pos.HalfMoveClock = 0

	// Create moves
	previousMove := NewMove(D2, D4, WhitePawn, NoPiece, NoType, 0)
	counterMove := NewMove(D7, D5, BlackPawn, NoPiece, NoType, 0)
	normalMove := NewMove(E7, E6, BlackPawn, NoPiece, NoType, 0)

	// Set up counter move
	SetLastMovePlayed(previousMove)
	UpdateCounterMove(counterMove, previousMove)

	// Test move ordering
	moves := []Move{normalMove, counterMove} // normalMove first
	orderedMoves := orderMovesWithDepth(moves, 3, pos)

	if len(orderedMoves) != 2 {
		t.Fatalf("Expected 2 moves, got %d", len(orderedMoves))
	}

	// Check that counter move is first in ordered list
	if orderedMoves[0] != counterMove {
		t.Errorf("Expected counter move %s first, got %s",
			counterMove.ToString(), orderedMoves[0].ToString())
	}

	t.Log("✓ Counter moves affect ordering")
}
