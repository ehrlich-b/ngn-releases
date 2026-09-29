package rodentv12eval

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

var v12DefaultTrailer = bytes.Repeat([]byte("bullet"), 8)

// LoadV12Default loads only the exact external Rodent V1.2 default artifact.
// Failure returns no partially initialized model.
func LoadV12Default(path string) (*Model, error) {
	if path == "" {
		return nil, fmt.Errorf("rodent v1.2 default model: empty path")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("rodent v1.2 default model: open: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, fileSize+1))
	if err != nil {
		return nil, fmt.Errorf("rodent v1.2 default model: read: %w", err)
	}
	return loadV12DefaultBytes(data)
}

func loadV12DefaultBytes(data []byte) (*Model, error) {
	if len(data) != fileSize {
		return nil, fmt.Errorf("rodent v1.2 default model: size %d, want %d", len(data), fileSize)
	}
	if len(v12DefaultTrailer) != fileSize-payloadSize ||
		!bytes.Equal(data[payloadSize:], v12DefaultTrailer) {
		return nil, fmt.Errorf("rodent v1.2 default model: invalid Bullet trailer")
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(data))
	if digest != V12DefaultSHA256 {
		return nil, fmt.Errorf("rodent v1.2 default model: SHA-256 %s, want %s", digest, V12DefaultSHA256)
	}

	model := new(Model)
	offset := 0
	readI16 := func() int16 {
		value := int16(binary.LittleEndian.Uint16(data[offset : offset+2]))
		offset += 2
		return value
	}
	for input := range model.inputWeights {
		for hidden := range model.inputWeights[input] {
			model.inputWeights[input][hidden] = readI16()
		}
	}
	for hidden := range model.inputBiases {
		model.inputBiases[hidden] = readI16()
	}
	for bucket := range model.outputWeights {
		for perspective := range model.outputWeights[bucket] {
			for hidden := range model.outputWeights[bucket][perspective] {
				model.outputWeights[bucket][perspective][hidden] = readI16()
			}
		}
	}
	for bucket := range model.outputBiases {
		model.outputBiases[bucket] = readI16()
	}
	if offset != payloadSize {
		return nil, fmt.Errorf("rodent v1.2 default model: decoded %d payload bytes, want %d", offset, payloadSize)
	}
	model.metadata = Metadata{
		SHA256:             digest,
		Bytes:              len(data),
		PayloadBytes:       payloadSize,
		TrailerBytes:       len(v12DefaultTrailer),
		InputBuckets:       InputBuckets,
		InputSize:          InputSize,
		TotalInputFeatures: TotalInputFeatures,
		HiddenSize:         HiddenSize,
		OutputBuckets:      OutputBuckets,
		Scale:              outputScale,
	}
	model.validated = true
	return model, nil
}
