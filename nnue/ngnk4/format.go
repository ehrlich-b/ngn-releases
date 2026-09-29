package ngnk4

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
)

var (
	ErrFormat      = errors.New("invalid NGN K4 NNUE format")
	ErrUnsupported = errors.New("unsupported NGN K4 NNUE contract")
	ErrIntegrity   = errors.New("NGN K4 NNUE integrity failure")
)

var fileMagic = [8]byte{'N', 'G', 'N', 'K', '4', 'V', '1', 0}

const (
	fileVersion      uint16 = 1
	architectureCode uint32 = 1
	featureSetCode   uint32 = 1
	quantizationCode uint32 = 1
	scorePolicyCode  uint32 = 1
)

// RecognizesHeader reports whether prefix starts with the exact magic and
// version accepted by Load. It is only a dispatch hint; Load remains mandatory.
func RecognizesHeader(prefix []byte) bool {
	return len(prefix) >= 10 && bytes.Equal(prefix[:8], fileMagic[:]) &&
		binary.LittleEndian.Uint16(prefix[8:10]) == fileVersion
}

// LoadFile opens and strictly decodes one NGN-owned model.
func LoadFile(path string) (*Model, error) {
	if path == "" {
		return nil, fmt.Errorf("%w: empty path", ErrFormat)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("NGN K4 NNUE: open: %w", err)
	}
	defer file.Close()
	return Load(file)
}

