package engine

import (
	"fmt"
	"math/bits"

	"github.com/ehrlich-b/ngn/countereval"
)

// counterBoardFromPosition uses the engine's maintained per-piece bitboards.
// It performs no FEN conversion or mailbox scan on search transitions.
func counterBoardFromPosition(pos *Position) (countereval.Board, error) {
	if pos == nil {
		return countereval.Board{}, fmt.Errorf("%w: nil Counter position", errWorkerEvaluator)
	}
	var board countereval.Board
	for piece := WhitePawn; piece <= BlackKing; piece++ {
		board[piece-WhitePawn] = pos.Board.GetBitboardOf(piece)
	}
	return board, nil
}

// counterMoveDelta decodes Counter feature updates directly from the engine's
// packed move and admitted pre-move position. It preserves the semantic checks
// needed by the checked post-board push without constructing the richer NGN-v1
// feature/facts delta.
func counterMoveDelta(pos *Position, move Move) (countereval.MoveDelta, error) {
	if pos == nil {
		return countereval.MoveDelta{}, fmt.Errorf("%w: nil Counter position", errWorkerEvaluator)
	}
	moving := move.MovingPiece()
	movingPlane, err := counterPlaneFromPiece(moving)
	if err != nil {
		return countereval.MoveDelta{}, err
	}
	if moving.Color() != pos.Turn() {
		return countereval.MoveDelta{}, fmt.Errorf("%w: Counter mover colour does not match side to move", errWorkerEvaluator)
	}
	from := move.Source()
	to := move.Destination()
	if from == to {
		return countereval.MoveDelta{}, fmt.Errorf("%w: Counter mover source equals destination", errWorkerEvaluator)
	}
	if got := pos.Board.PieceAt(from); got != moving {
		return countereval.MoveDelta{}, fmt.Errorf("%w: Counter source has %d, move encodes %d", errWorkerEvaluator, got, moving)
	}

	result := countereval.MoveDelta{
		MovingPlane: uint8(movingPlane),
		From:        uint8(from),
		To:          uint8(to),
	}
	promotion := move.PromoType()
	captured := move.CapturedPiece()
	switch {
	case move.IsCastle():
		if promotion != NoType || move.IsCapture() || move.IsEnPassant() || captured != NoPiece || moving.Type() != King {
			return countereval.MoveDelta{}, fmt.Errorf("%w: malformed Counter castle move", errWorkerEvaluator)
		}
		rookFrom, rookTo, ok := castleRookTransition(from, to)
		if !ok {
			return countereval.MoveDelta{}, fmt.Errorf("%w: invalid Counter castle squares %s%s", errWorkerEvaluator, from, to)
		}
		rook := GetPiece(Rook, moving.Color())
		if pos.Board.PieceAt(to) != NoPiece ||
			pos.Board.PieceAt(rookFrom) != rook ||
			pos.Board.PieceAt(rookTo) != NoPiece {
			return countereval.MoveDelta{}, fmt.Errorf("%w: Counter castle board state mismatch", errWorkerEvaluator)
		}
		result.HasCastleRook = true
		result.CastleRookFrom = uint8(rookFrom)
		result.CastleRookTo = uint8(rookTo)

	case move.IsEnPassant():
		if captured < WhitePawn || captured > BlackKing ||
			promotion != NoType || moving.Type() != Pawn || captured.Type() != Pawn || captured.Color() == moving.Color() ||
			pos.Board.PieceAt(to) != NoPiece {
			return countereval.MoveDelta{}, fmt.Errorf("%w: malformed Counter en-passant move", errWorkerEvaluator)
		}
		captureSquare := to ^ 8
		if pos.Board.PieceAt(captureSquare) != captured {
			return countereval.MoveDelta{}, fmt.Errorf("%w: Counter en-passant victim mismatch", errWorkerEvaluator)
		}
		capturedPlane, err := counterPlaneFromPiece(captured)
		if err != nil {
			return countereval.MoveDelta{}, err
		}
		result.HasCapture = true
		result.CapturedPlane = uint8(capturedPlane)
		result.CaptureSquare = uint8(captureSquare)

	case promotion != NoType:
		if moving.Type() != Pawn || promotion < Knight || promotion > Queen {
			return countereval.MoveDelta{}, fmt.Errorf("%w: malformed Counter promotion move", errWorkerEvaluator)
		}
		promoted := GetPiece(promotion, moving.Color())
		promotionPlane, err := counterPlaneFromPiece(promoted)
		if err != nil {
			return countereval.MoveDelta{}, err
		}
		result.HasPromotion = true
		result.PromotionPlane = uint8(promotionPlane)
		if move.IsCapture() {
			if captured < WhitePawn || captured > BlackKing || captured.Color() == moving.Color() ||
				pos.Board.PieceAt(to) != captured {
				return countereval.MoveDelta{}, fmt.Errorf("%w: Counter promotion capture victim mismatch", errWorkerEvaluator)
			}
			capturedPlane, err := counterPlaneFromPiece(captured)
			if err != nil {
				return countereval.MoveDelta{}, err
			}
			result.HasCapture = true
			result.CapturedPlane = uint8(capturedPlane)
			result.CaptureSquare = uint8(to)
		} else if captured != NoPiece || pos.Board.PieceAt(to) != NoPiece {
			return countereval.MoveDelta{}, fmt.Errorf("%w: Counter promotion destination is occupied", errWorkerEvaluator)
		}

	case move.IsCapture():
		if captured < WhitePawn || captured > BlackKing || captured.Color() == moving.Color() ||
			pos.Board.PieceAt(to) != captured {
			return countereval.MoveDelta{}, fmt.Errorf("%w: Counter capture victim mismatch", errWorkerEvaluator)
		}
		capturedPlane, err := counterPlaneFromPiece(captured)
		if err != nil {
			return countereval.MoveDelta{}, err
		}
		result.HasCapture = true
		result.CapturedPlane = uint8(capturedPlane)
		result.CaptureSquare = uint8(to)

	default:
		if captured != NoPiece || pos.Board.PieceAt(to) != NoPiece {
			return countereval.MoveDelta{}, fmt.Errorf("%w: Counter quiet destination is occupied", errWorkerEvaluator)
		}
	}
	return result, nil
}

