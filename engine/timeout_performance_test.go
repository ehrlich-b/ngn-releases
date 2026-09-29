package engine

import (
	"context"
	"testing"
	"time"
)

func TestTimeoutPerformance(t *testing.T) {
	pos, err := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatalf("Failed to parse FEN: %v", err)
	}

	// Test with time budgets
	timeouts := []time.Duration{
		100 * time.Millisecond,
		500 * time.Millisecond,
		1 * time.Second,
		2 * time.Second,
	}

	for _, timeout := range timeouts {
		t.Run(timeout.String(), func(t *testing.T) {
			// Create context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			// Use a channel to capture results
			type result struct {
				info *SearchInfo
				err  error
			}
			resultChan := make(chan result, 1)

			// Run search in goroutine
			go func() {
				// Set up time manager
				tm := NewTimeManager()
				params := SearchParams{
					MoveTime: int(timeout.Milliseconds()),
				}
				tm.SetTimeControl(params, true)

				// Try iterative deepening with high max depth
				info := SearchIterativeDeepening(pos, 20, tm)

				resultChan <- result{info: info, err: nil}
			}()

			// Wait for either completion or timeout
			select {
			case res := <-resultChan:
				if res.err != nil {
					t.Fatalf("Search error: %v", res.err)
				}

				nps := float64(res.info.Nodes) / timeout.Seconds()

				t.Logf("Performance in %v:", timeout)
				t.Logf("  Depth reached: %d", res.info.Depth)
				t.Logf("  Nodes: %d", res.info.Nodes)
				t.Logf("  NPS: %.0f", nps)
				t.Logf("  Best move: %s", res.info.BestMove.ToString())

				// Performance analysis
				if nps < 50000 {
					t.Logf("  ⚠️  Sub-optimal NPS")
				} else if nps > 200000 {
					t.Logf("  ✅ Good NPS")
				} else {
					t.Logf("  📊 Reasonable NPS")
				}

				// Depth analysis for time budget
				expectedDepth := map[time.Duration]int{
					100 * time.Millisecond: 4,
					500 * time.Millisecond: 5,
					1 * time.Second:        6,
					2 * time.Second:        7,
				}

				if expected, exists := expectedDepth[timeout]; exists {
					if res.info.Depth < expected-1 {
						t.Logf("  ⚠️  Lower depth than expected (got %d, expected ~%d)", res.info.Depth, expected)
					} else if res.info.Depth > expected+1 {
						t.Logf("  🚀 Higher depth than expected (got %d, expected ~%d)", res.info.Depth, expected)
					} else {
						t.Logf("  ✅ Expected depth range (got %d, expected ~%d)", res.info.Depth, expected)
					}
				}

			case <-ctx.Done():
				t.Logf("Search timed out after %v", timeout)
				t.Log("  This indicates the search is not respecting time limits properly")
			}
		})
	}
}

func TestPerformanceComparison(t *testing.T) {
	t.Log("=== NGN Chess Engine Performance Assessment ===")

	pos, err := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatalf("Failed to parse FEN: %v", err)
	}

	// Quick fixed-depth test
	start := time.Now()
	info := SearchFixed(pos, 4, nil)
	elapsed := time.Since(start)

	nps := float64(info.Nodes) / elapsed.Seconds()

	t.Logf("Fixed depth 4 search:")
	t.Logf("  Nodes: %d", info.Nodes)
	t.Logf("  Time: %v", elapsed)
	t.Logf("  NPS: %.0f", nps)

	t.Log("\n=== Performance Comparison to Other Engines ===")
	t.Log("Reference NPS (nodes per second) for established engines:")
	t.Log("  • Stockfish 15: ~5-25M nps")
	t.Log("  • Komodo: ~2-10M nps")
	t.Log("  • Chess.com engines: ~1-5M nps")
	t.Log("  • Basic hobby engines: ~10K-500K nps")
	t.Log("  • Very basic engines: ~1K-50K nps")

	t.Logf("\nNGN current performance: %.0f nps", nps)

	if nps < 10000 {
		t.Log("❌ RECOMMENDATION: Major optimization required")
		t.Log("   Focus on: bitboard operations, move generation efficiency")
	} else if nps < 100000 {
		t.Log("⚠️  RECOMMENDATION: Performance optimization needed for competitive play")
		t.Log("   Focus on: search optimizations, evaluation caching, bitboard ops")
	} else if nps < 500000 {
		t.Log("📊 RECOMMENDATION: Performance is adequate, focus on engine features")
		t.Log("   Continue with: evaluation improvements, search enhancements")
	} else {
		t.Log("✅ RECOMMENDATION: Performance is good, focus on chess knowledge")
		t.Log("   Continue with: advanced evaluation, opening books, endgame tables")
	}
}
