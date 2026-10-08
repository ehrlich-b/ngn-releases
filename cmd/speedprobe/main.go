// speedprobe measures NGN's own search with cold, single-threaded state.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime/pprof"
	"time"

	"github.com/ehrlich-b/ngn/engine"
)

var positions = []string{
	"r1bq1rk1/pp2bppp/2n2n2/2pp4/3P4/2N1PN2/PPQ1BPPP/R1B2RK1 w - - 0 10",
	"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
	"r4rk1/1pp1qppp/p1np1n2/2b1p1B1/2B1P1b1/P1NP1N2/1PP1QPPP/R4RK1 w - - 0 10",
	"8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1",
	"8/5k2/2p2p2/3p4/3P1P2/2P2K2/8/8 w - - 0 1",
	"6k1/5pp1/7p/3q4/8/3Q2P1/5P1P/6K1 w - - 0 1",
}

type result struct {
	Position int      `json:"position"`
	Kind     string   `json:"kind"`
	Depth    int      `json:"depth"`
	Nodes    uint64   `json:"nodes"`
	Score    int      `json:"score"`
	Best     string   `json:"best"`
	PV       []string `json:"pv"`
	Seconds  float64  `json:"seconds"`
	NPS      float64  `json:"nps"`
}

func record(index int, kind string, info *engine.SearchInfo, elapsed time.Duration, pv []engine.Move) result {
	r := result{Position: index, Kind: kind, Depth: info.Depth, Nodes: info.Nodes,
		Score: info.BestScore, Best: info.BestMove.ToString(), Seconds: elapsed.Seconds(),
		NPS: float64(info.Nodes) / elapsed.Seconds(), PV: []string{}}
	for _, move := range pv {
		r.PV = append(r.PV, move.ToString())
	}
	return r
}

func main() {
	mode := flag.String("mode", "identity", "identity or timed")
	depth := flag.Int("depth", 9, "identity search depth")
	seconds := flag.Duration("duration", 10*time.Second, "timed search duration per position")
	position := flag.Int("position", -1, "position index, or -1 for all six")
	net := flag.String("net", "", "NGN-produced NGNN1/2/3 file; empty selects HCE")
	profile := flag.String("profile", "", "CPU profile output file")
	flag.Parse()
	if (*mode != "identity" && *mode != "timed") || *depth < 1 || *depth > 32 || *seconds <= 0 || *position < -1 || *position >= len(positions) {
		panic("invalid probe options")
	}
	if *profile != "" {
		f, err := os.Create(*profile)
		must(err)
		must(pprof.StartCPUProfile(f))
		defer f.Close()
		defer pprof.StopCPUProfile()
	}
	enc := json.NewEncoder(os.Stdout)
	for index, fen := range positions {
		if *position >= 0 && index != *position {
			continue
		}
		for _, kind := range []string{"iterative", "fixed"} {
			if *mode == "timed" && kind == "fixed" {
				continue
			}
			e, err := engine.NewSearchEngineWithHash(16)
			must(err)
			must(selectNetwork(e, *net))
			p, err := engine.ParseFEN(fen)
			must(err)
			var info *engine.SearchInfo
			start := time.Now()
			if *mode == "timed" {
				// Joining the callback prevents a late stop from outliving this trial.
				done := make(chan struct{})
				timer := time.AfterFunc(*seconds, func() { e.RequestStop(); close(done) })
				info = e.SearchIterativeDeepening(p, 64, nil)
				if !timer.Stop() {
					<-done
				}
			} else if kind == "fixed" {
				info = e.SearchFixed(p, *depth, nil)
			} else {
				info = e.SearchIterativeDeepening(p, *depth, nil)
			}
			elapsed := time.Since(start)
			pv := e.ExtractPVFromTT(p, info.BestMove, *depth)
			must(enc.Encode(record(index, kind, info, elapsed, pv)))
		}
	}
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
