//go:build !amd64 || !amd64.v3

package countereval

import "testing"

func TestFeatureUpdateRowsSelectPortableFallback(t *testing.T) {
	if featureUpdateRowsUseAVX2 {
		t.Fatal("non-v3 build selected Counter AVX2 feature-row kernels")
	}
}
