// SPDX-License-Identifier: GPL-3.0-or-later
// Test-only transition oracle around CounterGo commit 63c487ca724c620f71c129d62129c6fb9109c872.
//go:build ngn_counter_transition_oracle

package eval

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"runtime"
	"strings"
	"testing"

	. "github.com/ChizhovVadim/CounterGo/pkg/common"
)

type transitionFixture struct {
	ID         string
	FEN        string
	Operations []string
}

type transitionUpdate struct {
	Feature     int  `json:"feature"`
	Coefficient int8 `json:"coefficient"`
}

type transitionRecord struct {
	ID              string             `json:"id"`
	Sequence        string             `json:"sequence"`
	Step            int                `json:"step"`
	Operation       string             `json:"operation"`
	Move            string             `json:"move,omitempty"`
	Depth           int                `json:"depth"`
	PreFEN          string             `json:"pre_fen,omitempty"`
	PostFEN         string             `json:"post_fen"`
	Restores        string             `json:"restores,omitempty"`
	Updates         []transitionUpdate `json:"updates"`
	Features        []int              `json:"features"`
	AccumulatorBits []uint32           `json:"accumulator_bits"`
	RawBits         uint32             `json:"raw_white_bits"`
}

type transitionOracleOutput struct {
	Schema        string             `json:"schema"`
	CounterCommit string             `json:"counter_commit"`
	GoVersion     string             `json:"go_version"`
	GOARCH        string             `json:"goarch"`
	ModelSHA256   string             `json:"model_sha256"`
	Records       []transitionRecord `json:"records"`
}

func TestNGNCounter55TransitionOracle(t *testing.T) {
	modelPath := requiredTransitionEnv(t, "COUNTER_MODEL")
	fixturePath := requiredTransitionEnv(t, "COUNTER_TRANSITIONS")
	outputPath := requiredTransitionEnv(t, "COUNTER_TRANSITION_ORACLE_OUTPUT")

	modelBytes, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	modelFile, err := os.Open(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	weights, err := LoadWeights(modelFile)
	closeErr := modelFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	fixtures, err := readTransitionFixtures(fixturePath)
	if err != nil {
		t.Fatal(err)
	}

	output := transitionOracleOutput{
		Schema:        "counter55-transition-oracle-v1",
		CounterCommit: "63c487ca724c620f71c129d62129c6fb9109c872",
		GoVersion:     runtime.Version(),
		GOARCH:        runtime.GOARCH,
		ModelSHA256:   transitionOracleSHA256(modelBytes),
	}
	for _, fixture := range fixtures {
		position, err := NewPositionFromFEN(fixture.FEN)
		if err != nil {
			t.Fatalf("%s root: %v", fixture.ID, err)
		}
		evaluator := NewEvaluationService(weights)
		evaluator.Init(&position)
		positions := []Position{position}
		frameRecords := []string{fixture.ID + "/root"}
		output.Records = append(output.Records, captureTransitionRecord(
			fixture.ID+"/root", fixture.ID, 0, "root", "", "", position.String(),
			"", evaluator, &position,
		))

		for step, token := range fixture.Operations {
			recordID := fmt.Sprintf("%s/%02d-%s", fixture.ID, step+1, token)
			current := &positions[len(positions)-1]
			preFEN := current.String()
			switch token {
			case "pop":
				if len(positions) == 1 {
					t.Fatalf("%s step %d: root pop", fixture.ID, step+1)
				}
				evaluator.UnmakeMove()
				positions = positions[:len(positions)-1]
				frameRecords = frameRecords[:len(frameRecords)-1]
				current = &positions[len(positions)-1]
				output.Records = append(output.Records, captureTransitionRecord(
					recordID, fixture.ID, step+1, "pop", "", preFEN, current.String(),
					frameRecords[len(frameRecords)-1], evaluator, current,
				))
			case "null":
				var child Position
				current.MakeNullMove(&child)
				evaluator.MakeMove(current, MoveEmpty)
				positions = append(positions, child)
				frameRecords = append(frameRecords, recordID)
				current = &positions[len(positions)-1]
				output.Records = append(output.Records, captureTransitionRecord(
					recordID, fixture.ID, step+1, "null", "", preFEN, current.String(),
					"", evaluator, current,
				))
			default:
				move, child, err := findTransitionMove(current, token)
				if err != nil {
					t.Fatalf("%s step %d: %v", fixture.ID, step+1, err)
				}
				evaluator.MakeMove(current, move)
				positions = append(positions, child)
				frameRecords = append(frameRecords, recordID)
				current = &positions[len(positions)-1]
				output.Records = append(output.Records, captureTransitionRecord(
					recordID, fixture.ID, step+1, "move", token, preFEN, current.String(),
					"", evaluator, current,
				))
			}
		}
	}
	encoded, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(outputPath, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
}

func captureTransitionRecord(
	id, sequence string,
	step int,
	operation, move, preFEN, postFEN, restores string,
	evaluator *EvaluationService,
	position *Position,
) transitionRecord {
	var updates []transitionUpdate
	if operation == "move" {
		updates = make([]transitionUpdate, evaluator.updates.Size)
		for i := range updates {
			updates[i] = transitionUpdate{
				Feature:     int(evaluator.updates.Indices[i]),
				Coefficient: evaluator.updates.Coeffs[i],
			}
		}
	}
	features := make([]int, 0, 64)
	for square := 0; square < 64; square++ {
		piece, white := position.GetPieceTypeAndSide(square)
		if piece != Empty {
			features = append(features, int(calculateNetInputIndex(white, piece, square)))
		}
	}
	accumulatorBits := make([]uint32, HiddenSize)
	for lane, value := range evaluator.hiddenOutputs[evaluator.currentHidden] {
		accumulatorBits[lane] = math.Float32bits(value)
	}
	return transitionRecord{
		ID: id, Sequence: sequence, Step: step, Operation: operation, Move: move,
		Depth: evaluator.currentHidden, PreFEN: preFEN, PostFEN: postFEN,
		Restores: restores, Updates: updates, Features: features,
		AccumulatorBits: accumulatorBits,
		RawBits:         math.Float32bits(evaluator.QuickFeed()),
	}
}

func findTransitionMove(position *Position, uci string) (Move, Position, error) {
	var buffer [MaxMoves]OrderedMove
	for _, ordered := range position.GenerateMoves(buffer[:]) {
		move := ordered.Move
		if !strings.EqualFold(move.String(), uci) {
			continue
		}
		var child Position
		if position.MakeMove(move, &child) {
			return move, child, nil
		}
		return MoveEmpty, Position{}, fmt.Errorf("move %s generated but illegal", uci)
	}
	return MoveEmpty, Position{}, fmt.Errorf("move %s not generated", uci)
}

func readTransitionFixtures(path string) ([]transitionFixture, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var result []transitionFixture
	seen := make(map[string]bool)
	scanner := bufio.NewScanner(file)
	for line := 1; scanner.Scan(); line++ {
		text := scanner.Text()
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		parts := strings.Split(text, "\t")
		if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
			return nil, fmt.Errorf("transition fixture line %d: expected ID<TAB>FEN<TAB>operations", line)
		}
		if seen[parts[0]] {
			return nil, fmt.Errorf("transition fixture line %d: duplicate ID %q", line, parts[0])
		}
		seen[parts[0]] = true
		result = append(result, transitionFixture{
			ID: parts[0], FEN: parts[1], Operations: strings.Fields(parts[2]),
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no transition fixtures")
	}
	return result, nil
}

func transitionOracleSHA256(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func requiredTransitionEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required", name)
	}
	return value
}
