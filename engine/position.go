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

// NOTE: Modified to remove unnecessary parts

package engine

import (
	"math/bits"
	"sync"
)

const CHECKMATE_EVAL int16 = 30000
const MAX_NON_CHECKMATE float32 = 25000
const MIN_NON_CHECKMATE float32 = -MAX_NON_CHECKMATE
const CASTLING_FLAG = WhiteCanCastleQueenSide | WhiteCanCastleKingSide | BlackCanCastleQueenSide | BlackCanCastleKingSide

type Position struct {
	Board          Bitboard
	EnPassant      Square
	Tag            PositionTag
	hash           uint64
	Positions      map[uint64]int
	positionsMutex sync.RWMutex
	HalfMoveClock  uint8
}

type PositionTag uint8

const (
	WhiteCanCastleKingSide PositionTag = 1 << iota
	WhiteCanCastleQueenSide
	BlackCanCastleKingSide
	BlackCanCastleQueenSide
	InCheck
	BlackToMove
	WhiteToMove
)

func (p *Position) SetTag(tag PositionTag)      { p.Tag |= tag }
func (p *Position) ClearTag(tag PositionTag)    { p.Tag &= ^tag }
func (p *Position) ToggleTag(tag PositionTag)   { p.Tag ^= tag }
func (p *Position) HasTag(tag PositionTag) bool { return p.Tag&tag != 0 }

func (p *Position) HasCastling() bool {
	return p.HasTag(CASTLING_FLAG)
}

func (p *Position) Turn() Color {
	if p.HasTag(WhiteToMove) {
		return White
	}
	return Black
}

func (p *Position) MakeNullMove() Square {
	ep := p.EnPassant
	p.EnPassant = NoSquare
	p.HalfMoveClock += 1
	p.ToggleTurn()
	updateHashForNullMove(p, NoSquare, ep)
	return ep
}

func (p *Position) UnMakeNullMove(ep Square) {
	updateHashForNullMove(p, NoSquare, ep)
	p.EnPassant = ep
	p.HalfMoveClock -= 1
	p.ToggleTurn()
}

func (p *Position) ToggleTurn() {
	p.ToggleTag(BlackToMove)
	p.ToggleTag(WhiteToMove)
}

// only for movegen
func (p *Position) partialMakeMove(move Move) {
	source := move.Source()
	dest := move.Destination()
	movingPiece := move.MovingPiece()
	cp := move.CapturedPiece()

	// EnPassant flag is a form of capture, captures do not result in enpassant allowance
	if move.IsEnPassant() {
		ep := findEnPassantCaptureSquare(move)
		p.Board.Move(source, dest, movingPiece, NoPiece)
		p.Board.Clear(ep, cp)
	} else {
		p.Board.Move(source, dest, movingPiece, cp)
	}

	// Do promotion
	promoType := move.PromoType()
	if promoType != NoType {
		promoPiece := GetPiece(promoType, p.Turn())
		p.Board.UpdateSquare(dest, promoPiece, movingPiece)
	}

	p.ToggleTurn()
}

// only for movegen
func (p *Position) partialUnMakeMove(move Move) {
	p.ToggleTurn()
	movingPiece := move.MovingPiece()
	capturedPiece := move.CapturedPiece()
	source := move.Source()
	dest := move.Destination()

	// Undo promotion
	promoType := move.PromoType()
	if promoType != NoType {
		promoPiece := GetPiece(promoType, p.Turn())
		p.Board.UpdateSquare(dest, movingPiece, promoPiece)
	}

	p.Board.Move(dest, source, movingPiece, NoPiece)
	// Undo enpassant
	if move.IsEnPassant() {
		cp := findEnPassantCaptureSquare(move)
		p.Board.UpdateSquare(cp, capturedPiece, NoPiece)
	} else if move.IsCapture() { // Undo capture
		p.Board.UpdateSquare(dest, capturedPiece, NoPiece)
	}

	if move.IsQueenSideCastle() {
		// white
		if dest == C1 {
			p.Board.Move(D1, A1, WhiteRook, NoPiece)
		} else { // black
			p.Board.Move(D8, A8, BlackRook, NoPiece)
		}
	} else if move.IsKingSideCastle() {
		// white
		if dest == G1 {
			p.Board.Move(F1, H1, WhiteRook, NoPiece)
		} else { // black
			p.Board.Move(F8, H8, BlackRook, NoPiece)
		}
	}
}

