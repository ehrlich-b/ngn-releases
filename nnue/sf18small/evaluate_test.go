package sf18small

import (
	"errors"
	"reflect"
	"testing"

	base "github.com/ehrlich-b/ngn/nnue"
)

func blankEvaluationModel() *Model {
	return &Model{
		loaded:         true,
		featureWeights: make([]int16, inputFeatures*transformerLanes),
		psqtWeights:    make([]int32, inputFeatures*psqtBuckets),
	}
}

func sfPiece(piece base.PieceType, color base.Color, square base.Square) base.PieceOnSquare {
	return base.PieceOnSquare{Piece: piece, Color: color, Square: square}
}

func TestFeatureIndexPinnedExamples(t *testing.T) {
	tests := []struct {
		name        string
		piece       boardPiece
		square      base.Square
		king        base.Square
		perspective base.Color
		want        int
	}{
		{"white own pawn e1 king", boardPiece{piece: base.Pawn, color: base.White}, 8, 4, base.White, 21832},
		{"white enemy pawn e1 king", boardPiece{piece: base.Pawn, color: base.Black}, 48, 4, base.White, 21936},
		{"white own king e1", boardPiece{piece: base.King, color: base.White}, 4, 4, base.White, 22468},
		{"white enemy king e1", boardPiece{piece: base.King, color: base.Black}, 60, 4, base.White, 22524},
		{"black own pawn e8 king", boardPiece{piece: base.Pawn, color: base.Black}, 48, 60, base.Black, 21832},
		{"black enemy pawn e8 king", boardPiece{piece: base.Pawn, color: base.White}, 8, 60, base.Black, 21936},
		{"black own king e8", boardPiece{piece: base.King, color: base.Black}, 60, 60, base.Black, 22468},
		{"black enemy king e8", boardPiece{piece: base.King, color: base.White}, 4, 60, base.Black, 22524},
		{"file mirror c1 king", boardPiece{piece: base.Pawn, color: base.White}, 8, 2, base.White, 21135},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := featureIndex(test.piece, test.square, test.king, test.perspective); got != test.want {
				t.Fatalf("featureIndex = %d, want %d", got, test.want)
			}
		})
	}
}

func TestEvaluateAllZeroModelAllBucketsAndCanonicalOrder(t *testing.T) {
	model := blankEvaluationModel()
	position := base.Position{
		SideToMove: base.White,
		Pieces: []base.PieceOnSquare{
			sfPiece(base.King, base.Black, 60),
			sfPiece(base.Pawn, base.Black, 48),
			sfPiece(base.King, base.White, 4),
			sfPiece(base.Pawn, base.White, 8),
		},
	}
	trace, err := model.evaluateAllTrace(position)
	if err != nil {
		t.Fatal(err)
	}
	if trace.public.CorrectBucket != 0 || trace.public.Buckets != [layerStacks]Components{} {
		t.Fatalf("zero model public trace = %+v", trace.public)
	}
	wantWhite := []uint16{22468, 21832, 21936, 22524}
	wantBlack := []uint16{22524, 21936, 21832, 22468}
	if got := trace.accumulator.active[base.White][:trace.accumulator.activeCount[base.White]]; !reflect.DeepEqual(got, wantWhite) {
		t.Fatalf("white active indices = %v, want %v", got, wantWhite)
	}
	if got := trace.accumulator.active[base.Black][:trace.accumulator.activeCount[base.Black]]; !reflect.DeepEqual(got, wantBlack) {
		t.Fatalf("black active indices = %v, want %v", got, wantBlack)
	}

	reversed := position
	reversed.Pieces = append([]base.PieceOnSquare(nil), position.Pieces...)
	for left, right := 0, len(reversed.Pieces)-1; left < right; left, right = left+1, right-1 {
		reversed.Pieces[left], reversed.Pieces[right] = reversed.Pieces[right], reversed.Pieces[left]
	}
	reversedTrace, err := model.evaluateAllTrace(reversed)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(trace, reversedTrace) {
		t.Fatal("full-refresh trace depends on public piece order")
	}
}

