package sf18small

import (
	"errors"
	"fmt"

	base "github.com/ehrlich-b/ngn/nnue"
)

var ErrEvaluation = errors.New("invalid Stockfish 18 SMALL evaluation input")

// Components are the Stockfish network's side-to-move PSQT and positional
// values after each component's separate division by OutputScale. No search
// score policy is applied.
type Components struct {
	PSQT       int32
	Positional int32
}

// Trace contains the reference full-refresh result for all eight layer stacks.
type Trace struct {
	CorrectBucket uint8
	Buckets       [layerStacks]Components
}

type boardPiece struct {
	piece base.PieceType
	color base.Color
	set   bool
}

type accumulatorTrace struct {
	activeCount [2]uint8
	active      [2][32]uint16
	values      [2][transformerLanes]int16
	psqt        [2][psqtBuckets]int32
	wideMin     [2]int64
	wideMax     [2]int64
	psqtWideMin [2]int64
	psqtWideMax [2]int64
}

type stackTrace struct {
	fc0            [16]int32
	squared        [15]uint8
	clipped0       [15]uint8
	fc1            [32]int32
	clipped1       [32]uint8
	fc2            int32
	forward        int32
	positionalRaw  int32
	fc0WideMin     int64
	fc0WideMax     int64
	fc1WideMin     int64
	fc1WideMax     int64
	fc2WideMin     int64
	fc2WideMax     int64
	forwardWide    int64
	positionalWide int64
}

type evaluationTrace struct {
	public             Trace
	accumulator        accumulatorTrace
	transformed        [transformerLanes]uint8
	psqtRaw            [psqtBuckets]int32
	psqtDifferenceWide [psqtBuckets]int64
	stacks             [layerStacks]stackTrace
}

// EvaluateAll performs a scalar full refresh and evaluates every layer stack.
// The model must have been returned successfully by Load. Integer operations
// stored in int16 or int32 use explicit two's-complement wrapping, including
// for adversarial tensors outside the official network's observed range.
func (m *Model) EvaluateAll(position base.Position) (Trace, error) {
	trace, err := m.evaluateAllTrace(position)
	if err != nil {
		return Trace{}, err
	}
	return trace.public, nil
}

func (m *Model) evaluateAllTrace(position base.Position) (evaluationTrace, error) {
	var trace evaluationTrace
	if m == nil || !m.loaded {
		return trace, fmt.Errorf("%w: model was not returned by Load", ErrEvaluation)
	}
	board, kings, occupied, err := validatePosition(position)
	if err != nil {
		return trace, err
	}

	for perspective := base.White; perspective <= base.Black; perspective++ {
		m.refreshPerspective(board, kings[perspective], perspective, &trace.accumulator)
	}
	return m.completeEvaluationTrace(trace, position.SideToMove, occupied), nil
}

func (m *Model) refreshPerspective(board [64]boardPiece, king base.Square, perspective base.Color, accumulator *accumulatorTrace) {
	p := int(perspective)
	copy(accumulator.values[p][:], m.featureBias[:])
	accumulator.wideMin[p] = int64(m.featureBias[0])
	accumulator.wideMax[p] = int64(m.featureBias[0])
	for _, value := range m.featureBias[1:] {
		accumulator.wideMin[p] = min64(accumulator.wideMin[p], int64(value))
		accumulator.wideMax[p] = max64(accumulator.wideMax[p], int64(value))
	}
	for square, entry := range board {
		if !entry.set {
			continue
		}
		index := featureIndex(entry, base.Square(square), king, perspective)
		count := accumulator.activeCount[p]
		accumulator.active[p][count] = uint16(index)
		accumulator.activeCount[p]++
		weightOffset := index * transformerLanes
		for lane := 0; lane < transformerLanes; lane++ {
			before := accumulator.values[p][lane]
			weight := m.featureWeights[weightOffset+lane]
			wide := int64(before) + int64(weight)
			accumulator.wideMin[p] = min64(accumulator.wideMin[p], wide)
			accumulator.wideMax[p] = max64(accumulator.wideMax[p], wide)
			accumulator.values[p][lane] = wrapAdd16(before, weight)
		}
		psqtOffset := index * psqtBuckets
		for bucket := 0; bucket < psqtBuckets; bucket++ {
			before := accumulator.psqt[p][bucket]
			weight := m.psqtWeights[psqtOffset+bucket]
			wide := int64(before) + int64(weight)
			accumulator.psqtWideMin[p] = min64(accumulator.psqtWideMin[p], wide)
			accumulator.psqtWideMax[p] = max64(accumulator.psqtWideMax[p], wide)
			accumulator.psqt[p][bucket] = wrapAdd32(before, weight)
		}
	}
}

