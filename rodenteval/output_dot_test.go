package rodenteval

import (
	"math"
	"testing"
)

var rodentOutputDotSink int32

func TestRodentOutputDotMatchesIndependentModularReference(t *testing.T) {
	tests := []struct {
		name string
		fill func(*[HiddenSize]int16, *[HiddenSize]int16, *[HiddenSize]int16, *[HiddenSize]int16)
	}{
		{name: "zero"},
		{name: "lane layout and extremes", fill: fillRodentOutputExtremes},
		{name: "positive overflow", fill: func(us, them, usWeights, themWeights *[HiddenSize]int16) {
			for lane := 0; lane < HiddenSize; lane++ {
				us[lane], them[lane] = math.MaxInt16, inputScale
				usWeights[lane], themWeights[lane] = math.MaxInt16, math.MaxInt16
			}
		}},
		{name: "negative overflow", fill: func(us, them, usWeights, themWeights *[HiddenSize]int16) {
			for lane := 0; lane < HiddenSize; lane++ {
				us[lane], them[lane] = inputScale, math.MaxInt16
				usWeights[lane], themWeights[lane] = math.MinInt16, math.MinInt16
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var us, them, usWeights, themWeights [HiddenSize]int16
			if test.fill != nil {
				test.fill(&us, &them, &usWeights, &themWeights)
			}
			assertRodentOutputDot(t, &us, &them, &usWeights, &themWeights)
		})
	}
}

func TestRodentOutputDotRandomFullInt16Domain(t *testing.T) {
	state := uint64(0x6a09e667f3bcc909)
	next := func() int16 {
		state ^= state << 13
		state ^= state >> 7
		state ^= state << 17
		return int16(state)
	}
	for sample := 0; sample < 256; sample++ {
		var us, them, usWeights, themWeights [HiddenSize]int16
		for lane := 0; lane < HiddenSize; lane++ {
			us[lane], them[lane] = next(), next()
			usWeights[lane], themWeights[lane] = next(), next()
		}
		assertRodentOutputDot(t, &us, &them, &usWeights, &themWeights)
	}
}

func TestRodentOutputDotSideAndWeightLayout(t *testing.T) {
	var us, them, usWeights, themWeights [HiddenSize]int16
	us[0], them[0] = 1, 2
	us[1], them[1] = 3, 4
	us[127], them[127] = 5, 6
	us[128], them[128] = 7, 8
	us[511], them[511] = 254, 255
	usWeights[0], themWeights[0] = 11, 13
	usWeights[1], themWeights[1] = 17, 19
	usWeights[127], themWeights[127] = 23, 29
	usWeights[128], themWeights[128] = 31, 37
	usWeights[511], themWeights[511] = math.MinInt16, math.MaxInt16
	assertRodentOutputDot(t, &us, &them, &usWeights, &themWeights)
	assertRodentOutputDot(t, &them, &us, &usWeights, &themWeights)
}

func TestRodentOutputDotDoesNotMutateOrAllocate(t *testing.T) {
	var us, them, usWeights, themWeights [HiddenSize]int16
	fillRodentOutputExtremes(&us, &them, &usWeights, &themWeights)
	beforeUs, beforeThem := us, them
	beforeUsWeights, beforeThemWeights := usWeights, themWeights
	allocations := testing.AllocsPerRun(100, func() {
		rodentOutputDotSink = rodentOutputDot(&us, &them, &usWeights, &themWeights)
	})
	if allocations != 0 {
		t.Fatalf("Rodent output allocations = %v, want 0", allocations)
	}
	if us != beforeUs || them != beforeThem || usWeights != beforeUsWeights || themWeights != beforeThemWeights {
		t.Fatal("Rodent output kernel mutated an input")
	}
}

func assertRodentOutputDot(
	t *testing.T,
	us, them, usWeights, themWeights *[HiddenSize]int16,
) {
	t.Helper()
	want := independentRodentOutputDot(us, them, usWeights, themWeights)
	if portable := rodentOutputDotPortable(us, them, usWeights, themWeights); portable != want {
		t.Fatalf("portable output = %d, independent reference = %d", portable, want)
	}
	if got := rodentOutputDot(us, them, usWeights, themWeights); got != want {
		t.Fatalf("selected output = %d, independent reference = %d", got, want)
	}
}

func independentRodentOutputDot(
	us, them, usWeights, themWeights *[HiddenSize]int16,
) int32 {
	var sum uint32
	for lane := 0; lane < HiddenSize; lane++ {
		sum += uint32(independentClippedSquareWeight(us[lane], usWeights[lane]))
		sum += uint32(independentClippedSquareWeight(them[lane], themWeights[lane]))
	}
	return int32(sum)
}

func independentClippedSquareWeight(value, weight int16) int64 {
	v := int64(value)
	if v < 0 {
		v = 0
	} else if v > inputScale {
		v = inputScale
	}
	return v * v * int64(weight)
}

func fillRodentOutputExtremes(
	us, them, usWeights, themWeights *[HiddenSize]int16,
) {
	values := [...]int16{math.MinInt16, -32767, -256, -1, 0, 1, 2, 3, 127, 128, 129, 254, 255, 256, 32766, math.MaxInt16}
	weights := [...]int16{math.MinInt16, -32767, -1, 0, 1, 2, 32766, math.MaxInt16}
	for lane := 0; lane < HiddenSize; lane++ {
		us[lane] = values[lane%len(values)]
		them[lane] = values[(lane*7+3)%len(values)]
		usWeights[lane] = weights[(lane*5+1)%len(weights)]
		themWeights[lane] = weights[(lane*3+2)%len(weights)]
	}
}
