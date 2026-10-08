package main

import (
	"math"
	"testing"
)

// TestPentaBucketing checks the pair-points -> pentanomial-index mapping and the
// point estimate (no games played; pure arithmetic on the bucket counts).
func TestPentaBucketing(t *testing.T) {
	var penta [5]int
	add := func(p1, p2 float64) { penta[int((p1+p2)*2+0.5)]++ }
	add(1, 1)     // WW   -> idx 4
	add(1, 0)     // WL   -> idx 2
	add(0.5, 0.5) // DD   -> idx 2
	add(0, 0)     // LL   -> idx 0
	if want := [5]int{1, 0, 2, 0, 1}; penta != want {
		t.Fatalf("bucketing: got %v want %v", penta, want)
	}
	_, pairs, est, _, _ := pentaStats(penta, 0, 10)
	if pairs != 4 {
		t.Errorf("pairs=%d want 4", pairs)
	}
	if math.Abs(est) > 1e-9 { // mu=0.5 -> 0 Elo
		t.Errorf("penta Elo est=%.3f want ~0", est)
	}
}

func TestUCIOptionFlags(t *testing.T) {
	var options optionFlags
	for _, option := range []string{"EvalFile=/net dir/pilot.nnue", "UseNNUE=true", "Move Overhead=10", "Empty="} {
		if err := options.Set(option); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"setoption name EvalFile value /net dir/pilot.nnue", "setoption name UseNNUE value true", "setoption name Move Overhead value 10", "setoption name Empty value "}
	for i := range want {
		if options[i] != want[i] {
			t.Fatalf("option %d: got %q, want %q", i, options[i], want[i])
		}
	}
	for _, bad := range []string{"UseNNUE", "=true", "Hash=64\nquit", "Hash\r=64", "Hash value 1=2"} {
		if err := options.Set(bad); err == nil {
			t.Fatalf("accepted invalid option %q", bad)
		}
	}
	if len(options) != len(want) {
		t.Fatal("invalid option changed the list")
	}
}

// TestPentaReducesToTrinomialUnderIndependence is the correctness anchor: when the
// two games of a pair are independent, the pentanomial variance is exactly
// var_game/2 and pentaLLR must equal the trinomial sprtLLR to floating-point. The
// distribution {WW:k, idx2:2k, LL:k} is precisely the convolution of independent
// 50/50-decisive games, so the two statistics are algebraically identical here.
func TestPentaReducesToTrinomialUnderIndependence(t *testing.T) {
	const k = 50
	penta := [5]int{k, 0, 2 * k, 0, k} // 4k pairs = 8k games
	w, d, l := 4*k, 0, 4*k             // the matching trinomial record
	pLLR, _, _, _, _ := pentaStats(penta, 0, 10)
	tLLR := sprtLLR(w, d, l, 0, 10)
	if math.Abs(pLLR-tLLR) > 1e-9 {
		t.Errorf("independence: pentaLLR=%.6f != trinomialLLR=%.6f", pLLR, tLLR)
	}
}

// TestPentaBeatsTrinomialUnderCorrelation is the value demonstration: every pair
// splits 1W-1L (a perfectly balanced match — the SPRT null). The trinomial sees
// high per-game variance and barely moves; the pentanomial sees zero pair variance
// and drives toward H0 far faster. This is exactly the null-validation scenario
// ("every reversed-color pair nets 1W-1L") the trinomial could never resolve.
func TestPentaBeatsTrinomialUnderCorrelation(t *testing.T) {
	const P = 100
	penta := [5]int{0, 0, P, 0, 0} // all pairs at idx 2 (1W-1L)
	w, d, l := P, 0, P             // 2P games, w == l
	pLLR, _, _, _, _ := pentaStats(penta, 0, 10)
	tLLR := sprtLLR(w, d, l, 0, 10)
	if !(pLLR < tLLR) {
		t.Errorf("correlation: expected pentaLLR(%.3f) < trinomialLLR(%.3f)", pLLR, tLLR)
	}
	if pLLR >= 0 {
		t.Errorf("perfect-balance pentaLLR should favor H0 (negative), got %.3f", pLLR)
	}
}
