//go:build rodentoracle

package rodentv12eval

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/bits"
	"os"
	"strconv"
	"strings"
	"testing"
)

const (
	v12OracleSHA256        = "a16487b7a7faf725d4c2369c91467274c10ecd04b8a4a0b8faec5c48fc08d440"
	v12ReleaseBinarySHA256 = "9cfb8195207ee5695c1973a89664ab73b34b5bcbc10ad3bc0f0afe28b9713cbc"
	v12TaggedSourceCommit  = "b53ffaf670590932957cb63b7b6d871f6f33b7d8"
)

type oracleDeterminism struct {
	Passes         int  `json:"passes"`
	Matched        bool `json:"matched"`
	RecordsPerPass int  `json:"records_per_pass"`
}

type oracleConfiguration struct {
	Scale               int `json:"scale"`
	HorizontalMirroring int `json:"horizontal_mirroring"`
	HCEWeight           int `json:"hce_weight"`
	NNUEWeight          int `json:"nnue_weight"`
	InputBuckets        int `json:"input_buckets"`
	HiddenSize          int `json:"hidden_size"`
	OutputBuckets       int `json:"output_buckets"`
}

type rawOracleManifest struct {
	Schema                  string              `json:"schema"`
	Classification          string              `json:"classification"`
	ReleaseBinarySHA256     string              `json:"release_binary_sha256"`
	NetworkSHA256           string              `json:"network_sha256"`
	TaggedSourceCommit      string              `json:"tagged_source_commit"`
	RawOracleCommand        string              `json:"raw_oracle_command"`
	RawOracleSemantics      string              `json:"raw_oracle_semantics"`
	Determinism             oracleDeterminism   `json:"determinism"`
	ReleaseStaticDerivation json.RawMessage     `json:"release_static_derivation"`
	ReleaseConfiguration    oracleConfiguration `json:"release_configuration"`
	Records                 []rawOracleRecord   `json:"records"`
}

type rawOracleRecord struct {
	Name          string `json:"name"`
	FEN           string `json:"fen"`
	Raw           int    `json:"raw"`
	ReleaseStatic int    `json:"release_static"`
}

