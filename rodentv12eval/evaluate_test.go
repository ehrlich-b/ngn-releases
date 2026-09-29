package rodentv12eval

import (
	"math"
	"strings"
	"testing"
)

func testModel() *Model {
	model := new(Model)
	model.metadata = Metadata{
		SHA256:             V12DefaultSHA256,
		Bytes:              fileSize,
		PayloadBytes:       payloadSize,
		TrailerBytes:       len(v12DefaultTrailer),
		InputBuckets:       InputBuckets,
		InputSize:          InputSize,
		TotalInputFeatures: TotalInputFeatures,
		HiddenSize:         HiddenSize,
		OutputBuckets:      OutputBuckets,
		Scale:              outputScale,
	}
	model.validated = true
	return model
}

func minimalPosition(side Color) Position {
	var board Board
	board[WhiteKing] = uint64(1) << 4
	board[BlackKing] = uint64(1) << 60
	return Position{Board: board, SideToMove: side}
}

func TestFeatureIndexUsesPerspectiveMirrorAndKingBucket(t *testing.T) {
	const knightC3 = 18
	tests := []struct {
		name        string
		color       Color
		kingSquare  int
		perspective Color
		want        int
	}{
		{name: "white central rank one bucket zero", color: White, kingSquare: 3, perspective: White, want: 82},
		{name: "white flank rank one bucket one", color: White, kingSquare: 0, perspective: White, want: 850},
		{name: "white rank two bucket two", color: White, kingSquare: 8, perspective: White, want: 1618},
		{name: "white upper rank bucket three", color: White, kingSquare: 16, perspective: White, want: 2386},
		{name: "white horizontal mirror", color: White, kingSquare: 4, perspective: White, want: 85},
		{name: "black perspective vertical", color: White, kingSquare: 59, perspective: Black, want: 490},
		{name: "black perspective vertical and mirror", color: White, kingSquare: 60, perspective: Black, want: 493},
		{name: "black perspective flank bucket", color: White, kingSquare: 56, perspective: Black, want: 1258},
		{name: "friendly black plane", color: Black, kingSquare: 59, perspective: Black, want: 106},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := featureIndex(test.color, 1, knightC3, test.kingSquare, test.perspective); got != test.want {
				t.Fatalf("featureIndex = %d, want %d", got, test.want)
			}
		})
	}
}

func TestValidatePositionRejectsInvalidSideOverlapAndKingCounts(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Position)
		want string
	}{
		{name: "side", edit: func(p *Position) { p.SideToMove = Color(2) }, want: "invalid side"},
		{name: "overlap", edit: func(p *Position) { p.Board[WhiteQueen] = p.Board[WhiteKing] }, want: "overlapping"},
		{name: "missing white king", edit: func(p *Position) { p.Board[WhiteKing] = 0 }, want: "white king count 0"},
		{name: "two black kings", edit: func(p *Position) { p.Board[BlackKing] |= uint64(1) << 56 }, want: "black king count 2"},
	}
	model := testModel()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			position := minimalPosition(White)
			test.edit(&position)
			before := position
			if _, err := model.EvaluateRaw(position); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("EvaluateRaw error = %v, want substring %q", err, test.want)
			}
			if position != before {
				t.Fatal("rejected evaluation mutated input position")
			}
		})
	}
}

func TestFullRefreshUsesModularInt16ArithmeticWithoutMutation(t *testing.T) {
	model := testModel()
	model.inputBiases[0] = math.MaxInt16
	model.inputBiases[1] = math.MinInt16
	position := minimalPosition(White)
	before := position
	whiteKingSquare := 4
	blackKingSquare := 60
	for plane, square := range map[int]int{WhiteKing: whiteKingSquare, BlackKing: blackKingSquare} {
		color := Color(plane / 6)
		pieceType := plane % 6
		for _, perspective := range []Color{White, Black} {
			kingSquare := whiteKingSquare
			if perspective == Black {
				kingSquare = blackKingSquare
			}
			row := featureIndex(color, pieceType, square, kingSquare, perspective)
			model.inputWeights[row][0] = 1
			model.inputWeights[row][1] = -1
		}
	}
	accumulator := model.fullRefresh(position.Board)
	for _, perspective := range []Color{White, Black} {
		if got := accumulator[perspective][0]; got != -32767 {
			t.Fatalf("perspective %d overflow lane = %d, want -32767", perspective, got)
		}
		if got := accumulator[perspective][1]; got != 32766 {
			t.Fatalf("perspective %d underflow lane = %d, want 32766", perspective, got)
		}
	}
	if position != before {
		t.Fatal("full refresh mutated input position")
	}
}

