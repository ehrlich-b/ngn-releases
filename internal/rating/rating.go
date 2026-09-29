// Package rating converts game scores to Elo and confidence intervals — the math
// shared by the self-play SPRT, the smoke rolling estimate, and the CCRL gauntlet.
// It was consolidated from three byte-identical copies that had drifted in
// signature (int vs float n; absolute vs relative); this is the single source.
package rating

import "math"

// ScoreToEloDiff converts a score fraction p in (0,1) to an Elo difference vs the
// opponent: 400*log10(p/(1-p)), clamped to ±800 at the extremes (eps 1e-4).
func ScoreToEloDiff(p float64) float64 {
	const eps = 1e-4
	if p <= eps {
		return -800
	}
	if p >= 1-eps {
		return 800
	}
	return -400 * math.Log10(1/p-1)
}

// WilsonCI returns the 95% Wilson score interval (lo, hi) on the score fraction,
// where score = wins + 0.5*draws over n games. n may be fractional (weighted /
// effective sample sizes). Bounds are clamped to [0,1].
func WilsonCI(score, n float64) (lo, hi float64) {
	if n <= 0 {
		return 0, 1
	}
	p := score / n
	const z = 1.96
	denom := 1 + z*z/n
	center := (p + z*z/(2*n)) / denom
	margin := z * math.Sqrt(p*(1-p)/n+z*z/(4*n*n)) / denom
	lo, hi = center-margin, center+margin
	if lo < 0 {
		lo = 0
	}
	if hi > 1 {
		hi = 1
	}
	return lo, hi
}

// PerfRating returns the absolute performance rating (point + 95% CI) of a player
// that scored `score` over `n` games against opponents rated anchorELO. With
// anchorELO=0 it is the relative Elo vs the opponent.
func PerfRating(score, n, anchorELO float64) (est, lo, hi float64) {
	if n <= 0 {
		return anchorELO, anchorELO - 800, anchorELO + 800
	}
	loP, hiP := WilsonCI(score, n)
	return anchorELO + ScoreToEloDiff(score/n),
		anchorELO + ScoreToEloDiff(loP),
		anchorELO + ScoreToEloDiff(hiP)
}

// Score is wins + 0.5*draws.
func Score(w, d int) float64 { return float64(w) + 0.5*float64(d) }

// RelativeElo returns the Elo of a player relative to its opponent from a W-D-L
// record, with a 95% CI (the new-minus-base estimate the SPRT reports).
func RelativeElo(w, d, l int) (est, lo, hi float64) {
	return PerfRating(Score(w, d), float64(w+d+l), 0)
}
