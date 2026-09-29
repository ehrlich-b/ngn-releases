package engine

import (
	"fmt"
	"math/rand"
	"testing"
)

// PlayQualityScore represents the overall quality assessment
type PlayQualityScore struct {
	TacticalScore   int      // 0-100, tactical awareness
	PositionalScore int      // 0-100, positional understanding
	MaterialScore   int      // 0-100, material evaluation accuracy
	EndgameScore    int      // 0-100, endgame technique
	OverallScore    int      // 0-100, weighted average
	EstimatedELO    int      // Estimated playing strength
	WeaknessReport  []string // List of detected weaknesses
}

// TestEnginePlayingStrength runs comprehensive evaluation of engine quality
func TestEnginePlayingStrength(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping comprehensive play quality test in short mode")
	}

	t.Log("🧪 COMPREHENSIVE ENGINE QUALITY EVALUATION")
	t.Log("================================================")

	score := &PlayQualityScore{
		WeaknessReport: make([]string, 0),
	}

	// Run all quality tests
	score.TacticalScore = testTacticalAwareness(t)
	score.MaterialScore = testMaterialEvaluation(t, score)
	score.PositionalScore = testPositionalUnderstanding(t, score)
	score.EndgameScore = testEndgameTechnique(t, score)

	// Calculate overall score and ELO estimate
	score.calculateOverallScore()

	// Report results
	t.Log("📊 FINAL ENGINE QUALITY REPORT")
	t.Log("==============================")
	t.Logf("Tactical Awareness:    %d/100", score.TacticalScore)
	t.Logf("Material Evaluation:   %d/100", score.MaterialScore)
	t.Logf("Positional Play:       %d/100", score.PositionalScore)
	t.Logf("Endgame Technique:     %d/100", score.EndgameScore)
	t.Logf("OVERALL SCORE:         %d/100", score.OverallScore)
	t.Logf("ESTIMATED ELO:         %d", score.EstimatedELO)

	if len(score.WeaknessReport) > 0 {
		t.Log("\n🚨 CRITICAL WEAKNESSES DETECTED:")
		for i, weakness := range score.WeaknessReport {
			t.Logf("%d. %s", i+1, weakness)
		}
	}

	// Set expectations
	if score.EstimatedELO < 1200 {
		t.Errorf("❌ ENGINE PLAYING STRENGTH CRITICAL: Estimated %d ELO (expected >1200)", score.EstimatedELO)
	} else if score.EstimatedELO < 1600 {
		t.Logf("⚠️  ENGINE BELOW TARGET: Estimated %d ELO (target 1800+)", score.EstimatedELO)
	} else {
		t.Logf("✅ ENGINE MEETS EXPECTATIONS: Estimated %d ELO", score.EstimatedELO)
	}
}

