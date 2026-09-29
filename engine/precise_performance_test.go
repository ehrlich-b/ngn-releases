package engine

import (
	"testing"
	"time"
)

func TestPrecise5SecondPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping 5-second performance test in short mode")
	}

	pos, err := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatalf("Failed to parse FEN: %v", err)
	}

	t.Log("Starting 5-second performance test...")

	// Set up time manager for exactly 5 seconds
	timeManager := NewTimeManager()
	params := SearchParams{
		MoveTime: 5000, // 5 seconds in milliseconds
	}
	timeManager.SetTimeControl(params, true)

	// Record precise start time
	startTime := time.Now()

	// Run fixed depth search with time limit - use high depth, will be limited by time
	info := SearchFixed(pos, 20, timeManager) // High depth, will be limited by time manager

	// Record precise end time
	endTime := time.Now()
	actualElapsed := endTime.Sub(startTime)

	// Calculate precise nodes per second
	actualSeconds := actualElapsed.Seconds()
	nps := float64(info.Nodes) / actualSeconds

	// Report results
	t.Logf("=== PRECISE 5-SECOND PERFORMANCE TEST ===")
	t.Logf("Planned time: 5.000 seconds")
	t.Logf("Actual time:  %.3f seconds", actualSeconds)
	t.Logf("Total nodes:  %d", info.Nodes)
	t.Logf("Depth reached: %d", info.Depth)
	t.Logf("Best move: %s", info.BestMove.ToString())
	t.Logf("Nodes per second: %.0f", nps)

	// Performance assessment
	if actualElapsed > 10*time.Second {
		t.Fatalf("❌ ENGINE PROBLEM: Test ran for %.1f seconds (expected ~5s) - search not respecting time limits", actualElapsed.Seconds())
	}

	if actualElapsed < 3*time.Second {
		t.Errorf("⚠️  Test ended too early (%.1f seconds) - possible time management issue", actualElapsed.Seconds())
	}

	// Final assessment
	t.Logf("\n=== PERFORMANCE VERDICT ===")
	if nps >= 100000 {
		t.Logf("✅ PERFORMANCE ACCEPTABLE: %.0f nps (≥100K threshold)", nps)
		t.Log("   Recommendation: Continue with engine features")
	} else if nps >= 50000 {
		t.Logf("⚠️  PERFORMANCE MARGINAL: %.0f nps (50K-100K range)", nps)
		t.Log("   Recommendation: Some optimization may be needed")
	} else {
		t.Logf("❌ PERFORMANCE INSUFFICIENT: %.0f nps (<50K)", nps)
		t.Log("   Recommendation: Optimization required before continuing")
	}

	// Additional diagnostics
	if info.Nodes == 0 {
		t.Fatal("❌ CRITICAL: No nodes were searched!")
	}

	if info.Depth < 4 {
		t.Logf("⚠️  Low depth reached in 5s: %d (expected 5-8)", info.Depth)
	} else if info.Depth >= 8 {
		t.Logf("🚀 Good depth reached in 5s: %d", info.Depth)
	} else {
		t.Logf("✅ Reasonable depth reached in 5s: %d", info.Depth)
	}
}
