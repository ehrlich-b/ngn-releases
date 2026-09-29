// SPDX-License-Identifier: GPL-3.0-or-later
// Test-only adapter around CounterGo commit 63c487ca724c620f71c129d62129c6fb9109c872.
//go:build ngn_counter_oracle

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

type oracleFixture struct {
	ID  string
	FEN string
}

type oracleRecord struct {
	ID              string   `json:"id"`
	FEN             string   `json:"fen"`
	Canonical       string   `json:"counter_canonical"`
	WhiteMove       bool     `json:"white_move"`
	Rule50          int      `json:"rule50"`
	NPMaterial      int      `json:"np_material"`
	Features        []int    `json:"features"`
	AccumulatorBits []uint32 `json:"accumulator_bits"`
	Raw             float32  `json:"raw_white"`
	RawBits         uint32   `json:"raw_white_bits"`
	Adapted         int      `json:"adapted_stm"`
}

type oracleAdapterCase struct {
	ID         string  `json:"id"`
	Raw        float32 `json:"raw_white"`
	RawBits    uint32  `json:"raw_white_bits"`
	WhiteMove  bool    `json:"white_move"`
	Rule50     int     `json:"rule50"`
	NPMaterial int     `json:"np_material"`
	Adapted    int     `json:"adapted_stm"`
}

type oracleOutput struct {
	Schema        string              `json:"schema"`
	CounterCommit string              `json:"counter_commit"`
	GoVersion     string              `json:"go_version"`
	GOARCH        string              `json:"goarch"`
	ModelSHA256   string              `json:"model_sha256"`
	Records       []oracleRecord      `json:"records"`
	AdapterCases  []oracleAdapterCase `json:"adapter_cases"`
}

func TestNGNCounter55Oracle(t *testing.T) {
	modelPath := requiredEnv(t, "COUNTER_MODEL")
	fixturePath := requiredEnv(t, "COUNTER_FENS")
	outputPath := requiredEnv(t, "COUNTER_ORACLE_OUTPUT")

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

	fixtures, err := readOracleFixtures(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	output := oracleOutput{
		Schema:        "counter55-portable-oracle-v1",
		CounterCommit: "63c487ca724c620f71c129d62129c6fb9109c872",
		GoVersion:     runtime.Version(),
		GOARCH:        runtime.GOARCH,
		ModelSHA256:   hex.EncodeToString(sha256Sum(modelBytes)),
		Records:       make([]oracleRecord, 0, len(fixtures)),
		AdapterCases:  buildAdapterCases(),
	}
	for _, fixture := range fixtures {
		position, err := NewPositionFromFEN(fixture.FEN)
		if err != nil {
			t.Fatalf("fixture %s: %v", fixture.ID, err)
		}
		evaluator := NewEvaluationService(weights)
		evaluator.Init(&position)
		features := make([]int, 0, 32)
		for square := 0; square < 64; square++ {
			piece, white := position.GetPieceTypeAndSide(square)
			if piece != Empty {
				features = append(features, int(calculateNetInputIndex(white, piece, square)))
			}
		}
		accumulatorBits := make([]uint32, HiddenSize)
		for i, value := range evaluator.hiddenOutputs[evaluator.currentHidden] {
			accumulatorBits[i] = math.Float32bits(value)
		}
		raw := evaluator.QuickFeed()
		npMaterial := 4*PopCount(position.Knights|position.Bishops) + 6*PopCount(position.Rooks) + 12*PopCount(position.Queens)
		output.Records = append(output.Records, oracleRecord{
			ID:              fixture.ID,
			FEN:             fixture.FEN,
			Canonical:       position.String(),
			WhiteMove:       position.WhiteMove,
			Rule50:          position.Rule50,
			NPMaterial:      npMaterial,
			Features:        features,
			AccumulatorBits: accumulatorBits,
			Raw:             raw,
			RawBits:         math.Float32bits(raw),
			Adapted:         evaluator.EvaluateQuick(&position),
		})
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

func buildAdapterCases() []oracleAdapterCase {
	type spec struct {
		id        string
		raw       float32
		whiteMove bool
		rule50    int
	}
	specs := []spec{
		{id: "positive_fraction", raw: 1.75, whiteMove: true, rule50: 0},
		{id: "negative_fraction", raw: -1.75, whiteMove: false, rule50: 0},
		{id: "positive_clip_before_scale", raw: 20_000.75, whiteMove: true, rule50: 0},
		{id: "negative_clip_before_scale", raw: -20_000.75, whiteMove: true, rule50: 0},
		{id: "sequential_division", raw: -14_995.75, whiteMove: true, rule50: 99},
		{id: "black_sign", raw: 7_654.25, whiteMove: false, rule50: 37},
	}
	result := make([]oracleAdapterCase, 0, len(specs))
	for _, item := range specs {
		position := Position{
			WhiteMove: item.whiteMove,
			Rule50:    item.rule50,
			Knights:   1 << 0,
			Bishops:   1 << 1,
			Rooks:     1 << 2,
			Queens:    1 << 3,
		}
		npMaterial := 4*PopCount(position.Knights|position.Bishops) + 6*PopCount(position.Rooks) + 12*PopCount(position.Queens)
		evaluator := NewEvaluationService(&Weights{OutputBias: item.raw})
		result = append(result, oracleAdapterCase{
			ID: item.id, Raw: item.raw, RawBits: math.Float32bits(item.raw),
			WhiteMove: item.whiteMove, Rule50: item.rule50,
			NPMaterial: npMaterial, Adapted: evaluator.EvaluateQuick(&position),
		})
	}
	return result
}

func readOracleFixtures(path string) ([]oracleFixture, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	seen := make(map[string]bool)
	var fixtures []oracleFixture
	scanner := bufio.NewScanner(file)
	for line := 1; scanner.Scan(); line++ {
		text := scanner.Text()
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		parts := strings.SplitN(text, "\t", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("fixture line %d: expected ID<TAB>FEN", line)
		}
		if seen[parts[0]] {
			return nil, fmt.Errorf("fixture line %d: duplicate ID %q", line, parts[0])
		}
		seen[parts[0]] = true
		fixtures = append(fixtures, oracleFixture{ID: parts[0], FEN: parts[1]})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(fixtures) == 0 {
		return nil, fmt.Errorf("no fixtures")
	}
	return fixtures, nil
}

func requiredEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required", name)
	}
	return value
}

func sha256Sum(data []byte) []byte {
	digest := sha256.Sum256(data)
	return digest[:]
}