// testTacticalAwareness tests basic tactical pattern recognition
func testTacticalAwareness(t *testing.T) int {
	t.Log("\n🎯 Testing Tactical Awareness...")

	tacticalPuzzles := []struct {
		name        string
		fen         string
		bestMoves   []string // Expected good moves
		badMoves    []string // Moves that should NOT be chosen
		description string
	}{
		{
			name:        "Simple_Material_Win",
			fen:         "rnbqk2r/pppp1ppp/5n2/2b1p3/2B1P3/3P1N2/PPP2PPP/RNBQK2R w KQkq - 0 4",
			bestMoves:   []string{"Bxf7+"},          // Free pawn with check
			badMoves:    []string{"a3", "h3", "b3"}, // Random pawn moves
			description: "Should capture free pawn with check",
		},
		{
			name:        "Avoid_Material_Loss",
			fen:         "rnbqkbnr/pppp1ppp/8/4p3/4P3/8/PPPP1PPP/RNBQKBNR w KQkq e6 0 2",
			bestMoves:   []string{"Nf3", "Nc3", "d3", "f4"}, // Normal development
			badMoves:    []string{"Qh5"},                    // Hanging queen to g5
			description: "Should avoid hanging pieces",
		},
		{
			name:        "Basic_Fork",
			fen:         "rnbqkb1r/pppp1ppp/5n2/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R w KQkq - 2 3",
			bestMoves:   []string{"Ng5"},            // Knight fork threat
			badMoves:    []string{"h3", "a3", "b3"}, // Useless moves
			description: "Should see knight fork opportunities",
		},
		{
			name:        "Defend_Attacked_Piece",
			fen:         "rnbqkbnr/ppp2ppp/8/3pp3/3PP3/8/PPP2PPP/RNBQKBNR w KQkq d6 0 3",
			bestMoves:   []string{"Nc3", "Bd2", "f4"}, // Support the center
			badMoves:    []string{"h3", "a3", "g3"},   // Ignore the center
			description: "Should defend attacked pieces",
		},
		{
			name:        "Simple_Pin",
			fen:         "rnbqk2r/pppp1ppp/5n2/2b1p3/2B1P3/8/PPPP1PPP/RNBQKBNR w KQkq - 0 4",
			bestMoves:   []string{"d3", "f4", "Nf3"}, // Normal moves
			badMoves:    []string{"f3"},              // Allows devastating pin
			description: "Should avoid creating pins",
		},
	}

	correctCount := 0
	totalPuzzles := len(tacticalPuzzles)

	for _, puzzle := range tacticalPuzzles {
		t.Run(puzzle.name, func(t *testing.T) {
			pos, err := ParseFEN(puzzle.fen)
			if err != nil {
				t.Fatalf("Failed to parse FEN: %v", err)
			}

			// Get engine's best move
			result := Search(pos, 4) // Reasonable depth for tactics
			if result == nil {
				t.Error("Search returned nil")
				return
			}

			engineMove := result.BestMove.ToString()

			// Check if engine chose a good move
			choseGoodMove := false
			for _, goodMove := range puzzle.bestMoves {
				if engineMove == goodMove {
					choseGoodMove = true
					break
				}
			}

			// Check if engine chose a bad move
			choseBadMove := false
			for _, badMove := range puzzle.badMoves {
				if engineMove == badMove {
					choseBadMove = true
					break
				}
			}

			if choseGoodMove {
				t.Logf("✅ CORRECT: %s - chose %s (expected: %v)", puzzle.description, engineMove, puzzle.bestMoves)
				correctCount++
			} else if choseBadMove {
				t.Errorf("❌ TERRIBLE: %s - chose %s (explicitly bad move!)", puzzle.description, engineMove)
			} else {
				t.Logf("⚠️  SUBOPTIMAL: %s - chose %s (expected: %v)", puzzle.description, engineMove, puzzle.bestMoves)
			}
		})
	}

	tacticalScore := (correctCount * 100) / totalPuzzles
	t.Logf("🎯 Tactical Score: %d/100 (%d/%d puzzles correct)", tacticalScore, correctCount, totalPuzzles)

	return tacticalScore
}

