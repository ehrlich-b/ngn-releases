package nnue_test

import (
	"math/rand"
	"testing"

	"github.com/ehrlich-b/ngn/nnue"
)

func independentBoundedDot(
	us [nnue.HiddenSize]int32,
	them [nnue.HiddenSize]int32,
	weights [nnue.PerspectiveCount * nnue.HiddenSize]int16,
) int64 {
	clip := func(value int32) int64 {
		if value <= 0 {
			return 0
		}
		if value >= nnue.QA {
			return nnue.QA
		}
		return int64(value)
	}
	var dot int64
	for hidden := 0; hidden < nnue.HiddenSize; hidden++ {
		ours := clip(us[hidden])
		theirs := clip(them[hidden])
		dot += ours * ours * int64(weights[hidden])
		dot += theirs * theirs * int64(weights[nnue.HiddenSize+hidden])
	}
	return dot
}

func requireBoundedDotExact(
	t *testing.T,
	us [nnue.HiddenSize]int32,
	them [nnue.HiddenSize]int32,
	weights [nnue.PerspectiveCount * nnue.HiddenSize]int16,
) {
	t.Helper()
	selected, portable, _ := nnue.BoundedOutputDotsForTest(us, them, weights)
	want := independentBoundedDot(us, them, weights)
	if int64(portable) != want {
		t.Fatalf("portable bounded dot = %d, independent int64 oracle = %d", portable, want)
	}
	if int64(selected) != want {
		t.Fatalf("selected bounded dot = %d, independent int64 oracle = %d", selected, want)
	}
}

func TestN4VBoundedDotEveryLanePreservesOrder(t *testing.T) {
	for lane := 0; lane < nnue.PerspectiveCount*nnue.HiddenSize; lane++ {
		var us [nnue.HiddenSize]int32
		var them [nnue.HiddenSize]int32
		var weights [nnue.PerspectiveCount * nnue.HiddenSize]int16
		value := int32(1 + lane%255)
		weight := int16(-128 + lane%257)
		if weight == 0 {
			weight = 127
		}
		if lane < nnue.HiddenSize {
			us[lane] = value
		} else {
			them[lane-nnue.HiddenSize] = value
		}
		weights[lane] = weight
		requireBoundedDotExact(t, us, them, weights)
	}
}

func TestN4VBoundedDotSigned32BoundaryAndClipping(t *testing.T) {
	for _, weight := range []int16{-128, 128} {
		var us [nnue.HiddenSize]int32
		var them [nnue.HiddenSize]int32
		var weights [nnue.PerspectiveCount * nnue.HiddenSize]int16
		for hidden := 0; hidden < nnue.HiddenSize; hidden++ {
			us[hidden] = nnue.MaxAccumulatorValue
			them[hidden] = nnue.MaxAccumulatorValue
			weights[hidden] = weight
			weights[nnue.HiddenSize+hidden] = weight
		}
		requireBoundedDotExact(t, us, them, weights)
		selected, _, _ := nnue.BoundedOutputDotsForTest(us, them, weights)
		const absoluteBound = int64(2130739200)
		want := absoluteBound
		if weight < 0 {
			want = -want
		}
		if int64(selected) != want {
			t.Fatalf("boundary weight %d dot = %d, want %d", weight, selected, want)
		}
	}

	var us [nnue.HiddenSize]int32
	var them [nnue.HiddenSize]int32
	var weights [nnue.PerspectiveCount * nnue.HiddenSize]int16
	for hidden := 0; hidden < nnue.HiddenSize; hidden++ {
		switch hidden % 4 {
		case 0:
			us[hidden], them[hidden] = nnue.MinAccumulatorValue, -1
		case 1:
			us[hidden], them[hidden] = 0, 1
		case 2:
			us[hidden], them[hidden] = nnue.QA-1, nnue.QA
		case 3:
			us[hidden], them[hidden] = nnue.QA+1, nnue.MaxAccumulatorValue
		}
		weights[hidden] = int16(hidden%257 - 128)
		weights[nnue.HiddenSize+hidden] = int16(128 - hidden%257)
	}
	requireBoundedDotExact(t, us, them, weights)
}

