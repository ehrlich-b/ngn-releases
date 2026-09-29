package countereval

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math"
)

const arithmeticBoundLimit = float64(math.MaxFloat32) / 4

// LoadCounter55Legacy reads one exact Counter 5.5 768x512x1 legacy file.
// Artifact identity is deliberately separate from format compatibility: the
// caller can compare metadata.SHA256 with its experiment manifest.
func LoadCounter55Legacy(r io.Reader) (*Model, LoadMetadata, error) {
	if r == nil {
		return nil, LoadMetadata{}, fmt.Errorf("counter legacy model: nil reader")
	}

	data, err := io.ReadAll(io.LimitReader(r, LegacyFileSize+1))
	if err != nil {
		return nil, LoadMetadata{}, fmt.Errorf("counter legacy model: read: %w", err)
	}
	if len(data) != LegacyFileSize {
		return nil, LoadMetadata{}, fmt.Errorf("counter legacy model: size %d, want %d", len(data), LegacyFileSize)
	}
	if !bytes.Equal(data[:LegacyHeaderSize], legacyHeader[:]) {
		return nil, LoadMetadata{}, fmt.Errorf("counter legacy model: unsupported 24-byte header")
	}

	model := new(Model)
	offset := LegacyHeaderSize
	read := func(section string, index int) (float32, error) {
		bits := binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4
		value := math.Float32frombits(bits)
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return 0, fmt.Errorf("counter legacy model: %s[%d] is nonfinite", section, index)
		}
		return value, nil
	}

	for i := range model.hiddenWeights {
		model.hiddenWeights[i], err = read("hidden_weights", i)
		if err != nil {
			return nil, LoadMetadata{}, err
		}
	}
	for i := range model.hiddenBiases {
		model.hiddenBiases[i], err = read("hidden_biases", i)
		if err != nil {
			return nil, LoadMetadata{}, err
		}
	}
	for i := range model.outputWeights {
		model.outputWeights[i], err = read("output_weights", i)
		if err != nil {
			return nil, LoadMetadata{}, err
		}
	}
	model.outputBias, err = read("output_bias", 0)
	if err != nil {
		return nil, LoadMetadata{}, err
	}
	if offset != len(data) {
		return nil, LoadMetadata{}, fmt.Errorf("counter legacy model: internal payload count consumed %d of %d bytes", offset, len(data))
	}

	metadata, err := validateArithmeticBounds(model)
	if err != nil {
		return nil, LoadMetadata{}, err
	}
	digest := sha256.Sum256(data)
	metadata.SHA256 = hex.EncodeToString(digest[:])
	metadata.Bytes = len(data)
	metadata.Values = payloadValues
	for hidden := 0; hidden < HiddenSize; hidden++ {
		for feature := 0; feature < InputSize; feature++ {
			weight := math.Abs(float64(model.hiddenWeights[feature*HiddenSize+hidden]))
			if weight > model.maxAbsUpdateWeight[hidden] {
				model.maxAbsUpdateWeight[hidden] = weight
			}
		}
	}
	model.metadata = metadata
	model.validated = true
	return model, metadata, nil
}

func validateArithmeticBounds(model *Model) (LoadMetadata, error) {
	var laneBounds [HiddenSize]float64
	var metadata LoadMetadata

	for hidden := 0; hidden < HiddenSize; hidden++ {
		bound := math.Abs(float64(model.hiddenBiases[hidden]))
		for square := 0; square < 64; square++ {
			maxAtSquare := 0.0
			for plane := 0; plane < FeaturePlaneCount; plane++ {
				feature := plane*64 + square
				weight := math.Abs(float64(model.hiddenWeights[feature*HiddenSize+hidden]))
				if weight > maxAtSquare {
					maxAtSquare = weight
				}
			}
			bound += maxAtSquare
		}
		if !finiteAndWithinMargin(bound) {
			return LoadMetadata{}, fmt.Errorf("counter legacy model: accumulator bound lane %d is unsafe: %g", hidden, bound)
		}
		laneBounds[hidden] = bound
		if bound > metadata.MaxAccumulatorBound {
			metadata.MaxAccumulatorBound = bound
		}
	}

	outputBound := math.Abs(float64(model.outputBias))
	if !finiteAndWithinMargin(outputBound) {
		return LoadMetadata{}, fmt.Errorf("counter legacy model: output bias bound is unsafe: %g", outputBound)
	}
	for hidden, laneBound := range laneBounds {
		productBound := laneBound * math.Abs(float64(model.outputWeights[hidden]))
		if !finiteAndWithinMargin(productBound) {
			return LoadMetadata{}, fmt.Errorf("counter legacy model: output product bound lane %d is unsafe: %g", hidden, productBound)
		}
		if productBound > metadata.MaxProductBound {
			metadata.MaxProductBound = productBound
		}
		outputBound += productBound
		if !finiteAndWithinMargin(outputBound) {
			return LoadMetadata{}, fmt.Errorf("counter legacy model: cumulative output bound after lane %d is unsafe: %g", hidden, outputBound)
		}
	}
	metadata.OutputBound = outputBound
	return metadata, nil
}

func finiteAndWithinMargin(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value <= arithmeticBoundLimit
}
