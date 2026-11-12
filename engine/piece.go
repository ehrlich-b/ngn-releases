package engine

// Piece identifies a chess piece value.
type Piece int8

const (
	NoPiece Piece = iota
	WhitePawn
	WhiteKnight
	WhiteBishop
	WhiteRook
	WhiteQueen
	WhiteKing
	BlackPawn
	BlackKnight
	BlackBishop
	BlackRook
	BlackQueen
	BlackKing
)

// Pieces contains the nonempty piece values in numeric order.
var Pieces = []Piece{
	WhitePawn,
	WhiteKnight,
	WhiteBishop,
	WhiteRook,
	WhiteQueen,
	WhiteKing,
	BlackPawn,
	BlackKnight,
	BlackBishop,
	BlackRook,
	BlackQueen,
	BlackKing,
}

// PieceType identifies a piece kind without its color.
type PieceType int8

const (
	NoType PieceType = iota
	Pawn
	Knight
	Bishop
	Rook
	Queen
	King
)

// Color identifies a side.
type Color int8

const (
	NoColor Color = -1
	Black   Color = 0
	White   Color = 1
)

// KingExchangeValue is the maximum signed int16 value used as the king weight.
const KingExchangeValue int16 = 32767

// Other returns the opposite supported color.
func (c Color) Other() Color {
	switch c {
	case Black:
		return White
	case White:
		return Black
	default:
		return NoColor
	}
}

// Name returns the uppercase name of a piece type.
func (t PieceType) Name() string {
	switch t {
	case Pawn:
		return "P"
	case Knight:
		return "N"
	case Bishop:
		return "B"
	case Rook:
		return "R"
	case Queen:
		return "Q"
	case King:
		return "K"
	default:
		return " "
	}
}

func (p Piece) lowNibble() uint8 {
	return uint8(p) & 0x0f
}

// Type returns the type encoded by the low nibble of p.
func (p Piece) Type() PieceType {
	n := p.lowNibble()
	switch {
	case n >= 1 && n <= 6:
		return PieceType(n)
	case n >= 7 && n <= 12:
		return PieceType(n - 6)
	default:
		return NoType
	}
}

// Color returns the color encoded by the low nibble of p.
func (p Piece) Color() Color {
	n := p.lowNibble()
	switch {
	case n == 0:
		return NoColor
	case n <= 6:
		return White
	default:
		return Black
	}
}

func (p Piece) weight() int16 {
	switch p.Type() {
	case Pawn:
		return 100
	case Knight:
		return 300
	case Bishop:
		return 340
	case Rook:
		return 500
	case Queen:
		return 950
	case King:
		return KingExchangeValue
	default:
		return 0
	}
}

// Weight returns the material weight encoded by p.
func (p Piece) Weight() int16 {
	return p.weight()
}

func (p Piece) seeWeight() int16 {
	return p.weight()
}

// Name returns the colored character for an exact piece value.
func (p Piece) Name() string {
	switch p {
	case WhitePawn:
		return "P"
	case WhiteKnight:
		return "N"
	case WhiteBishop:
		return "B"
	case WhiteRook:
		return "R"
	case WhiteQueen:
		return "Q"
	case WhiteKing:
		return "K"
	case BlackPawn:
		return "p"
	case BlackKnight:
		return "n"
	case BlackBishop:
		return "b"
	case BlackRook:
		return "r"
	case BlackQueen:
		return "q"
	case BlackKing:
		return "k"
	default:
		return " "
	}
}

func parsePieceGlyph(r rune) Piece {
	switch r {
	case 'P':
		return WhitePawn
	case 'N':
		return WhiteKnight
	case 'B':
		return WhiteBishop
	case 'R':
		return WhiteRook
	case 'Q':
		return WhiteQueen
	case 'K':
		return WhiteKing
	case 'p':
		return BlackPawn
	case 'n':
		return BlackKnight
	case 'b':
		return BlackBishop
	case 'r':
		return BlackRook
	case 'q':
		return BlackQueen
	case 'k':
		return BlackKing
	default:
		return NoPiece
	}
}

// GetPiece combines a type and color into a piece value.
func GetPiece(pieceType PieceType, color Color) Piece {
	if pieceType == NoType {
		return NoPiece
	}
	switch color {
	case White:
		return Piece(pieceType)
	case Black:
		return Piece(pieceType + 6)
	default:
		return NoPiece
	}
}
