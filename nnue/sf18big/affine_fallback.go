//go:build !amd64 || !amd64.v3

package sf18big

const affine1024UsesAVX2 = false
const affine32UsesAVX2 = false

func affineRowValue1024(input *[transformerLanes]uint8, weights *int8, bias int32) int32 {
	return affineRowValue1024Portable(input, weights, bias)
}

func affineLayer32x32(output *[32]int32, input *[32]uint8, weights *int8, biases *[32]int32) {
	affineLayer32x32Portable(output, input, weights, biases)
}

func affineRowValue32(input *[32]uint8, weights *int8, bias int32) int32 {
	return affineRowValue32Portable(input, weights, bias)
}
