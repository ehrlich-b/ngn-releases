//go:build amd64 && amd64.v3

package sf18big

const transformInputsUsesAVX2 = true

var (
	_ [transformerLanes - 1024]byte
	_ [1024 - transformerLanes]byte
)

func transformInputs(
	output *[transformerLanes]uint8,
	stmBase, stmThreats, ntmBase, ntmThreats *[transformerLanes]int16,
) {
	transformInputsAVX2(output, stmBase, stmThreats, ntmBase, ntmThreats)
}

//go:noescape
func transformInputsAVX2(
	output *[transformerLanes]uint8,
	stmBase, stmThreats, ntmBase, ntmThreats *[transformerLanes]int16,
)
