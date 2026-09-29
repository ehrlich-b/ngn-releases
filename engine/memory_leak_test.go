// Memory leak detection for long searches
// This file provides tests to detect memory leaks during extended search operations

package engine

import (
	"runtime"
	"testing"
	"time"
)

// MemoryStats captures memory usage statistics
type MemoryStats struct {
	Timestamp  time.Time
	AllocMB    float64
	TotalMB    float64
	SysMB      float64
	NumGC      uint32
	Goroutines int
}

// captureMemoryStats captures current memory usage
func captureMemoryStats() MemoryStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return MemoryStats{
		Timestamp:  time.Now(),
		AllocMB:    float64(m.Alloc) / 1024 / 1024,
		TotalMB:    float64(m.TotalAlloc) / 1024 / 1024,
		SysMB:      float64(m.Sys) / 1024 / 1024,
		NumGC:      m.NumGC,
		Goroutines: runtime.NumGoroutine(),
	}
}

// TestLongSearchMemoryLeak runs extended searches to detect memory leaks
func TestLongSearchMemoryLeak(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}

	// Test positions that trigger different search patterns
	testPositions := []struct {
		name  string
		fen   string
		depth int
	}{
		{
			name:  "Complex Middlegame",
			fen:   "r1bqk2r/ppp2ppp/2n2n2/2b1p3/2B1P3/3P1N2/PPP2PPP/RNBQK2R w KQkq - 4 6",
			depth: 6,
		},
		{
			name:  "Tactical Position",
			fen:   "r1bqkb1r/pppp1ppp/2n2n2/4p3/2B1P3/3P1N2/PPP2PPP/RNBQK2R b KQkq - 0 4",
			depth: 6,
		},
		{
			name:  "Endgame Position",
			fen:   "8/8/8/8/8/8/4K1P1/5k2 w - - 0 1",
			depth: 10,
		},
	}

	const iterations = 5            // Reduced from 10 for faster testing
	const maxMemoryIncreaseMB = 5.0 // Allow some memory increase but not excessive

	for _, pos := range testPositions {
		t.Run(pos.name, func(t *testing.T) {
			position, err := ParseFEN(pos.fen)
			if err != nil {
				t.Fatalf("Failed to parse FEN %s: %v", pos.fen, err)
			}

			// Force garbage collection before starting
			runtime.GC()
			runtime.GC()                      // Run twice to ensure clean state
			time.Sleep(10 * time.Millisecond) // Let GC complete

			initialStats := captureMemoryStats()
			t.Logf("Initial memory: %.2f MB allocated, %d goroutines",
				initialStats.AllocMB, initialStats.Goroutines)

			// Run multiple search iterations
			var totalNodes uint64
			searchStart := time.Now()

			for i := 0; i < iterations; i++ {
				// Run search and get nodes count
				info := Search(position, pos.depth)
				totalNodes += info.Nodes

				// Force garbage collection periodically
				if i%2 == 1 {
					runtime.GC()
				}
			}

			searchDuration := time.Since(searchStart)

			// Final garbage collection and memory check
			runtime.GC()
			runtime.GC()
			time.Sleep(10 * time.Millisecond)

			finalStats := captureMemoryStats()

			memoryIncrease := finalStats.AllocMB - initialStats.AllocMB
			goroutineIncrease := finalStats.Goroutines - initialStats.Goroutines

			t.Logf("Search completed: %d iterations, %d total nodes, %.2f seconds",
				iterations, totalNodes, searchDuration.Seconds())
			t.Logf("Final memory: %.2f MB allocated, %d goroutines",
				finalStats.AllocMB, finalStats.Goroutines)
			t.Logf("Memory change: %+.2f MB, Goroutine change: %+d, GC runs: %d",
				memoryIncrease, goroutineIncrease, finalStats.NumGC-initialStats.NumGC)

			// Check for memory leaks
			if memoryIncrease > maxMemoryIncreaseMB {
				t.Errorf("Potential memory leak: memory increased by %.2f MB (limit: %.2f MB)",
					memoryIncrease, maxMemoryIncreaseMB)
			}

			// Check for goroutine leaks
			if goroutineIncrease > 0 {
				t.Errorf("Goroutine leak: %d new goroutines after search", goroutineIncrease)
			}

			// Performance sanity check
			avgNPS := float64(totalNodes) / searchDuration.Seconds()
			if avgNPS < 50000 { // Very conservative threshold
				t.Errorf("Search performance degraded: %.0f NPS (expected >50,000)", avgNPS)
			} else {
				t.Logf("✓ Search performance: %.0f NPS", avgNPS)
			}
		})
	}
}

