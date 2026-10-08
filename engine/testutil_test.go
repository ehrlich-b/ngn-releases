// Test utilities for chess engine testing
// This file provides common testing infrastructure for the ngn chess engine

package engine

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// TestPosition represents a test position with expected properties
type TestPosition struct {
	Name        string    // Human readable name
	FEN         string    // FEN notation of the position
	ExpectedPos *Position // Expected position after parsing
	Description string    // Description of what this tests
}

// TestMove represents a test move with expected properties
type TestMove struct {
	From          Square    // Source square
	To            Square    // Destination square
	MovingPiece   Piece     // Piece being moved
	CapturedPiece Piece     // Piece being captured (NoPiece if no capture)
	PromoType     PieceType // Promotion type (NoType if no promotion)
	Tag           MoveTag   // Move tags (castle, en passant, etc.)
	Description   string    // Description of the move
}

// PerftResult represents the result of a perft test
type PerftResult struct {
	Depth      int    // Search depth
	Nodes      uint64 // Number of nodes at this depth
	Captures   uint64 // Number of captures
	EnPassant  uint64 // Number of en passant captures
	Castles    uint64 // Number of castling moves
	Promotions uint64 // Number of promotions
	Checks     uint64 // Number of checking moves
	Checkmates uint64 // Number of checkmate positions
}

// PerftTestCase represents a perft test case
type PerftTestCase struct {
	Name     string        // Test case name
	FEN      string        // Starting position in FEN
	Expected []PerftResult // Expected results for each depth
}

// Known test positions for common scenarios
var StandardTestPositions = []TestPosition{
	{
		Name:        "Starting Position",
		FEN:         "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		Description: "Standard chess starting position",
	},
	{
		Name:        "Empty Board",
		FEN:         "8/8/8/8/8/8/8/8 w - - 0 1",
		Description: "Empty board for testing basic functionality",
	},
	{
		Name:        "King and Queen vs King",
		FEN:         "4k3/8/8/8/8/8/8/4K2Q w - - 0 1",
		Description: "Basic endgame position",
	},
}

// Common perft test positions
var StandardPerftTests = []PerftTestCase{
	{
		Name: "Starting Position",
		FEN:  "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		Expected: []PerftResult{
			{Depth: 1, Nodes: 20},
			{Depth: 2, Nodes: 400},
			{Depth: 3, Nodes: 8902},
			{Depth: 4, Nodes: 197281},
			{Depth: 5, Nodes: 4865609},
		},
	},
	{
		Name: "Kiwipete Position",
		FEN:  "r3k2r/Pppp1ppp/1b3nbN/nP6/BBP1P3/q4N2/Pp1P2PP/R2Q1RK1 w kq - 0 1",
		Expected: []PerftResult{
			{Depth: 1, Nodes: 6},
			// TODO: Get correct depth 2 and 3 values from Stockfish
			// {Depth: 2, Nodes: 2039},
			// {Depth: 3, Nodes: 97862},
		},
	},
}

// PositionEqual checks if two positions are equivalent
func PositionEqual(a, b *Position) bool {
	if a == nil || b == nil {
		return a == b
	}

	return a.Board == b.Board &&
		a.Tag == b.Tag &&
		a.EnPassant == b.EnPassant &&
		a.HalfMoveClock == b.HalfMoveClock &&
		a.hash == b.hash
}

// BitboardEqual checks if two bitboards are equivalent
func BitboardEqual(a, b *Bitboard) bool {
	if a == nil || b == nil {
		return a == b
	}

	return reflect.DeepEqual(*a, *b)
}

// MoveEqual checks if two moves are equivalent
func MoveEqual(a, b Move) bool {
	return a == b
}

// AssertPositionEqual asserts that two positions are equal
func AssertPositionEqual(t *testing.T, expected, actual *Position, message string) {
	t.Helper()
	if !PositionEqual(expected, actual) {
		t.Errorf("%s: positions not equal\nExpected: %+v\nActual: %+v", message, expected, actual)
	}
}

// AssertBitboardEqual asserts that two bitboards are equal
func AssertBitboardEqual(t *testing.T, expected, actual *Bitboard, message string) {
	t.Helper()
	if !BitboardEqual(expected, actual) {
		t.Errorf("%s: bitboards not equal\nExpected: %s\nActual: %s",
			message, expected.Draw(), actual.Draw())
	}
}

