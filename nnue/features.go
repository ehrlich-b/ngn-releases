package nnue

import "fmt"

// Color is both a board colour and an accumulator perspective.
type Color uint8

const (
	White Color = iota
	Black
)

// PieceType follows Bullet Chess768's zero-based Pawn..King order.
type PieceType uint8

const (
	Pawn PieceType = iota
	Knight
	Bishop
	Rook
	Queen
	King
)

// Square is zero-based A1..H8, with files in the low three bits.
type Square uint8

// PieceOnSquare is one active Chess768 input before perspective mapping.
type PieceOnSquare struct {
	Piece  PieceType
	Color  Color
	Square Square
}

// Position is the compact, engine-independent input to full-refresh inference.
// It does not attempt to validate chess legality beyond one piece per square.
type Position struct {
	SideToMove Color
	Pieces     []PieceOnSquare
}

// FeatureIndex maps a piece into Bullet Chess768 for one perspective. Friendly
// pieces occupy planes 0..5 and opposing pieces planes 6..11. Black's view is
// rank-flipped with square XOR 56; files are not mirrored.
func FeatureIndex(piece PieceOnSquare, perspective Color) (int, error) {
	if perspective > Black {
		return 0, fmt.Errorf("%w: perspective %d", ErrFormat, perspective)
	}
	if piece.Color > Black {
		return 0, fmt.Errorf("%w: piece colour %d", ErrFormat, piece.Color)
	}
	if piece.Piece > King {
		return 0, fmt.Errorf("%w: piece type %d", ErrFormat, piece.Piece)
	}
	if piece.Square >= 64 {
		return 0, fmt.Errorf("%w: square %d", ErrFormat, piece.Square)
	}

	return featureIndexUnchecked(piece, perspective), nil
}

func featureIndexUnchecked(piece PieceOnSquare, perspective Color) int {
	relativeColor := int(piece.Color ^ perspective)
	relativeSquare := int(piece.Square)
	if perspective == Black {
		relativeSquare ^= 56
	}
	return relativeColor*384 + int(piece.Piece)*64 + relativeSquare
}
