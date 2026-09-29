package ngnk4

// A validated board has disjoint piece planes, so at most one input row can
// be selected for each of its 64 squares.
const k4MaximumRefreshRows = 64

// k4RefreshPerspectivePortable is the arithmetic oracle for a
// king-perspective refresh. Every addition intentionally wraps in int16, and
// rows are accumulated in their original plane-and-square order.
func k4RefreshPerspectivePortable(
	destination, biases *[HiddenSize]int16,
	rows *[k4MaximumRefreshRows]*[HiddenSize]int16,
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
