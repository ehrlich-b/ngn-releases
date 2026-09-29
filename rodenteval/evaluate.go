package rodenteval

import (
	"fmt"
	"math/bits"
)

type accumulator [2][HiddenSize]int16

func (model *Model) validate() error {
	if model == nil || !model.validated || model.metadata.SHA256 != V11AnandSHA256 ||
		model.metadata.Bytes != fileSize {
		return fmt.Errorf("rodent v1.1 Anand model: model is not strict-loader validated")
	}
	return nil
}

func validatePosition(position Position) error {
	if position.SideToMove != White && position.SideToMove != Black {
		return fmt.Errorf("rodent v1.1 Anand position: invalid side to move %d", position.SideToMove)
	}
	var occupied uint64
	for plane, pieces := range position.Board {
		if occupied&pieces != 0 {
			return fmt.Errorf("rodent v1.1 Anand position: overlapping piece plane %d", plane)
		}
		occupied |= pieces
	}
	if count := bits.OnesCount64(position.Board[WhiteKing]); count != 1 {
		return fmt.Errorf("rodent v1.1 Anand position: white king count %d, want 1", count)
	}
	if count := bits.OnesCount64(position.Board[BlackKing]); count != 1 {
		return fmt.Errorf("rodent v1.1 Anand position: black king count %d, want 1", count)
	}
	return nil
}

func featureIndex(color Color, pieceType, square, kingSquare int, perspective Color) int {
	orientedSquare := square
	if perspective == Black {
		orientedSquare ^= 56
	}
	if kingSquare%8 > 3 {
		orientedSquare ^= 7
	}
	return int(color^perspective)*384 + pieceType*64 + orientedSquare
}

func (model *Model) fullRefresh(board Board) accumulator {
	var result accumulator
	copy(result[White][:], model.inputBiases[:])
	copy(result[Black][:], model.inputBiases[:])
	whiteKingSquare := bits.TrailingZeros64(board[WhiteKing])
	blackKingSquare := bits.TrailingZeros64(board[BlackKing])

	for plane, pieceBits := range board {
		color := Color(plane / 6)
		pieceType := plane % 6
		for pieceBits != 0 {
			square := bits.TrailingZeros64(pieceBits)
			pieceBits &= pieceBits - 1
			whiteRow := &model.inputWeights[featureIndex(color, pieceType, square, whiteKingSquare, White)]
			blackRow := &model.inputWeights[featureIndex(color, pieceType, square, blackKingSquare, Black)]
			for hidden := 0; hidden < HiddenSize; hidden++ {
				result[White][hidden] += whiteRow[hidden]
				result[Black][hidden] += blackRow[hidden]
			}
		}
	}
	return result
}

func clippedSquaredWeighted(value, weight int16) int32 {
	v := int32(value)
	if v < 0 {
		v = 0
	} else if v > inputScale {
		v = inputScale
	}
	return v * v * int32(weight)
}

func (model *Model) evaluateAccumulator(accumulator *accumulator, sideToMove Color) int {
	sum := rodentOutputDot(
		&accumulator[sideToMove],
		&accumulator[sideToMove^1],
		&model.outputWeights[0],
		&model.outputWeights[1],
	)
	sum = sum/inputScale + int32(model.outputBias)
	return int(sum * outputScale / (inputScale * layerScale))
}

// EvaluateRaw returns the exact Anand full-refresh score before Rodent's
// release-specific material factor.
func (model *Model) EvaluateRaw(position Position) (int, error) {
	if err := model.validate(); err != nil {
		return 0, err
	}
	if err := validatePosition(position); err != nil {
		return 0, err
	}
	accumulator := model.fullRefresh(position.Board)
	return model.evaluateAccumulator(&accumulator, position.SideToMove), nil
}

func releaseMaterial(board Board) int64 {
	count := func(plane int) int64 { return int64(bits.OnesCount64(board[plane])) }
	return 100*(count(WhitePawn)+count(BlackPawn)) +
		300*(count(WhiteKnight)+count(BlackKnight)) +
		300*count(WhiteBishop) +
		300*count(WhiteRook) +
		500*(count(WhiteRook)+count(BlackRook)) +
		900*(count(WhiteQueen)+count(BlackQueen))
}

func scaleReleaseStatic(raw int, board Board) int {
	return int(int64(raw) * (25000 + releaseMaterial(board)) / 32768)
}

// EvaluateReleaseStatic returns the exact pure-Anand V1.1 static score,
// including the asymmetric material-count formula present in the testers
// release. It intentionally performs no pre-100-halfmove rule-50 damping.
func (model *Model) EvaluateReleaseStatic(position Position) (int, error) {
	raw, err := model.EvaluateRaw(position)
	if err != nil {
		return 0, err
	}
	return scaleReleaseStatic(raw, position.Board), nil
}
