//go:build counteroracle

package countereval_test

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
	"github.com/ehrlich-b/ngn/engine"
)

type contextOracleUpdate struct {
	Feature     int  `json:"feature"`
	Coefficient int8 `json:"coefficient"`
}

type contextOracleRecord struct {
	ID              string                `json:"id"`
	Sequence        string                `json:"sequence"`
	Step            int                   `json:"step"`
	Operation       string                `json:"operation"`
	Move            string                `json:"move,omitempty"`
	Depth           int                   `json:"depth"`
	PreFEN          string                `json:"pre_fen,omitempty"`
	PostFEN         string                `json:"post_fen"`
	Restores        string                `json:"restores,omitempty"`
	Updates         []contextOracleUpdate `json:"updates"`
	Features        []int                 `json:"features"`
	AccumulatorBits []uint32              `json:"accumulator_bits"`
	RawBits         uint32                `json:"raw_white_bits"`
}

type contextOracleOutput struct {
	Schema        string                `json:"schema"`
	CounterCommit string                `json:"counter_commit"`
	ModelSHA256   string                `json:"model_sha256"`
	Records       []contextOracleRecord `json:"records"`
}

type contextFixture struct {
	ID         string
	FEN        string
	Operations []string
}

type contextUndo struct {
	null      bool
	move      engine.Move
	enPassant engine.Square
	tag       engine.PositionTag
	halfClock uint8
}

func TestCounter55IncrementalTransitionOracleParity(t *testing.T) {
	modelPath := requiredExternalEnv(t, "COUNTER_MODEL")
	fixturePath := requiredExternalEnv(t, "COUNTER_TRANSITIONS")
	oraclePath := requiredExternalEnv(t, "COUNTER_TRANSITION_ORACLE_JSON")

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
	fixtures := readContextFixtures(t, fixturePath)
	encoded, err := os.ReadFile(oraclePath)
	if err != nil {
		t.Fatal(err)
	}
	var oracle contextOracleOutput
	if err := json.Unmarshal(encoded, &oracle); err != nil {
		t.Fatal(err)
	}
	if oracle.Schema != "counter55-transition-oracle-v1" ||
		oracle.CounterCommit != "63c487ca724c620f71c129d62129c6fb9109c872" ||
		oracle.ModelSHA256 != counter55ModelSHA256 {
		t.Fatalf("oracle identity mismatch: %+v", oracle)
	}
	records := make(map[string]contextOracleRecord, len(oracle.Records))
	for _, record := range oracle.Records {
		if record.ID == "" || records[record.ID].ID != "" {
			t.Fatalf("empty or duplicate oracle ID %q", record.ID)
		}
		records[record.ID] = record
	}

	visited := make(map[string]bool)
	for _, fixture := range fixtures {
		position, err := engine.ParseFEN(fixture.FEN)
		if err != nil {
			t.Fatalf("%s root: %v", fixture.ID, err)
		}
		rootBoard := externalBoardFromNGN(t, position)
		context, err := model.NewContext(rootBoard)
		if err != nil {
			t.Fatalf("%s root context: %v", fixture.ID, err)
		}
		frameIDs := []string{fixture.ID + "/root"}
		var undos []contextUndo
		rootRecord := records[frameIDs[0]]
		if rootRecord.Sequence != fixture.ID || rootRecord.Step != 0 ||
			rootRecord.Operation != "root" || rootRecord.Move != "" {
			t.Fatalf("%s malformed root metadata: %+v", fixture.ID, rootRecord)
		}
		requireContextOracleRecord(t, context, position, nil, rootRecord, frameIDs[0])
		visited[frameIDs[0]] = true

		for step, token := range fixture.Operations {
			recordID := fmt.Sprintf("%s/%02d-%s", fixture.ID, step+1, token)
			preFEN := engine.GenerateFEN(position)
			expectedOperation := "move"
			expectedMove := token
			var updates []contextOracleUpdate
			switch token {
			case "pop":
				expectedOperation, expectedMove = "pop", ""
				if len(undos) == 0 {
					t.Fatalf("%s: root pop", recordID)
				}
				undo := undos[len(undos)-1]
				undos = undos[:len(undos)-1]
				if undo.null {
					position.UnMakeNullMove(undo.enPassant)
				} else {
					position.UnMakeMove(undo.move, undo.tag, undo.enPassant, undo.halfClock)
				}
				if err := context.Pop(); err != nil {
					t.Fatalf("%s: %v", recordID, err)
				}
				frameIDs = frameIDs[:len(frameIDs)-1]
				if records[recordID].Restores != frameIDs[len(frameIDs)-1] {
					t.Fatalf("%s restores=%q want=%q", recordID, records[recordID].Restores, frameIDs[len(frameIDs)-1])
				}
			case "null":
				expectedOperation, expectedMove = "null", ""
				oldEP := position.MakeNullMove()
				undos = append(undos, contextUndo{null: true, enPassant: oldEP})
				if err := context.PushNull(); err != nil {
					t.Fatalf("%s: %v", recordID, err)
				}
				frameIDs = append(frameIDs, recordID)
			default:
				move, err := engine.ParseUCIMove(position, token)
				if err != nil {
					t.Fatalf("%s parse: %v", recordID, err)
				}
				delta := contextDeltaFromNGN(move)
				updates = contextUpdates(delta)
				oldEP, oldTag, oldClock, legal := position.MakeMove(move)
				if !legal {
					t.Fatalf("%s became illegal", recordID)
				}
				post := externalBoardFromNGN(t, position)
				if err := context.PushMove(delta, post); err != nil {
					t.Fatalf("%s context: %v", recordID, err)
				}
				undos = append(undos, contextUndo{move: move, enPassant: oldEP, tag: oldTag, halfClock: oldClock})
				frameIDs = append(frameIDs, recordID)
			}
			record := records[recordID]
			if record.Sequence != fixture.ID || record.Step != step+1 ||
				record.Operation != expectedOperation || record.Move != expectedMove ||
				!sameFirstFiveFEN(preFEN, record.PreFEN) {
				t.Fatalf("%s malformed transition metadata: %+v", recordID, record)
			}
			requireContextOracleRecord(t, context, position, updates, record, recordID)
			visited[recordID] = true
		}
	}
	if len(visited) != len(records) {
		t.Fatalf("visited records=%d oracle records=%d", len(visited), len(records))
	}
}

