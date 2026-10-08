package engine

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestUCIBasicCommands(t *testing.T) {
	engine := NewUCIEngine()
	var output bytes.Buffer

	// Test UCI command
	engine.handleCommand("uci", &output)
	response := output.String()

	if !strings.Contains(response, "id name ngn") {
		t.Error("UCI response should contain engine name")
	}
	if !strings.Contains(response, "id author Bryan Ehrlich") {
		t.Error("UCI response should contain author")
	}
	if !strings.Contains(response, "uciok") {
		t.Error("UCI response should end with uciok")
	}

	// Test isready command
	output.Reset()
	engine.handleCommand("isready", &output)
	response = output.String()

	if !strings.Contains(response, "readyok") {
		t.Error("isready should respond with readyok")
	}
}

func TestUCIPosition(t *testing.T) {
	engine := NewUCIEngine()
	var output bytes.Buffer

	// Test startpos
	engine.handleCommand("position startpos", &output)

	// Verify starting position is set
	if engine.position.Turn() != White {
		t.Error("Starting position should be white to move")
	}

	// Test position with moves
	engine.handleCommand("position startpos moves e2e4 e7e5", &output)

	// The position should have changed
	if engine.position.Board.PieceAt(E2) != NoPiece {
		t.Error("Pawn should have moved from e2")
	}
	if engine.position.Board.PieceAt(E4) != WhitePawn {
		t.Error("White pawn should be on e4")
	}
}

func TestUCIFEN(t *testing.T) {
	engine := NewUCIEngine()
	var output bytes.Buffer

	// Test FEN position
	fenString := "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1"
	command := "position fen " + fenString
	engine.handleCommand(command, &output)

	// Verify the position was set correctly
	if engine.position.Board.PieceAt(E4) != WhitePawn {
		t.Error("White pawn should be on e4 from FEN")
	}
	if engine.position.Turn() != Black {
		t.Error("Should be black to move from FEN")
	}
	// After 1.e4 no black pawn can capture en passant, so the e3 target is a
	// phantom: it is gated out so it cannot pollute the position hash / repetition
	// signature (Stockfish reports "-" here too). A legitimate, capturable EP
	// square from FEN is covered by TestEnPassantHashCapturabilityGate.
	if engine.position.EnPassant != NoSquare {
		t.Errorf("phantom en passant square should be gated to NoSquare, got %v", engine.position.EnPassant)
	}
}

func TestUCIDebugMode(t *testing.T) {
	engine := NewUCIEngine()
	var output bytes.Buffer

	// Enable debug mode
	engine.handleCommand("debug on", &output)
	if !engine.debugMode {
		t.Error("Debug mode should be enabled")
	}

	// Test that debug info is printed
	output.Reset()
	engine.handleCommand("unknown_command", &output)
	response := output.String()

	if !strings.Contains(response, "unknown command") {
		t.Error("Debug mode should print unknown command info")
	}

	// Disable debug mode
	engine.handleCommand("debug off", &output)
	if engine.debugMode {
		t.Error("Debug mode should be disabled")
	}
}

func TestUCINewGame(t *testing.T) {
	engine := NewUCIEngine()
	var output bytes.Buffer

	// Make some moves
	engine.handleCommand("position startpos moves e2e4 e7e5", &output)

	// Start new game
	engine.handleCommand("ucinewgame", &output)

	// Should be back to starting position
	if engine.position.Turn() != White {
		t.Error("New game should reset to white to move")
	}
	if engine.position.Board.PieceAt(E2) != WhitePawn {
		t.Error("New game should reset pieces to starting position")
	}
}

func TestUCIHashOptionResizesTable(t *testing.T) {
	engine := NewUCIEngine()
	var output bytes.Buffer

	engine.handleCommand("setoption name Hash value 1", &output)
	if engine.searcher.HashSize() != 1 {
		t.Fatalf("TT size = %d MB, want 1 MB", engine.searcher.HashSize())
	}

	engine.handleCommand("ucinewgame", &output)
	if engine.searcher.HashSize() != 1 {
		t.Fatalf("ucinewgame reset TT to %d MB, want to preserve 1 MB Hash option", engine.searcher.HashSize())
	}
}

func TestUCIGo(t *testing.T) {
	engine := NewUCIEngine()
	var output bytes.Buffer

	// Set up a simple position
	engine.handleCommand("position startpos", &output)
	output.Reset()

	// Start search with depth 2 (fast)
	engine.handleCommand("go depth 2", &output)

	// Wait for search to complete
	for engine.isSearching() {
		time.Sleep(10 * time.Millisecond)
	}
	response := output.String()
	if !strings.Contains(response, "info depth") {
		t.Error("Should provide search info")
	}
	if !strings.Contains(response, "bestmove") {
		t.Error("Should provide best move")
	}
}

func TestUCIStop(t *testing.T) {
	engine := NewUCIEngine()
	var output bytes.Buffer

	// Start a search
	go func() {
		engine.handleCommand("go depth 10", &output) // Deeper search
	}()

	// Wait a bit then stop
	time.Sleep(50 * time.Millisecond)
	engine.handleCommand("stop", &output)

	// Should stop relatively quickly
	timeout := time.After(2 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Error("Search did not stop in reasonable time")
			return
		case <-ticker.C:
			if !engine.isSearching() {
				return // Success
			}
		}
	}
}

func BenchmarkUCIPosition(b *testing.B) {
	engine := NewUCIEngine()
	var output bytes.Buffer

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.handleCommand("position startpos moves e2e4 e7e5 g1f3", &output)
		output.Reset()
	}
}

func BenchmarkUCIGo(b *testing.B) {
	engine := NewUCIEngine()
	var output bytes.Buffer

	engine.handleCommand("position startpos", &output)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.handleCommand("go depth 1", &output)

		// Wait for search to complete
		for engine.isSearching() {
			time.Sleep(1 * time.Millisecond)
		}
		output.Reset()
	}
}

func TestUCIMoveOverheadOptionAndNewGamePersistence(t *testing.T) {
	engine := NewUCIEngine()
	var output bytes.Buffer
	if got := engine.timeManager.moveOverhead; got != 100*time.Millisecond {
		t.Fatalf("UCI default move overhead=%v, want advertised 100ms", got)
	}

	engine.handleCommand("setoption name Move Overhead value 275", &output)
	if got := engine.timeManager.moveOverhead; got != 275*time.Millisecond {
		t.Fatalf("configured move overhead=%v, want 275ms", got)
	}
	for _, command := range []string{
		"setoption name Move Overhead value -1",
		"setoption name Move Overhead value 5001",
		"setoption name Move Overhead value invalid",
	} {
		engine.handleCommand(command, &output)
		if got := engine.timeManager.moveOverhead; got != 275*time.Millisecond {
			t.Fatalf("%q changed move overhead to %v", command, got)
		}
	}

	engine.handleCommand("ucinewgame", &output)
	if got := engine.timeManager.moveOverhead; got != 275*time.Millisecond {
		t.Fatalf("ucinewgame reset configured move overhead to %v", got)
	}
}
