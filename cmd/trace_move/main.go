package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ehrlich-b/ngn/engine"
)

func main() {
	fenFlag := flag.String("fen", "", "FEN position")
	moveFlag := flag.String("move", "", "Move in algebraic notation (e.g., e2e4)")
	depthFlag := flag.Int("depth", 5, "Search depth after the move")
	flag.Parse()

	if *fenFlag == "" || *moveFlag == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s -fen <FEN> -move <move> [-depth <depth>]\n", os.Args[0])
		os.Exit(1)
	}

	// Parse the FEN position
	pos, err := engine.ParseFEN(*fenFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing FEN: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Starting position: %s\n", *fenFlag)
	fmt.Printf("Turn: %d\n\n", pos.Turn())

	// Generate all legal moves
	moves := engine.GenerateLegalMoves(pos)

	// Find the requested move
	var targetMove engine.Move
	found := false
	for _, move := range moves {
		if move.ToString() == *moveFlag {
			targetMove = move
			found = true
			break
		}
	}

	if !found {
		fmt.Fprintf(os.Stderr, "Move %s not found in legal moves\n", *moveFlag)
		fmt.Fprintf(os.Stderr, "Legal moves: ")
		for _, m := range moves {
			fmt.Fprintf(os.Stderr, "%s ", m.ToString())
		}
		fmt.Fprintf(os.Stderr, "\n")
		os.Exit(1)
	}

	// Make the move
	ep, tag, hc, _ := pos.MakeMove(targetMove)

	newFEN := engine.GenerateFEN(pos)
	fmt.Printf("After %s: %s\n", *moveFlag, newFEN)
	fmt.Printf("Turn: %d\n", pos.Turn())
	fmt.Printf("Static eval: %+d\n\n", engine.Evaluate(&pos.Board))

	// Search from this position
	fmt.Printf("Searching at depths 1-%d:\n", *depthFlag)
	for depth := 1; depth <= *depthFlag; depth++ {
		info := engine.SearchFixed(pos, depth, nil)

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

	// Unmake the move
	pos.UnMakeMove(targetMove, tag, ep, hc)
}
