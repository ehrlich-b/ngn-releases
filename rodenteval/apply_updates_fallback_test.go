//go:build !amd64 || !amd64.v3

package rodenteval

import "testing"

func TestRodentApplyUpdatesSelectsPortableFallback(t *testing.T) {
	if rodentApplyUpdatesUsesAVX2 {
		t.Fatal("portable build selected AVX2 Rodent update kernels")
	}
}
