//go:build !amd64 || !amd64.v3

package ngnk4

import "testing"

func TestK4OutputDotSelectsPortableFallback(t *testing.T) {
	if k4OutputDotUsesAVX2 {
		t.Fatal("portable build selected AVX2 K4 output kernel")
	}
}
