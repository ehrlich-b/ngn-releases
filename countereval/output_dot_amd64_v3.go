//go:build amd64 && amd64.v3

package countereval

const counterOutputDotUsesAVX2 = true

// These pairs fail compilation if the fixed Counter hidden width changes
// without a corresponding kernel review.
var (
	_ [HiddenSize - 512]byte
	_ [512 - HiddenSize]byte
)

func counterOutputDot(accumulator, weights *[HiddenSize]float32) float32 {
	return counterOutputDotAVX2(accumulator, weights)
}

//go:noescape
func counterOutputDotAVX2(accumulator, weights *[HiddenSize]float32) float32
