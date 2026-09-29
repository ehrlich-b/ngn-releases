package k4pack

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	whiteFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 7 1"
	blackFEN = "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq - 4 1"
)

func fixtureSHA(text string) string {
	digest := sha256.Sum256([]byte(text))
	return hex.EncodeToString(digest[:])
}

func fixtureHeader(records int) outputHeader {
	return outputHeader{
		Type: "header", Schema: LabelSchema, ContractVersion: ContractVersion,
		InputSHA256: fixtureSHA("sampler input"),
		Input:       inputHeader{Type: "header", Schema: InputSchema, ShardID: "fixture-000", Split: "train", SourceManifestSHA256: fixtureSHA("source manifest"), RecordCount: records},
		Teacher: teacherProvenance{
			Name: "Stockfish 18", ExecutablePath: "/fixture/stockfish", ExecutableSHA256: fixtureSHA("teacher executable"),
			SourceCommit: TeacherSourceCommit, BigNetworkSHA256: TeacherBigNetworkSHA, SmallNetworkSHA256: TeacherSmallNetSHA,
			Command: []string{"/fixture/stockfish"}, HandshakeOptionDigest: fixtureSHA("options"),
		},
		Search:        searchConfig{Nodes: 5_000, Threads: 1, HashMiB: 16, MultiPV: 1, MinimumDepth: 4, TimeoutMillis: 2_000, SyzygyPath: "<empty>"},
		ScoreContract: ScoreContract,
	}
}

func acceptedFixture(t *testing.T, id, fen, sourceMove, bestMove string, cp int) labelRecord {
	t.Helper()
	reset, _, err := resetPosition(fen)
	if err != nil {
		t.Fatal(err)
	}
	board, material, err := parseBoard(fen, 0)
	if err != nil {
		t.Fatal(err)
	}
	score, target, normalizer, err := scoreTarget(material, cp)
	if err != nil {
		t.Fatal(err)
	}
	info := "info depth 8 seldepth 11 multipv 1 score cp " + strconvItoa(cp) + " nodes 4999 nps 100000 pv " + bestMove + " a7a6"
	infoDigest := sha256.Sum256([]byte(info))
	return labelRecord{
		Type: "label", ID: fixtureSHA(id), K4InputSHA256: hex.EncodeToString(board.k4InputSHA256[:]), Split: "train", OriginalFEN: fen, ResetFEN: reset, SourceMove: sourceMove,
		BestMove: bestMove, PVMove: bestMove, UCICP: cp, NGNScore: score, Target: target, MaterialUnits: material,
		MaterialNormalizer: normalizer, Depth: 8, SelDepth: 11, Nodes: 4_999,
		InfoLineSHA256: hex.EncodeToString(infoDigest[:]), TeacherInfoLine: info, Status: "accepted",
	}
}

func strconvItoa(value int) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var bytes [24]byte
	index := len(bytes)
	for value > 0 {
		index--
		bytes[index] = byte('0' + value%10)
		value /= 10
	}
	if negative {
		index--
		bytes[index] = '-'
	}
	return string(bytes[index:])
}

func rejectedFixture(t *testing.T) labelRecord {
	t.Helper()
	reset, _, err := resetPosition(whiteFEN)
	if err != nil {
		t.Fatal(err)
	}
	board, _, err := parseBoard(whiteFEN, 0)
	if err != nil {
		t.Fatal(err)
	}
	return labelRecord{Type: "label", ID: fixtureSHA("rejected"), K4InputSHA256: hex.EncodeToString(board.k4InputSHA256[:]), Split: "train", OriginalFEN: whiteFEN, ResetFEN: reset, SourceMove: "d2d4", Status: "rejected", RejectionReason: "saturated-tail"}
}

type fixture struct {
	header  outputHeader
	records []labelRecord
	footer  outputFooter
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	records := []labelRecord{
		acceptedFixture(t, "white", whiteFEN, "e2e4", "g1f3", 50),
		acceptedFixture(t, "black", blackFEN, "e7e5", "g8f6", -70),
		rejectedFixture(t),
	}
	return fixture{header: fixtureHeader(len(records)), records: records}
}

func (value *fixture) seal(t *testing.T) {
	t.Helper()
	previous, err := canonicalDigest(value.header)
	if err != nil {
		t.Fatal(err)
	}
	rejections := make(map[string]int)
	accepted, rejected := 0, 0
	for index := range value.records {
		value.records[index].PreviousRecordSHA256 = previous
		value.records[index].RecordSHA256 = ""
		value.records[index].RecordSHA256, err = recordDigest(value.records[index])
		if err != nil {
			t.Fatal(err)
		}
		previous = value.records[index].RecordSHA256
		if value.records[index].Status == "accepted" {
			accepted++
		} else {
			rejected++
			rejections[value.records[index].RejectionReason]++
		}
	}
	value.header.Input.RecordCount = len(value.records)
	value.footer = outputFooter{Type: "footer", Schema: LabelSchema, Records: len(value.records), Accepted: accepted, Rejected: rejected, Rejections: rejections, LastRecordSHA256: previous}
}

