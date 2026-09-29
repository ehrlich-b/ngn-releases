package k4label

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeAnalyzer struct {
	provenance TeacherProvenance
	results    map[string]SearchResult
	failAfter  int
	calls      []string
}

func TestRunShardRejectsDiagnosticTeacherConfiguration(t *testing.T) {
	config := FrozenSearchConfig()
	config.Nodes = 20_000
	_, err := RunShard(context.Background(), "unused", filepath.Join(t.TempDir(), "labels.jsonl"), &Session{config: config})
	if !errors.Is(err, ErrContract) {
		t.Fatalf("diagnostic teacher RunShard error = %v, want contract rejection", err)
	}
}

func (analyzer *fakeAnalyzer) Provenance() TeacherProvenance { return analyzer.provenance }
func (analyzer *fakeAnalyzer) Close() error                  { return nil }
func (analyzer *fakeAnalyzer) Analyze(_ context.Context, fen string) (SearchResult, string, error) {
	if analyzer.failAfter >= 0 && len(analyzer.calls) == analyzer.failAfter {
		return SearchResult{}, "", errors.New("injected teacher failure")
	}
	analyzer.calls = append(analyzer.calls, fen)
	result, exists := analyzer.results[fen]
	if !exists {
		return SearchResult{}, "", errors.New("unexpected fixture FEN")
	}
	return result, "", nil
}

func fakeProvenance(identity string) TeacherProvenance {
	return TeacherProvenance{
		Name: "fixture teacher", ExecutablePath: "/fixture/stockfish",
		ExecutableSHA256: testSHA("executable " + identity), SourceCommit: TeacherSourceCommit,
		BigNetworkSHA256: TeacherBigNetworkSHA, SmallNetworkSHA256: TeacherSmallNetSHA,
		Command: []string{"/fixture/stockfish"}, HandshakeOptionDigest: testSHA("options"),
	}
}

func fakeResults(t *testing.T, positions []InputPosition) map[string]SearchResult {
	t.Helper()
	best := []string{"e2e4", "e7e5", "e3d4"}
	cp := []int{100, -75, 33}
	results := make(map[string]SearchResult)
	for index, input := range positions {
		fen, _, err := resetClockFEN(input.FEN)
		if err != nil {
			t.Fatal(err)
		}
		info := "info depth 8 seldepth 12 multipv 1 score cp " + jsonNumber(cp[index]) +
			" nodes 5000 pv " + best[index]
		results[fen] = SearchResult{
			BestMove: best[index], PVMove: best[index], CP: cp[index], Depth: 8,
			SelDepth: 12, Nodes: 5000, InfoLine: info,
		}
	}
	return results
}

func jsonNumber(value int) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func TestRunShardResumesVerifiedJournalAndPublishesNoClobber(t *testing.T) {
	positions := testPositions(t)
	input := writeInputShard(t, positions)
	output := filepath.Join(t.TempDir(), "labeled.jsonl")
	first := &fakeAnalyzer{provenance: fakeProvenance("same"), results: fakeResults(t, positions), failAfter: 2}
	if _, err := RunShard(context.Background(), input, output, first); err == nil || !strings.Contains(err.Error(), "injected") {
		t.Fatalf("first run error = %v", err)
	}
	partial := output + ".partial"
	file, err := os.OpenFile(partial, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	const torn = `{"type":"label"`
	if _, err := file.WriteString(torn); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	second := &fakeAnalyzer{provenance: fakeProvenance("same"), results: fakeResults(t, positions), failAfter: -1}
	receipt, err := RunShard(context.Background(), input, output, second)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.calls) != 1 || receipt.Records != 3 || receipt.Accepted != 2 || receipt.Rejected != 1 ||
		receipt.Rejections["nonquiet-bestmove"] != 1 || receipt.RecoveredPartialByte != int64(len(torn)) ||
		!validLowerHexSHA(receipt.OutputSHA256) {
		t.Fatalf("resume receipt=%+v calls=%v", receipt, second.calls)
	}
	if _, err := os.Stat(partial); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial remains after publish: %v", err)
	}

	written, err := os.Open(output)
	if err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(written)
	var lines [][]byte
	for scanner.Scan() {
		lines = append(lines, append([]byte(nil), scanner.Bytes()...))
	}
	_ = written.Close()
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(lines) != 5 {
		t.Fatalf("output lines=%d, want header+3+footer", len(lines))
	}
	var footer OutputFooter
	if err := json.Unmarshal(lines[len(lines)-1], &footer); err != nil || footer.Accepted != 2 ||
		footer.Rejected != 1 || footer.RecoveredPartialByte != int64(len(torn)) {
		t.Fatalf("footer=%+v err=%v", footer, err)
	}
	if _, err := RunShard(context.Background(), input, output, second); err == nil || !strings.Contains(err.Error(), "refusing existing") {
		t.Fatalf("existing output rerun error = %v", err)
	}
	receiptPath := filepath.Join(t.TempDir(), "receipt.json")
	if err := WriteReceiptNew(receiptPath, receipt); err != nil {
		t.Fatal(err)
	}
	if err := WriteReceiptNew(receiptPath, receipt); err == nil {
		t.Fatal("receipt publisher replaced existing evidence")
	}
}

func TestRunShardRejectsTamperedResumeChainBeforeTeacher(t *testing.T) {
	positions := testPositions(t)[:2]
	input := writeInputShard(t, positions)
	output := filepath.Join(t.TempDir(), "labeled.jsonl")
	first := &fakeAnalyzer{provenance: fakeProvenance("same"), results: fakeResults(t, testPositions(t)), failAfter: 1}
	if _, err := RunShard(context.Background(), input, output, first); err == nil {
		t.Fatal("injected first run succeeded")
	}
	partial := output + ".partial"
	data, err := os.ReadFile(partial)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("partial lines=%d", len(lines))
	}
	var record LabelRecord
	if err := json.Unmarshal([]byte(lines[1]), &record); err != nil {
		t.Fatal(err)
	}
	record.UCICP++
	mutated, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(partial, []byte(lines[0]+"\n"+string(mutated)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	second := &fakeAnalyzer{provenance: fakeProvenance("same"), results: fakeResults(t, testPositions(t)), failAfter: -1}
	if _, err := RunShard(context.Background(), input, output, second); err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("tampered resume error = %v", err)
	}
	if len(second.calls) != 0 {
		t.Fatalf("teacher called before resume verification: %v", second.calls)
	}
}

func TestRunShardRejectsChangedTeacherProvenanceOnResume(t *testing.T) {
	positions := testPositions(t)[:2]
	input := writeInputShard(t, positions)
	output := filepath.Join(t.TempDir(), "labeled.jsonl")
	first := &fakeAnalyzer{provenance: fakeProvenance("first"), results: fakeResults(t, testPositions(t)), failAfter: 1}
	if _, err := RunShard(context.Background(), input, output, first); err == nil {
		t.Fatal("injected first run succeeded")
	}
	second := &fakeAnalyzer{provenance: fakeProvenance("second"), results: fakeResults(t, testPositions(t)), failAfter: -1}
	if _, err := RunShard(context.Background(), input, output, second); err == nil || !strings.Contains(err.Error(), "header does not match") {
		t.Fatalf("changed provenance error = %v", err)
	}
	if len(second.calls) != 0 {
		t.Fatal("changed-provenance teacher was used")
	}
}
