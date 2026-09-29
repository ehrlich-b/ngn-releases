//go:build counteroracle

package engine

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/ehrlich-b/ngn/countereval"
)

const acceptedCounter55SHA256 = "3488baed71f4d432d028e05b65e2ff13f8c9afe7f91974d1a3c34a0691d6670c"

type counterSearchOracleRecord struct {
	ID              string   `json:"id"`
	Sequence        string   `json:"sequence"`
	Step            int      `json:"step"`
	Operation       string   `json:"operation"`
	Move            string   `json:"move,omitempty"`
	Depth           int      `json:"depth"`
	Restores        string   `json:"restores,omitempty"`
	Features        []int    `json:"features"`
	AccumulatorBits []uint32 `json:"accumulator_bits"`
	RawBits         uint32   `json:"raw_white_bits"`
}

type counterSearchOracle struct {
	Schema        string                      `json:"schema"`
	CounterCommit string                      `json:"counter_commit"`
	ModelSHA256   string                      `json:"model_sha256"`
	Records       []counterSearchOracleRecord `json:"records"`
}

type counterSearchFixture struct {
	id         string
	fen        string
	operations []string
}

type counterSearchUndo struct {
	null      bool
	move      Move
	enPassant Square
	tag       PositionTag
	halfClock uint8
}

