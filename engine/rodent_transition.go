package engine

import (
	"fmt"

	"github.com/ehrlich-b/ngn/rodenteval"
)

// rodentPositionFromPosition converts the engine's maintained per-piece
// bitboards directly to Rodent's explicit 12-plane input.
func rodentPositionFromPosition(pos *Position) (rodenteval.Position, error) {
	if pos == nil {
		return rodenteval.Position{}, fmt.Errorf("%w: nil Rodent position", errWorkerEvaluator)
	}
	var board rodenteval.Board
	for piece := WhitePawn; piece <= BlackKing; piece++ {
		board[piece-WhitePawn] = pos.Board.GetBitboardOf(piece)
	}
	side := rodenteval.White
	if pos.Turn() == Black {
		side = rodenteval.Black
	}
	return rodenteval.Position{Board: board, SideToMove: side}, nil
}

// rodentMoveDelta decodes the admitted packed move without FEN conversion or
// mailbox scans. Search legality remains the engine's responsibility; these
// checks protect the incremental evaluator transaction.
func rodentMoveDelta(pos *Position, move Move) (rodenteval.MoveDelta, error) {
	if pos == nil {
		return rodenteval.MoveDelta{}, fmt.Errorf("%w: nil Rodent position", errWorkerEvaluator)
	}
	moving := move.MovingPiece()
	movingPlane, err := rodentPlaneFromPiece(moving)
	if err != nil {
		return rodenteval.MoveDelta{}, err
	}
	if moving.Color() != pos.Turn() {
		return rodenteval.MoveDelta{}, fmt.Errorf("%w: Rodent mover colour does not match side to move", errWorkerEvaluator)
	}
	from := move.Source()
	to := move.Destination()
	if from == to {
		return rodenteval.MoveDelta{}, fmt.Errorf("%w: Rodent mover source equals destination", errWorkerEvaluator)
	}
	if got := pos.Board.PieceAt(from); got != moving {
		return rodenteval.MoveDelta{}, fmt.Errorf("%w: Rodent source has %d, move encodes %d", errWorkerEvaluator, got, moving)
	}

	result := rodenteval.MoveDelta{
		MovingPlane: uint8(movingPlane),
		From:        uint8(from),
		To:          uint8(to),
	}
	promotion := move.PromoType()
	captured := move.CapturedPiece()
	switch {
	case move.IsCastle():
		if promotion != NoType || move.IsCapture() || move.IsEnPassant() || captured != NoPiece || moving.Type() != King {
			return rodenteval.MoveDelta{}, fmt.Errorf("%w: malformed Rodent castle move", errWorkerEvaluator)
		}
		rookFrom, rookTo, ok := castleRookTransition(from, to)
		if !ok {
			return rodenteval.MoveDelta{}, fmt.Errorf("%w: invalid Rodent castle squares %s%s", errWorkerEvaluator, from, to)
		}
		rook := GetPiece(Rook, moving.Color())
		if pos.Board.PieceAt(to) != NoPiece || pos.Board.PieceAt(rookFrom) != rook || pos.Board.PieceAt(rookTo) != NoPiece {
			return rodenteval.MoveDelta{}, fmt.Errorf("%w: Rodent castle board state mismatch", errWorkerEvaluator)
		}
		result.HasCastleRook = true
		result.CastleRookFrom = uint8(rookFrom)
		result.CastleRookTo = uint8(rookTo)

	case move.IsEnPassant():
		if captured < WhitePawn || captured > BlackKing || promotion != NoType || moving.Type() != Pawn ||
			captured.Type() != Pawn || captured.Color() == moving.Color() || pos.Board.PieceAt(to) != NoPiece {
			return rodenteval.MoveDelta{}, fmt.Errorf("%w: malformed Rodent en-passant move", errWorkerEvaluator)
		}
		captureSquare := to ^ 8
		if pos.Board.PieceAt(captureSquare) != captured {
			return rodenteval.MoveDelta{}, fmt.Errorf("%w: Rodent en-passant victim mismatch", errWorkerEvaluator)
		}
		capturedPlane, err := rodentPlaneFromPiece(captured)
		if err != nil {
			return rodenteval.MoveDelta{}, err
		}
		result.HasCapture = true
		result.CapturedPlane = uint8(capturedPlane)
		result.CaptureSquare = uint8(captureSquare)

	case promotion != NoType:
		if moving.Type() != Pawn || promotion < Knight || promotion > Queen {
			return rodenteval.MoveDelta{}, fmt.Errorf("%w: malformed Rodent promotion move", errWorkerEvaluator)
		}
		promotionPlane, err := rodentPlaneFromPiece(GetPiece(promotion, moving.Color()))
		if err != nil {
			return rodenteval.MoveDelta{}, err
		}
		result.HasPromotion = true
		result.PromotionPlane = uint8(promotionPlane)
		if move.IsCapture() {
			if captured < WhitePawn || captured > BlackKing || captured.Color() == moving.Color() || pos.Board.PieceAt(to) != captured {
				return rodenteval.MoveDelta{}, fmt.Errorf("%w: Rodent promotion capture victim mismatch", errWorkerEvaluator)
			}
			capturedPlane, err := rodentPlaneFromPiece(captured)
			if err != nil {
				return rodenteval.MoveDelta{}, err
			}
			result.HasCapture = true
			result.CapturedPlane = uint8(capturedPlane)
			result.CaptureSquare = uint8(to)
		} else if captured != NoPiece || pos.Board.PieceAt(to) != NoPiece {
			return rodenteval.MoveDelta{}, fmt.Errorf("%w: Rodent promotion destination is occupied", errWorkerEvaluator)
		}

	case move.IsCapture():
		if captured < WhitePawn || captured > BlackKing || captured.Color() == moving.Color() || pos.Board.PieceAt(to) != captured {
			return rodenteval.MoveDelta{}, fmt.Errorf("%w: Rodent capture victim mismatch", errWorkerEvaluator)
		}
		capturedPlane, err := rodentPlaneFromPiece(captured)
		if err != nil {
			return rodenteval.MoveDelta{}, err
		}
		result.HasCapture = true
		result.CapturedPlane = uint8(capturedPlane)
		result.CaptureSquare = uint8(to)

	default:
		if captured != NoPiece || pos.Board.PieceAt(to) != NoPiece {
			return rodenteval.MoveDelta{}, fmt.Errorf("%w: Rodent quiet destination is occupied", errWorkerEvaluator)
		}
	}
	return result, nil
}

func rodentPlaneFromPiece(piece Piece) (int, error) {
	if piece < WhitePawn || piece > BlackKing {
		return 0, fmt.Errorf("%w: invalid Rodent piece %d", errWorkerEvaluator, piece)
	}
	return int(piece - WhitePawn), nil
}

func validateRodentMoveAfter(pos *Position, delta rodenteval.MoveDelta) (rodenteval.Position, error) {
	if pos == nil {
		return rodenteval.Position{}, fmt.Errorf("%w: nil Rodent post-move position", errWorkerEvaluator)
	}
	movingWhite := delta.MovingPlane < 6
	if (pos.Turn() == White) == movingWhite {
		return rodenteval.Position{}, fmt.Errorf("%w: Rodent move did not toggle side", errWorkerEvaluator)
	}
	return rodentPositionFromPosition(pos)
}
