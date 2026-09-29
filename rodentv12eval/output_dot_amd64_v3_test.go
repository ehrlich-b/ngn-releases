//go:build amd64 && amd64.v3

package rodentv12eval

import "testing"

func TestRodentV12OutputDotSelectsAVX2OnAMD64V3(t *testing.T) {
	if !rodentV12OutputDotUsesAVX2 {
		t.Fatal("amd64.v3 build selected portable Rodent V1.2 output kernel")
	}
}
