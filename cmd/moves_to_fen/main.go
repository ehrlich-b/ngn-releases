package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/ehrlich-b/ngn/engine"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <move_number> <moves...>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example: %s 36 e2e4 e7e5 g1f3...\n", os.Args[0])
		os.Exit(1)
	}

	targetMove, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid move number: %s\n", os.Args[1])
		os.Exit(1)
	}

	moves := os.Args[2:]

	pos, err := engine.ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse starting position: %v\n", err)
		os.Exit(1)
	}

	for i, moveStr := range moves {
		if i == targetMove {
			fmt.Printf("FEN at move %d (after %s):\n%s\n", targetMove, moves[i-1], engine.GenerateFEN(pos))
			return
		}

		// Parse UCI move
		if len(moveStr) < 4 {
			fmt.Fprintf(os.Stderr, "Invalid move format: %s\n", moveStr)
			os.Exit(1)
		}

		from, err := engine.ParseSquare(moveStr[0:2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid from square: %s\n", moveStr[0:2])
			os.Exit(1)
		}

		to, err := engine.ParseSquare(moveStr[2:4])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid to square: %s\n", moveStr[2:4])
			os.Exit(1)
		}

		// Generate legal moves
		legalMoves := make([]engine.Move, 256)
		count := engine.GenerateMovesIntoBuffer(pos, legalMoves)
		legalMoves = legalMoves[:count]

		// Find the matching move
		var foundMove engine.Move
		found := false
		for _, m := range legalMoves {
			if m.Source() == from && m.Destination() == to {
				// Handle promotion
				if len(moveStr) == 5 {
					promoPiece := parsePromotion(moveStr[4])
					if m.PromoType() == promoPiece {
						foundMove = m
						found = true
						break
					}
				} else {
					foundMove = m
					found = true
					break
				}
			}
		}

		if !found {
			fmt.Fprintf(os.Stderr, "Illegal move at position %d: %s\n", i+1, moveStr)
			fmt.Fprintf(os.Stderr, "Current FEN: %s\n", engine.GenerateFEN(pos))
			os.Exit(1)
		}

		pos.MakeMove(foundMove)
	}

	// If we get here, targetMove >= len(moves)
	fmt.Printf("FEN at end of game (move %d):\n%s\n", len(moves), engine.GenerateFEN(pos))
}

func parsePromotion(c byte) engine.PieceType {
	switch strings.ToLower(string(c)) {
	case "q":
		return engine.Queen
	case "r":
		return engine.Rook
	case "b":
		return engine.Bishop
	case "n":
		return engine.Knight
	default:
		return engine.NoType
	}
}