func (m *Model) completeEvaluationTrace(trace evaluationTrace, sideToMove base.Color, occupied int) evaluationTrace {
	perspectives := [2]base.Color{sideToMove, sideToMove ^ 1}
	for half, perspective := range perspectives {
		accumulator := &trace.accumulator.values[int(perspective)]
		for lane := 0; lane < transformerLanes/2; lane++ {
			a := clampInt16(accumulator[lane], 0, 254)
			b := clampInt16(accumulator[lane+transformerLanes/2], 0, 254)
			trace.transformed[half*transformerLanes/2+lane] = uint8((uint32(a) * uint32(b)) / 512)
		}
	}

	stm := int(sideToMove)
	ntm := stm ^ 1
	trace.public.CorrectBucket = uint8((occupied - 1) / 4)
	for bucket := 0; bucket < layerStacks; bucket++ {
		trace.psqtDifferenceWide[bucket] = int64(trace.accumulator.psqt[stm][bucket]) - int64(trace.accumulator.psqt[ntm][bucket])
		trace.psqtRaw[bucket] = wrapSub32(trace.accumulator.psqt[stm][bucket], trace.accumulator.psqt[ntm][bucket]) / 2
		trace.stacks[bucket] = m.propagateStack(bucket, &trace.transformed)
		trace.public.Buckets[bucket] = Components{
			PSQT:       trace.psqtRaw[bucket] / 16,
			Positional: trace.stacks[bucket].positionalRaw / 16,
		}
	}
	return trace
}

func validatePosition(position base.Position) ([64]boardPiece, [2]base.Square, int, error) {
	var board [64]boardPiece
	var kings [2]base.Square
	var kingCount [2]uint8
	if position.SideToMove > base.Black {
		return board, kings, 0, fmt.Errorf("%w: side to move %d", ErrEvaluation, position.SideToMove)
	}
	if len(position.Pieces) > 32 {
		return board, kings, 0, fmt.Errorf("%w: %d occupied squares, maximum 32", ErrEvaluation, len(position.Pieces))
	}
	for _, piece := range position.Pieces {
		if piece.Color > base.Black {
			return board, kings, 0, fmt.Errorf("%w: piece colour %d", ErrEvaluation, piece.Color)
		}
		if piece.Piece > base.King {
			return board, kings, 0, fmt.Errorf("%w: piece type %d", ErrEvaluation, piece.Piece)
		}
		if piece.Square >= 64 {
			return board, kings, 0, fmt.Errorf("%w: square %d", ErrEvaluation, piece.Square)
		}
		if board[piece.Square].set {
			return board, kings, 0, fmt.Errorf("%w: duplicate square %d", ErrEvaluation, piece.Square)
		}
		board[piece.Square] = boardPiece{piece: piece.Piece, color: piece.Color, set: true}
		if piece.Piece == base.King {
			kingCount[piece.Color]++
			kings[piece.Color] = piece.Square
		}
	}
	for color := base.White; color <= base.Black; color++ {
		if kingCount[color] != 1 {
			return board, kings, 0, fmt.Errorf("%w: colour %d has %d kings", ErrEvaluation, color, kingCount[color])
		}
	}
	return board, kings, len(position.Pieces), nil
}

var kingBucket = [64]uint8{
	28, 29, 30, 31, 31, 30, 29, 28,
	24, 25, 26, 27, 27, 26, 25, 24,
	20, 21, 22, 23, 23, 22, 21, 20,
	16, 17, 18, 19, 19, 18, 17, 16,
	12, 13, 14, 15, 15, 14, 13, 12,
	8, 9, 10, 11, 11, 10, 9, 8,
	4, 5, 6, 7, 7, 6, 5, 4,
	0, 1, 2, 3, 3, 2, 1, 0,
}