func (value *fixture) write(t *testing.T, path string) {
	t.Helper()
	value.seal(t)
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	encoder := json.NewEncoder(file)
	if err := encoder.Encode(value.header); err != nil {
		t.Fatal(err)
	}
	for _, record := range value.records {
		if err := encoder.Encode(record); err != nil {
			t.Fatal(err)
		}
	}
	if err := encoder.Encode(value.footer); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPackPublishesVerifiedBulletShard(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "labels.jsonl")
	output := filepath.Join(dir, "train.bf")
	receiptPath := filepath.Join(dir, "receipt.json")
	value := newFixture(t)
	value.write(t, input)
	receipt, err := Pack(input, output, receiptPath, []string{"fixture"})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Records != 3 || receipt.Accepted != 2 || receipt.Rejected != 1 || receipt.OutputBytes != 64 || receipt.ResultConstant != 1 {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	wantHex := "ffff00000000ffff1342253100000000888888889bcaadb95c00010404000000" +
		"ffff00001000efff1342253100000000888888889bcaadb97fff010404000000"
	if hex.EncodeToString(data) != wantHex {
		t.Fatalf("BF bytes mismatch\n got %x\nwant %s", data, wantHex)
	}
	if data[26] != 1 || data[58] != 1 {
		t.Fatal("WDL result is not constant draw encoding")
	}
	if _, err := os.Stat(receiptPath); err != nil {
		t.Fatal(err)
	}
	if _, err := Pack(input, output, receiptPath, nil); err == nil || !strings.Contains(err.Error(), "refusing existing") {
		t.Fatalf("expected no-clobber refusal, got %v", err)
	}
}

func TestK4InputIdentityGoldens(t *testing.T) {
	tests := []struct{ fen, want string }{
		{whiteFEN, "754c73a16b5b3d57e57872163f8df31d7f867efe6b6dd6d3590edb2e2dae1b16"},
		{blackFEN, "e8dc98ae95a55187e5498a2ee1678a7877a5bcf2fd154756f8ea46a80a0827df"},
	}
	for _, test := range tests {
		board, _, err := parseBoard(test.fen, 0)
		if err != nil {
			t.Fatal(err)
		}
		if got := hex.EncodeToString(board.k4InputSHA256[:]); got != test.want {
			t.Fatalf("K4 key for %s = %s, want %s", test.fen, got, test.want)
		}
	}
}

func TestPackRejectsResealedSemanticMutations(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*fixture)
		want   string
	}{
		{"target", func(value *fixture) { value.records[0].Target = math.Nextafter(value.records[0].Target, 1) }, "score contract mismatch"},
		{"network", func(value *fixture) { value.header.Teacher.SmallNetworkSHA256 = fixtureSHA("wrong net") }, "teacher provenance mismatch"},
		{"info", func(value *fixture) {
			value.records[0].TeacherInfoLine = strings.Replace(value.records[0].TeacherInfoLine, "score cp 50", "score cp 51", 1)
			digest := sha256.Sum256([]byte(value.records[0].TeacherInfoLine))
			value.records[0].InfoLineSHA256 = hex.EncodeToString(digest[:])
		}, "info-line fields mismatch"},
		{"perspective", func(value *fixture) { value.records[1].NGNScore = -value.records[1].NGNScore }, "score contract mismatch"},
		{"k4-input", func(value *fixture) { value.records[0].K4InputSHA256 = fixtureSHA("wrong K4 input") }, "K4 input identity mismatch"},
		{"duplicate", func(value *fixture) { value.records[1].ID = value.records[0].ID }, "duplicate label id"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			value := newFixture(t)
			test.mutate(&value)
			input := filepath.Join(dir, "labels.jsonl")
			value.write(t, input)
			_, err := Pack(input, filepath.Join(dir, "out.bf"), filepath.Join(dir, "receipt.json"), nil)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("got %v, want %q", err, test.want)
			}
			if _, statErr := os.Stat(filepath.Join(dir, "out.bf")); !os.IsNotExist(statErr) {
				t.Fatalf("failed pack published output: %v", statErr)
			}
		})
	}
}

func TestPackRejectsBrokenChainAndTruncation(t *testing.T) {
	for _, test := range []struct {
		name, want string
		mutate     func([]byte) []byte
	}{
		{"chain", "label identity/chain mismatch", func(data []byte) []byte {
			return []byte(strings.Replace(string(data), `"record_sha256":"`, `"record_sha256":"0`, 1))
		}},
		{"newline", "newline terminated", func(data []byte) []byte { return data[:len(data)-1] }},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			input := filepath.Join(dir, "labels.jsonl")
			value := newFixture(t)
			value.write(t, input)
			data, err := os.ReadFile(input)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(input, test.mutate(data), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err = Pack(input, filepath.Join(dir, "out.bf"), filepath.Join(dir, "receipt.json"), nil)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("got %v, want %q", err, test.want)
			}
		})
	}
}
