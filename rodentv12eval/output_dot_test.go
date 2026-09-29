package rodentv12eval

import (
	"math"
	"testing"
	"unsafe"
)

var rodentV12OutputDotSink int32

func TestRodentV12OutputDotMatchesIndependentModularReference(t *testing.T) {
	tests := []struct {
		name string
		fill func(*[HiddenSize]int16, *[HiddenSize]int16, *[HiddenSize]int16, *[HiddenSize]int16)
	}{
		{name: "zero"},
		{name: "lane layout and extremes", fill: fillRodentV12OutputExtremes},
		{name: "positive overflow", fill: func(stm, nonSTM, stmWeights, nonSTMWeights *[HiddenSize]int16) {
			for lane := 0; lane < HiddenSize; lane++ {
				stm[lane], nonSTM[lane] = math.MaxInt16, inputScale
				stmWeights[lane], nonSTMWeights[lane] = math.MaxInt16, math.MaxInt16
			}
		}},
		{name: "negative overflow", fill: func(stm, nonSTM, stmWeights, nonSTMWeights *[HiddenSize]int16) {
			for lane := 0; lane < HiddenSize; lane++ {
				stm[lane], nonSTM[lane] = inputScale, math.MaxInt16
				stmWeights[lane], nonSTMWeights[lane] = math.MinInt16, math.MinInt16
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stm, nonSTM, stmWeights, nonSTMWeights [HiddenSize]int16
			if test.fill != nil {
				test.fill(&stm, &nonSTM, &stmWeights, &nonSTMWeights)
			}
			assertRodentV12OutputDot(t, &stm, &nonSTM, &stmWeights, &nonSTMWeights)
		})
	}
}

func TestRodentV12OutputDotRandomFullInt16Domain(t *testing.T) {
	state := uint64(0xbb67ae8584caa73b)
	next := func() int16 {
		state ^= state << 13
		state ^= state >> 7
		state ^= state << 17
		return int16(state)
	}
	for sample := 0; sample < 256; sample++ {
		var stm, nonSTM, stmWeights, nonSTMWeights [HiddenSize]int16
		for lane := 0; lane < HiddenSize; lane++ {
			stm[lane], nonSTM[lane] = next(), next()
			stmWeights[lane], nonSTMWeights[lane] = next(), next()
		}
		assertRodentV12OutputDot(t, &stm, &nonSTM, &stmWeights, &nonSTMWeights)
	}
}

func TestRodentV12OutputDotAllHeadsAndPerspectiveOrder(t *testing.T) {
	model := testModel()
	var accum accumulator
	var nonSTMWeights [HiddenSize]int16
	fillRodentV12OutputExtremes(&accum[White], &accum[Black], &model.outputWeights[0][0], &nonSTMWeights)
	for bucket := 0; bucket < OutputBuckets; bucket++ {
		for lane := 0; lane < HiddenSize; lane++ {
			model.outputWeights[bucket][0][lane] = int16(bucket*113 + lane*17)
			model.outputWeights[bucket][1][lane] = int16(-bucket*97 - lane*19)
		}
		model.outputBiases[bucket] = int16(bucket*31 - 79)
		for _, side := range []Color{White, Black} {
			stm := &accum[side]
			nonSTM := &accum[side^1]
			stmWeights := &model.outputWeights[bucket][0]
			nonSTMWeights := &model.outputWeights[bucket][1]
			sum := independentRodentV12OutputDot(stm, nonSTM, stmWeights, nonSTMWeights)
			sum = sum/inputScale + int32(model.outputBiases[bucket])
			want := int(sum * outputScale / (inputScale * layerScale))
			if got := model.evaluateAccumulator(&accum, side, bucket); got != want {
				t.Fatalf("bucket=%d side=%d raw=%d, want %d", bucket, side, got, want)
			}
		}
	}
}

func TestRodentV12OutputDotAcceptsUnalignedInputs(t *testing.T) {
	type shiftedStorage [HiddenSize + 1]int16
	var stmStorage, nonSTMStorage, stmWeightStorage, nonSTMWeightStorage shiftedStorage
	stm := (*[HiddenSize]int16)(unsafe.Pointer(&stmStorage[1]))
	nonSTM := (*[HiddenSize]int16)(unsafe.Pointer(&nonSTMStorage[1]))
	stmWeights := (*[HiddenSize]int16)(unsafe.Pointer(&stmWeightStorage[1]))
	nonSTMWeights := (*[HiddenSize]int16)(unsafe.Pointer(&nonSTMWeightStorage[1]))
	fillRodentV12OutputExtremes(stm, nonSTM, stmWeights, nonSTMWeights)
	assertRodentV12OutputDot(t, stm, nonSTM, stmWeights, nonSTMWeights)
}

func TestRodentV12OutputDotDoesNotMutateOrAllocate(t *testing.T) {
	var stm, nonSTM, stmWeights, nonSTMWeights [HiddenSize]int16
	fillRodentV12OutputExtremes(&stm, &nonSTM, &stmWeights, &nonSTMWeights)
	beforeSTM, beforeNonSTM := stm, nonSTM
	beforeSTMWeights, beforeNonSTMWeights := stmWeights, nonSTMWeights
	allocations := testing.AllocsPerRun(100, func() {
		rodentV12OutputDotSink = rodentV12OutputDot(&stm, &nonSTM, &stmWeights, &nonSTMWeights)
	})
	if allocations != 0 {
		t.Fatalf("Rodent V1.2 output allocations = %v, want 0", allocations)
	}
	if stm != beforeSTM || nonSTM != beforeNonSTM || stmWeights != beforeSTMWeights || nonSTMWeights != beforeNonSTMWeights {
		t.Fatal("Rodent V1.2 output kernel mutated an input")
	}
}

func assertRodentV12OutputDot(
	t *testing.T,
	stm, nonSTM, stmWeights, nonSTMWeights *[HiddenSize]int16,
) {
	t.Helper()
	want := independentRodentV12OutputDot(stm, nonSTM, stmWeights, nonSTMWeights)
	if portable := rodentV12OutputDotPortable(stm, nonSTM, stmWeights, nonSTMWeights); portable != want {
		t.Fatalf("portable output = %d, independent reference = %d", portable, want)
	}
	if got := rodentV12OutputDot(stm, nonSTM, stmWeights, nonSTMWeights); got != want {
		t.Fatalf("selected output = %d, independent reference = %d", got, want)
	}
}

func independentRodentV12OutputDot(
	stm, nonSTM, stmWeights, nonSTMWeights *[HiddenSize]int16,
) int32 {
	var sum uint32
	for lane := 0; lane < HiddenSize; lane++ {
		sum += uint32(independentRodentV12ClippedSquareWeight(stm[lane], stmWeights[lane]))
		sum += uint32(independentRodentV12ClippedSquareWeight(nonSTM[lane], nonSTMWeights[lane]))
	}
	return int32(sum)
}

func independentRodentV12ClippedSquareWeight(value, weight int16) int64 {
	v := int64(value)
	if v < 0 {
		v = 0
	} else if v > inputScale {
		v = inputScale
	}
	return v * v * int64(weight)
}

func fillRodentV12OutputExtremes(
	stm, nonSTM, stmWeights, nonSTMWeights *[HiddenSize]int16,
) {
	values := [...]int16{math.MinInt16, -32767, -256, -1, 0, 1, 2, 3, 127, 128, 129, 254, 255, 256, 32766, math.MaxInt16}
	weights := [...]int16{math.MinInt16, -32767, -1, 0, 1, 2, 32766, math.MaxInt16}
	for lane := 0; lane < HiddenSize; lane++ {
		stm[lane] = values[lane%len(values)]
		nonSTM[lane] = values[(lane*7+3)%len(values)]
		stmWeights[lane] = weights[(lane*5+1)%len(weights)]
		nonSTMWeights[lane] = weights[(lane*3+2)%len(weights)]
	}
}

func BenchmarkRodentV12OutputDot(b *testing.B) {
	var stm, nonSTM, stmWeights, nonSTMWeights [HiddenSize]int16
	fillRodentV12OutputExtremes(&stm, &nonSTM, &stmWeights, &nonSTMWeights)
	for _, benchmark := range []struct {
		name string
		run  func(*[HiddenSize]int16, *[HiddenSize]int16, *[HiddenSize]int16, *[HiddenSize]int16) int32
	}{
		{"portable", rodentV12OutputDotPortable},
		{"selected", rodentV12OutputDot},
	} {
		b.Run(benchmark.name, func(b *testing.B) {
			b.ReportAllocs()
			for iteration := 0; iteration < b.N; iteration++ {
				rodentV12OutputDotSink = benchmark.run(&stm, &nonSTM, &stmWeights, &nonSTMWeights)
			}
		})
	}
}
