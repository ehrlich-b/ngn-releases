//go:build amd64 && amd64.v3

package rodentv12eval

import (
	"testing"
	"unsafe"
)

func TestRodentV12ApplyUpdatesSelectAVX2OnAMD64V3(t *testing.T) {
	if !rodentV12ApplyUpdatesUsesAVX2 {
		t.Fatal("amd64.v3 build selected portable Rodent V1.2 update kernels")
	}
}

func TestRodentV12ApplyUpdatesAVX2AcceptUnalignedInputs(t *testing.T) {
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
	initial := *destination

	for _, test := range []struct {
		name     string
		portable func(*[HiddenSize]int16)
		avx2     func(*[HiddenSize]int16)
	}{
		{"as", func(d *[HiddenSize]int16) { rodentV12ApplyUpdates2Portable(d, add0, subtract1) }, func(d *[HiddenSize]int16) { rodentV12ApplyUpdates2AVX2(d, add0, subtract1) }},
		{"ass", func(d *[HiddenSize]int16) { rodentV12ApplyUpdates3Portable(d, add0, subtract1, add2) }, func(d *[HiddenSize]int16) { rodentV12ApplyUpdates3AVX2(d, add0, subtract1, add2) }},
		{"asas", func(d *[HiddenSize]int16) { rodentV12ApplyUpdates4Portable(d, add0, subtract1, add2, subtract3) }, func(d *[HiddenSize]int16) { rodentV12ApplyUpdates4AVX2(d, add0, subtract1, add2, subtract3) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			*destination = initial
			want := initial
			test.portable(&want)
			test.avx2(destination)
			if *destination != want {
				t.Fatal("unaligned AVX2 update differs from portable oracle")
			}
		})
	}
}
