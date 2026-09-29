package sf18small

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	base "github.com/ehrlich-b/ngn/nnue"
)

const (
	incrementalCoreOracleEnvironment   = "NGN_SF18_SMALL_INCREMENTAL_CORE_JSONL"
	incrementalGrowthOracleEnvironment = "NGN_SF18_SMALL_INCREMENTAL_GROWTH_JSONL"
	incrementalFixtureSHA256           = "fe843291d9dde5b52bedc6b81debeeb99017d552009d811a854603c661906246"
	incrementalSequenceSchema          = "sf18-small-incremental-sequences/v2"
	incrementalRowSchema               = "sf18-small-incremental-oracle/v1"
)

type incrementalSequenceManifest struct {
	Schema        string                              `json:"schema"`
	SourceCommit  string                              `json:"source_commit"`
	NetworkSHA256 string                              `json:"network_sha256"`
	Suites        map[string]incrementalSequenceSuite `json:"suites"`
}

type incrementalSequenceSuite struct {
	CaseCount int                       `json:"case_count"`
	RowCount  int                       `json:"row_count"`
	Cases     []incrementalSequenceCase `json:"cases"`
}

type incrementalSequenceCase struct {
	ID      string   `json:"id"`
	FEN     string   `json:"fen"`
	Actions []string `json:"actions"`
}

type incrementalDirty struct {
	Mover   base.PieceOnSquare  `json:"mover"`
	To      *base.Square        `json:"to"`
	Removed *base.PieceOnSquare `json:"removed"`
	Added   *base.PieceOnSquare `json:"added"`
}

type incrementalOracleRow struct {
	Schema           string
	Case             string
	SequenceIndex    int
	Operation        string
	Action           string
	LogicalDepth     int
	AccumulatorDepth int
	MoveType         *string
	Dirty            *incrementalDirty
	RequiresRefresh  [2]bool
	UpdateKind       [2]string
	State            upstreamOracleCase
}

type expectedIncrementalRow struct {
	caseID           string
	sequenceIndex    int
	operation        string
	action           string
	logicalDepth     int
	accumulatorDepth int
	moveType         *string
	dirty            *incrementalDirty
	requiresRefresh  [2]bool
	updateKind       [2]string
	position         base.Position
	rootFEN          string
	delta            *base.Delta
}

type replayAction struct {
	text     string
	before   base.Position
	after    base.Position
	delta    *base.Delta
	dirty    *incrementalDirty
	moveType *string
	refresh  [2]bool
	isNull   bool
}

func TestFrozenIncrementalOracleFixture(t *testing.T) {
	manifest := boundIncrementalSequenceManifest(t)
	if got := manifest.Suites["core"].CaseCount + manifest.Suites["growth"].CaseCount; got != 143 {
		t.Fatalf("incremental case count = %d, want 143", got)
	}
	if got := manifest.Suites["core"].RowCount + manifest.Suites["growth"].RowCount; got != 703 {
		t.Fatalf("incremental row count = %d, want 703", got)
	}
	assertIncrementalKingCorpus(t, manifest.Suites["core"])
}

func TestOfficialIncrementalUpstreamOracleCore(t *testing.T) {
	runOfficialIncrementalOracle(t, "core", incrementalCoreOracleEnvironment)
}

func TestOfficialIncrementalUpstreamOracleGrowth(t *testing.T) {
	runOfficialIncrementalOracle(t, "growth", incrementalGrowthOracleEnvironment)
}

