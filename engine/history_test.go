package engine

import (
	"testing"
)

func TestHistoryHeuristic(t *testing.T) {
	// Clear history table before test
	ClearHistoryTable()

	// Create a simple quiet move
	move := NewMove(E2, E4, WhitePawn, NoPiece, NoType, 0)

	// Initially, history score should be 0
	initialScore := GetHistoryScore(move, EmptyMove, EmptyMove)
	if initialScore != 0 {
		t.Errorf("Expected initial history score to be 0, got %d", initialScore)
	}

	// Update history table with a cutoff at depth 3
	UpdateHistoryTable(move, EmptyMove, EmptyMove,3)

	// History score should now be depth^2 = 9
	updatedScore := GetHistoryScore(move, EmptyMove, EmptyMove)
	expectedScore := 3 * 3 // depth^2
	if updatedScore != expectedScore {
		t.Errorf("Expected history score to be %d, got %d", expectedScore, updatedScore)
	}

	// Update again at depth 2
	UpdateHistoryTable(move, EmptyMove, EmptyMove,2)

	// History score should now be 9 + 4 = 13
	finalScore := GetHistoryScore(move, EmptyMove, EmptyMove)
	expectedFinalScore := 9 + 4
	if finalScore != expectedFinalScore {
		t.Errorf("Expected final history score to be %d, got %d", expectedFinalScore, finalScore)
	}

	// Test that captures don't get history scores
	captureMove := NewMove(E4, D5, WhitePawn, BlackPawn, NoType, Capture)
	UpdateHistoryTable(captureMove, EmptyMove, EmptyMove, 5)
	captureScore := GetHistoryScore(captureMove, EmptyMove, EmptyMove)
	if captureScore != 0 {
		t.Errorf("Expected capture move to have 0 history score, got %d", captureScore)
	}

	t.Logf("✓ History heuristic working: move %s has score %d", move.ToString(), finalScore)
}

func TestHistoryTableClear(t *testing.T) {
	// Create and update a move
	move := NewMove(D2, D4, WhitePawn, NoPiece, NoType, 0)
	UpdateHistoryTable(move, EmptyMove, EmptyMove,4)

	// Verify it has a score
	score := GetHistoryScore(move, EmptyMove, EmptyMove)
	if score == 0 {
		t.Error("Expected move to have non-zero history score after update")
	}

	// Clear the table
	ClearHistoryTable()

	// Score should now be 0
	clearedScore := GetHistoryScore(move, EmptyMove, EmptyMove)
	if clearedScore != 0 {
		t.Errorf("Expected cleared history score to be 0, got %d", clearedScore)
	}

	t.Log("✓ History table clearing working")
}

func TestHistoryInMoveOrdering(t *testing.T) {
	// Clear history table
	ClearHistoryTable()

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

	// Give move1 a history advantage
	UpdateHistoryTable(move1, EmptyMove, EmptyMove, 5) // score = 25
	UpdateHistoryTable(move2, EmptyMove, EmptyMove, 2) // score = 4

	// Create a list with both moves
	moves := []Move{move2, move1} // move2 first, then move1

	// Order the moves
	orderedMoves := orderMoves(moves, pos)

	// move1 should come first due to higher history score
	if len(orderedMoves) != 2 {
		t.Fatalf("Expected 2 moves, got %d", len(orderedMoves))
	}

	// The move with higher history score should come first
	if orderedMoves[0] != move1 {
		t.Errorf("Expected move1 with higher history score first, got %s", orderedMoves[0].ToString())
	}

	t.Log("✓ History affects move ordering")
}
