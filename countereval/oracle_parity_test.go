//go:build counteroracle

package countereval

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/bits"
	"os"
	"reflect"
	"strings"
	"testing"
)

const pinnedCounter55SHA256 = "3488baed71f4d432d028e05b65e2ff13f8c9afe7f91974d1a3c34a0691d6670c"

type parityOracleRecord struct {
	ID               string   `json:"id"`
	FEN              string   `json:"fen"`
	CounterCanonical string   `json:"counter_canonical"`
	WhiteMove        bool     `json:"white_move"`
	Rule50           int      `json:"rule50"`
	NPMaterial       int      `json:"np_material"`
	Features         []int    `json:"features"`
	AccumulatorBits  []uint32 `json:"accumulator_bits"`
	RawBits          uint32   `json:"raw_white_bits"`
	Adapted          int      `json:"adapted_stm"`
}

type parityAdapterCase struct {
	ID         string  `json:"id"`
	Raw        float32 `json:"raw_white"`
	RawBits    uint32  `json:"raw_white_bits"`
	WhiteMove  bool    `json:"white_move"`
	Rule50     int     `json:"rule50"`
	NPMaterial int     `json:"np_material"`
	Adapted    int     `json:"adapted_stm"`
}

type parityOracle struct {
	Schema        string               `json:"schema"`
	CounterCommit string               `json:"counter_commit"`
	GoVersion     string               `json:"go_version"`
	GOARCH        string               `json:"goarch"`
	ModelSHA256   string               `json:"model_sha256"`
	Records       []parityOracleRecord `json:"records"`
	AdapterCases  []parityAdapterCase  `json:"adapter_cases"`
}

type comparedPosition struct {
	oracle      parityOracleRecord
	board       Board
	features    []int
	accumulator [HiddenSize]float32
	raw         float32
	whiteMove   bool
	rule50      int
	npMaterial  int
}

