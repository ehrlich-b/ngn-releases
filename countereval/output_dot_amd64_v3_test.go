//go:build amd64 && amd64.v3

package countereval

import "testing"

func TestCounterOutputDotAVX2Selected(t *testing.T) {
	if !counterOutputDotUsesAVX2 {
		t.Fatal("amd64.v3 build did not select AVX2 output kernel")
	}
}
