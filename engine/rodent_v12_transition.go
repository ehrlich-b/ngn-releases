package engine

import (
	"fmt"

	"github.com/ehrlich-b/ngn/rodentv12eval"
)

func rodentV12PositionFromPosition(pos *Position) (rodentv12eval.Position, error) {
	if pos == nil {
		return rodentv12eval.Position{}, fmt.Errorf("%w: nil Rodent V1.2 position", errWorkerEvaluator)
	}
	var board rodentv12eval.Board
	for piece := WhitePawn; piece <= BlackKing; piece++ {
		board[piece-WhitePawn] = pos.Board.GetBitboardOf(piece)
	}
	side := rodentv12eval.White
	if pos.Turn() == Black {
		side = rodentv12eval.Black
	}
	return rodentv12eval.Position{Board: board, SideToMove: side}, nil
}

// rodentV12MoveDelta reuses the already-admitted packed-move validation shared
// by Rodent's identical 12-plane transition contract, then converts the value
// into the distinct V1.2 package type.
func rodentV12MoveDelta(pos *Position, move Move) (rodentv12eval.MoveDelta, error) {
	delta, err := rodentMoveDelta(pos, move)
	if err != nil {
		return rodentv12eval.MoveDelta{}, err
	}
	return rodentv12eval.MoveDelta{
		MovingPlane:    delta.MovingPlane,
		From:           delta.From,
		To:             delta.To,
		HasCapture:     delta.HasCapture,
		CapturedPlane:  delta.CapturedPlane,
		CaptureSquare:  delta.CaptureSquare,
		HasPromotion:   delta.HasPromotion,
		PromotionPlane: delta.PromotionPlane,
		HasCastleRook:  delta.HasCastleRook,
		CastleRookFrom: delta.CastleRookFrom,
		CastleRookTo:   delta.CastleRookTo,
	}, nil
}

func validateRodentV12MoveAfter(pos *Position, delta rodentv12eval.MoveDelta) (rodentv12eval.Position, error) {
	if pos == nil {
		return rodentv12eval.Position{}, fmt.Errorf("%w: nil Rodent V1.2 post-move position", errWorkerEvaluator)
	}
	movingWhite := delta.MovingPlane < 6
	if (pos.Turn() == White) == movingWhite {
		return rodentv12eval.Position{}, fmt.Errorf("%w: Rodent V1.2 move did not toggle side", errWorkerEvaluator)
	}
	return rodentV12PositionFromPosition(pos)
}
