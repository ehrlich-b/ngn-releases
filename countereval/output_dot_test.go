package countereval

import (
	"math"
	"testing"
)

var counterOutputDotSink float32

func requireOutputDotBits(t *testing.T, accumulator, weights *[HiddenSize]float32) {
	t.Helper()
	beforeAccumulator := *accumulator
	beforeWeights := *weights
	want := counterOutputDotPortable(accumulator, weights)
	got := counterOutputDot(accumulator, weights)
	if gotBits, wantBits := math.Float32bits(got), math.Float32bits(want); gotBits != wantBits {
		t.Fatalf("output bits=%08x want=%08x", gotBits, wantBits)
	}
	for lane := range accumulator {
		if got, want := math.Float32bits(accumulator[lane]), math.Float32bits(beforeAccumulator[lane]); got != want {
			t.Fatalf("accumulator lane %d mutated: bits=%08x want=%08x", lane, got, want)
		}
		if got, want := math.Float32bits(weights[lane]), math.Float32bits(beforeWeights[lane]); got != want {
			t.Fatalf("weight lane %d mutated: bits=%08x want=%08x", lane, got, want)
		}
	}
}

func TestCounterOutputDotAllActivityMasks(t *testing.T) {
	for activityMask := 0; activityMask < 1<<8; activityMask++ {
		var accumulator, weights [HiddenSize]float32
		for lane := range accumulator {
			withinBlock := lane & 7
			value := float32((lane&15)+1) * 0.0625
			if activityMask&(1<<withinBlock) != 0 {
				accumulator[lane] = value
			} else if lane&1 == 0 {
				accumulator[lane] = -value
			} else {
				accumulator[lane] = math.Float32frombits(uint32(lane&1) << 31)
			}
			weights[lane] = float32((lane%13)-6) * 0.03125
		}
		t.Run(activityMaskName(uint8(activityMask)), func(t *testing.T) {
			requireOutputDotBits(t, &accumulator, &weights)
		})
	}
}

func activityMaskName(mask uint8) string {
	const hex = "0123456789abcdef"
	return "mask_" + string([]byte{hex[mask>>4], hex[mask&15]})
}

func TestCounterOutputDotIEEEBoundaries(t *testing.T) {
	var accumulator, weights [HiddenSize]float32
	accumulator[0] = 0
	accumulator[1] = math.Float32frombits(1 << 31)
	accumulator[2] = math.SmallestNonzeroFloat32
	accumulator[3] = -math.SmallestNonzeroFloat32
	accumulator[4] = math.Float32frombits(0x007fffff)
	accumulator[5] = math.Float32frombits(0x00800000)
	accumulator[6] = 1
	accumulator[7] = math.Float32frombits(0x3f7fffff)
	weights[0] = math.MaxFloat32
	weights[1] = -math.MaxFloat32
	weights[2] = 0.5
	weights[3] = math.MaxFloat32
	weights[4] = -1
	weights[5] = math.SmallestNonzeroFloat32
	weights[6] = math.Float32frombits(1 << 31)
	weights[7] = math.Float32frombits(0x3f800001)
	requireOutputDotBits(t, &accumulator, &weights)
}

func TestCounterOutputDotRoundsProductsBeforeOrderedAddition(t *testing.T) {
	t.Run("non_fused_product", func(t *testing.T) {
		var accumulator, weights [HiddenSize]float32
		accumulator[0] = 1
		weights[0] = -1
		accumulator[1] = math.Float32frombits(0x3f800001)
		weights[1] = math.Float32frombits(0x3f7ffffe)
		requireOutputDotBits(t, &accumulator, &weights)
		if got := math.Float32bits(counterOutputDot(&accumulator, &weights)); got != 0 {
			t.Fatalf("non-fused witness bits=%08x want=00000000", got)
		}
	})

	t.Run("cross_block_order", func(t *testing.T) {
		var accumulator, weights [HiddenSize]float32
		accumulator[7], weights[7] = 1, 1<<24
		accumulator[8], weights[8] = 1, 1
		accumulator[9], weights[9] = 1, -(1 << 24)
		requireOutputDotBits(t, &accumulator, &weights)
		if got := math.Float32bits(counterOutputDot(&accumulator, &weights)); got != 0 {
			t.Fatalf("ordered witness bits=%08x want=00000000", got)
		}
	})

	t.Run("not_eight_partial_sums", func(t *testing.T) {
		var accumulator, weights [HiddenSize]float32
		accumulator[0], weights[0] = 1, 1<<24
		accumulator[1], weights[1] = 1, 1
		accumulator[8], weights[8] = 1, -(1 << 24)
		requireOutputDotBits(t, &accumulator, &weights)
		if got := math.Float32bits(counterOutputDot(&accumulator, &weights)); got != 0 {
			t.Fatalf("partial-sum witness bits=%08x want=00000000", got)
		}
	})
}

func TestEvaluateAccumulatorAddsBiasLast(t *testing.T) {
	var model Model
	var accumulator [HiddenSize]float32
	accumulator[7], model.outputWeights[7] = 1, 1<<24
	accumulator[8], model.outputWeights[8] = 1, 1
	accumulator[9], model.outputWeights[9] = 1, -(1 << 24)
	model.outputBias = 1
	if got := math.Float32bits(model.evaluateAccumulator(&accumulator)); got != math.Float32bits(1) {
		t.Fatalf("bias-last witness bits=%08x want=%08x", got, math.Float32bits(1))
	}
}

func TestCounterOutputDotAdversarialFinite(t *testing.T) {
	state := uint64(0x4d595df4d0f33173)
	next := func() uint32 {
		state ^= state << 13
		state ^= state >> 7
		state ^= state << 17
		return uint32(state >> 16)
	}
	for trial := 0; trial < 128; trial++ {
		var accumulator, weights [HiddenSize]float32
		for lane := range accumulator {
			aBits := next()
			wBits := next()
			// Preserve random signs and mantissas while bounding exponents so every
			// product and cumulative result stays finite, as required by the loader.
			aBits = aBits&0x807fffff | uint32(96+next()%48)<<23
			wBits = wBits&0x807fffff | uint32(96+next()%48)<<23
			accumulator[lane] = math.Float32frombits(aBits)
			weights[lane] = math.Float32frombits(wBits)
		}
		requireOutputDotBits(t, &accumulator, &weights)
	}
}

func TestCounterOutputDotDoesNotAllocate(t *testing.T) {
	var accumulator, weights [HiddenSize]float32
	for lane := range accumulator {
		accumulator[lane] = float32((lane%17)-8) * 0.125
		weights[lane] = float32((lane%11)-5) * 0.0625
	}
	if got := testing.AllocsPerRun(1000, func() {
		counterOutputDotSink = counterOutputDot(&accumulator, &weights)
	}); got != 0 {
		t.Fatalf("allocations/run=%v want=0", got)
	}
}