// TestTranspositionTableMemoryManagement tests TT memory behavior
func TestTranspositionTableMemoryManagement(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping TT memory test in short mode")
	}

	// Force garbage collection
	runtime.GC()
	runtime.GC()

	initialStats := captureMemoryStats()

	// Create and populate a transposition table
	position, err := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatalf("Failed to parse starting position: %v", err)
	}

	// Run multiple searches to populate TT
	const numSearches = 10
	var totalNodes uint64

	for i := 0; i < numSearches; i++ {
		info := Search(position, 5)
		totalNodes += info.Nodes
	}

	// Memory after TT population
	runtime.GC()
	populatedStats := captureMemoryStats()

	memoryForTT := populatedStats.AllocMB - initialStats.AllocMB

	t.Logf("TT populated with %d searches, %d total nodes", numSearches, totalNodes)
	t.Logf("Memory for TT operations: %.2f MB", memoryForTT)

	// Check that TT memory usage is reasonable
	const maxTTMemoryMB = 50.0 // Conservative limit
	if memoryForTT > maxTTMemoryMB {
		t.Errorf("Transposition table using excessive memory: %.2f MB (limit: %.2f MB)",
			memoryForTT, maxTTMemoryMB)
	}
}

// TestMemoryStressSearch runs a stress test with deep searches
func TestMemoryStressSearch(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory stress test in short mode")
	}

	// Use a complex position that creates large search trees
	position, err := ParseFEN("r1bqk2r/ppp2ppp/2n2n2/2b1p3/2B1P3/3P1N2/PPP2PPP/RNBQK2R w KQkq - 4 6")
	if err != nil {
		t.Fatalf("Failed to parse test position: %v", err)
	}

	runtime.GC()
	runtime.GC()

	initialStats := captureMemoryStats()

	// Run progressively deeper searches
	for depth := 4; depth <= 7; depth++ {
		searchStart := time.Now()
		info := Search(position, depth)
		searchTime := time.Since(searchStart)

		runtime.GC() // Clean up after each search

		currentStats := captureMemoryStats()
		memoryUsed := currentStats.AllocMB - initialStats.AllocMB

		nps := float64(info.Nodes) / searchTime.Seconds()

		t.Logf("Depth %d: %d nodes, %.3fs, %.0f NPS, %.2f MB memory",
			depth, info.Nodes, searchTime.Seconds(), nps, memoryUsed)

		// Check for excessive memory growth
		expectedMaxMemory := float64(depth) * 2.0 // Allow 2MB per depth level
		if memoryUsed > expectedMaxMemory {
			t.Errorf("Excessive memory usage at depth %d: %.2f MB (expected ≤%.2f MB)",
				depth, memoryUsed, expectedMaxMemory)
		}
	}

	// Final cleanup and check
	runtime.GC()
	runtime.GC()
	time.Sleep(20 * time.Millisecond)

	finalStats := captureMemoryStats()
	totalMemoryUsed := finalStats.AllocMB - initialStats.AllocMB

	t.Logf("Total memory after stress test: %.2f MB", totalMemoryUsed)

	// Check that memory returns to reasonable levels after GC
	const maxFinalMemoryMB = 10.0
	if totalMemoryUsed > maxFinalMemoryMB {
		t.Errorf("Memory not properly released: %.2f MB remaining (limit: %.2f MB)",
			totalMemoryUsed, maxFinalMemoryMB)
	}
}
