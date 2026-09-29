package countereval

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"math"
	"strings"
	"testing"
)

const (
	hiddenWeightsStart = 0
	hiddenBiasesStart  = InputSize * HiddenSize
	outputWeightsStart = hiddenBiasesStart + HiddenSize
	outputBiasIndex    = outputWeightsStart + HiddenSize
)

func makeLegacyFile(values map[int]float32) []byte {
	data := make([]byte, LegacyFileSize)
	copy(data, legacyHeader[:])
	for index, value := range values {
		binary.LittleEndian.PutUint32(data[LegacyHeaderSize+index*4:], math.Float32bits(value))
	}
	return data
}

func TestLoadCounter55LegacyZeroModel(t *testing.T) {
	data := makeLegacyFile(nil)
	model, metadata, err := LoadCounter55Legacy(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if model == nil {
		t.Fatal("nil model")
	}
	digest := sha256.Sum256(data)
	if metadata.SHA256 != hex.EncodeToString(digest[:]) || metadata.Bytes != LegacyFileSize || metadata.Values != payloadValues {
		t.Fatalf("metadata mismatch: %+v", metadata)
	}
	if metadata.MaxAccumulatorBound != 0 || metadata.MaxProductBound != 0 || metadata.OutputBound != 0 {
		t.Fatalf("zero model bounds are nonzero: %+v", metadata)
	}
}

func TestLoadCounter55LegacyRejectsHeaderMutations(t *testing.T) {
	for index := 0; index < LegacyHeaderSize; index++ {
		data := makeLegacyFile(nil)
		data[index] ^= 0x80
		if _, _, err := LoadCounter55Legacy(bytes.NewReader(data)); err == nil || !strings.Contains(err.Error(), "header") {
			t.Fatalf("header mutation %d: err=%v", index, err)
		}
	}
}

func TestLoadCounter55LegacyRejectsTruncationAndTrailingData(t *testing.T) {
	data := makeLegacyFile(nil)
	cuts := []int{
		0, 1, LegacyHeaderSize - 1, LegacyHeaderSize,
		LegacyHeaderSize + 1,
		LegacyHeaderSize + hiddenBiasesStart*4 - 1,
		LegacyHeaderSize + hiddenBiasesStart*4,
		LegacyHeaderSize + outputWeightsStart*4 - 1,
		LegacyHeaderSize + outputWeightsStart*4,
		LegacyHeaderSize + outputBiasIndex*4 - 1,
		LegacyHeaderSize + outputBiasIndex*4,
		LegacyFileSize - 1,
	}
	for _, cut := range cuts {
		if _, _, err := LoadCounter55Legacy(bytes.NewReader(data[:cut])); err == nil || !strings.Contains(err.Error(), "size") {
			t.Fatalf("cut %d: err=%v", cut, err)
		}
	}
	if _, _, err := LoadCounter55Legacy(bytes.NewReader(append(data, 0))); err == nil || !strings.Contains(err.Error(), "size") {
		t.Fatalf("trailing byte: err=%v", err)
	}
}

func TestLoadCounter55LegacyRejectsNonfiniteEachSection(t *testing.T) {
	cases := []struct {
		name  string
		index int
	}{
		{"hidden_weights", hiddenWeightsStart},
		{"hidden_biases", hiddenBiasesStart},
		{"output_weights", outputWeightsStart},
		{"output_bias", outputBiasIndex},
	}
	values := []float32{
		float32(math.NaN()),
		float32(math.Inf(1)),
		float32(math.Inf(-1)),
	}
	for _, tc := range cases {
		for _, value := range values {
			data := makeLegacyFile(map[int]float32{tc.index: value})
			if _, _, err := LoadCounter55Legacy(bytes.NewReader(data)); err == nil || !strings.Contains(err.Error(), tc.name) {
				t.Fatalf("%s %08x: err=%v", tc.name, math.Float32bits(value), err)
			}
		}
	}
}

func TestLoadCounter55LegacyRejectsUnsafeFiniteBounds(t *testing.T) {
	cases := []struct {
		name   string
		values map[int]float32
		want   string
	}{
		{
			name:   "accumulator",
			values: map[int]float32{hiddenBiasesStart: math.MaxFloat32},
			want:   "accumulator bound",
		},
		{
			name: "product",
			values: map[int]float32{
				hiddenBiasesStart:  1e30,
				outputWeightsStart: 1e10,
			},
			want: "product bound",
		},
		{
			name: "sum",
			values: func() map[int]float32 {
				values := make(map[int]float32, HiddenSize*2)
				for hidden := 0; hidden < HiddenSize; hidden++ {
					values[hiddenBiasesStart+hidden] = 1e36
					values[outputWeightsStart+hidden] = 1
				}
				return values
			}(),
			want: "cumulative output bound",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := makeLegacyFile(tc.values)
			if _, _, err := LoadCounter55Legacy(bytes.NewReader(data)); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v, want %q", err, tc.want)
			}
		})
	}
}

