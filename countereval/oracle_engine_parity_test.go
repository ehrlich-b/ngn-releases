//go:build counteroracle

package countereval_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"math"
	"math/bits"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/ehrlich-b/ngn/countereval"
	"github.com/ehrlich-b/ngn/engine"
)

const counter55ModelSHA256 = "3488baed71f4d432d028e05b65e2ff13f8c9afe7f91974d1a3c34a0691d6670c"

type engineOracleRecord struct {
	ID               string `json:"id"`
	FEN              string `json:"fen"`
	CounterCanonical string `json:"counter_canonical"`
	WhiteMove        bool   `json:"white_move"`
	Rule50           int    `json:"rule50"`
	NPMaterial       int    `json:"np_material"`
	Features         []int  `json:"features"`
	RawBits          uint32 `json:"raw_white_bits"`
	Adapted          int    `json:"adapted_stm"`
}

type engineOracle struct {
	Schema        string               `json:"schema"`
	CounterCommit string               `json:"counter_commit"`
	ModelSHA256   string               `json:"model_sha256"`
	Records       []engineOracleRecord `json:"records"`
}

func TestCounter55EngineBoundaryOracleParity(t *testing.T) {
	modelPath := requiredExternalEnv(t, "COUNTER_MODEL")
	fixturePath := requiredExternalEnv(t, "COUNTER_FENS")
	oraclePath := requiredExternalEnv(t, "COUNTER_ORACLE_JSON")

	modelBytes, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	model, metadata, err := countereval.LoadCounter55Legacy(bytes.NewReader(modelBytes))
	if err != nil {
		t.Fatal(err)
	}
	if metadata.SHA256 != counter55ModelSHA256 {
		t.Fatalf("model SHA256=%s", metadata.SHA256)
	}

	fixtures := readExternalFENs(t, fixturePath)
	encoded, err := os.ReadFile(oraclePath)
	if err != nil {
		t.Fatal(err)
	}
	var oracle engineOracle
	if err := json.Unmarshal(encoded, &oracle); err != nil {
		t.Fatal(err)
	}
	if oracle.Schema != "counter55-portable-oracle-v1" ||
		oracle.CounterCommit != "63c487ca724c620f71c129d62129c6fb9109c872" ||
		oracle.ModelSHA256 != counter55ModelSHA256 {
		t.Fatalf("oracle identity mismatch: %+v", oracle)
	}
	if len(oracle.Records) != len(fixtures) {
		t.Fatalf("oracle records=%d fixtures=%d", len(oracle.Records), len(fixtures))
	}

	seen := make(map[string]bool)
	for _, record := range oracle.Records {
		literal, ok := fixtures[record.ID]
		if !ok || seen[record.ID] || record.FEN != literal {
			t.Fatalf("unknown, duplicate, or changed record %q", record.ID)
		}
		seen[record.ID] = true
		position, err := engine.ParseFEN(literal)
		if err != nil {
			t.Fatalf("NGN fixture %s: %v", record.ID, err)
		}
		literalFields := strings.Fields(literal)
		counterFields := strings.Fields(record.CounterCanonical)
		ngnFields := strings.Fields(engine.GenerateFEN(position))
		if len(literalFields) != 6 || len(counterFields) != 6 || len(ngnFields) != 6 ||
			!reflect.DeepEqual(counterFields[:5], literalFields[:5]) ||
			!reflect.DeepEqual(ngnFields[:5], literalFields[:5]) {
			t.Fatalf("fixture %s parser identity differs: literal=%v counter=%v ngn=%v", record.ID, literalFields, counterFields, ngnFields)
		}
		board := externalBoardFromNGN(t, position)
		if features := externalFeatures(board); !reflect.DeepEqual(features, record.Features) {
			t.Fatalf("fixture %s ordered features=%v want=%v", record.ID, features, record.Features)
		}
		raw, err := model.EvaluateFullRefresh(board)
		if err != nil {
			t.Fatalf("fixture %s: %v", record.ID, err)
		}
		if got := math.Float32bits(raw); got != record.RawBits {
			t.Fatalf("fixture %s raw bits=%08x want=%08x", record.ID, got, record.RawBits)
		}
		whiteMove := position.Turn() == engine.White
		rule50 := int(position.HalfMoveClock)
		npMaterial := externalNPMaterial(board)
		if whiteMove != record.WhiteMove || rule50 != record.Rule50 || npMaterial != record.NPMaterial {
			t.Fatalf("fixture %s position facts differ", record.ID)
		}
		if got := externalCounterAdapter(raw, whiteMove, rule50, npMaterial); got != record.Adapted {
			t.Fatalf("fixture %s adapted=%d want=%d", record.ID, got, record.Adapted)
		}
	}
	for id := range fixtures {
		if !seen[id] {
			t.Fatalf("missing oracle record %q", id)
		}
	}
}

func externalBoardFromNGN(t *testing.T, position *engine.Position) countereval.Board {
	t.Helper()
	var board countereval.Board
	for square := 0; square < 64; square++ {
		piece := position.Board.PieceAt(engine.Square(square))
		if piece == engine.NoPiece {
			continue
		}
		plane := int(piece) - int(engine.WhitePawn)
		if plane < 0 || plane >= countereval.FeaturePlaneCount {
			t.Fatalf("unexpected NGN piece %d at square %d", piece, square)
		}
		board[plane] |= uint64(1) << square
	}
	return board
}

func externalNPMaterial(board countereval.Board) int {
	knightsAndBishops := board[1] | board[2] | board[7] | board[8]
	rooks := board[3] | board[9]
	queens := board[4] | board[10]
	return 4*bits.OnesCount64(knightsAndBishops) + 6*bits.OnesCount64(rooks) + 12*bits.OnesCount64(queens)
}

func externalCounterAdapter(raw float32, whiteMove bool, rule50, npMaterial int) int {
	output := int(raw)
	if output > 15_000 {
		output = 15_000
	} else if output < -15_000 {
		output = -15_000
	}
	output = output * (160 + npMaterial) / 160
	output = output * (200 - rule50) / 200
	if !whiteMove {
		output = -output
	}
	return output
}

func readExternalFENs(t *testing.T, path string) map[string]string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	result := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 || result[parts[0]] != "" {
			t.Fatalf("bad/duplicate fixture line %q", line)
		}
		result[parts[0]] = parts[1]
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func requiredExternalEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required", name)
	}
	return value
}
