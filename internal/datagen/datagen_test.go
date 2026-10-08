package datagen

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ehrlich-b/ngn/engine"
	"github.com/ehrlich-b/ngn/internal/ngnp"
)

func board(t *testing.T, fen string) *engine.Position {
	t.Helper()
	pos, err := engine.ParseFEN(fen)
	if err != nil {
		t.Fatal(err)
	}
	return pos
}

func TestAdjudicationThresholdsAndResets(t *testing.T) {
	s := Defaults()
	for _, tc := range []struct {
		score  int
		result ngnp.Result
	}{{2000, ngnp.Win}, {-2000, ngnp.Loss}} {
		var a adjudicator
		for ply := 0; ply < 6; ply++ {
			r, done := a.observe(s, tc.score, ply)
			if done != (ply == 5) || (done && r != tc.result) {
				t.Fatalf("ply %d score %d: %d %v", ply, tc.score, r, done)
			}
		}
	}
	var a adjudicator
	for _, score := range []int{2000, 2000, 2000, 2000, 2000, -2000, 2000, 1999} {
		if _, done := a.observe(s, score, 100); done {
			t.Fatal("sign/threshold reset failed")
		}
	}
	for ply := 0; ply < 92; ply++ {
		r, done := a.observe(s, 8, ply)
		if done != (ply == 91) || (done && r != ngnp.Draw) {
			t.Fatalf("draw boundary ply %d: %d %v", ply, r, done)
		}
	}
	a = adjudicator{}
	for ply := 80; ply < 91; ply++ {
		a.observe(s, -8, ply)
	}
	if _, done := a.observe(s, 9, 91); done {
		t.Fatal("draw threshold reset failed")
	}
	if _, done := a.observe(s, 0, 92); done {
		t.Fatal("draw streak survived reset")
	}
}

func TestWholeGameResultAssignment(t *testing.T) {
	for result := ngnp.Loss; result <= ngnp.Win; result++ {
		records := []ngnp.Record{{Score: 100, Ply: 8}, {Score: -200, Ply: 9}, {Score: 300, Ply: 10}}
		assignResult(records, result)
		for i, r := range records {
			if r.Result != result || r.Ply != uint16(8+i) {
				t.Fatalf("record %d: %+v", i, r)
			}
		}
	}
}

func TestRuleDrawsAndMatePriority(t *testing.T) {
	for _, tc := range []struct {
		fen, reason string
		result      ngnp.Result
	}{
		{"7k/6Q1/5K2/8/8/8/8/8 b - - 100 1", "checkmate", ngnp.Win},
		{"7k/5K2/6Q1/8/8/8/8/8 b - - 0 1", "stalemate", ngnp.Draw},
		{"4k3/8/8/8/8/8/8/R3K3 w - - 100 1", "fifty_move", ngnp.Draw},
		{"4k3/8/8/8/8/8/8/4K3 w - - 0 1", "insufficient_material", ngnp.Draw},
		{"4k3/8/8/8/8/4B3/8/2B1K3 w - - 0 1", "insufficient_material", ngnp.Draw},
	} {
		pos := board(t, tc.fen)
		_, result, reason, done := terminal(pos, 0, 400)
		if !done || result != tc.result || reason != tc.reason {
			t.Fatalf("%s: %d %s %v", tc.fen, result, reason, done)
		}
	}
	pos := board(t, StartFEN)
	for _, name := range []string{"g1f3", "g8f6", "f3g1", "f6g8", "g1f3", "g8f6", "f3g1", "f6g8"} {
		found := false
		for _, move := range engine.GenerateLegalMoves(pos) {
			if move.ToString() == name {
				pos.GameMakeMove(move)
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing legal move %s", name)
		}
	}
	if _, result, reason, done := terminal(pos, 8, 400); !done || result != ngnp.Draw || reason != "repetition" {
		t.Fatalf("threefold: %d %s %v", result, reason, done)
	}
	if _, result, reason, done := terminal(board(t, StartFEN), 40, 40); !done || result != ngnp.Draw || reason != "maxply" {
		t.Fatalf("maxply: %d %s %v", result, reason, done)
	}
	for _, fen := range []string{"4k3/8/8/8/8/8/8/2NNK3 w - - 0 1", "4k3/8/8/8/8/8/8/2BBK3 w - - 0 1"} {
		if insufficient(board(t, fen)) {
			t.Fatalf("material can deliver mate: %s", fen)
		}
	}
}

func TestSearchCapWhitePerspectiveAndRootPreservation(t *testing.T) {
	for _, stm := range []string{"w", "b"} {
		fen := strings.Replace(StartFEN, " w ", " "+stm+" ", 1)
		pos := board(t, fen)
		before := engine.GenerateFEN(pos)
		reference, _ := engine.NewSearchEngineWithHash(1)
		reference.NewGame()
		reference.SetMaxNodes(512)
		info := reference.Search(pos, engine.MaximumDepth)
		e, _ := engine.NewSearchEngineWithHash(1)
		e.NewGame()
		meta := ngnp.Metadata{}
		found, err := search(e, pos, 512, &meta)
		if err != nil {
			t.Fatal(err)
		}
		want := info.BestScore
		if stm == "b" {
			want = -want
		}
		if info.Nodes != 512 || meta.SearchNodes != 512 || found.whiteScore != want || found.move != info.BestMove || engine.GenerateFEN(pos) != before {
			t.Fatalf("search contract/perspective/root changed: %+v expected score %d nodes %d", found, want, meta.SearchNodes)
		}
	}
}

func TestSampleFiltersNoisyPositions(t *testing.T) {
	meta := ngnp.Metadata{Filtered: make(map[string]uint64)}
	pos := board(t, StartFEN)
	quiet := engine.GenerateLegalMoves(pos)[0]
	if r, keep, err := sample(pos, searched{quiet, 123}, 8, &meta); !keep || err != nil || r.Ply != 8 || r.Score != 123 {
		t.Fatalf("quiet: %+v %v %v", r, keep, err)
	}
	for _, found := range []searched{
		{engine.NewMove(engine.A2, engine.B3, engine.WhitePawn, engine.BlackPawn, engine.NoType, engine.Capture), 0},
		{engine.NewMove(engine.A7, engine.A8, engine.WhitePawn, engine.NoPiece, engine.Queen, 0), 0},
		{quiet, engine.MATE_IN_MAX},
	} {
		if _, keep, err := sample(pos, found, 8, &meta); keep || err != nil {
			t.Fatal("noisy sample kept")
		}
	}
	check := board(t, "4k3/8/8/8/8/8/4r3/4K3 w - - 0 1")
	if _, keep, err := sample(check, searched{quiet, 0}, 8, &meta); keep || err != nil {
		t.Fatal("in-check sample kept")
	}
}

func TestCanceledRunClosesCleanly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	path := filepath.Join(t.TempDir(), "canceled.ngnp")
	meta, err := Run(ctx, path, Defaults(), Provenance{strings.Repeat("a", 40), strings.Repeat("b", 64)})
	if err != nil || !meta.Closed || meta.Games != 0 || meta.Positions != 0 || meta.Status != "interrupted" {
		t.Fatalf("%+v %v", meta, err)
	}
	if meta.NetSHA256 != "" {
		t.Fatal("HCE shard claims network provenance")
	}
}

