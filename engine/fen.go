package engine

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseFEN parses a FEN (Forsyth-Edwards Notation) string and returns a Position
func ParseFEN(fenString string) (*Position, error) {
	parts := strings.Fields(fenString)
	if len(parts) != 6 {
		return nil, fmt.Errorf("FEN must have exactly 6 parts, got %d", len(parts))
	}

	position := &Position{}
	position.Board = Bitboard{}
	position.Positions = make(map[uint64]int)

	// 1. Parse piece placement
	err := parsePiecePlacement(parts[0], &position.Board)
	if err != nil {
		return nil, fmt.Errorf("invalid piece placement: %v", err)
	}

	// 2. Parse active color
	switch parts[1] {
	case "w":
		position.SetTag(WhiteToMove)
	case "b":
		position.SetTag(BlackToMove)
	default:
		return nil, fmt.Errorf("invalid active color: %s", parts[1])
	}

	// 3. Parse castling rights
	err = parseCastlingRights(parts[2], position)
	if err != nil {
		return nil, fmt.Errorf("invalid castling rights: %v", err)
	}

	// 4. Parse en passant square
	if parts[3] == "-" {
		position.EnPassant = NoSquare
	} else {
		square, err := ParseSquare(parts[3])
		if err != nil {
			return nil, fmt.Errorf("invalid en passant square: %v", err)
		}
		position.EnPassant = square
		// Drop a non-capturable EP target so a FEN that lists a phantom EP square
		// (no enemy pawn able to capture) hashes identically to the same position
		// reached by playing moves, where the EP target is gated the same way.
		if !canCaptureEnPassant(position) {
			position.EnPassant = NoSquare
		}
	}
	// Preserve the raw adjacency/X-FEN target above for move generation,
	// while recording separately whether it is legal and may
	// distinguish the ordinary repetition/TT key.
	position.refreshEnPassantHash()

	// 5. Parse halfmove clock
	halfmove, err := strconv.Atoi(parts[4])
	if err != nil {
		return nil, fmt.Errorf("invalid halfmove clock: %v", err)
	}
	if halfmove < 0 || halfmove > 255 {
		return nil, fmt.Errorf("halfmove clock out of range: %d", halfmove)
	}
	position.HalfMoveClock = uint8(halfmove)

	// 6. Parse fullmove number (we don't store this currently, but validate it)
	_, err = strconv.Atoi(parts[5])
	if err != nil {
		return nil, fmt.Errorf("invalid fullmove number: %v", err)
	}

	// Add initial position to repetition tracking
	position.positionsMutex.Lock()
	position.Positions[position.Hash()] = 1
	position.positionsMutex.Unlock()

	// Set InCheck flag if the side to move is in check
	if isInCheck(position, position.Turn()) {
		position.SetTag(InCheck)
	}

	return position, nil
}

// parsePiecePlacement parses the piece placement part of FEN
func parsePiecePlacement(placement string, board *Bitboard) error {
	ranks := strings.Split(placement, "/")
	if len(ranks) != 8 {
		return fmt.Errorf("must have exactly 8 ranks, got %d", len(ranks))
	}

	for rankIndex, rankStr := range ranks {
		rank := 7 - rankIndex // FEN starts from rank 8, we start from rank 0
		file := 0

		for _, char := range rankStr {
			if char >= '1' && char <= '8' {
				// Empty squares
				emptySquares := int(char - '0')
				file += emptySquares
			} else {
				// Piece
				piece := pieceFromName(char)
				if piece == NoPiece {
					return fmt.Errorf("invalid piece character: %c", char)
				}

				if file >= 8 {
					return fmt.Errorf("too many pieces/spaces in rank %d", rankIndex+1)
				}

				square := Square(rank*8 + file)
				board.UpdateSquare(square, piece, NoPiece)
				file++
			}
		}

		if file != 8 {
			return fmt.Errorf("rank %d has %d files instead of 8", rankIndex+1, file)
		}
	}

	return nil
}

// parseCastlingRights parses the castling rights part of FEN
func parseCastlingRights(castling string, position *Position) error {
	if castling == "-" {
		// No castling rights
		return nil
	}

	for _, char := range castling {
		switch char {
		case 'K':
			position.SetTag(WhiteCanCastleKingSide)
		case 'Q':
			position.SetTag(WhiteCanCastleQueenSide)
		case 'k':
			position.SetTag(BlackCanCastleKingSide)
		case 'q':
			position.SetTag(BlackCanCastleQueenSide)
		default:
			return fmt.Errorf("invalid castling character: %c", char)
		}
	}

	return nil
}

// GenerateFEN generates a FEN string from a Position
func GenerateFEN(position *Position) string {
	var parts []string

	// 1. Piece placement
	parts = append(parts, generatePiecePlacement(&position.Board))

	// 2. Active color
	if position.Turn() == White {
		parts = append(parts, "w")
	} else {
		parts = append(parts, "b")
	}

	// 3. Castling rights
	parts = append(parts, generateCastlingRights(position))

	// 4. En passant square
	if position.EnPassant == NoSquare {
		parts = append(parts, "-")
	} else {
		parts = append(parts, position.EnPassant.Name())
	}

	// 5. Halfmove clock
	parts = append(parts, strconv.Itoa(int(position.HalfMoveClock)))

	// 6. Fullmove number (simplified - always 1 for now)
	parts = append(parts, "1")

	return strings.Join(parts, " ")
}

// generatePiecePlacement generates the piece placement part of FEN
func generatePiecePlacement(board *Bitboard) string {
	var ranks []string

	for rankIndex := 7; rankIndex >= 0; rankIndex-- { // Start from rank 8
		var rankStr strings.Builder
		emptyCount := 0

		for file := 0; file < 8; file++ {
			square := Square(rankIndex*8 + file)
			piece := board.PieceAt(square)

			if piece == NoPiece {
				emptyCount++
			} else {
				// Output empty squares count if any
				if emptyCount > 0 {
					rankStr.WriteString(strconv.Itoa(emptyCount))
					emptyCount = 0
				}
				// Output piece
				rankStr.WriteString(piece.Name())
			}
		}

		// Output remaining empty squares count if any
		if emptyCount > 0 {
			rankStr.WriteString(strconv.Itoa(emptyCount))
		}

		ranks = append(ranks, rankStr.String())
	}

	return strings.Join(ranks, "/")
}

// generateCastlingRights generates the castling rights part of FEN
func generateCastlingRights(position *Position) string {
	var rights strings.Builder

	if position.HasTag(WhiteCanCastleKingSide) {
		rights.WriteString("K")
	}
	if position.HasTag(WhiteCanCastleQueenSide) {
		rights.WriteString("Q")
	}
	if position.HasTag(BlackCanCastleKingSide) {
		rights.WriteString("k")
	}
	if position.HasTag(BlackCanCastleQueenSide) {
		rights.WriteString("q")
	}

	if rights.Len() == 0 {
		return "-"
	}

	return rights.String()
}

// ParseSquare parses a square name like "e4" into a Square
func ParseSquare(name string) (Square, error) {
	if len(name) != 2 {
		return NoSquare, fmt.Errorf("square name must be 2 characters, got %d", len(name))
	}

	file := name[0]
	rank := name[1]

	if file < 'a' || file > 'h' {
		return NoSquare, fmt.Errorf("invalid file: %c", file)
	}

	if rank < '1' || rank > '8' {
		return NoSquare, fmt.Errorf("invalid rank: %c", rank)
	}

	fileIndex := int(file - 'a')
	rankIndex := int(rank - '1')

	return Square(rankIndex*8 + fileIndex), nil
}