func TestOutputBucketCoversPieceCountBoundaries(t *testing.T) {
	tests := []struct {
		pieces int
		want   int
	}{
		{pieces: 2, want: 0}, {pieces: 5, want: 0},
		{pieces: 6, want: 1}, {pieces: 9, want: 1},
		{pieces: 10, want: 2}, {pieces: 13, want: 2},
		{pieces: 14, want: 3}, {pieces: 17, want: 3},
		{pieces: 18, want: 4}, {pieces: 21, want: 4},
		{pieces: 22, want: 5}, {pieces: 25, want: 5},
		{pieces: 26, want: 6}, {pieces: 29, want: 6},
		{pieces: 30, want: 7}, {pieces: 64, want: 7},
	}
	for _, test := range tests {
		position := minimalPosition(White)
		remaining := test.pieces - 2
		for square := 0; remaining > 0; square++ {
			if square == 4 || square == 60 {
				continue
			}
			position.Board[WhitePawn] |= uint64(1) << square
			remaining--
		}
		if got := outputBucket(position.Board); got != test.want {
			t.Errorf("outputBucket with %d pieces = %d, want %d", test.pieces, got, test.want)
		}
	}
}

func TestEvaluateAccumulatorClipsSquaresWeightsAndUsesSideOrder(t *testing.T) {
	model := testModel()
	const bucket = 3
	model.outputWeights[bucket][0][0] = 1
	model.outputWeights[bucket][1][0] = -2
	model.outputBiases[bucket] = -3
	var accumulator accumulator
	accumulator[White][0] = 255
	accumulator[Black][0] = 128
	if got := model.evaluateAccumulator(&accumulator, White, bucket); got != 1 {
		t.Fatalf("white raw = %d, want 1", got)
	}
	if got := model.evaluateAccumulator(&accumulator, Black, bucket); got != -5 {
		t.Fatalf("black raw = %d, want -5", got)
	}

	if got := clippedSquaredWeighted(-1, 7); got != 0 {
		t.Fatalf("negative clipped value = %d, want 0", got)
	}
	if got := clippedSquaredWeighted(300, 2); got != 130050 {
		t.Fatalf("upper clipped value = %d, want 130050", got)
	}
}

func TestEvaluateAccumulatorPreservesFourPartInt32Wrap(t *testing.T) {
	model := testModel()
	for lane := 0; lane < 4; lane++ {
		model.outputWeights[0][0][lane] = math.MaxInt16
		model.outputWeights[0][1][lane] = math.MaxInt16
	}
	var accumulator accumulator
	for lane := 0; lane < 4; lane++ {
		accumulator[White][lane] = inputScale
		accumulator[Black][lane] = inputScale
	}

	// Each perspective term is 2,130,674,175. The pair in each of the
	// four partial sums wraps to -33,618,946 before the four sums combine.
	if got := model.evaluateAccumulator(&accumulator, White, 0); got != -6656 {
		t.Fatalf("wrapped raw = %d, want -6656", got)
	}
}

func TestEvaluateAccumulatorNegativeDivisionTruncatesTowardZero(t *testing.T) {
	model := testModel()
	model.outputWeights[0][0][0] = -1
	var accumulator accumulator
	accumulator[White][0] = 1
	if got := model.evaluateAccumulator(&accumulator, White, 0); got != 0 {
		t.Fatalf("first negative truncation raw = %d, want 0", got)
	}

	model.outputWeights[0][0][0] = 0
	model.outputBiases[0] = -79
	if got := model.evaluateAccumulator(&accumulator, White, 0); got != 0 {
		t.Fatalf("second negative truncation raw = %d, want 0", got)
	}
}

func TestReleaseMaterialFormulaIsSymmetric(t *testing.T) {
	base := minimalPosition(White).Board
	tests := []struct {
		name  string
		plane int
		want  int
	}{
		{name: "kings only", plane: -1, want: 25000},
		{name: "white pawn", plane: WhitePawn, want: 25100},
		{name: "black pawn", plane: BlackPawn, want: 25100},
		{name: "white knight", plane: WhiteKnight, want: 25300},
		{name: "black knight", plane: BlackKnight, want: 25300},
		{name: "white bishop", plane: WhiteBishop, want: 25300},
		{name: "black bishop", plane: BlackBishop, want: 25300},
		{name: "white rook", plane: WhiteRook, want: 25500},
		{name: "black rook", plane: BlackRook, want: 25500},
		{name: "white queen", plane: WhiteQueen, want: 25900},
		{name: "black queen", plane: BlackQueen, want: 25900},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			board := base
			if test.plane >= 0 {
				board[test.plane] = uint64(1) << 24
			}
			if got := scaleReleaseStatic(32768, board); got != test.want {
				t.Fatalf("release static = %d, want %d", got, test.want)
			}
		})
	}
}
