package rodenteval

// rodentOutputDotPortable is the canonical output-layer accumulator. Each
// addition intentionally wraps in int32, matching the released evaluator.
func rodentOutputDotPortable(
	us, them, usWeights, themWeights *[HiddenSize]int16,
) int32 {
	var sum int32
	for hidden := 0; hidden < HiddenSize; hidden++ {
		sum += clippedSquaredWeighted(us[hidden], usWeights[hidden])
		sum += clippedSquaredWeighted(them[hidden], themWeights[hidden])
	}
	return sum
}
