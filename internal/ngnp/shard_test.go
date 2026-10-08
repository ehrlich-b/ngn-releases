package ngnp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestShardWholeGamesChecksumAndNoOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.ngnp")
	meta := Metadata{StartedAt: time.Now().UTC(), EngineCommit: strings.Repeat("a", 40), BinarySHA256: strings.Repeat("b", 64),
		Settings: json.RawMessage(`{}`), Status: "complete", Games: 1, GameResults: [3]uint64{0, 0, 1}}
	s, err := Create(path, meta)
	if err != nil {
		t.Fatal(err)
	}
	r, err := FromPosition(position(t, startFEN), 120, Win, 12, 0)
	if err != nil {
		t.Fatal(err)
	}
	bad := r
	bad.STM = 9
	if err := s.AppendGame([]Record{r, bad}); err == nil {
		t.Fatal("accepted invalid game")
	}
	st, _ := os.Stat(path)
	if st.Size() != 0 {
		t.Fatal("partial game was written")
	}
	if err := s.AppendGame([]Record{r, r}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(&meta); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(&meta); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendGame([]Record{r}); err == nil {
		t.Fatal("appended to closed shard")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(raw)
	sidecar, err := os.ReadFile(path + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var got Metadata
	if err := json.Unmarshal(sidecar, &got); err != nil {
		t.Fatal(err)
	}
	if len(raw) != 64 || !got.Closed || got.Positions != 2 || got.Bytes != 64 || got.SHA256 != hex.EncodeToString(h[:]) {
		t.Fatalf("incorrect finalization: %+v", got)
	}
	if _, err := Create(path, meta); err == nil {
		t.Fatal("overwrote existing shard")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(raw) {
		t.Fatal("existing data changed")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(path, meta); err == nil {
		t.Fatal("reused a reserved sidecar")
	}
}

func TestExistingDataSurvivesSidecarReservation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.ngnp")
	if err := os.WriteFile(path, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(path, Metadata{}); err == nil {
		t.Fatal("overwrote data")
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != "keep" {
		t.Fatal("data changed")
	}
	if _, err := os.Stat(path + ".json"); !os.IsNotExist(err) {
		t.Fatal("failed create left a sidecar")
	}
}
