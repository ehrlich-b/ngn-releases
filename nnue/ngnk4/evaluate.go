package ngnk4

import (
	"fmt"
	"math/bits"
)

// Accumulators are int16. The loader proves that |bias| plus the 32 largest
// input-row magnitudes of every bucket fits in int16, so for every legal board
// the modular int16 lanes equal the exact integer sums.
type accumulator [2][HiddenSize]int16

func (model *Model) validate() error {
	if model == nil || !model.loaded || model.metadata.ArchitectureID != ArchitectureID ||
		model.metadata.FileBytes != FileSize || model.metadata.PayloadBytes != PayloadSize {
		return fmt.Errorf("%w: model is not strict-loader validated", ErrFormat)
	}
	return nil
}

func validatePosition(position Position) error {
	if position.SideToMove != White && position.SideToMove != Black {
		return fmt.Errorf("NGN K4 position: invalid side to move %d", position.SideToMove)
	}
	var occupied uint64
	for plane, pieces := range position.Board {
		if occupied&pieces != 0 {
			return fmt.Errorf("NGN K4 position: overlapping piece plane %d", plane)
		}
		occupied |= pieces
	}
	if count := bits.OnesCount64(position.Board[WhiteKing]); count != 1 {
		return fmt.Errorf("NGN K4 position: white king count %d, want 1", count)
	}
	if count := bits.OnesCount64(position.Board[BlackKing]); count != 1 {
		return fmt.Errorf("NGN K4 position: black king count %d, want 1", count)
	}
	if count := bits.OnesCount64(occupied); count > 32 {
		return fmt.Errorf("NGN K4 position: piece count %d exceeds legal-board bound 32", count)
	}
	return nil
}

func featureIndex(color Color, pieceType, square, kingSquare int, perspective Color) int {
	orientedSquare := square
	orientedKing := kingSquare
	if perspective == Black {
		orientedSquare ^= 56
		orientedKing ^= 56
	}
	if kingSquare%8 > 3 {
		orientedSquare ^= 7
	}
	bucket := kingBucketTable[orientedKing]
	return bucket*InputSize + int(color^perspective)*384 + pieceType*64 + orientedSquare
}

func (model *Model) fullRefresh(board Board) accumulator {
	var result accumulator
	model.refreshPerspective(&result[White], board, White, bits.TrailingZeros64(board[WhiteKing]))
	model.refreshPerspective(&result[Black], board, Black, bits.TrailingZeros64(board[BlackKing]))
	return result
}

func clippedSquaredWeighted(value, weight int16) int64 {
	v := int64(value)
	if v < 0 {
		v = 0
	} else if v > InputScale {
		v = InputScale
	}
	return v * v * int64(weight)
}

func outputBucket(board Board) int {
	pieceCount := bits.OnesCount64(occupiedBoard(board))
	bucket := (pieceCount - 2) / 4
	if bucket < 0 {
		return 0
	}
	if bucket >= OutputBuckets {
		return OutputBuckets - 1
	}
	return bucket
}

func (model *Model) evaluateAccumulator(accumulator *accumulator, sideToMove Color, bucket int) int64 {
	stm := &accumulator[sideToMove]
	nonSTM := &accumulator[sideToMove^1]
	stmWeights := &model.outputWeights[bucket][0]
	nonSTMWeights := &model.outputWeights[bucket][1]
	if model.metadata.FastOutputSafe[bucket] {
		sum := k4OutputDot(stm, nonSTM, stmWeights, nonSTMWeights)
		q := int64(sum)/InputScale + int64(model.outputBiases[bucket])
		return q * OutputScale / (InputScale * LayerScale)
	}
	var sum int64
	for hidden := 0; hidden < HiddenSize; hidden += 4 {
		sum += clippedSquaredWeighted(stm[hidden], stmWeights[hidden])
		sum += clippedSquaredWeighted(nonSTM[hidden], nonSTMWeights[hidden])
		sum += clippedSquaredWeighted(stm[hidden+1], stmWeights[hidden+1])
		sum += clippedSquaredWeighted(nonSTM[hidden+1], nonSTMWeights[hidden+1])
		sum += clippedSquaredWeighted(stm[hidden+2], stmWeights[hidden+2])
		sum += clippedSquaredWeighted(nonSTM[hidden+2], nonSTMWeights[hidden+2])
		sum += clippedSquaredWeighted(stm[hidden+3], stmWeights[hidden+3])
		sum += clippedSquaredWeighted(nonSTM[hidden+3], nonSTMWeights[hidden+3])
	}
	q := sum/InputScale + int64(model.outputBiases[bucket])
	return q * OutputScale / (InputScale * LayerScale)
}

// EvaluateRaw returns the side-to-move score under the frozen 400*z integer
// contract. Rule-50 attenuation and mate-band reservation belong to the engine.
func (model *Model) EvaluateRaw(position Position) (int64, error) {
	if err := model.validate(); err != nil {
		return 0, err
	}
	if err := validatePosition(position); err != nil {
		return 0, err
	}
	accumulator := model.fullRefresh(position.Board)
	return model.evaluateAccumulator(&accumulator, position.SideToMove, outputBucket(position.Board)), nil
}
