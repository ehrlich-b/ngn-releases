//go:build amd64 && amd64.v3

package nnue

const boundedOutputDotUsesAVX2 = true

// The assembly loop and clipping immediate are intentionally fixed to the
// validated NGN-v1 shape. These pairs fail compilation if either contract
// constant changes without a corresponding kernel review.
var (
	_ [HiddenSize - 128]byte
	_ [128 - HiddenSize]byte
	_ [PerspectiveCount - 2]byte
	_ [2 - PerspectiveCount]byte
	_ [QA - 255]byte
	_ [255 - QA]byte
)

func boundedOutputDot(
	us *[HiddenSize]int32,
	them *[HiddenSize]int32,
	weights *[PerspectiveCount * HiddenSize]int16,
) int32 {
	return boundedOutputDotAVX2(us, them, weights)
}

//go:noescape
func boundedOutputDotAVX2(
	us *[HiddenSize]int32,
	them *[HiddenSize]int32,
	weights *[PerspectiveCount * HiddenSize]int16,
) int32