func contextDeltaFromNGN(move engine.Move) countereval.MoveDelta {
	moving := move.MovingPiece()
	delta := countereval.MoveDelta{
		MovingPlane: uint8(int(moving) - int(engine.WhitePawn)),
		From:        uint8(move.Source()),
		To:          uint8(move.Destination()),
	}
	if move.IsCapture() {
		captureSquare := move.Destination()
		if move.IsEnPassant() {
			if moving.Color() == engine.White {
				captureSquare -= 8
			} else {
				captureSquare += 8
			}
		}
		delta.HasCapture = true
		delta.CapturedPlane = uint8(int(move.CapturedPiece()) - int(engine.WhitePawn))
		delta.CaptureSquare = uint8(captureSquare)
	}
	if move.PromoType() != engine.NoType {
		delta.HasPromotion = true
		delta.PromotionPlane = uint8(int(engine.GetPiece(move.PromoType(), moving.Color())) - int(engine.WhitePawn))
	}
	if move.IsCastle() {
		delta.HasCastleRook = true
		switch {
		case moving.Color() == engine.White && move.IsKingSideCastle():
			delta.CastleRookFrom, delta.CastleRookTo = uint8(engine.H1), uint8(engine.F1)
		case moving.Color() == engine.White:
			delta.CastleRookFrom, delta.CastleRookTo = uint8(engine.A1), uint8(engine.D1)
		case move.IsKingSideCastle():
			delta.CastleRookFrom, delta.CastleRookTo = uint8(engine.H8), uint8(engine.F8)
		default:
			delta.CastleRookFrom, delta.CastleRookTo = uint8(engine.A8), uint8(engine.D8)
		}
	}
	return delta
}

func contextUpdates(delta countereval.MoveDelta) []contextOracleUpdate {
	updates := []contextOracleUpdate{{
		Feature: int(delta.MovingPlane)*64 + int(delta.From), Coefficient: -1,
	}}
	if delta.HasCapture {
		updates = append(updates, contextOracleUpdate{
			Feature: int(delta.CapturedPlane)*64 + int(delta.CaptureSquare), Coefficient: -1,
		})
	}
	targetPlane := delta.MovingPlane
	if delta.HasPromotion {
		targetPlane = delta.PromotionPlane
	}
	updates = append(updates, contextOracleUpdate{
		Feature: int(targetPlane)*64 + int(delta.To), Coefficient: 1,
	})
	if delta.HasCastleRook {
		rookPlane := 3
		if delta.MovingPlane >= 6 {
			rookPlane += 6
		}
		updates = append(updates,
			contextOracleUpdate{Feature: rookPlane*64 + int(delta.CastleRookFrom), Coefficient: -1},
			contextOracleUpdate{Feature: rookPlane*64 + int(delta.CastleRookTo), Coefficient: 1},
		)
	}
	return updates
}

