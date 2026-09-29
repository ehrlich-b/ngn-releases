package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ComprehensiveEvaluationResult represents detailed engine analysis
type ComprehensiveEvaluationResult struct {
	TacticalScore   int
	PositionalScore int
	MaterialScore   int
	EndgameScore    int
	OverallScore    int
	EstimatedELO    int
	AverageDepth    float64
	WeaknessReport  []string
}

// TestComprehensiveEngineEvaluation runs a thorough 2-minute engine evaluation
// with proper time allocation for meaningful search depth
func TestComprehensiveEngineEvaluation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping comprehensive evaluation in short mode")
	}

	// Skip unless explicitly running comprehensive evaluation
	if os.Getenv("RUN_COMPREHENSIVE_EVAL") != "1" {
		t.Skip("Skipping comprehensive evaluation - set RUN_COMPREHENSIVE_EVAL=1 to enable")
	}

	t.Log("🔥 COMPREHENSIVE ENGINE EVALUATION (2 MINUTE TEST)")
	t.Log("=================================================")
	t.Log("Each position gets 10-15 seconds for deep analysis")

	startTime := time.Now()
	result := &ComprehensiveEvaluationResult{
		WeaknessReport: make([]string, 0),
	}

	// Test categories with proper time allocation for deep 2-minute evaluation
	tacticalScore, depthTactical := testTacticalWithTime(t, 80*time.Second)       // 80s for tactics (most important)
	materialScore, depthMaterial := testMaterialWithTime(t, 20*time.Second)       // 20s for material
	positionalScore, depthPositional := testPositionalWithTime(t, 15*time.Second) // 15s for positional
	endgameScore, depthEndgame := testEndgameWithTime(t, 5*time.Second)           // 5s for endgame

	// Assign to result struct
	result.TacticalScore = tacticalScore
	result.MaterialScore = materialScore
	result.PositionalScore = positionalScore
	result.EndgameScore = endgameScore

	// Calculate average depth
	result.AverageDepth = (depthTactical + depthMaterial + depthPositional + depthEndgame) / 4.0

	// Calculate overall score and ELO
	result.calculateComprehensiveScore()

	totalTime := time.Since(startTime)

	// Report comprehensive results
	t.Log("\n🎯 COMPREHENSIVE EVALUATION RESULTS")
	t.Log("===================================")
	t.Logf("Total test time: %.1f seconds", totalTime.Seconds())
	t.Logf("Average search depth: %.1f", result.AverageDepth)
	t.Log("")
	t.Logf("Tactical Strength:     %d/100", result.TacticalScore)
	t.Logf("Material Evaluation:   %d/100", result.MaterialScore)
	t.Logf("Positional Play:       %d/100", result.PositionalScore)
	t.Logf("Endgame Technique:     %d/100", result.EndgameScore)
	t.Log("")
	t.Logf("OVERALL SCORE:         %d/100", result.OverallScore)
	t.Logf("ESTIMATED ELO:         %d", result.EstimatedELO)

	if len(result.WeaknessReport) > 0 {
		t.Log("\n🚨 IDENTIFIED WEAKNESSES:")
		for i, weakness := range result.WeaknessReport {
			t.Logf("%d. %s", i+1, weakness)
		}
	}

	// Success criteria
	if result.EstimatedELO >= 1400 {
		t.Logf("✅ ENGINE MEETS MINIMUM STANDARD: %d ELO (≥1400 required)", result.EstimatedELO)
	} else {
		t.Errorf("❌ ENGINE BELOW MINIMUM: %d ELO (≥1400 required)", result.EstimatedELO)
	}

	// Write results to output file
	writeEvaluationResults(result, totalTime, t)
}

