package nnue

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	InputSize        = 768
	HiddenSize       = 128
	PerspectiveCount = 2
	OutputSize       = 1

	QA         = 255
	QB         = 64
	ScoreScale = 400

	HeaderSize  = 128
	PayloadSize = 197378
	FileSize    = HeaderSize + PayloadSize

	// The public Position contract permits at most 64 occupied squares. With
	// int16 feature bias and weights, every accumulator lane stays in this range.
	MinAccumulatorValue int32 = -2129920 // -32768 bias + 64*-32768 weights.
	MaxAccumulatorValue int32 = 2129855  // 32767 bias + 64*32767 weights.
)

const (
	FormatVersion   uint32 = 1
	ArchitectureID  uint32 = 1 // Chess768x2-128-SCReLU-1.
	FeatureID       uint32 = 1 // Chess768, colour relative, vertical flip.
	QuantizationID  uint32 = 1 // i16, QA=255, QB=64, two truncating divisions.
	ScoreContractID uint32 = 1 // Raw side-to-move centipawns.
)

var (
	ErrFormat       = errors.New("invalid NGN NNUE format")
	ErrIntegrity    = errors.New("NGN NNUE integrity failure")
	ErrQuantization = errors.New("invalid NNUE quantization input")
)

var fileMagic = [8]byte{'N', 'G', 'N', 'N', 'U', 'E', 0, 0}

// RecognizesHeader reports whether prefix carries the exact NGN-v1 format
// identifier. Full structural and integrity validation remains Load's job.
func RecognizesHeader(prefix []byte) bool {
	return len(prefix) >= len(fileMagic) && string(prefix[:len(fileMagic)]) == string(fileMagic[:])
}

const (
	offsetFormatVersion = 8
	offsetHeaderSize    = 12
	offsetArchitecture  = 16
	offsetFeature       = 20
	offsetQuantization  = 24
	offsetScoreContract = 28
	offsetInputSize     = 32
	offsetHiddenSize    = 36
	offsetPerspectives  = 40
	offsetOutputSize    = 44
	offsetQA            = 48
	offsetQB            = 52
	offsetScale         = 56
	offsetPayloadSize   = 60
	offsetPayloadDigest = 68
	offsetReserved      = 100
)

const (
	featureWeightsBytes = InputSize * HiddenSize * 2
	featureBiasOffset   = featureWeightsBytes
	outputWeightsOffset = featureBiasOffset + HiddenSize*2
	outputBiasOffset    = outputWeightsOffset + PerspectiveCount*HiddenSize*2
)

// Tensors is the canonical, unpacked NGN v1 parameter order. Marshal copies
// these values into an immutable on-disk model.
type Tensors struct {
	FeatureWeights [InputSize][HiddenSize]int16
	FeatureBias    [HiddenSize]int16
	OutputWeights  [PerspectiveCount * HiddenSize]int16
	OutputBias     int16
}

// Metadata identifies both the tensor payload and the complete model file.
type Metadata struct {
	FormatVersion   uint32
	ArchitectureID  uint32
	FeatureID       uint32
	QuantizationID  uint32
	ScoreContractID uint32
	PayloadSHA256   [sha256.Size]byte
	FileSHA256      [sha256.Size]byte
}

// Capabilities are immutable facts derived while loading the exact model.
// PortableInt32Accumulator follows from the strict NGN-v1 tensor and Position
// bounds. BoundedInt32Output is the separate, model-specific precondition for
// the exact packed output kernel; models outside it retain int64 evaluation.
type Capabilities struct {
	PortableInt32Accumulator bool
	BoundedInt32Output       bool
	AccumulatorMin           int32
	AccumulatorMax           int32
}

// Model is immutable after loading. Its arrays are intentionally unexported.
type Model struct {
	featureWeights [InputSize][HiddenSize]int16
	featureBias    [HiddenSize]int16
	outputWeights  [PerspectiveCount * HiddenSize]int16
	outputBias     int16
	metadata       Metadata
	capabilities   Capabilities
	loaded         bool
}

