package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ehrlich-b/ngn/engine"
)

func main() {
	fenFlag := flag.String("fen", "", "FEN position to analyze")
	maxDepthFlag := flag.Int("depth", 10, "Maximum search depth")
	flag.Parse()

	if *fenFlag == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s -fen <FEN> [-depth <maxdepth>]\n", os.Args[0])
		os.Exit(1)
	}

	// Parse the FEN position
	pos, err := engine.ParseFEN(*fenFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing FEN: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Analyzing position: %s\n", *fenFlag)
	fmt.Printf("Turn: %d\n", pos.Turn())
	fmt.Println()

	// Advance TT age to reduce pollution from previous searches
	// (not perfect but helps reduce cross-depth contamination)
	for i := 0; i < 256; i++ {
		engine.AdvanceHashAge()
	}

	// Analyze position at each depth from 1 to maxDepth
	for depth := 1; depth <= *maxDepthFlag; depth++ {
		// Use SearchIterativeDeepening instead of SearchFixed to match real game behavior
		info := engine.SearchIterativeDeepening(pos, depth, nil)

		// Format score
		var scoreStr string
		if info.BestScore >= engine.MATE_IN_MAX {
			mate := (engine.MATE_VALUE - info.BestScore + 1) / 2
			scoreStr = fmt.Sprintf("mate %d", mate)
		} else if info.BestScore <= -engine.MATE_IN_MAX {
			mateDistance := -info.BestScore - engine.MATE_VALUE
			if mateDistance < 0 || mateDistance > 1000 {
				mateDistance = 1000
			}
			mate := -(mateDistance/2 + 1)
			scoreStr = fmt.Sprintf("mate %d", mate)
		} else {
			scoreStr = fmt.Sprintf("%+dcp", info.BestScore)
		}

		fmt.Printf("Depth %2d: %6s | Best: %s | Nodes: %d\n",
			depth, scoreStr, info.BestMove.ToString(), info.Nodes)
	}
}
