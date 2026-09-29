package ngnk4

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

// Tensors is the canonical deployed logical ordering accepted by Marshal.
// Training-only factorizer weights must be merged before constructing it.
type Tensors struct {
	InputWeights  [TotalInputFeatures][HiddenSize]int16
	InputBiases   [HiddenSize]int16
	OutputWeights [OutputBuckets][2][HiddenSize]int16
	OutputBiases  [OutputBuckets]int16
}

// Marshal writes the exact versioned format consumed by Load. manifestSHA
// binds the deployed tensors to a separately retained training manifest.
func Marshal(tensors *Tensors, manifestSHA [sha256.Size]byte) ([]byte, error) {
	if tensors == nil {
		return nil, fmt.Errorf("%w: nil tensors", ErrFormat)
	}
	if manifestSHA == ([sha256.Size]byte{}) {
		return nil, fmt.Errorf("%w: zero training-manifest SHA-256", ErrIntegrity)
	}
	data := make([]byte, FileSize)
	header := data[:HeaderSize]
	payload := data[HeaderSize:]
	copy(header[:8], fileMagic[:])
	binary.LittleEndian.PutUint16(header[8:10], fileVersion)
	binary.LittleEndian.PutUint16(header[10:12], HeaderSize)
	values := []uint32{
		architectureCode, featureSetCode, quantizationCode, scorePolicyCode,
		InputBuckets, InputSize, HiddenSize, OutputBuckets,
		InputScale, LayerScale, OutputScale,
	}
	for i, value := range values {
		binary.LittleEndian.PutUint32(header[12+i*4:16+i*4], value)
	}
	binary.LittleEndian.PutUint64(header[56:64], PayloadSize)
	copy(header[64:96], manifestSHA[:])

	offset := 0
	writeI16 := func(value int16) {
		binary.LittleEndian.PutUint16(payload[offset:offset+2], uint16(value))
		offset += 2
	}
	for input := range tensors.InputWeights {
		for _, value := range tensors.InputWeights[input] {
			writeI16(value)
		}
	}
	for _, value := range tensors.InputBiases {
		writeI16(value)
	}
	for bucket := range tensors.OutputWeights {
		for perspective := range tensors.OutputWeights[bucket] {
			for _, value := range tensors.OutputWeights[bucket][perspective] {
				writeI16(value)
			}
		}
	}
	for _, value := range tensors.OutputBiases {
		writeI16(value)
	}
	if offset != PayloadSize {
		panic("ngnk4: internal payload size mismatch")
	}
	payloadSHA := sha256.Sum256(payload)
	copy(header[96:128], payloadSHA[:])
	return data, nil
}
