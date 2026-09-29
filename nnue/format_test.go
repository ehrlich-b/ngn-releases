package nnue_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"testing"

	"github.com/ehrlich-b/ngn/nnue"
)

const (
	manualHeaderSize  = 128
	manualPayloadSize = 197378
)

func manualContainer(payload []byte) []byte {
	if len(payload) != manualPayloadSize {
		panic("bad manual payload size")
	}
	encoded := make([]byte, manualHeaderSize+manualPayloadSize)
	copy(encoded[:8], []byte{'N', 'G', 'N', 'N', 'U', 'E', 0, 0})
	fields := [...]uint32{
		1, 128, 1, 1, 1, 1, 768, 128, 2, 1, 255, 64, 400,
	}
	for i, value := range fields {
		binary.LittleEndian.PutUint32(encoded[8+4*i:], value)
	}
	binary.LittleEndian.PutUint64(encoded[60:], 197378)
	digest := sha256.Sum256(payload)
	copy(encoded[68:100], digest[:])
	copy(encoded[128:], payload)
	return encoded
}

func loadManual(t *testing.T, payload []byte) *nnue.Model {
	t.Helper()
	model, err := nnue.Load(bytes.NewReader(manualContainer(payload)))
	if err != nil {
		t.Fatalf("Load manual model: %v", err)
	}
	return model
}

