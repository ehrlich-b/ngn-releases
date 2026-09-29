//go:build !amd64 || !amd64.v3

package rodentv12eval

const rodentV12OutputDotUsesAVX2 = false

func rodentV12OutputDot(
	stm, nonSTM, stmWeights, nonSTMWeights *[HiddenSize]int16,
) int32 {
	return rodentV12OutputDotPortable(stm, nonSTM, stmWeights, nonSTMWeights)
}
