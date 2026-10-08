package main

import (
	"testing"
	"time"

	"github.com/ehrlich-b/ngn/engine"
)

func TestProbePositionsAndRecord(t *testing.T) {
	for _, fen := range positions {
		p, err := engine.ParseFEN(fen)
		if err != nil || len(engine.GenerateLegalMoves(p)) == 0 {
			t.Fatalf("invalid probe position %q: %v", fen, err)
		}
	}
	move := engine.NewMove(engine.E2, engine.E4, engine.WhitePawn, engine.NoPiece, engine.NoType, 0)
	r := record(2, "fixed", &engine.SearchInfo{Nodes: 100, Depth: 5, BestMove: move}, 2*time.Second, []engine.Move{move})
	if r.NPS != 50 || r.Best != "e2e4" || len(r.PV) != 1 || r.PV[0] != "e2e4" {
		t.Fatalf("bad measurement: %+v", r)
	}
}
