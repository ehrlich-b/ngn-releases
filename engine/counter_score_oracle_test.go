//go:build counteroracle

package engine

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"testing"

	"github.com/ehrlich-b/ngn/countereval"
)

type counterScoreOracleRecord struct {
	ID         string `json:"id"`
	FEN        string `json:"fen"`
	RawBits    uint32 `json:"raw_white_bits"`
	AdaptedSTM int    `json:"adapted_stm"`
}

type counterScoreOracleAdapterCase struct {
	ID         string `json:"id"`
	RawBits    uint32 `json:"raw_white_bits"`
	WhiteMove  bool   `json:"white_move"`
	Rule50     uint8  `json:"rule50"`
	NPMaterial int64  `json:"np_material"`
	AdaptedSTM int    `json:"adapted_stm"`
}

type counterScoreOracle struct {
	Schema        string                          `json:"schema"`
	CounterCommit string                          `json:"counter_commit"`
	ModelSHA256   string                          `json:"model_sha256"`
	Records       []counterScoreOracleRecord      `json:"records"`
	AdapterCases  []counterScoreOracleAdapterCase `json:"adapter_cases"`
}

func TestCounterSearchScoreMatchesPinnedUpstreamAdapterVectors(t *testing.T) {
	modelPath := requireCounterOracleEnv(t, "COUNTER_MODEL")
	oraclePath := requireCounterOracleEnv(t, "COUNTER_ORACLE_JSON")
	modelBytes, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	_, metadata, err := countereval.LoadCounter55Legacy(bytes.NewReader(modelBytes))
	if err != nil {
		t.Fatal(err)
	}
	if metadata.SHA256 != acceptedCounter55SHA256 {
		t.Fatalf("Counter model SHA256=%s", metadata.SHA256)
	}
	encoded, err := os.ReadFile(oraclePath)
	if err != nil {
		t.Fatal(err)
	}
	var oracle counterScoreOracle
	if err := json.Unmarshal(encoded, &oracle); err != nil {
		t.Fatal(err)
	}
	if oracle.Schema != "counter55-portable-oracle-v1" ||
		oracle.CounterCommit != "63c487ca724c620f71c129d62129c6fb9109c872" ||
		oracle.ModelSHA256 != acceptedCounter55SHA256 {
		t.Fatalf("oracle identity=%+v", oracle)
	}
	requireExactCounterOracleIDs(t, "records", oracleRecordIDs(oracle.Records), []string{
		"start_white", "start_black", "kiwipete_white", "kiwipete_black",
		"ep_white", "ep_black", "promoted_white", "promoted_black",
		"bounds_white", "bounds_black", "asymmetric_white", "asymmetric_black",
	})
	requireExactCounterOracleIDs(t, "adapter_cases", oracleAdapterCaseIDs(oracle.AdapterCases), []string{
		"positive_fraction", "negative_fraction", "positive_clip_before_scale",
		"negative_clip_before_scale", "sequential_division", "black_sign",
	})
	for _, record := range oracle.Records {
		pos, err := ParseFEN(record.FEN)
		if err != nil {
			t.Fatalf("%s: %v", record.ID, err)
		}
		raw := math.Float32frombits(record.RawBits)
		if got := counterSearchScore(raw, pos, true); got != record.AdaptedSTM {
			t.Fatalf("%s score=%d upstream=%d raw_bits=%08x", record.ID, got, record.AdaptedSTM, record.RawBits)
		}
	}
	for _, test := range oracle.AdapterCases {
		raw := math.Float32frombits(test.RawBits)
		got := counterSearchScoreInputs(raw, test.WhiteMove, test.Rule50, test.NPMaterial, true)
		if got != test.AdaptedSTM {
			t.Fatalf("adapter %s score=%d upstream=%d raw_bits=%08x", test.ID, got, test.AdaptedSTM, test.RawBits)
		}
	}

}

func oracleRecordIDs(records []counterScoreOracleRecord) []string {
	ids := make([]string, len(records))
	for index := range records {
		ids[index] = records[index].ID
	}
	return ids
}

func oracleAdapterCaseIDs(records []counterScoreOracleAdapterCase) []string {
	ids := make([]string, len(records))
	for index := range records {
		ids[index] = records[index].ID
	}
	return ids
}

func requireExactCounterOracleIDs(t *testing.T, label string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s count=%d want=%d ids=%v", label, len(got), len(want), got)
	}
	seen := make(map[string]bool, len(got))
	for _, id := range got {
		if id == "" || seen[id] {
			t.Fatalf("%s empty/duplicate id=%q", label, id)
		}
		seen[id] = true
	}
	for _, id := range want {
		if !seen[id] {
			t.Fatalf("%s missing id=%q got=%v", label, id, got)
		}
	}
}
