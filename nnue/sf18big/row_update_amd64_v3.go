//go:build amd64 && amd64.v3

package sf18big

const rowUpdateUsesAVX2 = true

var (
	_ [transformerLanes - 1024]byte
	_ [1024 - transformerLanes]byte
	_ [psqtBuckets - 8]byte
	_ [8 - psqtBuckets]byte
)

func updateBaseRow(values *[transformerLanes]int16, weights *int16, psqt *[psqtBuckets]int32, psqtWeights *int32, add bool) {
	if add {
		addBaseRowAVX2(values, weights, psqt, psqtWeights)
	} else {
		subBaseRowAVX2(values, weights, psqt, psqtWeights)
	}
}

func updateThreatRow(values *[transformerLanes]int16, weights *int8, psqt *[psqtBuckets]int32, psqtWeights *int32, add bool) {
	if add {
		addThreatRowAVX2(values, weights, psqt, psqtWeights)
	} else {
		subThreatRowAVX2(values, weights, psqt, psqtWeights)
	}
}

//go:noescape
func addBaseRowAVX2(values *[transformerLanes]int16, weights *int16, psqt *[psqtBuckets]int32, psqtWeights *int32)

//go:noescape
func subBaseRowAVX2(values *[transformerLanes]int16, weights *int16, psqt *[psqtBuckets]int32, psqtWeights *int32)

//go:noescape
func addThreatRowAVX2(values *[transformerLanes]int16, weights *int8, psqt *[psqtBuckets]int32, psqtWeights *int32)

//go:noescape
func subThreatRowAVX2(values *[transformerLanes]int16, weights *int8, psqt *[psqtBuckets]int32, psqtWeights *int32)
