package nnue_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/ehrlich-b/ngn/nnue"
)

func setFeatureWeight(payload []byte, feature, hidden int, value int16) {
	offset := 2 * (feature*128 + hidden)
	binary.LittleEndian.PutUint16(payload[offset:], uint16(value))
}

func setFeatureBias(payload []byte, hidden int, value int16) {
	binary.LittleEndian.PutUint16(payload[196608+2*hidden:], uint16(value))
}

func setOutputWeight(payload []byte, hidden int, value int16) {
	binary.LittleEndian.PutUint16(payload[196864+2*hidden:], uint16(value))
}

func setOutputBias(payload []byte, value int16) {
	binary.LittleEndian.PutUint16(payload[197376:], uint16(value))
}

func TestChess768FeatureIndexGolden(t *testing.T) {
	cases := []struct {
		name        string
		piece       nnue.PieceOnSquare
		perspective nnue.Color
		want        int
	}{
		{"white pawn a1 white", nnue.PieceOnSquare{Piece: nnue.Pawn, Color: nnue.White, Square: 0}, nnue.White, 0},
		{"white pawn a1 black", nnue.PieceOnSquare{Piece: nnue.Pawn, Color: nnue.White, Square: 0}, nnue.Black, 440},
		{"white knight c3 white", nnue.PieceOnSquare{Piece: nnue.Knight, Color: nnue.White, Square: 18}, nnue.White, 82},
		{"white knight c3 black", nnue.PieceOnSquare{Piece: nnue.Knight, Color: nnue.White, Square: 18}, nnue.Black, 490},
		{"black queen d7 white", nnue.PieceOnSquare{Piece: nnue.Queen, Color: nnue.Black, Square: 51}, nnue.White, 691},
		{"black queen d7 black", nnue.PieceOnSquare{Piece: nnue.Queen, Color: nnue.Black, Square: 51}, nnue.Black, 267},
		{"black king h8 white", nnue.PieceOnSquare{Piece: nnue.King, Color: nnue.Black, Square: 63}, nnue.White, 767},
		{"black king h8 black", nnue.PieceOnSquare{Piece: nnue.King, Color: nnue.Black, Square: 63}, nnue.Black, 327},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, err := nnue.FeatureIndex(test.piece, test.perspective)
			if err != nil {
				t.Fatalf("FeatureIndex: %v", err)
			}
			if got != test.want {
				t.Fatalf("FeatureIndex = %d, want %d", got, test.want)
			}
		})
	}
}

func TestFeatureIndexRejectsInvalidInput(t *testing.T) {
	cases := []struct {
		piece       nnue.PieceOnSquare
		perspective nnue.Color
	}{
		{nnue.PieceOnSquare{Piece: nnue.PieceType(6), Color: nnue.White, Square: 0}, nnue.White},
		{nnue.PieceOnSquare{Piece: nnue.Pawn, Color: nnue.Color(2), Square: 0}, nnue.White},
		{nnue.PieceOnSquare{Piece: nnue.Pawn, Color: nnue.White, Square: 64}, nnue.White},
		{nnue.PieceOnSquare{Piece: nnue.Pawn, Color: nnue.White, Square: 0}, nnue.Color(2)},
	}
	for i, test := range cases {
		if _, err := nnue.FeatureIndex(test.piece, test.perspective); !errors.Is(err, nnue.ErrFormat) {
			t.Errorf("case %d error = %v", i, err)
		}
	}
}

