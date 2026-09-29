package ngnk4

import (
	"bytes"
	"math"
	"testing"
)

func testPosition(side Color) Position {
	var board Board
	board[WhiteKing] = uint64(1) << 4
	board[BlackKing] = uint64(1) << 60
	board[WhitePawn] = uint64(1) << 12
	return Position{Board: board, SideToMove: side}
}

func TestFeatureContractAllKingSquaresBothPerspectives(t *testing.T) {
	if InputBuckets != 4 {
		t.Skip("independent table covers only the frozen four-bucket layout")
	}
	for perspective := White; perspective <= Black; perspective++ {
		for king := 0; king < 64; king++ {
			for plane := 0; plane < FeaturePlaneCount; plane++ {
				for square := 0; square < 64; square++ {
					orientedKing, orientedSquare := king, square
					if perspective == Black {
						orientedKing ^= 56
						orientedSquare ^= 56
					}
					if king%8 > 3 {
						orientedSquare ^= 7
					}
					rank, file := orientedKing/8, orientedKing%8
					bucket := 3
					if rank == 0 {
						if file < 2 || file > 5 {
							bucket = 1
						} else {
							bucket = 0
						}
					} else if rank == 1 {
						bucket = 2
					}
					want := bucket*InputSize + int(Color(plane/6)^perspective)*384 + (plane%6)*64 + orientedSquare
					if got := featureIndex(Color(plane/6), plane%6, square, king, perspective); got != want {
						t.Fatalf("perspective=%d king=%d plane=%d square=%d: got %d want %d", perspective, king, plane, square, got, want)
					}
				}
			}
		}
	}
}

func TestEvaluateUsesAsymmetricSTMOrderAndFrozenScale(t *testing.T) {
	position := testPosition(White)
	data := makeTestFile(t, func(payload []byte) {
		putI16(payload, inputBiasOffset, 100)
		for plane, pieces := range position.Board {
			if pieces == 0 {
				continue
			}
			square := bitsTrailing(pieces)
			color, pieceType := Color(plane/6), plane%6
			whiteFeature := featureIndex(color, pieceType, square, 4, White)
			blackFeature := featureIndex(color, pieceType, square, 60, Black)
			putI16(payload, inputWeightOffset(whiteFeature, 0), int16(plane+1))
			putI16(payload, inputWeightOffset(blackFeature, 0), int16(2*(plane+1)))
		}
		putI16(payload, outputWeightOffset(0, 0, 0), 3)
		putI16(payload, outputWeightOffset(0, 1, 0), -2)
		putI16(payload, outputBiasOffset, 7)
	})
	model, err := Load(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	accumulator := model.fullRefresh(position.Board)
	whiteAccumulator := int64(accumulator[White][0])
	blackAccumulator := int64(accumulator[Black][0])
	wantWhite := ((whiteAccumulator*whiteAccumulator*3+blackAccumulator*blackAccumulator*-2)/InputScale + 7) * OutputScale / (InputScale * LayerScale)
	gotWhite, err := model.EvaluateRaw(position)
	if err != nil || gotWhite != wantWhite {
		t.Fatalf("white EvaluateRaw = %d, %v; want %d", gotWhite, err, wantWhite)
	}
	position.SideToMove = Black
	wantBlack := ((blackAccumulator*blackAccumulator*3+whiteAccumulator*whiteAccumulator*-2)/InputScale + 7) * OutputScale / (InputScale * LayerScale)
	gotBlack, err := model.EvaluateRaw(position)
	if err != nil || gotBlack != wantBlack {
		t.Fatalf("black EvaluateRaw = %d, %v; want %d", gotBlack, err, wantBlack)
	}
	if gotBlack == -gotWhite {
		t.Fatal("invalid global side-to-move antisymmetry was imposed")
	}
}

func bitsTrailing(value uint64) int {
	for square := 0; square < 64; square++ {
		if value&(uint64(1)<<square) != 0 {
			return square
		}
	}
	return 64
}

func TestAllOutputBuckets(t *testing.T) {
	model, err := Load(bytes.NewReader(makeTestFile(t, nil)))
	if err != nil {
		t.Fatal(err)
	}
	for bucket := 0; bucket < OutputBuckets; bucket++ {
		model.outputBiases[bucket] = int16(1600 + 1000*bucket)
		var position Position
		position.Board[WhiteKing] = uint64(1) << 0
		position.Board[BlackKing] = uint64(1) << 63
		pieces := 2 + 4*bucket
		for square, remaining := 1, pieces-2; remaining > 0; square++ {
			if square == 63 {
				continue
			}
			position.Board[WhitePawn] |= uint64(1) << square
			remaining--
		}
		if got := outputBucket(position.Board); got != bucket {
			t.Fatalf("piece count %d selected head %d, want %d", pieces, got, bucket)
		}
		want := int64(model.outputBiases[bucket]) * OutputScale / (InputScale * LayerScale)
		got, evalErr := model.EvaluateRaw(position)
		if evalErr != nil || got != want {
			t.Fatalf("head %d = %d, %v; want %d", bucket, got, evalErr, want)
		}
	}
}

func TestWideEvaluationDoesNotWrap(t *testing.T) {
	data := makeTestFile(t, func(payload []byte) {
		putI16(payload, inputBiasOffset, InputScale)
		for perspective := 0; perspective < 2; perspective++ {
			for hidden := 0; hidden < HiddenSize; hidden++ {
				putI16(payload, inputBiasOffset+hidden*2, InputScale)
				putI16(payload, outputWeightOffset(0, perspective, hidden), math.MaxInt16)
			}
		}
	})
	model, err := Load(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	position := Position{SideToMove: White}
	position.Board[WhiteKing] = 1 << 4
	position.Board[BlackKing] = 1 << 60
	sum := int64(2 * HiddenSize * InputScale * InputScale * math.MaxInt16)
	want := (sum / InputScale) * OutputScale / (InputScale * LayerScale)
	got, err := model.EvaluateRaw(position)
	if err != nil || got != want || sum <= math.MaxInt32 || model.Metadata().FastOutputSafe[0] {
		t.Fatalf("wide EvaluateRaw = %d, %v; want %d from sum %d", got, err, want, sum)
	}
}

func TestPositionValidation(t *testing.T) {
	model, err := Load(bytes.NewReader(makeTestFile(t, nil)))
	if err != nil {
		t.Fatal(err)
	}
	valid := testPosition(White)
	tests := []struct {
		name string
		edit func(*Position)
	}{
		{"side", func(p *Position) { p.SideToMove = 2 }},
		{"overlap", func(p *Position) { p.Board[BlackPawn] = p.Board[WhitePawn] }},
		{"white king", func(p *Position) { p.Board[WhiteKing] = 0 }},
		{"black kings", func(p *Position) { p.Board[BlackKing] |= 1 << 59 }},
		{"men", func(p *Position) { p.Board[WhitePawn] = math.MaxUint64 &^ p.Board[WhiteKing] &^ p.Board[BlackKing] }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			position := valid
			test.edit(&position)
			if _, err := model.EvaluateRaw(position); err == nil {
				t.Fatal("invalid position accepted")
			}
		})
	}
}