func TestLoadCounter55LegacyRejectsNilAndReadError(t *testing.T) {
	if _, _, err := LoadCounter55Legacy(nil); err == nil || !strings.Contains(err.Error(), "nil reader") {
		t.Fatalf("nil reader: %v", err)
	}
	want := errors.New("injected read failure")
	reader := io.MultiReader(bytes.NewReader(makeLegacyFile(nil)[:100]), errorReader{err: want})
	if _, _, err := LoadCounter55Legacy(reader); err == nil || !errors.Is(err, want) {
		t.Fatalf("read error: %v", err)
	}
}

type errorReader struct{ err error }

func (r errorReader) Read([]byte) (int, error) { return 0, r.err }

func TestLoadCounter55LegacyTensorOrder(t *testing.T) {
	values := map[int]float32{
		hiddenWeightsStart: 2,
		hiddenWeightsStart + InputSize*HiddenSize - 1: -11,
		hiddenBiasesStart:                   3,
		hiddenBiasesStart + HiddenSize - 1:  -13,
		outputWeightsStart:                  5,
		outputWeightsStart + HiddenSize - 1: -17,
		outputBiasIndex:                     7,
	}
	model, _, err := LoadCounter55Legacy(bytes.NewReader(makeLegacyFile(values)))
	if err != nil {
		t.Fatal(err)
	}
	if model.hiddenWeights[0] != 2 || model.hiddenWeights[len(model.hiddenWeights)-1] != -11 ||
		model.hiddenBiases[0] != 3 || model.hiddenBiases[len(model.hiddenBiases)-1] != -13 ||
		model.outputWeights[0] != 5 || model.outputWeights[len(model.outputWeights)-1] != -17 ||
		model.outputBias != 7 {
		t.Fatal("tensor boundary marker mismatch")
	}
	var board Board
	board[0] = 1 // White pawn on A1 selects input row zero.
	output, err := model.EvaluateFullRefresh(board)
	if err != nil {
		t.Fatal(err)
	}
	if output != 32 { // ReLU(3+2)*5 + 7; the negative last lane is clipped.
		t.Fatalf("output=%v want=32", output)
	}
}

func TestLegacyHeaderRecognitionAndValidatedMetadata(t *testing.T) {
	data := makeLegacyFile(map[int]float32{hiddenWeightsStart: -3.5})
	if RecognizesLegacyHeader(data[:LegacyHeaderSize-1]) {
		t.Fatal("short prefix recognized")
	}
	if !RecognizesLegacyHeader(data[:LegacyHeaderSize]) || !RecognizesLegacyHeader(data) {
		t.Fatal("exact legacy header not recognized")
	}
	mutated := append([]byte(nil), data[:LegacyHeaderSize]...)
	mutated[7] ^= 1
	if RecognizesLegacyHeader(mutated) {
		t.Fatal("mutated header recognized")
	}
	if _, err := (*Model)(nil).ValidatedMetadata(); err == nil {
		t.Fatal("nil model metadata succeeded")
	}
	if _, err := new(Model).ValidatedMetadata(); err == nil {
		t.Fatal("zero model metadata succeeded")
	}
	model, loaded, err := LoadCounter55Legacy(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	got, err := model.ValidatedMetadata()
	if err != nil {
		t.Fatal(err)
	}
	if got != loaded {
		t.Fatalf("cached metadata=%+v loaded=%+v", got, loaded)
	}
}
