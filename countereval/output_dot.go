package countereval

// counterOutputDotPortable is the canonical output-layer arithmetic order.
// Keep the product conversion explicit so compiler targets cannot fuse the
// multiplication with the following addition.
func counterOutputDotPortable(accumulator, weights *[HiddenSize]float32) float32 {
	var output float32
	for hidden, value := range accumulator {
		if value > 0 {
			product := float32(value * weights[hidden])
			output += product
		}
	}
	return output
}
