package k4label

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ehrlich-b/ngn/engine"
)

func testSHA(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func testPositions(t *testing.T) []InputPosition {
	t.Helper()
	archive := testSHA("source archive")
	positions := []InputPosition{
		{
			Type: "position", ID: testSHA("position 0"),
			FEN: "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 37 1", SourceMove: "e2e4",
			EncodedChain: 10, ChainEntry: 2, SourcePosition: 100,
			SourceArchiveSHA: archive,
		},
		{
			Type: "position", ID: testSHA("position 1"),
			FEN: "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq - 12 1", SourceMove: "e7e5",
			EncodedChain: 11, ChainEntry: 3, SourcePosition: 101,
			SourceArchiveSHA: archive,
		},
		{
			Type: "position", ID: testSHA("position 2"),
			FEN: "4k3/8/8/8/3p4/4P3/8/4K3 w - - 7 1", SourceMove: "e1e2",
			EncodedChain: 12, ChainEntry: 4, SourcePosition: 102,
			SourceArchiveSHA: archive,
		},
	}
	for index := range positions {
		position, err := engine.ParseFEN(positions[index].FEN)
		if err != nil {
			t.Fatal(err)
		}
		positions[index].K4InputSHA256, err = K4InputSHA256(position)
		if err != nil {
			t.Fatal(err)
		}
	}
	return positions
}

func writeInputShard(t *testing.T, positions []InputPosition) string {
	t.Helper()
	header := InputHeader{
		Type: "header", Schema: InputSchema, ShardID: "train-000000", Split: "train",
		SourceManifestSHA256: testSHA("source manifest"), RecordCount: len(positions),
	}
	var lines [][]byte
	encoded, err := json.Marshal(header)
	if err != nil {
		t.Fatal(err)
	}
	lines = append(lines, encoded)
	for _, position := range positions {
		encoded, err := json.Marshal(position)
		if err != nil {
			t.Fatal(err)
		}
		lines = append(lines, encoded)
	}
	path := filepath.Join(t.TempDir(), "input.jsonl")
	if err := os.WriteFile(path, append([]byte(strings.Join(byteStrings(lines), "\n")), '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func byteStrings(lines [][]byte) []string {
	result := make([]string, len(lines))
	for index := range lines {
		result[index] = string(lines[index])
	}
	return result
}

func TestReadInputShardValidatesCompleteSemanticInput(t *testing.T) {
	positions := testPositions(t)
	path := writeInputShard(t, positions)
	shard, err := ReadInputShard(path)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	if shard.SHA256 != hex.EncodeToString(digest[:]) || shard.Header.RecordCount != len(positions) ||
		len(shard.Positions) != len(positions) {
		t.Fatalf("shard identity/count = %+v / %d", shard.Header, len(shard.Positions))
	}

	duplicate := append([]InputPosition(nil), positions...)
	duplicate[1].ID = duplicate[0].ID
	if _, err := ReadInputShard(writeInputShard(t, duplicate)); err == nil || !strings.Contains(err.Error(), "duplicate position id") {
		t.Fatalf("duplicate input error = %v", err)
	}

	wrongK4 := append([]InputPosition(nil), positions...)
	wrongK4[0].K4InputSHA256 = testSHA("plain Chess768 key")
	if _, err := ReadInputShard(writeInputShard(t, wrongK4)); err == nil || !strings.Contains(err.Error(), "K4 input identity mismatch") {
		t.Fatalf("wrong K4 input error = %v", err)
	}

	phantomEP := append([]InputPosition(nil), positions[:1]...)
	phantomEP[0].FEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq e3 37 1"
	if _, err := ReadInputShard(writeInputShard(t, phantomEP)); err == nil || !strings.Contains(err.Error(), "canonical/preservable") {
		t.Fatalf("phantom EP error = %v", err)
	}

	captureSource := append([]InputPosition(nil), positions[2:]...)
	captureSource[0].SourceMove = "e3d4"
	if _, err := ReadInputShard(writeInputShard(t, captureSource)); err == nil || !strings.Contains(err.Error(), "normal quiet") {
		t.Fatalf("capture source error = %v", err)
	}
}

func TestScoreTargetFrozenMaterialInversion(t *testing.T) {
	start, err := engine.ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	score, target, normalizer, material, err := ScoreTarget(start, 100)
	if err != nil {
		t.Fatal(err)
	}
	if material != 78 || math.Abs(normalizer-382.3728893754336) > 1e-12 || score != 184 ||
		math.Abs(target-0.6130141761393355) > 1e-15 {
		t.Fatalf("start score = %d target=%.17g normalizer=%.17g material=%d", score, target, normalizer, material)
	}

	bare, err := engine.ParseFEN("4k3/8/8/8/8/8/8/4K3 b - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	positive, positiveTarget, normalizer, material, err := ScoreTarget(bare, 100)
	if err != nil {
		t.Fatal(err)
	}
	negative, negativeTarget, _, _, err := ScoreTarget(bare, -100)
	if err != nil {
		t.Fatal(err)
	}
	zero, zeroTarget, _, _, err := ScoreTarget(bare, 0)
	if err != nil {
		t.Fatal(err)
	}
	if material != 0 || math.Abs(normalizer-388.22278368297987) > 1e-12 || positive != 187 || negative != -187 ||
		zero != 0 || zeroTarget != 0.5 || math.Abs((positiveTarget+negativeTarget)-1) > 1e-15 {
		t.Fatalf("bare scores = %d/%d/%d targets %.17g/%.17g/%.17g normalizer %.17g material %d",
			positive, negative, zero, positiveTarget, negativeTarget, zeroTarget, normalizer, material)
	}
}