// testTacticalWithTime tests tactical awareness with proper time allocation
func testTacticalWithTime(t *testing.T, timeLimit time.Duration) (int, float64) {
	t.Log("\n🎯 TACTICAL ANALYSIS (Deep Search)")
	t.Log("=================================")

	// Key tactical positions that require depth to solve correctly
	tacticalSuite := []struct {
		name        string
		fen         string
		timeAlloc   time.Duration
		bestMoves   []string
		description string
	}{
		{
			name:        "Fork_Attack",
			fen:         "rnbqkb1r/pppp1ppp/5n2/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R w KQkq - 2 3",
			timeAlloc:   15 * time.Second,
			bestMoves:   []string{"f3g5", "f3e5"}, // Accept either tactical setup or material gain
			description: "Knight fork setup vs immediate material",
		},
		{
			name:        "Free_Material",
			fen:         "rnbqkb1r/pppp1ppp/5n2/4p3/2B1P3/5N2/PPPP1PPP/RNBQK2R w KQkq - 0 4",
			timeAlloc:   10 * time.Second,
			bestMoves:   []string{"c4f7"}, // Bishop takes free pawn
			description: "Should capture completely free material",
		},
		{
			name:        "Pin_Defense",
			fen:         "r1bqkbnr/pppp1ppp/2n5/1B2p3/4P3/5N2/PPPP1PPP/RNBQK2R w KQkq - 4 4",
			timeAlloc:   12 * time.Second,
			bestMoves:   []string{"b5c6", "b5d7"}, // Attack the pinned knight
			description: "Should exploit pin tactically",
		},
		{
			name:        "Mate_Threat",
			fen:         "rnb1kbnr/pppp1ppp/8/4p1q1/4P3/5N2/PPPP1PPP/RNBQKB1R w KQkq - 2 3",
			timeAlloc:   20 * time.Second,
			bestMoves:   []string{"f1e2", "d2d3", "h2h3"}, // Block mate threat
			description: "Should defend against mate threats",
		},
		{
			name:        "Double_Attack",
			fen:         "r1bqk2r/pppp1ppp/2n2n2/2b1p3/2B1P3/3P1N2/PPP2PPP/RNBQK2R w KQkq - 0 6",
			timeAlloc:   15 * time.Second,
			bestMoves:   []string{"c4f7", "f3g5"}, // Double attack ideas
			description: "Should find double attack opportunities",
		},
		{
			name:        "Skewer_Attack",
			fen:         "r3k2r/1pp2pp1/p1n1bn1p/2bpp3/2B1P3/3P1N2/PPPN1PPP/R2QK2R w KQkq - 0 8",
			timeAlloc:   18 * time.Second,
			bestMoves:   []string{"d1d5", "f3e5"}, // Skewer or attack
			description: "Should find skewer or strong central attack",
		},
		{
			name:        "Discovered_Attack",
			fen:         "rnbqk2r/ppppbppp/5n2/4p3/2B1P3/3P1N2/PPP2PPP/RNBQK2R w KQkq - 0 5",
			timeAlloc:   12 * time.Second,
			bestMoves:   []string{"f3d4", "f3g5"}, // Discovered attack options
			description: "Should exploit discovered attack potential",
		},
		{
			name:        "Sacrifice_For_Mate",
			fen:         "r1bq1rk1/pppp1ppp/2n2n2/2b1p3/2B1P3/3P1N2/PPP2PPP/RNBQR1K1 w - - 0 7",
			timeAlloc:   25 * time.Second,
			bestMoves:   []string{"c4f7", "f3g5", "e1e3"}, // Tactical sacrifices
			description: "Should consider sacrificial attacks",
		},
	}

	correct := 0
	totalDepth := 0.0

	for _, test := range tacticalSuite {
		t.Run(test.name, func(t *testing.T) {
			pos, err := ParseFEN(test.fen)
			if err != nil {
				t.Fatalf("Failed to parse FEN: %v", err)
			}

			t.Logf("Position: %s", test.description)
			t.Logf("Time allocation: %.1f seconds", test.timeAlloc.Seconds())

			// Search with time limit using iterative deepening
			timeManager := NewTimeManager()
			params := SearchParams{
				MoveTime: int(test.timeAlloc.Milliseconds()),
			}
			timeManager.SetTimeControl(params, pos.Turn() == White)
			result := SearchIterativeDeepening(pos, 20, timeManager)

			if result != nil {
				move := result.BestMove.ToString()
				depth := result.Depth
				score := result.BestScore
				nodes := result.Nodes

				totalDepth += float64(depth)

				t.Logf("Result: %s (depth: %d, score: %d, nodes: %d)", move, depth, score, nodes)

				// Check if move is acceptable
				moveGood := false
				for _, goodMove := range test.bestMoves {
					if move == goodMove {
						moveGood = true
						break
					}
				}

				if moveGood {
					t.Logf("✅ CORRECT: Good tactical choice")
					correct++
				} else {
					t.Logf("⚠️  SUBOPTIMAL: Expected %v, got %s", test.bestMoves, move)
				}
			} else {
				t.Logf("❌ FAILED: No result from search")
			}
		})
	}

	avgDepth := totalDepth / float64(len(tacticalSuite))
	score := (correct * 100) / len(tacticalSuite)

	t.Logf("\n🎯 Tactical Results: %d/100 (%d/%d correct, avg depth: %.1f)",
		score, correct, len(tacticalSuite), avgDepth)

	return score, avgDepth
}

