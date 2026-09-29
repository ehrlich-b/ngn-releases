package engine

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestZZTermAgg (throwaway): over a corpus TSV (lossidx ply ngncolor phase eval fen),
// restricted to a phase band, aggregate each aux term's mean contribution to NGN's
// OWN-SIDE eval (term delta flipped to NGN POV). Answers: in the phase-6-11 hole,
// which aux term most inflates NGN's own-side eval, across MANY positions (not the 4
// cherry-picked FENs)? Env: AGG_TSV, AGG_PMIN, AGG_PMAX (inclusive phase band).
func TestZZTermAgg(t *testing.T) {
	tsv := os.Getenv("AGG_TSV")
	if tsv == "" {
		t.Skip("set AGG_TSV")
	}
	pmin, pmax := 6, 11
	if v := os.Getenv("AGG_PMIN"); v != "" {
		pmin, _ = strconv.Atoi(v)
	}
	if v := os.Getenv("AGG_PMAX"); v != "" {
		pmax, _ = strconv.Atoi(v)
	}

	terms := []struct {
		name   string
		mg, eg *int
	}{
		{"passed", &passedPawnBonus, &passedPawnBonusEG},
		{"doubled", &doubledPawnPenalty, &doubledPawnPenaltyEG},
		{"isolated", &isolatedPawnPenalty, &isolatedPawnPenaltyEG},
		{"chain", &pawnChainWeight, &pawnChainWeightEG},
		{"mobility", &mobilityWeight, &mobilityWeightEG},
		{"rookOpen", &rookOpenWeight, &rookOpenWeightEG},
		{"outpost", &outpostWeight, &outpostWeightEG},
		{"kingSafety", &kingSafetyWeight, &kingSafetyWeightEG},
		{"kingActivity", &kingActivityWeight, &kingActivityWeightEG},
	}

	f, err := os.Open(tsv)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<22)
	sc.Scan() // header

	sum := make([]float64, len(terms)) // sum of own-POV contribution
	n := 0
	for sc.Scan() {
		fld := strings.Split(strings.TrimSpace(sc.Text()), "\t")
		if len(fld) < 6 {
			continue
		}
		color := fld[2]
		phase, _ := strconv.Atoi(fld[3])
		if phase < pmin || phase > pmax {
			continue
		}
		pos, err := ParseFEN(fld[5])
		if err != nil {
			continue
		}
		b := &pos.Board
		full := evaluateUnsafe(b)
		for i, tm := range terms {
			smg, seg := *tm.mg, *tm.eg
			*tm.mg, *tm.eg = 0, 0
			d := full - evaluateUnsafe(b) // white-POV contribution of this term
			*tm.mg, *tm.eg = smg, seg
			own := float64(d)
			if color == "b" {
				own = -own
			}
			sum[i] += own
		}
		n++
	}
	if n == 0 {
		t.Fatalf("no positions in phase band [%d,%d]", pmin, pmax)
	}
	type kv struct {
		name string
		mean float64
	}
	var rows []kv
	for i, tm := range terms {
		rows = append(rows, kv{tm.name, sum[i] / float64(n)})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].mean > rows[j].mean })
	fmt.Printf("\n=== AUX TERM own-side contribution, phase[%d,%d], n=%d (%s) ===\n", pmin, pmax, n, tsv)
	fmt.Println("  (+ = term pushes NGN's OWN eval UP; the over-optimism culprits sort to top)")
	for _, r := range rows {
		fmt.Printf("    %-13s %+7.1f\n", r.name, r.mean)
	}
}
