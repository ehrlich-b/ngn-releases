//go:build amd64 && amd64.v3

package rodentv12eval

const rodentV12ApplyUpdatesUsesAVX2 = true

// These pairs fail compilation if the fixed V1.2 hidden width changes without
// a corresponding kernel review.
var (
	_ [HiddenSize - 768]byte
	_ [768 - HiddenSize]byte
)

func rodentV12ApplyUpdates2(
	destination, add0, subtract1 *[HiddenSize]int16,
) {
	rodentV12ApplyUpdates2AVX2(destination, add0, subtract1)
}

func rodentV12ApplyUpdates3(
	destination, add0, subtract1, subtract2 *[HiddenSize]int16,
) {
	rodentV12ApplyUpdates3AVX2(destination, add0, subtract1, subtract2)
}

func rodentV12ApplyUpdates4(
	destination, add0, subtract1, add2, subtract3 *[HiddenSize]int16,
) {
	rodentV12ApplyUpdates4AVX2(destination, add0, subtract1, add2, subtract3)
}

//go:noescape
func rodentV12ApplyUpdates2AVX2(
	destination, add0, subtract1 *[HiddenSize]int16,
)

//go:noescape
func rodentV12ApplyUpdates3AVX2(
	destination, add0, subtract1, subtract2 *[HiddenSize]int16,
)

//go:noescape
func rodentV12ApplyUpdates4AVX2(
	destination, add0, subtract1, add2, subtract3 *[HiddenSize]int16,
)
