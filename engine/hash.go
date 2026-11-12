package engine

import (
	"crypto/sha256"
	"encoding/binary"
	"math/rand"
)

var (
	occupantKeys [64][13]uint64
	castlingKeys [4]uint64
	epTargetKeys [16]uint64
	whiteSideKey uint64
)

func init() {
	digest := sha256.Sum256([]byte("NGN board keys v1"))
	seed := int64(binary.LittleEndian.Uint64(digest[:8]))
	random := rand.New(rand.NewSource(seed))
	for index := range epTargetKeys {
		epTargetKeys[index] = random.Uint64()
	}
	for index := range castlingKeys {
		castlingKeys[index] = random.Uint64()
	}
	for square := 63; square >= 0; square-- {
		for piece := 12; piece >= 1; piece-- {
			occupantKeys[square][piece] = random.Uint64()
		}
	}
	whiteSideKey = random.Uint64()
	initCuckoo()
}


func epTargetKey(square Square, turn Color) uint64 {
	switch turn {
	case Black:
		if square >= A3 && square <= H3 {
			return epTargetKeys[int(square-A3)]
		}
	case White:
		if square >= A6 && square <= H6 {
			return epTargetKeys[8+int(square-A6)]
		}
	}
	return 0
}

func occupantKey(piece Piece, square Square) uint64 {
	return occupantKeys[int(square)][int(piece)]
}

func computePositionKey(pos *Position) uint64 {
	var hash uint64
	turn := pos.Turn()
	if turn == White {
		hash ^= whiteSideKey
	}

	for index := 0; index < 4; index++ {
		if pos.Tag&PositionState(1<<index) != 0 {
			hash ^= castlingKeys[index]
		}
	}

	for square := Square(0); square <= H8; square++ {
		piece := pos.Board.PieceAt(square)
		if piece != NoPiece {
			hash ^= occupantKey(piece, square)
		}
	}

	if pos.HasTag(enPassantHash) {
		hash ^= epTargetKey(pos.EnPassant, turn)
	}
	return hash
}

func advanceNullKey(pos *Position, newEnPassant Square, oldEnPassant Square) {
	if pos.hash == 0 {
		pos.Hash()
		return
	}

	hash := pos.hash ^ whiteSideKey
	if pos.HasTag(enPassantHash) {
		turn := pos.Turn()
		hash ^= epTargetKey(newEnPassant, turn)
		hash ^= epTargetKey(oldEnPassant, turn.Other())
	}
	pos.hash = hash
}

func advanceMoveKey(pos *Position, move Move, captureSquare Square, newEnPassant Square, oldEnPassant Square, promoPiece Piece, previousState PositionState) {
	if pos.hash == 0 {
		pos.Hash()
		return
	}
	if move.MovingPiece() == NoPiece {
		pos.Hash()
		return
	}

	hash := pos.hash ^ whiteSideKey
	source := move.Source()
	if move.IsKingSideCastle() {
		switch source {
		case E1:
			hash ^= occupantKey(WhiteRook, H1)
			hash ^= occupantKey(WhiteRook, F1)
		case E8:
			hash ^= occupantKey(BlackRook, H8)
			hash ^= occupantKey(BlackRook, F8)
		}
	} else if move.IsQueenSideCastle() {
		switch source {
		case E1:
			hash ^= occupantKey(WhiteRook, A1)
			hash ^= occupantKey(WhiteRook, D1)
		case E8:
			hash ^= occupantKey(BlackRook, A8)
			hash ^= occupantKey(BlackRook, D8)
		}
	}

	changedCastle := previousState ^ pos.Tag
	for index := 0; index < 4; index++ {
		if changedCastle&PositionState(1<<index) != 0 {
			hash ^= castlingKeys[index]
		}
	}

	turn := pos.Turn()
	if pos.HasTag(enPassantHash) {
		hash ^= epTargetKey(newEnPassant, turn)
	}
	if previousState&enPassantHash != 0 {
		hash ^= epTargetKey(oldEnPassant, turn.Other())
	}

	moving := move.MovingPiece()
	hash ^= occupantKey(moving, source)
	destinationPiece := moving
	if promoPiece != NoPiece {
		destinationPiece = promoPiece
	}
	hash ^= occupantKey(destinationPiece, move.Destination())

	if captured := move.CapturedPiece(); captured != NoPiece {
		hash ^= occupantKey(captured, captureSquare)
	}

	pos.hash = hash
}

func hasLegalEnPassant(pos *Position) bool {
	target := pos.EnPassant
	if target == NoSquare || target < A1 || target > H8 {
		return false
	}
	if pos.Board.PieceAt(target) != NoPiece {
		return false
	}

	turn := pos.Turn()
	targetFile := int(target.File())
	victim := NoSquare
	candidateRank := Rank1
	ownPawn := NoPiece
	victimPawn := NoPiece
	switch turn {
	case White:
		if target.Rank() != Rank6 {
			return false
		}
		victim = SquareOf(File(targetFile), Rank5)
		candidateRank = Rank5
		ownPawn = WhitePawn
		victimPawn = BlackPawn
	case Black:
		if target.Rank() != Rank3 {
			return false
		}
		victim = SquareOf(File(targetFile), Rank4)
		candidateRank = Rank4
		ownPawn = BlackPawn
		victimPawn = WhitePawn
	default:
		return false
	}
	if pos.Board.PieceAt(victim) != victimPawn {
		return false
	}

	for direction := -1; direction <= 1; direction += 2 {
		candidateFile := targetFile + direction
		if candidateFile < int(FileA) || candidateFile > int(FileH) {
			continue
		}
		source := SquareOf(File(candidateFile), candidateRank)
		if pos.Board.PieceAt(source) != ownPawn {
			continue
		}

		board := pos.Board
		board.Move(source, target, ownPawn, NoPiece)
		board.Clear(victim, victimPawn)
		candidate := Position{Board: board}
		if !isInCheck(&candidate, turn) {
			return true
		}
	}
	return false
}

func (pos *Position) refreshEnPassantHash() {
	pos.ClearTag(enPassantHash)
	if hasLegalEnPassant(pos) {
		pos.SetTag(enPassantHash)
	}
}

func (pos *Position) ensureEnPassantHash() {
	if pos.EnPassant != NoSquare {
		pos.refreshEnPassantHash()
	}
}
