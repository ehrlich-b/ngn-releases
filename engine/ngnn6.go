package engine

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
)

const (
	ngnn6HeaderSize = 120
	ngnn6Units      = 16
	ngnn6Buckets    = 8
	ngnn6InputScale = 64
)

// NGNN6Network is a parsed residual-head layout, separate from the evaluator
// configuration. ReadNGNN1 also loads this format for explicit selection.
// The embedded linear layout uses version 3 so its feature semantics stay exact.
type NGNN6Network struct {
	linear       *NGNN1Network
	hiddenWeight []int16 // [16][2H], side to move first
	hiddenBias   [ngnn6Units]int32
	output       [ngnn6Buckets * ngnn6Units]int16
	outputBias   [ngnn6Buckets]int32
}

// ReadNGNN6 validates all dimensions before allocating a bounded payload. The
// only widths reserved by version 6 are H32 (fixtures) and H768. See docs/ngnn6.md.
func ReadNGNN6(r io.Reader) (*NGNN6Network, error) {
	var header [ngnn6HeaderSize]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, fmt.Errorf("NGNN6 header: %w", err)
	}
	u32 := func(offset int) uint32 { return binary.LittleEndian.Uint32(header[offset:]) }
	if string(header[:4]) != "NGN6" || u32(4) != 6 {
		return nil, fmt.Errorf("NGNN6: unsupported magic/version")
	}
	h := int(u32(8))
	if h != 32 && h != 768 {
		return nil, fmt.Errorf("NGNN6: unsupported hidden width %d", h)
	}
	if u32(12) != 8 || u32(16) != ngnn6Buckets || u32(96) != ngnn6Units || u32(116) != 0 {
		return nil, fmt.Errorf("NGNN6: unsupported dimensions or reserved flags")
	}
	if u32(20) != ngnn1QA || u32(24) != ngnn1QB || u32(28) != ngnn1Scale ||
		u32(100) != ngnn6InputScale || u32(104) != ngnn1QA || u32(108) != ngnn1QB || u32(112) != ngnn1Scale {
		return nil, fmt.Errorf("NGNN6: invalid scales")
	}
	for _, bucket := range header[32:96] {
		if bucket >= 8 {
			return nil, fmt.Errorf("NGNN6: king bucket map entry out of range")
		}
	}
	// 120-byte header, 12386*H+384-byte payload, four-byte CRC. With
	// validated H this is at most 9,512,956 bytes, including on 32-bit hosts.
	size := 12386*h + 508
	data := make([]byte, size)
	copy(data, header[:])
	if _, err := io.ReadFull(r, data[len(header):]); err != nil {
		return nil, fmt.Errorf("NGNN6 payload: %w", err)
	}
	var extra [1]byte
	if count, err := io.ReadFull(r, extra[:]); count != 0 || err != io.EOF {
		return nil, fmt.Errorf("NGNN6: trailing data or read error: %v", err)
	}
	if crc32.ChecksumIEEE(data[:size-4]) != binary.LittleEndian.Uint32(data[size-4:]) {
		return nil, fmt.Errorf("NGNN6: CRC mismatch")
	}
	offset := len(header)
	read16 := func(count int) []int16 {
		values := make([]int16, count)
		for i := range values {
			values[i] = int16(binary.LittleEndian.Uint16(data[offset:]))
			offset += 2
		}
		return values
	}
	read32 := func() int32 {
		value := int32(binary.LittleEndian.Uint32(data[offset:]))
		offset += 4
		return value
	}
	base := &NGNN1Network{hidden: h, version: 3, kingBuckets: 8, buckets: ngnn6Buckets}
	copy(base.kingMap[:], header[32:96])
	base.weights = read16(8 * ngnn1Features * h)
	base.bias = read16(h)
	base.output = read16(ngnn6Buckets * 2 * h)
	base.outputBias = read32()
	base.otherBiases = make([]int32, ngnn6Buckets-1)
	for i := range base.otherBiases {
		base.otherBiases[i] = read32()
	}
	n := &NGNN6Network{linear: base, hiddenWeight: read16(ngnn6Units * 2 * h)}
	for i := range n.hiddenBias {
		n.hiddenBias[i] = read32()
	}
	copy(n.output[:], read16(len(n.output)))
	for i := range n.outputBias {
		n.outputBias[i] = read32()
	}
	// Accumulate in int64, but reserve signed int32 preactivation ranges
	// for both residual stages. Prove the complete [0,255] input box, not
	// merely the fixture positions or a bias-only absolute-value estimate.
	for j, bias := range n.hiddenBias {
		if !ngnn6AffineBound(n.hiddenWeight[j*2*h:(j+1)*2*h], bias) {
			return nil, fmt.Errorf("NGNN6: bottleneck arithmetic bound at unit %d", j)
		}
	}
	for b, bias := range n.outputBias {
		if !ngnn6AffineBound(n.output[b*ngnn6Units:(b+1)*ngnn6Units], bias) {
			return nil, fmt.Errorf("NGNN6: projection arithmetic bound at bucket %d", b)
		}
	}
	base.prepareOutputLayout()
	return n, nil
}

func ngnn6AffineBound(weights []int16, bias int32) bool {
	low, high := int64(bias), int64(bias)
	for _, w := range weights {
		term := int64(w) * ngnn1QA
		low += min(int64(0), term)
		high += max(int64(0), term)
	}
	return low >= -1<<31 && high <= 1<<31-1
}