func TestCounterSearchAdapterMatchesPinnedTransitionOracle(t *testing.T) {
	modelPath := requireCounterOracleEnv(t, "COUNTER_MODEL")
	fixturePath := requireCounterOracleEnv(t, "COUNTER_TRANSITIONS")
	oraclePath := requireCounterOracleEnv(t, "COUNTER_TRANSITION_ORACLE_JSON")

	modelBytes, err := os.ReadFile(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	model, metadata, err := countereval.LoadCounter55Legacy(bytes.NewReader(modelBytes))
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
	var oracle counterSearchOracle
	if err := json.Unmarshal(encoded, &oracle); err != nil {
		t.Fatal(err)
	}
	if oracle.Schema != "counter55-transition-oracle-v1" ||
		oracle.CounterCommit != "63c487ca724c620f71c129d62129c6fb9109c872" ||
		oracle.ModelSHA256 != acceptedCounter55SHA256 {
		t.Fatalf("oracle identity=%+v", oracle)
	}
	records := make(map[string]counterSearchOracleRecord, len(oracle.Records))
	for _, record := range oracle.Records {
		if record.ID == "" || records[record.ID].ID != "" {
			t.Fatalf("empty or duplicate oracle record %q", record.ID)
		}
		records[record.ID] = record
	}

	visited := make(map[string]bool, len(records))
	for _, fixture := range readCounterSearchFixtures(t, fixturePath) {
		position, err := ParseFEN(fixture.fen)
		if err != nil {
			t.Fatalf("%s root: %v", fixture.id, err)
		}
		cold, err := counter55EvaluatorModel(model, 1)
		if err != nil {
			t.Fatal(err)
		}
		worker, err := cold.newWorker(nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := worker.Reset(position); err != nil {
			t.Fatal(err)
		}
		frameIDs := []string{fixture.id + "/root"}
		var undos []counterSearchUndo
		requireCounterSearchOracleFrame(t, worker, position, records[frameIDs[0]], frameIDs[0])
		visited[frameIDs[0]] = true

		for step, token := range fixture.operations {
			recordID := fmt.Sprintf("%s/%02d-%s", fixture.id, step+1, token)
			expectedOperation := "move"
			expectedMove := token
			switch token {
			case "pop":
				expectedOperation, expectedMove = "pop", ""
				if len(undos) == 0 {
					t.Fatalf("%s root pop", recordID)
				}
				if err := worker.Pop(); err != nil {
					t.Fatal(err)
				}
				undo := undos[len(undos)-1]
				undos = undos[:len(undos)-1]
				if undo.null {
					position.UnMakeNullMove(undo.enPassant)
				} else {
					position.UnMakeMove(undo.move, undo.tag, undo.enPassant, undo.halfClock)
				}
				frameIDs = frameIDs[:len(frameIDs)-1]
				if records[recordID].Restores != frameIDs[len(frameIDs)-1] {
					t.Fatalf("%s restores=%q want=%q", recordID, records[recordID].Restores, frameIDs[len(frameIDs)-1])
				}
			case "null":
				expectedOperation, expectedMove = "null", ""
				transition, err := worker.PrepareNull(position)
				if err != nil {
					t.Fatal(err)
				}
				oldEP := position.MakeNullMove()
				if err := worker.PushNull(position, transition); err != nil {
					t.Fatal(err)
				}
				undos = append(undos, counterSearchUndo{null: true, enPassant: oldEP})
				frameIDs = append(frameIDs, recordID)
			default:
				move, err := ParseUCIMove(position, token)
				if err != nil {
					t.Fatalf("%s: %v", recordID, err)
				}
				transition, err := worker.PrepareMove(position, move)
				if err != nil {
					t.Fatalf("%s prepare: %v", recordID, err)
				}
				oldEP, oldTag, oldClock, legal := position.MakeMove(move)
				if !legal {
					t.Fatalf("%s became illegal", recordID)
				}
				if err := worker.PushMove(position, transition); err != nil {
					t.Fatalf("%s push: %v", recordID, err)
				}
				undos = append(undos, counterSearchUndo{move: move, enPassant: oldEP, tag: oldTag, halfClock: oldClock})
				frameIDs = append(frameIDs, recordID)
			}
			record := records[recordID]
			if record.Sequence != fixture.id || record.Step != step+1 || record.Operation != expectedOperation || record.Move != expectedMove {
				t.Fatalf("%s transition metadata=%+v", recordID, record)
			}
			requireCounterSearchOracleFrame(t, worker, position, record, recordID)
			visited[recordID] = true
		}
	}
	if len(visited) != len(records) {
		t.Fatalf("visited records=%d oracle records=%d", len(visited), len(records))
	}
}

func requireCounterSearchOracleFrame(t *testing.T, worker *workerEvaluator, position *Position, record counterSearchOracleRecord, wantID string) {
	t.Helper()
	if record.ID != wantID || record.Depth != worker.counterContext.Depth() {
		t.Fatalf("record=%q depth=%d, want=%q depth=%d", record.ID, record.Depth, wantID, worker.counterContext.Depth())
	}
	board, err := counterBoardFromPosition(position)
	if err != nil {
		t.Fatal(err)
	}
	if board != worker.counterContext.Board() {
		t.Fatalf("%s direct bitboards differ from search context", wantID)
	}
	features := make([]int, 0, 32)
	for square := 0; square < 64; square++ {
		mask := uint64(1) << square
		for plane := 0; plane < countereval.FeaturePlaneCount; plane++ {
			if board[plane]&mask != 0 {
				features = append(features, plane*64+square)
				break
			}
		}
	}
	if !reflect.DeepEqual(features, record.Features) {
		t.Fatalf("%s ordered features=%v want=%v", wantID, features, record.Features)
	}
	bits, err := worker.counterContext.OracleAccumulatorBits()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(bits[:], record.AccumulatorBits) {
		t.Fatalf("%s accumulator bits differ", wantID)
	}
	if got := math.Float32bits(worker.counterContext.EvaluateRaw()); got != record.RawBits {
		t.Fatalf("%s raw bits=%08x want=%08x", wantID, got, record.RawBits)
	}
}

func readCounterSearchFixtures(t *testing.T, path string) []counterSearchFixture {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var result []counterSearchFixture
	seen := make(map[string]bool)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 3 {
			t.Fatalf("bad transition fixture %q", line)
		}
		id := strings.TrimSpace(parts[0])
		fen := strings.TrimSpace(parts[1])
		operationText := strings.TrimSpace(parts[2])
		operations := strings.Fields(operationText)
		if id == "" || fen == "" || operationText == "" || len(operations) == 0 || seen[id] {
			t.Fatalf("bad/duplicate transition fixture %q", line)
		}
		seen[id] = true
		result = append(result, counterSearchFixture{id: id, fen: fen, operations: operations})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(result) == 0 {
		t.Fatal("empty transition fixtures")
	}
	return result
}

func requireCounterOracleEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required", name)
	}
	return value
}
