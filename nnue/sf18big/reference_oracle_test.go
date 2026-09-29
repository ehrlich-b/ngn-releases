package sf18big

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sort"
	"testing"

	base "github.com/ehrlich-b/ngn/nnue"
)

const (
	referenceCoreOracleEnvironment   = "NGN_SF18_BIG_REFERENCE_CORE_JSONL"
	referenceGrowthOracleEnvironment = "NGN_SF18_BIG_REFERENCE_GROWTH_JSONL"
	referenceGameOracleEnvironment   = "NGN_SF18_BIG_REFERENCE_GAME_JSONL"
	referenceOracleSchema            = "sf18-big-reference-oracle/v1"
	referenceSourceCommit            = "cb3d4ee9b47d0c5aae855b12379378ea1439675c"
)

type referenceOracleTransition struct {
	Removed         [2][]uint32 `json:"removed"`
	Added           [2][]uint32 `json:"added"`
	RequiresRefresh [2]bool     `json:"requires_refresh"`
}

type referenceOracleState struct {
	SourceCommit      string                     `json:"source_commit"`
	NetworkSHA256     string                     `json:"network_sha256"`
	FEN               string                     `json:"fen"`
	SideToMove        base.Color                 `json:"side_to_move"`
	Pieces            []base.PieceOnSquare       `json:"pieces"`
	Bucket            uint8                      `json:"bucket"`
	ActiveThreats     [2][]uint32                `json:"active_threats"`
	BaseAccumulator   [2][transformerLanes]int16 `json:"base_accumulator"`
	ThreatAccumulator [2][transformerLanes]int16 `json:"threat_accumulator"`
	BasePSQT          [2][psqtBuckets]int32      `json:"base_psqt"`
	ThreatPSQT        [2][psqtBuckets]int32      `json:"threat_psqt"`
	Transformed       [transformerLanes]uint8    `json:"transformed"`
	PSQTRaw           int32                      `json:"psqt_raw"`
	PositionalRaw     int32                      `json:"positional_raw"`
	Components        Components                 `json:"components"`
}

type referenceOracleRow struct {
	Schema        string                     `json:"schema"`
	Case          string                     `json:"case"`
	SequenceIndex int                        `json:"sequence_index"`
	Operation     string                     `json:"operation"`
	Action        string                     `json:"action"`
	Transition    *referenceOracleTransition `json:"transition"`
	State         referenceOracleState       `json:"state"`
}

func TestOfficialReferenceOracleCore(t *testing.T) {
	runOfficialReferenceOracle(t, referenceCoreOracleEnvironment)
}

func TestOfficialReferenceOracleGrowth(t *testing.T) {
	runOfficialReferenceOracle(t, referenceGrowthOracleEnvironment)
}

func TestOfficialReferenceOracleRepresentativeGames(t *testing.T) {
	runOfficialReferenceOracle(t, referenceGameOracleEnvironment)
}

