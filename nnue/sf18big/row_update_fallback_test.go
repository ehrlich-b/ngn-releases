//go:build !amd64 || !amd64.v3

package sf18big

import "testing"

func TestPortableBuildSelectsScalarRowUpdates(t *testing.T) {
	if rowUpdateUsesAVX2 {
		t.Fatal("portable build unexpectedly selected AVX2 row updates")
	}
}
