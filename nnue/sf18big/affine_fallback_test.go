//go:build !amd64 || !amd64.v3

package sf18big

import "testing"

func TestPortableBuildSelectsScalarAffine1024(t *testing.T) {
	if affine1024UsesAVX2 || affine32UsesAVX2 {
		t.Fatal("portable build unexpectedly selected AVX2 affine kernels")
	}
}
