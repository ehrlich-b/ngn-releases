//go:build !amd64 || !amd64.v3

package rodenteval

const rodentOutputDotUsesAVX2 = false

func rodentOutputDot(
	us, them, usWeights, themWeights *[HiddenSize]int16,
) int32 {
	return rodentOutputDotPortable(us, them, usWeights, themWeights)
}
