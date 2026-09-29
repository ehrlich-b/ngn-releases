package ngnk4

import (
	"math"
	"testing"
	"unsafe"
)

var k4OutputDotSink int32

func TestK4OutputDotMatchesIndependentModularReference(t *testing.T) {
	tests := []struct {
		name string
		fill func(*[HiddenSize]int16, *[HiddenSize]int16, *[HiddenSize]int16, *[HiddenSize]int16)
	}{
		{name: "zero"},
		{name: "lane layout and extremes", fill: fillK4OutputExtremes},
		{name: "positive overflow", fill: func(stm, nonSTM, stmWeights, nonSTMWeights *[HiddenSize]int16) {
			for lane := 0; lane < HiddenSize; lane++ {
				stm[lane], nonSTM[lane] = math.MaxInt16, InputScale
				stmWeights[lane], nonSTMWeights[lane] = math.MaxInt16, math.MaxInt16
			}
		}},
		{name: "negative overflow", fill: func(stm, nonSTM, stmWeights, nonSTMWeights *[HiddenSize]int16) {
			for lane := 0; lane < HiddenSize; lane++ {
				stm[lane], nonSTM[lane] = InputScale, math.MaxInt16
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
			assertK4OutputDot(t, &stm, &nonSTM, &stmWeights, &nonSTMWeights)
		})
	}
}

func TestK4OutputDotRandomFullInt16Domain(t *testing.T) {
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
		assertK4OutputDot(t, &stm, &nonSTM, &stmWeights, &nonSTMWeights)
	}
}

func TestK4OutputDotAcceptsUnalignedInputs(t *testing.T) {
	type shiftedStorage [HiddenSize + 1]int16
	var stmStorage, nonSTMStorage, stmWeightStorage, nonSTMWeightStorage shiftedStorage
	stm := (*[HiddenSize]int16)(unsafe.Pointer(&stmStorage[1]))
	nonSTM := (*[HiddenSize]int16)(unsafe.Pointer(&nonSTMStorage[1]))
	stmWeights := (*[HiddenSize]int16)(unsafe.Pointer(&stmWeightStorage[1]))
	nonSTMWeights := (*[HiddenSize]int16)(unsafe.Pointer(&nonSTMWeightStorage[1]))
	fillK4OutputExtremes(stm, nonSTM, stmWeights, nonSTMWeights)
	assertK4OutputDot(t, stm, nonSTM, stmWeights, nonSTMWeights)
}

func TestK4OutputDotDoesNotMutateOrAllocate(t *testing.T) {
	var stm, nonSTM, stmWeights, nonSTMWeights [HiddenSize]int16
	fillK4OutputExtremes(&stm, &nonSTM, &stmWeights, &nonSTMWeights)
	beforeSTM, beforeNonSTM := stm, nonSTM
	beforeSTMWeights, beforeNonSTMWeights := stmWeights, nonSTMWeights
	allocations := testing.AllocsPerRun(100, func() {
		k4OutputDotSink = k4OutputDot(&stm, &nonSTM, &stmWeights, &nonSTMWeights)
	})
	if allocations != 0 {
		t.Fatalf("K4 output allocations = %v, want 0", allocations)
	}
	if stm != beforeSTM || nonSTM != beforeNonSTM || stmWeights != beforeSTMWeights || nonSTMWeights != beforeNonSTMWeights {
		t.Fatal("K4 output kernel mutated an input")
	}
}

func assertK4OutputDot(
	t *testing.T,
	stm, nonSTM, stmWeights, nonSTMWeights *[HiddenSize]int16,
) {
	t.Helper()
	want := independentK4OutputDot(stm, nonSTM, stmWeights, nonSTMWeights)
	if portable := k4OutputDotPortable(stm, nonSTM, stmWeights, nonSTMWeights); portable != want {
		t.Fatalf("portable output = %d, independent reference = %d", portable, want)
	}
	if got := k4OutputDot(stm, nonSTM, stmWeights, nonSTMWeights); got != want {
		t.Fatalf("selected output = %d, independent reference = %d", got, want)
	}
}

func independentK4OutputDot(
	stm, nonSTM, stmWeights, nonSTMWeights *[HiddenSize]int16,
) int32 {
	var sum uint32
	for lane := 0; lane < HiddenSize; lane++ {
		sum += uint32(independentK4ClippedSquareWeight(stm[lane], stmWeights[lane]))
		sum += uint32(independentK4ClippedSquareWeight(nonSTM[lane], nonSTMWeights[lane]))
	}
	return int32(sum)
}

func independentK4ClippedSquareWeight(value, weight int16) int64 {
	v := int64(value)
	if v < 0 {
		v = 0
	} else if v > InputScale {
		v = InputScale
	}
	return v * v * int64(weight)
}

func fillK4OutputExtremes(
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

func BenchmarkK4OutputDot(b *testing.B) {
	var stm, nonSTM, stmWeights, nonSTMWeights [HiddenSize]int16
	fillK4OutputExtremes(&stm, &nonSTM, &stmWeights, &nonSTMWeights)
	for _, benchmark := range []struct {
		name string
		run  func(*[HiddenSize]int16, *[HiddenSize]int16, *[HiddenSize]int16, *[HiddenSize]int16) int32
	}{
		{"portable", k4OutputDotPortable},
		{"selected", k4OutputDot},
	} {
		b.Run(benchmark.name, func(b *testing.B) {
			b.ReportAllocs()
			for iteration := 0; iteration < b.N; iteration++ {
				k4OutputDotSink = benchmark.run(&stm, &nonSTM, &stmWeights, &nonSTMWeights)
			}
		})
	}
}
