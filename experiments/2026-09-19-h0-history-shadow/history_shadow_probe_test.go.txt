//go:build h0historyshadow

package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"testing"

	"github.com/ehrlich-b/ngn/rodentv12eval"
)

const h0ProbeStartFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

type h0ProbeInput struct {
	Schema           string `json:"schema"`
	SourcePGN        string `json:"source_pgn"`
	SourcePGNSHA256  string `json:"source_pgn_sha256"`
	ResultsExcluded  bool   `json:"results_excluded"`
	TargetAfterPlies []int  `json:"target_after_plies"`
	Records          []struct {
		GameIndex         int      `json:"game_index"`
		FullGamePlyCount  int      `json:"full_game_ply_count"`
		MovesThroughPly64 []string `json:"moves_through_ply_64"`
		MovesSHA256       string   `json:"moves_sha256"`
	} `json:"records"`
}

type h0ProbeTrajectory struct {
	GameIndex int      `json:"game_index"`
	AfterPly  int      `json:"after_ply"`
	Nodes     uint64   `json:"nodes"`
	Depth     int      `json:"depth"`
	SelDepth  int      `json:"seldepth"`
	RootDepth int      `json:"root_depth"`
	BestMove  string   `json:"best_move"`
	BestScore int      `json:"best_score"`
	Stopped   bool     `json:"stopped"`
	PV        []string `json:"pv"`
}

type h0ProbeRecord struct {
	Trajectory  h0ProbeTrajectory        `json:"trajectory"`
	Diagnostics *H0HistoryShadowSnapshot `json:"diagnostics,omitempty"`
}

type h0ProbeOutput struct {
	Schema             string          `json:"schema"`
	SourceCommit       string          `json:"source_commit"`
	PrefixPath         string          `json:"prefix_path"`
	PrefixSHA256       string          `json:"prefix_sha256"`
	SourcePGNSHA256    string          `json:"source_pgn_sha256"`
	ModelPath          string          `json:"model_path"`
	ModelSHA256        string          `json:"model_sha256"`
	NodesPerSearch     uint64          `json:"nodes_per_search"`
	Threads            int             `json:"threads"`
	HashMB             int             `json:"hash_mb"`
	TrajectoryIdentity bool            `json:"trajectory_identity"`
	Baseline           []h0ProbeRecord `json:"baseline"`
	Observer           []h0ProbeRecord `json:"observer"`
}