// Metadata returns a copy of the model identity.
func (m *Model) Metadata() Metadata {
	return m.metadata
}

// Capabilities returns a copy of the checked arithmetic capabilities.
func (m *Model) Capabilities() Capabilities {
	if m == nil {
		return Capabilities{}
	}
	return m.capabilities
}

// Marshal encodes tensors in the strict NGN v1 container. It emits no padding.
func Marshal(tensors *Tensors) ([]byte, error) {
	if tensors == nil {
		return nil, fmt.Errorf("%w: nil tensors", ErrFormat)
	}

	payload := make([]byte, PayloadSize)
	at := 0
	for feature := 0; feature < InputSize; feature++ {
		for hidden := 0; hidden < HiddenSize; hidden++ {
			binary.LittleEndian.PutUint16(payload[at:], uint16(tensors.FeatureWeights[feature][hidden]))
			at += 2
		}
	}
	for _, value := range tensors.FeatureBias {
		binary.LittleEndian.PutUint16(payload[at:], uint16(value))
		at += 2
	}
	for _, value := range tensors.OutputWeights {
		binary.LittleEndian.PutUint16(payload[at:], uint16(value))
		at += 2
	}
	binary.LittleEndian.PutUint16(payload[at:], uint16(tensors.OutputBias))
	at += 2
	if at != PayloadSize {
		panic("nnue: internal payload size mismatch")
	}

	payloadDigest := sha256.Sum256(payload)
	encoded := make([]byte, FileSize)
	copy(encoded[:len(fileMagic)], fileMagic[:])
	putHeaderU32(encoded, offsetFormatVersion, FormatVersion)
	putHeaderU32(encoded, offsetHeaderSize, HeaderSize)
	putHeaderU32(encoded, offsetArchitecture, ArchitectureID)
	putHeaderU32(encoded, offsetFeature, FeatureID)
	putHeaderU32(encoded, offsetQuantization, QuantizationID)
	putHeaderU32(encoded, offsetScoreContract, ScoreContractID)
	putHeaderU32(encoded, offsetInputSize, InputSize)
	putHeaderU32(encoded, offsetHiddenSize, HiddenSize)
	putHeaderU32(encoded, offsetPerspectives, PerspectiveCount)
	putHeaderU32(encoded, offsetOutputSize, OutputSize)
	putHeaderU32(encoded, offsetQA, QA)
	putHeaderU32(encoded, offsetQB, QB)
	putHeaderU32(encoded, offsetScale, ScoreScale)
	binary.LittleEndian.PutUint64(encoded[offsetPayloadSize:], PayloadSize)
	copy(encoded[offsetPayloadDigest:offsetReserved], payloadDigest[:])
	copy(encoded[HeaderSize:], payload)
	return encoded, nil
}

