package engine

import (
	"math/bits"
	"strings"
)

// Bitboard is the board representation used by the position code.  The
// individual piece bitboards and the two colour bitboards are kept alongside
// a mailbox so that both kinds of lookup are inexpensive.
type Bitboard struct {
	pieces                   [13]uint64
	whitePieces, blackPieces uint64
	accMG, accEG, accPhase   int
	mailbox                  [64]Piece
}

// SquareMask contains one bit for every square, using A1 as bit zero.
var SquareMask = initSquareMask()

func initSquareMask() [64]uint64 {
	var masks [64]uint64
	for square := 0; square < len(masks); square++ {
		masks[square] = uint64(1) << uint(square)
	}
	return masks
}

// bitScanForward returns the index of the least-significant set bit.  The
// value 64 is the sentinel for an empty input.
func bitScanForward(value uint64) uint8 {
	if value == 0 {
		return 64
	}
	return uint8(bits.TrailingZeros64(value))
}

// bitScanReverse returns the index of the most-significant set bit.  The
// legacy empty-input sentinel is 127.
func bitScanReverse(value uint64) uint8 {
	if value == 0 {
		return 127
	}
	return uint8(63 - bits.LeadingZeros64(value))
}

// PopCount returns the number of set bits in value.
func PopCount(value uint64) int {
	return bits.OnesCount64(value)
}

func validSquare(square Square) bool {
	return square >= A1 && square <= H8
}

func validPiece(piece Piece) bool {
	return piece >= WhitePawn && piece <= BlackKing
}

func (b *Bitboard) squareBit(square Square) uint64 {
	return SquareMask[uint8(square)]
}

// addPiece adds a piece and its evaluation contribution at square.  It is an
// internal primitive: callers that need an empty mailbox use the explicit
// mailbox assignment in UpdateSquare or Move.
func (b *Bitboard) addPiece(square Square, piece Piece) {
	if piece == NoPiece || !validSquare(square) || !validPiece(piece) {
		return
	}

	mask := b.squareBit(square)
	b.pieces[piece] |= mask
	if piece.Color() == White {
		b.whitePieces |= mask
	} else {
		b.blackPieces |= mask
	}
	b.accMG += mgPST[piece][uint8(square)]
	b.accEG += egPST[piece][uint8(square)]
	b.accPhase += piecePhaseInc[piece]
	b.mailbox[uint8(square)] = piece
}

// Pawns returns the occupancy of both colours' pawns.
func (b *Bitboard) Pawns() uint64 {
	return b.pieces[WhitePawn] | b.pieces[BlackPawn]
}

// Knights returns the occupancy of both colours' knights.
func (b *Bitboard) Knights() uint64 {
	return b.pieces[WhiteKnight] | b.pieces[BlackKnight]
}

// Bishops returns the occupancy of both colours' bishops.
func (b *Bitboard) Bishops() uint64 {
	return b.pieces[WhiteBishop] | b.pieces[BlackBishop]
}

// Rooks returns the occupancy of both colours' rooks.
func (b *Bitboard) Rooks() uint64 {
	return b.pieces[WhiteRook] | b.pieces[BlackRook]
}

// Queens returns the occupancy of both colours' queens.
func (b *Bitboard) Queens() uint64 {
	return b.pieces[WhiteQueen] | b.pieces[BlackQueen]
}

// Kings returns the occupancy of both colours' kings.
func (b *Bitboard) Kings() uint64 {
	return b.pieces[WhiteKing] | b.pieces[BlackKing]
}

// GetWhitePieces returns the occupancy of all white pieces.
func (b *Bitboard) GetWhitePieces() uint64 {
	return b.whitePieces
}

// GetBlackPieces returns the occupancy of all black pieces.
func (b *Bitboard) GetBlackPieces() uint64 {
	return b.blackPieces
}