func h0ProbeFileSHA256(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func h0ProbeTrajectoryFromInfo(gameIndex, afterPly int, info *SearchInfo) h0ProbeTrajectory {
	pvLength := info.PVLength
	if pvLength > len(info.PV) {
		pvLength = len(info.PV)
	}
	pv := make([]string, 0, pvLength)
	for _, move := range info.PV[:pvLength] {
		if move == EmptyMove {
			break
		}
		pv = append(pv, move.ToString())
	}
	return h0ProbeTrajectory{
		GameIndex: gameIndex,
		AfterPly:  afterPly,
		Nodes:     info.Nodes,
		Depth:     info.Depth,
		SelDepth:  info.SelDepth,
		RootDepth: info.RootDepth,
		BestMove:  info.BestMove.ToString(),
		BestScore: info.BestScore,
		Stopped:   info.Stopped,
		PV:        pv,
	}
}

func h0RunProbePass(t *testing.T, model *rodentv12eval.Model, input h0ProbeInput, nodes uint64, observer bool) []h0ProbeRecord {
	t.Helper()
	const hashMB = 128
	searcher, err := NewSearchEngineWithHash(hashMB)
	if err != nil {
		t.Fatal(err)
	}
	if err := searcher.ConfigureThreads(1); err != nil {
		t.Fatal(err)
	}
	if err := searcher.SelectRodentV12DefaultEvaluator(model); err != nil {
		t.Fatal(err)
	}
	targets := make(map[int]bool, len(input.TargetAfterPlies))
	for _, ply := range input.TargetAfterPlies {
		targets[ply] = true
	}
	records := make([]h0ProbeRecord, 0, len(input.Records)*len(input.TargetAfterPlies))
	for _, game := range input.Records {
		searcher.NewGame()
		// The first NewGame prepares the newly selected evaluator and may replace
		// workerHistory as part of that identity transition. Attach only afterward;
		// later NewGame calls retain the pointer while clearing both live and shadow
		// tables.
		if observer && searcher.worker.history.h0Shadow == nil {
			searcher.worker.history.enableH0HistoryShadow()
		}
		pos, err := ParseFEN(h0ProbeStartFEN)
		if err != nil {
			t.Fatal(err)
		}
		for index, notation := range game.MovesThroughPly64 {
			move, err := ParseUCIMove(pos, notation)
			if err != nil {
				t.Fatalf("game %d ply %d parse %s: %v", game.GameIndex, index+1, notation, err)
			}
			if _, _, _, ok := pos.GameMakeMove(move); !ok {
				t.Fatalf("game %d ply %d rejected %s", game.GameIndex, index+1, notation)
			}
			ply := index + 1
			if !targets[ply] {
				continue
			}
			searcher.SetLastMovePlayed(move)
			searcher.SetMaxNodes(nodes)
			info := searcher.Search(pos, MaximumDepth)
			searcher.SetMaxNodes(0)
			record := h0ProbeRecord{Trajectory: h0ProbeTrajectoryFromInfo(game.GameIndex, ply, info)}
			if observer {
				snapshot := searcher.worker.history.h0HistoryShadowSnapshotAndResetCounters()
				record.Diagnostics = &snapshot
			}
			records = append(records, record)
		}
	}
	return records
}

func TestH0HistoryShadowProbe(t *testing.T) {
	prefixPath := os.Getenv("NGN_H0_PREFIXES")
	modelPath := os.Getenv("RODENT_V12_DEFAULT_MODEL")
	outputPath := os.Getenv("NGN_H0_OUTPUT")
	if prefixPath == "" || modelPath == "" || outputPath == "" {
		t.Fatal("NGN_H0_PREFIXES, RODENT_V12_DEFAULT_MODEL, and NGN_H0_OUTPUT are required")
	}
	nodes := uint64(200000)
	if text := os.Getenv("NGN_H0_NODES"); text != "" {
		parsed, err := strconv.ParseUint(text, 10, 64)
		if err != nil || parsed == 0 {
			t.Fatalf("invalid NGN_H0_NODES %q", text)
		}
		nodes = parsed
	}
	prefixData, err := os.ReadFile(prefixPath)
	if err != nil {
		t.Fatal(err)
	}
	var input h0ProbeInput
	if err := json.Unmarshal(prefixData, &input); err != nil {
		t.Fatal(err)
	}
	if input.Schema != "ngn-h0-history-shadow-prefixes-v1" || !input.ResultsExcluded || len(input.Records) != 8 || !reflect.DeepEqual(input.TargetAfterPlies, []int{16, 32, 48, 64}) {
		t.Fatalf("unexpected frozen input shape: schema=%q excluded=%v records=%d targets=%v", input.Schema, input.ResultsExcluded, len(input.Records), input.TargetAfterPlies)
	}
	model, err := rodentv12eval.LoadV12Default(modelPath)
	if err != nil {
		t.Fatal(err)
	}

	baseline := h0RunProbePass(t, model, input, nodes, false)
	observer := h0RunProbePass(t, model, input, nodes, true)
	baselineTrajectories := make([]h0ProbeTrajectory, len(baseline))
	observerTrajectories := make([]h0ProbeTrajectory, len(observer))
	for index := range baseline {
		baselineTrajectories[index] = baseline[index].Trajectory
		observerTrajectories[index] = observer[index].Trajectory
	}
	identical := reflect.DeepEqual(baselineTrajectories, observerTrajectories)
	output := h0ProbeOutput{
		Schema:             "ngn-h0-history-shadow-probe-v1",
		SourceCommit:       os.Getenv("NGN_H0_SOURCE_COMMIT"),
		PrefixPath:         prefixPath,
		PrefixSHA256:       h0ProbeFileSHA256(t, prefixPath),
		SourcePGNSHA256:    input.SourcePGNSHA256,
		ModelPath:          modelPath,
		ModelSHA256:        h0ProbeFileSHA256(t, modelPath),
		NodesPerSearch:     nodes,
		Threads:            1,
		HashMB:             128,
		TrajectoryIdentity: identical,
		Baseline:           baseline,
		Observer:           observer,
	}
	encoded, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	encoded = append(encoded, '\n')
	if err := os.WriteFile(outputPath, encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	if !identical {
		t.Fatalf("baseline and observer trajectories differ; receipt written to %s", outputPath)
	}
	if len(observer) != 32 {
		t.Fatalf("observer records=%d want=32", len(observer))
	}
	for index, record := range observer {
		if record.Diagnostics == nil {
			t.Fatalf("observer record %d has no diagnostics", index)
		}
		if record.Diagnostics.Counters.LMRBaselineRecomputeMismatches != 0 {
			t.Fatalf("observer record %d has %d LMR baseline mismatches", index, record.Diagnostics.Counters.LMRBaselineRecomputeMismatches)
		}
	}
	observedNodes := uint64(0)
	for _, record := range observer {
		observedNodes += record.Diagnostics.Counters.CompletedRootAttempts
		observedNodes += record.Diagnostics.Counters.CompletedInteriorNodes
	}
	if observedNodes == 0 {
		t.Fatal("observer recorded no completed nodes")
	}
	fmt.Printf("H0_RESULT %s\n", outputPath)
}
