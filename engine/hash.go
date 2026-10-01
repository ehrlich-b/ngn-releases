package engine

import (
	"crypto/sha256"
	"encoding/binary"
	"math/rand"
)

var (
	piecesZC       [12][64]uint64
	castleRightsZC [4]uint64
	enPassantZC    [16]uint64
	whiteTurnZC    uint64
)

func init() {
	digest := sha256.Sum256([]byte("ehrlich"))
	seed := int64(binary.LittleEndian.Uint64(digest[:8]))
	random := rand.New(rand.NewSource(seed))

	whiteTurnZC = random.Uint64()
	for piece := range piecesZC {
		for square := range piecesZC[piece] {
			piecesZC[piece][square] = random.Uint64()
		}
	}
	for index := range castleRightsZC {
		castleRightsZC[index] = random.Uint64()
	}
	for index := range enPassantZC {
		enPassantZC[index] = random.Uint64()
	}
}

func enPassantZobrist(square Square, turn Color) uint64 {
	switch turn {
	case Black:
		if square >= A3 && square <= H3 {
			return enPassantZC[int(square-A3)]
		}
	case White:
		if square >= A6 && square <= H6 {
			return enPassantZC[8+int(square-A6)]
		}
	}
	return 0
}

func pieceZobrist(piece Piece, square Square) uint64 {
	return piecesZC[int(piece)-1][int(square)]
}

func generateZobristHash(pos *Position) uint64 {
	var hash uint64
	turn := pos.Turn()
	if turn == White {
		hash ^= whiteTurnZC
	}

	for index := 0; index < 4; index++ {
		if pos.Tag&PositionTag(1<<index) != 0 {
			hash ^= castleRightsZC[index]
		}
	}

	for square := Square(0); square <= H8; square++ {
		piece := pos.Board.PieceAt(square)
		if piece != NoPiece {
			hash ^= pieceZobrist(piece, square)
		}
	}

	if pos.HasTag(enPassantHash) {
		hash ^= enPassantZobrist(pos.EnPassant, turn)
	}
	return hash
}

func updateHashForNullMove(pos *Position, newEnPassant Square, oldEnPassant Square) {
	if pos.hash == 0 {
		pos.Hash()
		return
	}

	hash := pos.hash ^ whiteTurnZC
	if pos.HasTag(enPassantHash) {
		turn := pos.Turn()
		hash ^= enPassantZobrist(newEnPassant, turn)
		hash ^= enPassantZobrist(oldEnPassant, turn.Other())
	}
	pos.hash = hash
}

func updateHash(pos *Position, move Move, captureSquare Square, newEnPassant Square, oldEnPassant Square, promoPiece Piece, oldPositionTag PositionTag) {
	if pos.hash == 0 {
		pos.Hash()
		return
	}
	if move.MovingPiece() == NoPiece {
		pos.Hash()
		return
	}

	hash := pos.hash ^ whiteTurnZC
	source := move.Source()
	if move.IsKingSideCastle() {
		switch source {
		case E1:
			hash ^= pieceZobrist(WhiteRook, H1)
			hash ^= pieceZobrist(WhiteRook, F1)
		case E8:
			hash ^= pieceZobrist(BlackRook, H8)
			hash ^= pieceZobrist(BlackRook, F8)
		}
	} else if move.IsQueenSideCastle() {
		switch source {
		case E1:
			hash ^= pieceZobrist(WhiteRook, A1)
			hash ^= pieceZobrist(WhiteRook, D1)
		case E8:
			hash ^= pieceZobrist(BlackRook, A8)
			hash ^= pieceZobrist(BlackRook, D8)
		}
	}

	changedCastle := oldPositionTag ^ pos.Tag
	for index := 0; index < 4; index++ {
		if changedCastle&PositionTag(1<<index) != 0 {
			hash ^= castleRightsZC[index]
		}
	}

	turn := pos.Turn()
	if pos.HasTag(enPassantHash) {
		hash ^= enPassantZobrist(newEnPassant, turn)
	}
	if oldPositionTag&enPassantHash != 0 {
		hash ^= enPassantZobrist(oldEnPassant, turn.Other())
	}

	moving := move.MovingPiece()
	hash ^= pieceZobrist(moving, source)
	destinationPiece := moving
	if promoPiece != NoPiece {
		destinationPiece = promoPiece
	}
	hash ^= pieceZobrist(destinationPiece, move.Destination())

	if captured := move.CapturedPiece(); captured != NoPiece {
		hash ^= pieceZobrist(captured, captureSquare)
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