// GetBitboardOf returns the bitboard belonging to piece.  Invalid piece
// values have no corresponding board and therefore return an empty mask.
func (b *Bitboard) GetBitboardOf(piece Piece) uint64 {
	if piece < NoPiece || piece > BlackKing {
		return 0
	}
	return b.pieces[piece]
}

// AllPieces returns a new map containing the board's occupied mailbox
// entries.  The map is deliberately rebuilt for every call so that callers
// cannot mutate the board through the result.
func (b *Bitboard) AllPieces() map[Square]Piece {
	result := make(map[Square]Piece, 32)
	for square, piece := range b.mailbox {
		if piece != NoPiece {
			result[Square(square)] = piece
		}
	}
	return result
}

// PieceAt returns the mailbox occupant at square.  NoSquare, and any other
// out-of-range square, denotes an empty lookup.
func (b *Bitboard) PieceAt(square Square) Piece {
	if !validSquare(square) {
		return NoPiece
	}
	return b.mailbox[uint8(square)]
}

// Clear removes the specified piece's contribution at square.  The supplied
// piece, rather than the mailbox value, controls which contribution is
// removed.  That is important while a capture is being represented by
// overlapping piece masks.
func (b *Bitboard) Clear(square Square, piece Piece) {
	if piece == NoPiece || !validSquare(square) || !validPiece(piece) {
		return
	}

	mask := b.squareBit(square)
	b.pieces[piece] &^= mask
	if piece.Color() == White {
		b.whitePieces &^= mask
	} else {
		b.blackPieces &^= mask
	}
	b.accMG -= mgPST[piece][uint8(square)]
	b.accEG -= egPST[piece][uint8(square)]
	b.accPhase -= piecePhaseInc[piece]
	if b.mailbox[uint8(square)] == piece {
		b.mailbox[uint8(square)] = NoPiece
	}
}

// UpdateSquare replaces oldPiece with newPiece at square.
func (b *Bitboard) UpdateSquare(square Square, newPiece Piece, oldPiece Piece) {
	if !validSquare(square) {
		return
	}

	b.Clear(square, oldPiece)
	if newPiece == NoPiece {
		b.mailbox[uint8(square)] = NoPiece
		return
	}
	b.addPiece(square, newPiece)
}

func (b *Bitboard) movePiece(squareFrom Square, squareTo Square, piece Piece) {
	b.Clear(squareFrom, piece)
	if piece == NoPiece {
		b.mailbox[uint8(squareTo)] = NoPiece
		return
	}
	b.addPiece(squareTo, piece)
}

// Move updates a source/destination pair and applies the rook part of a
// castling move when the supplied king coordinates identify one.
func (b *Bitboard) Move(src, dest Square, sourcePiece, destinationPiece Piece) {
	if src == NoSquare || dest == NoSquare {
		return
	}
	if !validSquare(src) || !validSquare(dest) {
		return
	}

	b.Clear(dest, destinationPiece)
	if sourcePiece == NoPiece {
		b.mailbox[uint8(dest)] = NoPiece
	} else if validPiece(sourcePiece) {
		// Relocation preserves material phase. Update each occupancy once and
		// add only the PST difference; Clear/addPiece would undo and redo the
		// same material contribution and repeat square/piece validation.
		from, to := uint8(src), uint8(dest)
		remove, add := SquareMask[from], SquareMask[to]
		b.pieces[sourcePiece] = (b.pieces[sourcePiece] &^ remove) | add
		if sourcePiece <= WhiteKing {
			b.whitePieces = (b.whitePieces &^ remove) | add
		} else {
			b.blackPieces = (b.blackPieces &^ remove) | add
		}
		b.accMG += mgPST[sourcePiece][to] - mgPST[sourcePiece][from]
		b.accEG += egPST[sourcePiece][to] - egPST[sourcePiece][from]
		if b.mailbox[from] == sourcePiece {
			b.mailbox[from] = NoPiece
		}
		b.mailbox[to] = sourcePiece
	}

	switch sourcePiece {
	case WhiteKing:
		switch {
		case src == E1 && dest == G1:
			b.movePiece(H1, F1, WhiteRook)
		case src == E1 && dest == C1:
			b.movePiece(A1, D1, WhiteRook)
		}
	case BlackKing:
		switch {
		case src == E8 && dest == G8:
			b.movePiece(H8, F8, BlackRook)
		case src == E8 && dest == C8:
			b.movePiece(A8, D8, BlackRook)
		}
	}
}

