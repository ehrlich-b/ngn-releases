package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ehrlich-b/ngn/engine"
)

func TestRecordFixtureCapturesColdWarmCallbacksAndFullHistory(t *testing.T) {
	f := fixture{
		Name:            "repetition-history",
		Purpose:         "exercise a real played-position repetition map",
		StartFEN:        "4k3/8/8/8/8/8/8/R3K3 w - - 0 1",
		Moves:           "a1a2 e8e7 a2a1 e7e8 a1a2 e8e7 a2a1 e7e8",
		ExpectedRootFEN: "4k3/8/8/8/8/8/8/R3K3 w - - 8 1",
		Depth:           1,
	}
	got, err := recordFixture(1, f)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Passes) != 2 || got.Passes[0].State != "cold" || got.Passes[1].State != "warm" {
		t.Fatalf("passes = %#v", got.Passes)
	}
	if got.Root.PlayedPlies != 8 || len(got.Root.History) < 4 || got.Root.LastMove != "e7e8" {
		t.Fatalf("root history was not retained: %#v", got.Root)
	}
	rootCount := 0
	for _, entry := range got.Root.History {
		if entry.Hash == got.Root.Hash {
			rootCount = entry.Count
		}
	}
	if rootCount != 3 {
		t.Fatalf("root repetition count = %d, want 3", rootCount)
	}
	for _, pass := range got.Passes {
		if len(pass.Iterations) != 1 {
			t.Fatalf("%s callbacks = %d, want 1", pass.State, len(pass.Iterations))
		}
		if pass.Iterations[0].BestMove == "0000" || pass.Iterations[0].Nodes == 0 {
			t.Fatalf("%s callback lacks move/nodes: %#v", pass.State, pass.Iterations[0])
		}
		if pass.Iterations[0].Diagnostics == nil {
			t.Fatalf("%s callback lacks deterministic counters", pass.State)
		}
	}
}

func TestBoardDigestIncludesIncrementalAndMailboxState(t *testing.T) {
	pos, err := engine.ParseFEN("4k3/8/8/8/8/8/4P3/4K3 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	before := boardDigest(pos.Board)
	pos.Board.UpdateSquare(engine.E2, engine.NoPiece, engine.WhitePawn)
	after := boardDigest(pos.Board)
	if before == after {
		t.Fatal("board digest ignored a board/mailbox/accumulator update")
	}
}

func TestComparatorReportsIdentityWithoutFailingBehavior(t *testing.T) {
	base := minimalSnapshot()
	candidate := base
	candidate.Identity.RequestedSource = "candidate"
	got, err := compareSnapshots(base, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal || len(got.MetadataDifferences) != 1 || len(got.BehaviorDifferences) != 0 {
		t.Fatalf("comparison = %#v", got)
	}
}

func TestComparatorDetectsDeterministicSearchAndHistoryChanges(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*snapshot)
		path string
	}{
		{"move", func(s *snapshot) { s.Fixtures[0].Passes[0].Final.BestMove = "a1a3" }, "best_move"},
		{"score", func(s *snapshot) { s.Fixtures[0].Passes[0].Final.Score++ }, "score"},
		{"nodes", func(s *snapshot) { s.Fixtures[0].Passes[0].Final.Nodes++ }, "nodes"},
		{"pv", func(s *snapshot) { s.Fixtures[0].Passes[0].Final.PV = []string{"a1a3"} }, "pv"},
		{"callback", func(s *snapshot) { s.Fixtures[0].Passes[0].Iterations[0].SelDepth++ }, "seldepth"},
		{"history", func(s *snapshot) { s.Fixtures[0].Root.History[0].Count++ }, "history"},
		{"option", func(s *snapshot) { s.Options.HashMB++ }, "hash_mb"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := minimalSnapshot()
			candidate := cloneSnapshot(t, base)
			tc.edit(&candidate)
			got, err := compareSnapshots(base, candidate)
			if err != nil {
				t.Fatal(err)
			}
			if got.Equal || len(got.BehaviorDifferences) == 0 {
				t.Fatalf("change was not detected: %#v", got)
			}
			found := false
			for _, diff := range got.BehaviorDifferences {
				if strings.Contains(diff.Path, tc.path) {
					found = true
				}
			}
			if !found {
				t.Fatalf("no %q path in %#v", tc.path, got.BehaviorDifferences)
			}
		})
	}
}

func minimalSnapshot() snapshot {
	obs := observation{Depth: 1, SelDepth: 1, RootDepth: 1, BestMove: "a1a2", Score: 1, Nodes: 2, PV: []string{"a1a2"}, Diagnostics: map[string]json.RawMessage{"QNodes": json.RawMessage("1")}}
	return snapshot{
		Schema:   snapshotSchema,
		Identity: buildIdentity{RequestedSource: "baseline"},
		Options:  optionIdentity{HashMB: 1},
		Fixtures: []fixtureSnapshot{{
			Name: "one", Root: rootState{Hash: "01", History: []historyCount{{Hash: "01", Count: 1}}},
			Passes: []passSnapshot{{State: "cold", Iterations: []observation{obs}, Final: obs}},
		}},
	}
}

func cloneSnapshot(t *testing.T, in snapshot) snapshot {
	t.Helper()
	encoded, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out snapshot
	if err := json.Unmarshal(encoded, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
