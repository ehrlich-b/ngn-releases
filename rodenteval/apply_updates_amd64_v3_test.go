//go:build amd64 && amd64.v3

package rodenteval

import (
	"testing"
	"unsafe"
)

func TestRodentApplyUpdatesSelectsAVX2OnAMD64V3(t *testing.T) {
	if !rodentApplyUpdatesUsesAVX2 {
		t.Fatal("amd64.v3 build selected portable Rodent update kernels")
	}
}

func TestRodentApplyUpdatesAVX2AcceptsUnalignedInputs(t *testing.T) {
	storage := make([][HiddenSize + 1]int16, 5)
	destination := (*[HiddenSize]int16)(unsafe.Pointer(&storage[0][1]))
	add0 := (*[HiddenSize]int16)(unsafe.Pointer(&storage[1][1]))
	subtract1 := (*[HiddenSize]int16)(unsafe.Pointer(&storage[2][1]))
	add2 := (*[HiddenSize]int16)(unsafe.Pointer(&storage[3][1]))
	subtract3 := (*[HiddenSize]int16)(unsafe.Pointer(&storage[4][1]))
	for lane := 0; lane < HiddenSize; lane++ {
		destination[lane] = int16(lane*193 + 32760)
		add0[lane] = int16(lane*257 + 1)
		subtract1[lane] = int16(lane*509 - 32768)
		add2[lane] = int16(lane*769 + 32767)
		subtract3[lane] = int16(lane*1021 - 1)
	}
	want := *destination
	rodentApplyUpdates4Portable(&want, add0, subtract1, add2, subtract3)
	rodentApplyUpdates4AVX2(destination, add0, subtract1, add2, subtract3)
	if *destination != want {
		t.Fatal("unaligned AVX2 update differs from portable oracle")
	}
}
