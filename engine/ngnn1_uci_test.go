package engine

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNGNN1UCIOptionsAndFailClosed(t *testing.T) {
	u := NewUCIEngine()
	if err := u.searcher.ResizeHash(1); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	u.handleCommand("uci", &out)
	for _, option := range []string{"option name EvalFile type string default <empty>\n", "option name UseNNUE type check default false\n"} {
		if !strings.Contains(out.String(), option) {
			t.Fatal("missing option: " + option)
		}
	}
	if got := u.searcher.SelectedEvaluatorBackend(); got != "hce" {
		t.Fatal(got)
	}
	u.handleCommand("setoption name UseNNUE value true", &out)
	if !strings.Contains(out.String(), "info string error eval option:") || u.searcher.SelectedEvaluatorBackend() != "hce" {
		t.Fatal("missing-file enable did not fail closed")
	}
	path := filepath.Join(t.TempDir(), "random net.nnue")
	data := ngnn1TestBytes(32, 17, false)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	u.handleCommand("setoption name EvalFile value "+path, &out)
	if u.searcher.SelectedEvaluatorBackend() != "ngnn1" {
		t.Fatal("valid file did not enable pending UseNNUE")
	}
	n := u.evaluatorConfigSnapshot().network
	if n == nil {
		t.Fatal("missing loaded weights")
	}
	got, backend, err := u.searcher.EvaluateSelected(u.position)
	if err != nil || backend != "ngnn1" || got != ngnn1Oracle(n, u.position) {
		t.Fatalf("eval=%d %s %v", got, backend, err)
	}
	u.handleCommand("setoption name UseNNUE value false", &out)
	if u.searcher.SelectedEvaluatorBackend() != "hce" {
		t.Fatal("disable did not select HCE")
	}
	u.handleCommand("setoption name UseNNUE value true", &out)
	data[99] ^= 1
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	u.handleCommand("setoption name EvalFile value "+path, &out)
	if u.searcher.SelectedEvaluatorBackend() != "hce" || u.evaluatorConfigSnapshot().network != nil || !strings.Contains(out.String(), "CRC mismatch; using hce") {
		t.Fatal("reload kept stale weights: " + out.String())
	}
	u.handleCommand("setoption name EvalFile value <empty>", &out)
	if u.evaluatorConfigSnapshot().file != "" || u.searcher.SelectedEvaluatorBackend() != "hce" {
		t.Fatal("empty path failed")
	}
}

func TestNGNN1UCITransitionsAndModelIdentity(t *testing.T) {
	u := NewUCIEngine()
	if err := u.searcher.ResizeHash(1); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "net.nnue")
	if err := os.WriteFile(path, ngnn1TestBytes(32, 23, false), 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	u.handleCommand("setoption name EvalFile value "+path, &out)
	u.handleCommand("setoption name UseNNUE value true", &out)
	for _, command := range []string{
		"go infinite", "stop", "setoption name Threads value 4", "position startpos moves e2e4 e7e5 g1f3",
		"go infinite", "setoption name Threads value 1", "ucinewgame", "position startpos moves d2d4 d7d5",
	} {
		u.handleCommand(command, &out)
	}
	score, backend, err := u.searcher.EvaluateSelected(u.position)
	if err != nil || backend != "ngnn1" || score != ngnn1Oracle(u.evaluatorConfigSnapshot().network, u.position) {
		t.Fatalf("lifecycle eval=%d %s %v", score, backend, err)
	}
	const key = 98764321
	u.searcher.TTStore(key, EmptyMove, 42, 1, Exact, false)
	u.searcher.worker.history.lastMovePlayed = NewMove(A1, A2, WhiteKing, NoPiece, NoType, 0)
	old := u.evaluatorConfigSnapshot().network
	u.handleCommand("setoption name EvalFile value "+path, &out)
	if u.evaluatorConfigSnapshot().network == old {
		t.Fatal("reload retained model identity")
	}
	if _, _, _, _, hit, _ := u.searcher.TTProbe(key); hit {
		t.Fatal("reload retained TT entry")
	}
	if _, _, err := u.searcher.EvaluateSelected(u.position); err != nil {
		t.Fatal(err)
	}
	if u.searcher.worker.history.lastMovePlayed != EmptyMove {
		t.Fatal("reload retained old history")
	}
	// Loading different widths while a search is active must join that search
	// and rebuild the private stack before the next evaluation.
	for _, h := range []int{16, 256, 2048} {
		if err := os.WriteFile(path, ngnn1TestBytes(h, 23, false), 0600); err != nil {
			t.Fatal(err)
		}
		u.handleCommand("go infinite", &out)
		u.handleCommand("setoption name EvalFile value "+path, &out)
		n := u.evaluatorConfigSnapshot().network
		score, backend, err := u.searcher.EvaluateSelected(u.position)
		if err != nil || n == nil || n.hidden != h || backend != "ngnn1" || score != ngnn1Oracle(n, u.position) {
			t.Fatalf("H=%d reload: score=%d backend=%s err=%v", h, score, backend, err)
		}
		if e := u.searcher.worker.evaluator; e.depth != 0 || e.nnue.network != n {
			t.Fatalf("H=%d reload retained stale accumulator state", h)
		}
	}
}
