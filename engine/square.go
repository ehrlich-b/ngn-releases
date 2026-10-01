package engine

import "strconv"

// Rank identifies a chessboard rank.
type Rank int8

const (
	Rank1 Rank = iota
	Rank2
	Rank3
	Rank4
	Rank5
	Rank6
	Rank7
	Rank8
)

// File identifies a chessboard file.
type File int8

const (
	FileA File = iota
	FileB
	FileC
	FileD
	FileE
	FileF
	FileG
	FileH
)

// Ranks contains the ranks in ascending order.
var Ranks = []Rank{Rank1, Rank2, Rank3, Rank4, Rank5, Rank6, Rank7, Rank8}

// Files contains the files in ascending order.
var Files = []File{FileA, FileB, FileC, FileD, FileE, FileF, FileG, FileH}

// Square identifies a square by its zero-based rank-major index.
type Square int8

const NoSquare Square = -1

const (
	A1 Square = iota
	B1
	C1
	D1
	E1
	F1
	G1
	H1
	A2
	B2
	C2
	D2
	E2
	F2
	G2
	H2
	A3
	B3
	C3
	D3
	E3
	F3
	G3
	H3
	A4
	B4
	C4
	D4
	E4
	F4
	G4
	H4
	A5
	B5
	C5
	D5
	E5
	F5
	G5
	H5
	A6
	B6
	C6
	D6
	E6
	F6
	G6
	H6
	A7
	B7
	C7
	D7
	E7
	F7
	G7
	H7
	A8
	B8
	C8
	D8
	E8
	F8
	G8
	H8
)

// Name returns the byte-wrapped file character.
func (f File) Name() string {
	return string(byte('a') + byte(f))
}

// Name returns the one-based rank number.
func (r Rank) Name() int {
	return int(r) + 1
}

var _ interface{ Name() int } = Rank(0)

// File returns the low three bits of s.
func (s Square) File() File {
	return File(uint8(s) & 0x07)
}

// Rank returns s divided by eight with int8 truncation semantics.
func (s Square) Rank() Rank {
	return Rank(s / 8)
}

// SquareOf constructs a rank-major square index.
func SquareOf(file File, rank Rank) Square {
	return Square(int8(rank)*8 + int8(file))
}

// Name returns the coordinate name of s.
func (s Square) Name() string {
	return s.File().Name() + strconv.Itoa(s.Rank().Name())
}

// String returns the coordinate name of s.
func (s Square) String() string {
	return s.Name()
}

// DarkSquares is the bitset of dark squares.
const DarkSquares uint64 = 0xAA55AA55AA55AA55

// GetColor returns the board color of s.
func (s Square) GetColor() Color {
	if s < 0 || s > 63 {
		return White
	}
	if DarkSquares&(uint64(1)<<uint8(s)) != 0 {
		return Black
	}
	return White
}

func makeNameToSquareMap() map[string]Square {
	result := make(map[string]Square, 64)
	for rank := Rank1; rank <= Rank8; rank++ {
		for file := FileA; file <= FileH; file++ {
			square := SquareOf(file, rank)
			result[square.Name()] = square
		}
	}
	return result
}

// NameToSquareMap maps lowercase coordinate names to squares.
var NameToSquareMap = makeNameToSquareMap()