func TestTransformerPairProductPerspectiveOrderAndPSQT(t *testing.T) {
	model := blankEvaluationModel()
	model.featureBias[0] = 254
	model.featureBias[64] = 254
	whitePawnIndex := 21832
	model.featureWeights[whitePawnIndex*transformerLanes] = -254
	model.psqtWeights[whitePawnIndex*psqtBuckets] = 33
	position := base.Position{SideToMove: base.White, Pieces: []base.PieceOnSquare{
		sfPiece(base.King, base.White, 4),
		sfPiece(base.Pawn, base.White, 8),
		sfPiece(base.King, base.Black, 60),
	}}
	whiteTrace, err := model.evaluateAllTrace(position)
	if err != nil {
		t.Fatal(err)
	}
	if whiteTrace.transformed[0] != 0 || whiteTrace.transformed[64] != 126 {
		t.Fatalf("white STM transformed pair = %d/%d", whiteTrace.transformed[0], whiteTrace.transformed[64])
	}
	if whiteTrace.psqtRaw[0] != 16 || whiteTrace.public.Buckets[0].PSQT != 1 {
		t.Fatalf("white STM PSQT raw/output = %d/%d", whiteTrace.psqtRaw[0], whiteTrace.public.Buckets[0].PSQT)
	}
	position.SideToMove = base.Black
	blackTrace, err := model.evaluateAllTrace(position)
	if err != nil {
		t.Fatal(err)
	}
	if blackTrace.transformed[0] != 126 || blackTrace.transformed[64] != 0 {
		t.Fatalf("black STM transformed pair = %d/%d", blackTrace.transformed[0], blackTrace.transformed[64])
	}
	if blackTrace.psqtRaw[0] != -16 || blackTrace.public.Buckets[0].PSQT != -1 {
		t.Fatalf("black STM PSQT raw/output = %d/%d", blackTrace.psqtRaw[0], blackTrace.public.Buckets[0].PSQT)
	}
}

func TestEvaluateAllSelectsEveryMaterialBucket(t *testing.T) {
	model := blankEvaluationModel()
	for bucket := 0; bucket < layerStacks; bucket++ {
		count := (bucket + 1) * 4
		pieces := []base.PieceOnSquare{
			sfPiece(base.King, base.White, 4),
			sfPiece(base.King, base.Black, 60),
		}
		for square := 0; len(pieces) < count; square++ {
			if square == 4 || square == 60 {
				continue
			}
			pieces = append(pieces, sfPiece(base.Pawn, base.Color(square&1), base.Square(square)))
		}
		trace, err := model.EvaluateAll(base.Position{SideToMove: base.Color(bucket & 1), Pieces: pieces})
		if err != nil {
			t.Fatal(err)
		}
		if trace.CorrectBucket != uint8(bucket) {
			t.Fatalf("%d pieces selected bucket %d, want %d", count, trace.CorrectBucket, bucket)
		}
	}
}

