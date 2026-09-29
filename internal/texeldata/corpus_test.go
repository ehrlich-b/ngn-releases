package texeldata

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ehrlich-b/ngn/engine"
)

var mateLineA = []string{"a2a3", "a7a6", "b2b3", "b7b6", "c2c3", "c7c6", "d2d3", "d7d6", "e2e3", "e7e6", "h2h3", "h7h6", "g1f3", "g8f6", "f3g1", "f6g8", "g1f3", "g8f6", "f3g1", "f6g8"}
var mateLineB = []string{"b2b3", "b7b6", "a2a3", "a7a6", "c2c3", "c7c6", "d2d3", "d7d6", "e2e3", "e7e6", "h2h3", "h7h6", "g1f3", "g8f6", "f3g1", "f6g8", "g1f3", "g8f6", "f3g1", "f6g8"}
var stateLineA = []string{"a2a3", "a7a6", "a1a2", "a8a7", "a2a1", "a7a8", "h2h3", "h7h6", "g1f3", "g8f6", "f3g1", "f6g8", "g1f3", "g8f6", "f3g1", "f6g8"}
var stateLineB = []string{"a2a3", "a7a6", "g1f3", "g8f6", "f3g1", "f6g8", "h2h3", "h7h6", "b1c3", "b8c6", "c3b1", "c6b8", "g1f3", "g8f6", "f3g1", "f6g8"}

func validRecord(t *testing.T, id string, moves []string, openingPly int) GameRecord {
	t.Helper()
	pos, err := engine.ParseFEN(StartFEN)
	if err != nil {
		t.Fatal(err)
	}
	var opening, sample string
	for i, text := range moves {
		m, err := engine.ParseUCIMove(pos, text)
		if err != nil {
			t.Fatalf("%s at %d: %v", text, i, err)
		}
		pos.GameMakeMove(m)
		if i+1 == openingPly {
			opening = engine.GenerateFEN(pos)
		}
		if i+1 == 12 {
			sample = corpusFEN(pos, i+1)
		}
	}
	if openingPly == 0 {
		opening = StartFEN
	}
	result := 0.5
	return GameRecord{Version: SchemaVersion, Generator: GeneratorIdentity{ExecutableSHA256: "test-bin", SourceIdentity: "test-source"}, GameID: id, OpeningID: canonicalOpeningID(opening), OpeningPly: openingPly, Generation: testGeneration(0), Moves: moves, Result: &result, TerminalReason: "threefold", Completed: true, Samples: []SampleRecord{{Ply: 12, FEN: sample}}}
}

func testGeneration(index int) GenerationSettings {
	return GenerationSettings{Nodes: 8000, MaxPlies: 240, Seed: 1, OpeningIndex: index, SampleFilter: "engine.IsQuietPosition/v1", SampleFilterSource: "test-source:engine/texel.go"}
}

func TestCorpusSampleCarriesActualFullmoveNumber(t *testing.T) {
	r := validRecord(t, "fullmove", mateLineA, 6)
	if !strings.HasSuffix(r.Samples[0].FEN, " 0 7") {
		t.Fatalf("after 12 plies sample must be White's seventh move: %s", r.Samples[0].FEN)
	}
}

