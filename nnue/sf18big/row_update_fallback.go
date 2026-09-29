//go:build !amd64 || !amd64.v3

package sf18big

const rowUpdateUsesAVX2 = false

func updateBaseRow(values *[transformerLanes]int16, weights *int16, psqt *[psqtBuckets]int32, psqtWeights *int32, add bool) {
	updateBaseRowPortable(values, weights, psqt, psqtWeights, add)
}

func updateThreatRow(values *[transformerLanes]int16, weights *int8, psqt *[psqtBuckets]int32, psqtWeights *int32, add bool) {
	updateThreatRowPortable(values, weights, psqt, psqtWeights, add)
}
