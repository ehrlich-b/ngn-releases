package engine

import (
	"fmt"
	"testing"
	"time"
)

func TestBasicMoveGeneration(t *testing.T) {
	// Test move generation from starting position
	pos := &Position{
		Board:     StartingBoard(),
		Tag:       WhiteCanCastleKingSide | WhiteCanCastleQueenSide | BlackCanCastleKingSide | BlackCanCastleQueenSide | WhiteToMove,
		EnPassant: NoSquare, // Make sure en passant is properly initialized
	}

	moves := GenerateMoves(pos)

	// Starting position should have exactly 20 legal moves
	if len(moves) != 20 {
		t.Errorf("Starting position should have 20 moves, got %d", len(moves))
	}

	// Count move types
	pawnMoves := 0
	knightMoves := 0
	for _, move := range moves {
		switch move.MovingPiece() {
		case WhitePawn:
			pawnMoves++
		case WhiteKnight:
			knightMoves++
		}
	}

	// Should have 16 pawn moves (8 single pushes, 8 double pushes)
	if pawnMoves != 16 {
		t.Errorf("Expected 16 pawn moves, got %d", pawnMoves)
	}

	// Should have 4 knight moves (2 knights × 2 moves each)
	if knightMoves != 4 {
		t.Errorf("Expected 4 knight moves, got %d", knightMoves)
	}
}

func TestPawnMoves(t *testing.T) {
	tests := []struct {
		name        string
		setupBoard  func() *Position
		expectedMin int
		expectedMax int
	}{
		{
			name: "Starting position white pawns",
			setupBoard: func() *Position {
				return &Position{
					Board:     StartingBoard(),
					Tag:       WhiteToMove,
					EnPassant: NoSquare,
				}
			},
			expectedMin: 16,
			expectedMax: 16,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos := tt.setupBoard()
			moves := generateWhitePawnMoves(pos)

			if len(moves) < tt.expectedMin || len(moves) > tt.expectedMax {
				t.Errorf("%s: expected %d-%d pawn moves, got %d",
					tt.name, tt.expectedMin, tt.expectedMax, len(moves))
			}
		})
	}
}