func featureIndex(piece boardPiece, square, king base.Square, perspective base.Color) int {
	flip := base.Square(56 * perspective)
	orient := base.Square(0)
	if king&7 < 4 {
		orient = 7
	}
	plane := 10
	if piece.piece != base.King {
		plane = 2*int(piece.piece) + int(piece.color^perspective)
	}
	return int(square^orient^flip) + plane*64 + int(kingBucket[king^flip])*704
}

func (m *Model) propagateStack(bucket int, input *[transformerLanes]uint8) stackTrace {
	var trace stackTrace
	stack := &m.stacks[bucket]
	trace.fc0WideMin, trace.fc0WideMax = int64(stack.fc0Bias[0]), int64(stack.fc0Bias[0])
	for output := range trace.fc0 {
		trace.fc0[output], trace.fc0WideMin, trace.fc0WideMax = affineRow(
			input[:], stack.fc0Weight[output*transformerLanes:(output+1)*transformerLanes],
			stack.fc0Bias[output], trace.fc0WideMin, trace.fc0WideMax,
		)
	}
	for i := range trace.squared {
		trace.squared[i] = squaredClippedReLU(trace.fc0[i])
		trace.clipped0[i] = clippedReLU(trace.fc0[i])
	}
	var fc1Input [32]uint8
	copy(fc1Input[:15], trace.squared[:])
	copy(fc1Input[15:30], trace.clipped0[:])
	trace.fc1WideMin, trace.fc1WideMax = int64(stack.fc1Bias[0]), int64(stack.fc1Bias[0])
	for output := range trace.fc1 {
		trace.fc1[output], trace.fc1WideMin, trace.fc1WideMax = affineRow(
			fc1Input[:], stack.fc1Weight[output*32:(output+1)*32],
			stack.fc1Bias[output], trace.fc1WideMin, trace.fc1WideMax,
		)
		trace.clipped1[output] = clippedReLU(trace.fc1[output])
	}
	trace.fc2, trace.fc2WideMin, trace.fc2WideMax = affineRow(
		trace.clipped1[:], stack.fc2Weight[:], stack.fc2Bias[0],
		int64(stack.fc2Bias[0]), int64(stack.fc2Bias[0]),
	)
	trace.forwardWide = int64(trace.fc0[15]) * 9600
	trace.forward = wrapMul32(trace.fc0[15], 9600) / 8128
	trace.positionalWide = int64(trace.fc2) + int64(trace.forward)
	trace.positionalRaw = wrapAdd32(trace.fc2, trace.forward)
	return trace
}

func affineRow(input []uint8, weights []int8, bias int32, minimum, maximum int64) (int32, int64, int64) {
	value := bias
	minimum = min64(minimum, int64(bias))
	maximum = max64(maximum, int64(bias))
	for i, inputValue := range input {
		product := int32(inputValue) * int32(weights[i])
		wide := int64(value) + int64(product)
		minimum = min64(minimum, wide)
		maximum = max64(maximum, wide)
		value = wrapAdd32(value, product)
	}
	return value, minimum, maximum
}

func squaredClippedReLU(value int32) uint8 {
	squared := (int64(value) * int64(value)) >> 19
	if squared > 127 {
		return 127
	}
	return uint8(squared)
}

func clippedReLU(value int32) uint8 {
	value >>= 6
	if value < 0 {
		return 0
	}
	if value > 127 {
		return 127
	}
	return uint8(value)
}

func clampInt16(value, minimum, maximum int16) int16 {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}

func wrapAdd16(left, right int16) int16 { return int16(uint16(left) + uint16(right)) }
func wrapAdd32(left, right int32) int32 { return int32(uint32(left) + uint32(right)) }
func wrapSub32(left, right int32) int32 { return int32(uint32(left) - uint32(right)) }
func wrapMul32(left, right int32) int32 { return int32(uint32(left) * uint32(right)) }
func min64(left, right int64) int64 {
	if left < right {
		return left
	}
	return right
}
func max64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
