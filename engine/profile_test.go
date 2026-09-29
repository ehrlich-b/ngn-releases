package engine

import (
	"testing"
	"time"
)

func TestProfileHotPath(t *testing.T) {
	// Run a longer search to get meaningful profiling data
	pos, err := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatalf("Failed to parse FEN: %v", err)
	}

	t.Log("=== PROFILING HOT PATH PERFORMANCE ===")

	// Profile fixed depth search
	start := time.Now()
	info := SearchFixed(pos, 5, nil) // Depth 5 for meaningful work
	elapsed := time.Since(start)

	nps := float64(info.Nodes) / elapsed.Seconds()

	t.Logf("Depth 5 search profile:")
	t.Logf("  Nodes: %d", info.Nodes)
	t.Logf("  Time: %v", elapsed)
	t.Logf("  NPS: %.0f", nps)
	t.Logf("  Target: 100,000+ NPS")

	if nps < 100000 {
		t.Logf("❌ BELOW RED LINE: %.0f NPS (need 100K+)", nps)
		t.Logf("Performance gap: %.1fx slower than target", 100000.0/nps)
	} else {
		t.Logf("✅ ABOVE RED LINE: %.0f NPS", nps)
	}

	// Calculate node-per-millisecond for micro-optimization targets
	npms := float64(info.Nodes) / float64(elapsed.Nanoseconds()) * 1000000
	t.Logf("  Micro-optimization target: %.2f nodes per millisecond", npms)
}

func BenchmarkCriticalPath(b *testing.B) {
	pos, _ := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SearchFixed(pos, 4, nil)
	}
}

// BenchmarkSearchProfile drives a phase-diverse basket (middlegame / tactical /
// endgame) to a fixed depth for CPU and allocation profiling of the per-node hot
// path. Each op resets the TT to a fresh cold table so every iteration is a full,
// representative search with a near-constant node count (dense, comparable samples).
// The NewCache + ParseFEN allocations are test scaffolding — filter them out in the
// memprofile; what matters is whether any site inside search/movegen/eval allocates.
//
//	go test -run=^$ -bench=BenchmarkSearchProfile -benchmem -benchtime=10x \
//	  -cpuprofile=/tmp/cpu.prof -memprofile=/tmp/mem.prof ./engine/
func BenchmarkSearchProfile(b *testing.B) {
	fens := []string{
		"r1bq1rk1/pp2bppp/2n2n2/2pp4/3P4/2N1PN2/PPQ1BPPP/R1B2RK1 w - - 0 10",   // middlegame ~35 moves
		"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1", // Kiwipete tactical
		"8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1",                            // endgame
	}
	const depth = 11
	b.ResetTimer()
	var nodes int64
	for i := 0; i < b.N; i++ {
		defaultSearchEngine, _ = NewSearchEngineWithHash(16) // cold table per op → full representative search
		for _, f := range fens {
			p, err := ParseFEN(f)
			if err != nil {
				b.Fatalf("fen %q: %v", f, err)
			}
			info := SearchFixed(p, depth, nil)
			nodes += int64(info.Nodes)
		}
	}
	b.StopTimer()
	b.ReportMetric(float64(nodes)/float64(b.N), "nodes/op")
}

// TestHotPathAllocationFree ENFORCES the allocation-free per-node search hot path. This
// claim (docs/10 T1) was silently FALSE for ages — probcut declared a per-node [256]Move
// that escaped to the heap via the non-inlined GenerateMovesIntoBuffer (~34MB/run), found
// only by profiling, never by a test. This is that test: alphaBetaPV / quiescence / movegen
// / eval / SEE must allocate ZERO heap per node. Per-search scaffolding (TT, frame pool,
// ParseFEN) is constant across depths, so the DELTA in allocations between two depths
// isolates per-node allocation — alloc-free ⇒ ~0/node; a regression ⇒ thousands+. If this
// fails: a scratch buffer is escaping; route it through the searchFrame pool, do not
// `var x [N]Move` then pass `x[:]` to a non-inlined function.
func TestHotPathAllocationFree(t *testing.T) {
	const fen = "r1bq1rk1/pp2bppp/2n2n2/2pp4/3P4/2N1PN2/PPQ1BPPP/R1B2RK1 w - - 0 10"
	allocsAt := func(depth int) (allocs float64, nodes uint64) {
		a := testing.AllocsPerRun(2, func() {
			defaultSearchEngine, _ = NewSearchEngineWithHash(16) // constant scaffolding — cancels in the delta
			p, err := ParseFEN(fen)
			if err != nil {
				t.Fatalf("fen: %v", err)
			}
			nodes = SearchFixed(p, depth, nil).Nodes
		})
		return a, nodes
	}
	aLo, nLo := allocsAt(6)
	aHi, nHi := allocsAt(9)
	if nHi <= nLo {
		t.Fatalf("node counts not increasing (%d -> %d); cannot isolate per-node allocs", nLo, nHi)
	}
	perNode := (aHi - aLo) / float64(nHi-nLo)
	t.Logf("depth6: %.0f allocs / %d nodes | depth9: %.0f allocs / %d nodes | per-node = %.6f",
		aLo, nLo, aHi, nHi, perNode)
	// Alloc-free ⇒ per-node ≈ 0. The probcut regression was ~0.1-0.5 allocs/node; 0.01 sits
	// far below any real per-node allocation yet well above measurement noise.
	if perNode > 0.01 {
		t.Fatalf("HOT PATH ALLOCATES ~%.4f heap allocs/node — a per-node allocation regressed. "+
			"Find it with: go test -run=^$ -bench=BenchmarkSearchProfile -memprofile=/tmp/m ./engine/ && go tool pprof -list=alphaBetaPV -alloc_space ./engine.test /tmp/m. "+
			"Route the scratch buffer through the searchFrame pool. "+
			"(depth6 %.0f allocs/%d nodes, depth9 %.0f allocs/%d nodes)", perNode, aLo, nLo, aHi, nHi)
	}
}
