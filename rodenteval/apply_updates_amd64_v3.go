//go:build amd64 && amd64.v3

package rodenteval

const rodentApplyUpdatesUsesAVX2 = true

// These pairs fail compilation if the fixed Anand hidden width changes
// without a corresponding kernel review.
var (
	_ [HiddenSize - 512]byte
	_ [512 - HiddenSize]byte
)

func rodentApplyUpdates2(
	destination, add0, subtract1 *[HiddenSize]int16,
) {
	rodentApplyUpdates2AVX2(destination, add0, subtract1)
}

func rodentApplyUpdates3(
	destination, add0, subtract1, subtract2 *[HiddenSize]int16,
) {
	rodentApplyUpdates3AVX2(destination, add0, subtract1, subtract2)
}

func rodentApplyUpdates4(
	destination, add0, subtract1, add2, subtract3 *[HiddenSize]int16,
) {
	rodentApplyUpdates4AVX2(destination, add0, subtract1, add2, subtract3)
}

//go:noescape
func rodentApplyUpdates2AVX2(
	destination, add0, subtract1 *[HiddenSize]int16,
)

//go:noescape
func rodentApplyUpdates3AVX2(
	destination, add0, subtract1, subtract2 *[HiddenSize]int16,
)

//go:noescape
func rodentApplyUpdates4AVX2(
	destination, add0, subtract1, add2, subtract3 *[HiddenSize]int16,
)
