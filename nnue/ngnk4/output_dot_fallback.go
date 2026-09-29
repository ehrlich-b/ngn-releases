//go:build !amd64 || !amd64.v3

package ngnk4

const k4OutputDotUsesAVX2 = false

func k4OutputDot(
	stm, nonSTM, stmWeights, nonSTMWeights *[HiddenSize]int16,
) int32 {
	return k4OutputDotPortable(stm, nonSTM, stmWeights, nonSTMWeights)
}
