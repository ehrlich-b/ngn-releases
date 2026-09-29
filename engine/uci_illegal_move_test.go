package engine

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestUCIIllegalMoveInCheck(t *testing.T) {
	// Create a UCI engine instance
	var output bytes.Buffer
	engine := NewUCIEngine()

	// Initialize the engine
	engine.handleCommand("uci", &output)
	engine.handleCommand("isready", &output)

	// Set up the exact position from the game where the illegal move occurred
	// This is the position after 33...Rd1 34.Kf4 g5+ where White is in check
	fenPosition := "6k1/5p2/4p2p/2n1b1p1/2B1NK2/2P5/1R3n1P/3r4 w - g6 0 35"

	// First test: set position directly from FEN
	engine.handleCommand("position fen "+fenPosition, &output)

	// Verify the position is set correctly and White is in check
	if !isInCheck(engine.position, White) {
		t.Errorf("White should be in check in this position")
	}

	// Now search for a move
	output.Reset()
	engine.handleCommand("go depth 3", &output)

	// Wait for search to complete
	for engine.isSearching() {
		// Wait for search to finish
		time.Sleep(time.Millisecond)
	}

	// Parse the output to find the best move
	outputStr := output.String()
	lines := strings.Split(outputStr, "\n")
	var bestMove string
	for _, line := range lines {
		if strings.HasPrefix(line, "bestmove ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				bestMove = parts[1]
			}
			break
		}
	}

	if bestMove == "" {
		t.Errorf("Engine did not return a best move")
		return
	}

	t.Logf("Engine returned move: %s", bestMove)

	// Parse and validate the move
	move, err := ParseAlgebraicMove(bestMove, engine.position)
	if err != nil {
		t.Errorf("Failed to parse engine's move: %v", err)
		return
	}

	// Make the move and check if it's legal
	ep, tag, hc, _ := engine.position.MakeMove(move)
	if isInCheck(engine.position, White) {
		engine.position.UnMakeMove(move, tag, ep, hc)
		t.Errorf("ENGINE BUG: Returned illegal move %s that leaves king in check!", bestMove)
	} else {
		engine.position.UnMakeMove(move, tag, ep, hc)
		t.Logf("Move %s is legal and gets king out of check", bestMove)
	}
}

func TestUCISequentialMovesInCheck(t *testing.T) {
	// Test the exact move sequence from the game
	var output bytes.Buffer
	engine := NewUCIEngine()

	// Initialize
	engine.handleCommand("uci", &output)
	engine.handleCommand("isready", &output)

	// Start from position before the critical moves
	// Position after 33.Kd4
	startFen := "6k1/5p2/4p2p/2n1b3/2BKr3/2P5/1R3n1P/7R b - - 0 33"
	engine.handleCommand("position fen "+startFen, &output)

	// Apply the moves that led to the illegal move:
	// 33...Rd1 (Black moves rook to d1)
	// 34.Ke5 (White king to e5)
	// Actually, let me use the real moves from the game

	// Set position with moves that led to the problem
	// Note: The actual position from the game where the bug occurred
	// The pawn is on f7, and can't give check by moving to g5
	// Let's use the actual problematic position from the original game
	actualPosition := "position fen 6k1/5p2/4p2p/2n1b1p1/2B1rK2/2P5/1R3n1P/7R w - - 0 35"
	engine.handleCommand(actualPosition, &output)

	// Now White is in check from the g5 pawn
	if !isInCheck(engine.position, White) {
		t.Errorf("White should be in check after g7g5")
	}

	// Search for White's response
	output.Reset()
	engine.handleCommand("go depth 5", &output)
	// Wait for search to complete
	for engine.isSearching() {
		// Wait for search to finish
		time.Sleep(time.Millisecond)
	}

	// Check the response
	outputStr := output.String()
	lines := strings.Split(outputStr, "\n")
	var bestMove string
	for _, line := range lines {
		if strings.HasPrefix(line, "bestmove ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				bestMove = parts[1]
			}
			break
		}
	}

	t.Logf("Engine's response to check: %s", bestMove)

	// Verify it's legal
	if bestMove != "" {
		move, err := ParseAlgebraicMove(bestMove, engine.position)
		if err == nil {
			ep, tag, hc, _ := engine.position.MakeMove(move)
			if isInCheck(engine.position, White) {
				t.Errorf("CRITICAL BUG: Engine played %s which leaves king in check!", bestMove)
			}
			engine.position.UnMakeMove(move, tag, ep, hc)
		}
	}
}
