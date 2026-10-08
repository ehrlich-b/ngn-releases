package datagen

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ehrlich-b/ngn/engine"
	"github.com/ehrlich-b/ngn/internal/ngnp"
)

func TestBucketNetworkSearchMatchesEngineSelection(t *testing.T) {
	assertBucketNetworkSearch(t, filepath.Join("..", "..", "testdata", "ngnn2", "random_h32_b8.nnue"), engine.EvaluatorBackendNGNN2Name)
}

func TestKingBucketNetworkSearchMatchesEngineSelection(t *testing.T) {
	assertBucketNetworkSearch(t, filepath.Join("..", "..", "testdata", "ngnn3", "random_h32_k8_b8.nnue"), engine.EvaluatorBackendNGNN3Name)
}

func assertBucketNetworkSearch(t *testing.T, path, backend string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := Defaults()
	s.HashMB, s.NetPath = 1, path
	e, digest, err := newSearcher(s)
	if err != nil {
		t.Fatal(err)
	}
	if digest != fmt.Sprintf("%x", sha256.Sum256(raw)) || e.SelectedEvaluatorBackend() != backend {
		t.Fatal("bucket network not selected/identified")
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
	for _, fen := range []string{StartFEN, "r3k2r/8/8/8/8/8/8/R3K2R b KQkq - 0 1", "8/4k3/3p4/3P4/4K3/8/8/8 w - - 0 1"} {
		pos := board(t, fen)
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
		if pos.Turn() == engine.Black {
			want = -want
		}
		if found.whiteScore != want || found.move != info.BestMove || meta.SearchNodes != info.Nodes || engine.GenerateFEN(pos) != before {
			t.Fatal("bucket network datagen search/perspective/root mismatch")
		}
	}
	raw[99] ^= 1
	s.NetPath = filepath.Join(t.TempDir(), "corrupt.nnue")
	if err := os.WriteFile(s.NetPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if failed, _, err := newSearcher(s); err == nil || failed != nil || !strings.Contains(err.Error(), "CRC") {
		t.Fatal("datagen accepted corrupt network or fell back to HCE")
	}
}