func TestCounter55PinnedPortableOracleParity(t *testing.T) {
	modelPath := requireTestEnv(t, "COUNTER_MODEL")
	fixturePath := requireTestEnv(t, "COUNTER_FENS")
	oraclePath := requireTestEnv(t, "COUNTER_ORACLE_JSON")

	modelBytes, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	model, metadata, err := LoadCounter55Legacy(bytes.NewReader(modelBytes))
	if err != nil {
		t.Fatal(err)
	}
	if metadata.SHA256 != pinnedCounter55SHA256 {
		t.Fatalf("model SHA256=%s", metadata.SHA256)
	}
	if metadata.Bytes != LegacyFileSize || metadata.Values != payloadValues ||
		math.Abs(metadata.MaxAccumulatorBound-806.5222992897034) > 1e-9 ||
		math.Abs(metadata.MaxProductBound-27614.78091822753) > 1e-9 ||
		math.Abs(metadata.OutputBound-949174.8251401994) > 1e-9 {
		t.Fatalf("pinned model metadata/bounds mismatch: %+v", metadata)
	}
	t.Logf("pinned model loader: sha=%s bytes=%d values=%d bounds=%.16g/%.16g/%.16g", metadata.SHA256, metadata.Bytes, metadata.Values, metadata.MaxAccumulatorBound, metadata.MaxProductBound, metadata.OutputBound)

	fixtureFENs := readFixtureFENs(t, fixturePath)
	encoded, err := os.ReadFile(oraclePath)
	if err != nil {
		t.Fatal(err)
	}
	var oracle parityOracle
	if err := json.Unmarshal(encoded, &oracle); err != nil {
		t.Fatal(err)
	}
	if oracle.Schema != "counter55-portable-oracle-v1" || oracle.CounterCommit != "63c487ca724c620f71c129d62129c6fb9109c872" || oracle.ModelSHA256 != pinnedCounter55SHA256 {
		t.Fatalf("oracle identity mismatch: %+v", oracle)
	}
	if oracle.GOARCH != "amd64" || !strings.HasPrefix(oracle.GoVersion, "go1.25.5") {
		t.Fatalf("oracle toolchain mismatch: go=%s arch=%s", oracle.GoVersion, oracle.GOARCH)
	}
	if err := validateOracleRecordIDs(fixtureFENs, oracle.Records); err != nil {
		t.Fatal(err)
	}

	seen := make(map[string]bool)
	positions := make([]comparedPosition, 0, len(oracle.Records))
	for _, record := range oracle.Records {
		literal, ok := fixtureFENs[record.ID]
		if !ok || seen[record.ID] {
			t.Fatalf("unknown or duplicate oracle record %q", record.ID)
		}
		seen[record.ID] = true
		if record.FEN != literal {
			t.Fatalf("fixture %s FEN changed: %q", record.ID, record.FEN)
		}
		board := boardFromFeatures(t, record.Features)
		features, accumulator, raw, err := model.evaluateFullRefreshTrace(board)
		if err != nil {
			t.Fatalf("fixture %s: %v", record.ID, err)
		}
		whiteMove := record.WhiteMove
		rule50 := record.Rule50
		npMaterial := boardNPMaterial(board)
		if !reflect.DeepEqual(features, record.Features) {
			t.Fatalf("fixture %s features differ: got=%v want=%v", record.ID, features, record.Features)
		}
		if len(record.AccumulatorBits) != HiddenSize {
			t.Fatalf("fixture %s accumulator lanes=%d", record.ID, len(record.AccumulatorBits))
		}
		for lane, value := range accumulator {
			if got := math.Float32bits(value); got != record.AccumulatorBits[lane] {
				t.Fatalf("fixture %s accumulator lane %d bits=%08x want=%08x", record.ID, lane, got, record.AccumulatorBits[lane])
			}
		}
		if got := math.Float32bits(raw); got != record.RawBits {
			t.Fatalf("fixture %s raw bits=%08x want=%08x", record.ID, got, record.RawBits)
		}
		if npMaterial != record.NPMaterial {
			t.Fatalf("fixture %s material=%d want=%d", record.ID, npMaterial, record.NPMaterial)
		}
		if got := counterAdapter(raw, whiteMove, rule50, npMaterial); got != record.Adapted {
			t.Fatalf("fixture %s adapted=%d want=%d", record.ID, got, record.Adapted)
		}
		positions = append(positions, comparedPosition{record, board, features, accumulator, raw, whiteMove, rule50, npMaterial})
	}
	for id := range fixtureFENs {
		if !seen[id] {
			t.Fatalf("missing oracle record %q", id)
		}
	}

	t.Logf("all-layer parity: fixtures=%d feature-lists=%d accumulator-lanes=%d raw-outputs=%d adapted-scores=%d", len(positions), len(positions), len(positions)*HiddenSize, len(positions), len(positions))
	requireMappingMutationsKilled(t, model, positions)
	requireArithmeticMutationsKilled(t, model, positions)
	requireAdapterCases(t, oracle.AdapterCases)
}

func boardFromFeatures(t *testing.T, features []int) Board {
	t.Helper()
	var board Board
	for _, feature := range features {
		if feature < 0 || feature >= InputSize {
			t.Fatalf("feature %d out of range", feature)
		}
		plane := feature / 64
		square := feature % 64
		mask := uint64(1) << square
		if occupied(board)&mask != 0 {
			t.Fatalf("multiple features occupy square %d", square)
		}
		board[plane] |= mask
	}
	return board
}

func boardNPMaterial(board Board) int {
	knightsAndBishops := board[1] | board[2] | board[7] | board[8]
	rooks := board[3] | board[9]
	queens := board[4] | board[10]
	return 4*bits.OnesCount64(knightsAndBishops) + 6*bits.OnesCount64(rooks) + 12*bits.OnesCount64(queens)
}

