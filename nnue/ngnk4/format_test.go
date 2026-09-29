package ngnk4

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"testing"
)

const (
	inputBiasOffset  = TotalInputFeatures * HiddenSize * 2
	outputWeightBase = inputBiasOffset + HiddenSize*2
	outputBiasOffset = outputWeightBase + OutputBuckets*2*HiddenSize*2
)

func makeTestFile(t *testing.T, editPayload func([]byte)) []byte {
	t.Helper()
	payload := make([]byte, PayloadSize)
	if editPayload != nil {
		editPayload(payload)
	}
	header := make([]byte, HeaderSize)
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
	manifest := sha256.Sum256([]byte("ngn-k4 test training manifest"))
	copy(header[64:96], manifest[:])
	payloadSHA := sha256.Sum256(payload)
	copy(header[96:128], payloadSHA[:])
	return append(header, payload...)
}

func putI16(data []byte, offset int, value int16) {
	binary.LittleEndian.PutUint16(data[offset:offset+2], uint16(value))
}

func inputWeightOffset(feature, hidden int) int {
	return (feature*HiddenSize + hidden) * 2
}

func outputWeightOffset(bucket, perspective, hidden int) int {
	return outputWeightBase + ((bucket*2+perspective)*HiddenSize+hidden)*2
}

func TestStructuralContractAndHeaderRecognition(t *testing.T) {
	if InputBuckets != 4 || HiddenSize != 768 {
		t.Skip("frozen sizes cover only the default K4-768 build")
	}
	if HeaderSize != 256 || PayloadSize != 4_744_720 || FileSize != 4_744_976 {
		t.Fatalf("sizes = %d/%d/%d", HeaderSize, PayloadSize, FileSize)
	}
	if TotalInputFeatures != 3072 || HiddenSize != 768 || OutputBuckets != 8 {
		t.Fatal("architecture dimensions changed")
	}
	data := makeTestFile(t, nil)
	if !RecognizesHeader(data[:10]) {
		t.Fatal("valid header not recognized")
	}
	if RecognizesHeader(data[:9]) {
		t.Fatal("short header recognized")
	}
	data[8]++
	if RecognizesHeader(data[:10]) {
		t.Fatal("wrong version recognized")
	}
}

