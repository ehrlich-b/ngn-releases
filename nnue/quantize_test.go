package nnue_test

import (
	"bytes"
	"errors"
	"math"
	"testing"

	"github.com/ehrlich-b/ngn/nnue"
)

func TestQuantizeScalesAndRoundsHalfAwayFromZero(t *testing.T) {
	input := new(nnue.FloatTensors)
	input.FeatureWeights[0][0] = 0.5 / 255
	input.FeatureWeights[0][1] = -0.5 / 255
	input.FeatureBias[0] = 1.5 / 255
	input.OutputWeights[0] = 2.5 / 64
	input.OutputWeights[1] = -2.5 / 64
	input.OutputBias = 3.5 / 16320

	got, err := nnue.Quantize(input)
	if err != nil {
		t.Fatalf("Quantize: %v", err)
	}
	checks := []struct {
		name string
		got  int16
		want int16
	}{
		{"positive feature tie", got.FeatureWeights[0][0], 1},
		{"negative feature tie", got.FeatureWeights[0][1], -1},
		{"feature bias tie", got.FeatureBias[0], 2},
		{"positive output tie", got.OutputWeights[0], 3},
		{"negative output tie", got.OutputWeights[1], -3},
		{"output bias tie", got.OutputBias, 4},
	}
	for _, check := range checks {
		if check.got != check.want {
			t.Errorf("%s = %d, want %d", check.name, check.got, check.want)
		}
	}
}

func TestQuantizeRejectsNonFiniteAndOverflow(t *testing.T) {
	cases := []struct {
		name  string
		apply func(*nnue.FloatTensors)
	}{
		{"NaN", func(input *nnue.FloatTensors) { input.FeatureWeights[0][0] = float32(math.NaN()) }},
		{"positive infinity", func(input *nnue.FloatTensors) { input.FeatureBias[0] = float32(math.Inf(1)) }},
		{"negative infinity", func(input *nnue.FloatTensors) { input.OutputWeights[0] = float32(math.Inf(-1)) }},
		{"positive overflow", func(input *nnue.FloatTensors) { input.FeatureWeights[0][0] = 32767.5 / 255 }},
		{"negative overflow", func(input *nnue.FloatTensors) { input.OutputWeights[0] = -32768.5 / 64 }},
		{"output bias overflow", func(input *nnue.FloatTensors) { input.OutputBias = 32767.5 / 16320 }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			input := new(nnue.FloatTensors)
			test.apply(input)
			if _, err := nnue.Quantize(input); !errors.Is(err, nnue.ErrQuantization) {
				t.Fatalf("Quantize error = %v", err)
			}
		})
	}
	if _, err := nnue.Quantize(nil); !errors.Is(err, nnue.ErrQuantization) {
		t.Fatalf("Quantize(nil) error = %v", err)
	}
}

func TestQuantizeMatchesBulletF32InputPromotionWitness(t *testing.T) {
	const featureWitness float32 = 0.5039215683937073
	input := new(nnue.FloatTensors)
	input.FeatureWeights[0][0] = featureWitness
	input.FeatureWeights[0][1] = -featureWitness
	input.OutputBias = 0.007873774506151676
	got, err := nnue.Quantize(input)
	if err != nil {
		t.Fatalf("Quantize: %v", err)
	}
	if got.FeatureWeights[0][0] != 128 || got.FeatureWeights[0][1] != -128 {
		t.Fatalf("feature witnesses = %d, %d; want 128, -128", got.FeatureWeights[0][0], got.FeatureWeights[0][1])
	}
	if got.OutputBias != 128 {
		t.Fatalf("output-bias witness = %d, want 128", got.OutputBias)
	}

	input.OutputBias = -0.007873774506151676
	got, err = nnue.Quantize(input)
	if err != nil {
		t.Fatalf("Quantize negative output-bias witness: %v", err)
	}
	if got.OutputBias != -128 {
		t.Fatalf("negative output-bias witness = %d, want -128", got.OutputBias)
	}
}

func TestQuantizeAcceptsI16Boundaries(t *testing.T) {
	input := new(nnue.FloatTensors)
	input.FeatureWeights[0][0] = float32(32767) / 255
	input.FeatureWeights[0][1] = float32(-32768) / 255
	got, err := nnue.Quantize(input)
	if err != nil {
		t.Fatalf("Quantize boundaries: %v", err)
	}
	if got.FeatureWeights[0][0] != 32767 || got.FeatureWeights[0][1] != -32768 {
		t.Fatalf("boundaries = %d, %d; want 32767, -32768", got.FeatureWeights[0][0], got.FeatureWeights[0][1])
	}
}

func TestFloatToContainerToEvaluation(t *testing.T) {
	input := new(nnue.FloatTensors)
	input.FeatureWeights[8][0] = 1
	input.OutputWeights[0] = 1
	tensors, err := nnue.Quantize(input)
	if err != nil {
		t.Fatalf("Quantize: %v", err)
	}
	encoded, err := nnue.Marshal(tensors)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	model, err := nnue.Load(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	position := nnue.Position{
		SideToMove: nnue.White,
		Pieces:     []nnue.PieceOnSquare{{Piece: nnue.Pawn, Color: nnue.White, Square: 8}},
	}
	got, err := model.Evaluate(position)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if got != 400 {
		t.Fatalf("score = %d, want 400", got)
	}
}
