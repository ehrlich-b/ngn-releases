package nnue

// AccumulatorSnapshotForTest returns a value copy for independent test oracles.
// It is compiled only into tests and cannot mutate Context state.
func AccumulatorSnapshotForTest(c *Context) ([PerspectiveCount][HiddenSize]int64, error) {
	if err := c.ready(); err != nil {
		return [PerspectiveCount][HiddenSize]int64{}, err
	}
	return c.states[c.depth].accumulators, nil
}

// Int32AccumulatorSnapshotForTest returns the narrow stored lanes without
// widening, so tests can verify both representation and numerical parity.
func Int32AccumulatorSnapshotForTest(c *Int32Context) ([PerspectiveCount][HiddenSize]int32, error) {
	if err := c.ready(); err != nil {
		return [PerspectiveCount][HiddenSize]int32{}, err
	}
	return c.states[c.depth].accumulators, nil
}

// BoundedOutputDotsForTest compares the selected build-tag implementation with
// the portable bounded-int32 kernel on caller-owned lanes. Callers must supply
// validated bounded output weights in [-128, 128].
func BoundedOutputDotsForTest(
	us [HiddenSize]int32,
	them [HiddenSize]int32,
	weights [PerspectiveCount * HiddenSize]int16,
) (selected int32, portable int32, avx2 bool) {
	return boundedOutputDot(&us, &them, &weights),
		boundedOutputDotPortable(&us, &them, &weights),
		boundedOutputDotUsesAVX2
}
