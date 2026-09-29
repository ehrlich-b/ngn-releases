//go:build amd64 && amd64.v3

package ngnk4

const k4OutputDotUsesAVX2 = true

// These pairs fail compilation if the hidden width is not a whole number of
// 16-lane blocks; the kernels iterate k4LaneBlocks times.
var (
	_ [-(HiddenSize % 16)]byte
)

func k4OutputDot(
	stm, nonSTM, stmWeights, nonSTMWeights *[HiddenSize]int16,
) int32 {
	return k4OutputDotAVX2(stm, nonSTM, stmWeights, nonSTMWeights)
}

//go:noescape
func k4OutputDotAVX2(
	stm, nonSTM, stmWeights, nonSTMWeights *[HiddenSize]int16,
) int32
