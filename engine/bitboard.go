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

import (
	"fmt"
	"math/bits"
)

type Bitboard struct {
	// Per-piece occupancy indexed directly by Piece (NoPiece=0 slot stays empty),
	// so GetBitboardOf is a single array load instead of a 12-way switch (F1).
	pieces      [13]uint64
	whitePieces uint64
	blackPieces uint64

	// Incremental eval accumulator: running material+PST sums (white-perspective,
	// MG and EG) and game phase, maintained by Clear/UpdateSquare/Move so
	// evaluatePeSTO never rescans the board. Bit-exact with a full scan (integer
	// sums), so search stays node-identical. Rebuild with recomputeAccumulator
	// after any bulk PST-table change (RebuildPST) on a board that outlives it.
	accMG    int
	accEG    int
	accPhase int

	// Piece mailbox (square -> Piece): maintained by Clear/UpdateSquare/Move in
	// lockstep with the piece bitboards so PieceAt is an O(1) array read instead
	// of a 12-way bitboard scan (T6). Bit-exact with the scan (NoPiece==0, so the
	// zero-value board is all-empty), so search stays node-identical.
	mailbox [64]Piece
}

func (b *Bitboard) Pawns() uint64 {
	return b.pieces[WhitePawn] | b.pieces[BlackPawn]
}

func (b *Bitboard) Knights() uint64 {
	return b.pieces[WhiteKnight] | b.pieces[BlackKnight]
}

func (b *Bitboard) Bishops() uint64 {
	return b.pieces[WhiteBishop] | b.pieces[BlackBishop]
}

func (b *Bitboard) Rooks() uint64 {
	return b.pieces[WhiteRook] | b.pieces[BlackRook]
}

func (b *Bitboard) Queens() uint64 {
	return b.pieces[WhiteQueen] | b.pieces[BlackQueen]
}

func (b *Bitboard) Kings() uint64 {
	return b.pieces[WhiteKing] | b.pieces[BlackKing]
}

func (b *Bitboard) GetWhitePieces() uint64 {
	return b.whitePieces
}

func (b *Bitboard) GetBlackPieces() uint64 {
	return b.blackPieces
}

func (b *Bitboard) GetBitboardOf(piece Piece) uint64 {
	return b.pieces[piece]
}

func (b *Bitboard) AllPieces() map[Square]Piece {
	allPieces := make(map[Square]Piece, 32)
	allBits := b.whitePieces | b.blackPieces
	for allBits != 0 {
		index := bitScanForward(allBits)
		mask := SquareMask[index]
		sq := Square(index)
		if b.pieces[BlackPawn]&(mask) != 0 {
			allPieces[sq] = BlackPawn
		} else if b.pieces[WhitePawn]&(mask) != 0 {
			allPieces[sq] = WhitePawn
		} else if b.pieces[BlackKnight]&(mask) != 0 {
			allPieces[sq] = BlackKnight
		} else if b.pieces[WhiteKnight]&(mask) != 0 {
			allPieces[sq] = WhiteKnight
		} else if b.pieces[BlackBishop]&(mask) != 0 {
			allPieces[sq] = BlackBishop
		} else if b.pieces[WhiteBishop]&(mask) != 0 {
			allPieces[sq] = WhiteBishop
		} else if b.pieces[BlackRook]&(mask) != 0 {
			allPieces[sq] = BlackRook
		} else if b.pieces[WhiteRook]&(mask) != 0 {
			allPieces[sq] = WhiteRook
		} else if b.pieces[BlackQueen]&(mask) != 0 {
			allPieces[sq] = BlackQueen
		} else if b.pieces[WhiteQueen]&(mask) != 0 {
			allPieces[sq] = WhiteQueen
		} else if b.pieces[BlackKing]&(mask) != 0 {
			allPieces[sq] = BlackKing
		} else if b.pieces[WhiteKing]&(mask) != 0 {
			allPieces[sq] = WhiteKing
		}
		allBits ^= mask
	}
	return allPieces
}

