package engine

// wave2ScoreTime tracks only completed root scores, in the root side's cp units.
// Configuration is worker-private and copied with TimeManager for a UCI session.
// The zero value keeps the experiment off. These constants are not SPSA knobs.
type wave2ScoreTime struct {
	enabled       bool
	previousScore int
	previousKnown bool
	loss          int
}

const (
	scoreTimeScale = 200.0
	scoreTimeMax   = 0.25
)

func (trend *wave2ScoreTime) reset() {
	*trend = wave2ScoreTime{enabled: trend.enabled}
}

func (trend *wave2ScoreTime) report(scores []int) {
	trend.loss = 0
	// Omitted/unknown scores and either sign of mate break the cp sequence:
	// the next ordinary score is again a neutral first observation.
	if len(scores) != 1 || scores[0] <= -MATE_IN_MAX || scores[0] >= MATE_IN_MAX {
		trend.previousScore = 0
		trend.previousKnown = false
		return
	}
	score := scores[0]
	if trend.previousKnown && score < trend.previousScore {
		trend.loss = trend.previousScore - score
	}
	trend.previousScore = score
	trend.previousKnown = true
}

func (trend *wave2ScoreTime) factor() float64 {
	if !trend.enabled {
		return 1
	}
	extra := float64(trend.loss) / scoreTimeScale
	if extra > scoreTimeMax {
		extra = scoreTimeMax
	}
	return 1 + extra
}
