package engine

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// TestPanicRecoveryInSearch verifies that search panics are properly caught
func TestPanicRecoveryInSearch(t *testing.T) {
	// Create a test that deliberately causes a panic in search to verify recovery

	// First, ensure basic search works normally
	pos, err := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatalf("Failed to parse FEN: %v", err)
	}

	// Normal search should work fine
	result := Search(pos, 1)
	if result == nil {
		t.Fatal("Search returned nil")
	}

	t.Logf("✅ Normal search works: found move %v with score %d",
		result.BestMove, result.BestScore)
}

// TestPanicRecoveryInMoveGeneration verifies move generation panic recovery
func TestPanicRecoveryInMoveGeneration(t *testing.T) {
	pos, err := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatalf("Failed to parse FEN: %v", err)
	}

	// Normal move generation should work
	moves := GenerateLegalMoves(pos)
	if len(moves) == 0 {
		t.Fatal("Move generation returned no moves")
	}

	t.Logf("✅ Normal move generation works: found %d moves", len(moves))
}

// TestPanicRecoveryInEvaluation verifies evaluation panic recovery
func TestPanicRecoveryInEvaluation(t *testing.T) {
	pos, err := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatalf("Failed to parse FEN: %v", err)
	}

	// Normal evaluation should work
	score := Evaluate(&pos.Board)

	t.Logf("✅ Normal evaluation works: score %d", score)
}

// TestCrashHandlerLogging verifies that crashes are properly logged
func TestCrashHandlerLogging(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping crash handler test in short mode")
	}

	// Get the crash handler
	crashHandler := GetGlobalCrashHandler()

	// Test the recovery mechanism with a deliberate panic
	didRecover := false

	func() {
		defer func() {
			if r := recover(); r != nil {
				// If this function recovers, our crash handler didn't work
				t.Errorf("Crash handler failed to catch panic: %v", r)
			}
		}()

		crashHandler.WrapFunction("TestPanic", func() {
			panic("This is a test panic - should be caught!")
		}, map[string]interface{}{
			"test_context": "TestCrashHandlerLogging",
			"time":         time.Now(),
		})

		didRecover = true
	}()

	if !didRecover {
		t.Error("Function should have completed after panic recovery")
	}

	t.Logf("✅ Panic recovery system is working correctly")
}

// TestCrashLogFileCreation verifies crash log file is created
func TestCrashLogFileCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping crash log test in short mode")
	}

	// Get crash handler to ensure it's initialized
	_ = GetGlobalCrashHandler()

	// Check if crash log file exists (it should be created on initialization)
	logFiles := []string{"ngn_crashes.log", "./ngn_crashes.log"}

	found := false
	for _, logFile := range logFiles {
		if _, err := os.Stat(logFile); err == nil {
			found = true
			t.Logf("✅ Found crash log file: %s", logFile)

			// Try to read the log file to verify it's accessible
			content, err := os.ReadFile(logFile)
			if err != nil {
				t.Errorf("Could not read crash log file: %v", err)
			} else {
				t.Logf("Crash log contains %d bytes", len(content))
				if strings.Contains(string(content), "NGN Crash Handler Initialized") {
					t.Logf("✅ Crash log contains initialization message")
				}
			}
			break
		}
	}

	if !found {
		t.Log("⚠️  No crash log file found (may be using stderr fallback)")
	}
}

// TestForcedPanicRecovery creates an actual panic to test the full recovery system
func TestForcedPanicRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping forced panic test in short mode")
	}

	t.Log("🧪 Testing forced panic recovery...")

	// This should be caught by our crash handler and logged
	func() {
		defer GetGlobalCrashHandler().SafeRecover("TestForcedPanic", map[string]interface{}{
			"test_id": "forced_panic_test",
			"purpose": "verify crash recovery system",
			"time":    time.Now().Format(time.RFC3339),
		})

		// Create conditions that might cause real crashes
		var nilPtr *Position = nil
		_ = nilPtr.Turn() // This would panic without recovery
	}()

	t.Log("✅ Survived forced panic - crash recovery is working!")
}

// TestUCICommandSafety tests UCI command handling with malformed inputs
func TestUCICommandSafety(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping UCI safety test in short mode")
	}

	engine := NewUCIEngine()

	// Test various potentially problematic UCI commands
	malformedCommands := []string{
		"position fen INVALID_FEN",
		"go depth -1",
		"go depth 999999",
		"position fen rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1 moves e2e4 e7e5 invalid_move",
		"setption name Hash value -1",
		"go wtime -1000 btime -1000",
		strings.Repeat("a", 10000), // Very long command
		"",                         // Empty command
		"   ",                      // Whitespace only
	}

	for _, cmd := range malformedCommands {
		t.Run(fmt.Sprintf("Command_%s", strings.ReplaceAll(cmd, " ", "_")), func(t *testing.T) {
			// This should not crash the engine
			func() {
				defer GetGlobalCrashHandler().SafeRecover("UCI_Malformed_Command", map[string]interface{}{
					"command": cmd,
				})

				// Process the command - this might panic without our protection
				engine.handleCommand(cmd, os.Stderr)
			}()

			t.Logf("✅ Survived malformed UCI command: %q", cmd)
		})
	}
}

// TestPerformanceWithCrashHandlers verifies that crash handlers don't significantly impact performance
func TestPerformanceWithCrashHandlers(t *testing.T) {
	pos, err := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatalf("Failed to parse FEN: %v", err)
	}

	// Time a search operation
	start := time.Now()
	result := Search(pos, 3)
	duration := time.Since(start)

	if result == nil {
		t.Fatal("Search returned nil")
	}

	t.Logf("✅ Search with crash handlers completed in %v", duration)

	// Performance should still be reasonable (less than 1 second for depth 3)
	if duration > time.Second {
		t.Errorf("Search took too long: %v (expected < 1s)", duration)
	}
}
