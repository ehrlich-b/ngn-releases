package engine

import (
	"testing"
)

// TestMateDetectionFailure demonstrates the critical bug described in TODO.md:
// Engine chooses non-mate moves over mate-in-1 moves
func TestMateDetectionFailure(t *testing.T) {
	// This is the exact test case from TODO.md:
	// K+R vs k position where Ra8# is mate-in-1 but engine chooses h6g5 instead

	t.Log("🔍 REPRODUCING MATE DETECTION BUG FROM TODO.md")
	t.Log("Position: K+R vs k (simple mate-in-1 with Ra8#)")

	// Set up the position: White King + Rook vs Black King
	// This should be a simple mate-in-1 with Ra8#
	pos, err := ParseFEN("7k/R7/6KP/8/8/8/8/8 w - - 0 1")
	if err != nil {
		t.Fatalf("Failed to create test position: %v", err)
	}

	t.Logf("Position FEN: %s", GenerateFEN(pos))

	// Search at depth 1 (same as TODO.md test)
	info := Search(pos, 1)

	score := info.BestScore
	t.Logf("Search result:")
	t.Logf("  Best move: %s", MoveToAlgebraic(info.BestMove))
	t.Logf("  Score: %d cp", score)
	t.Logf("  Nodes: %d", info.Nodes)

	// According to TODO.md, the bug is:
	// - NGN chooses h6g5 (+827cp) instead of Ra8# (mate)
	// - Engine sees random centipawn values when Stockfish sees "mate 1"

	// Check if we found the mate move Ra8#
	expectedMate := "a7a8" // Ra8#
	actualMove := MoveToAlgebraic(info.BestMove)

	if actualMove == expectedMate {
		t.Logf("✅ Engine correctly found mate move: %s", actualMove)

		// Check if score indicates mate (should be near MATE_VALUE)
		if int(score) >= MATE_IN_MAX {
			t.Logf("✅ Engine correctly evaluates as mate: %d cp", score)
		} else {
			t.Errorf("❌ BUG: Engine found mate move but scored as %d cp (not mate)", score)
		}
	} else {
		t.Errorf("❌ CRITICAL BUG CONFIRMED: Engine chose %s (+%dcp) instead of Ra8# (mate)", actualMove, score)
		t.Errorf("   This is exactly the problem described in TODO.md!")
		t.Errorf("   Engine cannot detect moves that deliver checkmate")
	}
}

// TestMateInOneDetection tests various mate-in-1 positions
func TestMateInOneDetection(t *testing.T) {
	positions := []struct {
		name      string
		fen       string
		mateMoves []string // Possible mate-in-1 moves
	}{
		{
			name:      "Queen mate on back rank",
			fen:       "rnbqkb1r/pppp1Qpp/5n2/4p3/2B1P3/8/PPPP1PPP/RNB1K1NR b KQkq - 0 4",
			mateMoves: []string{}, // Black to move, no mate available
		},
		{
			name:      "Rook mate in corner",
			fen:       "7k/R7/6K1/8/8/8/8/8 w - - 0 1",
			mateMoves: []string{"a7a8"},
		},
		{
			name:      "Queen mate with king support",
			fen:       "k7/8/2K5/1Q6/8/8/8/8 w - - 0 1",
			mateMoves: []string{"b5b7"},
		},
	}

	for _, test := range positions {
		t.Run(test.name, func(t *testing.T) {
			pos, err := ParseFEN(test.fen)
			if err != nil {
				t.Fatalf("Failed to create position: %s: %v", test.fen, err)
			}

			// Search deeper to find mate
			info := Search(pos, 3)
			score := info.BestScore

			move := MoveToAlgebraic(info.BestMove)
			t.Logf("Position: %s", test.name)
			t.Logf("  Best move: %s", move)
			t.Logf("  Score: %d cp", score)

			// Check if we found a mate move
			foundMate := false
			for _, mateMove := range test.mateMoves {
				if move == mateMove {
					foundMate = true
					break
				}
			}

			if len(test.mateMoves) > 0 {
				if foundMate && int(score) >= MATE_IN_MAX {
					t.Logf("✅ Correctly found mate: %s (%d cp)", move, score)
				} else if foundMate {
					t.Errorf("❌ Found mate move but wrong score: %s (%d cp, expected ≥%d)", move, score, MATE_IN_MAX)
				} else {
					t.Errorf("❌ Missed mate-in-1: chose %s instead of %v", move, test.mateMoves)
				}
			}
		})
	}
}
