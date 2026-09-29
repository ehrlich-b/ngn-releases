package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadTexelSamplesRejectsInvalidOutcomeLabels(t *testing.T) {
	const fen = "4k3/8/8/8/8/8/4P3/4K3 w - - 0 1"
	for _, label := range []string{"NaN", "+Inf", "-Inf", "-0.1", "1.1"} {
		t.Run(label, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "data.txt")
			// Even an already-seen board cannot hide an invalid observation.
			if err := os.WriteFile(path, []byte(fen+" 0.5\n"+fen+" "+label+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, _, err := LoadTexelSamples(path); err == nil || !strings.Contains(err.Error(), "line 2") {
				t.Fatalf("label %q must fail with its line number; got %v", label, err)
			}
		})
	}
}

func TestLoadTexelSamplesPreservesFiniteSoftOutcome(t *testing.T) {
	path := filepath.Join(t.TempDir(), "soft.txt")
	if err := os.WriteFile(path, []byte("4k3/8/8/8/8/8/4P3/4K3 w - - 0 1 0.375\n"), 0600); err != nil {
		t.Fatal(err)
	}
	samples, duplicates, err := LoadTexelSamples(path)
	if err != nil || duplicates != 0 || len(samples) != 1 || samples[0].Result != 0.375 {
		t.Fatalf("valid soft target changed: samples=%v duplicates=%d err=%v", samples, duplicates, err)
	}
}