// testMaterialWithTime tests material evaluation with time
func testMaterialWithTime(t *testing.T, timeLimit time.Duration) (int, float64) {
	t.Log("\n💎 MATERIAL EVALUATION")
	t.Log("=====================")

	materialTests := []struct {
		name     string
		fen      string
		expected string // "white_wins", "black_wins", "equal"
	}{
		{
			name:     "Queen_vs_Rook",
			fen:      "4k3/8/8/8/8/8/4Q3/4K3 w - - 0 1",
			expected: "white_wins",
		},
		{
			name:     "Rook_vs_Minor_Pieces",
			fen:      "4k3/8/8/8/8/2bn4/4R3/4K3 w - - 0 1",
			expected: "equal", // Should be roughly equal
		},
	}

	correct := 0

	for _, test := range materialTests {
		pos, _ := ParseFEN(test.fen)
		eval := EvaluateForPlayer(&pos.Board, White)

		var result string
		if eval > 200 {
			result = "white_wins"
		} else if eval < -200 {
			result = "black_wins"
		} else {
			result = "equal"
		}

		if result == test.expected {
			correct++
			t.Logf("✅ %s: %d (correct)", test.name, eval)
		} else {
			t.Logf("❌ %s: %d (expected %s)", test.name, eval, test.expected)
		}
	}

	score := (correct * 100) / len(materialTests)
	return score, 1.0 // Material evaluation is static
}

// testPositionalWithTime tests positional understanding
func testPositionalWithTime(t *testing.T, timeLimit time.Duration) (int, float64) {
	t.Log("\n🏰 POSITIONAL UNDERSTANDING")
	t.Log("==========================")

	positionalTests := []struct {
		name        string
		fen         string
		timeAlloc   time.Duration
		bestMoves   []string
		description string
	}{
		{
			name:        "Center_Control",
			fen:         "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1",
			timeAlloc:   8 * time.Second,
			bestMoves:   []string{"d7d5", "e7e5", "d7d6"}, // Control center
			description: "Should contest central squares",
		},
		{
			name:        "Piece_Development",
			fen:         "rnbqkbnr/pppp1ppp/8/4p3/4P3/8/PPPP1PPP/RNBQKBNR w KQkq e6 0 2",
			timeAlloc:   8 * time.Second,
			bestMoves:   []string{"g1f3", "b1c3", "f1c4"}, // Develop pieces
			description: "Should prioritize piece development",
		},
		{
			name:        "King_Safety",
			fen:         "rnbqkb1r/pppp1ppp/5n2/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R w KQkq - 4 3",
			timeAlloc:   9 * time.Second,
			bestMoves:   []string{"e1g1", "f1e2"}, // Castle or prepare castling
			description: "Should prioritize king safety",
		},
	}

	correct := 0
	totalDepth := 0.0

	for _, test := range positionalTests {
		pos, err := ParseFEN(test.fen)
		if err != nil {
			t.Logf("❌ %s: Failed to parse FEN", test.name)
			continue
		}

		t.Logf("Testing %s: %s", test.name, test.description)

		timeManager := NewTimeManager()
		params := SearchParams{
			MoveTime: int(test.timeAlloc.Milliseconds()),
		}
		timeManager.SetTimeControl(params, pos.Turn() == White)
		result := SearchIterativeDeepening(pos, 15, timeManager)

		if result != nil {
			move := result.BestMove.ToString()
			depth := result.Depth
			totalDepth += float64(depth)

			// Check if move is acceptable
			moveGood := false
			for _, goodMove := range test.bestMoves {
				if move == goodMove {
					moveGood = true
					break
				}
			}

			if moveGood {
				t.Logf("✅ %s: %s (depth %d) - Good positional choice", test.name, move, depth)
				correct++
			} else {
				t.Logf("⚠️  %s: %s (depth %d) - Expected %v", test.name, move, depth, test.bestMoves)
			}
		} else {
			t.Logf("❌ %s: No result from search", test.name)
		}
	}

	avgDepth := totalDepth / float64(len(positionalTests))
	score := (correct * 100) / len(positionalTests)

	t.Logf("🏰 Positional Results: %d/100 (%d/%d correct, avg depth: %.1f)",
		score, correct, len(positionalTests), avgDepth)

	return score, avgDepth
}

