// pruning-analysis — the endgame-tree autopsy.
//
// WHY: h2h.sh proved NGN's ENDGAME search tree is 2.5-5x Blunder's at fixed
// depth, while its endgame EVAL is fine (M3). This tool localizes WHERE the
// extra endgame nodes physically go by searching a bucket of real ENDGAME
// positions and a bucket of real MIDDLEGAME positions to the same fixed depth
// and comparing the per-node tree composition (qsearch share, move-ordering
// quality, TT effectiveness, pruning rates, extension storms). The metric that
// differs most between the buckets is the eg-specific leak.
//
// Positions come from a real corpus (default output/lichess_eval.txt, one
// "<6-field-FEN> <winprob>" per line); the tool computes the PeSTO game phase
// itself and routes each position to the eg or mg bucket. Node counts are
// deterministic and core-independent, so P-core vs E-core does not matter.
//
// Usage: pruning-analysis [-fens file] [-depth 8] [-n 30] [-egmax 5] [-mgmin 14] [-stride 251]
package main

import (
	"bufio"
	"flag"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/ehrlich-b/ngn/engine"
)

// phaseOf computes the PeSTO game phase (0..24) from a FEN piece-placement
// field: N=1, B=1, R=2, Q=4 per piece (both colors), capped at 24. Matches the
// weights in engine/eval.go (totalPhase=24).
func phaseOf(fen string) int {
	placement := fen
	if i := strings.IndexByte(fen, ' '); i >= 0 {
		placement = fen[:i]
	}
	p := 0
	for _, c := range placement {
		switch c {
		case 'N', 'n', 'B', 'b':
			p++
		case 'R', 'r':
			p += 2
		case 'Q', 'q':
			p += 4
		}
	}
	if p > 24 {
		p = 24
	}
	return p
}

type bucket struct {
	name                                 string
	n                                    int
	nodes, qnodes, beta, firstcut        uint64
	ttprobes, tthits, ttcutoffs          uint64
	nmp, futility, lmp, see, lmr         uint64
	checkExt, recapExt, passExt, singExt uint64
	nullScout, lmrReSearch, pvReSearch   uint64
}

func (b *bucket) add(info *engine.SearchInfo) {
	b.n++
	b.nodes += info.Nodes
	b.qnodes += info.QNodes
	b.beta += info.BetaCutoffs
	b.firstcut += info.FirstMoveCutoffs
	b.ttprobes += info.TTProbes
	b.tthits += info.TTHits
	b.ttcutoffs += info.TTCutoffs
	b.nmp += info.NullMoveCutoffs
	b.futility += info.FutilityPrunes
	b.lmp += info.LMPPrunes
	b.see += info.SEEQuietPrunes
	b.lmr += info.LMRReductions
	b.nullScout += info.NullWindowScouts
	b.lmrReSearch += info.LMRReSearches
	b.pvReSearch += info.PVReSearches
	b.checkExt += info.CheckExtensions
	b.recapExt += info.RecaptureExtensions
	b.passExt += info.PassedPawnExtensions
	b.singExt += info.SingularExtensions
}

func main() {
	fens := flag.String("fens", "output/lichess_eval.txt", "FEN corpus (one '<FEN> ...' per line)")
	depth := flag.Int("depth", 8, "fixed search depth")
	n := flag.Int("n", 30, "positions per bucket")
	egmax := flag.Int("egmax", 5, "endgame bucket: phase <= this")
	mgmin := flag.Int("mgmin", 14, "middlegame bucket: phase >= this")
	stride := flag.Int("stride", 251, "sample every Nth line (spreads across the corpus)")
	flag.Parse()

	f, err := os.Open(*fens)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open %s: %v\n", *fens, err)
		os.Exit(1)
	}
	defer f.Close()

	eg := &bucket{name: fmt.Sprintf("ENDGAME(ph<=%d)", *egmax)}
	mg := &bucket{name: fmt.Sprintf("MIDGAME(ph>=%d)", *mgmin)}

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<20)
	line := 0
	start := time.Now()
	for sc.Scan() && (eg.n < *n || mg.n < *n) {
		line++
		if line%*stride != 0 {
			continue
		}
		fields := strings.Fields(sc.Text())
		if len(fields) < 6 {
			continue
		}
		fen := strings.Join(fields[:6], " ")
		ph := phaseOf(fen)
		var b *bucket
		switch {
		case ph <= *egmax && eg.n < *n:
			b = eg
		case ph >= *mgmin && mg.n < *n:
			b = mg
		default:
			continue
		}
		pos, err := engine.ParseFEN(fen)
		if err != nil {
			continue
		}
		// Cold start per position: fresh TT + cleared ordering tables so the
		// measurement is independent and comparable across positions.
		engine.ClearHash()
		engine.ClearHistoryTable()
		engine.ClearKillerMoves()
		engine.ClearCounterMoves()
		engine.ClearStop()
		engine.SetMaxNodes(0)

		info := engine.Search(pos, *depth)
		b.add(info)
		fmt.Fprintf(os.Stderr, "\r%s n=%d  %s n=%d  (%.0fs)   ",
			eg.name, eg.n, mg.name, mg.n, time.Since(start).Seconds())
	}
	fmt.Fprintln(os.Stderr)

	if eg.n == 0 || mg.n == 0 {
		fmt.Fprintf(os.Stderr, "not enough positions (eg=%d mg=%d); raise -stride coverage or lower -n\n", eg.n, mg.n)
		os.Exit(1)
	}

	report(eg, mg, *depth, *fens)
}

