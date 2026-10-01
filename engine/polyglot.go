package engine

import (
	"encoding/binary"
	"os"
	"sort"
)

// PolyglotEntry represents a single entry in a Polyglot opening book
type PolyglotEntry struct {
	Key    uint64
	Move   uint16
	Weight uint16
	Learn  uint32
}

// PolyglotBook represents a Polyglot format opening book
type PolyglotBook struct {
	entries []PolyglotEntry
	path    string
}

// Polyglot piece mapping (different from our internal representation)
var polyglotPieces = [12]Piece{
	BlackPawn, WhitePawn,
	BlackKnight, WhiteKnight,
	BlackBishop, WhiteBishop,
	BlackRook, WhiteRook,
	BlackQueen, WhiteQueen,
	BlackKing, WhiteKing,
}

// PolyglotHash computes the Polyglot hash for a position
func PolyglotHash(pos *Position) uint64 {
	var hash uint64

	// Pieces on squares
	for sq := 0; sq < 64; sq++ {
		piece := pos.Board.PieceAt(Square(sq))
		if piece != NoPiece {
			// Convert to Polyglot piece index
			polyPiece := -1
			switch piece {
			case BlackPawn:
				polyPiece = 0
			case WhitePawn:
				polyPiece = 1
			case BlackKnight:
				polyPiece = 2
			case WhiteKnight:
				polyPiece = 3
			case BlackBishop:
				polyPiece = 4
			case WhiteBishop:
				polyPiece = 5
			case BlackRook:
				polyPiece = 6
			case WhiteRook:
				polyPiece = 7
			case BlackQueen:
				polyPiece = 8
			case WhiteQueen:
				polyPiece = 9
			case BlackKing:
				polyPiece = 10
			case WhiteKing:
				polyPiece = 11
			}
			if polyPiece >= 0 {
				hash ^= polyglotRandom[64*polyPiece+sq]
			}
		}
	}

	// Castling rights
	if pos.HasTag(WhiteCanCastleKingSide) {
		hash ^= polyglotRandom[768]
	}
	if pos.HasTag(WhiteCanCastleQueenSide) {
		hash ^= polyglotRandom[769]
	}
	if pos.HasTag(BlackCanCastleKingSide) {
		hash ^= polyglotRandom[770]
	}
	if pos.HasTag(BlackCanCastleQueenSide) {
		hash ^= polyglotRandom[771]
	}

	// En passant uses the Polyglot book contract: hash the file when an
	// adjacent pawn could capture geometrically, without a king-safety test.
	if pos.EnPassant != NoSquare {
		epFile := int(pos.EnPassant) % 8
		// Check if there's actually a pawn that can capture
		if canCaptureEnPassant(pos) {
			hash ^= polyglotRandom[772+epFile]
		}
	}

	// Side to move (white = 1)
	if pos.Turn() == White {
		hash ^= polyglotRandom[780]
	}

	return hash
}

// canCaptureEnPassant checks the pseudo-legal adjacent-pawn condition required
// by Polyglot. It deliberately does not test whether king safety makes the
// capture legal for repetition identity.
func canCaptureEnPassant(pos *Position) bool {
	if pos.EnPassant == NoSquare {
		return false
	}

	epSquare := pos.EnPassant
	epFile := int(epSquare) % 8
	epRank := int(epSquare) / 8

	if pos.Turn() == White {
		// White pawns on rank 5 (index 4) can capture on rank 6
		if epRank == 5 { // Rank 6 in 0-indexed
			// Check left
			if epFile > 0 {
				leftSq := Square((epRank-1)*8 + epFile - 1)
				if pos.Board.PieceAt(leftSq) == WhitePawn {
					return true
				}
			}
			// Check right
			if epFile < 7 {
				rightSq := Square((epRank-1)*8 + epFile + 1)
				if pos.Board.PieceAt(rightSq) == WhitePawn {
					return true
				}
			}
		}
	} else {
		// Black pawns on rank 4 (index 3) can capture on rank 3
		if epRank == 2 { // Rank 3 in 0-indexed
			// Check left
			if epFile > 0 {
				leftSq := Square((epRank+1)*8 + epFile - 1)
				if pos.Board.PieceAt(leftSq) == BlackPawn {
					return true
				}
			}
			// Check right
			if epFile < 7 {
				rightSq := Square((epRank+1)*8 + epFile + 1)
				if pos.Board.PieceAt(rightSq) == BlackPawn {
					return true
				}
			}
		}
	}

	return false
}