func TestMarshalCanonicalHeaderAndTensorOrder(t *testing.T) {
	tensors := new(nnue.Tensors)
	tensors.FeatureWeights[0][0] = 0x1234
	tensors.FeatureWeights[13][57] = -0x123
	tensors.FeatureWeights[767][127] = -2
	tensors.FeatureBias[0] = 0x2345
	tensors.FeatureBias[73] = -0x234
	tensors.OutputWeights[0] = 0x3456
	tensors.OutputWeights[173] = -0x345
	tensors.OutputWeights[255] = -3
	tensors.OutputBias = -4

	first, err := nnue.Marshal(tensors)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	second, err := nnue.Marshal(tensors)
	if err != nil {
		t.Fatalf("second Marshal: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("Marshal is not deterministic")
	}
	if len(first) != 197506 {
		t.Fatalf("file length = %d, want 197506", len(first))
	}
	if got := first[:8]; !bytes.Equal(got, []byte{'N', 'G', 'N', 'N', 'U', 'E', 0, 0}) {
		t.Fatalf("magic = %x", got)
	}
	wantFields := [...]uint32{1, 128, 1, 1, 1, 1, 768, 128, 2, 1, 255, 64, 400}
	for i, want := range wantFields {
		if got := binary.LittleEndian.Uint32(first[8+4*i:]); got != want {
			t.Fatalf("header field %d = %d, want %d", i, got, want)
		}
	}
	if got := binary.LittleEndian.Uint64(first[60:]); got != 197378 {
		t.Fatalf("payload length = %d", got)
	}
	for i, value := range first[100:128] {
		if value != 0 {
			t.Fatalf("reserved byte %d = %d", i, value)
		}
	}

	payload := first[128:]
	checks := []struct {
		name   string
		offset int
		want   int16
	}{
		{"first feature weight", 0, 0x1234},
		{"interior feature weight", 3442, -0x123},
		{"last feature weight", 196606, -2},
		{"first feature bias", 196608, 0x2345},
		{"interior feature bias", 196754, -0x234},
		{"first output weight", 196864, 0x3456},
		{"interior output weight", 197210, -0x345},
		{"last output weight", 197374, -3},
		{"output bias", 197376, -4},
	}
	for _, check := range checks {
		got := int16(binary.LittleEndian.Uint16(payload[check.offset:]))
		if got != check.want {
			t.Errorf("%s = %d, want %d", check.name, got, check.want)
		}
	}
	payloadDigest := sha256.Sum256(payload)
	if !bytes.Equal(first[68:100], payloadDigest[:]) {
		t.Fatal("payload digest does not cover canonical payload")
	}

	model, err := nnue.Load(bytes.NewReader(first))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	metadata := model.Metadata()
	fileDigest := sha256.Sum256(first)
	if metadata.FileSHA256 != fileDigest {
		t.Fatalf("file digest = %x, want %x", metadata.FileSHA256, fileDigest)
	}
	if metadata.PayloadSHA256 != payloadDigest {
		t.Fatalf("payload digest = %x, want %x", metadata.PayloadSHA256, payloadDigest)
	}
}

func TestLoadRejectsMalformedContainers(t *testing.T) {
	base := manualContainer(make([]byte, manualPayloadSize))
	mutateU32 := func(offset int, value uint32) []byte {
		result := bytes.Clone(base)
		binary.LittleEndian.PutUint32(result[offset:], value)
		return result
	}
	mutateU64 := func(offset int, value uint64) []byte {
		result := bytes.Clone(base)
		binary.LittleEndian.PutUint64(result[offset:], value)
		return result
	}

	badMagic := bytes.Clone(base)
	badMagic[0] ^= 0xff
	bigEndianVersion := bytes.Clone(base)
	copy(bigEndianVersion[8:12], []byte{0, 0, 0, 1})
	reserved := bytes.Clone(base)
	reserved[127] = 1
	badChecksum := bytes.Clone(base)
	badChecksum[68] ^= 1
	badPayload := bytes.Clone(base)
	badPayload[len(badPayload)-1] ^= 1

	cases := []struct {
		name string
		data []byte
		want error
	}{
		{"short header", base[:127], nnue.ErrFormat},
		{"short payload", base[:len(base)-1], nnue.ErrFormat},
		{"extra byte", append(bytes.Clone(base), 0), nnue.ErrFormat},
		{"magic", badMagic, nnue.ErrFormat},
		{"big-endian version", bigEndianVersion, nnue.ErrFormat},
		{"format version", mutateU32(8, 2), nnue.ErrFormat},
		{"header length", mutateU32(12, 127), nnue.ErrFormat},
		{"architecture", mutateU32(16, 2), nnue.ErrFormat},
		{"feature", mutateU32(20, 2), nnue.ErrFormat},
		{"quantization", mutateU32(24, 2), nnue.ErrFormat},
		{"score contract", mutateU32(28, 2), nnue.ErrFormat},
		{"input width", mutateU32(32, 767), nnue.ErrFormat},
		{"hidden width", mutateU32(36, 127), nnue.ErrFormat},
		{"perspectives", mutateU32(40, 1), nnue.ErrFormat},
		{"output width", mutateU32(44, 2), nnue.ErrFormat},
		{"QA", mutateU32(48, 254), nnue.ErrFormat},
		{"QB", mutateU32(52, 63), nnue.ErrFormat},
		{"scale", mutateU32(56, 399), nnue.ErrFormat},
		{"zero payload length", mutateU64(60, 0), nnue.ErrFormat},
		{"overflow payload length", mutateU64(60, math.MaxUint64), nnue.ErrFormat},
		{"reserved", reserved, nnue.ErrFormat},
		{"checksum", badChecksum, nnue.ErrIntegrity},
		{"payload corruption", badPayload, nnue.ErrIntegrity},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := nnue.Load(bytes.NewReader(test.data))
			if !errors.Is(err, test.want) {
				t.Fatalf("Load error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestMarshalRejectsNil(t *testing.T) {
	if _, err := nnue.Marshal(nil); !errors.Is(err, nnue.ErrFormat) {
		t.Fatalf("Marshal(nil) error = %v", err)
	}
	if _, err := nnue.Load(nil); !errors.Is(err, nnue.ErrFormat) {
		t.Fatalf("Load(nil) error = %v", err)
	}
}

type intermittentReader struct {
	reader io.Reader
	zero   bool
}

func (r *intermittentReader) Read(buffer []byte) (int, error) {
	if !r.zero {
		r.zero = true
		return 0, nil
	}
	r.zero = false
	if len(buffer) > 7 {
		buffer = buffer[:7]
	}
	return r.reader.Read(buffer)
}

func TestLoadAcceptsChunkedReaderWithLegalZeroReads(t *testing.T) {
	encoded := manualContainer(make([]byte, manualPayloadSize))
	reader := &intermittentReader{reader: bytes.NewReader(encoded)}
	if _, err := nnue.Load(reader); err != nil {
		t.Fatalf("Load intermittent reader: %v", err)
	}
}

func TestRecognizesHeaderExactMagic(t *testing.T) {
	magic := []byte{'N', 'G', 'N', 'N', 'U', 'E', 0, 0}
	prefix := make([]byte, nnue.HeaderSize)
	copy(prefix, magic)
	if nnue.RecognizesHeader(prefix[:len(magic)-1]) {
		t.Fatal("short prefix recognized")
	}
	if !nnue.RecognizesHeader(prefix) {
		t.Fatal("exact magic not recognized")
	}
	prefix[3] ^= 1
	if nnue.RecognizesHeader(prefix) {
		t.Fatal("mutated magic recognized")
	}
}