func TestEvaluateAllRejectsInvalidPositionAndUnloadedModel(t *testing.T) {
	valid := base.Position{SideToMove: base.White, Pieces: []base.PieceOnSquare{
		sfPiece(base.King, base.White, 4), sfPiece(base.King, base.Black, 60),
	}}
	if got, err := (*Model)(nil).EvaluateAll(valid); got != (Trace{}) || !errors.Is(err, ErrEvaluation) {
		t.Fatalf("nil model = %+v, %v", got, err)
	}
	if got, err := (&Model{}).EvaluateAll(valid); got != (Trace{}) || !errors.Is(err, ErrEvaluation) {
		t.Fatalf("zero model = %+v, %v", got, err)
	}
	model := blankEvaluationModel()
	tests := []base.Position{
		{SideToMove: 2, Pieces: valid.Pieces},
		{SideToMove: base.White},
		{SideToMove: base.White, Pieces: []base.PieceOnSquare{sfPiece(base.King, base.White, 4), sfPiece(base.King, base.White, 5), sfPiece(base.King, base.Black, 60)}},
		{SideToMove: base.White, Pieces: []base.PieceOnSquare{sfPiece(base.King, base.White, 4), sfPiece(base.King, base.Black, 4)}},
		{SideToMove: base.White, Pieces: []base.PieceOnSquare{sfPiece(6, base.White, 4), sfPiece(base.King, base.Black, 60)}},
		{SideToMove: base.White, Pieces: []base.PieceOnSquare{sfPiece(base.King, 2, 4), sfPiece(base.King, base.Black, 60)}},
		{SideToMove: base.White, Pieces: []base.PieceOnSquare{sfPiece(base.King, base.White, 64), sfPiece(base.King, base.Black, 60)}},
	}
	tooMany := valid
	for square := 0; len(tooMany.Pieces) < 33; square++ {
		if square != 4 && square != 60 {
			tooMany.Pieces = append(tooMany.Pieces, sfPiece(base.Pawn, base.White, base.Square(square)))
		}
	}
	tests = append(tests, tooMany)
	for index, position := range tests {
		got, err := model.EvaluateAll(position)
		if got != (Trace{}) || !errors.Is(err, ErrEvaluation) {
			t.Fatalf("invalid case %d = %+v, %v", index, got, err)
		}
	}
}

func TestScalarLayerArithmeticAndLayout(t *testing.T) {
	model := blankEvaluationModel()
	stack := &model.stacks[3]
	stack.fc0Bias[0] = -8192
	stack.fc0Bias[2] = 5
	stack.fc0Weight[2*transformerLanes+3] = 4
	stack.fc1Weight[0] = 1
	stack.fc1Weight[1*32+15] = 2
	stack.fc1Weight[2*32+30] = 99
	stack.fc2Weight[0] = 1
	stack.fc2Weight[1] = 2
	var input [transformerLanes]uint8
	input[3] = 2
	trace := model.propagateStack(3, &input)
	if trace.fc0[0] != -8192 || trace.squared[0] != 127 || trace.clipped0[0] != 0 {
		t.Fatalf("signed FC0 activations = %d/%d/%d", trace.fc0[0], trace.squared[0], trace.clipped0[0])
	}
	if trace.fc0[2] != 13 {
		t.Fatalf("FC0 row-major result = %d, want 13", trace.fc0[2])
	}
	if trace.fc1[0] != 127 || trace.fc1[1] != 0 || trace.fc1[2] != 0 {
		t.Fatalf("FC1 square/regular/padding routing = %d/%d/%d", trace.fc1[0], trace.fc1[1], trace.fc1[2])
	}
	if trace.clipped1[0] != 1 || trace.fc2 != 1 {
		t.Fatalf("FC1 activation/FC2 = %d/%d", trace.clipped1[0], trace.fc2)
	}
}

func TestPSQTPerspectiveDifferenceUsesDefinedWrapping(t *testing.T) {
	model := blankEvaluationModel()
	model.psqtWeights[21832*psqtBuckets] = 2147483647
	model.psqtWeights[21936*psqtBuckets] = -1
	position := base.Position{SideToMove: base.White, Pieces: []base.PieceOnSquare{
		sfPiece(base.King, base.White, 4), sfPiece(base.Pawn, base.White, 8), sfPiece(base.King, base.Black, 60),
	}}
	trace, err := model.evaluateAllTrace(position)
	if err != nil {
		t.Fatal(err)
	}
	if trace.psqtDifferenceWide[0] != 2147483648 || trace.psqtRaw[0] != -1073741824 {
		t.Fatalf("PSQT widened/wrapped difference = %d/%d", trace.psqtDifferenceWide[0], trace.psqtRaw[0])
	}
}

