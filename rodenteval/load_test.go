package rodenteval

import (
	"os"
	"strings"
	"testing"
)

func TestLoadV11AnandRejectsMalformedBytesTransactionally(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{name: "empty", data: nil, want: "size 0"},
		{name: "truncated", data: make([]byte, fileSize-1), want: "size 789567"},
		{name: "bad trailer", data: make([]byte, fileSize), want: "invalid Bullet trailer"},
	}
	wrongDigest := make([]byte, fileSize)
	copy(wrongDigest[payloadSize:], v11AnandTrailer)
	tests = append(tests, struct {
		name string
		data []byte
		want string
	}{name: "wrong digest", data: wrongDigest, want: "SHA-256"})

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model, err := loadV11AnandBytes(test.data)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("load error = %v, want substring %q", err, test.want)
			}
			if model != nil {
				t.Fatal("malformed load published a model")
			}
		})
	}
}

func TestLoadV11AnandRejectsEmptyPath(t *testing.T) {
	model, err := LoadV11Anand("")
	if err == nil || !strings.Contains(err.Error(), "empty path") {
		t.Fatalf("LoadV11Anand empty path error = %v", err)
	}
	if model != nil {
		t.Fatal("empty path published a model")
	}
}

func TestLoadV11AnandBoundsOversizeRead(t *testing.T) {
	path := t.TempDir() + "/oversize.nn"
	if err := os.WriteFile(path, make([]byte, fileSize+2), 0o600); err != nil {
		t.Fatal(err)
	}

	model, err := LoadV11Anand(path)
	if err == nil || !strings.Contains(err.Error(), "size 789569") {
		t.Fatalf("LoadV11Anand oversize error = %v, want bounded size 789569", err)
	}
	if model != nil {
		t.Fatal("oversize file published a model")
	}
}

func TestZeroModelIsRejected(t *testing.T) {
	var model Model
	position := minimalPosition(White)
	if _, err := model.Metadata(); err == nil {
		t.Fatal("zero model metadata unexpectedly succeeded")
	}
	if _, err := model.EvaluateRaw(position); err == nil {
		t.Fatal("zero model evaluation unexpectedly succeeded")
	}
}
