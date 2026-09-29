//go:build amd64 && amd64.v3

package rodentv12eval

import (
	"testing"
	"unsafe"
)

func TestRodentV12RefreshPerspectiveSelectsAVX2OnAMD64V3(t *testing.T) {
	if !rodentV12RefreshPerspectiveUsesAVX2 {
		t.Fatal("amd64.v3 build selected portable Rodent V1.2 refresh kernel")
	}
}

func TestRodentV12RefreshPerspectiveAVX2AcceptsUnalignedInputs(t *testing.T) {
	biasStorage := make([]int16, HiddenSize+1)
	destinationStorage := make([]int16, HiddenSize+1)
	biases := (*[HiddenSize]int16)(unsafe.Pointer(&biasStorage[1]))
	destination := (*[HiddenSize]int16)(unsafe.Pointer(&destinationStorage[1]))
	rowStorage := make([][]int16, 4)
	var rows [rodentV12MaximumRefreshRows]*[HiddenSize]int16
	for row := range rowStorage {
		rowStorage[row] = make([]int16, HiddenSize+1)
		rows[row] = (*[HiddenSize]int16)(unsafe.Pointer(&rowStorage[row][1]))
	}
	for lane := 0; lane < HiddenSize; lane++ {
		biases[lane] = int16(lane*193 + 32760)
		for row := range rowStorage {
			rows[row][lane] = int16(lane*(row*2+257) + row + 1)
		}
	}
	var want [HiddenSize]int16
	rodentV12RefreshPerspectivePortable(&want, biases, &rows, 4)
	rodentV12RefreshPerspectiveAVX2(destination, biases, &rows, 4)
	if *destination != want {
		t.Fatal("unaligned AVX2 refresh differs from portable oracle")
	}
}
