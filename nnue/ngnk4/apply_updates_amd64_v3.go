//go:build amd64 && amd64.v3

package ngnk4

const k4ApplyUpdatesUsesAVX2 = true

// These pairs fail compilation if the hidden width is not a whole number of
// 16-lane blocks; the kernels iterate k4LaneBlocks times.
var (
	_ [-(HiddenSize % 16)]byte
)

func k4ApplyUpdates2(
	destination, add0, subtract1 *[HiddenSize]int16,
) {
	k4ApplyUpdates2AVX2(destination, add0, subtract1)
}

func k4ApplyUpdates3(
	destination, add0, subtract1, subtract2 *[HiddenSize]int16,
) {
	k4ApplyUpdates3AVX2(destination, add0, subtract1, subtract2)
}

func k4ApplyUpdates4(
	destination, add0, subtract1, add2, subtract3 *[HiddenSize]int16,
) {
	k4ApplyUpdates4AVX2(destination, add0, subtract1, add2, subtract3)
}

//go:noescape
func k4ApplyUpdates2AVX2(
	destination, add0, subtract1 *[HiddenSize]int16,
)

//go:noescape
func k4ApplyUpdates3AVX2(
	destination, add0, subtract1, subtract2 *[HiddenSize]int16,
)

//go:noescape
func k4ApplyUpdates4AVX2(
	destination, add0, subtract1, add2, subtract3 *[HiddenSize]int16,
)
