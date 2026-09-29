package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLISmokeWritesCompleteUnresolvedRow(t *testing.T) {
	dir := t.TempDir()
	openings := filepath.Join(dir, "openings.txt")
	out := filepath.Join(dir, "corpus.jsonl")
	if err := os.WriteFile(openings, []byte("e2e4\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", ".", "-openings", openings, "-opening-start", "0", "-games", "1", "-nodes", "1", "-maxplies", "1", "-seed", "7", "-out", out, "-source", "test-smoke")
	cmd.Env = os.Environ()
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("command: %v\n%s", err, output)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"completed":false`) || !strings.Contains(string(b), `"terminal_reason":"maxplies"`) {
		t.Fatalf("unexpected corpus row: %s", b)
	}
}
