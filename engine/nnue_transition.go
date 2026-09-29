package engine

import (
	"errors"
	"fmt"
	"math/bits"

	"github.com/ehrlich-b/ngn/nnue"
)

var errNNUETransition = errors.New("invalid engine NNUE transition")

// nnueRootBuffer owns the only slice backing used to refresh one worker's
// context. The returned position is valid until the buffer's next conversion;
// Context.Reset consumes it synchronously and retains no slice.
type nnueRootBuffer struct {
	pieces [64]nnue.PieceOnSquare
}

func (b *nnueRootBuffer) position(pos *Position) (nnue.Position, error) {
	if pos == nil {
		return nnue.Position{}, fmt.Errorf("%w: nil position", errNNUETransition)
	}
	count := 0
	var mailboxPieces [13]uint64
	for square := Square(0); square < 64; square++ {
		piece := pos.Board.PieceAt(square)
		if piece == NoPiece {
			continue
		}
		converted, err := enginePieceForNNUE(piece, square)
		if err != nil {
			return nnue.Position{}, err
		}
		mailboxPieces[piece] |= uint64(1) << square
		b.pieces[count] = converted
		count++
	}
	for piece := WhitePawn; piece <= BlackKing; piece++ {
		if mailboxPieces[piece] != pos.Board.GetBitboardOf(piece) {
			return nnue.Position{}, fmt.Errorf("%w: mailbox and piece bitboard %d disagree", errNNUETransition, piece)
		}
	}
	return nnue.Position{
		SideToMove: engineColorForNNUE(pos.Turn()),
		Pieces:     b.pieces[:count],
	}, nil
}

// nnueMoveDelta derives all feature changes from an engine move and its
// pre-move position. Search must call it before MakeMove and submit the result
// to Context.PushDelta only after the move passes legality checks.
func nnueMoveDelta(pos *Position, move Move) (nnue.Delta, error) {
	if pos == nil {
		return nnue.Delta{}, fmt.Errorf("%w: nil position", errNNUETransition)
	}
	moving := move.MovingPiece()
	if moving < WhitePawn || moving > BlackKing {
		return nnue.Delta{}, fmt.Errorf("%w: invalid moving piece %d", errNNUETransition, moving)
	}
	if moving.Color() != pos.Turn() {
		return nnue.Delta{}, fmt.Errorf("%w: mover colour does not match side to move", errNNUETransition)
	}
	source := move.Source()
	destination := move.Destination()
	if source == destination {
		return nnue.Delta{}, fmt.Errorf("%w: mover source equals destination", errNNUETransition)
	}
	if got := pos.Board.PieceAt(source); got != moving {
		return nnue.Delta{}, fmt.Errorf("%w: source has %d, move encodes %d", errNNUETransition, got, moving)
	}

	before, err := nnuePositionFacts(pos)
	if err != nil {
		return nnue.Delta{}, err
	}
	moverBefore, err := enginePieceForNNUE(moving, source)
	if err != nil {
		return nnue.Delta{}, err
	}
	moverAfter := moverBefore
	moverAfter.Square = nnue.Square(destination)

	var delta nnue.Delta
	delta.Before = before
	delta.Removed[0] = moverBefore
	delta.RemovedCount = 1
	delta.Added[0] = moverAfter
	delta.AddedCount = 1

	promotion := move.PromoType()
	captured := move.CapturedPiece()
	switch {
	case move.IsCastle():
		if promotion != NoType || move.IsCapture() || move.IsEnPassant() || captured != NoPiece || moving.Type() != King {
			return nnue.Delta{}, fmt.Errorf("%w: malformed castle move", errNNUETransition)
		}
		rookSource, rookDestination, ok := castleRookTransition(source, destination)
		if !ok {
			return nnue.Delta{}, fmt.Errorf("%w: invalid castle squares %s%s", errNNUETransition, source, destination)
		}
		rook := GetPiece(Rook, moving.Color())
		if pos.Board.PieceAt(destination) != NoPiece ||
			pos.Board.PieceAt(rookSource) != rook ||
			pos.Board.PieceAt(rookDestination) != NoPiece {
			return nnue.Delta{}, fmt.Errorf("%w: castle board state mismatch", errNNUETransition)
		}
		rookBefore, err := enginePieceForNNUE(rook, rookSource)
		if err != nil {
			return nnue.Delta{}, err
		}
		rookAfter, err := enginePieceForNNUE(rook, rookDestination)
		if err != nil {
			return nnue.Delta{}, err
		}
		delta.Kind = nnue.MoveCastle
		delta.Removed[1] = rookBefore
		delta.RemovedCount = 2
		delta.Added[1] = rookAfter
		delta.AddedCount = 2

	case move.IsEnPassant():
		if captured < WhitePawn || captured > BlackKing ||
			promotion != NoType || moving.Type() != Pawn || captured.Type() != Pawn || captured.Color() == moving.Color() ||
			pos.Board.PieceAt(destination) != NoPiece {
			return nnue.Delta{}, fmt.Errorf("%w: malformed en-passant move", errNNUETransition)
		}
		captureSquare := destination ^ 8
		if pos.Board.PieceAt(captureSquare) != captured {
			return nnue.Delta{}, fmt.Errorf("%w: en-passant victim mismatch", errNNUETransition)
		}
		victim, err := enginePieceForNNUE(captured, captureSquare)
		if err != nil {
			return nnue.Delta{}, err
		}
		delta.Kind = nnue.MoveEnPassant
		delta.Removed[1] = victim
		delta.RemovedCount = 2

	case promotion != NoType:
		if moving.Type() != Pawn || promotion < Knight || promotion > Queen {
			return nnue.Delta{}, fmt.Errorf("%w: malformed promotion move", errNNUETransition)
		}
		promoted := GetPiece(promotion, moving.Color())
		delta.Added[0], err = enginePieceForNNUE(promoted, destination)
		if err != nil {
			return nnue.Delta{}, err
		}
		if move.IsCapture() {
			if captured < WhitePawn || captured > BlackKing || captured.Color() == moving.Color() ||
				pos.Board.PieceAt(destination) != captured {
				return nnue.Delta{}, fmt.Errorf("%w: promotion capture victim mismatch", errNNUETransition)
			}
			victim, err := enginePieceForNNUE(captured, destination)
			if err != nil {
				return nnue.Delta{}, err
			}
			delta.Kind = nnue.MovePromotionCapture
			delta.Removed[1] = victim
			delta.RemovedCount = 2
		} else {
			if captured != NoPiece || pos.Board.PieceAt(destination) != NoPiece {
				return nnue.Delta{}, fmt.Errorf("%w: promotion destination is occupied", errNNUETransition)
			}
			delta.Kind = nnue.MovePromotion
		}

	case move.IsCapture():
		if captured < WhitePawn || captured > BlackKing || captured.Color() == moving.Color() ||
			pos.Board.PieceAt(destination) != captured {
			return nnue.Delta{}, fmt.Errorf("%w: capture victim mismatch", errNNUETransition)
		}
		victim, err := enginePieceForNNUE(captured, destination)
		if err != nil {
			return nnue.Delta{}, err
		}
		delta.Kind = nnue.MoveCapture
		delta.Removed[1] = victim
		delta.RemovedCount = 2

	default:
		if captured != NoPiece || pos.Board.PieceAt(destination) != NoPiece {
			return nnue.Delta{}, fmt.Errorf("%w: quiet destination is occupied", errNNUETransition)
		}
		delta.Kind = nnue.MoveNormal
	}

	delta.After, err = applyNNUETransitionFacts(delta)
	if err != nil {
		return nnue.Delta{}, err
	}
	return delta, nil
}

