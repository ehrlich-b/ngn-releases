/*
https://github.com/amanjpro/zahak/?tab=MIT-1-ov-file#readme
MIT License

Copyright (c) 2021 Amanj Sherwany

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package engine

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

type Color int8

const (
	NoColor Color = iota - 1
	Black
	White
)

func (c Color) Other() Color {
	if c == White {
		return Black
	} else if c == Black {
		return White
	}
	return NoColor
}

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
	}
	return " "
}

// Hot-path lookup tables replacing the Type/seeWeight/Weight/Color switch chains —
// these are the innermost ops of SEE, MVV/LVA ordering, delta pruning and the
// make-legality test. Indexed by int(p)&15 so any out-of-range Piece value maps to
// the same defaults the old switches returned (NoType / 0 / Black) instead of
// panicking. Layout: NoPiece=0, White Pawn..King = 1..6, Black Pawn..King = 7..12.
var pieceTypeTable = [16]PieceType{
	NoType, Pawn, Knight, Bishop, Rook, Queen, King, Pawn, Knight, Bishop, Rook, Queen, King, NoType, NoType, NoType,
}
var pieceSeeWeightTable = [16]int16{
	0, 100, 320, 330, 525, 1000, MAX_INT, 100, 320, 330, 525, 1000, MAX_INT, 0, 0, 0,
}
var pieceWeightTable = [16]int16{
	0, 100, 320, 330, 525, 1000, MAX_INT, 100, 320, 330, 525, 1000, MAX_INT, 0, 0, 0,
}
var pieceColorTable = [16]Color{
	NoColor, White, White, White, White, White, White, Black, Black, Black, Black, Black, Black, Black, Black, Black,
}

func (p Piece) Type() PieceType {
	return pieceTypeTable[int(p)&15]
}

const MAX_INT = int16(32767)

// seeWeight uses the same values as Weight() for consistency between SEE and evaluation.
func (p Piece) seeWeight() int16 {
	return pieceSeeWeightTable[int(p)&15]
}

func (p Piece) Weight() int16 {
	return pieceWeightTable[int(p)&15]
}

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
	}
	return " "
}

func pieceFromName(name rune) Piece {
	switch name {
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
	}
	return NoPiece
}

func (p Piece) Color() Color {
	return pieceColorTable[int(p)&15]
}

func GetPiece(pieceType PieceType, color Color) Piece {
	// Pieces are laid out White (Pawn..King = 1..6) then Black (Pawn..King = 7..12),
	// mirroring PieceType (Pawn..King = 1..6): a white piece is Piece(pieceType) and a
	// black piece is that plus the 6 white pieces. Replaces a 12-case double switch
	// (hot in attack/check detection). NoType or NoColor map to NoPiece as before.
	if pieceType == NoType {
		return NoPiece
	}
	if color == White {
		return Piece(pieceType)
	}
	if color == Black {
		return Piece(pieceType) + 6
	}
	return NoPiece
}