func requireContextOracleRecord(
	t *testing.T,
	context *countereval.Context,
	position *engine.Position,
	updates []contextOracleUpdate,
	record contextOracleRecord,
	wantID string,
) {
	t.Helper()
	if record.ID != wantID {
		t.Fatalf("missing oracle record %s", wantID)
	}
	if record.Depth != context.Depth() {
		t.Fatalf("%s depth=%d want=%d", wantID, context.Depth(), record.Depth)
	}
	if !reflect.DeepEqual(record.Updates, updates) {
		t.Fatalf("%s updates=%v want=%v", wantID, updates, record.Updates)
	}
	board := context.Board()
	if want := externalBoardFromNGN(t, position); board != want {
		t.Fatalf("%s board differs", wantID)
	}
	if !reflect.DeepEqual(externalFeatures(board), record.Features) {
		t.Fatalf("%s active features differ", wantID)
	}
	bits, err := context.OracleAccumulatorBits()
	if err != nil {
		t.Fatalf("%s accumulator: %v", wantID, err)
	}
	if len(record.AccumulatorBits) != countereval.HiddenSize {
		t.Fatalf("%s oracle lanes=%d", wantID, len(record.AccumulatorBits))
	}
	for lane, got := range bits {
		if got != record.AccumulatorBits[lane] {
			t.Fatalf("%s lane %d bits=%08x want=%08x", wantID, lane, got, record.AccumulatorBits[lane])
		}
	}
	raw := context.EvaluateRaw()
	if got := math.Float32bits(raw); got != record.RawBits {
		t.Fatalf("%s raw bits=%08x want=%08x", wantID, got, record.RawBits)
	}
	if !sameFirstFiveFEN(engine.GenerateFEN(position), record.PostFEN) {
		t.Fatalf("%s post FEN differs: ngn=%q counter=%q", wantID, engine.GenerateFEN(position), record.PostFEN)
	}
}

func externalFeatures(board countereval.Board) []int {
	var result []int
	for square := 0; square < 64; square++ {
		mask := uint64(1) << square
		for plane := 0; plane < countereval.FeaturePlaneCount; plane++ {
			if board[plane]&mask != 0 {
				result = append(result, plane*64+square)
				break
			}
		}
	}
	return result
}

func sameFirstFiveFEN(a, b string) bool {
	af, aOK := canonicalNGNFirstFiveFEN(a)
	bf, bOK := canonicalNGNFirstFiveFEN(b)
	return aOK && bOK && af == bf
}

func canonicalNGNFirstFiveFEN(fen string) ([5]string, bool) {
	var canonical [5]string
	position, err := engine.ParseFEN(fen)
	if err != nil {
		return canonical, false
	}
	fields := strings.Fields(engine.GenerateFEN(position))
	if len(fields) != 6 {
		return canonical, false
	}
	copy(canonical[:], fields[:5])
	return canonical, true
}

func TestCounter55FENCompatibilityNormalizesOnlyPhantomEP(t *testing.T) {
	phantomEP := "7k/8/8/8/4P3/8/8/K7 b - e3 0 1"
	withoutEP := "7k/8/8/8/4P3/8/8/K7 b - - 0 1"
	if !sameFirstFiveFEN(phantomEP, withoutEP) {
		t.Fatal("non-capturable Counter EP target did not normalize to NGN identity")
	}

	capturableEP := "7k/8/8/8/3pP3/8/8/K7 b - e3 0 1"
	capturableWithoutEP := "7k/8/8/8/3pP3/8/8/K7 b - - 0 1"
	if sameFirstFiveFEN(capturableEP, capturableWithoutEP) {
		t.Fatal("capturable EP difference was normalized away")
	}
	if sameFirstFiveFEN(withoutEP, "7k/8/8/8/4P3/8/8/K7 w - - 0 1") {
		t.Fatal("side-to-move difference was ignored")
	}
	if sameFirstFiveFEN(withoutEP, "7k/8/8/8/4P3/8/8/K7 b K - 0 1") {
		t.Fatal("castling-right difference was ignored")
	}
	if sameFirstFiveFEN(withoutEP, "7k/8/8/8/4P3/8/8/K7 b - - 1 1") {
		t.Fatal("halfmove-clock difference was ignored")
	}
	if sameFirstFiveFEN(withoutEP, "not a FEN") {
		t.Fatal("malformed FEN was accepted")
	}
}

func readContextFixtures(t *testing.T, path string) []contextFixture {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var result []contextFixture
	seen := make(map[string]bool)
	scanner := bufio.NewScanner(file)
	for line := 1; scanner.Scan(); line++ {
		parts := strings.Split(scanner.Text(), "\t")
		if len(parts) != 3 {
			t.Fatalf("bad fixture line %d", line)
		}
		id := strings.TrimSpace(parts[0])
		operations := strings.Fields(parts[2])
		if id == "" || len(operations) == 0 || seen[id] {
			t.Fatalf("empty/duplicate fixture ID or empty operations at line %d", line)
		}
		seen[id] = true
		result = append(result, contextFixture{ID: id, FEN: parts[1], Operations: operations})
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(result) == 0 {
		t.Fatal("no transition fixtures")
	}
	return result
}