// validateNNUEMoveAfter bridges the pre-move prediction to the engine's actual
// legal post-move state. It must succeed before Context.PushDelta commits.
func validateNNUEMoveAfter(pos *Position, delta nnue.Delta) error {
	if pos == nil {
		return fmt.Errorf("%w: nil post-move position", errNNUETransition)
	}
	wantSide := delta.Removed[0].Color ^ 1
	if engineColorForNNUE(pos.Turn()) != wantSide {
		return fmt.Errorf("%w: real move did not toggle side", errNNUETransition)
	}
	actual, err := nnuePositionFacts(pos)
	if err != nil {
		return err
	}
	if actual != delta.After {
		return fmt.Errorf("%w: actual post-move facts differ from predicted delta", errNNUETransition)
	}
	for i := 0; i < int(delta.AddedCount); i++ {
		want := delta.Added[i]
		actualPiece := pos.Board.PieceAt(Square(want.Square))
		got, err := enginePieceForNNUE(actualPiece, Square(want.Square))
		if err != nil || got != want {
			return fmt.Errorf("%w: actual added piece %d differs from predicted delta", errNNUETransition, i)
		}
	}
	for i := 0; i < int(delta.RemovedCount); i++ {
		removed := delta.Removed[i]
		readded := false
		for j := 0; j < int(delta.AddedCount); j++ {
			if delta.Added[j].Square == removed.Square {
				readded = true
				break
			}
		}
		if !readded && pos.Board.PieceAt(Square(removed.Square)) != NoPiece {
			return fmt.Errorf("%w: removed square %d is not empty after move", errNNUETransition, removed.Square)
		}
	}
	return nil
}

type nnuePieceBitboards [12]uint64

func nnuePositionPieceBitboards(pos *Position) (nnuePieceBitboards, error) {
	if pos == nil {
		return nnuePieceBitboards{}, fmt.Errorf("%w: nil position", errNNUETransition)
	}
	var snapshot nnuePieceBitboards
	for piece := WhitePawn; piece <= BlackKing; piece++ {
		snapshot[piece-WhitePawn] = pos.Board.GetBitboardOf(piece)
	}
	return snapshot, nil
}

