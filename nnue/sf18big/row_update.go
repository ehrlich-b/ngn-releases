package sf18big

func updateBaseRowPortable(
	values *[transformerLanes]int16,
	weights *int16,
	psqt *[psqtBuckets]int32,
	psqtWeights *int32,
	add bool,
) {
	weightSlice := unsafeInt16Slice(weights, transformerLanes)
	psqtWeightSlice := unsafeInt32Slice(psqtWeights, psqtBuckets)
	for lane, weight := range weightSlice {
		if add {
			values[lane] = wrapAdd16(values[lane], weight)
		} else {
			values[lane] = wrapSub16(values[lane], weight)
		}
	}
	for bucket, weight := range psqtWeightSlice {
		if add {
			psqt[bucket] = wrapAdd32(psqt[bucket], weight)
		} else {
			psqt[bucket] = wrapSub32(psqt[bucket], weight)
		}
	}
}

func updateThreatRowPortable(
	values *[transformerLanes]int16,
	weights *int8,
	psqt *[psqtBuckets]int32,
	psqtWeights *int32,
	add bool,
) {
	weightSlice := unsafeInt8Slice(weights, transformerLanes)
	psqtWeightSlice := unsafeInt32Slice(psqtWeights, psqtBuckets)
	for lane, weight := range weightSlice {
		if add {
			values[lane] = wrapAdd16(values[lane], int16(weight))
		} else {
			values[lane] = wrapSub16(values[lane], int16(weight))
		}
	}
	for bucket, weight := range psqtWeightSlice {
		if add {
			psqt[bucket] = wrapAdd32(psqt[bucket], weight)
		} else {
			psqt[bucket] = wrapSub32(psqt[bucket], weight)
		}
	}
}
