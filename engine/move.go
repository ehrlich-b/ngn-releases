package engine

// Move is a packed chess move.
type Move uint32

const EmptyMove Move = 0

// MoveTag contains packed move flags.
type MoveTag uint8

const (
	KingSideCastle  MoveTag = 1
	QueenSideCastle MoveTag = 2
	Capture         MoveTag = 4
	EnPassant       MoveTag = 8
)

// NewMove packs the supplied move fields.
func NewMove(from Square, to Square, moving Piece, captured Piece, promotion PieceType, tag MoveTag) Move {
	return Move(uint32(from)&0x3f) |
		Move((uint32(to)&0x3f)<<6) |
		Move((uint32(moving)&0x0f)<<12) |
		Move((uint32(captured)&0x0f)<<16) |
		Move((uint32(promotion)&0x07)<<20) |
		Move((uint32(tag)&0xff)<<23)
}

// Source returns the packed source square.
func (m Move) Source() Square {
	return Square(m & 0x3f)
}

// Destination returns the packed destination square.
func (m Move) Destination() Square {
	return Square((m >> 6) & 0x3f)
}

// MovingPiece returns the packed moving piece.
func (m Move) MovingPiece() Piece {
	return Piece((m >> 12) & 0x0f)
}

// CapturedPiece returns the packed captured piece.
func (m Move) CapturedPiece() Piece {
	return Piece((m >> 16) & 0x0f)
}

// PromoType returns the packed promotion type.
func (m Move) PromoType() PieceType {
	return PieceType((m >> 20) & 0x07)
}

// Tag returns the packed flags, excluding bit 31.
func (m Move) Tag() MoveTag {
	return MoveTag(m >> 23)
}

// IsKingSideCastle reports whether the king-side castling flag is set.
func (m Move) IsKingSideCastle() bool {
	return m.Tag()&KingSideCastle != 0
}

// IsQueenSideCastle reports whether the queen-side castling flag is set.
func (m Move) IsQueenSideCastle() bool {
	return m.Tag()&QueenSideCastle != 0
}

// IsCastle reports whether either castling flag is set.
func (m Move) IsCastle() bool {
	return m.IsKingSideCastle() || m.IsQueenSideCastle()
}

// IsCapture reports whether the capture flag is set.
func (m Move) IsCapture() bool {
	return m.Tag()&Capture != 0
}

// IsEnPassant reports whether the en-passant flag is set.
func (m Move) IsEnPassant() bool {
	return m.Tag()&EnPassant != 0
}

func promotionName(t PieceType) string {
	switch t {
	case Pawn:
		return "p"
	case Knight:
		return "n"
	case Bishop:
		return "b"
	case Rook:
		return "r"
	case Queen:
		return "q"
	case King:
		return "k"
	default:
		return " "
	}
}

// ToString returns the coordinate move with an optional promotion suffix.
func (m Move) ToString() string {
	result := m.Source().Name() + m.Destination().Name()
	if promotion := m.PromoType(); promotion != NoType {
		result += promotionName(promotion)
	}
	return result
}
