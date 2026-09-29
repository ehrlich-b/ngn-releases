package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ehrlich-b/ngn/internal/uci"
)

// sfpv prints Stockfish's depth-20 score + PV for each FEN arg (side-to-move POV).
func main() {
	sf := uci.Start("/opt/homebrew/bin/stockfish", "stockfish", true)
	if sf == nil {
		fmt.Println("no sf")
		return
	}
	defer uci.Stop(sf)
	uci.Send(sf, "setoption name MultiPV value 1")
	uci.Send(sf, "isready")
	uci.WaitFor(sf, "readyok", 5*time.Second)
	for _, fen := range os.Args[1:] {
		uci.Send(sf, "position fen "+fen)
		lines := uci.Analyze(sf, "go depth 20", 30*time.Second)
		best := ""
		for _, ln := range lines {
			if strings.Contains(ln, " score ") {
				best = ln
			}
		}
		if i := strings.Index(best, " score "); i >= 0 {
			best = best[i:]
		}
		fmt.Printf("FEN %s\n  -> %s\n", fen, best)
	}
}
