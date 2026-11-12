package engine

// W2CorrMargin widens only reverse-futility margins when learned corrections
// disagree with the raw evaluator. Strength validation is pending; default off.
var W2CorrMargin = 0

func wave2CorrMargin(raw, corrected int) int {
	if W2CorrMargin == 0 || raw == unknownStaticEval || corrected == unknownStaticEval ||
		raw <= -MATE_IN_MAX || raw >= MATE_IN_MAX || corrected <= -MATE_IN_MAX || corrected >= MATE_IN_MAX {
		return 0
	}
	// Both scores are strictly inside the mate range before subtraction, so
	// neither the difference nor its absolute value can overflow an int.
	delta := corrected - raw
	if delta < 0 {
		delta = -delta
	}
	if delta >= 256 {
		return 128
	}
	return delta / 2
}