// LoadPolyglotBook loads a Polyglot book from a .bin file
func LoadPolyglotBook(path string) (*PolyglotBook, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	numEntries := info.Size() / 16 // Each entry is 16 bytes
	entries := make([]PolyglotEntry, numEntries)

	for i := int64(0); i < numEntries; i++ {
		var entry PolyglotEntry
		binary.Read(file, binary.BigEndian, &entry.Key)
		binary.Read(file, binary.BigEndian, &entry.Move)
		binary.Read(file, binary.BigEndian, &entry.Weight)
		binary.Read(file, binary.BigEndian, &entry.Learn)
		entries[i] = entry
	}

	// Sort by key for binary search
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Key < entries[j].Key
	})

	return &PolyglotBook{
		entries: entries,
		path:    path,
	}, nil
}

// ProbeBook looks up a position in the book and returns a move
func (book *PolyglotBook) ProbeBook(pos *Position) (Move, bool) {
	if book == nil || len(book.entries) == 0 {
		return EmptyMove, false
	}

	hash := PolyglotHash(pos)

	// Binary search for first entry with matching key
	idx := sort.Search(len(book.entries), func(i int) bool {
		return book.entries[i].Key >= hash
	})

	if idx >= len(book.entries) || book.entries[idx].Key != hash {
		return EmptyMove, false
	}

	// Collect all moves with matching key
	var moves []PolyglotEntry
	for i := idx; i < len(book.entries) && book.entries[i].Key == hash; i++ {
		moves = append(moves, book.entries[i])
	}

	if len(moves) == 0 {
		return EmptyMove, false
	}

	// Select move by weight (higher weight = more likely)
	totalWeight := uint32(0)
	for _, m := range moves {
		totalWeight += uint32(m.Weight)
	}

	// For now, just pick the highest-weighted move (deterministic)
	// Could add randomness weighted by probability
	bestMove := moves[0]
	for _, m := range moves[1:] {
		if m.Weight > bestMove.Weight {
			bestMove = m
		}
	}

	// Convert Polyglot move to our move format
	move := polyglotMoveToMove(pos, bestMove.Move)
	if move == EmptyMove {
		return EmptyMove, false
	}

	return move, true
}

// polyglotMoveToMove converts a Polyglot move encoding to our Move type
func polyglotMoveToMove(pos *Position, polyMove uint16) Move {
	// Polyglot move format:
	// bits 0-5: destination square
	// bits 6-11: source square
	// bits 12-14: promotion piece (0=none, 1=knight, 2=bishop, 3=rook, 4=queen)

	toSq := Square(polyMove & 0x3F)
	fromSq := Square((polyMove >> 6) & 0x3F)
	promoPiece := (polyMove >> 12) & 0x7

	movingPiece := pos.Board.PieceAt(fromSq)

	if movingPiece == NoPiece {
		return EmptyMove
	}

	// Standard Polyglot encodes orthodox castling as king-to-own-rook.
	// Normalize that destination before capture and castle-tag classification;
	// retain the already-supported king-to-g/c representation as-is.
	if movingPiece.Type() == King && pos.Board.PieceAt(toSq) == GetPiece(Rook, movingPiece.Color()) {
		switch {
		case fromSq == E1 && toSq == H1:
			toSq = G1
		case fromSq == E1 && toSq == A1:
			toSq = C1
		case fromSq == E8 && toSq == H8:
			toSq = G8
		case fromSq == E8 && toSq == A8:
			toSq = C8
		}
	}
	capturedPiece := pos.Board.PieceAt(toSq)

	// Determine move flags
	var tag MoveTag
	var promoType PieceType

	// Check for capture
	if capturedPiece != NoPiece {
		tag |= Capture
	}

	// Check for en passant
	if movingPiece.Type() == Pawn && toSq == pos.EnPassant {
		tag |= EnPassant | Capture
		capturedPiece = GetPiece(Pawn, pos.Turn().Other())
	}

	// Check for castling
	if movingPiece.Type() == King {
		if fromSq == E1 && toSq == G1 {
			tag |= KingSideCastle
		} else if fromSq == E1 && toSq == C1 {
			tag |= QueenSideCastle
		} else if fromSq == E8 && toSq == G8 {
			tag |= KingSideCastle
		} else if fromSq == E8 && toSq == C8 {
			tag |= QueenSideCastle
		}
	}

	// Check for promotion
	if promoPiece > 0 {
		switch promoPiece {
		case 1:
			promoType = Knight
		case 2:
			promoType = Bishop
		case 3:
			promoType = Rook
		case 4:
			promoType = Queen
		}
	}

	return NewMove(fromSq, toSq, movingPiece, capturedPiece, promoType, tag)
}

// Size returns the number of entries in the book
func (book *PolyglotBook) Size() int {
	if book == nil {
		return 0
	}
	return len(book.entries)
}