func boundIncrementalSequenceManifest(t *testing.T) incrementalSequenceManifest {
	t.Helper()
	const path = "testdata/upstream_incremental_oracle/sequences.json"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	if got := hex.EncodeToString(digest[:]); got != incrementalFixtureSHA256 {
		t.Fatalf("incremental fixture digest = %s, want %s", got, incrementalFixtureSHA256)
	}
	var manifest incrementalSequenceManifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		t.Fatal("incremental fixture has trailing JSON")
	}
	if manifest.Schema != incrementalSequenceSchema || manifest.SourceCommit != officialSourceCommit ||
		manifest.NetworkSHA256 != officialNetworkSHA256 {
		t.Fatalf("incremental fixture identity = %q/%q/%q", manifest.Schema, manifest.SourceCommit, manifest.NetworkSHA256)
	}
	if len(manifest.Suites) != 2 {
		t.Fatalf("incremental fixture has %d suites, want 2", len(manifest.Suites))
	}
	for _, name := range []string{"core", "growth"} {
		suite, ok := manifest.Suites[name]
		if !ok {
			t.Fatalf("incremental fixture misses suite %q", name)
		}
		if suite.CaseCount != len(suite.Cases) {
			t.Fatalf("suite %s case count = %d, actual %d", name, suite.CaseCount, len(suite.Cases))
		}
		ids := make(map[string]bool)
		rows := 0
		for index, sequence := range suite.Cases {
			if sequence.ID == "" || sequence.FEN == "" || ids[sequence.ID] {
				t.Fatalf("suite %s case %d has empty/duplicate identity", name, index)
			}
			ids[sequence.ID] = true
			if _, err := parseOracleFEN(sequence.FEN); err != nil {
				t.Fatalf("suite %s case %s FEN: %v", name, sequence.ID, err)
			}
			for actionIndex, action := range sequence.Actions {
				if action == "" {
					t.Fatalf("suite %s case %s action %d is empty", name, sequence.ID, actionIndex)
				}
			}
			rows += 1 + 2*len(sequence.Actions)
		}
		if rows != suite.RowCount {
			t.Fatalf("suite %s row count = %d, actual %d", name, suite.RowCount, rows)
		}
	}
	return manifest
}

func assertIncrementalKingCorpus(t *testing.T, suite incrementalSequenceSuite) {
	t.Helper()
	seen := make(map[string]bool)
	for _, sequence := range suite.Cases {
		if strings.HasPrefix(sequence.ID, "king-white-") || strings.HasPrefix(sequence.ID, "king-black-") {
			if len(sequence.Actions) != 1 {
				t.Fatalf("king corpus %s has %d actions", sequence.ID, len(sequence.Actions))
			}
			seen[sequence.ID] = true
		}
	}
	for _, color := range []string{"white", "black"} {
		for square := 0; square < 64; square++ {
			id := fmt.Sprintf("king-%s-%c%d", color, 'a'+rune(square&7), square/8+1)
			if !seen[id] {
				t.Fatalf("incremental fixture misses %s", id)
			}
		}
	}
	if len(seen) != 128 {
		t.Fatalf("incremental king corpus has %d cases, want 128", len(seen))
	}
}

func runOfficialIncrementalOracle(t *testing.T, suiteName, oracleEnvironment string) {
	t.Helper()
	manifest := boundIncrementalSequenceManifest(t)
	suite := manifest.Suites[suiteName]
	networkPath := os.Getenv(officialFileEnvironment)
	oraclePath := os.Getenv(oracleEnvironment)
	if networkPath == "" || oraclePath == "" {
		t.Skip("official network and incremental upstream oracle JSONL are required for this explicit gate")
	}
	network, err := os.Open(networkPath)
	if err != nil {
		t.Fatal(err)
	}
	model, err := Load(network)
	network.Close()
	if err != nil {
		t.Fatal(err)
	}
	metadata := model.Metadata()
	if digest := hex.EncodeToString(metadata.FileSHA256[:]); digest != officialNetworkSHA256 {
		t.Fatalf("loaded model digest = %s, want %s", digest, officialNetworkSHA256)
	}

	oracle, err := os.Open(oraclePath)
	if err != nil {
		t.Fatal(err)
	}
	defer oracle.Close()
	rows, err := decodeIncrementalOracleRows(oracle, suite)
	if err != nil {
		t.Fatal(err)
	}
	rowOffset := 0
	for _, sequence := range suite.Cases {
		expected, err := prepareIncrementalRows(sequence)
		if err != nil {
			t.Fatalf("case %s: %v", sequence.ID, err)
		}
		caseRows := rows[rowOffset : rowOffset+len(expected)]
		rowOffset += len(expected)
		replayIncrementalCase(t, model, suiteName, expected, caseRows)
	}
	if rowOffset != len(rows) {
		t.Fatalf("suite %s consumed %d rows, decoded %d", suiteName, rowOffset, len(rows))
	}
}

