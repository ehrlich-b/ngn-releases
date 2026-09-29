package nnue

// boundedOutputDotPortable is the exact scalar implementation for validated
// output weights in [-128, 128]. The sum of absolute products is bounded by
// 256*255^2*128 = 2,130,739,200, so every partial sum fits signed int32.
func boundedOutputDotPortable(
	us *[HiddenSize]int32,
	them *[HiddenSize]int32,
	weights *[PerspectiveCount * HiddenSize]int16,
) int32 {
	var dot int32
	for hidden := 0; hidden < HiddenSize; hidden++ {
		ours := clippedActivation32(us[hidden])
		theirs := clippedActivation32(them[hidden])
		dot += ours * ours * int32(weights[hidden])
		dot += theirs * theirs * int32(weights[HiddenSize+hidden])
	}
	return dot
}

func clippedActivation32(value int32) int32 {
	if value <= 0 {
		return 0
	}
	if value >= QA {
		return QA
	}
	return value
}
