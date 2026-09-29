package rodentv12eval

import (
	"os"
	"strings"
	"testing"
)

func TestLoadV12DefaultRejectsMalformedBytesTransactionally(t *testing.T) {
	tests := []struct {
		name string
		data func() []byte
		want string
	}{
		{name: "empty", data: func() []byte { return nil }, want: "size 0"},
		{name: "truncated", data: func() []byte { return make([]byte, fileSize-1) }, want: "size 4744767"},
		{name: "bad trailer", data: func() []byte { return make([]byte, fileSize) }, want: "invalid Bullet trailer"},
		{name: "wrong digest", data: func() []byte {
			data := make([]byte, fileSize)
			copy(data[payloadSize:], v12DefaultTrailer)
			return data
		}, want: "SHA-256"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model, err := loadV12DefaultBytes(test.data())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("load error = %v, want substring %q", err, test.want)
			}
			if model != nil {
				t.Fatal("malformed load published a model")
			}
		})
	}
}

func TestLoadV12DefaultRejectsEmptyPath(t *testing.T) {
	model, err := LoadV12Default("")
	if err == nil || !strings.Contains(err.Error(), "empty path") {
		t.Fatalf("LoadV12Default empty path error = %v", err)
	}
	if model != nil {
		t.Fatal("empty path published a model")
	}
}

func TestLoadV12DefaultBoundsOversizeRead(t *testing.T) {
	path := t.TempDir() + "/oversize.nn"
	if err := os.WriteFile(path, make([]byte, fileSize+2), 0o600); err != nil {
		t.Fatal(err)
	}

	model, err := LoadV12Default(path)
	if err == nil || !strings.Contains(err.Error(), "size 4744769") {
		t.Fatalf("LoadV12Default oversize error = %v, want bounded size 4744769", err)
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
