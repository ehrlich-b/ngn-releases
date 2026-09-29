//go:build amd64 && amd64.v3

package countereval

import "testing"

func TestFeatureUpdateRowsSelectAVX2(t *testing.T) {
	if !featureUpdateRowsUseAVX2 {
		t.Fatal("GOAMD64 v3 did not select Counter AVX2 feature-row kernels")
	}
}
