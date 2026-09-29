//go:build !amd64 || !amd64.v3

package rodentv12eval

import "testing"

func TestRodentV12OutputDotSelectsPortableFallback(t *testing.T) {
	if rodentV12OutputDotUsesAVX2 {
		t.Fatal("portable build selected AVX2 Rodent V1.2 output kernel")
	}
}