func writeRecords(t *testing.T, name string, records ...GameRecord) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range records {
		if err := EncodeRecord(f, r); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadKeepsOpeningAndGameTogetherAcrossFileOrder(t *testing.T) {
	a := validRecord(t, "a", mateLineA, 1)
	b := validRecord(t, "b", mateLineA, 1)
	p1 := writeRecords(t, "one.jsonl", a, b)
	p2 := writeRecords(t, "two.jsonl", b, a)
	x, rx, err := Load([]string{p1}, LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	y, ry, err := Load([]string{p2}, LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(x.Train) != len(y.Train) || len(x.Validation) != len(y.Validation) || len(x.Test) != len(y.Test) {
		t.Fatal("file order changed split")
	}
	if rx.Rows != 1 || ry.Rows != 1 || rx.Duplicates != 1 || ry.Duplicates != 1 {
		t.Fatalf("rows %d %d", rx.Rows, ry.Rows)
	}
}

func TestLoadBlocksBoardOnlyCrossPartitionLeak(t *testing.T) {
	// The two legal games reach equal piece placement at ply 12, but A has lost
	// castling rights through rook round trips. They must still collide because
	// TexelSample contains a BOARD, not side/castling/EP state.
	a := validRecord(t, "a", stateLineA, 1)
	b := validRecord(t, "b", stateLineB, 2)
	if a.Samples[0].FEN == b.Samples[0].FEN || boardKey(a.Samples[0].FEN) != boardKey(b.Samples[0].FEN) {
		t.Fatalf("fixture failed to isolate non-board state: %q / %q", a.Samples[0].FEN, b.Samples[0].FEN)
	}
	seed := ""
	for i := 0; ; i++ {
		seed = fmt.Sprintf("seed-%d", i)
		if partitionFor(seed, a.OpeningID, 34, 33) != partitionFor(seed, b.OpeningID, 34, 33) {
			break
		}
	}
	p := writeRecords(t, "state.jsonl", a, b)
	_, report, err := Load([]string{p}, LoadOptions{Seed: seed, TrainPercent: 34, ValidationPercent: 33, TestPercent: 33})
	if err != nil {
		t.Fatal(err)
	}
	if report.Rows != 1 || report.ExcludedCrossPartition != 1 {
		t.Fatalf("collision report=%+v", report)
	}
}

func TestLoadPreservesDistinctGameRowsWithinOnePartitionAndStableOrder(t *testing.T) {
	a := validRecord(t, "a", stateLineA, 1)
	b := validRecord(t, "b", stateLineB, 2)
	p1 := writeRecords(t, "forward.jsonl", a, b)
	p2 := writeRecords(t, "reverse.jsonl", b, a)
	opts := LoadOptions{TrainPercent: 100, ValidationPercent: 0, TestPercent: 0}
	x, rx, err := Load([]string{p1}, opts)
	if err != nil {
		t.Fatal(err)
	}
	y, ry, err := Load([]string{p2}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(x.Train) != 2 || rx.ExcludedCrossPartition != 0 || ry.ExcludedCrossPartition != 0 {
		t.Fatalf("same-partition rows were deduped: %+v", rx)
	}
	if rx.TrainCompletedGames != 2 || rx.TrainCompletedOpenings != 2 || rx.TrainDraws != 2 {
		t.Fatalf("acceptance-audit partition counts wrong: %+v", rx)
	}
	if !reflect.DeepEqual(x, y) {
		t.Fatal("file order changed output row order")
	}
}

func TestLoadRejectsMalformedAndInconsistentRecords(t *testing.T) {
	r := validRecord(t, "ok", mateLineA, 1)
	badSample := r
	badSample.Samples = []SampleRecord{{Ply: 12, FEN: StartFEN}}
	for name, data := range map[string]string{
		"truncated": `{"version":1`,
		"unknown":   `{"version":1,"generator":{"executable_sha256":"x","source_identity":"x"},"game_id":"x","opening_id":"` + StartFEN + `","opening_ply":0,"moves":[],"result":0.5,"terminal_reason":"madeup","completed":true,"samples":[]}`,
		"infinite":  `{"version":1,"generator":{"executable_sha256":"x","source_identity":"x"},"game_id":"x","opening_id":"` + StartFEN + `","opening_ply":0,"moves":[],"result":1e309,"terminal_reason":"stalemate","completed":true,"samples":[]}`,
	} {
		p := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(p, []byte(data+"\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if _, _, err := Load([]string{p}, LoadOptions{}); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	p := writeRecords(t, "mismatch", badSample)
	if _, _, err := Load([]string{p}, LoadOptions{}); err == nil {
		t.Fatal("replay mismatch accepted")
	}
	r2 := r
	r2.GameID = "different-id"
	r2.Generator.SourceIdentity = "other-source"
	p = writeRecords(t, "sameid", r, r2)
	if _, _, err := Load([]string{p}, LoadOptions{}); err == nil {
		t.Fatal("inconsistent id accepted")
	}
	r2 = r
	r2.GameID = "same-content-different-samples"
	r2.Samples = nil
	p = writeRecords(t, "samples", r, r2)
	if _, _, err := Load([]string{p}, LoadOptions{}); err == nil {
		t.Fatal("same game content with different samples accepted")
	}
}

func TestLoadRejectsTerminalPrefixesAndTerminalUnresolvedOrSample(t *testing.T) {
	r := validRecord(t, "terminal", mateLineA, 1)
	post := r
	post.Moves = append(append([]string{}, r.Moves...), "g1f3")
	p := writeRecords(t, "post-terminal", post)
	if _, _, err := Load([]string{p}, LoadOptions{}); err == nil {
		t.Fatal("move after terminal accepted")
	}
	unresolved := r
	unresolved.Completed, unresolved.Result, unresolved.TerminalReason, unresolved.Samples = false, nil, "maxplies", nil
	p = writeRecords(t, "terminal-unresolved", unresolved)
	if _, _, err := Load([]string{p}, LoadOptions{}); err == nil {
		t.Fatal("terminal unresolved accepted")
	}
	terminalSample := r
	terminalSample.Samples = []SampleRecord{{Ply: len(r.Moves), FEN: replayFEN(t, r.Moves, len(r.Moves))}}
	p = writeRecords(t, "terminal-sample", terminalSample)
	if _, _, err := Load([]string{p}, LoadOptions{}); err == nil {
		t.Fatal("terminal sample accepted")
	}
}

func TestLoadRejectsTruncatedUnresolvedGame(t *testing.T) {
	r := GameRecord{
		Version: SchemaVersion, Generator: GeneratorIdentity{ExecutableSHA256: "test-bin", SourceIdentity: "test-source"},
		GameID: "truncated", OpeningID: canonicalOpeningID(replayFEN(t, []string{"e2e4"}, 1)), OpeningPly: 1,
		Generation: GenerationSettings{Nodes: 8000, MaxPlies: 2, Seed: 1, OpeningIndex: 0, SampleFilter: "engine.IsQuietPosition/v1", SampleFilterSource: "test-source:engine/texel.go"},
		Moves:      []string{"e2e4"}, TerminalReason: "maxplies", Completed: false,
	}
	p := writeRecords(t, "truncated-unresolved", r)
	if _, _, err := Load([]string{p}, LoadOptions{}); err == nil {
		t.Fatal("truncated unresolved game accepted")
	}
}

func TestDuplicateBookIndicesDeduplicatePlayedGame(t *testing.T) {
	var body bytes.Buffer
	cfg := GenerateOptions{Openings: [][]string{{"e2e4"}, {"e2e4"}}, Games: 2, Nodes: 1, MaxPlies: 1, Seed: 11, Generator: GeneratorIdentity{ExecutableSHA256: "x", SourceIdentity: "source"}}
	if _, err := Generate(&body, cfg); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "duplicates.jsonl")
	if err := os.WriteFile(p, body.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	_, report, err := Load([]string{p}, LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Games != 1 || report.Duplicates != 1 || report.UnresolvedGames != 1 {
		t.Fatalf("duplicate book lines were not deduped: %+v", report)
	}
}

func TestCanonicalOpeningAndPerGameSeed(t *testing.T) {
	if canonicalOpeningID("8/8/8/8/8/8/8/K6k b - - 47 99") != "8/8/8/8/8/8/8/K6k b - -" {
		t.Fatal("opening counters leaked into key")
	}
	a, b := perGameRNG(17, "opening-key"), perGameRNG(17, "opening-key")
	if a.Int63() != b.Int63() {
		t.Fatal("per-game RNG not reproducible")
	}
	if gameID(GenerateOptions{Nodes: 8, MaxPlies: 240, Seed: 1, Generator: GeneratorIdentity{SourceIdentity: "s"}}, 4) == gameID(GenerateOptions{Nodes: 9, MaxPlies: 240, Seed: 1, Generator: GeneratorIdentity{SourceIdentity: "s"}}, 4) {
		t.Fatal("game ID omits generation settings")
	}
	full := GenerateOptions{Openings: [][]string{{"e2e4"}, {"d2d4"}}, Games: 2, Nodes: 1, MaxPlies: 1, Seed: 17, Generator: GeneratorIdentity{ExecutableSHA256: "x", SourceIdentity: "s"}}
	var all, shard bytes.Buffer
	if _, err := Generate(&all, full); err != nil {
		t.Fatal(err)
	}
	full.Start, full.Games = 1, 1
	if _, err := Generate(&shard, full); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(all.String()), "\n")
	if len(lines) != 2 || lines[1] != strings.TrimSpace(shard.String()) {
		t.Fatal("sharded generation changed record")
	}
}

func TestGenerateMaxpliesIsUnresolvedAndWriteErrorsPropagate(t *testing.T) {
	var b bytes.Buffer
	s, err := Generate(&b, GenerateOptions{Openings: [][]string{{"e2e4"}}, Games: 1, Nodes: 1, MaxPlies: 1, Generator: GeneratorIdentity{ExecutableSHA256: "x", SourceIdentity: "x"}})
	if err != nil {
		t.Fatal(err)
	}
	if s.Unresolved != 1 || strings.Contains(b.String(), `"result"`) {
		t.Fatalf("unresolved summary=%+v row=%s", s, b.String())
	}
	_, err = Generate(errWriter{}, GenerateOptions{Openings: [][]string{{"e2e4"}}, Games: 1, Nodes: 1, MaxPlies: 1, Generator: GeneratorIdentity{ExecutableSHA256: "x", SourceIdentity: "x"}})
	if err == nil {
		t.Fatal("write error lost")
	}
}

func TestGeneratorHandsPlayedMoveToNextRootSearch(t *testing.T) {
	for _, maxPlies := range []int{1, 2} {
		cfg := GenerateOptions{Openings: [][]string{{"e2e4"}}, Games: 1, Nodes: 1000, MaxPlies: maxPlies, Generator: GeneratorIdentity{ExecutableSHA256: "test", SourceIdentity: "test"}}
		r, err := playOne(cfg, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(r.Moves) != maxPlies {
			t.Fatalf("expected %d plies, got %v", maxPlies, r.Moves)
		}
		if got := engine.GetLastMovePlayed(); got == engine.EmptyMove || got.ToString() != r.Moves[len(r.Moves)-1] {
			t.Fatalf("after %d plies root predecessor=%v, want last played move %s", maxPlies, got, r.Moves[len(r.Moves)-1])
		}
	}
}

func TestGenerateAcceptsMateOnLastPermittedPly(t *testing.T) {
	var b bytes.Buffer
	s, err := Generate(&b, GenerateOptions{Openings: [][]string{{"f2f3", "e7e5", "g2g4"}}, Games: 1, Nodes: 8000, MaxPlies: 4, Generator: GeneratorIdentity{ExecutableSHA256: "x", SourceIdentity: "x"}})
	if err != nil {
		t.Fatal(err)
	}
	if s.Completed != 1 || s.Unresolved != 0 || !strings.Contains(b.String(), `"terminal_reason":"checkmate"`) {
		t.Fatalf("last-ply mate was not completed: %+v %s", s, b.String())
	}
}

func TestGenerateRejectsOpeningPastTerminal(t *testing.T) {
	var b bytes.Buffer
	_, err := Generate(&b, GenerateOptions{Openings: [][]string{{"f2f3", "e7e5", "g2g4", "d8h4", "e2e4"}}, Games: 1, Nodes: 1, MaxPlies: 8, Generator: GeneratorIdentity{ExecutableSHA256: "x", SourceIdentity: "x"}})
	if err == nil {
		t.Fatal("opening continuation after mate accepted")
	}
}

func TestTerminalRecognizesMateBeforeDrawAndStalemate(t *testing.T) {
	mate, err := engine.ParseFEN("7k/6Q1/6K1/8/8/8/8/8 b - - 100 1")
	if err != nil {
		t.Fatal(err)
	}
	if reason, result, ok := terminal(mate); !ok || reason != "checkmate" || result != 1 {
		t.Fatalf("mate=%s %.1f %v", reason, result, ok)
	}
	stale, err := engine.ParseFEN("7k/5Q2/6K1/8/8/8/8/8 b - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	if reason, result, ok := terminal(stale); !ok || reason != "stalemate" || result != .5 {
		t.Fatalf("stale=%s %.1f %v", reason, result, ok)
	}
}

func ptr(v float64) *float64 { return &v }

func replayFEN(t *testing.T, moves []string, ply int) string {
	t.Helper()
	pos, err := engine.ParseFEN(StartFEN)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < ply; i++ {
		m, err := engine.ParseUCIMove(pos, moves[i])
		if err != nil {
			t.Fatal(err)
		}
		pos.GameMakeMove(m)
	}
	return corpusFEN(pos, ply)
}

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, os.ErrPermission }