func fixtureNetwork(t *testing.T) (string, []byte) {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "ngnn1", "random_h32.nnue")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return path, raw
}

func TestNetworkSearchMatchesEngineSelection(t *testing.T) {
	path, raw := fixtureNetwork(t)
	s := Defaults()
	s.HashMB, s.NetPath = 1, path
	e, digest, err := newSearcher(s)
	if err != nil {
		t.Fatal(err)
	}
	if digest != fmt.Sprintf("%x", sha256.Sum256(raw)) || e.SelectedEvaluatorBackend() != engine.EvaluatorBackendNGNN1Name {
		t.Fatalf("network not selected/identified: %s %s", e.SelectedEvaluatorBackend(), digest)
	}
	network, err := engine.LoadNGNN1(path)
	if err != nil {
		t.Fatal(err)
	}
	reference, err := engine.NewSearchEngineWithHash(1)
	if err != nil {
		t.Fatal(err)
	}
	if err := reference.SelectNGNN1Evaluator(network); err != nil {
		t.Fatal(err)
	}
	for _, stm := range []string{"w", "b"} {
		pos := board(t, strings.Replace(StartFEN, " w ", " "+stm+" ", 1))
		before := engine.GenerateFEN(pos)
		e.NewGame()
		reference.NewGame()
		reference.SetMaxNodes(512)
		info := reference.Search(pos, engine.MaximumDepth)
		meta := ngnp.Metadata{}
		found, err := search(e, pos, 512, &meta)
		if err != nil {
			t.Fatal(err)
		}
		want := info.BestScore
		if stm == "b" {
			want = -want
		}
		if found.whiteScore != want || found.move != info.BestMove || meta.SearchNodes != info.Nodes || info.EffectiveThreads != 1 || engine.GenerateFEN(pos) != before {
			t.Fatalf("NNUE search/perspective/root mismatch: %+v want %d nodes %d/%d", found, want, meta.SearchNodes, info.Nodes)
		}
	}
}

