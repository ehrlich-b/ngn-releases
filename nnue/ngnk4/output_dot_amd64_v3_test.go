//go:build amd64 && amd64.v3

package ngnk4

import "testing"

func TestK4OutputDotSelectsAVX2OnAMD64V3(t *testing.T) {
	if !k4OutputDotUsesAVX2 {
		t.Fatal("amd64.v3 build selected portable K4 output kernel")
	}
}