// testMaterialEvaluation tests if engine properly values material
func testMaterialEvaluation(t *testing.T, score *PlayQualityScore) int {
	t.Log("\n💎 Testing Material Evaluation...")

	materialTests := []struct {
		name           string
		fen            string
		expectedWinner string // "white", "black", or "equal"
		description    string
	}{
		{
			name:           "Queen_vs_Rook",
			fen:            "8/8/8/8/8/8/4Q3/4K2k b - - 0 1",
			expectedWinner: "white",
			description:    "Queen should be worth more than rook",
		},
		{
			name:           "Rook_vs_Bishop_Knight",
			fen:            "4k1n1/3b4/8/8/8/8/4R3/4K3 w - - 0 1",
			expectedWinner: "equal", // White R vs Black B+N (5 vs 3+3)
			description:    "Rook vs Bishop+Knight roughly equal",
		},
		{
			name:           "Three_Pawns_vs_Knight",
			fen:            "8/8/8/8/8/2PPP3/8/1N2K2k w - - 0 1",
			expectedWinner: "white", // 3 pawns(3) vs knight(3), but pawns can promote
			description:    "Three pawns vs knight",
		},
		{
			name:           "Queen_vs_Minor_Pieces",
			fen:            "8/8/8/8/8/8/1BN5/1Q2K2k w - - 0 1",
			expectedWinner: "white",
			description:    "Queen vs Bishop+Knight",
		},
		{
			name:           "Material_Balance",
			fen:            "r1bqkb1r/pppp1ppp/2n2n2/4p3/2B1P3/3P1N2/PPP2PPP/RNBQK2R w KQkq - 0 4",
			expectedWinner: "equal",
			description:    "Balanced material",
		},
	}

	correctEvaluations := 0

	for _, test := range materialTests {
		t.Run(test.name, func(t *testing.T) {
			pos, err := ParseFEN(test.fen)
			if err != nil {
				t.Fatalf("Failed to parse FEN: %v", err)
			}

			evaluation := Evaluate(&pos.Board)

			var engineWinner string
			if evaluation > 50 {
				engineWinner = "white"
			} else if evaluation < -50 {
				engineWinner = "black"
			} else {
				engineWinner = "equal"
			}

			if engineWinner == test.expectedWinner {
				t.Logf("✅ CORRECT: %s - eval %d (%s wins)", test.description, evaluation, engineWinner)
				correctEvaluations++
			} else {
				t.Errorf("❌ WRONG: %s - eval %d (engine thinks %s wins, expected %s)",
					test.description, evaluation, engineWinner, test.expectedWinner)
				score.WeaknessReport = append(score.WeaknessReport,
					fmt.Sprintf("Material evaluation error: %s", test.description))
			}
		})
	}

	materialScore := (correctEvaluations * 100) / len(materialTests)
	t.Logf("💎 Material Score: %d/100 (%d/%d correct)", materialScore, correctEvaluations, len(materialTests))

	return materialScore
}

// testPositionalUnderstanding tests basic positional principles
func testPositionalUnderstanding(t *testing.T, score *PlayQualityScore) int {
	t.Log("\n🏰 Testing Positional Understanding...")

	positionalTests := []struct {
		name        string
		fen         string
		goodMoves   []string
		badMoves    []string
		description string
	}{
		{
			name:        "Castle_Safety",
			fen:         "rnbqk2r/pppp1ppp/5n2/2b1p3/2B1P3/3P1N2/PPP2PPP/RNBQK2R w KQkq - 0 4",
			goodMoves:   []string{"O-O", "Castles"}, // Castle for safety
			badMoves:    []string{"Kf1", "Ke2"},     // Walking into center
			description: "Should castle for king safety",
		},
		{
			name:        "Center_Control",
			fen:         "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
			goodMoves:   []string{"e4", "d4"},        // Control center
			badMoves:    []string{"a4", "h4", "Na3"}, // Edge play
			description: "Should control the center",
		},
		{
			name:        "Develop_Pieces",
			fen:         "rnbqkbnr/pppp1ppp/8/4p3/4P3/8/PPPP1PPP/RNBQKBNR w KQkq e6 0 2",
			goodMoves:   []string{"Nf3", "Nc3", "Bc4", "Be2"}, // Development
			badMoves:    []string{"Qh5", "Qf3"},               // Early queen
			description: "Should develop pieces before queen",
		},
		{
			name:        "Avoid_Weak_Squares",
			fen:         "rnbqkb1r/pppp1ppp/5n2/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R w KQkq - 2 3",
			goodMoves:   []string{"d3", "Bb5", "Bc4"}, // Solid development
			badMoves:    []string{"g4", "h4"},         // Weakening moves
			description: "Should avoid creating weak squares",
		},
	}

	correctPositional := 0

	for _, test := range positionalTests {
		t.Run(test.name, func(t *testing.T) {
			pos, err := ParseFEN(test.fen)
			if err != nil {
				t.Fatalf("Failed to parse FEN: %v", err)
			}

			result := Search(pos, 3)
			if result == nil {
				t.Error("Search returned nil")
				return
			}

			engineMove := result.BestMove.ToString()

			choseGoodMove := false
			for _, goodMove := range test.goodMoves {
				if engineMove == goodMove {
					choseGoodMove = true
					break
				}
			}

			choseBadMove := false
			for _, badMove := range test.badMoves {
				if engineMove == badMove {
					choseBadMove = true
					break
				}
			}

			if choseGoodMove {
				t.Logf("✅ GOOD: %s - chose %s", test.description, engineMove)
				correctPositional++
			} else if choseBadMove {
				t.Errorf("❌ BAD: %s - chose %s (bad positional play)", test.description, engineMove)
				score.WeaknessReport = append(score.WeaknessReport,
					fmt.Sprintf("Poor positional play: %s", test.description))
			} else {
				t.Logf("⚠️  NEUTRAL: %s - chose %s", test.description, engineMove)
			}
		})
	}

	positionalScore := (correctPositional * 100) / len(positionalTests)
	t.Logf("🏰 Positional Score: %d/100 (%d/%d good positions)", positionalScore, correctPositional, len(positionalTests))

	return positionalScore
}