// StartingBoard returns the ordinary initial chess position.
func StartingBoard() Bitboard {
	var board Bitboard

	whiteBackRank := [...]Piece{
		WhiteRook, WhiteKnight, WhiteBishop, WhiteQueen,
		WhiteKing, WhiteBishop, WhiteKnight, WhiteRook,
	}
	blackBackRank := [...]Piece{
		BlackRook, BlackKnight, BlackBishop, BlackQueen,
		BlackKing, BlackBishop, BlackKnight, BlackRook,
	}
	for file := 0; file < 8; file++ {
		board.UpdateSquare(Square(file), whiteBackRank[file], NoPiece)
		board.UpdateSquare(Square(8+file), WhitePawn, NoPiece)
		board.UpdateSquare(Square(48+file), BlackPawn, NoPiece)
		board.UpdateSquare(Square(56+file), blackBackRank[file], NoPiece)
	}
	return board
}

// copy returns an independent value copy of the board.
func (b *Bitboard) copy() Bitboard {
	return *b
}

// recomputeAccumulator rebuilds all additive evaluation fields from the
// current piece masks and the current global value tables.
func (b *Bitboard) recomputeAccumulator() {
	b.accMG = 0
	b.accEG = 0
	b.accPhase = 0
	for piece := WhitePawn; piece <= BlackKing; piece++ {
		occupancy := b.pieces[piece]
		for occupancy != 0 {
			square := bitScanForward(occupancy)
			b.accMG += mgPST[piece][square]
			b.accEG += egPST[piece][square]
			b.accPhase += piecePhaseInc[piece]
			occupancy &= occupancy - 1
		}
	}
}

// IsEndGame reports whether color has no non-pawn, non-king material.
func (b *Bitboard) IsEndGame(color Color) bool {
	switch color {
	case White:
		return b.pieces[WhiteKnight]|b.pieces[WhiteBishop]|b.pieces[WhiteRook]|b.pieces[WhiteQueen] == 0
	case Black:
		return b.pieces[BlackKnight]|b.pieces[BlackBishop]|b.pieces[BlackRook]|b.pieces[BlackQueen] == 0
	default:
		return false
	}
}

func pieceGlyph(piece Piece) string {
	switch piece {
	case WhitePawn:
		return "♙"
	case WhiteKnight:
		return "♘"
	case WhiteBishop:
		return "♗"
	case WhiteRook:
		return "♖"
	case WhiteQueen:
		return "♕"
	case WhiteKing:
		return "♔"
	case BlackPawn:
		return "♟"
	case BlackKnight:
		return "♞"
	case BlackBishop:
		return "♝"
	case BlackRook:
		return "♜"
	case BlackQueen:
		return "♛"
	case BlackKing:
		return "♚"
	default:
		return "-"
	}
}

// Draw returns a rank-descending Unicode debugging diagram.
func (b *Bitboard) Draw() string {
	var result strings.Builder
	result.Grow(2 + 18 + 8*20)
	result.WriteByte('\n')
	result.WriteString(" A B C D E F G H\n")
	for rank := 7; rank >= 0; rank-- {
		result.WriteByte(byte('1' + rank))
		for file := 0; file < 8; file++ {
			result.WriteString(pieceGlyph(b.mailbox[rank*8+file]))
			result.WriteByte(' ')
		}
		result.WriteByte('\n')
	}
	return result.String()
}
