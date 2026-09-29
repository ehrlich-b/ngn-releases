//go:build amd64 && amd64.v3

package sf18big

import "testing"

func TestAMD64V3SelectsAVX2TransformInputs(t *testing.T) {
	if !transformInputsUsesAVX2 {
		t.Fatal("amd64.v3 build did not select AVX2 transform kernel")
	}
}