// testEndgameTechnique tests basic endgame knowledge
func testEndgameTechnique(t *testing.T, score *PlayQualityScore) int {
	t.Log("\n👑 Testing Endgame Technique...")

	endgameTests := []struct {
		name        string
		fen         string
		goodMoves   []string
		badMoves    []string
		description string
	}{
		{
			name:        "King_Activity_Endgame",
			fen:         "8/8/8/8/8/8/4K3/4k3 w - - 0 1",
			goodMoves:   []string{"Kd3", "Ke3", "Kf3"}, // Activate king
			badMoves:    []string{"Kd1", "Kf1"},        // Passive play
			description: "Should activate king in endgame",
		},
		{
			name:        "Pawn_Promotion_Race",
			fen:         "8/P7/8/8/8/8/7p/7K w - - 0 1",
			goodMoves:   []string{"a8=Q"},                 // Promote to queen
			badMoves:    []string{"a8=R", "a8=B", "a8=N"}, // Under-promote
			description: "Should promote to queen when possible",
		},
		{
			name:        "King_and_Pawn_vs_King",
			fen:         "8/8/8/8/8/2K5/2P5/2k5 w - - 0 1",
			goodMoves:   []string{"Kd4", "Kb4"}, // Support pawn
			badMoves:    []string{"c4", "c3"},   // Push too early
			description: "Should bring king up to support pawn",
		},
	}

	correctEndgame := 0

	for _, test := range endgameTests {
		t.Run(test.name, func(t *testing.T) {
			pos, err := ParseFEN(test.fen)
			if err != nil {
				t.Fatalf("Failed to parse FEN: %v", err)
			}

			result := Search(pos, 4) // Need deeper search for endgames
			if result == nil {
				t.Error("Search returned nil")
				return
			}

			engineMove := result.BestMove.ToString()

			choseGoodMove := false
			for _, goodMove := range test.goodMoves {
				if engineMove == goodMove {
					choseGoodMove = true
					break
				}
			}

			choseBadMove := false
			for _, badMove := range test.badMoves {
				if engineMove == badMove {
					choseBadMove = true
					break
				}
			}

			if choseGoodMove {
				t.Logf("✅ STRONG: %s - chose %s", test.description, engineMove)
				correctEndgame++
			} else if choseBadMove {
				t.Errorf("❌ WEAK: %s - chose %s (poor endgame technique)", test.description, engineMove)
				score.WeaknessReport = append(score.WeaknessReport,
					fmt.Sprintf("Weak endgame play: %s", test.description))
			} else {
				t.Logf("⚠️  OKAY: %s - chose %s", test.description, engineMove)
			}
		})
	}

	endgameScore := (correctEndgame * 100) / len(endgameTests)
	t.Logf("👑 Endgame Score: %d/100 (%d/%d correct)", endgameScore, correctEndgame, len(endgameTests))

	return endgameScore
}

// calculateOverallScore computes final rating and ELO estimate
func (pqs *PlayQualityScore) calculateOverallScore() {
	// Weighted average (tactics most important for rating)
	pqs.OverallScore = (pqs.TacticalScore*40 + pqs.MaterialScore*30 + pqs.PositionalScore*20 + pqs.EndgameScore*10) / 100

	// ELO estimation based on overall performance
	// Base rating 800, with 20 points per percentage point
	baseELO := 800
	pqs.EstimatedELO = baseELO + (pqs.OverallScore * 15)

	// Adjust for specific weaknesses
	if pqs.MaterialScore < 50 {
		pqs.EstimatedELO -= 200 // Major material blindness penalty
	}
	if pqs.TacticalScore < 30 {
		pqs.EstimatedELO -= 300 // Severe tactical blindness penalty
	}

	// Cap at reasonable ranges
	if pqs.EstimatedELO < 600 {
		pqs.EstimatedELO = 600
	}
	if pqs.EstimatedELO > 2200 {
		pqs.EstimatedELO = 2200
	}
}

