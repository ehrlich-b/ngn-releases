//go:build amd64 && amd64.v3

package rodentv12eval

const rodentV12RefreshPerspectiveUsesAVX2 = true

// These pairs fail compilation if the fixed V1.2 hidden width or the maximum
// refresh row count changes without a corresponding kernel review.
var (
	_ [HiddenSize - 768]byte
	_ [768 - HiddenSize]byte
	_ [rodentV12MaximumRefreshRows - 64]byte
	_ [64 - rodentV12MaximumRefreshRows]byte
)

func rodentV12RefreshPerspective(
	destination, biases *[HiddenSize]int16,
	rows *[rodentV12MaximumRefreshRows]*[HiddenSize]int16,
	rowCount int,
) {
	rodentV12RefreshPerspectiveAVX2(destination, biases, rows, rowCount)
}

//go:noescape
func rodentV12RefreshPerspectiveAVX2(
	destination, biases *[HiddenSize]int16,
	rows *[rodentV12MaximumRefreshRows]*[HiddenSize]int16,
	rowCount int,
)
