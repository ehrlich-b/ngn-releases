//go:build amd64 && amd64.v3

package rodenteval

import "testing"

func TestRodentOutputDotSelectsAVX2OnAMD64V3(t *testing.T) {
	if !rodentOutputDotUsesAVX2 {
		t.Fatal("amd64.v3 build selected portable Rodent output kernel")
	}
}
