package engine

import (
	"math"
	"testing"
)

// snapshotWeights copies the current tunable eval-term weights and returns a
// restore func — tuning tests mutate these package globals, so they must put them
// back to avoid corrupting eval-dependent tests that run later.
func snapshotWeights() func() {
	w := TexelWeights()
	saved := make([]int, len(w))
	for i := range w {
		saved[i] = *w[i].Ptr
	}
	return func() {
		w := TexelWeights()
		for i := range w {
			*w[i].Ptr = saved[i]
		}
	}
}

// materialDataset: White clearly up a rook (1.0), balanced startpos (0.5), and
// White clearly down a rook (0.0) — the static eval tracks material, so the
// logistic should fit these well.
func materialDataset(t *testing.T) []TexelSample {
	cases := []struct {
		fen    string
		result float64
	}{
		{"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1", 0.5},
		{"1nbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w Kkq - 0 1", 1.0},  // black down a8 rook
		{"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/1NBQKBNR w KQkq - 0 1", 0.0}, // white down a1 rook
	}
	var samples []TexelSample
	for _, c := range cases {
		pos, err := ParseFEN(c.fen)
		if err != nil {
			t.Fatalf("bad FEN %q: %v", c.fen, err)
		}
		samples = append(samples, TexelSample{Board: pos.Board, Result: c.result})
	}
	return samples
}

func TestTexelSigmoid(t *testing.T) {
	const k = 1.0
	if got := texelSigmoid(0, k); math.Abs(got-0.5) > 1e-9 {
		t.Errorf("sigmoid(0)=%v, want 0.5", got)
	}
	// Monotonic increasing and bounded (0,1).
	prev := -1.0
	for e := -2000.0; e <= 2000.0; e += 100 {
		s := texelSigmoid(e, k)
		if s <= 0 || s >= 1 {
			t.Errorf("sigmoid(%v)=%v out of (0,1)", e, s)
		}
		if s <= prev {
			t.Errorf("sigmoid not increasing at e=%v: %v <= %v", e, s, prev)
		}
		prev = s
	}
	// Symmetric about 0: sigmoid(-e) == 1 - sigmoid(e).
	if a, b := texelSigmoid(300, k), texelSigmoid(-300, k); math.Abs(a-(1-b)) > 1e-9 {
		t.Errorf("sigmoid not symmetric: s(300)=%v, 1-s(-300)=%v", a, 1-b)
	}
}

func TestTexelMSEAndFindK(t *testing.T) {
	samples := materialDataset(t)

	// MSE is a mean of squared errors in [0,1], so it is bounded [0,1].
	for _, k := range []float64{0.001, 0.5, 1.0, 2.0} {
		if m := texelMSE(samples, k); m < 0 || m > 1 {
			t.Errorf("MSE(k=%v)=%v out of [0,1]", k, m)
		}
	}

	// The fitted K should be no worse than a near-flat sigmoid or an oversharp one.
	k := TexelFindK(samples)
	if k <= 0 {
		t.Fatalf("TexelFindK returned non-positive K=%v", k)
	}
	best := texelMSE(samples, k)
	if flat := texelMSE(samples, 0.001); best > flat+1e-9 {
		t.Errorf("fitted K worse than flat: MSE(%v)=%v > MSE(0.001)=%v", k, best, flat)
	}
	if sharp := texelMSE(samples, 2.0); best > sharp+1e-9 {
		t.Errorf("fitted K worse than sharp: MSE(%v)=%v > MSE(2.0)=%v", k, best, sharp)
	}
}

func TestTexelWeightsWiring(t *testing.T) {
	w := TexelWeights()
	if len(w) != 20 {
		t.Fatalf("TexelWeights returned %d, want 20", len(w)) // 10 MG term weights + 10 EG twins (tapered scalars)
	}
	// A weight pointer must alias the real eval var: mutating it changes the live
	// package var the eval reads, proving the tuner edits the real eval.
	defer snapshotWeights()()
	orig := passedPawnBonus
	for _, x := range w {
		if x.Name == "passedPawnBonus" {
			*x.Ptr = orig + 7
		}
	}
	if passedPawnBonus != orig+7 {
		t.Errorf("weight pointer does not alias passedPawnBonus: got %d", passedPawnBonus)
	}
}

func TestTexelTuneNeverIncreasesMSE(t *testing.T) {
	defer snapshotWeights()()
	samples := materialDataset(t)
	k := TexelFindK(samples)
	before := texelMSE(samples, k)

	w := TexelWeights()
	ptrs := make([]*int, len(w))
	for i := range w {
		ptrs[i] = w[i].Ptr
	}
	after := TexelTune(samples, ptrs, k, 4, 3, nil, nil)

	if after > before+1e-12 {
		t.Errorf("coordinate descent increased MSE: before=%v after=%v", before, after)
	}
	// The returned value must equal a fresh MSE of the left-in-place tuned weights.
	if fresh := texelMSE(samples, k); math.Abs(fresh-after) > 1e-9 {
		t.Errorf("returned MSE %v != recomputed MSE %v of tuned weights", after, fresh)
	}
}
