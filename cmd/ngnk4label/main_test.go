package main

import (
	"bytes"
	"testing"
)

func TestRunRequiresCompleteFrozenInvocation(t *testing.T) {
	for _, args := range [][]string{nil, {"-input", "x"}, {"unexpected"}} {
		if err := run(args, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
			t.Fatalf("run(%q) succeeded", args)
		}
	}
}