func runOfficialReferenceOracle(t *testing.T, oracleEnvironment string) {
	t.Helper()
	networkPath := os.Getenv(officialFileEnvironment)
	oraclePath := os.Getenv(oracleEnvironment)
	if networkPath == "" || oraclePath == "" {
		t.Skip("official BIG network and upstream reference oracle JSONL are required for this explicit gate")
	}
	networkFile, err := os.Open(networkPath)
	if err != nil {
		t.Fatal(err)
	}
	model, err := Load(networkFile)
	networkFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	metadata := model.Metadata()
	if digest := hex.EncodeToString(metadata.FileSHA256[:]); digest != officialFileSHA256 {
		t.Fatalf("loaded model digest = %s, want %s", digest, officialFileSHA256)
	}

	oracleFile, err := os.Open(oraclePath)
	if err != nil {
		t.Fatal(err)
	}
	defer oracleFile.Close()
	scanner := bufio.NewScanner(oracleFile)
	scanner.Buffer(make([]byte, 64<<10), 2<<20)
	var previousCase string
	var previousPosition base.Position
	var previousState ReferenceState
	var context *Context
	var sparseContext *Context
	var casePositions []base.Position
	var caseStates []ReferenceState
	rows := 0
	for scanner.Scan() {
		var row referenceOracleRow
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatalf("row %d: %v", rows, err)
		}
		if row.Schema != referenceOracleSchema || row.Case == "" ||
			row.State.SourceCommit != referenceSourceCommit ||
			row.State.NetworkSHA256 != officialFileSHA256 {
			t.Fatalf("row %d has invalid identity", rows)
		}
		position := base.Position{SideToMove: row.State.SideToMove, Pieces: row.State.Pieces}
		state, occupied, err := model.ReferenceRefresh(position)
		if err != nil {
			t.Fatalf("row %d refresh: %v", rows, err)
		}
		if occupied == 0 || row.State.Bucket != uint8((occupied-1)/4) {
			t.Fatalf("row %d bucket = %d for %d pieces", rows, row.State.Bucket, occupied)
		}
		if state.BaseValues != row.State.BaseAccumulator ||
			state.ThreatValues != row.State.ThreatAccumulator ||
			state.BasePSQT != row.State.BasePSQT || state.ThreatPSQT != row.State.ThreatPSQT {
			t.Fatalf("row %d accumulator or PSQT differs from upstream", rows)
		}
		for perspective := base.White; perspective <= base.Black; perspective++ {
			want := append([]uint32(nil), row.State.ActiveThreats[perspective]...)
			sort.Slice(want, func(i, j int) bool { return want[i] < want[j] })
			got := state.Threats[perspective].Indices[:state.Threats[perspective].Count]
			if !slices.Equal(got, want) {
				t.Fatalf("row %d perspective %d active threats differ: got %v, want %v", rows, perspective, got, want)
			}
		}
		transformed, psqtRaw := transformReferenceState(&state, position.SideToMove, int(row.State.Bucket))
		if transformed != row.State.Transformed || psqtRaw != row.State.PSQTRaw {
			t.Fatalf("row %d transformed input or PSQT raw differs", rows)
		}
		positionalRaw := model.propagateSelectedStack(int(row.State.Bucket), &transformed)
		if positionalRaw != row.State.PositionalRaw {
			t.Fatalf("row %d positional raw = %d, want %d", rows, positionalRaw, row.State.PositionalRaw)
		}
		selected := model.evaluateSelectedState(&state, position.SideToMove, occupied)
		if selected.Bucket != row.State.Bucket || selected.Components != row.State.Components {
			t.Fatalf("row %d selected = %+v, want bucket %d %+v", rows, selected, row.State.Bucket, row.State.Components)
		}

		if row.Case != previousCase {
			if sparseContext != nil {
				requireSparseContextState(t, sparseContext, previousState, previousPosition, "case tail")
				requireContextUnwind(t, context, caseStates, casePositions)
			}
			casePositions = casePositions[:0]
			caseStates = caseStates[:0]
			if row.SequenceIndex != 0 || row.Operation != "root" || row.Transition != nil {
				t.Fatalf("row %d does not start case %q with a root", rows, row.Case)
			}
			context, err = NewContext(model)
			if err != nil {
				t.Fatal(err)
			}
			sparseContext, err = NewContext(model)
			if err != nil {
				t.Fatal(err)
			}
			if err := context.Reset(position); err != nil {
				t.Fatalf("row %d context reset: %v", rows, err)
			}
			if err := sparseContext.Reset(position); err != nil {
				t.Fatalf("row %d sparse context reset: %v", rows, err)
			}
		} else {
			if row.Transition == nil {
				t.Fatalf("row %d misses transition", rows)
			}
			for perspective := base.White; perspective <= base.Black; perspective++ {
				diff, err := DiffThreats(previousPosition, position, perspective)
				if err != nil {
					t.Fatalf("row %d perspective %d diff: %v", rows, perspective, err)
				}
				wantRemoved := row.Transition.Removed[perspective]
				wantAdded := row.Transition.Added[perspective]
				gotRemoved := diff.Removed.Indices[:diff.Removed.Count]
				gotAdded := diff.Added.Indices[:diff.Added.Count]
				if !slices.Equal(gotRemoved, wantRemoved) ||
					!slices.Equal(gotAdded, wantAdded) ||
					diff.RequiresRefresh != row.Transition.RequiresRefresh[perspective] {
					t.Fatalf("row %d perspective %d transition differs", rows, perspective)
				}
				if !diff.RequiresRefresh {
					updated := previousState
					if err := model.ApplyThreatDiff(&updated, perspective, diff); err != nil {
						t.Fatalf("row %d perspective %d apply: %v", rows, perspective, err)
					}
					p := int(perspective)
					if updated.ThreatValues[p] != state.ThreatValues[p] ||
						updated.ThreatPSQT[p] != state.ThreatPSQT[p] ||
						updated.Threats[p] != state.Threats[p] {
						t.Fatalf("row %d perspective %d applied threat diff differs from refresh", rows, perspective)
					}
				}
			}
			switch row.Operation {
			case "push":
				delta, err := oracleContextDelta(previousPosition, position, row.Action)
				if err != nil {
					t.Fatalf("row %d delta: %v", rows, err)
				}
				if err := context.Push(delta, position); err != nil {
					t.Fatalf("row %d context push: %v", rows, err)
				}
				if err := sparseContext.PushDelta(delta); err != nil {
					t.Fatalf("row %d sparse context push: %v", rows, err)
				}
			case "push_null":
				if err := context.PushNull(position); err != nil {
					t.Fatalf("row %d context null: %v", rows, err)
				}
				facts, err := contextBoardFacts(position)
				if err != nil {
					t.Fatalf("row %d null facts: %v", rows, err)
				}
				if err := sparseContext.PushNullFacts(facts); err != nil {
					t.Fatalf("row %d sparse context null: %v", rows, err)
				}
			default:
				t.Fatalf("row %d unexpected operation %q", rows, row.Operation)
			}
			frame := &context.frames[context.depth]
			if frame.baseComputed != [2]bool{} || frame.threatComputed != [2]bool{} {
				t.Fatalf("row %d push eagerly computed accumulator state", rows)
			}
		}

		contextSelected, err := context.EvaluateSelected()
		if err != nil {
			t.Fatalf("row %d context evaluate: %v", rows, err)
		}
		if contextSelected != selected || context.frames[context.depth].state != state {
			t.Fatalf("row %d demand context differs from full refresh", rows)
		}
		if row.SequenceIndex%8 == 0 {
			requireSparseContextState(t, sparseContext, state, position, "periodic donor")
		}
		previousCase = row.Case
		previousPosition = position
		previousState = state
		casePositions = append(casePositions, position)
		caseStates = append(caseStates, state)
		rows++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if rows == 0 {
		t.Fatal("oracle is empty")
	}
	requireSparseContextState(t, sparseContext, previousState, previousPosition, "final case tail")
	requireContextUnwind(t, context, caseStates, casePositions)
	if oracleEnvironment == referenceGameOracleEnvironment {
		sparseStats, err := sparseContext.Stats()
		if err != nil {
			t.Fatal(err)
		}
		incrementalStats, err := context.Stats()
		if err != nil {
			t.Fatal(err)
		}
		if sparseStats.AdaptiveFullRefreshes == 0 ||
			incrementalStats.BaseIncrementalFrames == [2]uint64{} ||
			incrementalStats.ThreatIncrementalFrames == [2]uint64{} {
			t.Fatalf("representative contexts did not exercise both donor policies: sparse=%+v incremental=%+v", sparseStats, incrementalStats)
		}
	}
}