func counterPlaneFromPiece(piece Piece) (int, error) {
	if piece < WhitePawn || piece > BlackKing {
		return 0, fmt.Errorf("%w: invalid Counter piece %d", errWorkerEvaluator, piece)
	}
	return int(piece - WhitePawn), nil
}

func validateCounterMoveAfter(pos *Position, delta countereval.MoveDelta) (countereval.Board, error) {
	if pos == nil {
		return countereval.Board{}, fmt.Errorf("%w: nil Counter post-move position", errWorkerEvaluator)
	}
	movingWhite := delta.MovingPlane < 6
	if (pos.Turn() == White) == movingWhite {
		return countereval.Board{}, fmt.Errorf("%w: Counter move did not toggle side", errWorkerEvaluator)
	}
	return counterBoardFromPosition(pos)
}

func counterNonPawnMaterial(pos *Position) int64 {
	minor := pos.Board.GetBitboardOf(WhiteKnight) | pos.Board.GetBitboardOf(WhiteBishop) |
		pos.Board.GetBitboardOf(BlackKnight) | pos.Board.GetBitboardOf(BlackBishop)
	rooks := pos.Board.GetBitboardOf(WhiteRook) | pos.Board.GetBitboardOf(BlackRook)
	queens := pos.Board.GetBitboardOf(WhiteQueen) | pos.Board.GetBitboardOf(BlackQueen)
	return 4*int64(bits.OnesCount64(minor)) + 6*int64(bits.OnesCount64(rooks)) +
		12*int64(bits.OnesCount64(queens))
}
