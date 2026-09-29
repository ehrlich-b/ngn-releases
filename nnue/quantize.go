package nnue

import (
	"fmt"
	"math"
)

// FloatTensors is the float32 counterpart of Tensors. Quantize matches Bullet
// 629ee500 crates/trainer/src/model/weights.rs: SavedFormat receives f32 values,
// promotes each value and multiplier to f64, then rounds halfway away from zero.
type FloatTensors struct {
	FeatureWeights [InputSize][HiddenSize]float32
	FeatureBias    [HiddenSize]float32
	OutputWeights  [PerspectiveCount * HiddenSize]float32
	OutputBias     float32
}

// Quantize converts finite floating-point tensors to the canonical i16 form.
// Values that do not fit after scaling and rounding are rejected.
func Quantize(input *FloatTensors) (*Tensors, error) {
	if input == nil {
		return nil, fmt.Errorf("%w: nil tensors", ErrQuantization)
	}
	output := new(Tensors)
	for feature := 0; feature < InputSize; feature++ {
		for hidden := 0; hidden < HiddenSize; hidden++ {
			value, err := quantizeI16(input.FeatureWeights[feature][hidden], QA)
			if err != nil {
				return nil, fmt.Errorf("feature weight [%d][%d]: %w", feature, hidden, err)
			}
			output.FeatureWeights[feature][hidden] = value
		}
	}
	for hidden := 0; hidden < HiddenSize; hidden++ {
		value, err := quantizeI16(input.FeatureBias[hidden], QA)
		if err != nil {
			return nil, fmt.Errorf("feature bias [%d]: %w", hidden, err)
		}
		output.FeatureBias[hidden] = value
	}
	for hidden := 0; hidden < PerspectiveCount*HiddenSize; hidden++ {
		value, err := quantizeI16(input.OutputWeights[hidden], QB)
		if err != nil {
			return nil, fmt.Errorf("output weight [%d]: %w", hidden, err)
		}
		output.OutputWeights[hidden] = value
	}
	value, err := quantizeI16(input.OutputBias, QA*QB)
	if err != nil {
		return nil, fmt.Errorf("output bias: %w", err)
	}
	output.OutputBias = value
	return output, nil
}

func quantizeI16(value float32, scale int) (int16, error) {
	if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
		return 0, fmt.Errorf("%w: non-finite value", ErrQuantization)
	}
	scaled := float64(value) * float64(scale)
	if math.IsNaN(scaled) || math.IsInf(scaled, 0) {
		return 0, fmt.Errorf("%w: non-finite scaled value", ErrQuantization)
	}
	rounded := math.Round(scaled)
	if rounded < math.MinInt16 || rounded > math.MaxInt16 {
		return 0, fmt.Errorf("%w: rounded value %.0f exceeds i16", ErrQuantization, rounded)
	}
	return int16(rounded), nil
}