func (b *Bitboard) UpdateSquare(sq Square, newPiece Piece, oldPiece Piece) {
	// Remove the piece from source square and add it to destination
	b.Clear(sq, oldPiece)
	if newPiece != NoPiece {
		b.accMG += mgPST[newPiece][sq]
		b.accEG += egPST[newPiece][sq]
		b.accPhase += piecePhaseInc[newPiece]
		mask := SquareMask[int(sq)]
		b.pieces[newPiece] |= mask
		if newPiece < BlackPawn { // white pieces are WhitePawn..WhiteKing (1..6)
			b.whitePieces |= mask
		} else {
			b.blackPieces |= mask
		}
	}
	b.mailbox[sq] = newPiece
}

func (b *Bitboard) PieceAt(sq Square) Piece {
	if sq == NoSquare {
		return NoPiece
	}
	// O(1) mailbox read (T6); maintained bit-exact with the bitboards by
	// Clear/UpdateSquare/Move, replacing the former 12-way scan.
	return b.mailbox[sq]
}

func (b *Bitboard) Clear(square Square, piece Piece) {
	if piece == NoPiece {
		return
	}
	// Only blank the mailbox if this piece is the recorded occupant. MakeMove
	// places the mover at dest BEFORE clearing the captured piece (Clear(dest,
	// captured) runs after Move set mailbox[dest]=mover), so an unconditional
	// blank here would erase the mover. The accumulator below stays unconditional
	// (it's commutative); only the positional mailbox needs the guard.
	if b.mailbox[square] == piece {
		b.mailbox[square] = NoPiece
	}
	b.accMG -= mgPST[piece][square]
	b.accEG -= egPST[piece][square]
	b.accPhase -= piecePhaseInc[piece]
	mask := SquareMask[int(square)]
	b.pieces[piece] &^= mask
	if piece < BlackPawn { // white pieces are WhitePawn..WhiteKing (1..6)
		b.whitePieces &^= mask
	} else {
		b.blackPieces &^= mask
	}
}

func (b *Bitboard) Move(src Square, dest Square, sourcePiece Piece, destinationPiece Piece) {

	if src == NoSquare || dest == NoSquare {
		return
	}
	// clear destination square
	b.Clear(dest, destinationPiece)
	b.Clear(src, sourcePiece)
	maskDest := SquareMask[int(dest)]
	if sourcePiece != NoPiece {
		b.accMG += mgPST[sourcePiece][dest]
		b.accEG += egPST[sourcePiece][dest]
		b.accPhase += piecePhaseInc[sourcePiece]
	}

	// Remove the piece from source square and add it to destination
	if sourcePiece != NoPiece {
		b.pieces[sourcePiece] |= maskDest
		if sourcePiece < BlackPawn { // white pieces are WhitePawn..WhiteKing (1..6)
			b.whitePieces |= maskDest
		} else {
			b.blackPieces |= maskDest
		}
	}
	// Castling: move the rook alongside the king. Order preserved — the rook's
	// nested Move runs after the king's bits are set and before mailbox[dest].
	if sourcePiece == WhiteKing {
		if src == E1 && dest == G1 {
			b.Move(H1, F1, WhiteRook, NoPiece)
		} else if src == E1 && dest == C1 {
			b.Move(A1, D1, WhiteRook, NoPiece)
		}
	} else if sourcePiece == BlackKing {
		if src == E8 && dest == G8 {
			b.Move(H8, F8, BlackRook, NoPiece)
		} else if src == E8 && dest == C8 {
			b.Move(A8, D8, BlackRook, NoPiece)
		}
	}
	// src was cleared to NoPiece above; place sourcePiece at dest (the nested
	// castle Move set the rook's own mailbox entry).
	b.mailbox[dest] = sourcePiece
}

