package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ehrlich-b/ngn/engine"
	"github.com/ehrlich-b/ngn/internal/ngnp"
)

func TestInspectValidatesHashesAndComputesDistributions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stats.ngnp")
	pos, _ := engine.ParseFEN("4k3/8/8/8/8/8/8/R3K3 w - - 0 1")
	r, err := ngnp.FromPosition(pos, -1, ngnp.Win, 81, 0)
	if err != nil {
		t.Fatal(err)
	}
	meta := ngnp.Metadata{StartedAt: time.Now().UTC(), EngineCommit: strings.Repeat("a", 40), BinarySHA256: strings.Repeat("b", 64),
		Settings: json.RawMessage(`{}`), Status: "complete", Games: 1, GamePlies: 100, GameResults: [3]uint64{0, 0, 1}}
	shard, err := ngnp.Create(path, meta)
	if err != nil {
		t.Fatal(err)
	}
	if err := shard.AppendGame([]ngnp.Record{r, r}); err != nil {
		t.Fatal(err)
	}
	if err := shard.Close(&meta); err != nil {
		t.Fatal(err)
	}
	s, err := inspect([]string{path, filepath.Dir(path)}, true, 10)
	if err != nil {
		t.Fatal(err)
	}
	if s.Files != 1 || s.Records != 2 || s.RecordResults.Win != 2 || s.DuplicateRate != 0.5 || s.ScoreHistogram["-100..-1"] != 2 ||
		s.PlyHistogram["80..99"] != 2 || s.PieceHistogram["3"] != 2 || s.AverageGamePlies != 100 || s.PositionsPerHour != 720 || s.BytesPerPosition != 32 {
		t.Fatalf("bad stats: %+v", s)
	}
	raw, _ := os.ReadFile(path)
	raw[24] ^= 1
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := inspect([]string{path}, true, 10); err == nil {
		t.Fatal("accepted checksum mismatch")
	}
	if err := os.Remove(path + ".json"); err != nil {
		t.Fatal(err)
	}
	if _, err := inspect([]string{path}, true, 10); err == nil {
		t.Fatal("accepted missing sidecar")
	}
	if _, err := inspect([]string{path}, false, 10); err != nil {
		t.Fatal(err)
	}
}

func TestDuplicateEstimatorBoundsMemory(t *testing.T) {
	d := duplicates{counts: make(map[uint64]uint64)}
	for i := uint64(0); i < 250000; i++ {
		d.add(ngnp.Record{Occupancy: i})
	}
	if len(d.counts) > 100000 || d.mask == 0 {
		t.Fatalf("unbounded sample: %d mask %d", len(d.counts), d.mask)
	}
	for i := uint64(0); i < 250000; i++ {
		d.add(ngnp.Record{Occupancy: i})
	}
	for _, count := range d.counts {
		if count != 2 {
			t.Fatalf("retained key lost prior count: %d", count)
		}
	}
}
