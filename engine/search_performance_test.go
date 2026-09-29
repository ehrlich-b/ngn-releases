package engine

import (
	"testing"
	"time"
)

func BenchmarkSearchPerformance(b *testing.B) {
	// Test search performance from starting position
	pos, err := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		b.Fatalf("Failed to parse FEN: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		info := SearchFixed(pos, 4, nil) // Fixed depth 4
		if info.Nodes == 0 {
			b.Fatal("No nodes searched")
		}
	}
}

func TestSearchNodesPerSecond(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	positions := []struct {
		name  string
		fen   string
		depth int
	}{
		{
			name:  "Starting position",
			fen:   "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
			depth: 4, // Reduced depth for faster testing
		},
		{
			name:  "Middlegame position",
			fen:   "r1bqkb1r/pppp1ppp/2n2n2/4p3/2B1P3/3P1N2/PPP2PPP/RNBQK2R w KQkq - 0 4",
			depth: 3, // Reduced depth for faster testing
		},
		{
			name:  "Tactical position",
			fen:   "r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
			depth: 3, // Reduced depth for faster testing
		},
	}

	for _, pos := range positions {
		t.Run(pos.name, func(t *testing.T) {
			position, err := ParseFEN(pos.fen)
			if err != nil {
				t.Fatalf("Failed to parse FEN: %v", err)
			}

			start := time.Now()
			info := SearchFixed(position, pos.depth, nil)
			elapsed := time.Since(start)

			if info.Nodes == 0 {
				t.Fatal("No nodes searched")
			}

			nps := float64(info.Nodes) / elapsed.Seconds()

			t.Logf("%s at depth %d:", pos.name, pos.depth)
			t.Logf("  Nodes: %d", info.Nodes)
			t.Logf("  Time: %v", elapsed)
			t.Logf("  NPS: %.0f", nps)
			t.Logf("  Best move: %s", info.BestMove.ToString())
			t.Logf("  Score: %d", info.BestScore)

			// Rough performance expectations:
			// - Basic engines: 50K-200K nps
			// - Good engines: 500K-2M nps
			// - Top engines: 5M+ nps

			if nps < 10000 {
				t.Logf("⚠️  Low performance: %.0f nps (< 10K)", nps)
			} else if nps < 100000 {
				t.Logf("📊 Moderate performance: %.0f nps", nps)
			} else {
				t.Logf("✅ Good performance: %.0f nps", nps)
			}
		})
	}
}

func TestSearchDepthReached(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping depth reach test in short mode")
	}

	// Test what depth we can reach in a reasonable time
	pos, err := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatalf("Failed to parse FEN: %v", err)
	}

	timeManager := NewTimeManager()
	params := SearchParams{
		MoveTime: 1000, // 1 second
	}
	timeManager.SetTimeControl(params, true)

	start := time.Now()
	info := SearchIterativeDeepening(pos, 20, timeManager) // Try up to depth 20
	elapsed := time.Since(start)

	nps := float64(info.Nodes) / elapsed.Seconds()

	t.Logf("Iterative deepening in 1 second:")
	t.Logf("  Depth reached: %d", info.Depth)
	t.Logf("  Nodes: %d", info.Nodes)
	t.Logf("  Time: %v", elapsed)
	t.Logf("  NPS: %.0f", nps)
	t.Logf("  Best move: %s", info.BestMove.ToString())

	// Expected depth ranges:
	// - 1000ms: should reach depth 6-8 for basic engine
	// - Strong engines reach depth 8-12 in 1 second

	if info.Depth < 4 {
		t.Logf("⚠️  Very low depth: %d", info.Depth)
	} else if info.Depth < 6 {
		t.Logf("📊 Low depth: %d", info.Depth)
	} else if info.Depth < 8 {
		t.Logf("✅ Reasonable depth: %d", info.Depth)
	} else {
		t.Logf("🚀 Good depth: %d", info.Depth)
	}
}
