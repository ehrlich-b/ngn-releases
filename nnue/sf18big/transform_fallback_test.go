//go:build !amd64 || !amd64.v3

package sf18big

import "testing"

func TestPortableBuildSelectsScalarTransformInputs(t *testing.T) {
	if transformInputsUsesAVX2 {
		t.Fatal("portable build unexpectedly selected AVX2 transform kernel")
	}
}
