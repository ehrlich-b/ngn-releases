//go:build !amd64 || !amd64.v3

package rodenteval

import "testing"

func TestRodentOutputDotSelectsPortableFallback(t *testing.T) {
	if rodentOutputDotUsesAVX2 {
		t.Fatal("portable build selected AVX2 Rodent output kernel")
	}
}
