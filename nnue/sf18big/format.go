package sf18big

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	fileVersion         uint32 = 0x7AF32F20
	networkHash         uint32 = 0xEC102EF2
	transformerHash     uint32 = 0x8F2344B8
	architectureHash    uint32 = 0x63336A4A
	baseInputFeatures          = 22528
	threatInputFeatures        = 79856
	transformerLanes           = 1024
	psqtBuckets                = 8
	layerStacks                = 8
	maxDescriptionBytes        = 1 << 20
	maxFileBytes               = 156266767
	readChunkBytes             = 64 << 10
)

const (
	architectureID  = "stockfish-18-big-1024x15x2-32-1"
	featureSetID    = "HalfKAv2_hm(Friend)-22528+Full_Threats(Friend)-79856"
	quantizationID  = "sf18-big-base-i16-threat-i8-psqt-i32-affine-i8"
	scoreContractID = "raw-psqt-positional-components-outputscale16"
)

var (
	ErrFormat      = errors.New("invalid Stockfish 18 BIG format")
	ErrUnsupported = errors.New("unsupported Stockfish NNUE format")
	ErrIntegrity   = errors.New("inconsistent Stockfish 18 BIG component hash")
	ErrRange       = errors.New("Stockfish 18 BIG parameter out of supported range")
)

const leb128Magic = "COMPRESSED_LEB128"

// RecognizesHeader reports whether prefix begins with the exact file-version
// and network-hash pair accepted by Load. It is intentionally only a dispatch
// hint: callers must still use Load for complete structural and integrity
// validation.
func RecognizesHeader(prefix []byte) bool {
	return len(prefix) >= 8 &&
		binary.LittleEndian.Uint32(prefix[0:4]) == fileVersion &&
		binary.LittleEndian.Uint32(prefix[4:8]) == networkHash
}

// Metadata is the immutable identity of one fully validated model file.
type Metadata struct {
	FileSHA256       [sha256.Size]byte
	FileSize         int64
	Version          uint32
	NetworkHash      uint32
	TransformerHash  uint32
	ArchitectureHash uint32
	Description      string
	ArchitectureID   string
	FeatureSetID     string
	QuantizationID   string
	ScoreContractID  string
}

type layerStack struct {
	fc0Bias   [16]int32
	fc0Weight [16 * transformerLanes]int8
	fc1Bias   [32]int32
	fc1Weight [32 * 32]int8
	fc2Bias   [1]int32
	fc2Weight [32]int8
}

// Model owns canonical logical-order tensors. All fields are private and are
// populated before publication, so a loaded Model is safe for concurrent reads.
type Model struct {
	loaded            bool
	metadata          Metadata
	featureBias       [transformerLanes]int16
	baseWeights       []int16
	threatWeights     []int8
	basePSQTWeights   []int32
	threatPSQTWeights []int32
	stacks            [layerStacks]layerStack
}

// Metadata returns a value copy of the model identity. Description is a Go
// string so callers cannot mutate the model's exact description bytes.
func (m *Model) Metadata() Metadata {
	if m == nil {
		return Metadata{}
	}
	return m.metadata
}

// Load decodes exactly one Stockfish 18 BIG model. It accepts the canonical
// output of Stockfish's signed-LEB writer and rejects trailing data. No partially
// initialized Model is returned on error.
func Load(reader io.Reader) (*Model, error) {
	if reader == nil {
		return nil, fmt.Errorf("%w: nil reader", ErrFormat)
	}

	guarded := &noProgressReader{reader: reader}
	limited := &io.LimitedReader{R: guarded, N: maxFileBytes + 1}
	digest := sha256.New()
	decoder := fileDecoder{
		reader: bufio.NewReaderSize(io.TeeReader(limited, digest), 64<<10),
	}

	model, err := decoder.decode()
	if err != nil {
		return nil, err
	}
	model.metadata.FileSize = maxFileBytes + 1 - limited.N
	if model.metadata.FileSize > maxFileBytes {
		return nil, fmt.Errorf("%w: file exceeds %d bytes", ErrFormat, maxFileBytes)
	}
	copy(model.metadata.FileSHA256[:], digest.Sum(nil))
	model.loaded = true
	return model, nil
}

type noProgressReader struct {
	reader     io.Reader
	emptyReads int
}

func (r *noProgressReader) Read(dst []byte) (int, error) {
	n, err := r.reader.Read(dst)
	if n == 0 && err == nil {
		r.emptyReads++
		if r.emptyReads >= 100 {
			return 0, io.ErrNoProgress
		}
	} else {
		r.emptyReads = 0
	}
	return n, err
}

type fileDecoder struct {
	reader io.Reader
}