// TestRandomGameAgainstBaseline plays games against simple opponents
func TestRandomGameAgainstBaseline(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping baseline game test in short mode")
	}

	t.Log("🎮 Testing Engine vs Simple Opponents...")

	// Test against pure random play
	randomWins := 0
	numGames := 10

	for i := 0; i < numGames; i++ {
		winner := playGameVsRandom(t)
		if winner == "engine" {
			randomWins++
		}
	}

	winRate := (randomWins * 100) / numGames
	t.Logf("🎮 Engine vs Random: %d/%d wins (%d%% win rate)", randomWins, numGames, winRate)

	if winRate < 80 {
		t.Errorf("❌ ENGINE TOO WEAK: Only %d%% win rate vs random (expected >80%%)", winRate)
	} else {
		t.Logf("✅ Engine dominates random play: %d%% win rate", winRate)
	}
}

// playGameVsRandom plays a single game against random opponent
func playGameVsRandom(t *testing.T) string {
	pos, _ := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")

	moveCount := 0
	maxMoves := 100

	for moveCount < maxMoves {
		moves := GenerateLegalMoves(pos)
		if len(moves) == 0 {
			break
		}

		var move Move
		if pos.Turn() == White {
			// Engine plays white - use search
			result := Search(pos, 2) // Shallow search for speed
			if result == nil || result.BestMove == EmptyMove {
				// Fallback to random if search fails
				move = moves[rand.Intn(len(moves))]
			} else {
				move = result.BestMove
			}
		} else {
			// Random player plays black
			move = moves[rand.Intn(len(moves))]
		}

		pos.MakeMove(move)
		moveCount++
	}

	// Simple material count to determine winner
	whiteScore := countMaterial(&pos.Board, White)
	blackScore := countMaterial(&pos.Board, Black)

	if whiteScore > blackScore+200 { // Engine playing white wins
		return "engine"
	} else if blackScore > whiteScore+200 {
		return "random"
	}

	return "draw"
}

// countMaterial counts material for simple evaluation
func countMaterial(board *Bitboard, color Color) int {
	score := 0

	if color == White {
		score += PopCount(board.GetBitboardOf(WhitePawn)) * 100
		score += PopCount(board.GetBitboardOf(WhiteKnight)) * 300
		score += PopCount(board.GetBitboardOf(WhiteBishop)) * 300
		score += PopCount(board.GetBitboardOf(WhiteRook)) * 500
		score += PopCount(board.GetBitboardOf(WhiteQueen)) * 900
	} else {
		score += PopCount(board.GetBitboardOf(BlackPawn)) * 100
		score += PopCount(board.GetBitboardOf(BlackKnight)) * 300
		score += PopCount(board.GetBitboardOf(BlackBishop)) * 300
		score += PopCount(board.GetBitboardOf(BlackRook)) * 500
		score += PopCount(board.GetBitboardOf(BlackQueen)) * 900
	}

	return score
}

// TestEngineConsistency tests if engine makes consistent choices
func TestEngineConsistency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping consistency test in short mode")
	}

	t.Log("🔄 Testing Engine Consistency...")

	// Test same position multiple times
	pos, _ := ParseFEN("rnbqkbnr/pppp1ppp/8/4p3/4P3/8/PPPP1PPP/RNBQKBNR w KQkq e6 0 2")

	moves := make([]string, 5)
	for i := 0; i < 5; i++ {
		result := Search(pos, 3)
		if result != nil {
			moves[i] = result.BestMove.ToString()
		}
	}

	// Check if all moves are the same
	allSame := true
	firstMove := moves[0]
	for _, move := range moves {
		if move != firstMove {
			allSame = false
			break
		}
	}

	if allSame {
		t.Logf("✅ Engine is consistent: always plays %s", firstMove)
	} else {
		t.Logf("⚠️  Engine shows inconsistency: %v", moves)
	}
}
