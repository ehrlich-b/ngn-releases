package rodenteval

import (
	"math"
	"strings"
	"testing"
)

func testModel() *Model {
	return &Model{
		metadata: Metadata{
			SHA256:       V11AnandSHA256,
			Bytes:        fileSize,
			PayloadBytes: payloadSize,
			TrailerBytes: len(v11AnandTrailer),
			InputSize:    InputSize,
			HiddenSize:   HiddenSize,
			Scale:        outputScale,
		},
		validated: true,
	}
}

func minimalPosition(side Color) Position {
	var board Board
	board[WhiteKing] = uint64(1) << 4
	board[BlackKing] = uint64(1) << 60
	return Position{Board: board, SideToMove: side}
}

func TestFeatureIndexPerspectiveAndHorizontalMirror(t *testing.T) {
	const knightC3 = 18
	tests := []struct {
		name        string
		color       Color
		kingSquare  int
		perspective Color
		want        int
	}{
		{name: "white no mirror", color: White, kingSquare: 3, perspective: White, want: 64 + 18},
		{name: "white mirror", color: White, kingSquare: 4, perspective: White, want: 64 + 21},
		{name: "black perspective vertical", color: White, kingSquare: 59, perspective: Black, want: 384 + 64 + 42},
		{name: "black perspective vertical and mirror", color: White, kingSquare: 60, perspective: Black, want: 384 + 64 + 45},
		{name: "friendly black plane", color: Black, kingSquare: 59, perspective: Black, want: 64 + 42},
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

func TestEvaluateAccumulatorClipsSquaresWeightsAndUsesSideOrder(t *testing.T) {
	model := testModel()
	model.outputWeights[0][0] = 1
	model.outputWeights[1][0] = -2
	model.outputBias = -3
	var accumulator accumulator
	accumulator[White][0] = 255
	accumulator[Black][0] = 128
	if got := model.evaluateAccumulator(&accumulator, White); got != 1 {
		t.Fatalf("white raw = %d, want 1", got)
	}
	if got := model.evaluateAccumulator(&accumulator, Black); got != -5 {
		t.Fatalf("black raw = %d, want -5", got)
	}

	if got := clippedSquaredWeighted(-1, 7); got != 0 {
		t.Fatalf("negative clipped value = %d, want 0", got)
	}
	if got := clippedSquaredWeighted(300, 2); got != 130050 {
		t.Fatalf("upper clipped value = %d, want 130050", got)
	}
}

func TestEvaluateAccumulatorPreservesInt32Wrap(t *testing.T) {
	model := testModel()
	model.outputWeights[0][0] = math.MaxInt16
	model.outputWeights[1][0] = math.MaxInt16
	var accumulator accumulator
	accumulator[White][0] = inputScale
	accumulator[Black][0] = inputScale

	// Each term is 2,130,674,175. Their int32 sum wraps to -33,618,946;
	// the release's two truncating divisions therefore produce -1551.
	if got := model.evaluateAccumulator(&accumulator, White); got != -1551 {
		t.Fatalf("wrapped raw = %d, want -1551", got)
	}
}

func TestEvaluateAccumulatorNegativeDivisionTruncatesTowardZero(t *testing.T) {
	model := testModel()
	model.outputWeights[0][0] = -21421
	var accumulator accumulator
	accumulator[White][0] = 1

	// -21421/255 truncates to -84, then -84*192/16320 truncates to zero.
	// Flooring either division would produce -1 instead.
	if got := model.evaluateAccumulator(&accumulator, White); got != 0 {
		t.Fatalf("negative truncation raw = %d, want 0", got)
	}
}

func TestReleaseMaterialFormulaPreservesTestersAsymmetry(t *testing.T) {
	base := minimalPosition(White).Board
	tests := []struct {
		name  string
		plane int
		want  int
	}{
		{name: "kings only", plane: -1, want: 25000},
		{name: "white bishop counted", plane: WhiteBishop, want: 25300},
		{name: "black bishop omitted", plane: BlackBishop, want: 25000},
		{name: "white rook counted twice", plane: WhiteRook, want: 25800},
		{name: "black rook counted once", plane: BlackRook, want: 25500},
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