func requireContextUnwind(
	t *testing.T,
	context *Context,
	states []ReferenceState,
	positions []base.Position,
) {
	t.Helper()
	if context == nil || len(states) != len(positions) {
		t.Fatal("invalid unwind fixture")
	}
	for index := len(states) - 2; index >= 0; index-- {
		if err := context.Pop(); err != nil {
			t.Fatalf("unwind row %d: %v", index, err)
		}
		got, err := context.EvaluateSelected()
		if err != nil {
			t.Fatalf("unwind row %d evaluate: %v", index, err)
		}
		want := context.model.evaluateSelectedState(&states[index], positions[index].SideToMove, len(positions[index].Pieces))
		if got != want || context.frames[context.depth].state != states[index] {
			t.Fatalf("unwind row %d differs from full refresh", index)
		}
	}
	if context.Depth() != 0 {
		t.Fatalf("unwind depth = %d, want 0", context.Depth())
	}
}

func requireSparseContextState(
	t *testing.T,
	context *Context,
	want ReferenceState,
	position base.Position,
	label string,
) {
	t.Helper()
	if context == nil {
		return
	}
	got, err := context.EvaluateSelected()
	if err != nil {
		t.Fatalf("%s evaluate: %v", label, err)
	}
	wantSelected := context.model.evaluateSelectedState(&want, position.SideToMove, len(position.Pieces))
	if got != wantSelected || context.frames[context.depth].state != want {
		t.Fatalf("%s demand context differs from full refresh", label)
	}
}

