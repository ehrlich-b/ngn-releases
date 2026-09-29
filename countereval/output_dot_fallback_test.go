//go:build !amd64 || !amd64.v3

package countereval

import "testing"

func TestCounterOutputDotPortableSelected(t *testing.T) {
	if counterOutputDotUsesAVX2 {
		t.Fatal("portable build selected AVX2 output kernel")
	}
}