func StartingBoard() Bitboard {
	bitboard := Bitboard{}
	bitboard.UpdateSquare(A2, WhitePawn, NoPiece)
	bitboard.UpdateSquare(B2, WhitePawn, NoPiece)
	bitboard.UpdateSquare(C2, WhitePawn, NoPiece)
	bitboard.UpdateSquare(D2, WhitePawn, NoPiece)
	bitboard.UpdateSquare(E2, WhitePawn, NoPiece)
	bitboard.UpdateSquare(F2, WhitePawn, NoPiece)
	bitboard.UpdateSquare(G2, WhitePawn, NoPiece)
	bitboard.UpdateSquare(H2, WhitePawn, NoPiece)

	bitboard.UpdateSquare(A7, BlackPawn, NoPiece)
	bitboard.UpdateSquare(B7, BlackPawn, NoPiece)
	bitboard.UpdateSquare(C7, BlackPawn, NoPiece)
	bitboard.UpdateSquare(D7, BlackPawn, NoPiece)
	bitboard.UpdateSquare(E7, BlackPawn, NoPiece)
	bitboard.UpdateSquare(F7, BlackPawn, NoPiece)
	bitboard.UpdateSquare(G7, BlackPawn, NoPiece)
	bitboard.UpdateSquare(H7, BlackPawn, NoPiece)

	bitboard.UpdateSquare(A1, WhiteRook, NoPiece)
	bitboard.UpdateSquare(B1, WhiteKnight, NoPiece)
	bitboard.UpdateSquare(C1, WhiteBishop, NoPiece)
	bitboard.UpdateSquare(D1, WhiteQueen, NoPiece)
	bitboard.UpdateSquare(E1, WhiteKing, NoPiece)
	bitboard.UpdateSquare(F1, WhiteBishop, NoPiece)
	bitboard.UpdateSquare(G1, WhiteKnight, NoPiece)
	bitboard.UpdateSquare(H1, WhiteRook, NoPiece)

	bitboard.UpdateSquare(A8, BlackRook, NoPiece)
	bitboard.UpdateSquare(B8, BlackKnight, NoPiece)
	bitboard.UpdateSquare(C8, BlackBishop, NoPiece)
	bitboard.UpdateSquare(D8, BlackQueen, NoPiece)
	bitboard.UpdateSquare(E8, BlackKing, NoPiece)
	bitboard.UpdateSquare(F8, BlackBishop, NoPiece)
	bitboard.UpdateSquare(G8, BlackKnight, NoPiece)
	bitboard.UpdateSquare(H8, BlackRook, NoPiece)

	return bitboard
}

func (b *Bitboard) IsEndGame(turn Color) bool {
	if turn == White {
		return b.pieces[WhiteKnight]+b.pieces[WhiteBishop]+b.pieces[WhiteRook]+b.pieces[WhiteQueen] == 0
	} else if turn == Black {
		return b.pieces[BlackKnight]+b.pieces[BlackBishop]+b.pieces[BlackRook]+b.pieces[BlackQueen] == 0
	}
	return false
}

// Draw returns visual representation of the board useful for debugging.
func (b *Bitboard) Draw() string {
	pieceUnicodes := []string{"♙", "♘", "♗", "♖", "♕", "♔", "♟", "♞", "♝", "♜", "♛", "♚"}
	s := "\n A B C D E F G H\n"
	for r := 7; r >= 0; r-- {
		s += fmt.Sprint(Rank(r + 1))
		for f := 0; f < len(Files); f++ {
			p := b.PieceAt(SquareOf(File(f), Rank(r)))
			if p == NoPiece {
				s += "-"
			} else {
				s += pieceUnicodes[int(p-1)]
			}
			s += " "
		}
		s += "\n"
	}
	return s
}

func (b *Bitboard) copy() Bitboard {
	return *b
}

// recomputeAccumulator rebuilds the incremental eval accumulator (material+PST
// MG/EG sums and game phase) from a full board scan. The accumulator is
// otherwise maintained incrementally by Clear/UpdateSquare/Move; call this for a
// board that outlives a bulk PST-table change (RebuildPST), where the cached
// sums would otherwise be stale.
func (b *Bitboard) recomputeAccumulator() {
	var mg, eg, phase int
	for p := WhitePawn; p <= BlackKing; p++ {
		bb := b.GetBitboardOf(p)
		for bb != 0 {
			sq := trailingZeros(bb)
			mg += mgPST[p][sq]
			eg += egPST[p][sq]
			phase += piecePhaseInc[p]
			bb &= bb - 1
		}
	}
	b.accMG = mg
	b.accEG = eg
	b.accPhase = phase
}

/*
moved from movegen.go
*/
var SquareMask = initSquareMask()

func initSquareMask() [64]uint64 {
	var sqm [64]uint64
	for sq := 0; sq < 64; sq++ {
		var b = uint64(1 << sq)
		sqm[sq] = b
	}
	return sqm
}

func bitScanForward(bb uint64) uint8 {
	return uint8(bits.TrailingZeros64(bb))
}

func bitScanReverse(bb uint64) uint8 {
	return uint8(bits.LeadingZeros64(bb) ^ 63)
}

func PopCount(bb uint64) int {
	return bits.OnesCount64(bb)
}
