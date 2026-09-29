//go:build amd64 && amd64.v3

package rodentv12eval

const rodentV12OutputDotUsesAVX2 = true

// These pairs fail compilation if the fixed V1.2 hidden width changes without
// a corresponding kernel review.
var (
	_ [HiddenSize - 768]byte
	_ [768 - HiddenSize]byte
)

func rodentV12OutputDot(
	stm, nonSTM, stmWeights, nonSTMWeights *[HiddenSize]int16,
) int32 {
	return rodentV12OutputDotAVX2(stm, nonSTM, stmWeights, nonSTMWeights)
}

//go:noescape
func rodentV12OutputDotAVX2(
	stm, nonSTM, stmWeights, nonSTMWeights *[HiddenSize]int16,
) int32
