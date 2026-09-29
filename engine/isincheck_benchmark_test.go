package engine

import (
	"testing"
	"time"
)

// BenchmarkIsInCheck benchmarks the optimized isInCheck function specifically
func BenchmarkIsInCheck(b *testing.B) {
	// Position where the king is exposed to many potential checks
	pos, _ := ParseFEN("4k3/8/8/8/8/8/8/R2QKB1R w - - 0 1")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// This exercises all the sliding piece check detection paths
		isInCheck(pos, White)
		isInCheck(pos, Black)
	}
}

// TestIsInCheckPerformance specifically measures isInCheck performance
func TestIsInCheckPerformance(t *testing.T) {
	t.Log("=== ISINCHECK OPTIMIZATION BENCHMARK ===")

	// Position with many potential checks - king in center with sliding pieces around
	pos, err := ParseFEN("4k3/2q5/8/3r4/3K4/8/2b5/8 w - - 0 1")
	if err != nil {
		t.Fatalf("Failed to parse FEN: %v", err)
	}

	// Warm up
	for i := 0; i < 1000; i++ {
		isInCheck(pos, White)
		isInCheck(pos, Black)
	}

	// Measure isInCheck performance specifically
	iterations := 1000000
	start := time.Now()
	for i := 0; i < iterations; i++ {
		isInCheck(pos, White) // King is in check from multiple pieces
		isInCheck(pos, Black) // King not in check
	}
	elapsed := time.Since(start)

	checksPerSecond := float64(iterations*2) / elapsed.Seconds()

	t.Logf("IsInCheck performance:")
	t.Logf("  Checks performed: %d", iterations*2)
	t.Logf("  Time: %v", elapsed)
	t.Logf("  Checks per second: %.0f", checksPerSecond)
	t.Logf("  Nanoseconds per check: %.2f ns", float64(elapsed.Nanoseconds())/float64(iterations*2))

	// Target: >1M checks per second (optimized should be much faster)
	if checksPerSecond < 1000000 {
		t.Logf("⚠️  Below optimization target: %.0f checks/sec (need >1M)", checksPerSecond)
	} else if checksPerSecond < 5000000 {
		t.Logf("✅ Good optimization: %.0f checks/sec", checksPerSecond)
	} else {
		t.Logf("🚀 Excellent optimization: %.0f checks/sec", checksPerSecond)
	}
}

// TestIsInCheckIntensiveSearchImpact tests impact on search performance
func TestIsInCheckIntensiveSearchImpact(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping intensive test in short mode")
	}

	t.Log("=== ISINCHECK IMPACT ON SEARCH PERFORMANCE ===")

	// Position that generates many positions needing check detection
	// This is a tactical position with many pieces that will cause lots of isInCheck calls
	pos, err := ParseFEN("r1bqr1k1/ppp2ppp/2n2n2/2b1p3/2B1P3/3P1N2/PPP2PPP/RNBQ1RK1 w - - 0 8")
	if err != nil {
		t.Fatalf("Failed to parse FEN: %v", err)
	}

	// Measure search performance on position with many checks
	start := time.Now()
	info := SearchFixed(pos, 4, nil) // Depth 4 to get meaningful data
	elapsed := time.Since(start)

	nps := float64(info.Nodes) / elapsed.Seconds()

	t.Logf("Tactical position search (intensive isInCheck usage):")
	t.Logf("  Nodes: %d", info.Nodes)
	t.Logf("  Time: %v", elapsed)
	t.Logf("  NPS: %.0f", nps)
	t.Logf("  Best move: %s", info.BestMove.ToString())

	// With optimized isInCheck, we should maintain good NPS even in check-heavy positions
	if nps < 200000 {
		t.Logf("❌ Poor performance in check-heavy position: %.0f NPS", nps)
		t.Logf("This suggests isInCheck optimization didn't work or regressed")
	} else if nps < 400000 {
		t.Logf("📊 Moderate performance: %.0f NPS", nps)
	} else {
		t.Logf("✅ Good performance in check-heavy position: %.0f NPS", nps)
		t.Logf("isInCheck optimization appears effective!")
	}

	// Calculate estimated isInCheck calls per search
	// Rough estimate: 1 isInCheck call per move made/unmade in search
	estimatedChecks := info.Nodes * 2 // Conservative estimate
	checksPerSecond := float64(estimatedChecks) / elapsed.Seconds()
	t.Logf("  Estimated checks per second during search: %.0f", checksPerSecond)
}
