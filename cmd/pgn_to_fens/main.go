package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/ehrlich-b/ngn/engine"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <pgn_file>\n", os.Args[0])
		os.Exit(1)
	}

	file, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	// Start with initial position
	pos := &engine.Position{
		Board:     engine.StartingBoard(),
		Tag:       engine.WhiteCanCastleKingSide | engine.WhiteCanCastleQueenSide | engine.BlackCanCastleKingSide | engine.BlackCanCastleQueenSide | engine.WhiteToMove,
		EnPassant: engine.NoSquare,
	}

	moveNum := 1
	fmt.Printf("Move 0 (Initial): %s\n", engine.GenerateFEN(pos))

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip header lines and empty lines
		if strings.HasPrefix(line, "[") || line == "" {
			continue
		}

		// Remove move numbers, comments, and result
		moves := extractMoves(line)

		for _, moveStr := range moves {
			if moveStr == "" {
				continue
			}

			// Parse and make the move
			move, err := parseMove(pos, moveStr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing move %s: %v\n", moveStr, err)
				continue
			}

			// Make the move
			pos.MakeMove(move)

			// Output position
			fmt.Printf("Move %d: %s -> %s\n", moveNum, moveStr, engine.GenerateFEN(pos))
			moveNum++
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}
}

// extractMoves extracts just the moves from a PGN line, removing annotations and comments
func extractMoves(line string) []string {
	// Remove all comments in braces (including nested evaluations)
	for strings.Contains(line, "{") {
		start := strings.Index(line, "{")
		if start == -1 {
			break
		}

		// Find the matching closing brace
		braceCount := 1
		end := start + 1
		for end < len(line) && braceCount > 0 {
			if line[end] == '{' {
				braceCount++
			} else if line[end] == '}' {
				braceCount--
			}
			end++
		}

		if braceCount == 0 {
			line = line[:start] + " " + line[end:]
		} else {
			// Malformed braces, remove everything from start
			line = line[:start]
			break
		}
	}

	// Remove move numbers (e.g., "1.", "2.", etc.)
	re := regexp.MustCompile(`\d+\.`)
	line = re.ReplaceAllString(line, " ")

	// Remove result
	line = strings.Replace(line, "0-1", "", -1)
	line = strings.Replace(line, "1-0", "", -1)
	line = strings.Replace(line, "1/2-1/2", "", -1)

	// Split on whitespace and filter empty strings
	parts := strings.Fields(line)
	var moves []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		// Skip if empty or looks like an evaluation score
		if part != "" && !strings.Contains(part, "/") && !strings.HasPrefix(part, "+") && !strings.HasPrefix(part, "-") {
			moves = append(moves, part)
		}
	}

	return moves
}

// parseMove converts algebraic notation to engine.Move
func parseMove(pos *engine.Position, moveStr string) (engine.Move, error) {
	// Generate legal moves
	legalMoves := engine.GenerateLegalMoves(pos)

	// Try to find matching move
	for _, move := range legalMoves {
		if matchesAlgebraic(pos, move, moveStr) {
			return move, nil
		}
	}

	return engine.EmptyMove, fmt.Errorf("no legal move matches %s", moveStr)
}

// matchesAlgebraic checks if a move matches the algebraic notation
func matchesAlgebraic(pos *engine.Position, move engine.Move, algebraic string) bool {
	// Remove check/checkmate indicators
	algebraic = strings.TrimSuffix(algebraic, "+")
	algebraic = strings.TrimSuffix(algebraic, "#")

	from := move.Source()
	to := move.Destination()
	piece := move.MovingPiece()

	// Handle castling
	if algebraic == "O-O" || algebraic == "O-O-O" {
		return move.IsCastle()
	}

	// Handle pawn moves
	if piece.Type() == engine.Pawn {
		// Simple pawn move (e4, d5, etc.)
		if len(algebraic) == 2 {
			return to.Name() == algebraic
		}

		// Pawn capture (exd4, etc.)
		if len(algebraic) == 4 && strings.Contains(algebraic, "x") {
			fromFile := string(algebraic[0])
			toSquare := algebraic[2:]
			return from.File().Name() == fromFile && to.Name() == toSquare
		}

		// Pawn promotion (e8=Q, etc.)
		if strings.Contains(algebraic, "=") {
			parts := strings.Split(algebraic, "=")
			if len(parts) == 2 {
				return to.Name() == parts[0] && move.PromoType() != engine.NoType
			}
		}
	}

	// Handle piece moves (Nf3, Bxc4, etc.)
	if len(algebraic) >= 3 {
		pieceChar := string(algebraic[0])
		expectedType := engine.Pawn

		switch pieceChar {
		case "N":
			expectedType = engine.Knight
		case "B":
			expectedType = engine.Bishop
		case "R":
			expectedType = engine.Rook
		case "Q":
			expectedType = engine.Queen
		case "K":
			expectedType = engine.King
		default:
			return false // Invalid piece character
		}

		if piece.Type() != expectedType {
			return false
		}

		// Extract destination square
		var toSquare string
		if strings.Contains(algebraic, "x") {
			parts := strings.Split(algebraic, "x")
			toSquare = parts[len(parts)-1]
		} else {
			toSquare = algebraic[1:]
		}

		return to.Name() == toSquare
	}

	return false
}
