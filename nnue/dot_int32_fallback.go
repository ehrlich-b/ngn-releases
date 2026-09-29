//go:build !amd64 || !amd64.v3

package nnue

const boundedOutputDotUsesAVX2 = false

func boundedOutputDot(
	us *[HiddenSize]int32,
	them *[HiddenSize]int32,
	weights *[PerspectiveCount * HiddenSize]int16,
) int32 {
	return boundedOutputDotPortable(us, them, weights)
}
