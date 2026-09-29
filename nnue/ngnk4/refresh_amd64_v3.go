//go:build amd64 && amd64.v3

package ngnk4

const k4RefreshPerspectiveUsesAVX2 = true

// These pairs fail compilation if the fixed K4 hidden width or the maximum
// refresh row count changes without a corresponding kernel review.
var (
	_ [-(HiddenSize % 16)]byte
	_ [k4MaximumRefreshRows - 64]byte
	_ [64 - k4MaximumRefreshRows]byte
)

func k4RefreshPerspective(
	destination, biases *[HiddenSize]int16,
	rows *[k4MaximumRefreshRows]*[HiddenSize]int16,
	rowCount int,
) {
	k4RefreshPerspectiveAVX2(destination, biases, rows, rowCount)
}

//go:noescape
func k4RefreshPerspectiveAVX2(
	destination, biases *[HiddenSize]int16,
	rows *[k4MaximumRefreshRows]*[HiddenSize]int16,
	rowCount int,
)
