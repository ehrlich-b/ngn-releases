package countereval

import (
	"math"
	"math/rand"
	"testing"
)

func TestFeatureUpdateRowsMatchPortableEveryLane(t *testing.T) {
	seeds := []int64{1, 17, 0x5eed}
	for _, seed := range seeds {
		random := rand.New(rand.NewSource(seed))
		for sample := 0; sample < 128; sample++ {
			var initial, weights [HiddenSize]float32
			for lane := 0; lane < HiddenSize; lane++ {
				initial[lane] = float32(random.Intn(1<<20)-(1<<19)) / 1024
				weights[lane] = float32(random.Intn(1<<18)-(1<<17)) / 4096
			}
			for _, add := range []bool{true, false} {
				got, want := initial, initial
				weightsBefore := weights
				if add {
					addFeatureRow(&got, &weights)
					addFeatureRowPortable(&want, &weights)
				} else {
					subFeatureRow(&got, &weights)
					subFeatureRowPortable(&want, &weights)
				}
				requireFeatureRowBits(t, got, want, seed, sample, add)
				requireFeatureRowBits(t, weights, weightsBefore, seed, sample, add)
			}
		}
	}
}

func TestApplyFeatureUpdatesMatchesPortableAfterEveryOrderedRow(t *testing.T) {
	model := deterministicContextModel()
	updates := [4]featureUpdate{
		{feature: 6*64 + 35},
		{feature: 35, add: true},
		{feature: 5*64 + 4},
		{feature: 5*64 + 6, add: true},
	}
	var got, want [HiddenSize]float32
	for lane := 0; lane < HiddenSize; lane++ {
		got[lane] = float32((lane%13)-6) * 0.125
		want[lane] = got[lane]
	}
	for count := 1; count <= len(updates); count++ {
		gotStep, wantStep := got, want
		applyFeatureUpdates(&gotStep, model, &updates, count)
		applyTestUpdates(model, &wantStep, updates[:count])
		requireFeatureRowBits(t, gotStep, wantStep, 0, count, false)
	}
}

func TestFeatureUpdateRowsSpecialFiniteInputsAndNoAllocation(t *testing.T) {
	bitPatterns := [...]uint32{
		0x00000000, 0x80000000, 0x00000001, 0x80000001,
		0x00800000, 0x80800000, 0x3f800000, 0xbf800000,
		0x4b000001, 0xcb000001, 0x7e000000, 0xfe000000,
	}
	var initial, weights [HiddenSize]float32
	for lane := 0; lane < HiddenSize; lane++ {
		initial[lane] = math.Float32frombits(bitPatterns[lane%len(bitPatterns)])
		weights[lane] = math.Float32frombits(bitPatterns[(lane*5+3)%len(bitPatterns)])
	}
	for _, add := range []bool{true, false} {
		got, want := initial, initial
		weightsBefore := weights
		if add {
			addFeatureRow(&got, &weights)
			addFeatureRowPortable(&want, &weights)
		} else {
			subFeatureRow(&got, &weights)
			subFeatureRowPortable(&want, &weights)
		}
		requireFeatureRowBits(t, got, want, 0, 0, add)
		requireFeatureRowBits(t, weights, weightsBefore, 0, 0, add)
	}

	var accumulator [HiddenSize]float32
	if allocations := testing.AllocsPerRun(1000, func() {
		addFeatureRow(&accumulator, &weights)
		subFeatureRow(&accumulator, &weights)
	}); allocations != 0 {
		t.Fatalf("selected row kernels allocate %g times per pair", allocations)
	}
}

func TestFeatureUpdateRowsDeclaredRoundingBits(t *testing.T) {
	tests := []struct {
		name                        string
		add                         bool
		accumulatorBits, weightBits uint32
		wantBits                    uint32
	}{
		{"add_halfway_ties_even", true, 0x3f800000, 0x33800000, 0x3f800000},
		{"add_adjacent_ulp", true, 0x3f800000, 0x34000000, 0x3f800001},
		{"sub_halfway_ties_even", false, 0x3f800001, 0x33800000, 0x3f800000},
		{"sub_adjacent_ulp", false, 0x3f800000, 0x33800000, 0x3f7fffff},
		{"add_large_small_absorption", true, 0x4c000000, 0x3f800000, 0x4c000000},
		{"sub_large_small_absorption", false, 0x4c000000, 0x3f800000, 0x4c000000},
		{"add_exact_cancellation", true, 0x3fc00000, 0xbfc00000, 0x00000000},
		{"sub_exact_cancellation", false, 0x3fc00000, 0x3fc00000, 0x00000000},
		{"add_subnormal_to_normal", true, 0x007fffff, 0x00000001, 0x00800000},
		{"sub_normal_to_subnormal", false, 0x00800000, 0x00000001, 0x007fffff},
		{"add_positive_zero", true, 0x00000000, 0x80000000, 0x00000000},
		{"add_negative_zero", true, 0x80000000, 0x80000000, 0x80000000},
		{"sub_positive_zero", false, 0x00000000, 0x00000000, 0x00000000},
		{"sub_negative_zero", false, 0x80000000, 0x00000000, 0x80000000},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got, portable, weights [HiddenSize]float32
			for lane := 0; lane < HiddenSize; lane++ {
				got[lane] = math.Float32frombits(test.accumulatorBits)
				portable[lane] = got[lane]
				weights[lane] = math.Float32frombits(test.weightBits)
			}
			if test.add {
				addFeatureRow(&got, &weights)
				addFeatureRowPortable(&portable, &weights)
			} else {
				subFeatureRow(&got, &weights)
				subFeatureRowPortable(&portable, &weights)
			}
			for lane := 0; lane < HiddenSize; lane++ {
				if gotBits := math.Float32bits(got[lane]); gotBits != test.wantBits {
					t.Fatalf("lane=%d bits=%08x want=%08x", lane, gotBits, test.wantBits)
				}
				if portableBits := math.Float32bits(portable[lane]); portableBits != test.wantBits {
					t.Fatalf("portable lane=%d bits=%08x want=%08x", lane, portableBits, test.wantBits)
				}
			}
		})
	}
}

func requireFeatureRowBits(t *testing.T, got, want [HiddenSize]float32, seed int64, sample int, add bool) {
	t.Helper()
	for lane := 0; lane < HiddenSize; lane++ {
		if math.Float32bits(got[lane]) != math.Float32bits(want[lane]) {
			t.Fatalf("seed=%d sample=%d add=%v lane=%d bits=%08x want=%08x", seed, sample, add, lane, math.Float32bits(got[lane]), math.Float32bits(want[lane]))
		}
	}
}
