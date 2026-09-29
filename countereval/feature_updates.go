package countereval

// The portable row kernels define the Counter compatibility order. Each lane
// performs exactly one float32 add or subtract; callers invoke one complete row
// at a time so the semantic feature-update order remains unchanged.
func addFeatureRowPortable(accumulator, weights *[HiddenSize]float32) {
	for hidden := 0; hidden < HiddenSize; hidden++ {
		accumulator[hidden] += weights[hidden]
	}
}

func subFeatureRowPortable(accumulator, weights *[HiddenSize]float32) {
	for hidden := 0; hidden < HiddenSize; hidden++ {
		accumulator[hidden] -= weights[hidden]
	}
}
