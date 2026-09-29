// cmd/evaltune — coordinate-descent tuner over NGN's aux eval weights, minimizing
// ACPL (move-quality cp-loss vs Stockfish) over the committed SF-labeled corpus.
//
// WHY THIS (vs the closed texel-MSE lane): texel was closed for its OBJECTIVE —
// MSE-to-game-result doesn't track play strength (0-for-4, the re-tunes were
// MSE-mirages that ACPL/gauntlet rejected). ACPL is the VALIDATED move-quality
// ruler: it caught the passer regression MSE would have missed and agreed with the
// shield keep. So this re-tunes the SAME aux weights against a SOUND objective.
//
// MECHANISM: ACPL needs NGN's SEARCH, so each trial spawns the engine; weights are
// injected via the NGN_EVAL_W env var (engine/texel.go init), so NO rebuild per
// step. ACPL is DETERMINISTIC per weight set (fixed nodes), so coordinate descent
// needs no SPSA/averaging — a strict decrease is a real improvement.
//
// KNOBS: only the LIVE aux weights (the dead mobilityWeight scalar is excluded —
// mobility is evaluated via the C1 per-count tables, not this var). The PeSTO core
// is NOT touched (already tuned on virgin data — the one real eval-tune win).
//
// The winner is a CANDIDATE: validate it on zzbias (optimism must not worsen) and a
// cloud gauntlet before pasting back. Overfit risk: 150-position corpus is small.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type knob struct {
	name string
	val  int
}

// baseline is the current eval.go aux-weight block (the descent's starting point).
// Order is fixed so passes are reproducible. mobilityWeight/EG are omitted (DEAD —
// no consumption in eval.go; verified 2026-06-16).
func baseline() []knob {
	return []knob{
		{"passedPawnBonus", 10}, {"passedPawnBonusEG", 36},
		{"blockedPasserPenalty", 8}, {"blockedPasserPenaltyEG", 20},
		{"doubledPawnPenalty", 10}, {"doubledPawnPenaltyEG", 23},
		{"isolatedPawnPenalty", 10}, {"isolatedPawnPenaltyEG", 14},
		{"pawnChainWeight", 76}, {"pawnChainWeightEG", 77},
		{"rookOpenWeight", 87}, {"rookOpenWeightEG", 86},
		{"outpostWeight", 84}, {"outpostWeightEG", 85},
		{"kingSafetyWeight", 106}, {"kingSafetyWeightEG", 84},
		{"kingActivityWeight", 29}, {"kingActivityWeightEG", 29},
	}
}

func envString(ks []knob) string {
	parts := make([]string, len(ks))
	for i, k := range ks {
		parts[i] = fmt.Sprintf("%s=%d", k.name, k.val)
	}
	return strings.Join(parts, ",")
}

// applySeed overrides starting values from an NGN_EVAL_W string (for coarse->fine
// chaining or resuming a prior run). Unknown names abort.
func applySeed(ks []knob, seed string) {
	idx := make(map[string]int)
	for i, k := range ks {
		idx[k.name] = i
	}
	for _, pair := range strings.Split(seed, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		eq := strings.IndexByte(pair, '=')
		if eq < 0 {
			fmt.Fprintf(os.Stderr, "seed: bad pair %q\n", pair)
			os.Exit(1)
		}
		name := strings.TrimSpace(pair[:eq])
		v, err := strconv.Atoi(strings.TrimSpace(pair[eq+1:]))
		if err != nil {
			fmt.Fprintf(os.Stderr, "seed: bad value %q: %v\n", pair, err)
			os.Exit(1)
		}
		i, ok := idx[name]
		if !ok {
			fmt.Fprintf(os.Stderr, "seed: unknown knob %q\n", name)
			os.Exit(1)
		}
		ks[i].val = v
	}
}

var acplRe = regexp.MustCompile(`ACPL\s+([0-9.]+)`)

func main() {
	oracle := flag.String("oracle", "build/oracle", "oracle binary")
	bin := flag.String("bin", "build/ngn", "engine binary (weights injected via NGN_EVAL_W)")
	labels := flag.String("labels", "acpl_corpus.labels", "SF-labeled ACPL corpus")
	nodes := flag.Int("nodes", 200000, "fixed nodes per position (the committed ACPL standard)")
	conc := flag.Int("concurrency", 6, "parallel engine processes (E-core count)")
	step := flag.Int("step", 8, "line-search increment")
	maxPasses := flag.Int("passes", 8, "max coordinate-descent passes")
	capW := flag.Int("cap", 400, "max weight (min 0)")
	seed := flag.String("seed", "", "NGN_EVAL_W to start from (default = compiled baseline)")
	timeoutS := flag.Int("timeout", 300, "per-measure oracle timeout seconds (a hang aborts with a resumable -seed)")
	flag.Parse()

	ks := baseline()
	if *seed != "" {
		applySeed(ks, *seed)
	}

	nEval := 0
	measure := func() float64 {
		nEval++
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeoutS)*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, *oracle, "-mode", "acpl", "-labels", *labels, "-new", *bin,
			"-nodes", strconv.Itoa(*nodes), "-concurrency", strconv.Itoa(*conc))
		cmd.Env = append(os.Environ(), "NGN_EVAL_W="+envString(ks))
		out, err := cmd.CombinedOutput()
		if ctx.Err() == context.DeadlineExceeded {
			fmt.Fprintf(os.Stderr, "\nMEASURE TIMEOUT after %ds (oracle hung). Resume with:\n  -seed '%s'\n", *timeoutS, envString(ks))
			os.Exit(3)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "measure: oracle failed: %v\n%s\n", err, out)
			os.Exit(1)
		}
		m := acplRe.FindStringSubmatch(string(out))
		if m == nil {
			fmt.Fprintf(os.Stderr, "measure: no ACPL in oracle output:\n%s\n", out)
			os.Exit(1)
		}
		v, _ := strconv.ParseFloat(m[1], 64)
		return v
	}

	best := measure()
	fmt.Printf("baseline ACPL %.2f  (%d knobs, step %d, nodes %d, conc %d)\n", best, len(ks), *step, *nodes, *conc)

	// lineSearch steps knob i by delta while ACPL strictly improves, leaving it at
	// the best value; returns whether it moved at all.
	lineSearch := func(i, delta int) bool {
		moved := false
		for {
			nv := ks[i].val + delta
			if nv < 0 || nv > *capW {
				break
			}
			old := ks[i].val
			ks[i].val = nv
			m := measure()
			if m < best-1e-9 {
				fmt.Printf("  %-22s %d -> %d   ACPL %.2f -> %.2f\n", ks[i].name, old, nv, best, m)
				best = m
				moved = true
				continue
			}
			ks[i].val = old
			break
		}
		return moved
	}

	for pass := 0; pass < *maxPasses; pass++ {
		moved := 0
		for i := range ks {
			if lineSearch(i, *step) {
				moved++
				continue
			}
			if lineSearch(i, -*step) {
				moved++
			}
		}
		fmt.Printf("pass %d: best ACPL %.3f, %d knobs moved, %d evals total\n", pass+1, best, moved, nEval)
		fmt.Printf("  seed: %s\n", envString(ks))
		if moved == 0 {
			break
		}
	}

	fmt.Println("\n=== winning NGN_EVAL_W ===")
	fmt.Println(envString(ks))
	fmt.Println("\n=== paste-back (eval.go aux block) ===")
	for _, k := range ks {
		fmt.Printf("\t%s = %d\n", k.name, k.val)
	}
	fmt.Printf("\nfinal ACPL %.2f  (%d evals)\n", best, nEval)
}
