package nnue

import "fmt"

// Evaluate performs a complete feature refresh and returns raw side-to-move
// centipawns. Every arithmetic step uses int64 as the portable reference.
func (m *Model) Evaluate(position Position) (int64, error) {
	if m == nil {
		return 0, fmt.Errorf("%w: nil model", ErrFormat)
	}
	if !m.loaded {
		return 0, fmt.Errorf("%w: model was not loaded from a validated container", ErrFormat)
	}
	if position.SideToMove > Black {
		return 0, fmt.Errorf("%w: side to move %d", ErrFormat, position.SideToMove)
	}
	if len(position.Pieces) > 64 {
		return 0, fmt.Errorf("%w: %d occupied squares", ErrFormat, len(position.Pieces))
	}

	var accumulators [PerspectiveCount][HiddenSize]int64
	for perspective := 0; perspective < PerspectiveCount; perspective++ {
		for hidden, bias := range m.featureBias {
			accumulators[perspective][hidden] = int64(bias)
		}
	}

	var occupied [64]bool
	for _, piece := range position.Pieces {
		if piece.Square >= 64 {
			return 0, fmt.Errorf("%w: square %d", ErrFormat, piece.Square)
		}
		if occupied[piece.Square] {
			return 0, fmt.Errorf("%w: duplicate square %d", ErrFormat, piece.Square)
		}
		occupied[piece.Square] = true
		for perspective := White; perspective <= Black; perspective++ {
			feature, err := FeatureIndex(piece, perspective)
			if err != nil {
				return 0, err
			}
			for hidden, weight := range m.featureWeights[feature] {
				accumulators[perspective][hidden] += int64(weight)
			}
		}
	}

	return m.evaluateAccumulators(&accumulators, position.SideToMove), nil
}

func (m *Model) evaluateAccumulators(
	accumulators *[PerspectiveCount][HiddenSize]int64,
	sideToMove Color,
) int64 {
	us := int(sideToMove)
	them := us ^ 1
	var dot int64
	for hidden := 0; hidden < HiddenSize; hidden++ {
		dot += squareClipped(accumulators[us][hidden]) * int64(m.outputWeights[hidden])
		dot += squareClipped(accumulators[them][hidden]) * int64(m.outputWeights[HiddenSize+hidden])
	}

	value := dot / QA
	value += int64(m.outputBias)
	return value * ScoreScale / (QA * QB)
}

func squareClipped(value int64) int64 {
	if value <= 0 {
		return 0
	}
	if value >= QA {
		return QA * QA
	}
	return value * value
}
