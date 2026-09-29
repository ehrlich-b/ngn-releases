//go:build amd64 && amd64.v3

package rodenteval

const rodentOutputDotUsesAVX2 = true

// These pairs fail compilation if the fixed Anand hidden width changes
// without a corresponding kernel review.
var (
	_ [HiddenSize - 512]byte
	_ [512 - HiddenSize]byte
)

func rodentOutputDot(
	us, them, usWeights, themWeights *[HiddenSize]int16,
) int32 {
	return rodentOutputDotAVX2(us, them, usWeights, themWeights)
}

//go:noescape
func rodentOutputDotAVX2(
	us, them, usWeights, themWeights *[HiddenSize]int16,
) int32
