package engine

import (
	"fmt"
	"strings"
)

// ParseAlgebraicMove parses a move in algebraic notation (e.g., "e2e4", "e7e8q")
// and returns the corresponding Move for the given position
func ParseAlgebraicMove(moveStr string, pos *Position) (Move, error) {
	moveStr = strings.ToLower(moveStr)

	if len(moveStr) < 4 {
		return EmptyMove, fmt.Errorf("move string too short: %s", moveStr)
	}

	// Parse source square
	fromSquare, err := ParseSquare(moveStr[0:2])
	if err != nil {
		return EmptyMove, fmt.Errorf("invalid from square: %v", err)
	}

	// Parse destination square
	toSquare, err := ParseSquare(moveStr[2:4])
	if err != nil {
		return EmptyMove, fmt.Errorf("invalid to square: %v", err)
	}

	// Get the piece at the source square
	movingPiece := pos.Board.PieceAt(fromSquare)
	if movingPiece == NoPiece {
		return EmptyMove, fmt.Errorf("no piece at square %s", moveStr[0:2])
	}

	// Check if it's the right color's turn
	if movingPiece.Color() != pos.Turn() {
		return EmptyMove, fmt.Errorf("wrong color piece at %s", moveStr[0:2])
	}

	// Get captured piece (if any)
	capturedPiece := pos.Board.PieceAt(toSquare)

	// Parse promotion (if present)
	var promoType PieceType = NoType
	if len(moveStr) == 5 {
		switch moveStr[4] {
		case 'q':
			promoType = Queen
		case 'r':
			promoType = Rook
		case 'b':
			promoType = Bishop
		case 'n':
			promoType = Knight
		default:
			return EmptyMove, fmt.Errorf("invalid promotion piece: %c", moveStr[4])
		}
	}

	// Determine move flags
	var moveTag MoveTag = 0

	// Check for castling
	if movingPiece.Type() == King {
		fromFile := int(fromSquare % 8)
		toFile := int(toSquare % 8)

		if fromFile == 4 && toFile == 6 { // King side castle
			moveTag |= KingSideCastle
		} else if fromFile == 4 && toFile == 2 { // Queen side castle
			moveTag |= QueenSideCastle
		}
	}

	// Check for en passant
	if movingPiece.Type() == Pawn && capturedPiece == NoPiece && toSquare == pos.EnPassant {
		moveTag |= EnPassant
		// In en passant, we capture the pawn behind the target square
		if pos.Turn() == White {
			capturedPiece = BlackPawn
		} else {
			capturedPiece = WhitePawn
		}
	}

	// Check for regular capture
	if capturedPiece != NoPiece {
		moveTag |= Capture
	}

	// Create the move
	move := NewMove(fromSquare, toSquare, movingPiece, capturedPiece, promoType, moveTag)

	// Validate that this move is legal
	legalMoves := GenerateLegalMoves(pos)
	for _, legalMove := range legalMoves {
		if move == legalMove {
			return move, nil
		}
	}

	return EmptyMove, fmt.Errorf("illegal move: %s", moveStr)
}

// MoveToAlgebraic converts a Move to algebraic notation
func MoveToAlgebraic(move Move) string {
	result := move.Source().Name() + move.Destination().Name()

	// Add promotion piece if any
	if move.PromoType() != NoType {
		switch move.PromoType() {
		case Queen:
			result += "q"
		case Rook:
			result += "r"
		case Bishop:
			result += "b"
		case Knight:
			result += "n"
		}
	}

	return result
}

// MoveToSAN renders move in Standard Algebraic Notation in the context of pos (the
// position BEFORE the move is made). It handles castling, captures (incl. en passant),
// pawn promotion, source disambiguation, and check/checkmate suffixes — real PGN
// movetext the Lichess importer accepts, unlike MoveToAlgebraic's UCI long-form.
func MoveToSAN(move Move, pos *Position) string {
	src := move.Source()
	dst := move.Destination()
	pt := pos.Board.PieceAt(src).Type()

	if move.IsCastle() {
		s := "O-O"
		if dst.Name()[0] == 'c' { // king lands on the c-file for queenside
			s = "O-O-O"
		}
		return sanCheckSuffix(s, move, pos)
	}

	capture := move.IsCapture() || move.IsEnPassant()
	var s string
	if pt == Pawn {
		if capture {
			s = src.Name()[0:1] + "x" + dst.Name()
		} else {
			s = dst.Name()
		}
		if move.PromoType() != NoType {
			s += "=" + sanPieceLetter(move.PromoType())
		}
	} else {
		s = sanPieceLetter(pt) + sanDisambiguation(move, pos, pt, src, dst)
		if capture {
			s += "x"
		}
		s += dst.Name()
	}
	return sanCheckSuffix(s, move, pos)
}

func sanPieceLetter(pt PieceType) string {
	switch pt {
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
	return ""
}

// sanDisambiguation returns the minimal source qualifier (file, rank, or whole
// square) required when another same-type piece can also legally reach dst.
func sanDisambiguation(move Move, pos *Position, pt PieceType, src, dst Square) string {
	var others []Square
	for _, m := range GenerateLegalMoves(pos) {
		if m.Destination() == dst && m.Source() != src && pos.Board.PieceAt(m.Source()).Type() == pt {
			others = append(others, m.Source())
		}
	}
	if len(others) == 0 {
		return ""
	}
	srcName := src.Name()
	sameFile, sameRank := false, false
	for _, o := range others {
		on := o.Name()
		if on[0] == srcName[0] {
			sameFile = true
		}
		if on[1] == srcName[1] {
			sameRank = true
		}
	}
	if !sameFile {
		return srcName[0:1] // file alone disambiguates
	}
	if !sameRank {
		return srcName[1:2] // rank disambiguates
	}
	return srcName // need the whole source square
}

// sanCheckSuffix appends "+" (check) or "#" (checkmate) by trying the move on pos
// and restoring it.
func sanCheckSuffix(s string, move Move, pos *Position) string {
	ep, tag, hc, ok := pos.GameMakeMove(move)
	if !ok {
		return s
	}
	check := pos.IsInCheck()
	mate := check && len(GenerateLegalMoves(pos)) == 0
	pos.GameUnMakeMove(move, tag, ep, hc)
	if mate {
		return s + "#"
	}
	if check {
		return s + "+"
	}
	return s
}

// ParseLongAlgebraicNotation parses moves in long algebraic notation
// This is an alias for ParseAlgebraicMove for clarity
func ParseLongAlgebraicNotation(moveStr string, pos *Position) (Move, error) {
	return ParseAlgebraicMove(moveStr, pos)
}

// ValidateAlgebraicMove checks if an algebraic move string is valid for the position
func ValidateAlgebraicMove(moveStr string, pos *Position) error {
	_, err := ParseAlgebraicMove(moveStr, pos)
	return err
}
