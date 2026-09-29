package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRequiredArguments(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run(nil, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "required:") {
		t.Fatalf("got %v", err)
	}
}