func TestV12DefaultExactReleaseCorpusParity(t *testing.T) {
	modelPath := os.Getenv("RODENT_V12_DEFAULT_MODEL")
	oraclePath := os.Getenv("RODENT_V12_DEFAULT_ORACLE_JSON")
	if modelPath == "" || oraclePath == "" {
		t.Skip("set RODENT_V12_DEFAULT_MODEL and RODENT_V12_DEFAULT_ORACLE_JSON")
	}
	model, err := LoadV12Default(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(oraclePath)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != v12OracleSHA256 {
		t.Fatalf("oracle SHA-256 = %s, want %s", got, v12OracleSHA256)
	}
	var oracle rawOracleManifest
	if err := json.Unmarshal(data, &oracle); err != nil {
		t.Fatal(err)
	}
	wantConfiguration := oracleConfiguration{
		Scale: outputScale, HorizontalMirroring: 1, HCEWeight: 0, NNUEWeight: 100,
		InputBuckets: InputBuckets, HiddenSize: HiddenSize, OutputBuckets: OutputBuckets,
	}
	if oracle.Schema != "rodent-v1.2-default-raw-oracle-v1" ||
		oracle.Classification != "exact-release full-refresh raw runtime oracle; final static independently derived" ||
		oracle.ReleaseBinarySHA256 != v12ReleaseBinarySHA256 ||
		oracle.NetworkSHA256 != V12DefaultSHA256 ||
		oracle.TaggedSourceCommit != v12TaggedSourceCommit ||
		oracle.RawOracleCommand != "position fen <FEN>; nnue; isready; parse ^(-?[0-9]+)readyok$" ||
		oracle.RawOracleSemantics != "side-to-move full-refresh getEval integer before material factor" ||
		oracle.Determinism != (oracleDeterminism{Passes: 2, Matched: true, RecordsPerPass: 104}) ||
		oracle.ReleaseConfiguration != wantConfiguration || len(oracle.ReleaseStaticDerivation) == 0 {
		t.Fatalf("oracle provenance mismatch: %+v", oracle)
	}
	if len(oracle.Records) != 104 {
		t.Fatalf("oracle record count = %d, want 104", len(oracle.Records))
	}
	names := make(map[string]struct{}, len(oracle.Records))
	fens := make(map[string]struct{}, len(oracle.Records))
	for _, record := range oracle.Records {
		if record.Name == "" || record.FEN == "" {
			t.Fatal("oracle contains empty name or FEN")
		}
		if _, exists := names[record.Name]; exists {
			t.Fatalf("oracle contains duplicate name %q", record.Name)
		}
		if _, exists := fens[record.FEN]; exists {
			t.Fatalf("oracle contains duplicate FEN %q", record.FEN)
		}
		names[record.Name] = struct{}{}
		fens[record.FEN] = struct{}{}
		t.Run(record.Name, func(t *testing.T) {
			position, err := parseOracleFEN(record.FEN)
			if err != nil {
				t.Fatal(err)
			}
			if want := independentlyScaleReleaseStatic(record.Raw, position.Board); record.ReleaseStatic != want {
				t.Fatalf("stored release_static = %d, independently derived = %d", record.ReleaseStatic, want)
			}
			if got, err := model.EvaluateRaw(position); err != nil || got != record.Raw {
				t.Fatalf("EvaluateRaw = %d, %v; exact release = %d", got, err, record.Raw)
			}
			if got, err := model.EvaluateReleaseStatic(position); err != nil || got != record.ReleaseStatic {
				t.Fatalf("EvaluateReleaseStatic = %d, %v; release adapter = %d", got, err, record.ReleaseStatic)
			}
		})
	}
}

func independentlyScaleReleaseStatic(raw int, board Board) int {
	count := func(plane int) int64 { return int64(bits.OnesCount64(board[plane])) }
	material := int64(100)*(count(WhitePawn)+count(BlackPawn)) +
		int64(300)*(count(WhiteKnight)+count(BlackKnight)) +
		int64(300)*(count(WhiteBishop)+count(BlackBishop)) +
		int64(500)*(count(WhiteRook)+count(BlackRook)) +
		int64(900)*(count(WhiteQueen)+count(BlackQueen))
	return int(int64(raw) * (25000 + material) / 32768)
}

func parseOracleFEN(fen string) (Position, error) {
	fields := strings.Fields(fen)
	if len(fields) != 6 {
		return Position{}, fmt.Errorf("invalid FEN %q", fen)
	}
	var position Position
	ranks := strings.Split(fields[0], "/")
	if len(ranks) != 8 {
		return Position{}, fmt.Errorf("invalid FEN board %q", fields[0])
	}
	planes := map[byte]int{
		'P': WhitePawn, 'N': WhiteKnight, 'B': WhiteBishop, 'R': WhiteRook, 'Q': WhiteQueen, 'K': WhiteKing,
		'p': BlackPawn, 'n': BlackKnight, 'b': BlackBishop, 'r': BlackRook, 'q': BlackQueen, 'k': BlackKing,
	}
	for rankIndex, rankText := range ranks {
		file := 0
		for i := 0; i < len(rankText); i++ {
			ch := rankText[i]
			if ch >= '1' && ch <= '8' {
				file += int(ch - '0')
				continue
			}
			plane, ok := planes[ch]
			if !ok || file >= 8 {
				return Position{}, fmt.Errorf("invalid FEN rank %q", rankText)
			}
			square := (7-rankIndex)*8 + file
			position.Board[plane] |= uint64(1) << square
			file++
		}
		if file != 8 {
			return Position{}, fmt.Errorf("invalid FEN rank width %q", rankText)
		}
	}
	switch fields[1] {
	case "w":
		position.SideToMove = White
	case "b":
		position.SideToMove = Black
	default:
		return Position{}, fmt.Errorf("invalid FEN side %q", fields[1])
	}
	if _, err := strconv.Atoi(fields[4]); err != nil {
		return Position{}, fmt.Errorf("invalid FEN halfmove clock %q", fields[4])
	}
	if err := validatePosition(position); err != nil {
		return Position{}, err
	}
	return position, nil
}
