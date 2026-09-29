package sf18big

import "unsafe"

func unsafeInt8Slice(pointer *int8, length int) []int8 {
	return unsafe.Slice(pointer, length)
}

func unsafeInt16Slice(pointer *int16, length int) []int16 {
	return unsafe.Slice(pointer, length)
}

func unsafeInt32Slice(pointer *int32, length int) []int32 {
	return unsafe.Slice(pointer, length)
}
