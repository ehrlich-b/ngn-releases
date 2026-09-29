//go:build !amd64 || !amd64.v3

package rodentv12eval

const rodentV12ApplyUpdatesUsesAVX2 = false

func rodentV12ApplyUpdates2(
	destination, add0, subtract1 *[HiddenSize]int16,
) {
	rodentV12ApplyUpdates2Portable(destination, add0, subtract1)
}

func rodentV12ApplyUpdates3(
	destination, add0, subtract1, subtract2 *[HiddenSize]int16,
) {
	rodentV12ApplyUpdates3Portable(destination, add0, subtract1, subtract2)
}

func rodentV12ApplyUpdates4(
	destination, add0, subtract1, add2, subtract3 *[HiddenSize]int16,
) {
	rodentV12ApplyUpdates4Portable(destination, add0, subtract1, add2, subtract3)
}
