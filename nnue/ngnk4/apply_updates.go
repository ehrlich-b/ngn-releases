package ngnk4

// These portable kernels are the arithmetic oracle for incremental feature
// updates. Every operation intentionally wraps in int16 and stays in the
// semantic order emitted by deriveTransition.
func k4ApplyUpdates2Portable(
	destination, add0, subtract1 *[HiddenSize]int16,
) {
	for hidden := 0; hidden < HiddenSize; hidden++ {
		value := destination[hidden]
		value += add0[hidden]
		value -= subtract1[hidden]
		destination[hidden] = value
	}
}

func k4ApplyUpdates3Portable(
	destination, add0, subtract1, subtract2 *[HiddenSize]int16,
) {
	for hidden := 0; hidden < HiddenSize; hidden++ {
		value := destination[hidden]
		value += add0[hidden]
		value -= subtract1[hidden]
		value -= subtract2[hidden]
		destination[hidden] = value
	}
}

func k4ApplyUpdates4Portable(
	destination, add0, subtract1, add2, subtract3 *[HiddenSize]int16,
) {
	for hidden := 0; hidden < HiddenSize; hidden++ {
		value := destination[hidden]
		value += add0[hidden]
		value -= subtract1[hidden]
		value += add2[hidden]
		value -= subtract3[hidden]
		destination[hidden] = value
	}
}
