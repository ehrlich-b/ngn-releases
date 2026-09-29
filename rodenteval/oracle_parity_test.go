//go:build rodentoracle

package rodenteval

import (
	"encoding/json"
	"fmt"
	"math/bits"
	"os"
	"strconv"
	"strings"
	"testing"
)

const (
	minimumOracleRecords = 50
	releaseBinarySHA256  = "9c68d7b39dc933eff5fd2da88bd4be1b0d009f6d2add7485c121cfc0d1308531"
	taggedSourceCommit   = "5689d0babebe95d87592eaaaee73ea555ef9345c"
	anandPersonalitySHA  = "b0e336bc8892043063e6b1fcc42193bbbc6cacf347e9d561d089ddcb602bf3aa"
)

type rawOracleManifest struct {
	Schema                  string            `json:"schema"`
	ReleaseBinarySHA256     string            `json:"release_binary_sha256"`
	NetworkSHA256           string            `json:"network_sha256"`
	TaggedSourceCommit      string            `json:"tagged_source_commit"`
	PersonalitySHA256       string            `json:"personality_sha256"`
	RawOracleCommand        json.RawMessage   `json:"raw_oracle_command"`
	ReleaseStaticDerivation json.RawMessage   `json:"release_static_derivation"`
	Records                 []rawOracleRecord `json:"records"`
}

type rawOracleRecord struct {
	Name          string `json:"name"`
	FEN           string `json:"fen"`
	Raw           int    `json:"raw"`
	ReleaseStatic int    `json:"release_static"`
}

func TestV11AnandExactReleaseCorpusParity(t *testing.T) {
	modelPath := os.Getenv("RODENT_V11_ANAND_MODEL")
	oraclePath := os.Getenv("RODENT_V11_ANAND_ORACLE_JSON")
	if modelPath == "" || oraclePath == "" {
		t.Skip("set RODENT_V11_ANAND_MODEL and RODENT_V11_ANAND_ORACLE_JSON")
	}
	model, err := LoadV11Anand(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(oraclePath)
	if err != nil {
		t.Fatal(err)
	}
	var oracle rawOracleManifest
	if err := json.Unmarshal(data, &oracle); err != nil {
		t.Fatal(err)
	}
	if oracle.Schema != "rodent-v1.1-anand-raw-oracle-v1" ||
		oracle.ReleaseBinarySHA256 != releaseBinarySHA256 ||
		oracle.NetworkSHA256 != V11AnandSHA256 ||
		oracle.TaggedSourceCommit != taggedSourceCommit ||
		oracle.PersonalitySHA256 != anandPersonalitySHA {
		t.Fatalf("oracle provenance mismatch: %+v", oracle)
	}
	if len(oracle.RawOracleCommand) == 0 || len(oracle.ReleaseStaticDerivation) == 0 || len(oracle.Records) < minimumOracleRecords {
		t.Fatal("oracle lacks command, derivation, or records")
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
		int64(300)*count(WhiteBishop) + int64(300)*count(WhiteRook) +
		int64(500)*(count(WhiteRook)+count(BlackRook)) +
		int64(900)*(count(WhiteQueen)+count(BlackQueen))
	return int(int64(raw) * (25000 + material) / 32768)
}

func parseOracleFEN(fen string) (Position, error) {
	fields := strings.Fields(fen)
	if len(fields) < 2 {
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
	if len(fields) >= 5 {
		if _, err := strconv.Atoi(fields[4]); err != nil {
			return Position{}, fmt.Errorf("invalid FEN halfmove clock %q", fields[4])
		}
	}
	if err := validatePosition(position); err != nil {
		return Position{}, err
	}
	return position, nil
}