// validateNNUENullAfter verifies both invariants a facts-only context API cannot
// observe: the engine toggled side and every per-piece bitboard stayed unchanged.
func validateNNUENullAfter(
	pos *Position,
	before nnue.PositionFacts,
	beforePieces nnuePieceBitboards,
	beforeSide nnue.Color,
) error {
	if pos == nil {
		return fmt.Errorf("%w: nil post-null position", errNNUETransition)
	}
	if engineColorForNNUE(pos.Turn()) != beforeSide^1 {
		return fmt.Errorf("%w: null move did not toggle side", errNNUETransition)
	}
	actual, err := nnuePositionFacts(pos)
	if err != nil {
		return err
	}
	if actual != before {
		return fmt.Errorf("%w: null move changed board facts", errNNUETransition)
	}
	actualPieces, err := nnuePositionPieceBitboards(pos)
	if err != nil {
		return err
	}
	if actualPieces != beforePieces {
		return fmt.Errorf("%w: null move changed piece bitboards", errNNUETransition)
	}
	return nil
}

func nnuePositionFacts(pos *Position) (nnue.PositionFacts, error) {
	if pos == nil {
		return nnue.PositionFacts{}, fmt.Errorf("%w: nil position", errNNUETransition)
	}
	facts := nnue.PositionFacts{
		Occupied: pos.Board.GetWhitePieces() | pos.Board.GetBlackPieces(),
		Pawns: [nnue.PerspectiveCount]uint64{
			pos.Board.GetBitboardOf(WhitePawn),
			pos.Board.GetBitboardOf(BlackPawn),
		},
		KingSquare: [nnue.PerspectiveCount]nnue.Square{nnue.NoSquare, nnue.NoSquare},
	}
	for _, entry := range [...]struct {
		color nnue.Color
		piece Piece
	}{
		{nnue.White, WhiteKing},
		{nnue.Black, BlackKing},
	} {
		king := pos.Board.GetBitboardOf(entry.piece)
		if bits.OnesCount64(king) > 1 {
			return nnue.PositionFacts{}, fmt.Errorf("%w: multiple %v kings", errNNUETransition, entry.color)
		}
		if king != 0 {
			facts.KingSquare[entry.color] = nnue.Square(bits.TrailingZeros64(king))
		}
	}
	return facts, nil
}

func applyNNUETransitionFacts(delta nnue.Delta) (nnue.PositionFacts, error) {
	facts := delta.Before
	for i := 0; i < int(delta.RemovedCount); i++ {
		piece := delta.Removed[i]
		bit := uint64(1) << piece.Square
		if facts.Occupied&bit == 0 {
			return nnue.PositionFacts{}, fmt.Errorf("%w: removed square %d absent from facts", errNNUETransition, piece.Square)
		}
		facts.Occupied &^= bit
		if piece.Piece == nnue.Pawn {
			facts.Pawns[piece.Color] &^= bit
		}
		if piece.Piece == nnue.King {
			if facts.KingSquare[piece.Color] != piece.Square {
				return nnue.PositionFacts{}, fmt.Errorf("%w: king source disagrees with facts", errNNUETransition)
			}
			facts.KingSquare[piece.Color] = nnue.NoSquare
		}
	}
	for i := 0; i < int(delta.AddedCount); i++ {
		piece := delta.Added[i]
		bit := uint64(1) << piece.Square
		if facts.Occupied&bit != 0 {
			return nnue.PositionFacts{}, fmt.Errorf("%w: added square %d occupied in facts", errNNUETransition, piece.Square)
		}
		facts.Occupied |= bit
		if piece.Piece == nnue.Pawn {
			facts.Pawns[piece.Color] |= bit
		}
		if piece.Piece == nnue.King {
			if facts.KingSquare[piece.Color] != nnue.NoSquare {
				return nnue.PositionFacts{}, fmt.Errorf("%w: added a second king", errNNUETransition)
			}
			facts.KingSquare[piece.Color] = piece.Square
		}
	}
	return facts, nil
}

func enginePieceForNNUE(piece Piece, square Square) (nnue.PieceOnSquare, error) {
	if piece < WhitePawn || piece > BlackKing || square < A1 || square > H8 {
		return nnue.PieceOnSquare{}, fmt.Errorf("%w: piece %d on square %d", errNNUETransition, piece, square)
	}
	return nnue.PieceOnSquare{
		Piece:  nnue.PieceType(piece.Type() - Pawn),
		Color:  engineColorForNNUE(piece.Color()),
		Square: nnue.Square(square),
	}, nil
}

func engineColorForNNUE(color Color) nnue.Color {
	if color == White {
		return nnue.White
	}
	return nnue.Black
}

func castleRookTransition(source, destination Square) (Square, Square, bool) {
	switch {
	case source == E1 && destination == G1:
		return H1, F1, true
	case source == E1 && destination == C1:
		return A1, D1, true
	case source == E8 && destination == G8:
		return H8, F8, true
	case source == E8 && destination == C8:
		return A8, D8, true
	default:
		return NoSquare, NoSquare, false
	}
}