func TestNetworkRunRecordsSidecar(t *testing.T) {
	netPath, raw := fixtureNetwork(t)
	s := Defaults()
	s.NetPath, s.HashMB, s.Games = netPath, 1, 1
	s.Nodes, s.RandomPlies, s.MaxPly, s.OpeningLimitCP = 512, 0, 12, 30000
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "nnue.ngnp")
	meta, err := Run(ctx, path, s, Provenance{})
	if err != nil {
		t.Fatal(err)
	}
	if !meta.Closed || meta.Status != "complete" || meta.Games != 1 || meta.Positions == 0 {
		t.Fatalf("incomplete NNUE shard: %+v", meta)
	}
	jsonBytes, err := os.ReadFile(path + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var stored ngnp.Metadata
	if err := json.Unmarshal(jsonBytes, &stored); err != nil {
		t.Fatal(err)
	}
	var settings Settings
	if err := json.Unmarshal(stored.Settings, &settings); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("%x", sha256.Sum256(raw))
	if stored.NetSHA256 != want || meta.NetSHA256 != want || settings.NetPath != netPath || stored.SHA256 != meta.SHA256 {
		t.Fatalf("sidecar lost network/shard provenance: %+v", stored)
	}
}

func TestInvalidNetworkCreatesNoShard(t *testing.T) {
	_, raw := fixtureNetwork(t)
	corrupt := append([]byte(nil), raw...)
	corrupt[len(corrupt)-1] ^= 1
	for _, tc := range []struct {
		name string
		data []byte
	}{{"missing", nil}, {"truncated", raw[:20]}, {"checksum", corrupt}, {"trailing", append(append([]byte(nil), raw...), 0)}} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			s := Defaults()
			s.NetPath = filepath.Join(dir, "invalid.nnue")
			if tc.data != nil {
				if err := os.WriteFile(s.NetPath, tc.data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(dir, "rejected.ngnp")
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if _, err := Run(ctx, path, s, Provenance{}); err == nil {
				t.Fatal("invalid network accepted")
			}
			for _, output := range []string{path, path + ".json"} {
				if _, err := os.Stat(output); !os.IsNotExist(err) {
					t.Fatalf("invalid network reserved %s: %v", output, err)
				}
			}
		})
	}
}

type cancelAfterMove struct {
	context.Context
	checks int
}

func (c *cancelAfterMove) Err() error {
	c.checks++
	if c.checks > 1 {
		return context.Canceled
	}
	return nil
}

func TestCancellationDropsBufferedGame(t *testing.T) {
	e, _ := engine.NewSearchEngineWithHash(1)
	e.NewGame()
	s := Defaults()
	s.Nodes = 512
	meta := ngnp.Metadata{Filtered: make(map[string]uint64)}
	ctx := &cancelAfterMove{Context: context.Background()}
	records, _, plies, _, err := play(ctx, e, board(t, StartFEN), 0, s, &meta)
	if err != context.Canceled || records != nil || plies != 1 || meta.Searches != 1 {
		t.Fatalf("partial game survived cancellation: %v ply %d records %d searches %d", err, plies, len(records), meta.Searches)
	}
}

func TestUncompletedSearchCannotProduceLabel(t *testing.T) {
	e, _ := engine.NewSearchEngineWithHash(1)
	e.NewGame()
	meta := ngnp.Metadata{}
	_, err := search(e, board(t, StartFEN), 1, &meta)
	if err != errUnscored || meta.UnscoredSearches != 1 || meta.SearchNodes != 1 {
		t.Fatalf("invalid label accepted: %v %+v", err, meta)
	}
}

func TestEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("two deterministic two-game generator runs")
	}
	s := Defaults()
	s.Games = 2
	s.Nodes = 256
	s.HashMB = 1
	s.MaxPly = 32
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var previous []byte
	for run := 0; run < 2; run++ {
		path := filepath.Join(t.TempDir(), "e2e.ngnp")
		meta, err := Run(ctx, path, s, Provenance{strings.Repeat("a", 40), strings.Repeat("b", 64)})
		if err != nil {
			t.Fatal(err)
		}
		if meta.Games != 2 || meta.Positions == 0 || !meta.Closed || meta.Status != "complete" {
			t.Fatalf("incomplete run: %+v", meta)
		}
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		count := uint64(0)
		err = ngnp.Iterate(f, func(r ngnp.Record) error {
			count++
			if r.Flags != 0 || r.Result != ngnp.Draw || r.Ply < 8 || r.Ply >= 32 {
				t.Fatalf("bad record: %+v", r)
			}
			return nil
		})
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if count != meta.Positions {
			t.Fatalf("records %d != sidecar %d", count, meta.Positions)
		}
		raw, _ := os.ReadFile(path)
		if run == 1 && string(previous) != string(raw) {
			t.Fatal("same seed/settings were not reproducible")
		}
		previous = raw
		jsonBytes, _ := os.ReadFile(path + ".json")
		var stored ngnp.Metadata
		if err := json.Unmarshal(jsonBytes, &stored); err != nil || stored.SHA256 != meta.SHA256 {
			t.Fatalf("bad sidecar: %v", err)
		}
	}
}