func TestN4VBoundedDotDeterministicRandomParity(t *testing.T) {
	random := rand.New(rand.NewSource(0x4e3456))
	for sample := 0; sample < 512; sample++ {
		var us [nnue.HiddenSize]int32
		var them [nnue.HiddenSize]int32
		var weights [nnue.PerspectiveCount * nnue.HiddenSize]int16
		for hidden := 0; hidden < nnue.HiddenSize; hidden++ {
			us[hidden] = int32(random.Int63n(int64(nnue.MaxAccumulatorValue)-int64(nnue.MinAccumulatorValue)+1)) + nnue.MinAccumulatorValue
			them[hidden] = int32(random.Int63n(int64(nnue.MaxAccumulatorValue)-int64(nnue.MinAccumulatorValue)+1)) + nnue.MinAccumulatorValue
			weights[hidden] = int16(random.Intn(257) - 128)
			weights[nnue.HiddenSize+hidden] = int16(random.Intn(257) - 128)
		}
		requireBoundedDotExact(t, us, them, weights)
	}
}

func TestN4VSelectedScoreKeepsSignedDivisionOrderAndFullModelOracle(t *testing.T) {
	for _, outputWeight := range []int16{-13, 13} {
		tensors := new(nnue.Tensors)
		tensors.FeatureBias[7] = 17
		tensors.FeatureBias[91] = 23
		tensors.OutputWeights[7] = outputWeight
		tensors.OutputWeights[nnue.HiddenSize+91] = -outputWeight
		tensors.OutputBias = -7
		model := n4LoadTensors(t, tensors)
		if !model.Capabilities().BoundedInt32Output {
			t.Fatal("division fixture did not select bounded output")
		}
		position := nnue.Position{SideToMove: nnue.White}
		context, err := nnue.NewInt32Context(model)
		if err != nil {
			t.Fatal(err)
		}
		if err := context.Reset(position); err != nil {
			t.Fatal(err)
		}
		got, err := context.Evaluate()
		if err != nil {
			t.Fatal(err)
		}
		want, err := model.Evaluate(position)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("selected score = %d, full int64 model = %d", got, want)
		}
	}
}

func BenchmarkN4VInt32ContextEvaluate(b *testing.B) {
	tensors := new(nnue.Tensors)
	for feature := 0; feature < nnue.InputSize; feature++ {
		for hidden := 0; hidden < nnue.HiddenSize; hidden++ {
			tensors.FeatureWeights[feature][hidden] = int16((feature*17+hidden*29)%511 - 255)
		}
	}
	for hidden := 0; hidden < nnue.HiddenSize; hidden++ {
		tensors.FeatureBias[hidden] = int16((hidden*31)%511 - 255)
		tensors.OutputWeights[hidden] = int16((hidden*43)%257 - 128)
		tensors.OutputWeights[nnue.HiddenSize+hidden] = int16((hidden*71)%257 - 128)
	}
	model := n4LoadTensors(b, tensors)
	position := nnue.Position{
		SideToMove: nnue.White,
		Pieces: []nnue.PieceOnSquare{
			{Piece: nnue.King, Color: nnue.White, Square: 4},
			{Piece: nnue.Queen, Color: nnue.White, Square: 3},
			{Piece: nnue.Rook, Color: nnue.White, Square: 0},
			{Piece: nnue.Bishop, Color: nnue.White, Square: 2},
			{Piece: nnue.Knight, Color: nnue.White, Square: 1},
			{Piece: nnue.Pawn, Color: nnue.White, Square: 12},
			{Piece: nnue.Pawn, Color: nnue.White, Square: 13},
			{Piece: nnue.Pawn, Color: nnue.White, Square: 14},
			{Piece: nnue.King, Color: nnue.Black, Square: 60},
			{Piece: nnue.Queen, Color: nnue.Black, Square: 59},
			{Piece: nnue.Rook, Color: nnue.Black, Square: 56},
			{Piece: nnue.Bishop, Color: nnue.Black, Square: 58},
			{Piece: nnue.Knight, Color: nnue.Black, Square: 57},
			{Piece: nnue.Pawn, Color: nnue.Black, Square: 52},
			{Piece: nnue.Pawn, Color: nnue.Black, Square: 53},
			{Piece: nnue.Pawn, Color: nnue.Black, Square: 54},
		},
	}
	context, err := nnue.NewInt32Context(model)
	if err != nil {
		b.Fatal(err)
	}
	if err := context.Reset(position); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	var sink int64
	for i := 0; i < b.N; i++ {
		value, err := context.Evaluate()
		if err != nil {
			b.Fatal(err)
		}
		sink ^= value
	}
	_ = sink
}
