//go:build !amd64 || !amd64.v3

package ngnk4

const k4ApplyUpdatesUsesAVX2 = false

func k4ApplyUpdates2(
	destination, add0, subtract1 *[HiddenSize]int16,
) {
	k4ApplyUpdates2Portable(destination, add0, subtract1)
}

func k4ApplyUpdates3(
	destination, add0, subtract1, subtract2 *[HiddenSize]int16,
) {
	k4ApplyUpdates3Portable(destination, add0, subtract1, subtract2)
}

func k4ApplyUpdates4(
	destination, add0, subtract1, add2, subtract3 *[HiddenSize]int16,
) {
	k4ApplyUpdates4Portable(destination, add0, subtract1, add2, subtract3)
}
