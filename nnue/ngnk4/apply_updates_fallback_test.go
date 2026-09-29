//go:build !amd64 || !amd64.v3

package ngnk4

import "testing"

func TestK4ApplyUpdatesSelectPortableFallback(t *testing.T) {
	if k4ApplyUpdatesUsesAVX2 {
		t.Fatal("portable build selected AVX2 K4 update kernels")
	}
}