// Load reads exactly one strict NGN v1 model. Trailing data is rejected.
func Load(reader io.Reader) (*Model, error) {
	if reader == nil {
		return nil, fmt.Errorf("%w: nil reader", ErrFormat)
	}

	header := make([]byte, HeaderSize)
	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, fmt.Errorf("%w: read header: %v", ErrFormat, err)
	}
	if err := validateHeader(header); err != nil {
		return nil, err
	}

	payload := make([]byte, PayloadSize)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, fmt.Errorf("%w: read payload: %v", ErrFormat, err)
	}
	var extra [1]byte
	n, err := io.ReadFull(reader, extra[:])
	if n != 0 {
		return nil, fmt.Errorf("%w: trailing data", ErrFormat)
	}
	if err != io.EOF {
		return nil, fmt.Errorf("%w: check trailing data: %v", ErrFormat, err)
	}

	payloadDigest := sha256.Sum256(payload)
	if !equalDigest(payloadDigest, header[offsetPayloadDigest:offsetReserved]) {
		return nil, fmt.Errorf("%w: payload SHA256 mismatch", ErrIntegrity)
	}

	model := new(Model)
	at := 0
	for feature := 0; feature < InputSize; feature++ {
		for hidden := 0; hidden < HiddenSize; hidden++ {
			model.featureWeights[feature][hidden] = int16(binary.LittleEndian.Uint16(payload[at:]))
			at += 2
		}
	}
	for hidden := 0; hidden < HiddenSize; hidden++ {
		model.featureBias[hidden] = int16(binary.LittleEndian.Uint16(payload[at:]))
		at += 2
	}
	for output := 0; output < PerspectiveCount*HiddenSize; output++ {
		model.outputWeights[output] = int16(binary.LittleEndian.Uint16(payload[at:]))
		at += 2
	}
	model.outputBias = int16(binary.LittleEndian.Uint16(payload[at:]))
	at += 2
	if at != PayloadSize {
		panic("nnue: internal decoder size mismatch")
	}

	fileHash := sha256.New()
	_, _ = fileHash.Write(header)
	_, _ = fileHash.Write(payload)
	model.metadata = Metadata{
		FormatVersion:   FormatVersion,
		ArchitectureID:  ArchitectureID,
		FeatureID:       FeatureID,
		QuantizationID:  QuantizationID,
		ScoreContractID: ScoreContractID,
		PayloadSHA256:   payloadDigest,
	}
	copy(model.metadata.FileSHA256[:], fileHash.Sum(nil))
	boundedOutput := true
	for _, weight := range model.outputWeights {
		if weight < -128 || weight > 128 {
			boundedOutput = false
			break
		}
	}
	model.capabilities = Capabilities{
		PortableInt32Accumulator: true,
		BoundedInt32Output:       boundedOutput,
		AccumulatorMin:           MinAccumulatorValue,
		AccumulatorMax:           MaxAccumulatorValue,
	}
	model.loaded = true
	return model, nil
}

func validateHeader(header []byte) error {
	if len(header) != HeaderSize {
		return fmt.Errorf("%w: header length %d", ErrFormat, len(header))
	}
	if string(header[:len(fileMagic)]) != string(fileMagic[:]) {
		return fmt.Errorf("%w: magic", ErrFormat)
	}
	checks := []struct {
		name   string
		offset int
		want   uint32
	}{
		{"format version", offsetFormatVersion, FormatVersion},
		{"header length", offsetHeaderSize, HeaderSize},
		{"architecture", offsetArchitecture, ArchitectureID},
		{"feature mapping", offsetFeature, FeatureID},
		{"quantization", offsetQuantization, QuantizationID},
		{"score contract", offsetScoreContract, ScoreContractID},
		{"input width", offsetInputSize, InputSize},
		{"hidden width", offsetHiddenSize, HiddenSize},
		{"perspective count", offsetPerspectives, PerspectiveCount},
		{"output width", offsetOutputSize, OutputSize},
		{"QA", offsetQA, QA},
		{"QB", offsetQB, QB},
		{"scale", offsetScale, ScoreScale},
	}
	for _, check := range checks {
		got := binary.LittleEndian.Uint32(header[check.offset:])
		if got != check.want {
			return fmt.Errorf("%w: %s is %d, want %d", ErrFormat, check.name, got, check.want)
		}
	}
	gotPayloadSize := binary.LittleEndian.Uint64(header[offsetPayloadSize:])
	if gotPayloadSize != PayloadSize {
		return fmt.Errorf("%w: payload length is %d, want %d", ErrFormat, gotPayloadSize, PayloadSize)
	}
	for _, value := range header[offsetReserved:HeaderSize] {
		if value != 0 {
			return fmt.Errorf("%w: reserved bytes must be zero", ErrFormat)
		}
	}
	return nil
}

func putHeaderU32(header []byte, offset int, value uint32) {
	binary.LittleEndian.PutUint32(header[offset:], value)
}

func equalDigest(got [sha256.Size]byte, want []byte) bool {
	if len(want) != sha256.Size {
		return false
	}
	var mismatch byte
	for i := range got {
		mismatch |= got[i] ^ want[i]
	}
	return mismatch == 0
}
