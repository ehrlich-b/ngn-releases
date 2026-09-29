//go:build !amd64 || !amd64.v3

package countereval

const counterOutputDotUsesAVX2 = false

func counterOutputDot(accumulator, weights *[HiddenSize]float32) float32 {
	return counterOutputDotPortable(accumulator, weights)
}
