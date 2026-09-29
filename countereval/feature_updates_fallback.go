//go:build !amd64 || !amd64.v3

package countereval

const featureUpdateRowsUseAVX2 = false

func addFeatureRow(accumulator, weights *[HiddenSize]float32) {
	addFeatureRowPortable(accumulator, weights)
}

func subFeatureRow(accumulator, weights *[HiddenSize]float32) {
	subFeatureRowPortable(accumulator, weights)
}
