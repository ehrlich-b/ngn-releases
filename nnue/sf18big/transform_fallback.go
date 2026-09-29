//go:build !amd64 || !amd64.v3

package sf18big

const transformInputsUsesAVX2 = false

func transformInputs(
	output *[transformerLanes]uint8,
	stmBase, stmThreats, ntmBase, ntmThreats *[transformerLanes]int16,
) {
	transformInputsPortable(output, stmBase, stmThreats, ntmBase, ntmThreats)
}
