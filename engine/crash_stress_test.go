package engine

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"testing"
)

// TestMalformedFENStressTest tests the engine with various malformed FEN strings
func TestMalformedFENStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	malformedFENs := []string{
		// Empty and invalid
		"",
		"   ",
		"invalid",

		// Too many/few pieces
		"rnbqkbnr/pppppppp/pppppppp/pppppppp/pppppppp/pppppppp/pppppppp/rnbqkbnr w KQkq - 0 1",
		"8/8/8/8/8/8/8/8 w KQkq - 0 1",

		// Invalid pieces
		"xnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNx w KQkq - 0 1",

		// Invalid numbers
		"rnbqkbnr/pppppppp/9/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		"rnbqkbnr/pppppppp/0/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",

		// Invalid turn
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR x KQkq - 0 1",
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR wb KQkq - 0 1",

		// Invalid castling
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w XYZ - 0 1",
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkqx - 0 1",

		// Invalid en passant
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq z9 0 1",
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq e0 0 1",

		// Invalid counters
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - -1 1",
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 -1",
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - abc def",

		// Truncated FEN
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR",
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w",
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq",

		// Multiple kings
		"rnbqkknr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPKPPP/RNBQKBNR w KQkq - 0 1",

		// No kings
		"rnbqrbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQRBNR w KQkq - 0 1",

		// Very long FEN
		strings.Repeat("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1 ", 100),

		// Unicode and special characters
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1 🚀",
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1\x00",
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR\nw KQkq - 0 1",
	}

	t.Logf("Testing %d malformed FEN strings...", len(malformedFENs))

	for i, fen := range malformedFENs {
		t.Run(fmt.Sprintf("MalformedFEN_%d", i), func(t *testing.T) {
			// This should not crash
			pos, err := ParseFEN(fen)
			if err == nil {
				t.Logf("⚠️  Expected error for FEN %q but got none", fen)
				if pos != nil {
					// Try to use the position to ensure it doesn't crash later
					_ = GenerateLegalMoves(pos)
					_ = Evaluate(&pos.Board)
				}
			} else {
				t.Logf("✅ Correctly rejected malformed FEN: %q (error: %v)", fen, err)
			}
		})
	}
}

// TestExtremeMoveGeneration tests move generation in extreme positions
func TestExtremeMoveGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	extremeFENs := []struct {
		name string
		fen  string
		desc string
	}{
		{
			name: "PromotionFrenzy",
			fen:  "8/PPPPPPPP/8/8/8/8/pppppppp/8 w - - 0 1",
			desc: "All pawns about to promote",
		},
		{
			name: "MaximumPieces",
			fen:  "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
			desc: "Starting position with all pieces",
		},
		{
			name: "QueenApocalypse",
			fen:  "qqqqqqqq/qqqqqqqq/8/8/8/8/QQQQQQQQ/QQQQQQQK w - - 0 1",
			desc: "Maximum queens on board",
		},
		{
			name: "KnightSwarm",
			fen:  "nnnnnnnn/nnnnnnnn/8/8/8/8/NNNNNNNN/NNNNNNNK w - - 0 1",
			desc: "Knight explosion",
		},
		{
			name: "DeepEndgame",
			fen:  "8/8/8/8/8/8/8/K6k w - - 0 1",
			desc: "Minimal endgame",
		},
		{
			name: "CastlingChaos",
			fen:  "r3k2r/pppppppp/8/8/8/8/PPPPPPPP/R3K2R w KQkq - 0 1",
			desc: "All castling rights available",
		},
	}

	for _, test := range extremeFENs {
		t.Run(test.name, func(t *testing.T) {
			pos, err := ParseFEN(test.fen)
			if err != nil {
				t.Logf("⚠️  Could not parse %s: %v", test.desc, err)
				return
			}

			// Generate moves - this should not crash
			moves := GenerateLegalMoves(pos)
			t.Logf("✅ %s: generated %d legal moves", test.desc, len(moves))

			// Try a quick search - this should not crash
			if len(moves) > 0 {
				result := Search(pos, 1)
				if result != nil {
					t.Logf("✅ %s: search completed, found move with score %d", test.desc, result.BestScore)
				}
			}
		})
	}
}