func TestKnightMoves(t *testing.T) {
	// Create position with single knight on E4
	pos := &Position{
		Board:     Bitboard{},
		Tag:       WhiteToMove,
		EnPassant: NoSquare,
	}
	pos.Board.UpdateSquare(E4, WhiteKnight, NoPiece)

	moves := generateKnightMoves(pos, E4)

	// Knight on E4 should have 8 possible moves
	expected := 8
	if len(moves) != expected {
		t.Errorf("Knight on E4 should have %d moves, got %d", expected, len(moves))
	}

	// Test moves are correct
	expectedSquares := []Square{D2, F2, C3, G3, C5, G5, D6, F6}
	foundSquares := make([]Square, len(moves))
	for i, move := range moves {
		foundSquares[i] = move.Destination()
	}

	for _, expectedSq := range expectedSquares {
		found := false
		for _, foundSq := range foundSquares {
			if foundSq == expectedSq {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected knight move to %s not found", expectedSq.Name())
		}
	}
}

func TestBishopMoves(t *testing.T) {
	// Create position with single bishop on E4
	pos := &Position{
		Board:     Bitboard{},
		Tag:       WhiteToMove,
		EnPassant: NoSquare,
	}
	pos.Board.UpdateSquare(E4, WhiteBishop, NoPiece)

	moves := generateBishopMoves(pos, E4)

	// Bishop on E4 should have 13 possible moves (4 directions, varying lengths)
	expected := 13
	if len(moves) != expected {
		t.Errorf("Bishop on E4 should have %d moves, got %d", expected, len(moves))
	}
}

func TestRookMoves(t *testing.T) {
	// Create position with single rook on E4
	pos := &Position{
		Board:     Bitboard{},
		Tag:       WhiteToMove,
		EnPassant: NoSquare,
	}
	pos.Board.UpdateSquare(E4, WhiteRook, NoPiece)

	moves := generateRookMoves(pos, E4)

	// Rook on E4 should have 14 possible moves (4 directions, varying lengths)
	expected := 14
	if len(moves) != expected {
		t.Errorf("Rook on E4 should have %d moves, got %d", expected, len(moves))
	}
}

func TestQueenMoves(t *testing.T) {
	// Create position with single queen on E4
	pos := &Position{
		Board:     Bitboard{},
		Tag:       WhiteToMove,
		EnPassant: NoSquare,
	}
	pos.Board.UpdateSquare(E4, WhiteQueen, NoPiece)

	moves := generateQueenMoves(pos, E4)

	// Queen on E4 should have 27 possible moves (combines bishop + rook moves)
	expected := 27
	if len(moves) != expected {
		t.Errorf("Queen on E4 should have %d moves, got %d", expected, len(moves))
	}
}

func TestKingMoves(t *testing.T) {
	// Create position with single king on E4
	pos := &Position{
		Board:     Bitboard{},
		Tag:       WhiteToMove,
		EnPassant: NoSquare,
	}
	pos.Board.UpdateSquare(E4, WhiteKing, NoPiece)

	moves := generateKingMoves(pos, WhiteKing)

	// King on E4 should have 8 possible moves
	expected := 8
	if len(moves) != expected {
		t.Errorf("King on E4 should have %d moves, got %d", expected, len(moves))
	}
}

func TestCastlingMoves(t *testing.T) {
	// Test white castling
	pos := &Position{
		Board: Bitboard{},
		Tag:   WhiteToMove | WhiteCanCastleKingSide | WhiteCanCastleQueenSide,
	}
	// Set up for castling: King on E1, Rooks on A1 and H1
	pos.Board.UpdateSquare(E1, WhiteKing, NoPiece)
	pos.Board.UpdateSquare(A1, WhiteRook, NoPiece)
	pos.Board.UpdateSquare(H1, WhiteRook, NoPiece)

	moves := generateKingMoves(pos, WhiteKing)

	// Should include castling moves
	castlingMoves := 0
	for _, move := range moves {
		if move.IsCastle() {
			castlingMoves++
		}
	}

	if castlingMoves != 2 {
		t.Errorf("Expected 2 castling moves, got %d", castlingMoves)
	}
}

func TestMoveWithCaptures(t *testing.T) {
	// Create position with white pawn that can capture
	pos := &Position{
		Board:     Bitboard{},
		Tag:       WhiteToMove,
		EnPassant: NoSquare,
	}
	pos.Board.UpdateSquare(E4, WhitePawn, NoPiece)
	pos.Board.UpdateSquare(D5, BlackPawn, NoPiece)
	pos.Board.UpdateSquare(F5, BlackKnight, NoPiece)

	moves := generateWhitePawnMoves(pos)

	// Should have: 1 push forward + 2 captures
	expected := 3
	if len(moves) != expected {
		t.Errorf("Expected %d pawn moves, got %d", expected, len(moves))
	}

	// Count captures
	captures := 0
	for _, move := range moves {
		if move.IsCapture() {
			captures++
		}
	}

	if captures != 2 {
		t.Errorf("Expected 2 captures, got %d", captures)
	}
}

func TestEnPassant(t *testing.T) {
	// Create position with en passant possibility
	pos := &Position{
		Board:     Bitboard{},
		Tag:       WhiteToMove,
		EnPassant: D6, // Black pawn just moved D7-D5
	}
	pos.Board.UpdateSquare(C5, WhitePawn, NoPiece)
	pos.Board.UpdateSquare(E5, WhitePawn, NoPiece)
	pos.Board.UpdateSquare(D5, BlackPawn, NoPiece)

	moves := generateWhitePawnMoves(pos)

	// Count en passant moves
	enPassantMoves := 0
	for _, move := range moves {
		if move.IsEnPassant() {
			enPassantMoves++
		}
	}

	// Should have 2 en passant captures (from C5 and E5)
	if enPassantMoves != 2 {
		t.Errorf("Expected 2 en passant moves, got %d", enPassantMoves)
	}
}

func TestPromotionMoves(t *testing.T) {
	// Create position with pawn about to promote
	pos := &Position{
		Board:     Bitboard{},
		Tag:       WhiteToMove,
		EnPassant: NoSquare,
	}
	pos.Board.UpdateSquare(E7, WhitePawn, NoPiece)

	moves := generateWhitePawnMoves(pos)

	// Should generate 4 promotion moves (Q, R, B, N)
	expected := 4
	if len(moves) != expected {
		t.Errorf("Expected %d promotion moves, got %d", expected, len(moves))
	}

	// Check all promotion types are present
	promotionTypes := make(map[PieceType]bool)
	for _, move := range moves {
		promotionTypes[move.PromoType()] = true
	}

	expectedTypes := []PieceType{Queen, Rook, Bishop, Knight}
	for _, pieceType := range expectedTypes {
		if !promotionTypes[pieceType] {
			t.Errorf("Missing promotion to %v", pieceType)
		}
	}
}

func TestPerftStartingPosition(t *testing.T) {
	// Test perft for starting position
	pos := &Position{
		Board:     StartingBoard(),
		Tag:       WhiteCanCastleKingSide | WhiteCanCastleQueenSide | BlackCanCastleKingSide | BlackCanCastleQueenSide | WhiteToMove,
		EnPassant: NoSquare,
	}

	// Test depth 1 - should be 20 moves
	result := RunPerftTest(pos, 1)
	expected := uint64(20)
	if result.Nodes != expected {
		t.Errorf("Perft depth 1: expected %d nodes, got %d", expected, result.Nodes)
	}

	// Test depth 2 - should be 400 moves
	result = RunPerftTest(pos, 2)
	expected = uint64(400)
	if result.Nodes != expected {
		t.Errorf("Perft depth 2: expected %d nodes, got %d", expected, result.Nodes)
	}
}

func TestPerftStandardSuite(t *testing.T) {
	for _, testCase := range StandardPerftTests {
		t.Run(testCase.Name, func(t *testing.T) {
			pos, err := ParseFEN(testCase.FEN)
			if err != nil {
				t.Fatalf("Failed to parse FEN %s: %v", testCase.FEN, err)
			}

			// Test each expected depth
			for _, expected := range testCase.Expected {
				t.Run(fmt.Sprintf("Depth%d", expected.Depth), func(t *testing.T) {
					start := time.Now()
					result := RunPerftTest(pos, expected.Depth)
					duration := time.Since(start)

					if result.Nodes != expected.Nodes {
						t.Errorf("Perft %s depth %d: expected %d nodes, got %d nodes",
							testCase.Name, expected.Depth, expected.Nodes, result.Nodes)
					}

					// Report performance for insight
					if duration > time.Millisecond {
						nps := float64(result.Nodes) / duration.Seconds()
						t.Logf("Performance: %d nodes in %v (%.0f NPS)",
							result.Nodes, duration, nps)
					}
				})
			}
		})
	}
}

func BenchmarkMoveGeneration(b *testing.B) {
	pos := &Position{
		Board:     StartingBoard(),
		Tag:       WhiteCanCastleKingSide | WhiteCanCastleQueenSide | BlackCanCastleKingSide | BlackCanCastleQueenSide | WhiteToMove,
		EnPassant: NoSquare,
	}

	benchmarkFunction(b, func() {
		GenerateMoves(pos)
	})
}

func TestLegalMoveValidation(t *testing.T) {
	// Test position where king is in check - should filter out moves that don't address check
	pos := &Position{
		Board:     Bitboard{},
		Tag:       WhiteToMove,
		EnPassant: NoSquare,
	}

	// Set up position: White king on E1, Black rook on E8 giving check
	pos.Board.UpdateSquare(E1, WhiteKing, NoPiece)
	pos.Board.UpdateSquare(E8, BlackRook, NoPiece)
	pos.Board.UpdateSquare(D1, WhiteQueen, NoPiece) // White queen to potentially block or capture

	pseudoLegalMoves := GenerateMoves(pos)
	legalMoves := GenerateLegalMoves(pos)

	// Should have fewer legal moves than pseudo-legal moves due to check
	if len(legalMoves) >= len(pseudoLegalMoves) {
		t.Errorf("Expected fewer legal moves than pseudo-legal moves in check position")
	}

	// All legal moves should actually be legal
	for _, move := range legalMoves {
		if !IsLegalMove(pos, move) {
			t.Errorf("GenerateLegalMoves returned an illegal move: %s", move.ToString())
		}
	}
}

func TestLegalMovesStartingPosition(t *testing.T) {
	// Test starting position - all pseudo-legal moves should be legal
	pos := &Position{
		Board:     StartingBoard(),
		Tag:       WhiteCanCastleKingSide | WhiteCanCastleQueenSide | BlackCanCastleKingSide | BlackCanCastleQueenSide | WhiteToMove,
		EnPassant: NoSquare,
	}

	pseudoLegalMoves := GenerateMoves(pos)
	legalMoves := GenerateLegalMoves(pos)

	// In starting position, all pseudo-legal moves should be legal
	if len(legalMoves) != len(pseudoLegalMoves) {
		t.Errorf("Starting position: expected %d legal moves, got %d", len(pseudoLegalMoves), len(legalMoves))
	}

	// Verify all moves are actually legal
	for _, move := range legalMoves {
		if !IsLegalMove(pos, move) {
			t.Errorf("GenerateLegalMoves returned an illegal move: %s", move.ToString())
		}
	}
}

func TestPinnedPieceMoves(t *testing.T) {
	// Test position with a pinned piece
	pos := &Position{
		Board:     Bitboard{},
		Tag:       WhiteToMove,
		EnPassant: NoSquare,
	}

	// Set up position: White king on E1, white bishop on D2, black queen on C3
	// Bishop is pinned and cannot move without exposing king to check
	pos.Board.UpdateSquare(E1, WhiteKing, NoPiece)
	pos.Board.UpdateSquare(D2, WhiteBishop, NoPiece)
	pos.Board.UpdateSquare(C3, BlackQueen, NoPiece)

	pseudoLegalMoves := GenerateMoves(pos)
	legalMoves := GenerateLegalMoves(pos)

	// Should have fewer legal moves due to pinned bishop
	if len(legalMoves) >= len(pseudoLegalMoves) {
		t.Errorf("Expected fewer legal moves than pseudo-legal moves with pinned piece")
	}

	// Check that pinned bishop moves are filtered out
	for _, move := range legalMoves {
		if move.MovingPiece() == WhiteBishop && move.Source() == D2 {
			// Bishop can only move along the pin line (C3-E1 diagonal)
			// Valid moves: C1 (along pin line) or C3 (capture attacker)
			if move.Destination() != C1 && move.Destination() != C3 {
				t.Errorf("Pinned bishop should only be able to block or capture along pin line, but found move to %s", move.Destination().Name())
			}
		}
	}
}

func TestCastlingThroughCheck(t *testing.T) {
	// Test castling through check (should be illegal)
	pos := &Position{
		Board:     Bitboard{},
		Tag:       WhiteToMove | WhiteCanCastleKingSide,
		EnPassant: NoSquare,
	}

	// Set up position: White king on E1, white rook on H1, black rook on A1
	// Black rook attacks F1 horizontally along the 1st rank, making castling through F1 illegal
	pos.Board.UpdateSquare(E1, WhiteKing, NoPiece)
	pos.Board.UpdateSquare(H1, WhiteRook, NoPiece)
	pos.Board.UpdateSquare(A1, BlackRook, NoPiece)

	legalMoves := GenerateLegalMoves(pos)

	// Check that castling move is filtered out
	castlingFound := false
	for _, move := range legalMoves {
		if move.IsCastle() {
			castlingFound = true
			break
		}
	}

	if castlingFound {
		t.Errorf("Castling through check should be filtered out by legal move generation")
	}
}

func TestEnPassantPinDiscovery(t *testing.T) {
	// Test en passant that would discover check (should be illegal)
	pos := &Position{
		Board:     Bitboard{},
		Tag:       WhiteToMove,
		EnPassant: D6,
	}

	// Set up position: White king on E5, White pawn on C5, Black pawn on D5, Black rook on A5
	// En passant capture would remove the black pawn and expose king to check from rook
	pos.Board.UpdateSquare(E5, WhiteKing, NoPiece)
	pos.Board.UpdateSquare(C5, WhitePawn, NoPiece)
	pos.Board.UpdateSquare(D5, BlackPawn, NoPiece)
	pos.Board.UpdateSquare(A5, BlackRook, NoPiece)

	legalMoves := GenerateLegalMoves(pos)

	// Check that en passant move is filtered out
	enPassantFound := false
	for _, move := range legalMoves {
		if move.IsEnPassant() {
			enPassantFound = true
			break
		}
	}

	if enPassantFound {
		t.Errorf("En passant discovering check should be filtered out by legal move generation")
	}
}

func TestPawnCheckDetection(t *testing.T) {
	tests := []struct {
		name      string
		fen       string
		kingColor Color
		expected  bool
	}{
		{
			name:      "White king on e4 checked by black pawn on d5",
			fen:       "8/8/8/3p4/4K3/8/8/8 w - - 0 1",
			kingColor: White,
			expected:  true,
		},
		{
			name:      "White king on e4 checked by black pawn on f5",
			fen:       "8/8/8/5p2/4K3/8/8/8 w - - 0 1",
			kingColor: White,
			expected:  true,
		},
		{
			name:      "Black king on e5 checked by white pawn on d4",
			fen:       "8/8/8/4k3/3P4/8/8/8 b - - 0 1",
			kingColor: Black,
			expected:  true,
		},
		{
			name:      "Black king on e5 checked by white pawn on f4",
			fen:       "8/8/8/4k3/5P2/8/8/8 b - - 0 1",
			kingColor: Black,
			expected:  true,
		},
		{
			name:      "White king on a4 checked by black pawn on b5",
			fen:       "8/8/8/1p6/K7/8/8/8 w - - 0 1",
			kingColor: White,
			expected:  true,
		},
		{
			name:      "White king on h4 checked by black pawn on g5",
			fen:       "8/8/8/6p1/7K/8/8/8 w - - 0 1",
			kingColor: White,
			expected:  true,
		},
		{
			name:      "White king on e4 NOT checked by black pawn on e5 (same file, no diagonal)",
			fen:       "8/8/8/4p3/4K3/8/8/8 w - - 0 1",
			kingColor: White,
			expected:  false,
		},
		{
			name:      "Black king on e5 NOT checked by white pawn on e4 (same file, no diagonal)",
			fen:       "8/8/8/4k3/4P3/8/8/8 b - - 0 1",
			kingColor: Black,
			expected:  false,
		},
		{
			name:      "Edge case: White king on a1 checked by black pawn on b2",
			fen:       "8/8/8/8/8/8/1p6/K7 w - - 0 1",
			kingColor: White,
			expected:  true,
		},
		{
			name:      "Edge case: Black king on h8 checked by white pawn on g7",
			fen:       "7k/6P1/8/8/8/8/8/8 b - - 0 1",
			kingColor: Black,
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos, err := ParseFEN(tt.fen)
			if err != nil {
				t.Fatalf("Failed to parse FEN: %v", err)
			}

			result := isInCheck(pos, tt.kingColor)
			if result != tt.expected {
				t.Errorf("Expected isInCheck(%s, %v) = %v, got %v", tt.fen, tt.kingColor, tt.expected, result)
			}
		})
	}
}

func BenchmarkLegalMoveGeneration(b *testing.B) {
	pos := &Position{
		Board:     StartingBoard(),
		Tag:       WhiteCanCastleKingSide | WhiteCanCastleQueenSide | BlackCanCastleKingSide | BlackCanCastleQueenSide | WhiteToMove,
		EnPassant: NoSquare,
	}

	benchmarkFunction(b, func() {
		GenerateLegalMoves(pos)
	})
}

func BenchmarkPerftDepth3(b *testing.B) {
	pos := &Position{
		Board:     StartingBoard(),
		Tag:       WhiteCanCastleKingSide | WhiteCanCastleQueenSide | BlackCanCastleKingSide | BlackCanCastleQueenSide | WhiteToMove,
		EnPassant: NoSquare,
	}

	benchmarkFunction(b, func() {
		RunPerftTest(pos, 3)
	})
}
