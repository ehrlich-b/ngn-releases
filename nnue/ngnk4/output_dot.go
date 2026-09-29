package ngnk4

// k4OutputDotPortable is the arithmetic oracle for the fast-safe output
// heads. Every product is converted to int32 and every addition wraps modulo
// 2^32; the loader admits the fast path only when no sum can wrap. Modular
// addition is associative, so the SIMD kernel may reassociate lanes without
// changing the exact result.
func k4OutputDotPortable(
	stm, nonSTM, stmWeights, nonSTMWeights *[HiddenSize]int16,
) int32 {
	var sum int32
	for hidden := 0; hidden < HiddenSize; hidden++ {
		sum += clippedSquaredWeightedFast(stm[hidden], stmWeights[hidden])
		sum += clippedSquaredWeightedFast(nonSTM[hidden], nonSTMWeights[hidden])
	}
	return sum
}

func clippedSquaredWeightedFast(value, weight int16) int32 {
	v := int32(value)
	if v < 0 {
		v = 0
	} else if v > InputScale {
		v = InputScale
	}
	return v * v * int32(weight)
}
