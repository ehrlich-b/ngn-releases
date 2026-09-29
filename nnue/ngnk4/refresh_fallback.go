//go:build !amd64 || !amd64.v3

package ngnk4

const k4RefreshPerspectiveUsesAVX2 = false

func k4RefreshPerspective(
	destination, biases *[HiddenSize]int16,
	rows *[k4MaximumRefreshRows]*[HiddenSize]int16,
	rowCount int,
) {
	k4RefreshPerspectivePortable(destination, biases, rows, rowCount)
}