// testEndgameWithTime tests endgame technique
func testEndgameWithTime(t *testing.T, timeLimit time.Duration) (int, float64) {
	t.Log("\n👑 ENDGAME TECHNIQUE")
	t.Log("===================")

	endgameTests := []struct {
		name        string
		fen         string
		timeAlloc   time.Duration
		bestMoves   []string
		description string
	}{
		{
			name:        "King_And_Pawn",
			fen:         "8/8/8/8/8/8/4K1P1/4k3 w - - 0 1",
			timeAlloc:   5 * time.Second,
			bestMoves:   []string{"e2f3", "e2e3", "g2g4"}, // Advance king or pawn
			description: "Basic king and pawn endgame technique",
		},
		{
			name:        "Rook_Endgame",
			fen:         "8/8/8/8/8/8/4R3/4k1K1 w - - 0 1",
			timeAlloc:   5 * time.Second,
			bestMoves:   []string{"e2e1", "e2a2", "g1f2"}, // Cut off enemy king
			description: "Rook endgame cutting off enemy king",
		},
		{
			name:        "Opposition",
			fen:         "8/8/8/4k3/8/4K3/8/8 w - - 0 1",
			timeAlloc:   5 * time.Second,
			bestMoves:   []string{"e3d3", "e3f3", "e3e4"}, // Maintain or gain opposition
			description: "King opposition in pawn endgame",
		},
	}

	correct := 0
	totalDepth := 0.0

	for _, test := range endgameTests {
		pos, err := ParseFEN(test.fen)
		if err != nil {
			t.Logf("❌ %s: Failed to parse FEN", test.name)
			continue
		}

		t.Logf("Testing %s: %s", test.name, test.description)

		timeManager := NewTimeManager()
		params := SearchParams{
			MoveTime: int(test.timeAlloc.Milliseconds()),
		}
		timeManager.SetTimeControl(params, pos.Turn() == White)
		result := SearchIterativeDeepening(pos, 12, timeManager)

		if result != nil {
			move := result.BestMove.ToString()
			depth := result.Depth
			totalDepth += float64(depth)

			// Check if move is acceptable
			moveGood := false
			for _, goodMove := range test.bestMoves {
				if move == goodMove {
					moveGood = true
					break
				}
			}

			if moveGood {
				t.Logf("✅ %s: %s (depth %d) - Good endgame technique", test.name, move, depth)
				correct++
			} else {
				t.Logf("⚠️  %s: %s (depth %d) - Expected %v", test.name, move, depth, test.bestMoves)
			}
		} else {
			t.Logf("❌ %s: No result from search", test.name)
		}
	}

	avgDepth := totalDepth / float64(len(endgameTests))
	score := (correct * 100) / len(endgameTests)

	t.Logf("👑 Endgame Results: %d/100 (%d/%d correct, avg depth: %.1f)",
		score, correct, len(endgameTests), avgDepth)

	return score, avgDepth
}

// calculateComprehensiveScore calculates overall score and ELO estimate
func (r *ComprehensiveEvaluationResult) calculateComprehensiveScore() {
	// Weight tactical strength heavily since it's most important
	r.OverallScore = (r.TacticalScore*60 + r.MaterialScore*25 + r.PositionalScore*10 + r.EndgameScore*5) / 100

	// ELO calculation based on comprehensive score and depth
	baseELO := 800 + (r.OverallScore * 15) // Base scaling

	// Depth bonus - deeper search should increase ELO estimate
	depthBonus := int((r.AverageDepth - 3.0) * 50)
	if depthBonus < 0 {
		depthBonus = 0
	}

	r.EstimatedELO = baseELO + depthBonus

	// Cap at reasonable values
	if r.EstimatedELO > 2200 {
		r.EstimatedELO = 2200
	}
}

// writeEvaluationResults writes comprehensive evaluation results to output file
func writeEvaluationResults(result *ComprehensiveEvaluationResult, totalTime time.Duration, t *testing.T) {
	// Create output directory in project root
	outputDir := "../output"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Logf("Warning: Could not create output directory: %v", err)
		return
	}

	// Generate timestamped filename
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := filepath.Join(outputDir, fmt.Sprintf("eval_results_%s.txt", timestamp))

	// Create output content
	content := fmt.Sprintf(`NGN Chess Engine - Comprehensive Evaluation Results
Generated: %s
========================================================

🎯 OVERALL RESULTS
==================
Total test time: %.1f seconds
Average search depth: %.1f
OVERALL SCORE: %d/100
ESTIMATED ELO: %d

📊 DETAILED BREAKDOWN
=====================
Tactical Strength:     %d/100
Material Evaluation:   %d/100  
Positional Play:       %d/100
Endgame Technique:     %d/100

✅ STATUS
=========
`,
		time.Now().Format("2006-01-02 15:04:05"),
		totalTime.Seconds(),
		result.AverageDepth,
		result.OverallScore,
		result.EstimatedELO,
		result.TacticalScore,
		result.MaterialScore,
		result.PositionalScore,
		result.EndgameScore,
	)

	if result.EstimatedELO >= 1400 {
		content += fmt.Sprintf("ENGINE MEETS MINIMUM STANDARD: %d ELO (≥1400 required) ✅\n", result.EstimatedELO)
	} else {
		content += fmt.Sprintf("ENGINE BELOW MINIMUM: %d ELO (≥1400 required) ❌\n", result.EstimatedELO)
	}

	if len(result.WeaknessReport) > 0 {
		content += "\n🚨 IDENTIFIED WEAKNESSES\n========================\n"
		for i, weakness := range result.WeaknessReport {
			content += fmt.Sprintf("%d. %s\n", i+1, weakness)
		}
	}

	// Write to file
	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		t.Logf("Warning: Could not write evaluation results: %v", err)
		return
	}

	t.Logf("📄 Evaluation results written to: %s", filename)
}
