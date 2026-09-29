package engine

// TexelModelMSE measures the current integer white-perspective evaluator on
// labeled boards. Like fitting, it is offline-only and must not race a search
// or parameter mutation. It refreshes sample accumulators after weight changes.
func TexelModelMSE(samples []TexelSample, k float64) float64 {
	_ = mustAcquireHCEModelUse()
	defer releaseHCEModelUse()
	return texelMSE(samples, k)
}

type TexelCoverage struct {
	Samples     int
	PhaseCounts [25]int
	// Counts are observations with a nonzero signed feature and phase factor;
	// they describe support, not independent games or statistical power.
	AppendedParameterObservations map[string]int
}

func TexelTrainingCoverage(samples []TexelSample) TexelCoverage {
	_ = mustAcquireHCEModelUse()
	defer releaseHCEModelUse()
	r := TexelCoverage{Samples: len(samples), AppendedParameterObservations: make(map[string]int)}
	names := TexelParameterNames()
	for _, name := range names[texelLegacyParams:] {
		r.AppendedParameterObservations[name] = 0
	}
	for i := range samples {
		t := buildEvalTrace(&samples[i].Board)
		r.PhaseCounts[t.Phase]++
		for cell, n := range t.MobilityCounts {
			if n == 0 {
				continue
			}
			if t.Phase > 0 {
				r.AppendedParameterObservations[names[texelMobilityMGBase+cell]]++
			}
			if t.Phase < totalPhase {
				r.AppendedParameterObservations[names[texelMobilityEGBase+cell]]++
			}
		}
		for cell, n := range t.ThreatCounts {
			if n != 0 && t.Phase > 0 {
				r.AppendedParameterObservations[names[texelThreatBase+cell]]++
			}
		}
	}
	return r
}