func (p *Position) GameMakeMove(move Move) (Square, PositionTag, uint8, bool) {
	ep, tag, hc, legal := p.makeMoveHelper(move)
	if legal {
		// Update repetition detection for game moves
		if p.Positions != nil {
			p.positionsMutex.Lock()
			p.Positions[p.Hash()]++
			p.positionsMutex.Unlock()
		}
	}
	return ep, tag, hc, legal
}

func (p *Position) MakeMove(move Move) (Square, PositionTag, uint8, bool) {
	// Search version - no repetition map updates for performance
	return p.makeMoveHelper(move)
}

func (p *Position) makeMoveHelper(move Move) (Square, PositionTag, uint8, bool) {
	hc := p.HalfMoveClock
	ep := p.EnPassant
	tag := p.Tag
	movingPiece := move.MovingPiece()
	capturedPiece := move.CapturedPiece()
	source := move.Source()
	dest := move.Destination()
	captureSquare := NoSquare
	promoPiece := NoPiece

	p.Board.Move(source, dest, movingPiece, NoPiece)

	// Direct pawn test instead of movingPiece.Type() (a 12-way switch) on this hot
	// make path: a pawn is exactly WhitePawn or BlackPawn.
	if movingPiece == WhitePawn || movingPiece == BlackPawn || capturedPiece != NoPiece {
		p.HalfMoveClock = 0
	} else {
		p.HalfMoveClock += 1
	}

	// EnPassant flag is a form of capture, captures do not result in enpassant allowance
	if move.IsEnPassant() {
		p.EnPassant = NoSquare
		ep := findEnPassantCaptureSquare(move)
		captureSquare = ep
		p.Board.Clear(ep, capturedPiece)
	} else if move.IsCapture() {
		captureSquare = dest
		p.Board.Clear(dest, capturedPiece)
	}

	if movingPiece == WhitePawn &&
		source.Rank() == Rank2 && dest.Rank() == Rank4 {
		p.EnPassant = SquareOf(source.File(), Rank3)
	} else if movingPiece == BlackPawn &&
		source.Rank() == Rank7 && dest.Rank() == Rank5 {
		p.EnPassant = SquareOf(source.File(), Rank6)
	} else {
		p.EnPassant = NoSquare
	}

	// Do promotion
	turn := p.Turn()
	promoType := move.PromoType()
	if promoType != NoType {
		promoPiece = GetPiece(promoType, turn)
		p.Board.UpdateSquare(dest, promoPiece, movingPiece)
	}

	if movingPiece == BlackKing {
		p.ClearTag(BlackCanCastleKingSide)
		p.ClearTag(BlackCanCastleQueenSide)
	} else if movingPiece == WhiteKing {
		p.ClearTag(WhiteCanCastleKingSide)
		p.ClearTag(WhiteCanCastleQueenSide)
	} else if movingPiece == BlackRook && source == A8 {
		p.ClearTag(BlackCanCastleQueenSide)
	} else if movingPiece == BlackRook && source == H8 {
		p.ClearTag(BlackCanCastleKingSide)
	} else if movingPiece == WhiteRook && source == A1 {
		p.ClearTag(WhiteCanCastleQueenSide)
	} else if movingPiece == WhiteRook && source == H1 {
		p.ClearTag(WhiteCanCastleKingSide)
	}

	// capturing rook nullifies castling right for the opponent on the rooks side
	if dest == A8 && p.Turn() == White {
		p.ClearTag(BlackCanCastleQueenSide)
	} else if dest == H8 && p.Turn() == White {
		p.ClearTag(BlackCanCastleKingSide)
	} else if dest == A1 && p.Turn() == Black {
		p.ClearTag(WhiteCanCastleQueenSide)
	} else if dest == H1 && p.Turn() == Black {
		p.ClearTag(WhiteCanCastleKingSide)
	}

	// movingSide := p.Turn()
	p.ToggleTurn()

	// Update check status for the new position
	if isInCheck(p, p.Turn()) {
		p.SetTag(InCheck)
	} else {
		p.ClearTag(InCheck)
	}

	// En passant only distinguishes a position when an enemy pawn can actually
	// capture; drop a non-capturable target (p.Turn() is post-ToggleTurn, i.e. the
	// side that could capture) so phantom EP squares do not split the hash and
	// repetition signature of otherwise-identical positions. Mirrors PolyglotHash.
	if p.EnPassant != NoSquare && !canCaptureEnPassant(p) {
		p.EnPassant = NoSquare
	}

	updateHash(p, move, captureSquare, p.EnPassant, ep, promoPiece, tag)

	// Update repetition detection - disabled in hot path for performance
	// Only updated in GameMakeMove for actual game moves

	return ep, tag, hc, true
}

