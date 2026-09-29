package sf18big

import "testing"

func TestSelectedRowUpdateMatchesPortable(t *testing.T) {
	var baseWeights [transformerLanes]int16
	var threatWeights [transformerLanes]int8
	var psqtWeights [psqtBuckets]int32
	var seedValues [transformerLanes]int16
	var seedPSQT [psqtBuckets]int32
	for lane := range seedValues {
		seedValues[lane] = int16(uint16(lane*2909 + 61723))
		baseWeights[lane] = int16(uint16(lane*4051 + 51197))
		threatWeights[lane] = int8(uint8(lane*73 + 191))
	}
	for bucket := range seedPSQT {
		seedPSQT[bucket] = int32(uint32(bucket)*0x61234567 + 0x7f654321)
		psqtWeights[bucket] = int32(uint32(bucket)*0x71234569 + 0x6f123457)
	}

	for _, add := range []bool{false, true} {
		t.Run(map[bool]string{false: "subtract", true: "add"}[add], func(t *testing.T) {
			wantValues, gotValues := seedValues, seedValues
			wantPSQT, gotPSQT := seedPSQT, seedPSQT
			updateBaseRowPortable(&wantValues, &baseWeights[0], &wantPSQT, &psqtWeights[0], add)
			updateBaseRow(&gotValues, &baseWeights[0], &gotPSQT, &psqtWeights[0], add)
			if gotValues != wantValues || gotPSQT != wantPSQT {
				t.Fatal("selected base row update differs from portable")
			}

			wantValues, gotValues = seedValues, seedValues
			wantPSQT, gotPSQT = seedPSQT, seedPSQT
			updateThreatRowPortable(&wantValues, &threatWeights[0], &wantPSQT, &psqtWeights[0], add)
			updateThreatRow(&gotValues, &threatWeights[0], &gotPSQT, &psqtWeights[0], add)
			if gotValues != wantValues || gotPSQT != wantPSQT {
				t.Fatal("selected threat row update differs from portable")
			}
		})
	}
}
