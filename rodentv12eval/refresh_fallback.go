//go:build !amd64 || !amd64.v3

package rodentv12eval

const rodentV12RefreshPerspectiveUsesAVX2 = false

func rodentV12RefreshPerspective(
	destination, biases *[HiddenSize]int16,
	rows *[rodentV12MaximumRefreshRows]*[HiddenSize]int16,
	rowCount int,
) {
	rodentV12RefreshPerspectivePortable(destination, biases, rows, rowCount)
}
