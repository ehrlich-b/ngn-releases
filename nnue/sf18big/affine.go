package sf18big

// affineRowValue1024Portable evaluates the fixed-width fc0 row. Its input is
// the transformer's clipped product, so every lane is in [0, 127].
func affineRowValue1024Portable(input *[transformerLanes]uint8, weights *int8, bias int32) int32 {
	weightSlice := unsafeInt8Slice(weights, transformerLanes)
	value := bias
	for index, inputValue := range input {
		value = wrapAdd32(value, int32(inputValue)*int32(weightSlice[index]))
	}
	return value
}

func affineLayer32x32Portable(output *[32]int32, input *[32]uint8, weights *int8, biases *[32]int32) {
	weightSlice := unsafeInt8Slice(weights, 32*32)
	for row := range output {
		value := biases[row]
		for column, inputValue := range input {
			value = wrapAdd32(value, int32(inputValue)*int32(weightSlice[row*32+column]))
		}
		output[row] = value
	}
}

func affineRowValue32Portable(input *[32]uint8, weights *int8, bias int32) int32 {
	weightSlice := unsafeInt8Slice(weights, 32)
	value := bias
	for index, inputValue := range input {
		value = wrapAdd32(value, int32(inputValue)*int32(weightSlice[index]))
	}
	return value
}
