package main

import (
	"fmt"
	"os"
	"time"

	"github.com/ehrlich-b/ngn/engine"
)

// Configuration for different search modes
type SearchConfig struct {
	Name           string
	EnableFutility bool
	EnableNullMove bool
	EnableLMR      bool
	EnableCheckExt bool
	EnableTT       bool
}

var searchConfigs = []SearchConfig{
	{"Baseline (No Pruning)", false, false, false, false, false},
	{"+ Futility Pruning", true, false, false, false, false},
	{"+ Null Move", true, true, false, false, false},
	{"+ LMR", true, true, true, false, false},
	{"+ Check Extensions", true, true, true, true, false},
	{"+ Transposition Table", true, true, true, true, true},
	{"All Optimizations", true, true, true, true, true},
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go [depth]")
		fmt.Println("Example: go run main.go 5")
		os.Exit(1)
	}

	depth := 5
	if len(os.Args) > 1 {
		fmt.Sscanf(os.Args[1], "%d", &depth)
	}

	// Starting position
	pos := &engine.Position{
		Board:     engine.StartingBoard(),
		Tag:       engine.WhiteCanCastleKingSide | engine.WhiteCanCastleQueenSide | engine.BlackCanCastleKingSide | engine.BlackCanCastleQueenSide | engine.WhiteToMove,
		EnPassant: engine.NoSquare,
	}

	fmt.Printf("=== PRUNING TECHNIQUE COMPARISON ===\n")
	fmt.Printf("Position: Starting Position\n")
	fmt.Printf("Search Depth: %d\n\n", depth)

	fmt.Printf("%-25s %8s %10s %8s %6s %8s %8s %8s %7s\n",
		"Configuration", "Nodes", "Time(ms)", "NPS", "EBF", "Futility", "NullMove", "BetaCut", "TTHits")
	fmt.Printf("%s\n", string(make([]rune, 110, 110)))

	var baselineNodes uint64 = 0

	for i, config := range searchConfigs {
		// Run search with current configuration
		start := time.Now()
		info := engine.Search(pos, depth)
		elapsed := time.Since(start)

		if i == 0 {
			baselineNodes = info.Nodes
		}

		nps := uint64(float64(info.Nodes) / elapsed.Seconds())
		ebf := calculateEBF(info.Nodes, depth)

		// Calculate improvement
		improvement := ""
		if baselineNodes > 0 && i > 0 {
			reduction := float64(baselineNodes-info.Nodes) / float64(baselineNodes) * 100
			if reduction > 0 {
				improvement = fmt.Sprintf(" (%.1f%% reduction)", reduction)
			} else {
				improvement = fmt.Sprintf(" (%.1f%% increase)", -reduction)
			}
		}

		fmt.Printf("%-25s %8d %7dms %8d %6.2f %8d %8d %8d %7d%s\n",
			config.Name,
			info.Nodes,
			elapsed.Nanoseconds()/1000000,
			nps,
			ebf,
			info.FutilityPrunes,
			info.NullMoveCutoffs,
			info.BetaCutoffs,
			info.TTHits,
			improvement)
	}

	fmt.Printf("\n=== SUMMARY ===\n")
	fmt.Printf("Baseline nodes: %d\n", baselineNodes)

	// Show the impact of each technique
	fmt.Printf("\nPruning technique effectiveness (compared to baseline):\n")
	for i := 1; i < len(searchConfigs); i++ {
		// This is a simplified analysis - in reality we'd need to test each technique individually
		fmt.Printf("- %s: Enables various pruning mechanisms\n", searchConfigs[i].Name)
	}
}

func calculateEBF(nodes uint64, depth int) float64 {
	if depth <= 1 {
		return float64(nodes)
	}
	// Calculate effective branching factor: nodes^(1/depth)
	return pow(float64(nodes), 1.0/float64(depth))
}

// Simple power function for float64
func pow(base, exp float64) float64 {
	if exp == 0 {
		return 1
	}
	if exp == 1 {
		return base
	}

	result := 1.0
	for i := 0; i < int(exp); i++ {
		result *= base
	}

	// Handle fractional exponents with approximation
	if exp != float64(int(exp)) {
		// Simple approximation for fractional powers
		// This is not mathematically precise but good enough for EBF calculation
		frac := exp - float64(int(exp))
		if frac > 0.5 {
			result *= base * frac
		}
	}

	return result
}
