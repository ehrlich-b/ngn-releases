package rodenteval

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

var v11AnandTrailer = []byte("bulletbulletbulletbulletbulletbulletbulletbulletbulletbulletbu")

// LoadV11Anand loads only the exact external Rodent V1.1 Anand artifact.
// Failure returns no partially initialized model.
func LoadV11Anand(path string) (*Model, error) {
	if path == "" {
		return nil, fmt.Errorf("rodent v1.1 Anand model: empty path")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("rodent v1.1 Anand model: open: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, fileSize+1))
	if err != nil {
		return nil, fmt.Errorf("rodent v1.1 Anand model: read: %w", err)
	}
	return loadV11AnandBytes(data)
}

func loadV11AnandBytes(data []byte) (*Model, error) {
	if len(data) != fileSize {
		return nil, fmt.Errorf("rodent v1.1 Anand model: size %d, want %d", len(data), fileSize)
	}
	if len(v11AnandTrailer) != fileSize-payloadSize ||
		!bytes.Equal(data[payloadSize:], v11AnandTrailer) {
		return nil, fmt.Errorf("rodent v1.1 Anand model: invalid Bullet trailer")
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(data))
	if digest != V11AnandSHA256 {
		return nil, fmt.Errorf("rodent v1.1 Anand model: SHA-256 %s, want %s", digest, V11AnandSHA256)
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
	for row := range model.outputWeights {
		for hidden := range model.outputWeights[row] {
			model.outputWeights[row][hidden] = readI16()
		}
	}
	model.outputBias = readI16()
	if offset != payloadSize {
		return nil, fmt.Errorf("rodent v1.1 Anand model: decoded %d payload bytes, want %d", offset, payloadSize)
	}
	model.metadata = Metadata{
		SHA256:       digest,
		Bytes:        len(data),
		PayloadBytes: payloadSize,
		TrailerBytes: len(v11AnandTrailer),
		InputSize:    InputSize,
		HiddenSize:   HiddenSize,
		Scale:        outputScale,
	}
	model.validated = true
	return model, nil
}
