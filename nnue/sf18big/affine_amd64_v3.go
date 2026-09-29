//go:build amd64 && amd64.v3

package sf18big

const affine1024UsesAVX2 = true
const affine32UsesAVX2 = true

var (
	_ [transformerLanes - 1024]byte
	_ [1024 - transformerLanes]byte
)

func affineRowValue1024(input *[transformerLanes]uint8, weights *int8, bias int32) int32 {
	return affineRowValue1024AVX2(input, weights, bias)
}

func affineLayer32x32(output *[32]int32, input *[32]uint8, weights *int8, biases *[32]int32) {
	affineLayer32x32AVX2(output, input, weights, biases)
}

func affineRowValue32(input *[32]uint8, weights *int8, bias int32) int32 {
	return affineRowValue32AVX2(input, weights, bias)
}

//go:noescape
func affineRowValue1024AVX2(input *[transformerLanes]uint8, weights *int8, bias int32) int32

//go:noescape
func affineLayer32x32AVX2(output *[32]int32, input *[32]uint8, weights *int8, biases *[32]int32)

//go:noescape
func affineRowValue32AVX2(input *[32]uint8, weights *int8, bias int32) int32