func report(eg, mg *bucket, depth int, src string) {
	fmt.Printf("=== ENDGAME-TREE AUTOPSY ===  depth=%d  source=%s\n", depth, src)
	fmt.Printf("%-34s %16s %16s %8s\n", "metric", eg.name, mg.name, "eg/mg")

	// pct(part,total) and per-knode helpers operate on bucket totals (sums),
	// which is equivalent to the per-position mean of each rate weighted by tree
	// size — the right denominator for "where do the nodes go".
	rowF := func(label string, a, b float64, flag string) {
		ratio := 0.0
		if b != 0 {
			ratio = a / b
		}
		fmt.Printf("%-34s %16.2f %16.2f %7.2fx %s\n", label, a, b, ratio, flag)
	}
	rowP := func(label string, an, ad, bn, bd uint64, flag string) {
		ap, bp := pct(an, ad), pct(bn, bd)
		ratio := 0.0
		if bp != 0 {
			ratio = ap / bp
		}
		fmt.Printf("%-34s %15.1f%% %15.1f%% %7.2fx %s\n", label, ap, bp, ratio, flag)
	}

	fmt.Printf("%-34s %16d %16d\n", "positions", eg.n, mg.n)
	egMean := float64(eg.nodes) / float64(eg.n)
	mgMean := float64(mg.nodes) / float64(mg.n)
	rowF("mean nodes/pos", egMean, mgMean, "<-- the tree-size gap")
	rowF("EBF (mean^(1/depth))", math.Pow(egMean, 1.0/float64(depth)), math.Pow(mgMean, 1.0/float64(depth)), "")

	fmt.Println("--- composition ---")
	rowP("qsearch node %", eg.qnodes, eg.nodes, mg.qnodes, mg.nodes, "hi eg => qsearch explosion")
	rowP("beta-cutoff %", eg.beta, eg.nodes, mg.beta, mg.nodes, "")
	rowP("first-move-cutoff % (ordering)", eg.firstcut, eg.beta, mg.firstcut, mg.beta, "LOW eg => bad eg ordering")

	fmt.Println("--- transposition table ---")
	rowP("TT hit % (hits/probes)", eg.tthits, eg.ttprobes, mg.tthits, mg.ttprobes, "")
	rowP("TT cutoff % (cutoffs/probes)", eg.ttcutoffs, eg.ttprobes, mg.ttcutoffs, mg.ttprobes, "LOW eg => TT not helping")

	fmt.Println("--- pruning (count per 1000 nodes) ---")
	rowF("NMP cutoffs /knode", perK(eg.nmp, eg.nodes), perK(mg.nmp, mg.nodes), "LOW eg => null-move weak")
	rowF("futility prunes /knode", perK(eg.futility, eg.nodes), perK(mg.futility, mg.nodes), "")
	rowF("LMP prunes /knode", perK(eg.lmp, eg.nodes), perK(mg.lmp, mg.nodes), "")
	rowF("SEE prunes /knode", perK(eg.see, eg.nodes), perK(mg.see, mg.nodes), "")
	rowF("LMR reductions /knode", perK(eg.lmr, eg.nodes), perK(mg.lmr, mg.nodes), "")

	fmt.Println("--- LMR re-search economics (where verification nodes go) ---")
	rowF("null-window scouts /knode", perK(eg.nullScout, eg.nodes), perK(mg.nullScout, mg.nodes), "")
	rowF("LMR re-searches /knode", perK(eg.lmrReSearch, eg.nodes), perK(mg.lmrReSearch, mg.nodes), "HI eg => reduced-move verification churn")
	rowF("PV re-searches /knode", perK(eg.pvReSearch, eg.nodes), perK(mg.pvReSearch, mg.nodes), "")
	rowP("LMR re-search rate (resrch/reductions)", eg.lmrReSearch, eg.lmr, mg.lmrReSearch, mg.lmr, "HI => reductions undone, paid 2x")

	fmt.Println("--- extensions (count per 1000 nodes) ---")
	rowF("check ext /knode", perK(eg.checkExt, eg.nodes), perK(mg.checkExt, mg.nodes), "HI eg => check-ext storm")
	rowF("recapture ext /knode", perK(eg.recapExt, eg.nodes), perK(mg.recapExt, mg.nodes), "")
	rowF("passed-pawn ext /knode", perK(eg.passExt, eg.nodes), perK(mg.passExt, mg.nodes), "HI eg => passer-ext storm")
	rowF("singular ext /knode", perK(eg.singExt, eg.nodes), perK(mg.singExt, mg.nodes), "")
}

func pct(part, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}

func perK(count, nodes uint64) float64 {
	if nodes == 0 {
		return 0
	}
	return float64(count) / float64(nodes) * 1000
}
