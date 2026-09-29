package rodentv12eval

// A validated board has disjoint piece planes, so at most one input row can
// be selected for each of its 64 squares.
const rodentV12MaximumRefreshRows = 64

// rodentV12RefreshPerspectivePortable is the arithmetic oracle for a
// king-perspective refresh. Every addition intentionally wraps in int16, and
// rows are accumulated in their original plane-and-square order.
func rodentV12RefreshPerspectivePortable(
	destination, biases *[HiddenSize]int16,
	rows *[rodentV12MaximumRefreshRows]*[HiddenSize]int16,
	rowCount int,
) {
	for hidden := 0; hidden < HiddenSize; hidden++ {
		value := biases[hidden]
		for row := 0; row < rowCount; row++ {
			value += rows[row][hidden]
		}
		destination[hidden] = value
	}
}