func decodeIncrementalOracleRows(reader io.Reader, suite incrementalSequenceSuite) ([]incrementalOracleRow, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64<<10), 4<<20)
	rows := make([]incrementalOracleRow, 0, suite.RowCount)
	for scanner.Scan() {
		if len(rows) >= suite.RowCount {
			return nil, fmt.Errorf("incremental oracle has extra row %d", len(rows))
		}
		row, err := decodeIncrementalOracleRow(scanner.Bytes())
		if err != nil {
			return nil, fmt.Errorf("incremental oracle row %d: %w", len(rows), err)
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(rows) != suite.RowCount {
		return nil, fmt.Errorf("incremental oracle has %d rows, want %d", len(rows), suite.RowCount)
	}
	return rows, nil
}

func decodeIncrementalOracleRow(data []byte) (incrementalOracleRow, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return incrementalOracleRow{}, err
	}
	fields := []string{"schema", "case", "sequence_index", "operation", "action", "logical_depth", "accumulator_depth", "move_type", "dirty", "requires_refresh", "update_kind", "state"}
	if err := requireJSONObject(raw, "incremental row", fields); err != nil {
		return incrementalOracleRow{}, err
	}
	for _, name := range []string{"schema", "case", "sequence_index", "operation", "action", "logical_depth", "accumulator_depth", "requires_refresh", "update_kind", "state"} {
		if bytes.Equal(bytes.TrimSpace(raw[name]), []byte("null")) {
			return incrementalOracleRow{}, fmt.Errorf("incremental row.%s is null", name)
		}
	}
	if _, err := requireJSONArray(raw["requires_refresh"], "requires_refresh", 2); err != nil {
		return incrementalOracleRow{}, err
	}
	if _, err := requireJSONArray(raw["update_kind"], "update_kind", 2); err != nil {
		return incrementalOracleRow{}, err
	}

	var wire struct {
		Schema           string          `json:"schema"`
		Case             string          `json:"case"`
		SequenceIndex    int             `json:"sequence_index"`
		Operation        string          `json:"operation"`
		Action           string          `json:"action"`
		LogicalDepth     int             `json:"logical_depth"`
		AccumulatorDepth int             `json:"accumulator_depth"`
		MoveType         *string         `json:"move_type"`
		RequiresRefresh  [2]bool         `json:"requires_refresh"`
		UpdateKind       [2]string       `json:"update_kind"`
		Dirty            json.RawMessage `json:"dirty"`
		State            json.RawMessage `json:"state"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return incrementalOracleRow{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return incrementalOracleRow{}, fmt.Errorf("trailing JSON value")
	}
	state, err := decodeOracleCase(wire.State)
	if err != nil {
		return incrementalOracleRow{}, fmt.Errorf("state: %w", err)
	}
	dirty, err := decodeIncrementalDirty(wire.Dirty)
	if err != nil {
		return incrementalOracleRow{}, err
	}
	return incrementalOracleRow{
		Schema: wire.Schema, Case: wire.Case, SequenceIndex: wire.SequenceIndex,
		Operation: wire.Operation, Action: wire.Action, LogicalDepth: wire.LogicalDepth,
		AccumulatorDepth: wire.AccumulatorDepth, MoveType: wire.MoveType, Dirty: dirty,
		RequiresRefresh: wire.RequiresRefresh, UpdateKind: wire.UpdateKind, State: state,
	}, nil
}

func decodeIncrementalDirty(raw json.RawMessage) (*incrementalDirty, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	object, err := rawObject(raw, "dirty")
	if err != nil {
		return nil, err
	}
	if err := requireJSONObject(object, "dirty", []string{"mover", "to", "removed", "added"}); err != nil {
		return nil, err
	}
	mover, err := decodeIncrementalPiece(object["mover"], "dirty.mover", false)
	if err != nil {
		return nil, err
	}
	removed, err := decodeIncrementalPiece(object["removed"], "dirty.removed", true)
	if err != nil {
		return nil, err
	}
	added, err := decodeIncrementalPiece(object["added"], "dirty.added", true)
	if err != nil {
		return nil, err
	}
	var to *base.Square
	if !bytes.Equal(bytes.TrimSpace(object["to"]), []byte("null")) {
		var square base.Square
		if err := json.Unmarshal(object["to"], &square); err != nil || square >= 64 {
			return nil, fmt.Errorf("dirty.to is invalid")
		}
		to = &square
	}
	return &incrementalDirty{Mover: *mover, To: to, Removed: removed, Added: added}, nil
}

func decodeIncrementalPiece(raw json.RawMessage, name string, nullable bool) (*base.PieceOnSquare, error) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		if nullable {
			return nil, nil
		}
		return nil, fmt.Errorf("%s is null", name)
	}
	if err := rejectJSONNull(raw); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	object, err := rawObject(raw, name)
	if err != nil {
		return nil, err
	}
	if err := requireJSONObject(object, name, []string{"Piece", "Color", "Square"}); err != nil {
		return nil, err
	}
	var piece base.PieceOnSquare
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&piece); err != nil {
		return nil, err
	}
	if piece.Piece > base.King || piece.Color > base.Black || piece.Square >= 64 {
		return nil, fmt.Errorf("%s is out of range", name)
	}
	return &piece, nil
}

func prepareIncrementalRows(sequence incrementalSequenceCase) ([]expectedIncrementalRow, error) {
	root, err := parseOracleFEN(sequence.FEN)
	if err != nil {
		return nil, err
	}
	rows := []expectedIncrementalRow{{
		caseID: sequence.ID, operation: "root", position: root, rootFEN: sequence.FEN,
		updateKind: [2]string{"refresh", "refresh"},
	}}
	current := root
	accumulatorDepth := 0
	applied := make([]replayAction, 0, len(sequence.Actions))
	for _, action := range sequence.Actions {
		before := cloneIncrementalPosition(current)
		if action == "null" {
			current = cloneIncrementalPosition(current)
			current.SideToMove ^= 1
			applied = append(applied, replayAction{text: action, before: before, after: current, isNull: true})
			rows = append(rows, expectedIncrementalRow{
				caseID: sequence.ID, operation: "push_null", action: action,
				logicalDepth: len(applied), accumulatorDepth: accumulatorDepth,
				position: current, updateKind: [2]string{"reuse", "reuse"},
			})
			continue
		}
		after, delta, dirty, moveType, refresh, err := applyIncrementalAction(current, action)
		if err != nil {
			return nil, fmt.Errorf("action %s: %w", action, err)
		}
		current = after
		accumulatorDepth++
		applied = append(applied, replayAction{
			text: action, before: before, after: after, delta: &delta,
			dirty: dirty, moveType: &moveType, refresh: refresh,
		})
		updates := [2]string{"incremental", "incremental"}
		for perspective := range refresh {
			if refresh[perspective] {
				updates[perspective] = "refresh"
			}
		}
		rows = append(rows, expectedIncrementalRow{
			caseID: sequence.ID, operation: "push", action: action,
			logicalDepth: len(applied), accumulatorDepth: accumulatorDepth,
			moveType: &moveType, dirty: dirty, requiresRefresh: refresh,
			updateKind: updates, position: after, delta: &delta,
		})
	}
	for len(applied) > 0 {
		action := applied[len(applied)-1]
		applied = applied[:len(applied)-1]
		current = action.before
		operation := "pop"
		if action.isNull {
			operation = "pop_null"
		} else {
			accumulatorDepth--
		}
		rows = append(rows, expectedIncrementalRow{
			caseID: sequence.ID, operation: operation, action: action.text,
			logicalDepth: len(applied), accumulatorDepth: accumulatorDepth,
			moveType: action.moveType, dirty: action.dirty, requiresRefresh: action.refresh,
			updateKind: [2]string{"reuse", "reuse"}, position: current,
		})
	}
	for index := range rows {
		rows[index].sequenceIndex = index
	}
	return rows, nil
}

func applyIncrementalAction(before base.Position, action string) (base.Position, base.Delta, *incrementalDirty, string, [2]bool, error) {
	var zeroPosition base.Position
	var zeroDelta base.Delta
	var zeroRefresh [2]bool
	if len(action) != 4 && len(action) != 5 {
		return zeroPosition, zeroDelta, nil, "", zeroRefresh, fmt.Errorf("invalid UCI action %q", action)
	}
	from, err := incrementalSquare(action[:2])
	if err != nil {
		return zeroPosition, zeroDelta, nil, "", zeroRefresh, err
	}
	to, err := incrementalSquare(action[2:4])
	if err != nil {
		return zeroPosition, zeroDelta, nil, "", zeroRefresh, err
	}
	board, beforeFacts, err := contextBoardAndFacts(before)
	if err != nil {
		return zeroPosition, zeroDelta, nil, "", zeroRefresh, err
	}
	moverEntry := board[from]
	if !moverEntry.set || moverEntry.color != before.SideToMove || board[to].set && board[to].color == moverEntry.color {
		return zeroPosition, zeroDelta, nil, "", zeroRefresh, fmt.Errorf("invalid mover/destination")
	}
	mover := base.PieceOnSquare{Piece: moverEntry.piece, Color: moverEntry.color, Square: from}
	dirty := &incrementalDirty{Mover: mover}
	removed := []base.PieceOnSquare{mover}
	added := make([]base.PieceOnSquare, 0, 2)
	kind := base.MoveNormal
	moveType := "normal"

	fileDistance := int(to&7) - int(from&7)
	if mover.Piece == base.King && (fileDistance == 2 || fileDistance == -2) && from/8 == to/8 {
		kind = base.MoveCastle
		moveType = "castling"
		rookFrom := base.Square(int(from/8)*8 + 7)
		rookTo := base.Square(int(from/8)*8 + 5)
		if fileDistance < 0 {
			rookFrom = base.Square(int(from/8) * 8)
			rookTo = base.Square(int(from/8)*8 + 3)
		}
		rookEntry := board[rookFrom]
		if !rookEntry.set || rookEntry.piece != base.Rook || rookEntry.color != mover.Color {
			return zeroPosition, zeroDelta, nil, "", zeroRefresh, fmt.Errorf("castling rook is absent")
		}
		rookBefore := base.PieceOnSquare{Piece: base.Rook, Color: mover.Color, Square: rookFrom}
		rookAfter := base.PieceOnSquare{Piece: base.Rook, Color: mover.Color, Square: rookTo}
		removed = append(removed, rookBefore)
		added = append(added, base.PieceOnSquare{Piece: base.King, Color: mover.Color, Square: to}, rookAfter)
		dirty.To = incrementalSquarePointer(to)
		dirty.Removed = incrementalPiecePointer(rookBefore)
		dirty.Added = incrementalPiecePointer(rookAfter)
		board[from] = boardPiece{}
		board[rookFrom] = boardPiece{}
		board[to] = boardPiece{piece: base.King, color: mover.Color, set: true}
		board[rookTo] = boardPiece{piece: base.Rook, color: mover.Color, set: true}
	} else {
		var victim *base.PieceOnSquare
		if board[to].set {
			piece := base.PieceOnSquare{Piece: board[to].piece, Color: board[to].color, Square: to}
			victim = &piece
		} else if mover.Piece == base.Pawn && from&7 != to&7 {
			captureSquare := base.Square(int(to) - 8)
			if mover.Color == base.Black {
				captureSquare = base.Square(int(to) + 8)
			}
			entry := board[captureSquare]
			if !entry.set || entry.piece != base.Pawn || entry.color == mover.Color {
				return zeroPosition, zeroDelta, nil, "", zeroRefresh, fmt.Errorf("en-passant victim is absent")
			}
			piece := base.PieceOnSquare{Piece: base.Pawn, Color: entry.color, Square: captureSquare}
			victim = &piece
			kind = base.MoveEnPassant
			moveType = "en-passant"
			board[captureSquare] = boardPiece{}
		}
		if victim != nil {
			if victim.Piece == base.King {
				return zeroPosition, zeroDelta, nil, "", zeroRefresh, fmt.Errorf("captured king")
			}
			removed = append(removed, *victim)
			dirty.Removed = incrementalPiecePointer(*victim)
			if kind == base.MoveNormal {
				kind = base.MoveCapture
			}
		}
		board[from] = boardPiece{}
		board[to] = boardPiece{piece: mover.Piece, color: mover.Color, set: true}
		if len(action) == 5 {
			if mover.Piece != base.Pawn || to/8 != base.Square(7*(1-mover.Color)) {
				return zeroPosition, zeroDelta, nil, "", zeroRefresh, fmt.Errorf("invalid promotion")
			}
			promotion, ok := incrementalPromotion(action[4])
			if !ok {
				return zeroPosition, zeroDelta, nil, "", zeroRefresh, fmt.Errorf("invalid promotion piece")
			}
			promoted := base.PieceOnSquare{Piece: promotion, Color: mover.Color, Square: to}
			board[to] = boardPiece{piece: promotion, color: mover.Color, set: true}
			added = append(added, promoted)
			dirty.Added = incrementalPiecePointer(promoted)
			kind = base.MovePromotion
			if victim != nil {
				kind = base.MovePromotionCapture
			}
			moveType = "promotion"
		} else {
			if mover.Piece == base.Pawn && (to/8 == 0 || to/8 == 7) {
				return zeroPosition, zeroDelta, nil, "", zeroRefresh, fmt.Errorf("promotion suffix absent")
			}
			added = append(added, base.PieceOnSquare{Piece: mover.Piece, Color: mover.Color, Square: to})
			dirty.To = incrementalSquarePointer(to)
		}
	}

	after := incrementalPositionFromBoard(before.SideToMove^1, board)
	_, afterFacts, err := contextBoardAndFacts(after)
	if err != nil {
		return zeroPosition, zeroDelta, nil, "", zeroRefresh, err
	}
	delta := base.Delta{Kind: kind, Before: beforeFacts, After: afterFacts}
	delta.RemovedCount = uint8(len(removed))
	delta.AddedCount = uint8(len(added))
	copy(delta.Removed[:], removed)
	copy(delta.Added[:], added)
	refresh := [2]bool{}
	if mover.Piece == base.King {
		refresh[mover.Color] = true
	}
	return after, delta, dirty, moveType, refresh, nil
}

func replayIncrementalCase(t *testing.T, model *Model, suiteName string, expected []expectedIncrementalRow, rows []incrementalOracleRow) {
	t.Helper()
	if len(expected) != len(rows) {
		t.Fatalf("suite %s case %s row count = %d, want %d", suiteName, expected[0].caseID, len(rows), len(expected))
	}
	full, err := NewContext(model)
	if err != nil {
		t.Fatal(err)
	}
	compact, err := NewContext(model)
	if err != nil {
		t.Fatal(err)
	}
	if err := full.Reset(expected[0].position); err != nil {
		t.Fatal(err)
	}
	if err := compact.Reset(expected[0].position); err != nil {
		t.Fatal(err)
	}
	fullSaved := map[int]frozenContext{0: freezeContext(full)}
	compactSaved := map[int]frozenContext{0: freezeContext(compact)}
	for index := range expected {
		want := expected[index]
		oracle := rows[index]
		if index > 0 {
			switch want.operation {
			case "push":
				if err := full.Push(*want.delta, want.position); err != nil {
					t.Fatalf("%s row %d full Push: %v", want.caseID, index, err)
				}
				if err := compact.PushDelta(*want.delta); err != nil {
					t.Fatalf("%s row %d compact PushDelta: %v", want.caseID, index, err)
				}
				fullSaved[want.logicalDepth] = freezeContext(full)
				compactSaved[want.logicalDepth] = freezeContext(compact)
			case "push_null":
				if err := full.PushNull(want.position); err != nil {
					t.Fatalf("%s row %d full PushNull: %v", want.caseID, index, err)
				}
				if err := compact.PushNull(want.position); err != nil {
					t.Fatalf("%s row %d compact PushNull: %v", want.caseID, index, err)
				}
				fullSaved[want.logicalDepth] = freezeContext(full)
				compactSaved[want.logicalDepth] = freezeContext(compact)
			case "pop", "pop_null":
				if err := full.Pop(); err != nil {
					t.Fatalf("%s row %d full Pop: %v", want.caseID, index, err)
				}
				if err := compact.Pop(); err != nil {
					t.Fatalf("%s row %d compact Pop: %v", want.caseID, index, err)
				}
				if got := freezeContext(full); !reflect.DeepEqual(got, fullSaved[want.logicalDepth]) {
					t.Fatalf("%s row %d full unwind differs from saved state", want.caseID, index)
				}
				if got := freezeContext(compact); !reflect.DeepEqual(got, compactSaved[want.logicalDepth]) {
					t.Fatalf("%s row %d compact unwind differs from saved state", want.caseID, index)
				}
			default:
				t.Fatalf("%s row %d unsupported operation %q", want.caseID, index, want.operation)
			}
		}
		compareIncrementalMetadata(t, want, oracle)
		assertOfficialOracleNoOverflow(t, index, oracle.State)
		compareIncrementalContext(t, model, full, want, oracle.State)
		compareIncrementalContext(t, model, compact, want, oracle.State)
		if got := freezeContext(full); !reflect.DeepEqual(got, freezeContext(compact)) {
			t.Fatalf("%s row %d full/compact contexts differ", want.caseID, index)
		}
	}
	if suiteName == "growth" {
		if cap(full.frames) < 256 || cap(compact.frames) < 256 {
			t.Fatalf("growth capacity = %d/%d, want at least 256", cap(full.frames), cap(compact.frames))
		}
	}
	if full.Depth() != 0 || compact.Depth() != 0 {
		t.Fatalf("%s did not unwind to root", expected[0].caseID)
	}
}

func compareIncrementalMetadata(t *testing.T, want expectedIncrementalRow, got incrementalOracleRow) {
	t.Helper()
	if got.Schema != incrementalRowSchema || got.Case != want.caseID || got.SequenceIndex != want.sequenceIndex ||
		got.Operation != want.operation || got.Action != want.action || got.LogicalDepth != want.logicalDepth ||
		got.AccumulatorDepth != want.accumulatorDepth || !reflect.DeepEqual(got.MoveType, want.moveType) ||
		!reflect.DeepEqual(got.Dirty, want.dirty) || got.RequiresRefresh != want.requiresRefresh ||
		got.UpdateKind != want.updateKind {
		t.Fatalf("%s row %d metadata differs:\n got %+v\nwant %+v", want.caseID, want.sequenceIndex, got, want)
	}
	if got.State.SourceCommit != officialSourceCommit || got.State.NetworkSHA256 != officialNetworkSHA256 {
		t.Fatalf("%s row %d state provenance differs", want.caseID, want.sequenceIndex)
	}
	if want.sequenceIndex == 0 && got.State.FEN != want.rootFEN {
		t.Fatalf("%s root FEN = %q, want %q", want.caseID, got.State.FEN, want.rootFEN)
	}
	if got.State.SideToMove != want.position.SideToMove || !reflect.DeepEqual(got.State.Pieces, want.position.Pieces) {
		t.Fatalf("%s row %d board/side differs from frozen-action replay", want.caseID, want.sequenceIndex)
	}
}

func compareIncrementalContext(t *testing.T, model *Model, context *Context, want expectedIncrementalRow, oracle upstreamOracleCase) {
	t.Helper()
	if context.Depth() != want.logicalDepth {
		t.Fatalf("%s row %d Context depth = %d, want %d", want.caseID, want.sequenceIndex, context.Depth(), want.logicalDepth)
	}
	board, facts, err := contextBoardAndFacts(want.position)
	if err != nil {
		t.Fatal(err)
	}
	if context.board != board {
		t.Fatalf("%s row %d Context board differs", want.caseID, want.sequenceIndex)
	}
	frame := context.frames[len(context.frames)-1]
	if frame.facts != facts || frame.sideToMove != want.position.SideToMove {
		t.Fatalf("%s row %d Context frame facts/side differ", want.caseID, want.sequenceIndex)
	}
	if frame.values != oracle.Accumulator || frame.psqt != oracle.PSQTAccumulator {
		t.Fatalf("%s row %d Context FT/PSQT differs from upstream", want.caseID, want.sequenceIndex)
	}
	public, err := context.EvaluateAll()
	if err != nil {
		t.Fatal(err)
	}
	wantPublic := Trace{CorrectBucket: oracle.CorrectBucket, Buckets: oracle.Components}
	if public != wantPublic {
		t.Fatalf("%s row %d Context EvaluateAll differs from upstream: got %+v, want %+v", want.caseID, want.sequenceIndex, public, wantPublic)
	}
	selected, err := context.EvaluateSelected()
	if err != nil {
		t.Fatal(err)
	}
	wantSelected := SelectedEvaluation{
		Bucket:     oracle.CorrectBucket,
		Components: oracle.Components[oracle.CorrectBucket],
	}
	if selected != wantSelected {
		t.Fatalf("%s row %d Context EvaluateSelected differs from upstream: got %+v, want %+v", want.caseID, want.sequenceIndex, selected, wantSelected)
	}
	fresh, err := model.evaluateAllTrace(want.position)
	if err != nil {
		t.Fatal(err)
	}
	compareUpstreamOracle(t, want.sequenceIndex, fresh, oracle)
	var incremental evaluationTrace
	incremental.accumulator.values = frame.values
	incremental.accumulator.psqt = frame.psqt
	incremental = model.completeEvaluationTrace(incremental, want.position.SideToMove, len(want.position.Pieces))
	if incremental.public.CorrectBucket != oracle.CorrectBucket || incremental.public.Buckets != oracle.Components ||
		incremental.transformed != oracle.Transformed || incremental.psqtRaw != oracle.PSQTRaw ||
		incremental.psqtDifferenceWide != oracle.PSQTDifferenceWide {
		t.Fatalf("%s row %d incremental public/transform output differs", want.caseID, want.sequenceIndex)
	}
	for bucket := 0; bucket < layerStacks; bucket++ {
		got := incremental.stacks[bucket]
		upstream := oracle.Stacks[bucket]
		if got.fc0 != upstream.FC0 || got.squared != upstream.Squared || got.clipped0 != upstream.Clipped0 ||
			got.fc1 != upstream.FC1 || got.clipped1 != upstream.Clipped1 || got.fc2 != upstream.FC2 ||
			got.forward != upstream.Forward || got.positionalRaw != upstream.PositionalRaw ||
			got.fc0WideMin != upstream.FC0WideMin || got.fc0WideMax != upstream.FC0WideMax ||
			got.fc1WideMin != upstream.FC1WideMin || got.fc1WideMax != upstream.FC1WideMax ||
			got.fc2WideMin != upstream.FC2WideMin || got.fc2WideMax != upstream.FC2WideMax ||
			got.forwardWide != upstream.ForwardWide || got.positionalWide != upstream.PositionalWide {
			t.Fatalf("%s row %d incremental stack %d differs", want.caseID, want.sequenceIndex, bucket)
		}
	}
}

func incrementalSquare(text string) (base.Square, error) {
	if len(text) != 2 || text[0] < 'a' || text[0] > 'h' || text[1] < '1' || text[1] > '8' {
		return 0, fmt.Errorf("invalid square %q", text)
	}
	return base.Square(int(text[0]-'a') + 8*int(text[1]-'1')), nil
}

func incrementalPromotion(symbol byte) (base.PieceType, bool) {
	switch symbol {
	case 'n':
		return base.Knight, true
	case 'b':
		return base.Bishop, true
	case 'r':
		return base.Rook, true
	case 'q':
		return base.Queen, true
	default:
		return 0, false
	}
}

func incrementalPositionFromBoard(side base.Color, board [64]boardPiece) base.Position {
	position := base.Position{SideToMove: side}
	for square, entry := range board {
		if entry.set {
			position.Pieces = append(position.Pieces, base.PieceOnSquare{
				Piece: entry.piece, Color: entry.color, Square: base.Square(square),
			})
		}
	}
	return position
}

func cloneIncrementalPosition(position base.Position) base.Position {
	return base.Position{SideToMove: position.SideToMove, Pieces: append([]base.PieceOnSquare(nil), position.Pieces...)}
}

func incrementalSquarePointer(square base.Square) *base.Square {
	value := square
	return &value
}

func incrementalPiecePointer(piece base.PieceOnSquare) *base.PieceOnSquare {
	value := piece
	return &value
}

func TestIncrementalOracleEnvelopeRejectsMalformedShape(t *testing.T) {
	fen := "7k/8/8/8/8/8/8/K7 w - - 0 1"
	state := oracleJSONForTest(t, fen)
	valid := map[string]any{
		"schema": incrementalRowSchema, "case": "shape", "sequence_index": 0,
		"operation": "root", "action": "", "logical_depth": 0,
		"accumulator_depth": 0, "move_type": nil, "dirty": nil,
		"requires_refresh": []bool{false, false},
		"update_kind":      []string{"refresh", "refresh"},
		"state":            json.RawMessage(state),
	}
	marshal := func(value map[string]any) []byte {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	if _, err := decodeIncrementalOracleRow(marshal(valid)); err != nil {
		t.Fatalf("valid incremental envelope: %v", err)
	}
	mutate := func(change func(map[string]any)) []byte {
		copy := make(map[string]any, len(valid))
		for key, value := range valid {
			copy[key] = value
		}
		change(copy)
		return marshal(copy)
	}
	for _, test := range []struct {
		name   string
		change func(map[string]any)
	}{
		{"missing field", func(row map[string]any) { delete(row, "sequence_index") }},
		{"null zero scalar", func(row map[string]any) { row["sequence_index"] = nil }},
		{"short branch array", func(row map[string]any) { row["update_kind"] = []string{"refresh"} }},
		{"unknown field", func(row map[string]any) { row["extra"] = 1 }},
		{"dirty missing field", func(row map[string]any) {
			row["dirty"] = map[string]any{
				"mover": map[string]any{"Piece": 5, "Color": 0, "Square": 0},
				"to":    1, "removed": nil,
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := decodeIncrementalOracleRow(mutate(test.change)); err == nil {
				t.Fatal("malformed incremental oracle envelope was accepted")
			}
		})
	}
}
