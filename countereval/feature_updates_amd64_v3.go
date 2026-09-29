//go:build amd64 && amd64.v3

package countereval

const featureUpdateRowsUseAVX2 = true

// These pairs fail compilation if the fixed Counter hidden width changes
// without a corresponding kernel review.
var (
	_ [HiddenSize - 512]byte
	_ [512 - HiddenSize]byte
)

func addFeatureRow(accumulator, weights *[HiddenSize]float32) {
	addFeatureRowAVX2(accumulator, weights)
}

func subFeatureRow(accumulator, weights *[HiddenSize]float32) {
	subFeatureRowAVX2(accumulator, weights)
}

//go:noescape
func addFeatureRowAVX2(accumulator, weights *[HiddenSize]float32)

//go:noescape
func subFeatureRowAVX2(accumulator, weights *[HiddenSize]float32)