func counterAdapter(raw float32, whiteMove bool, rule50, npMaterial int) int {
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

func counterAdapterEarlySignEquivalent(raw float32, whiteMove bool, rule50, npMaterial int) int {
	output := int(raw)
	if !whiteMove {
		output = -output
	}
	if output > 15_000 {
		output = 15_000
	} else if output < -15_000 {
		output = -15_000
	}
	output = output * (160 + npMaterial) / 160
	output = output * (200 - rule50) / 200
	return output
}

func requireAdapterCases(t *testing.T, cases []parityAdapterCase) {
	t.Helper()
	if len(cases) < 6 {
		t.Fatalf("adapter oracle cases=%d", len(cases))
	}
	for _, item := range cases {
		if math.Float32bits(item.Raw) != item.RawBits {
			t.Fatalf("adapter case %s raw encoding mismatch", item.ID)
		}
		if got := counterAdapter(item.Raw, item.WhiteMove, item.Rule50, item.NPMaterial); got != item.Adapted {
			t.Fatalf("adapter case %s got=%d want=%d", item.ID, got, item.Adapted)
		}
		if got := counterAdapterEarlySignEquivalent(item.Raw, item.WhiteMove, item.Rule50, item.NPMaterial); got != item.Adapted {
			t.Fatalf("equivalent early-sign control %s got=%d want=%d", item.ID, got, item.Adapted)
		}
	}
	t.Logf("equivalent control early_stm_sign: matched %d/%d adapter cases", len(cases), len(cases))

	mutants := map[string]func(parityAdapterCase) int{
		"rounded_float_to_int": func(c parityAdapterCase) int {
			return counterAdapter(float32(math.Round(float64(c.Raw))), c.WhiteMove, c.Rule50, c.NPMaterial)
		},
		"clip_after_material": func(c parityAdapterCase) int {
			out := int(c.Raw) * (160 + c.NPMaterial) / 160
			if out > 15_000 {
				out = 15_000
			} else if out < -15_000 {
				out = -15_000
			}
			out = out * (200 - c.Rule50) / 200
			if !c.WhiteMove {
				out = -out
			}
			return out
		},
		"combined_division": func(c parityAdapterCase) int {
			out := int(c.Raw)
			if out > 15_000 {
				out = 15_000
			} else if out < -15_000 {
				out = -15_000
			}
			out = out * (160 + c.NPMaterial) * (200 - c.Rule50) / (160 * 200)
			if !c.WhiteMove {
				out = -out
			}
			return out
		},
		"omit_material": func(c parityAdapterCase) int {
			out := int(c.Raw)
			if out > 15_000 {
				out = 15_000
			} else if out < -15_000 {
				out = -15_000
			}
			out = out * (200 - c.Rule50) / 200
			if !c.WhiteMove {
				out = -out
			}
			return out
		},
		"omit_rule50": func(c parityAdapterCase) int {
			out := int(c.Raw)
			if out > 15_000 {
				out = 15_000
			} else if out < -15_000 {
				out = -15_000
			}
			out = out * (160 + c.NPMaterial) / 160
			if !c.WhiteMove {
				out = -out
			}
			return out
		},
		"wrong_clip_limit": func(c parityAdapterCase) int {
			output := int(c.Raw)
			if output > 14_999 {
				output = 14_999
			} else if output < -14_999 {
				output = -14_999
			}
			output = output * (160 + c.NPMaterial) / 160
			output = output * (200 - c.Rule50) / 200
			if !c.WhiteMove {
				output = -output
			}
			return output
		},
		"wrong_material_constant": func(c parityAdapterCase) int {
			output := int(c.Raw)
			if output > 15_000 {
				output = 15_000
			} else if output < -15_000 {
				output = -15_000
			}
			output = output * (159 + c.NPMaterial) / 159
			output = output * (200 - c.Rule50) / 200
			if !c.WhiteMove {
				output = -output
			}
			return output
		},
		"wrong_rule50_constant": func(c parityAdapterCase) int {
			output := int(c.Raw)
			if output > 15_000 {
				output = 15_000
			} else if output < -15_000 {
				output = -15_000
			}
			output = output * (160 + c.NPMaterial) / 160
			output = output * (199 - c.Rule50) / 199
			if !c.WhiteMove {
				output = -output
			}
			return output
		},
		"wrong_side_sign": func(c parityAdapterCase) int {
			return counterAdapter(c.Raw, true, c.Rule50, c.NPMaterial)
		},
	}
	for name, mutant := range mutants {
		witness := ""
		for _, item := range cases {
			if mutant(item) != item.Adapted {
				witness = item.ID
				break
			}
		}
		if witness == "" {
			t.Fatalf("adapter mutant %s survived all cases", name)
		}
		t.Logf("killed adapter mutant %s witness=%s", name, witness)
	}
}

func requireMappingMutationsKilled(t *testing.T, model *Model, positions []comparedPosition) {
	t.Helper()
	mutants := map[string]func(Board) Board{
		"swap_colors": func(board Board) Board {
			var result Board
			for plane := 0; plane < 6; plane++ {
				result[plane], result[plane+6] = board[plane+6], board[plane]
			}
			return result
		},
		"vertical_flip": func(board Board) Board {
			var result Board
			for plane, bb := range board {
				for bb != 0 {
					sq := bits.TrailingZeros64(bb)
					result[plane] |= uint64(1) << (sq ^ 56)
					bb &= bb - 1
				}
			}
			return result
		},
		"swap_pawn_knight_planes": func(board Board) Board {
			result := board
			result[0], result[1] = board[1], board[0]
			result[6], result[7] = board[7], board[6]
			return result
		},
	}
	for name, mutate := range mutants {
		witness := ""
		for _, item := range positions {
			features, accumulator, raw, err := model.evaluateFullRefreshTrace(mutate(item.board))
			if err != nil {
				t.Fatalf("mapping mutant %s: %v", name, err)
			}
			if !reflect.DeepEqual(features, item.features) || !equalAccumulatorBits(accumulator, item.accumulator) || math.Float32bits(raw) != math.Float32bits(item.raw) {
				witness = item.oracle.ID
				break
			}
		}
		if witness == "" {
			t.Fatalf("mapping mutant %s survived all fixtures", name)
		}
		t.Logf("killed mapping mutant %s witness=%s", name, witness)
	}

	planeMajorWitness := ""
	for _, item := range positions {
		accumulator, raw := evaluatePlaneMajor(model, item.board)
		if !equalAccumulatorBits(accumulator, item.accumulator) || math.Float32bits(raw) != math.Float32bits(item.raw) {
			planeMajorWitness = item.oracle.ID
			break
		}
	}
	if planeMajorWitness == "" {
		t.Fatal("plane-major feature iteration survived all fixtures")
	}
	t.Logf("killed mapping mutant plane_major_feature_order witness=%s", planeMajorWitness)
	stmSignWitness := ""
	for _, item := range positions {
		mutated := item.raw
		if !item.whiteMove {
			mutated = -mutated
		}
		if math.Float32bits(mutated) != math.Float32bits(item.raw) {
			stmSignWitness = item.oracle.ID
			break
		}
	}
	if stmSignWitness == "" {
		t.Fatal("STM-signed raw output survived all fixtures")
	}
	t.Logf("killed mapping mutant stm_signed_raw witness=%s", stmSignWitness)
}

func requireArithmeticMutationsKilled(t *testing.T, model *Model, positions []comparedPosition) {
	t.Helper()
	mutants := map[string]func(*Model, [HiddenSize]float32) float32{
		"omit_output_bias": func(model *Model, accumulator [HiddenSize]float32) float32 {
			var output float32
			for i, value := range accumulator {
				if value > 0 {
					output += value * model.outputWeights[i]
				}
			}
			return output
		},
		"disable_relu": func(model *Model, accumulator [HiddenSize]float32) float32 {
			var output float32
			for i, value := range accumulator {
				output += value * model.outputWeights[i]
			}
			return output + model.outputBias
		},
		"reverse_output_order": func(model *Model, accumulator [HiddenSize]float32) float32 {
			var output float32
			for i := HiddenSize - 1; i >= 0; i-- {
				if accumulator[i] > 0 {
					output += accumulator[i] * model.outputWeights[i]
				}
			}
			return output + model.outputBias
		},
	}
	for name, mutant := range mutants {
		witness := ""
		for _, item := range positions {
			if math.Float32bits(mutant(model, item.accumulator)) != math.Float32bits(item.raw) {
				witness = item.oracle.ID
				break
			}
		}
		if witness == "" {
			t.Fatalf("arithmetic mutant %s survived all fixtures", name)
		}
		t.Logf("killed arithmetic mutant %s witness=%s", name, witness)
	}
}

func evaluatePlaneMajor(model *Model, board Board) ([HiddenSize]float32, float32) {
	var accumulator [HiddenSize]float32
	copy(accumulator[:], model.hiddenBiases[:])
	for plane, bb := range board {
		for bb != 0 {
			square := bits.TrailingZeros64(bb)
			base := (plane*64 + square) * HiddenSize
			for hidden := 0; hidden < HiddenSize; hidden++ {
				accumulator[hidden] += model.hiddenWeights[base+hidden]
			}
			bb &= bb - 1
		}
	}
	var output float32
	for i, value := range accumulator {
		if value > 0 {
			output += value * model.outputWeights[i]
		}
	}
	return accumulator, output + model.outputBias
}

func equalAccumulatorBits(a, b [HiddenSize]float32) bool {
	for i := range a {
		if math.Float32bits(a[i]) != math.Float32bits(b[i]) {
			return false
		}
	}
	return true
}

func readFixtureFENs(t *testing.T, path string) map[string]string {
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

func requireTestEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required", name)
	}
	return value
}