func (p *Position) GameUnMakeMove(move Move, tag PositionTag, enPassant Square, halfClock uint8) {
	// Update repetition detection for game moves before undoing
	if p.Positions != nil {
		p.positionsMutex.Lock()
		currentHash := p.Hash()
		if count, exists := p.Positions[currentHash]; exists {
			if count > 1 {
				p.Positions[currentHash]--
			} else {
				delete(p.Positions, currentHash)
			}
		}
		p.positionsMutex.Unlock()
	}
	p.unMakeMoveHelper(move, tag, enPassant, halfClock)
}

func (p *Position) UnMakeMove(move Move, tag PositionTag, enPassant Square, halfClock uint8) {
	// Search version - no repetition map updates for performance
	p.unMakeMoveHelper(move, tag, enPassant, halfClock)
}

func (p *Position) unMakeMoveHelper(move Move, tag PositionTag, enPassant Square, halfClock uint8) {
	// Update repetition detection - disabled in hot path for performance
	// Only updated in GameUnMakeMove for actual game moves

	movingPiece := move.MovingPiece()
	capturedPiece := move.CapturedPiece()
	source := move.Source()
	dest := move.Destination()
	promoType := move.PromoType()

	// Reverse the hash update before restoring state.
	// updateHash is XOR-based, so calling it with the same args as make undoes the change.
	// At this point p.Tag and p.EnPassant still hold their post-make values, and
	// p.Turn() returns the side that did NOT move (post-make toggle), so the mover is p.Turn().Other().
	var promoPiece Piece = NoPiece
	if promoType != NoType {
		promoPiece = GetPiece(promoType, p.Turn().Other())
	}
	captureSquare := NoSquare
	if move.IsEnPassant() {
		captureSquare = findEnPassantCaptureSquare(move)
	} else if move.IsCapture() {
		captureSquare = dest
	}
	updateHash(p, move, captureSquare, p.EnPassant, enPassant, promoPiece, tag)

	p.Tag = tag
	p.HalfMoveClock = halfClock
	p.EnPassant = enPassant
	// Undo promotion
	if promoType != NoType {
		p.Board.UpdateSquare(dest, movingPiece, promoPiece)
	}
	p.Board.Move(dest, source, movingPiece, NoPiece)

	// Undo enpassant
	if move.IsEnPassant() {
		cp := findEnPassantCaptureSquare(move)
		p.Board.UpdateSquare(cp, capturedPiece, NoPiece)
	} else if move.IsCapture() { // Undo capture
		p.Board.UpdateSquare(dest, capturedPiece, NoPiece)
	}

	if move.IsQueenSideCastle() {
		// white
		if dest == C1 {
			p.Board.Move(D1, A1, WhiteRook, NoPiece)
		} else { // black
			p.Board.Move(D8, A8, BlackRook, NoPiece)
		}
	} else if move.IsKingSideCastle() {
		// white
		if dest == G1 {
			p.Board.Move(F1, H1, WhiteRook, NoPiece)
		} else { // black
			p.Board.Move(F8, H8, BlackRook, NoPiece)
		}
	}
}

