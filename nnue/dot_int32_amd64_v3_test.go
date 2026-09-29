//go:build amd64 && amd64.v3

package nnue_test

import (
	"testing"

	"github.com/ehrlich-b/ngn/nnue"
)

func TestN4VAMD64V3SelectsAVX2(t *testing.T) {
	_, _, avx2 := nnue.BoundedOutputDotsForTest(
		[nnue.HiddenSize]int32{},
		[nnue.HiddenSize]int32{},
		[nnue.PerspectiveCount * nnue.HiddenSize]int16{},
	)
	if !avx2 {
		t.Fatal("amd64.v3 build did not select AVX2 bounded output")
	}
}
