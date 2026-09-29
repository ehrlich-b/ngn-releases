//go:build amd64 && amd64.v3

package sf18big

import "testing"

func TestAMD64V3SelectsAVX2Affine1024(t *testing.T) {
	if !affine1024UsesAVX2 || !affine32UsesAVX2 {
		t.Fatal("amd64.v3 build did not select all AVX2 affine kernels")
	}
}
