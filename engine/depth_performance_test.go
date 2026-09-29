package engine

import (
	"fmt"
	"testing"
	"time"
)

func TestDepthPerformance(t *testing.T) {
	pos, err := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatalf("Failed to parse FEN: %v", err)
	}

	depths := []int{3, 4, 5}

	for _, depth := range depths {
		t.Run(fmt.Sprintf("depth_%d", depth), func(t *testing.T) {
			start := time.Now()
			info := SearchFixed(pos, depth, nil)
			elapsed := time.Since(start)

			// Safety check - abort if taking too long
			if elapsed > 30*time.Second {
				t.Fatalf("Search at depth %d took too long: %v", depth, elapsed)
			}

			nps := float64(info.Nodes) / elapsed.Seconds()

			t.Logf("Depth %d performance:", depth)
			t.Logf("  Nodes: %d", info.Nodes)
			t.Logf("  Time: %v", elapsed)
			t.Logf("  NPS: %.0f", nps)
			t.Logf("  Best move: %s", info.BestMove.ToString())

			// Expected branching factor analysis would go here
		})
	}
}