func TestEvaluateGoldenPerspectiveAndMutation(t *testing.T) {
	// This fixture is assembled at literal byte offsets, independently of
	// Marshal and FeatureIndex. White pawn a2 is feature 8 from White's view and
	// feature 432 from Black's view.
	payload := make([]byte, manualPayloadSize)
	setFeatureWeight(payload, 8, 0, 255)
	setOutputWeight(payload, 0, 64)
	setOutputWeight(payload, 128, -64)
	model := loadManual(t, payload)
	position := nnue.Position{
		Pieces: []nnue.PieceOnSquare{{Piece: nnue.Pawn, Color: nnue.White, Square: 8}},
	}

	position.SideToMove = nnue.White
	got, err := model.Evaluate(position)
	if err != nil {
		t.Fatalf("Evaluate White: %v", err)
	}
	if got != 400 {
		t.Fatalf("White score = %d, want 400", got)
	}
	position.SideToMove = nnue.Black
	got, err = model.Evaluate(position)
	if err != nil {
		t.Fatalf("Evaluate Black: %v", err)
	}
	if got != -400 {
		t.Fatalf("Black score = %d, want -400", got)
	}

	// Activate the independently known Black-perspective feature. Equal
	// accumulators cancel under the asymmetric STM/NTM output weights.
	setFeatureWeight(payload, 432, 0, 255)
	mutated := loadManual(t, payload)
	for _, side := range []nnue.Color{nnue.White, nnue.Black} {
		position.SideToMove = side
		got, err = mutated.Evaluate(position)
		if err != nil {
			t.Fatalf("Evaluate mutated side %d: %v", side, err)
		}
		if got != 0 {
			t.Fatalf("mutated side %d score = %d, want 0", side, got)
		}
	}
}

func TestEvaluatePreservesTwoTruncatingDivisions(t *testing.T) {
	t.Run("first division toward zero", func(t *testing.T) {
		payload := make([]byte, manualPayloadSize)
		setFeatureBias(payload, 0, 1)
		setOutputWeight(payload, 0, -254)
		setOutputBias(payload, 16320)
		model := loadManual(t, payload)
		got, err := model.Evaluate(nnue.Position{SideToMove: nnue.White})
		if err != nil {
			t.Fatalf("Evaluate: %v", err)
		}
		if got != 400 {
			t.Fatalf("score = %d, want 400", got)
		}
	})

	t.Run("second division toward zero", func(t *testing.T) {
		payload := make([]byte, manualPayloadSize)
		setOutputBias(payload, -41)
		model := loadManual(t, payload)
		got, err := model.Evaluate(nnue.Position{SideToMove: nnue.White})
		if err != nil {
			t.Fatalf("Evaluate: %v", err)
		}
		if got != -1 {
			t.Fatalf("score = %d, want -1", got)
		}
	})
}

func TestEvaluateSCReLUClipping(t *testing.T) {
	payload := make([]byte, manualPayloadSize)
	setFeatureBias(payload, 0, 300)
	setFeatureBias(payload, 1, -5)
	setOutputWeight(payload, 0, 64)
	setOutputWeight(payload, 1, 32767)
	model := loadManual(t, payload)
	got, err := model.Evaluate(nnue.Position{SideToMove: nnue.White})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if got != 400 {
		t.Fatalf("score = %d, want 400", got)
	}
}

func TestEvaluateExtremeTensorsUsesWideArithmetic(t *testing.T) {
	payload := make([]byte, manualPayloadSize)
	for hidden := 0; hidden < 128; hidden++ {
		setFeatureBias(payload, hidden, 32767)
	}
	for hidden := 0; hidden < 256; hidden++ {
		setOutputWeight(payload, hidden, 32767)
	}
	setOutputBias(payload, 32767)
	model := loadManual(t, payload)
	got, err := model.Evaluate(nnue.Position{SideToMove: nnue.White})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	// Hand-derived from 256*255^2*32767, then the two specified divisions.
	const want int64 = 52428003
	if got != want {
		t.Fatalf("score = %d, want %d", got, want)
	}
}

