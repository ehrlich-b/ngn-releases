package countereval

import "fmt"

// EvaluateFullRefresh evaluates an overlap-free board using Counter's portable
// square-major float32 operation order. The result is White-perspective and is
// independent of side to move.
func (model *Model) EvaluateFullRefresh(board Board) (float32, error) {
	_, _, output, err := model.evaluateFullRefreshTrace(board)
	return output, err
}

func (model *Model) evaluateFullRefreshTrace(board Board) ([]int, [HiddenSize]float32, float32, error) {
	if model == nil {
		return nil, [HiddenSize]float32{}, 0, fmt.Errorf("counter evaluator: nil model")
	}

	if err := validateBoard(board); err != nil {
		return nil, [HiddenSize]float32{}, 0, fmt.Errorf("counter evaluator: %w", err)
	}

	features := make([]int, 0, 32)
	for square := 0; square < 64; square++ {
		mask := uint64(1) << square
		for plane := 0; plane < FeaturePlaneCount; plane++ {
			if board[plane]&mask != 0 {
				features = append(features, plane*64+square)
				break
			}
		}
	}

	var accumulator [HiddenSize]float32
	copy(accumulator[:], model.hiddenBiases[:])
	for _, feature := range features {
		base := feature * HiddenSize
		for hidden := 0; hidden < HiddenSize; hidden++ {
			accumulator[hidden] += model.hiddenWeights[base+hidden]
		}
	}

	return features, accumulator, model.evaluateAccumulator(&accumulator), nil
}
