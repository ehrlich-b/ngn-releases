package rodentv12eval

// rodentV12OutputDotPortable is the canonical output-layer accumulator. Every
// product is converted to int32 and every addition intentionally wraps modulo
// 2^32, matching the released evaluator. Modular addition is associative, so
// the SIMD kernel may reassociate lanes without changing the exact result.
func rodentV12OutputDotPortable(
	stm, nonSTM, stmWeights, nonSTMWeights *[HiddenSize]int16,
) int32 {
	var sum int32
	for hidden := 0; hidden < HiddenSize; hidden++ {
		sum += clippedSquaredWeighted(stm[hidden], stmWeights[hidden])
		sum += clippedSquaredWeighted(nonSTM[hidden], nonSTMWeights[hidden])
	}
	return sum
}
