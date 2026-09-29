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
	"crypto/sha256"
	"encoding/binary"
	"math/rand"
)

var piecesZC [12][64]uint64
var castleRightsZC [4]uint64
var enPassantZC [16]uint64
var whiteTurnZC uint64

func init() {
	hash := sha256.Sum256([]byte("ehrlich"))
	seed := binary.LittleEndian.Uint64(hash[:8])
	var r = rand.New(rand.NewSource(int64(seed)))
	whiteTurnZC = r.Uint64()
	for i := 0; i < 12; i++ {
		for j := 0; j < 64; j++ {
			piecesZC[i][j] = r.Uint64()
		}
	}
	for i := 0; i < 4; i++ {
		castleRightsZC[i] = r.Uint64()
	}
	for i := 0; i < 16; i++ {
		enPassantZC[i] = r.Uint64()
	}
}

func generateZobristHash(pos *Position) uint64 {
	var hash uint64 = 0
	/* Turn */
	if pos.Turn() == White {
		hash ^= whiteTurnZC
	}

	/* Castle */
	if pos.HasTag(WhiteCanCastleKingSide) {
		hash ^= castleRightsZC[0]
	}
	if pos.HasTag(WhiteCanCastleQueenSide) {
		hash ^= castleRightsZC[1]
	}
	if pos.HasTag(BlackCanCastleKingSide) {
		hash ^= castleRightsZC[2]
	}
	if pos.HasTag(BlackCanCastleQueenSide) {
		hash ^= castleRightsZC[3]
	}

	/* En passant */
	enPassant := pos.EnPassant
	if enPassant != NoSquare {
		if pos.Turn() == Black {
			/* Next mov Black -> Current pos White -> White en passant square */
			if enPassant >= 16 && enPassant <= 23 {
				hash ^= enPassantZC[enPassant-16]
			}
		} else {
			/* Next mov White -> Current pos Black -> Black en passant square */
			if enPassant >= 40 && enPassant <= 47 {
				hash ^= enPassantZC[enPassant-40+8]
			}
		}
	}

	/* Board */
	board := pos.Board
	for sq := A1; sq <= H8; sq++ {
		p := board.PieceAt(sq)
		if p != NoPiece {
			hash ^= piecesZC[int8(p)-1][sq]
		}
	}

	return hash
}

func updateHashForNullMove(pos *Position, newEnPassant Square, oldEnPassant Square) {
	if pos.hash == 0 {
		pos.Hash()
		return
	}
	var hash uint64 = pos.hash
	/* Turn */
	hash ^= whiteTurnZC

	turn := pos.Turn()
	/* En passant */
	if newEnPassant != NoSquare {
		if turn == Black {
			/* Next mov Black -> Current pos White -> White en passant square */
			if newEnPassant >= 16 && newEnPassant <= 23 {
				hash ^= enPassantZC[newEnPassant-16]
			}
		} else {
			/* Next mov White -> Current pos Black -> Black en passant square */
			if newEnPassant >= 40 && newEnPassant <= 47 {
				hash ^= enPassantZC[newEnPassant-40+8]
			}
		}
	}

	if oldEnPassant != NoSquare {
		if turn == Black {
			/* Previous mov Black -> Current pos White -> Black en passant square */
			if oldEnPassant >= 40 && oldEnPassant <= 47 {
				hash ^= enPassantZC[oldEnPassant-40+8]
			}
		} else {
			/* Previous mov White -> Current pos Black -> White en passant square */
			if oldEnPassant >= 16 && oldEnPassant <= 23 {
				hash ^= enPassantZC[oldEnPassant-16]
			}
		}
	}

	pos.hash = hash
}

// capture square is provided for the case of enpassant
func updateHash(pos *Position, move Move, captureSquare Square,
	newEnPassant Square, oldEnPassant Square, promoPiece Piece, oldPositionTag PositionTag) {
	source := move.Source()
	dest := move.Destination()
	var hash uint64 = pos.hash
	if hash == 0 {
		pos.Hash()
		return
	}
	/* Turn */
	hash ^= whiteTurnZC
	turn := pos.Turn()

	/* Castle */
	if source == E1 { // White
		if move.IsKingSideCastle() {
			hash ^= piecesZC[int8(WhiteRook)-1][H1]
			hash ^= piecesZC[int8(WhiteRook)-1][F1]
		}
		if move.IsQueenSideCastle() {
			hash ^= piecesZC[int8(WhiteRook)-1][A1]
			hash ^= piecesZC[int8(WhiteRook)-1][D1]
		}
	} else if source == E8 { // Black
		if move.IsKingSideCastle() {
			hash ^= piecesZC[int8(BlackRook)-1][H8]
			hash ^= piecesZC[int8(BlackRook)-1][F8]
		}
		if move.IsQueenSideCastle() {
			hash ^= piecesZC[int8(BlackRook)-1][A8]
			hash ^= piecesZC[int8(BlackRook)-1][D8]
		}
	}

	// XOR the old/new tags once; a castle-right toggled iff its bit is set in the
	// diff. Replaces eight ANDs + four cross-compares with one XOR + four bit tests
	// (bit-identical: A&F != B&F  <=>  (A^B)&F != 0).
	castleChanged := oldPositionTag ^ pos.Tag
	if castleChanged&WhiteCanCastleKingSide != 0 {
		hash ^= castleRightsZC[0]
	}
	if castleChanged&WhiteCanCastleQueenSide != 0 {
		hash ^= castleRightsZC[1]
	}
	if castleChanged&BlackCanCastleKingSide != 0 {
		hash ^= castleRightsZC[2]
	}
	if castleChanged&BlackCanCastleQueenSide != 0 {
		hash ^= castleRightsZC[3]
	}

	/* En passant */
	if newEnPassant != NoSquare {
		if turn == Black {
			/* Next mov Black -> Current pos White -> White en passant square */
			if newEnPassant >= 16 && newEnPassant <= 23 {
				hash ^= enPassantZC[newEnPassant-16]
			}
		} else {
			/* Next mov White -> Current pos Black -> Black en passant square */
			if newEnPassant >= 40 && newEnPassant <= 47 {
				hash ^= enPassantZC[newEnPassant-40+8]
			}
		}
	}

	if oldEnPassant != NoSquare {
		if turn == Black {
			/* Previous mov Black -> Current pos White -> Black en passant square */
			if oldEnPassant >= 40 && oldEnPassant <= 47 {
				hash ^= enPassantZC[oldEnPassant-40+8]
			}
		} else {
			/* Previous mov White -> Current pos Black -> White en passant square */
			if oldEnPassant >= 16 && oldEnPassant <= 23 {
				hash ^= enPassantZC[oldEnPassant-16]
			}
		}
	}

	movingPiece := move.MovingPiece()

	// Safety check to prevent index out of range panic
	if movingPiece == NoPiece {
		// This should not happen in a valid move - recalculate hash from scratch
		pos.Hash()
		return
	}

	/* Board */
	hash ^= piecesZC[int8(movingPiece)-1][source]
	if promoPiece != NoPiece {
		hash ^= piecesZC[int8(promoPiece)-1][dest]
	} else {
		hash ^= piecesZC[int8(movingPiece)-1][dest]
	}

	cp := move.CapturedPiece()
	if cp != NoPiece {
		hash ^= piecesZC[int8(cp)-1][captureSquare]
	}

	pos.hash = hash
}
