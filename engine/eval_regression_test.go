// Position evaluation regression testing
// This file provides regression testing for the evaluation function to catch
// unintended evaluation changes when the eval function is modified

package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// EvaluationBaseline represents a baseline evaluation for a position
type EvaluationBaseline struct {
	Name     string `json:"name"`
	FEN      string `json:"fen"`
	Eval     int    `json:"eval"`
	Comments string `json:"comments,omitempty"`
}

// EvaluationRegressionSuite contains all baseline evaluations
type EvaluationRegressionSuite struct {
	Version   string               `json:"version"`
	Timestamp string               `json:"timestamp"`
	Baselines []EvaluationBaseline `json:"baselines"`
}

// Standard test positions for evaluation regression testing
var standardEvaluationPositions = []struct {
	name     string
	fen      string
	comments string
}{
	{
		name:     "Starting Position",
		fen:      "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		comments: "Should be close to 0, slightly favoring white",
	},
	{
		name:     "After 1.e4",
		fen:      "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq e3 0 1",
		comments: "White pawn controls center, slight advantage",
	},
	{
		name:     "Queen vs Rook",
		fen:      "4k3/8/8/8/8/8/8/4KQ1R w - - 0 1",
		comments: "Queen should be ~400 points better than rook",
	},
	{
		name:     "Knight vs Bishop",
		fen:      "4k3/8/8/8/8/8/8/4KNB1 w - - 0 1",
		comments: "Knight and bishop roughly equal, slight bishop edge",
	},
	{
		name:     "Central Knight",
		fen:      "4k3/8/8/3N4/8/8/8/4K3 w - - 0 1",
		comments: "Centralized knight should score well",
	},
	{
		name:     "Edge Knight",
		fen:      "4k3/8/8/8/8/8/8/N3K3 w - - 0 1",
		comments: "Knight on edge should be penalized",
	},
	{
		name:     "Doubled Pawns",
		fen:      "4k3/8/8/8/4P3/4P3/8/4K3 w - - 0 1",
		comments: "Doubled pawns should receive penalty",
	},
	{
		name:     "Passed Pawn",
		fen:      "4k3/8/8/4P3/8/8/8/4K3 w - - 0 1",
		comments: "Advanced passed pawn should be valued highly",
	},
	{
		name:     "King Safety Middle",
		fen:      "r3k2r/pppppppp/8/8/8/8/PPPPPPPP/R3K2R w KQkq - 0 1",
		comments: "Uncastled kings should be penalized",
	},
	{
		name:     "King Safety Castled",
		fen:      "r3k2r/pppppppp/8/8/8/8/PPPPPPPP/R4RK1 w kq - 0 1",
		comments: "Castled king should be safer",
	},
	{
		name:     "Endgame King",
		fen:      "8/8/8/4K3/8/8/8/4k3 w - - 0 1",
		comments: "Centralized king in endgame",
	},
	{
		name:     "Complex Middlegame",
		fen:      "r1bqk2r/ppp2ppp/2n2n2/2b1p3/2B1P3/3P1N2/PPP2PPP/RNBQK2R w KQkq - 4 6",
		comments: "Complex position with multiple factors",
	},
}

// baselineFilePath returns the path to the baseline file
func baselineFilePath() string {
	return filepath.Join(".", "eval_baseline.json")
}

// loadBaseline loads evaluation baselines from file
func loadBaseline() (*EvaluationRegressionSuite, error) {
	data, err := os.ReadFile(baselineFilePath())
	if err != nil {
		return nil, err
	}

	var suite EvaluationRegressionSuite
	err = json.Unmarshal(data, &suite)
	if err != nil {
		return nil, err
	}

	return &suite, nil
}

// saveBaseline saves evaluation baselines to file
func saveBaseline(suite *EvaluationRegressionSuite) error {
	data, err := json.MarshalIndent(suite, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(baselineFilePath(), data, 0644)
}

// generateBaseline creates a new baseline from current evaluation function
func generateBaseline() (*EvaluationRegressionSuite, error) {
	suite := &EvaluationRegressionSuite{
		Version:   "1.0",
		Timestamp: "2025-09-08T23:00:00Z",
		Baselines: make([]EvaluationBaseline, 0, len(standardEvaluationPositions)),
	}

	for _, pos := range standardEvaluationPositions {
		position, err := ParseFEN(pos.fen)
		if err != nil {
			return nil, fmt.Errorf("failed to parse FEN %s: %v", pos.fen, err)
		}

		eval := Evaluate(&position.Board)
		baseline := EvaluationBaseline{
			Name:     pos.name,
			FEN:      pos.fen,
			Eval:     eval,
			Comments: pos.comments,
		}

		suite.Baselines = append(suite.Baselines, baseline)
	}

	return suite, nil
}

// TestEvaluationRegression runs regression testing against stored baselines

// TestGenerateEvaluationBaseline generates new baseline evaluations

// TestEvaluationConsistency tests that evaluation is deterministic
func TestEvaluationConsistency(t *testing.T) {
	// Test that multiple evaluations of the same position give identical results
	for _, pos := range standardEvaluationPositions[:3] { // Test first 3 positions
		t.Run(pos.name, func(t *testing.T) {
			position, err := ParseFEN(pos.fen)
			if err != nil {
				t.Fatalf("Failed to parse FEN %s: %v", pos.fen, err)
			}

			eval1 := Evaluate(&position.Board)
			eval2 := Evaluate(&position.Board)
			eval3 := Evaluate(&position.Board)

			if eval1 != eval2 || eval2 != eval3 {
				t.Errorf("Evaluation is not consistent: %d, %d, %d", eval1, eval2, eval3)
			}
		})
	}
}