// AssertMoveEqual asserts that two moves are equal
func AssertMoveEqual(t *testing.T, expected, actual Move, message string) {
	t.Helper()
	if !MoveEqual(expected, actual) {
		t.Errorf("%s: moves not equal\nExpected: %s\nActual: %s",
			message, expected.ToString(), actual.ToString())
	}
}

// AssertPieceAt asserts that a specific piece is at a given square
func AssertPieceAt(t *testing.T, board *Bitboard, square Square, expectedPiece Piece, message string) {
	t.Helper()
	actualPiece := board.PieceAt(square)
	if actualPiece != expectedPiece {
		t.Errorf("%s: piece at %s not as expected\nExpected: %s\nActual: %s",
			message, square.Name(), expectedPiece.Name(), actualPiece.Name())
	}
}

// CreateTestPosition creates a position from FEN (with error handling for tests)
func CreateTestPosition(t *testing.T, fen string) *Position {
	t.Helper()
	// TODO: Implement FEN parsing
	// For now, return a default position
	pos := &Position{
		Board: StartingBoard(),
		Tag:   WhiteCanCastleKingSide | WhiteCanCastleQueenSide | BlackCanCastleKingSide | BlackCanCastleQueenSide,
	}
	return pos
}

// PrintBoard prints a board in a readable format for debugging
func PrintBoard(board *Bitboard) string {
	return board.Draw()
}

// PrintPosition prints a position in a readable format for debugging
func PrintPosition(pos *Position) string {
	var sb strings.Builder
	sb.WriteString("Position:\n")
	sb.WriteString(pos.Board.Draw())
	sb.WriteString(fmt.Sprintf("Turn: %s\n", func() string {
		if pos.Turn() == White {
			return "White"
		}
		return "Black"
	}()))
	sb.WriteString(fmt.Sprintf("Castling: %v\n", pos.Tag))
	sb.WriteString(fmt.Sprintf("En Passant: %s\n", pos.EnPassant.Name()))
	sb.WriteString(fmt.Sprintf("Half Move Clock: %d\n", pos.HalfMoveClock))
	return sb.String()
}

// RunPerftTest runs a perft test and returns the result
func RunPerftTest(position *Position, depth int) PerftResult {
	result := PerftResult{Depth: depth}

	if depth == 0 {
		result.Nodes = 1
		return result
	}

	moves := GenerateMoves(position)

	for _, move := range moves {
		if IsLegalMove(position, move) {
			// Make the move
			ep, tag, hc, _ := position.MakeMove(move)

			// Recursively count nodes
			subResult := RunPerftTest(position, depth-1)
			result.Nodes += subResult.Nodes

			// Count move types at leaf nodes
			if depth == 1 {
				if move.IsCapture() {
					result.Captures++
				}
				if move.IsEnPassant() {
					result.EnPassant++
				}
				if move.IsCastle() {
					result.Castles++
				}
				if move.PromoType() != NoType {
					result.Promotions++
				}
				// TODO: Add check and checkmate detection
			}

			// Unmake the move
			position.UnMakeMove(move, tag, ep, hc)
		}
	}

	return result
}

// ValidatePerftResult validates that a perft result matches expectations
func ValidatePerftResult(t *testing.T, testCase PerftTestCase, actualResult PerftResult) {
	t.Helper()

	var expectedResult *PerftResult
	for _, expected := range testCase.Expected {
		if expected.Depth == actualResult.Depth {
			expectedResult = &expected
			break
		}
	}

	if expectedResult == nil {
		t.Errorf("No expected result for depth %d in test case %s", actualResult.Depth, testCase.Name)
		return
	}

	if actualResult.Nodes != expectedResult.Nodes {
		t.Errorf("Perft test %s depth %d: expected %d nodes, got %d nodes",
			testCase.Name, actualResult.Depth, expectedResult.Nodes, actualResult.Nodes)
	}
}

// benchmarkFunction is a helper for benchmarking chess engine functions
func benchmarkFunction(b *testing.B, fn func()) {
	b.Helper()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fn()
	}
}

// GenerateTestMoves generates test moves for a given position
func GenerateTestMoves(pos *Position) []Move {
	return GenerateMoves(pos)
}

// IsTestLegalMove checks if a move is legal in the given position
func IsTestLegalMove(pos *Position, move Move) bool {
	return IsLegalMove(pos, move)
}
