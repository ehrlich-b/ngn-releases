package sf18big

func transformInputsPortable(
	output *[transformerLanes]uint8,
	stmBase, stmThreats, ntmBase, ntmThreats *[transformerLanes]int16,
) {
	bases := [2]*[transformerLanes]int16{stmBase, ntmBase}
	threats := [2]*[transformerLanes]int16{stmThreats, ntmThreats}
	for half := range bases {
		for lane := 0; lane < transformerLanes/2; lane++ {
			a := clampInt16(wrapAdd16(bases[half][lane], threats[half][lane]), 0, 255)
			b := clampInt16(
				wrapAdd16(
					bases[half][lane+transformerLanes/2],
					threats[half][lane+transformerLanes/2],
				),
				0,
				255,
			)
			output[half*transformerLanes/2+lane] = uint8((uint32(a) * uint32(b)) / 512)
		}
	}
}
