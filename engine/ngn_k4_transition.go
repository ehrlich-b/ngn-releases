package engine

import (
	"fmt"

	"github.com/ehrlich-b/ngn/nnue/ngnk4"
)

func ngnK4PositionFromPosition(pos *Position) (ngnk4.Position, error) {
	if pos == nil {
		return ngnk4.Position{}, fmt.Errorf("%w: nil NGN K4 position", errWorkerEvaluator)
	}
	var board ngnk4.Board
	for piece := WhitePawn; piece <= BlackKing; piece++ {
		board[piece-WhitePawn] = pos.Board.GetBitboardOf(piece)
	}
	side := ngnk4.White
	if pos.Turn() == Black {
		side = ngnk4.Black
	}
	return ngnk4.Position{Board: board, SideToMove: side}, nil
}

func ngnK4MoveDelta(pos *Position, move Move) (ngnk4.MoveDelta, error) {
	delta, err := rodentMoveDelta(pos, move)
	if err != nil {
		return ngnk4.MoveDelta{}, err
	}
	return ngnk4.MoveDelta{
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

func validateNGNK4MoveAfter(pos *Position, delta ngnk4.MoveDelta) (ngnk4.Position, error) {
	if pos == nil {
		return ngnk4.Position{}, fmt.Errorf("%w: nil NGN K4 post-move position", errWorkerEvaluator)
	}
	movingWhite := delta.MovingPlane < 6
	if (pos.Turn() == White) == movingWhite {
		return ngnk4.Position{}, fmt.Errorf("%w: NGN K4 move did not toggle side", errWorkerEvaluator)
	}
	return ngnK4PositionFromPosition(pos)
}
