package countereval

import (
	"bytes"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestEvaluateFullRefreshUsesCounterFeatureAndArithmeticOrder(t *testing.T) {
	model := new(Model)
	model.hiddenBiases[0] = 3
	model.hiddenBiases[1] = -2
	model.hiddenWeights[(0*64+0)*HiddenSize+0] = 2
	model.hiddenWeights[(0*64+0)*HiddenSize+1] = 3
	model.hiddenWeights[(10*64+63)*HiddenSize+0] = 4
	model.hiddenWeights[(10*64+63)*HiddenSize+1] = -1
	model.outputWeights[0] = 2
	model.outputWeights[1] = 5
	model.outputBias = 7

	var board Board
	board[0] = uint64(1) << 0
	board[10] = uint64(1) << 63
	features, accumulator, output, err := model.evaluateFullRefreshTrace(board)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int{0, 703}; !reflect.DeepEqual(features, want) {
		t.Fatalf("features=%v want=%v", features, want)
	}
	if accumulator[0] != 9 || accumulator[1] != 0 {
		t.Fatalf("accumulator[0:2]=%v", accumulator[:2])
	}
	if output != 25 {
		t.Fatalf("output=%v want=25", output)
	}
	publicOutput, err := model.EvaluateFullRefresh(board)
	if err != nil || math.Float32bits(publicOutput) != math.Float32bits(output) {
		t.Fatalf("public output=%v err=%v", publicOutput, err)
	}
}

func TestEvaluateFullRefreshRejectsOverlapAndNilModel(t *testing.T) {
	var board Board
	board[0] = 1 << 12
	board[11] = 1 << 12
	if _, err := new(Model).EvaluateFullRefresh(board); err == nil || !strings.Contains(err.Error(), "overlaps") {
		t.Fatalf("overlap err=%v", err)
	}
	var model *Model
	if _, err := model.EvaluateFullRefresh(Board{}); err == nil || !strings.Contains(err.Error(), "nil model") {
		t.Fatalf("nil model err=%v", err)
	}
}

func TestEvaluateFullRefreshScansSquaresBeforePlanes(t *testing.T) {
	model := new(Model)
	var board Board
	board[11] = 1 << 0
	board[0] = 1 << 63
	features, _, _, err := model.evaluateFullRefreshTrace(board)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int{704, 63}; !reflect.DeepEqual(features, want) {
		t.Fatalf("features=%v want=%v", features, want)
	}
}

func TestEvaluateAccumulatorRoundsEachProductBeforeAddition(t *testing.T) {
	// Load the values through the legacy file path so the witness exercises
	// runtime tensor values rather than a constant-folded arithmetic expression.
	model, _, err := LoadCounter55Legacy(bytes.NewReader(makeLegacyFile(map[int]float32{
		hiddenBiasesStart:      1,
		hiddenBiasesStart + 1:  math.Float32frombits(0x3f800001),
		outputWeightsStart:     -1,
		outputWeightsStart + 1: math.Float32frombits(0x3f7ffffe),
	})))
	if err != nil {
		t.Fatal(err)
	}

	firstProduct := float32(model.hiddenBiases[0] * model.outputWeights[0])
	if bits := math.Float32bits(firstProduct); bits != math.Float32bits(float32(-1)) {
		t.Fatalf("first product bits=%08x want=%08x", bits, math.Float32bits(float32(-1)))
	}
	secondProduct := float32(model.hiddenBiases[1] * model.outputWeights[1])
	if bits := math.Float32bits(secondProduct); bits != math.Float32bits(float32(1)) {
		t.Fatalf("separately rounded second product bits=%08x want=%08x", bits, math.Float32bits(float32(1)))
	}
	fusedReference := float32(math.FMA(
		float64(model.hiddenBiases[1]),
		float64(model.outputWeights[1]),
		float64(firstProduct),
	))
	if bits := math.Float32bits(fusedReference); bits != 0xa8800000 {
		t.Fatalf("fused reference bits=%08x want=a8800000", bits)
	}

	output, err := model.EvaluateFullRefresh(Board{})
	if err != nil {
		t.Fatal(err)
	}
	if bits := math.Float32bits(output); bits != math.Float32bits(float32(0)) {
		// A fused (-1)+(1+2^-23)*(1-2^-23) is -2^-46 (bits a8800000).
		t.Fatalf("portable non-fused output bits=%08x want=00000000; fused witness=a8800000", bits)
	}
}