func (d *fileDecoder) decode() (*Model, error) {
	version, err := d.readU32("file version")
	if err != nil {
		return nil, err
	}
	if version != fileVersion {
		return nil, fmt.Errorf("%w: version 0x%08x", ErrUnsupported, version)
	}
	topHash, err := d.readU32("network hash")
	if err != nil {
		return nil, err
	}
	if topHash != networkHash {
		return nil, fmt.Errorf("%w: network hash 0x%08x", ErrUnsupported, topHash)
	}
	descriptionSize, err := d.readU32("description length")
	if err != nil {
		return nil, err
	}
	if descriptionSize > maxDescriptionBytes {
		return nil, fmt.Errorf("%w: description is %d bytes, maximum %d", ErrFormat, descriptionSize, maxDescriptionBytes)
	}
	description := make([]byte, int(descriptionSize))
	if err := d.readFull(description, "description"); err != nil {
		return nil, err
	}

	componentHash, err := d.readU32("transformer hash")
	if err != nil {
		return nil, err
	}
	if componentHash != transformerHash {
		return nil, fmt.Errorf("%w: transformer hash 0x%08x", ErrIntegrity, componentHash)
	}

	candidate := &Model{
		baseWeights:       make([]int16, baseInputFeatures*transformerLanes),
		threatWeights:     make([]int8, threatInputFeatures*transformerLanes),
		basePSQTWeights:   make([]int32, baseInputFeatures*psqtBuckets),
		threatPSQTWeights: make([]int32, threatInputFeatures*psqtBuckets),
	}
	if err := d.readInt16Block("feature biases", candidate.featureBias[:]); err != nil {
		return nil, err
	}
	if err := d.readInt8s("threat feature weights", candidate.threatWeights); err != nil {
		return nil, err
	}
	if err := d.readInt16Block("base feature weights", candidate.baseWeights); err != nil {
		return nil, err
	}
	if err := d.readSplitInt32Block("combined PSQT weights", candidate.threatPSQTWeights, candidate.basePSQTWeights); err != nil {
		return nil, err
	}

	for i := range candidate.stacks {
		if err := d.readLayerStack(i, &candidate.stacks[i]); err != nil {
			return nil, err
		}
	}

	if err := d.requireEOF(); err != nil {
		return nil, err
	}
	candidate.metadata = Metadata{
		Version:          fileVersion,
		NetworkHash:      networkHash,
		TransformerHash:  transformerHash,
		ArchitectureHash: architectureHash,
		Description:      string(description),
		ArchitectureID:   architectureID,
		FeatureSetID:     featureSetID,
		QuantizationID:   quantizationID,
		ScoreContractID:  scoreContractID,
	}
	return candidate, nil
}

func (d *fileDecoder) readInt16Block(label string, dst []int16) error {
	return d.readSLEBBlock(label, len(dst), 16, func(index int, value int64) error {
		dst[index] = int16(value)
		return nil
	})
}

func (d *fileDecoder) readSplitInt32Block(label string, first, second []int32) error {
	count := len(first) + len(second)
	return d.readSLEBBlock(label, count, 32, func(index int, value int64) error {
		if index < len(first) {
			first[index] = int32(value)
		} else {
			second[index-len(first)] = int32(value)
		}
		return nil
	})
}

func (d *fileDecoder) readLayerStack(index int, stack *layerStack) error {
	if stack == nil {
		return fmt.Errorf("%w: nil layer stack %d", ErrFormat, index)
	}
	gotHash, err := d.readU32(fmt.Sprintf("layer stack %d hash", index))
	if err != nil {
		return err
	}
	if gotHash != architectureHash {
		return fmt.Errorf("%w: layer stack %d hash 0x%08x", ErrIntegrity, index, gotHash)
	}
	if err := d.readInt32s(fmt.Sprintf("layer stack %d FC0 biases", index), stack.fc0Bias[:]); err != nil {
		return err
	}
	if err := d.readInt8s(fmt.Sprintf("layer stack %d FC0 weights", index), stack.fc0Weight[:]); err != nil {
		return err
	}
	if err := d.readInt32s(fmt.Sprintf("layer stack %d FC1 biases", index), stack.fc1Bias[:]); err != nil {
		return err
	}
	if err := d.readInt8s(fmt.Sprintf("layer stack %d FC1 weights", index), stack.fc1Weight[:]); err != nil {
		return err
	}
	if err := d.readInt32s(fmt.Sprintf("layer stack %d FC2 bias", index), stack.fc2Bias[:]); err != nil {
		return err
	}
	return d.readInt8s(fmt.Sprintf("layer stack %d FC2 weights", index), stack.fc2Weight[:])
}