func validateOracleRecordIDs(expected map[string]string, records []parityOracleRecord) error {
	if len(records) != len(expected) {
		return fmt.Errorf("oracle records=%d fixtures=%d", len(records), len(expected))
	}
	seen := make(map[string]bool)
	for _, record := range records {
		literal, ok := expected[record.ID]
		if !ok || seen[record.ID] || record.FEN != literal {
			return fmt.Errorf("unknown, duplicate, or changed oracle record %q", record.ID)
		}
		seen[record.ID] = true
	}
	return nil
}

func TestCounter55OracleRecordSetGuard(t *testing.T) {
	expected := map[string]string{"a": "fen-a", "b": "fen-b"}
	cases := []struct {
		name    string
		records []parityOracleRecord
	}{
		{"success_returning_empty_stub", nil},
		{"missing", []parityOracleRecord{{ID: "a", FEN: "fen-a"}}},
		{"duplicate", []parityOracleRecord{{ID: "a", FEN: "fen-a"}, {ID: "a", FEN: "fen-a"}}},
		{"unknown", []parityOracleRecord{{ID: "a", FEN: "fen-a"}, {ID: "c", FEN: "fen-c"}}},
		{"changed_fen", []parityOracleRecord{{ID: "a", FEN: "changed"}, {ID: "b", FEN: "fen-b"}}},
	}
	for _, item := range cases {
		if err := validateOracleRecordIDs(expected, item.records); err == nil {
			t.Fatalf("guard mutation %s unexpectedly passed", item.name)
		}
	}
	if err := validateOracleRecordIDs(expected, []parityOracleRecord{{ID: "a", FEN: "fen-a"}, {ID: "b", FEN: "fen-b"}}); err != nil {
		t.Fatalf("valid record set: %v", err)
	}
	t.Logf("oracle record-set guard rejected %d malformed/stub cases", len(cases))
}