type Status uint8

const (
	Checkmate Status = iota
	Draw
	Unknown
)

func (p *Position) IsEndGame() bool {
	return p.Board.IsEndGame(p.Turn())
}

func (p *Position) IsInCheck() bool {
	return p.HasTag(InCheck)
}

func (p *Position) IsDraw() bool {
	if p.HalfMoveClock > 100 {
		return true
	} else {
		if p.Board.pieces[BlackPawn] != 0 || p.Board.pieces[WhitePawn] != 0 ||
			p.Board.pieces[BlackRook] != 0 || p.Board.pieces[WhiteRook] != 0 ||
			p.Board.pieces[BlackQueen] != 0 || p.Board.pieces[WhiteQueen] != 0 {
			return false
		} else {
			wKnights := bitScanForward(p.Board.pieces[WhiteKnight])
			bKnights := bitScanForward(p.Board.pieces[BlackKnight])
			wBishops := bitScanForward(p.Board.pieces[WhiteBishop])
			bBishops := bitScanForward(p.Board.pieces[BlackBishop])

			wKnightsNum := 0
			bKnightsNum := 0
			wBishopsNum := 0
			bBishopsNum := 0

			if wKnights != 64 {
				wKnightsNum = 1
			}

			if bKnights != 64 {
				bKnightsNum = 1
			}

			if wBishops != 64 {
				wBishopsNum = 1
			}

			if bBishops != 64 {
				bBishopsNum = 1
			}

			all := wKnightsNum + bKnightsNum + wBishopsNum + bBishopsNum

			// both sides have a bare king
			// one side has a king and a minor piece against a bare king

			if all <= 1 {
				if wKnightsNum != 0 {
					return bits.OnesCount64(p.Board.pieces[WhiteKnight]) == 1
				} else if bKnightsNum != 0 {
					return bits.OnesCount64(p.Board.pieces[BlackKnight]) == 1
				} else if wBishopsNum != 0 {
					return bits.OnesCount64(p.Board.pieces[WhiteBishop]) == 1
				} else if bBishopsNum != 0 {
					return bits.OnesCount64(p.Board.pieces[BlackBishop]) == 1
				}
			}
			// both sides have a king and a bishop, the bishops being the same color
			if wKnightsNum == 0 && bKnightsNum == 0 {
				otherWB := p.Board.pieces[WhiteBishop] ^ (1 << wBishops)
				otherBB := p.Board.pieces[BlackBishop] ^ (1 << bBishops)
				if otherWB == 0 && otherBB == 0 &&
					Square(bBishops).GetColor() == Square(wBishops).GetColor() {
					return true
				}
			}
		}
	}

	return false
}

func (p *Position) IsFIDEDrawRule() bool {
	if p.HalfMoveClock >= 100 {
		return true
	}
	if p.Positions == nil {
		return false
	}
	p.positionsMutex.RLock()
	value, ok := p.Positions[p.Hash()]
	p.positionsMutex.RUnlock()
	return (ok && value >= 3)
}

func (p *Position) Hash() uint64 {
	if p.hash == 0 {
		hash := generateZobristHash(p)
		p.hash = hash
	}
	return p.hash
}

func findEnPassantCaptureSquare(move Move) Square {
	return move.Destination() ^ 8
}

func (p *Position) Copy() *Position {
	copyMap := make(map[uint64]int, len(p.Positions))
	for k, v := range p.Positions {
		copyMap[k] = v
	}

	newPos := &Position{
		Board:         p.Board.copy(),
		EnPassant:     p.EnPassant,
		Tag:           p.Tag,
		hash:          p.hash,
		Positions:     copyMap,
		HalfMoveClock: p.HalfMoveClock,
		// Note: Don't copy the mutex - each copy gets its own mutex
	}
	return newPos
}