// TestRandomGameStress plays random games to find crashes
func TestRandomGameStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	numGames := 20
	maxMovesPerGame := 200

	t.Logf("Playing %d random games with up to %d moves each...", numGames, maxMovesPerGame)

	for game := 0; game < numGames; game++ {
		t.Run(fmt.Sprintf("RandomGame_%d", game), func(t *testing.T) {
			pos, _ := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
			moveCount := 0

			for move := 0; move < maxMovesPerGame; move++ {
				moves := GenerateLegalMoves(pos)
				if len(moves) == 0 {
					break // Game ended
				}

				// Pick a random move
				randomMove := moves[rand.Intn(len(moves))]

				// Make the move
				_, _, _, _ = pos.MakeMove(randomMove)
				moveCount++

				// Occasionally run search to stress test
				if move%10 == 0 {
					_ = Search(pos, 2)
				}

				// Evaluate position
				_ = Evaluate(&pos.Board)
			}

			t.Logf("✅ Game %d completed %d moves without crashing", game, moveCount)
		})
	}
}

// TestMemoryStress tests memory allocation patterns that could cause crashes
func TestMemoryStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory stress test in short mode")
	}

	pos, _ := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")

	// Stress test move generation (could cause slice reallocations)
	t.Run("MoveGenerationStress", func(t *testing.T) {
		for i := 0; i < 10000; i++ {
			moves := GenerateLegalMoves(pos)
			if len(moves) == 0 {
				t.Fatal("No moves generated")
			}
		}
		t.Log("✅ Generated moves 10,000 times without crashing")
	})

	// Stress test evaluation (could cause allocation issues)
	t.Run("EvaluationStress", func(t *testing.T) {
		for i := 0; i < 10000; i++ {
			_ = Evaluate(&pos.Board)
		}
		t.Log("✅ Evaluated position 10,000 times without crashing")
	})

	// Stress test search (could cause stack overflow or allocation issues)
	t.Run("SearchStress", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			result := Search(pos, 3)
			if result == nil {
				t.Fatal("Search returned nil")
			}
		}
		t.Log("✅ Searched position 100 times without crashing")
	})
}

// TestConcurrencyStress tests concurrent access that could cause data races
func TestConcurrencyStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrency stress test in short mode")
	}

	pos, _ := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	numGoroutines := 10

	// Test concurrent move generation
	t.Run("ConcurrentMoveGeneration", func(t *testing.T) {
		done := make(chan bool, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Goroutine %d panicked: %v", id, r)
					}
					done <- true
				}()

				for j := 0; j < 1000; j++ {
					moves := GenerateLegalMoves(pos)
					if len(moves) == 0 {
						t.Errorf("No moves in goroutine %d", id)
						return
					}
				}
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < numGoroutines; i++ {
			<-done
		}

		t.Log("✅ Concurrent move generation completed without crashes")
	})
}

// TestNumericalLimits tests numerical edge cases
func TestNumericalLimits(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping numerical limits test in short mode")
	}

	pos, _ := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")

	// Test extreme search depths
	extremeDepths := []int{0, 1, -1, 100, 1000, math.MaxInt32}

	for _, depth := range extremeDepths {
		t.Run(fmt.Sprintf("Depth_%d", depth), func(t *testing.T) {
			result := Search(pos, depth)
			if depth > 0 && result == nil {
				t.Errorf("Search with depth %d returned nil", depth)
			}
			t.Logf("✅ Survived search with depth %d", depth)
		})
	}
}

// TestComplexPositions tests positions known to cause issues in chess engines
func TestComplexPositions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping complex positions test in short mode")
	}

	complexPositions := []struct {
		name string
		fen  string
		desc string
	}{
		{
			name: "Perft_Position_4",
			fen:  "r3k2r/Pppp1ppp/1b3nbN/nP6/BBP1P3/q4N2/Pp1P2PP/R2Q1RK1 w kq - 0 1",
			desc: "Complex perft test position",
		},
		{
			name: "Kiwipete",
			fen:  "r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1",
			desc: "Famous Kiwipete position",
		},
		{
			name: "Endgame_Study",
			fen:  "8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1",
			desc: "Complex endgame study",
		},
		{
			name: "Promotion_Complex",
			fen:  "r1bqkb1r/pppp1Qpp/2n2n2/4p3/2B1P3/8/PPPP1PPP/RNB1K1NR b KQkq - 0 4",
			desc: "Complex promotion scenario",
		},
	}

	for _, test := range complexPositions {
		t.Run(test.name, func(t *testing.T) {
			pos, err := ParseFEN(test.fen)
			if err != nil {
				t.Fatalf("Failed to parse %s: %v", test.desc, err)
			}

			// Generate moves
			moves := GenerateLegalMoves(pos)
			t.Logf("✅ %s: %d legal moves", test.desc, len(moves))

			// Evaluate
			score := Evaluate(&pos.Board)
			t.Logf("✅ %s: evaluation score %d", test.desc, score)

			// Quick search
			result := Search(pos, 3)
			if result != nil {
				t.Logf("✅ %s: search found move with score %d", test.desc, result.BestScore)
			}
		})
	}
}
