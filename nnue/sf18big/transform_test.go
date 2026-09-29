package sf18big

import "testing"

func TestSelectedTransformInputsMatchesPortable(t *testing.T) {
	var stmBase, stmThreats, ntmBase, ntmThreats [transformerLanes]int16
	for lane := 0; lane < transformerLanes; lane++ {
		stmBase[lane] = int16(uint16(lane*2909 + 61723))
		stmThreats[lane] = int16(uint16(lane*4051 + 51197))
		ntmBase[lane] = int16(uint16(lane*7919 + 32749))
		ntmThreats[lane] = int16(uint16(lane*6551 + 12347))
	}
	// Pin the clipping and wrapping boundaries independently of the patterns.
	boundary := [][2]int16{
		{0, 0}, {255, 0}, {256, 0}, {-1, 0},
		{32767, 1}, {-32768, -1}, {127, 128}, {-128, 127},
	}
	for lane, pair := range boundary {
		stmBase[lane], stmThreats[lane] = pair[0], pair[1]
		ntmBase[lane], ntmThreats[lane] = pair[1], pair[0]
	}

	var want, got [transformerLanes]uint8
	transformInputsPortable(&want, &stmBase, &stmThreats, &ntmBase, &ntmThreats)
	transformInputs(&got, &stmBase, &stmThreats, &ntmBase, &ntmThreats)
	if got != want {
		for lane := range got {
			if got[lane] != want[lane] {
				t.Fatalf("selected transform lane %d = %d, want %d", lane, got[lane], want[lane])
			}
		}
		t.Fatal("selected transform differs from portable")
	}
}
