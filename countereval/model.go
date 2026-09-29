// Package countereval implements the portable full-refresh evaluator used by
// the Counter 5.5 768x512x1 legacy network format.
//
// The format and arithmetic are derived from CounterGo commit
// 63c487ca724c620f71c129d62129c6fb9109c872. This package owns strict loading,
// portable full refresh, and worker-local incremental contexts. Engine search
// selection and the historical score adapter remain explicit engine concerns;
// dispatched output kernels preserve the portable product and ordered-sum
// compatibility surface exactly.
package countereval

import (
	"bytes"
	"fmt"
)

const (
	LegacyHeaderSize  = 24
	LegacyFileSize    = 1_576_988
	InputSize         = 768
	HiddenSize        = 512
	FeaturePlaneCount = 12
	payloadValues     = InputSize*HiddenSize + HiddenSize + HiddenSize + 1
)

var legacyHeader = [LegacyHeaderSize]byte{
	0x42, 0x5a, 0x02, 0x00,
	0x01, 0x00, 0x00, 0x00,
	0x00, 0x03, 0x00, 0x00,
	0x01, 0x00, 0x00, 0x00,
	0x01, 0x00, 0x00, 0x00,
	0x00, 0x02, 0x00, 0x00,
}

// RecognizesLegacyHeader reports whether prefix carries the exact Counter 5.5
// legacy identifier. Full size, tensor and arithmetic validation remains the
// strict loader's responsibility.
func RecognizesLegacyHeader(prefix []byte) bool {
	return len(prefix) >= LegacyHeaderSize && bytes.Equal(prefix[:LegacyHeaderSize], legacyHeader[:])
}

// Board stores piece occupancy in Counter's feature-plane order: White
// pawn, knight, bishop, rook, queen, king, then the corresponding Black
// planes. A square may occur in at most one plane.
type Board [FeaturePlaneCount]uint64

// LoadMetadata binds the exact input and the conservative arithmetic bounds
// checked by LoadCounter55Legacy.
type LoadMetadata struct {
	SHA256              string
	Bytes               int
	Values              int
	MaxAccumulatorBound float64
	MaxProductBound     float64
	OutputBound         float64
}

// Model is an immutable Counter 5.5 legacy 768x512x1 network.
type Model struct {
	hiddenWeights [InputSize * HiddenSize]float32
	hiddenBiases  [HiddenSize]float32
	outputWeights [HiddenSize]float32
	outputBias    float32

	metadata           LoadMetadata
	maxAbsUpdateWeight [HiddenSize]float64
	validated          bool
}

// ValidatedMetadata returns the immutable identity and arithmetic bounds created
// by the strict legacy loader. A zero-value or otherwise unvalidated model is
// rejected before it can be selected for search.
func (model *Model) ValidatedMetadata() (LoadMetadata, error) {
	if model == nil || !model.validated || model.metadata.SHA256 == "" {
		return LoadMetadata{}, fmt.Errorf("counter legacy model: model is not loader-validated")
	}
	return model.metadata, nil
}
