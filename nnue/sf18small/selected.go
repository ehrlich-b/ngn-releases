package sf18small

import (
	"fmt"

	base "github.com/ehrlich-b/ngn/nnue"
)

// SelectedEvaluation is the trace-free result for the material-selected SMALL
// layer stack. It preserves the raw Stockfish network components; score policy
// and BIG/SMALL selection remain separate contracts.
type SelectedEvaluation struct {
	Bucket uint8
	Components
}

type selectedAccumulator struct {
	values [2][transformerLanes]int16
	psqt   [2][psqtBuckets]int32
}

// EvaluateSelected performs a scalar full refresh and propagates only the
// material-selected layer stack. EvaluateAll remains the diagnostic oracle.
func (m *Model) EvaluateSelected(position base.Position) (SelectedEvaluation, error) {
	if m == nil || !m.loaded {
		return SelectedEvaluation{}, fmt.Errorf("%w: model was not returned by Load", ErrEvaluation)
	}
	board, kings, occupied, err := validatePosition(position)
	if err != nil {
		return SelectedEvaluation{}, err
	}

	var accumulator selectedAccumulator
	for perspective := base.White; perspective <= base.Black; perspective++ {
		m.refreshSelectedPerspective(board, kings[perspective], perspective, &accumulator)
	}
	return m.evaluateSelectedAccumulator(&accumulator.values, &accumulator.psqt, position.SideToMove, occupied), nil
}

func (m *Model) refreshSelectedPerspective(
	board [64]boardPiece,
	king base.Square,
	perspective base.Color,
	accumulator *selectedAccumulator,
) {
	p := int(perspective)
	copy(accumulator.values[p][:], m.featureBias[:])
	for square, entry := range board {
		if !entry.set {
			continue
		}
		index := featureIndex(entry, base.Square(square), king, perspective)
		weightOffset := index * transformerLanes
		for lane := 0; lane < transformerLanes; lane++ {
			accumulator.values[p][lane] = wrapAdd16(accumulator.values[p][lane], m.featureWeights[weightOffset+lane])
		}
		psqtOffset := index * psqtBuckets
		for bucket := 0; bucket < psqtBuckets; bucket++ {
			accumulator.psqt[p][bucket] = wrapAdd32(accumulator.psqt[p][bucket], m.psqtWeights[psqtOffset+bucket])
		}
	}
}

func (m *Model) evaluateSelectedAccumulator(
	values *[2][transformerLanes]int16,
	psqt *[2][psqtBuckets]int32,
	sideToMove base.Color,
	occupied int,
) SelectedEvaluation {
	bucket := (occupied - 1) / 4
	var transformed [transformerLanes]uint8
	perspectives := [2]base.Color{sideToMove, sideToMove ^ 1}
	for half, perspective := range perspectives {
		accumulator := &values[int(perspective)]
		for lane := 0; lane < transformerLanes/2; lane++ {
			a := clampInt16(accumulator[lane], 0, 254)
			b := clampInt16(accumulator[lane+transformerLanes/2], 0, 254)
			transformed[half*transformerLanes/2+lane] = uint8((uint32(a) * uint32(b)) / 512)
		}
	}

	stm := int(sideToMove)
	ntm := stm ^ 1
	psqtRaw := wrapSub32(psqt[stm][bucket], psqt[ntm][bucket]) / 2
	positionalRaw := m.propagateSelectedStack(bucket, &transformed)
	return SelectedEvaluation{
		Bucket: uint8(bucket),
		Components: Components{
			PSQT:       psqtRaw / 16,
			Positional: positionalRaw / 16,
		},
	}
}

func (m *Model) propagateSelectedStack(bucket int, input *[transformerLanes]uint8) int32 {
	stack := &m.stacks[bucket]
	var fc0 [16]int32
	for output := range fc0 {
		fc0[output] = affineRowValue(
			input[:],
			stack.fc0Weight[output*transformerLanes:(output+1)*transformerLanes],
			stack.fc0Bias[output],
		)
	}

	var fc1Input [32]uint8
	for index := 0; index < 15; index++ {
		fc1Input[index] = squaredClippedReLU(fc0[index])
		fc1Input[index+15] = clippedReLU(fc0[index])
	}
	var clipped1 [32]uint8
	for output := range clipped1 {
		value := affineRowValue(
			fc1Input[:],
			stack.fc1Weight[output*32:(output+1)*32],
			stack.fc1Bias[output],
		)
		clipped1[output] = clippedReLU(value)
	}
	fc2 := affineRowValue(clipped1[:], stack.fc2Weight[:], stack.fc2Bias[0])
	forward := wrapMul32(fc0[15], 9600) / 8128
	return wrapAdd32(fc2, forward)
}

func affineRowValue(input []uint8, weights []int8, bias int32) int32 {
	value := bias
	for index, inputValue := range input {
		value = wrapAdd32(value, int32(inputValue)*int32(weights[index]))
	}
	return value
}