func contextBoardFacts(position base.Position) (base.PositionFacts, error) {
	_, facts, err := contextBoardAndFacts(position)
	return facts, err
}

func oracleContextDelta(before, after base.Position, action string) (base.Delta, error) {
	if len(action) != 4 && len(action) != 5 {
		return base.Delta{}, fmt.Errorf("invalid UCI action %q", action)
	}
	from, err := oracleSquare(action[:2])
	if err != nil {
		return base.Delta{}, err
	}
	to, err := oracleSquare(action[2:4])
	if err != nil {
		return base.Delta{}, err
	}
	beforeBoard, beforeFacts, err := contextBoardAndFacts(before)
	if err != nil {
		return base.Delta{}, err
	}
	afterBoard, afterFacts, err := contextBoardAndFacts(after)
	if err != nil {
		return base.Delta{}, err
	}
	moverEntry := beforeBoard[from]
	if !moverEntry.set || moverEntry.color != before.SideToMove {
		return base.Delta{}, fmt.Errorf("action %q has no mover", action)
	}
	moverBefore := base.PieceOnSquare{Piece: moverEntry.piece, Color: moverEntry.color, Square: from}
	moverAfterEntry := afterBoard[to]
	if !moverAfterEntry.set || moverAfterEntry.color != moverEntry.color {
		return base.Delta{}, fmt.Errorf("action %q has no mover at destination", action)
	}
	moverAfter := base.PieceOnSquare{Piece: moverAfterEntry.piece, Color: moverAfterEntry.color, Square: to}

	removed := []base.PieceOnSquare{moverBefore}
	added := []base.PieceOnSquare{moverAfter}
	for square, entry := range beforeBoard {
		if !entry.set || base.Square(square) == from || entry == afterBoard[square] {
			continue
		}
		removed = append(removed, base.PieceOnSquare{Piece: entry.piece, Color: entry.color, Square: base.Square(square)})
	}
	for square, entry := range afterBoard {
		if !entry.set || base.Square(square) == to || entry == beforeBoard[square] {
			continue
		}
		added = append(added, base.PieceOnSquare{Piece: entry.piece, Color: entry.color, Square: base.Square(square)})
	}
	if len(removed) > base.MaxDeltaPieces || len(added) > base.MaxDeltaPieces {
		return base.Delta{}, fmt.Errorf("action %q has %d removed and %d added pieces", action, len(removed), len(added))
	}

	kind := base.MoveNormal
	fileDistance := int(to&7) - int(from&7)
	switch {
	case moverEntry.piece == base.King && (fileDistance == 2 || fileDistance == -2):
		kind = base.MoveCastle
	case moverEntry.piece == base.Pawn && moverAfterEntry.piece != base.Pawn && len(removed) == 2:
		kind = base.MovePromotionCapture
	case moverEntry.piece == base.Pawn && moverAfterEntry.piece != base.Pawn:
		kind = base.MovePromotion
	case len(removed) == 2 && removed[1].Square != to:
		kind = base.MoveEnPassant
	case len(removed) == 2:
		kind = base.MoveCapture
	}
	delta := base.Delta{
		Kind:         kind,
		RemovedCount: uint8(len(removed)),
		AddedCount:   uint8(len(added)),
		Before:       beforeFacts,
		After:        afterFacts,
	}
	copy(delta.Removed[:], removed)
	copy(delta.Added[:], added)
	return delta, nil
}

func oracleSquare(value string) (base.Square, error) {
	if len(value) != 2 || value[0] < 'a' || value[0] > 'h' || value[1] < '1' || value[1] > '8' {
		return 0, fmt.Errorf("invalid square %q", value)
	}
	return base.Square(value[0]-'a') + 8*base.Square(value[1]-'1'), nil
}
