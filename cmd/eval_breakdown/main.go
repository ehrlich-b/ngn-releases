package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ehrlich-b/ngn/engine"
)

func main() {
	fenFlag := flag.String("fen", "", "FEN position to evaluate")
	flag.Parse()

	if *fenFlag == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s -fen <FEN>\n", os.Args[0])
		os.Exit(1)
	}

	// Parse the FEN position
	pos, err := engine.ParseFEN(*fenFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing FEN: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Position: %s\n", *fenFlag)
	fmt.Printf("Turn: %d\n\n", pos.Turn())

	// Get static evaluation
	eval := engine.Evaluate(&pos.Board)
	fmt.Printf("Static Evaluation: %+d\n\n", eval)

	// Show material count
	fmt.Println("Material:")
	whiteMaterial := countMaterial(pos, engine.White)
	blackMaterial := countMaterial(pos, engine.Black)
	fmt.Printf("  White: %d\n", whiteMaterial)
	fmt.Printf("  Black: %d\n", blackMaterial)
	fmt.Printf("  Difference: %+d\n\n", whiteMaterial-blackMaterial)

	// Note: Detailed breakdown would require exporting eval components from eval.go
	fmt.Println("Note: For detailed eval breakdown, would need to export individual")
	fmt.Println("evaluation components (PST, king safety, piece coordination, etc.)")
}

func countMaterial(pos *engine.Position, color engine.Color) int {
	material := 0
	// Pawn = 100, Knight = 320, Bishop = 330, Rook = 500, Queen = 900

	var pawns, knights, bishops, rooks, queens uint64
	if color == engine.White {
		pawns = pos.Board.GetBitboardOf(engine.WhitePawn)
		knights = pos.Board.GetBitboardOf(engine.WhiteKnight)
		bishops = pos.Board.GetBitboardOf(engine.WhiteBishop)
		rooks = pos.Board.GetBitboardOf(engine.WhiteRook)
		queens = pos.Board.GetBitboardOf(engine.WhiteQueen)
	} else {
		pawns = pos.Board.GetBitboardOf(engine.BlackPawn)
		knights = pos.Board.GetBitboardOf(engine.BlackKnight)
		bishops = pos.Board.GetBitboardOf(engine.BlackBishop)
		rooks = pos.Board.GetBitboardOf(engine.BlackRook)
		queens = pos.Board.GetBitboardOf(engine.BlackQueen)
	}

	material += popcount(pawns) * 100
	material += popcount(knights) * 320
	material += popcount(bishops) * 330
	material += popcount(rooks) * 500
	material += popcount(queens) * 900

	return material
}

func popcount(bb uint64) int {
	count := 0
	for bb != 0 {
		bb &= bb - 1
		count++
	}
	return count
}