func (d *fileDecoder) readSLEBBlock(label string, count, bits int, store func(int, int64) error) error {
	magic := make([]byte, len(leb128Magic))
	if err := d.readFull(magic, label+" magic"); err != nil {
		return err
	}
	if !bytes.Equal(magic, []byte(leb128Magic)) {
		return fmt.Errorf("%w: %s magic", ErrFormat, label)
	}
	encodedSize, err := d.readU32(label + " encoded length")
	if err != nil {
		return err
	}
	maxTokenBytes := 3
	if bits == 32 {
		maxTokenBytes = 5
	}
	if uint64(encodedSize) < uint64(count) || uint64(encodedSize) > uint64(count)*uint64(maxTokenBytes) {
		return fmt.Errorf("%w: %s encoded length %d outside [%d,%d]", ErrFormat, label, encodedSize, count, count*maxTokenBytes)
	}

	remaining := int64(encodedSize)
	for index := 0; index < count; index++ {
		var token [5]byte
		tokenLength := 0
		var raw uint64
		var shift uint
		for {
			if remaining == 0 {
				return fmt.Errorf("%w: %s ended before value %d", ErrFormat, label, index)
			}
			if tokenLength == maxTokenBytes {
				return fmt.Errorf("%w: %s value %d exceeds %d-byte encoding", ErrFormat, label, index, maxTokenBytes)
			}
			valueByte, err := d.readByte(label + " data")
			if err != nil {
				return err
			}
			remaining--
			token[tokenLength] = valueByte
			tokenLength++
			raw |= uint64(valueByte&0x7f) << shift
			shift += 7
			if valueByte&0x80 == 0 {
				if valueByte&0x40 != 0 {
					raw |= ^uint64(0) << shift
				}
				break
			}
		}
		value := int64(raw)
		minimum := -(int64(1) << (bits - 1))
		maximum := (int64(1) << (bits - 1)) - 1
		if value < minimum || value > maximum {
			return fmt.Errorf("%w: %s value %d at index %d exceeds int%d", ErrRange, label, value, index, bits)
		}
		var canonical [5]byte
		canonicalLength := encodeSLEB(value, canonical[:])
		if canonicalLength != tokenLength || !bytes.Equal(canonical[:canonicalLength], token[:tokenLength]) {
			return fmt.Errorf("%w: %s value %d at index %d is not canonical SLEB", ErrFormat, label, value, index)
		}
		if err := store(index, value); err != nil {
			return err
		}
	}
	if remaining != 0 {
		return fmt.Errorf("%w: %s has %d unused encoded bytes", ErrFormat, label, remaining)
	}
	return nil
}

func encodeSLEB(value int64, dst []byte) int {
	for index := 0; ; index++ {
		part := byte(value & 0x7f)
		value >>= 7
		done := (value == 0 && part&0x40 == 0) || (value == -1 && part&0x40 != 0)
		if !done {
			part |= 0x80
		}
		dst[index] = part
		if done {
			return index + 1
		}
	}
}

func (d *fileDecoder) readInt32s(label string, dst []int32) error {
	var encoded [4]byte
	for i := range dst {
		if err := d.readFull(encoded[:], fmt.Sprintf("%s[%d]", label, i)); err != nil {
			return err
		}
		dst[i] = int32(binary.LittleEndian.Uint32(encoded[:]))
	}
	return nil
}

func (d *fileDecoder) readInt8s(label string, dst []int8) error {
	var encoded [readChunkBytes]byte
	for offset := 0; offset < len(dst); {
		length := len(dst) - offset
		if length > len(encoded) {
			length = len(encoded)
		}
		if err := d.readFull(encoded[:length], fmt.Sprintf("%s[%d:%d]", label, offset, offset+length)); err != nil {
			return err
		}
		for i, value := range encoded[:length] {
			dst[offset+i] = int8(value)
		}
		offset += length
	}
	return nil
}

func (d *fileDecoder) readU32(label string) (uint32, error) {
	var encoded [4]byte
	if err := d.readFull(encoded[:], label); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(encoded[:]), nil
}

func (d *fileDecoder) readByte(label string) (byte, error) {
	var encoded [1]byte
	if err := d.readFull(encoded[:], label); err != nil {
		return 0, err
	}
	return encoded[0], nil
}

func (d *fileDecoder) readFull(dst []byte, label string) error {
	if _, err := io.ReadFull(d.reader, dst); err != nil {
		return fmt.Errorf("%w: %s: %w", ErrFormat, label, err)
	}
	return nil
}

func (d *fileDecoder) requireEOF() error {
	var trailing [1]byte
	n, err := io.ReadFull(d.reader, trailing[:])
	if n != 0 {
		return fmt.Errorf("%w: trailing data", ErrFormat)
	}
	if errors.Is(err, io.EOF) {
		return nil
	}
	return fmt.Errorf("%w: final EOF: %w", ErrFormat, err)
}