// Load accepts exactly one bounded, checksummed ngn-k4-768-v1 file. Failure
// never returns a partially initialized model.
func Load(reader io.Reader) (*Model, error) {
	if reader == nil {
		return nil, fmt.Errorf("%w: nil reader", ErrFormat)
	}
	data, err := io.ReadAll(io.LimitReader(&noProgressReader{reader: reader}, FileSize+1))
	if err != nil {
		return nil, fmt.Errorf("%w: read: %v", ErrFormat, err)
	}
	if len(data) != FileSize {
		return nil, fmt.Errorf("%w: file size %d, want %d", ErrFormat, len(data), FileSize)
	}
	header := data[:HeaderSize]
	payload := data[HeaderSize:]
	if !bytes.Equal(header[:8], fileMagic[:]) {
		return nil, fmt.Errorf("%w: magic %x", ErrUnsupported, header[:8])
	}
	version := binary.LittleEndian.Uint16(header[8:10])
	if version != fileVersion {
		return nil, fmt.Errorf("%w: version %d", ErrUnsupported, version)
	}
	if got := binary.LittleEndian.Uint16(header[10:12]); got != HeaderSize {
		return nil, fmt.Errorf("%w: header bytes %d", ErrUnsupported, got)
	}
	checks := []struct {
		name string
		got  uint32
		want uint32
	}{
		{"architecture", binary.LittleEndian.Uint32(header[12:16]), architectureCode},
		{"feature set", binary.LittleEndian.Uint32(header[16:20]), featureSetCode},
		{"quantization", binary.LittleEndian.Uint32(header[20:24]), quantizationCode},
		{"score policy", binary.LittleEndian.Uint32(header[24:28]), scorePolicyCode},
		{"input buckets", binary.LittleEndian.Uint32(header[28:32]), InputBuckets},
		{"input size", binary.LittleEndian.Uint32(header[32:36]), InputSize},
		{"hidden size", binary.LittleEndian.Uint32(header[36:40]), HiddenSize},
		{"output buckets", binary.LittleEndian.Uint32(header[40:44]), OutputBuckets},
		{"input scale", binary.LittleEndian.Uint32(header[44:48]), InputScale},
		{"layer scale", binary.LittleEndian.Uint32(header[48:52]), LayerScale},
		{"output scale", binary.LittleEndian.Uint32(header[52:56]), OutputScale},
	}
	for _, check := range checks {
		if check.got != check.want {
			return nil, fmt.Errorf("%w: %s %d, want %d", ErrUnsupported, check.name, check.got, check.want)
		}
	}
	payloadBytes := binary.LittleEndian.Uint64(header[56:64])
	if payloadBytes != PayloadSize {
		return nil, fmt.Errorf("%w: payload bytes %d, want %d", ErrUnsupported, payloadBytes, PayloadSize)
	}
	var manifestSHA, declaredPayloadSHA [sha256.Size]byte
	copy(manifestSHA[:], header[64:96])
	copy(declaredPayloadSHA[:], header[96:128])
	if manifestSHA == ([sha256.Size]byte{}) {
		return nil, fmt.Errorf("%w: zero training-manifest SHA-256", ErrIntegrity)
	}
	if !allZero(header[128:]) {
		return nil, fmt.Errorf("%w: nonzero reserved header bytes", ErrUnsupported)
	}
	payloadSHA := sha256.Sum256(payload)
	if payloadSHA != declaredPayloadSHA {
		return nil, fmt.Errorf("%w: payload SHA-256 %x, want %x", ErrIntegrity, payloadSHA, declaredPayloadSHA)
	}

	model := new(Model)
	offset := 0
	readI16 := func() int16 {
		value := int16(binary.LittleEndian.Uint16(payload[offset : offset+2]))
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
	if offset != PayloadSize {
		return nil, fmt.Errorf("%w: decoded %d payload bytes, want %d", ErrFormat, offset, PayloadSize)
	}

	if err := model.checkAccumulatorBound(); err != nil {
		return nil, err
	}

	var fastSafe [OutputBuckets]bool
	for bucket := range model.outputWeights {
		var absoluteWeightSum uint64
		for perspective := range model.outputWeights[bucket] {
			for _, weight := range model.outputWeights[bucket][perspective] {
				value := int64(weight)
				if value < 0 {
					value = -value
				}
				absoluteWeightSum += uint64(value)
			}
		}
		fastSafe[bucket] = uint64(InputScale*InputScale)*absoluteWeightSum <= math.MaxInt32
	}
	model.metadata = Metadata{
		FileSHA256:      sha256.Sum256(data),
		ManifestSHA256:  manifestSHA,
		PayloadSHA256:   payloadSHA,
		FileBytes:       int64(len(data)),
		HeaderBytes:     HeaderSize,
		PayloadBytes:    payloadBytes,
		Version:         version,
		ArchitectureID:  ArchitectureID,
		FeatureSetID:    FeatureSetID,
		QuantizationID:  QuantizationID,
		ScoreContractID: ScoreContractID,
		InputBuckets:    InputBuckets,
		InputSize:       InputSize,
		HiddenSize:      HiddenSize,
		OutputBuckets:   OutputBuckets,
		InputScale:      InputScale,
		LayerScale:      LayerScale,
		OutputScale:     OutputScale,
		FastOutputSafe:  fastSafe,
	}
	model.loaded = true
	return model, nil
}

type noProgressReader struct {
	reader     io.Reader
	emptyReads int
}

func (reader *noProgressReader) Read(dst []byte) (int, error) {
	n, err := reader.reader.Read(dst)
	if n == 0 && err == nil {
		reader.emptyReads++
		if reader.emptyReads >= 100 {
			return 0, io.ErrNoProgress
		}
	} else {
		reader.emptyReads = 0
	}
	return n, err
}

func allZero(data []byte) bool {
	for _, value := range data {
		if value != 0 {
			return false
		}
	}
	return true
}

// maximumActiveFeatures is the legal-board piece bound that validatePosition
// enforces; every active feature of one perspective shares a king bucket.
const maximumActiveFeatures = 32

// checkAccumulatorBound proves that int16 accumulator lanes never differ from
// exact integer sums: for every bucket and hidden lane, |bias| plus the 32
// largest row magnitudes must fit in int16. Intermediate int16 wraparound is
// then harmless because addition and subtraction are exact modulo 2^16.
func (model *Model) checkAccumulatorBound() error {
	for bucket := 0; bucket < InputBuckets; bucket++ {
		for hidden := 0; hidden < HiddenSize; hidden++ {
			var largest [maximumActiveFeatures]int32
			for feature := bucket * InputSize; feature < (bucket+1)*InputSize; feature++ {
				magnitude := int32(model.inputWeights[feature][hidden])
				if magnitude < 0 {
					magnitude = -magnitude
				}
				if magnitude <= largest[maximumActiveFeatures-1] {
					continue
				}
				index := maximumActiveFeatures - 1
				for index > 0 && largest[index-1] < magnitude {
					largest[index] = largest[index-1]
					index--
				}
				largest[index] = magnitude
			}
			bound := int32(model.inputBiases[hidden])
			if bound < 0 {
				bound = -bound
			}
			for _, magnitude := range largest {
				bound += magnitude
			}
			if bound > math.MaxInt16 {
				return fmt.Errorf("%w: bucket %d hidden %d accumulator bound %d exceeds int16", ErrUnsupported, bucket, hidden, bound)
			}
		}
	}
	return nil
}
