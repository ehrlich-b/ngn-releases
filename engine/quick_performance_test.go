package engine

import (
	"testing"
	"time"
)

func TestQuickSearchPerformance(t *testing.T) {
	// Quick performance test - depth 3 only
	pos, err := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatalf("Failed to parse FEN: %v", err)
	}

	start := time.Now()
	info := SearchFixed(pos, 3, nil) // Very shallow depth
	elapsed := time.Since(start)

	if elapsed > 5*time.Second {
		t.Fatal("Search took too long (>5s) - likely performance issue")
	}

	nps := float64(info.Nodes) / elapsed.Seconds()

	t.Logf("Quick performance test (depth 3):")
	t.Logf("  Nodes: %d", info.Nodes)
	t.Logf("  Time: %v", elapsed)
	t.Logf("  NPS: %.0f", nps)
	t.Logf("  Best move: %s", info.BestMove.ToString())

	// Performance analysis:
	// For reference, top engines achieve:
	// - Stockfish: 5-20M nps
	// - Komodo: 2-8M nps
	// - Basic engines: 100K-1M nps

	if nps < 10000 {
		t.Logf("❌ VERY LOW performance: %.0f nps", nps)
		t.Log("Recommendation: Major optimization needed")
	} else if nps < 50000 {
		t.Logf("⚠️  LOW performance: %.0f nps", nps)
		t.Log("Recommendation: Optimization needed for competitive play")
	} else if nps < 200000 {
		t.Logf("📊 MODERATE performance: %.0f nps", nps)
		t.Log("Recommendation: Can be competitive with more search improvements")
	} else if nps < 1000000 {
		t.Logf("✅ GOOD performance: %.0f nps", nps)
		t.Log("Recommendation: Focus on engine features over micro-optimization")
	} else {
		t.Logf("🚀 EXCELLENT performance: %.0f nps", nps)
		t.Log("Recommendation: Performance is not the bottleneck")
	}
}