func TestFullRefreshUsesDefinedWrappingForBroadLoaderRange(t *testing.T) {
	model := blankEvaluationModel()
	model.featureBias[0] = 32766
	model.featureWeights[22468*transformerLanes] = 2
	model.psqtWeights[22468*psqtBuckets] = 2147483647
	model.psqtWeights[22524*psqtBuckets] = 1
	position := base.Position{SideToMove: base.White, Pieces: []base.PieceOnSquare{
		sfPiece(base.King, base.White, 4), sfPiece(base.King, base.Black, 60),
	}}
	trace, err := model.evaluateAllTrace(position)
	if err != nil {
		t.Fatal(err)
	}
	for perspective := 0; perspective < 2; perspective++ {
		if trace.accumulator.values[perspective][0] != -32768 {
			t.Fatalf("perspective %d int16 accumulator = %d", perspective, trace.accumulator.values[perspective][0])
		}
		if trace.accumulator.psqt[perspective][0] != -2147483648 {
			t.Fatalf("perspective %d int32 PSQT accumulator = %d", perspective, trace.accumulator.psqt[perspective][0])
		}
		if trace.accumulator.wideMax[perspective] < 32768 || trace.accumulator.psqtWideMax[perspective] < 2147483648 {
			t.Fatalf("perspective %d widened extrema missed overflow: acc=%d psqt=%d", perspective, trace.accumulator.wideMax[perspective], trace.accumulator.psqtWideMax[perspective])
		}
	}
	stack := &model.stacks[0]
	stack.fc0Bias[0] = 2147483647
	stack.fc0Weight[0] = 127
	var input [transformerLanes]uint8
	input[0] = 255
	stackTrace := model.propagateStack(0, &input)
	if stackTrace.fc0[0] != -2147451264 || stackTrace.fc0WideMax != 2147516032 {
		t.Fatalf("wrapped/widened FC0 = %d/%d", stackTrace.fc0[0], stackTrace.fc0WideMax)
	}
}

func TestConcurrentImmutableEvaluateAll(t *testing.T) {
	model := blankEvaluationModel()
	position := base.Position{SideToMove: base.White, Pieces: []base.PieceOnSquare{
		sfPiece(base.King, base.White, 4), sfPiece(base.Pawn, base.White, 8), sfPiece(base.King, base.Black, 60),
	}}
	want, err := model.EvaluateAll(position)
	if err != nil {
		t.Fatal(err)
	}
	errorsSeen := make(chan error, 8)
	for worker := 0; worker < 8; worker++ {
		go func() {
			for iteration := 0; iteration < 16; iteration++ {
				got, err := model.EvaluateAll(position)
				if err != nil {
					errorsSeen <- err
					return
				}
				if got != want {
					errorsSeen <- errors.New("concurrent result changed")
					return
				}
			}
			errorsSeen <- nil
		}()
	}
	for worker := 0; worker < 8; worker++ {
		if err := <-errorsSeen; err != nil {
			t.Fatal(err)
		}
	}
}

func TestDefinedWrappingAndTruncation(t *testing.T) {
	if got := wrapAdd16(32767, 1); got != -32768 {
		t.Fatalf("wrapAdd16 = %d", got)
	}
	if got := wrapAdd32(2147483647, 1); got != -2147483648 {
		t.Fatalf("wrapAdd32 = %d", got)
	}
	if got := wrapSub32(-2147483648, 1); got != 2147483647 {
		t.Fatalf("wrapSub32 = %d", got)
	}
	if got := wrapMul32(2147483647, 2); got != -2 {
		t.Fatalf("wrapMul32 = %d", got)
	}
	if got := squaredClippedReLU(-8192); got != 127 {
		t.Fatalf("negative squared activation = %d", got)
	}
	if got := clippedReLU(-1); got != 0 {
		t.Fatalf("negative clipped activation = %d", got)
	}
	if got := int32(-17) / 16; got != -1 {
		t.Fatalf("signed division = %d", got)
	}
}
