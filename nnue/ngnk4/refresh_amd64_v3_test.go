//go:build amd64 && amd64.v3

package ngnk4

import (
	"testing"
	"unsafe"
)

func TestK4RefreshPerspectiveSelectsAVX2OnAMD64V3(t *testing.T) {
	if !k4RefreshPerspectiveUsesAVX2 {
		t.Fatal("amd64.v3 build selected portable K4 refresh kernel")
	}
}

func TestK4RefreshPerspectiveAVX2AcceptsUnalignedInputs(t *testing.T) {
	biasStorage := make([]int16, HiddenSize+1)
	destinationStorage := make([]int16, HiddenSize+1)
	biases := (*[HiddenSize]int16)(unsafe.Pointer(&biasStorage[1]))
	destination := (*[HiddenSize]int16)(unsafe.Pointer(&destinationStorage[1]))
	rowStorage := make([][]int16, 4)
	var rows [k4MaximumRefreshRows]*[HiddenSize]int16
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
	k4RefreshPerspectivePortable(&want, biases, &rows, 4)
	k4RefreshPerspectiveAVX2(destination, biases, &rows, 4)
	if *destination != want {
		t.Fatal("unaligned AVX2 refresh differs from portable oracle")
	}
}
