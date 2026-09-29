//go:build !amd64 || !amd64.v3

package rodenteval

const rodentApplyUpdatesUsesAVX2 = false

func rodentApplyUpdates2(
	destination, add0, subtract1 *[HiddenSize]int16,
) {
	rodentApplyUpdates2Portable(destination, add0, subtract1)
}

func rodentApplyUpdates3(
	destination, add0, subtract1, subtract2 *[HiddenSize]int16,
) {
	rodentApplyUpdates3Portable(destination, add0, subtract1, subtract2)
}

func rodentApplyUpdates4(
	destination, add0, subtract1, add2, subtract3 *[HiddenSize]int16,
) {
	rodentApplyUpdates4Portable(destination, add0, subtract1, add2, subtract3)
}