func TestEvaluateRejectsInvalidPosition(t *testing.T) {
	model := loadManual(t, make([]byte, manualPayloadSize))
	piece := nnue.PieceOnSquare{Piece: nnue.Pawn, Color: nnue.White, Square: 8}
	cases := []nnue.Position{
		{SideToMove: nnue.Color(2)},
		{SideToMove: nnue.White, Pieces: []nnue.PieceOnSquare{piece, piece}},
		{SideToMove: nnue.White, Pieces: []nnue.PieceOnSquare{{Piece: nnue.PieceType(6), Color: nnue.White, Square: 0}}},
	}
	tooMany := nnue.Position{SideToMove: nnue.White, Pieces: make([]nnue.PieceOnSquare, 65)}
	cases = append(cases, tooMany)
	for i, position := range cases {
		if _, err := model.Evaluate(position); !errors.Is(err, nnue.ErrFormat) {
			t.Errorf("case %d error = %v", i, err)
		}
	}
	var nilModel *nnue.Model
	if _, err := nilModel.Evaluate(nnue.Position{}); !errors.Is(err, nnue.ErrFormat) {
		t.Errorf("nil model error = %v", err)
	}
	if _, err := new(nnue.Model).Evaluate(nnue.Position{}); !errors.Is(err, nnue.ErrFormat) {
		t.Errorf("zero model error = %v", err)
	}
}

func TestEvaluateMultiPieceInteriorNeuronGolden(t *testing.T) {
	// Hand-derived features for white pawn a2 and black queen d7:
	// White view: 8 and 691; Black view: 432 and 267.
	payload := make([]byte, manualPayloadSize)
	setFeatureBias(payload, 57, -20)
	setFeatureWeight(payload, 8, 57, 100)
	setFeatureWeight(payload, 691, 57, 30)
	setFeatureWeight(payload, 432, 57, 40)
	setFeatureWeight(payload, 267, 57, 200)
	setOutputWeight(payload, 57, 64)
	setOutputWeight(payload, 128+57, 32)
	setOutputBias(payload, -100)
	model := loadManual(t, payload)
	position := nnue.Position{
		Pieces: []nnue.PieceOnSquare{
			{Piece: nnue.Pawn, Color: nnue.White, Square: 8},
			{Piece: nnue.Queen, Color: nnue.Black, Square: 51},
		},
	}

	for _, test := range []struct {
		side nnue.Color
		want int64
	}{
		{nnue.White, 220},
		{nnue.Black, 332},
	} {
		position.SideToMove = test.side
		got, err := model.Evaluate(position)
		if err != nil {
			t.Fatalf("Evaluate side %d: %v", test.side, err)
		}
		if got != test.want {
			t.Fatalf("side %d score = %d, want %d", test.side, got, test.want)
		}
	}
}

func TestLoadedModelDoesNotAliasInputOrMetadata(t *testing.T) {
	payload := make([]byte, manualPayloadSize)
	setFeatureWeight(payload, 8, 0, 255)
	setOutputWeight(payload, 0, 64)
	encoded := manualContainer(payload)
	model, err := nnue.Load(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	position := nnue.Position{
		SideToMove: nnue.White,
		Pieces:     []nnue.PieceOnSquare{{Piece: nnue.Pawn, Color: nnue.White, Square: 8}},
	}
	beforeScore, err := model.Evaluate(position)
	if err != nil {
		t.Fatalf("Evaluate before mutation: %v", err)
	}
	beforeMetadata := model.Metadata()

	for i := range encoded {
		encoded[i] = 0xff
	}
	metadataCopy := model.Metadata()
	metadataCopy.FileSHA256[0] ^= 0xff
	metadataCopy.PayloadSHA256[0] ^= 0xff

	afterScore, err := model.Evaluate(position)
	if err != nil {
		t.Fatalf("Evaluate after mutation: %v", err)
	}
	if afterScore != beforeScore || afterScore != 400 {
		t.Fatalf("score changed from %d to %d", beforeScore, afterScore)
	}
	if afterMetadata := model.Metadata(); afterMetadata != beforeMetadata {
		t.Fatalf("metadata changed: got %+v, want %+v", afterMetadata, beforeMetadata)
	}
}