func TestLoadExactIdentityAndTensorOrder(t *testing.T) {
	data := makeTestFile(t, func(payload []byte) {
		putI16(payload, inputWeightOffset(17, 23), -1234)
		putI16(payload, inputBiasOffset+31*2, 2345)
		for hidden := 0; hidden < HiddenSize; hidden++ {
			putI16(payload, outputWeightOffset(6, 0, hidden), math.MaxInt16)
		}
		putI16(payload, outputWeightOffset(6, 1, 47), -3210)
		putI16(payload, outputBiasOffset+5*2, 4567)
	})
	model, err := Load(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if model.inputWeights[17][23] != -1234 || model.inputBiases[31] != 2345 ||
		model.outputWeights[6][1][47] != -3210 || model.outputBiases[5] != 4567 {
		t.Fatal("tensor payload order changed")
	}
	metadata := model.Metadata()
	if metadata.FileSHA256 != sha256.Sum256(data) || metadata.FileBytes != FileSize ||
		metadata.ArchitectureID != ArchitectureID || metadata.FeatureSetID != FeatureSetID ||
		metadata.QuantizationID != QuantizationID || metadata.ScoreContractID != ScoreContractID {
		t.Fatalf("metadata = %+v", metadata)
	}
	if metadata.ManifestSHA256 == ([sha256.Size]byte{}) || metadata.PayloadSHA256 != sha256.Sum256(data[HeaderSize:]) {
		t.Fatal("missing manifest or payload identity")
	}
	if metadata.FastOutputSafe[6] {
		t.Fatal("large output weight was incorrectly admitted to the i32 fast path")
	}
	if got := (*Model)(nil).Metadata(); got != (Metadata{}) {
		t.Fatalf("nil metadata = %+v", got)
	}
}

func TestMarshalRoundTripAndRejectsMissingIdentity(t *testing.T) {
	manifest := sha256.Sum256([]byte("round-trip manifest"))
	tensors := new(Tensors)
	tensors.InputWeights[17][23] = -1234
	tensors.InputBiases[31] = 2345
	tensors.OutputWeights[6][1][47] = -3210
	tensors.OutputBiases[5] = 4567
	data, err := Marshal(tensors, manifest)
	if err != nil {
		t.Fatal(err)
	}
	model, err := Load(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if model.inputWeights[17][23] != -1234 || model.inputBiases[31] != 2345 ||
		model.outputWeights[6][1][47] != -3210 || model.outputBiases[5] != 4567 ||
		model.Metadata().ManifestSHA256 != manifest {
		t.Fatal("round trip changed tensors or manifest identity")
	}
	if data2, err := Marshal(nil, manifest); data2 != nil || !errors.Is(err, ErrFormat) {
		t.Fatalf("Marshal(nil) = %v, %v", data2, err)
	}
	if data2, err := Marshal(tensors, [sha256.Size]byte{}); data2 != nil || !errors.Is(err, ErrIntegrity) {
		t.Fatalf("Marshal(zero manifest) = %v, %v", data2, err)
	}
}

func TestLoadRejectsMalformedWithoutPublishing(t *testing.T) {
	base := makeTestFile(t, nil)
	tests := []struct {
		name   string
		mutate func([]byte) []byte
		want   error
	}{
		{"short", func(data []byte) []byte { return data[:len(data)-1] }, ErrFormat},
		{"trailing", func(data []byte) []byte { return append(data, 0) }, ErrFormat},
		{"magic", func(data []byte) []byte { data[0] ^= 1; return data }, ErrUnsupported},
		{"version", func(data []byte) []byte { data[8]++; return data }, ErrUnsupported},
		{"header size", func(data []byte) []byte { data[10]++; return data }, ErrUnsupported},
		{"architecture", func(data []byte) []byte { data[12]++; return data }, ErrUnsupported},
		{"feature set", func(data []byte) []byte { data[16]++; return data }, ErrUnsupported},
		{"quantization", func(data []byte) []byte { data[20]++; return data }, ErrUnsupported},
		{"score policy", func(data []byte) []byte { data[24]++; return data }, ErrUnsupported},
		{"dimension", func(data []byte) []byte { data[36]++; return data }, ErrUnsupported},
		{"payload length", func(data []byte) []byte { data[56]++; return data }, ErrUnsupported},
		{"zero manifest", func(data []byte) []byte { clear(data[64:96]); return data }, ErrIntegrity},
		{"reserved", func(data []byte) []byte { data[255] = 1; return data }, ErrUnsupported},
		{"payload checksum", func(data []byte) []byte { data[len(data)-1] ^= 1; return data }, ErrIntegrity},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := append([]byte(nil), base...)
			data = test.mutate(data)
			model, err := Load(bytes.NewReader(data))
			if model != nil || !errors.Is(err, test.want) {
				t.Fatalf("Load = %v, %v; want nil, %v", model, err, test.want)
			}
		})
	}
	for name, reader := range map[string]io.Reader{"nil": nil, "stuck": stuckReader{}} {
		t.Run(name, func(t *testing.T) {
			model, err := Load(reader)
			if model != nil || !errors.Is(err, ErrFormat) {
				t.Fatalf("Load = %v, %v", model, err)
			}
		})
	}
}

type stuckReader struct{}

func (stuckReader) Read([]byte) (int, error) { return 0, nil }

func TestWideOutputBoundClassification(t *testing.T) {
	data := makeTestFile(t, func(payload []byte) {
		for perspective := 0; perspective < 2; perspective++ {
			for hidden := 0; hidden < HiddenSize; hidden++ {
				putI16(payload, outputWeightOffset(0, perspective, hidden), math.MaxInt16)
			}
		}
	})
	model, err := Load(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if model.Metadata().FastOutputSafe[0] {
		t.Fatal("overflowing i32 dot bound admitted")
	}
	for bucket := 1; bucket < OutputBuckets; bucket++ {
		if !model.Metadata().FastOutputSafe[bucket] {
			t.Fatalf("zero head %d was not i32-safe", bucket)
		}
	}
}

func TestLoadEnforcesInt16AccumulatorBound(t *testing.T) {
	// 31 rows of 1024 plus 1022 and a bias of 1 reach exactly MaxInt16.
	atLimit := makeTestFile(t, func(payload []byte) {
		for feature := 0; feature < maximumActiveFeatures; feature++ {
			putI16(payload, inputWeightOffset(InputSize+feature, 5), -1024)
		}
		putI16(payload, inputWeightOffset(InputSize+maximumActiveFeatures-1, 5), -1022)
		putI16(payload, inputBiasOffset+5*2, 1)
	})
	if _, err := Load(bytes.NewReader(atLimit)); err != nil {
		t.Fatalf("bound exactly MaxInt16 rejected: %v", err)
	}
	overLimit := makeTestFile(t, func(payload []byte) {
		for feature := 0; feature < maximumActiveFeatures; feature++ {
			putI16(payload, inputWeightOffset(InputSize+feature, 5), -1024)
		}
		putI16(payload, inputBiasOffset+5*2, 1)
	})
	if _, err := Load(bytes.NewReader(overLimit)); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("bound MaxInt16+1 = %v, want ErrUnsupported", err)
	}
}
