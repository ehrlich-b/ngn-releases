package countereval

import (
	"bytes"
	"math"
	"reflect"
	"testing"
)

func TestContextTransitionOrderAndState(t *testing.T) {
	model := deterministicContextModel()
	cases := []struct {
		name    string
		root    Board
		delta   MoveDelta
		post    Board
		updates []featureUpdate
	}{
		{
			name:    "capture",
			root:    boardWith(pieceAt(0, 28), pieceAt(6, 35)),
			delta:   MoveDelta{MovingPlane: 0, From: 28, To: 35, HasCapture: true, CapturedPlane: 6, CaptureSquare: 35},
			post:    boardWith(pieceAt(0, 35)),
			updates: []featureUpdate{{feature: 28}, {feature: 6*64 + 35}, {feature: 35, add: true}},
		},
		{
			name:    "en_passant",
			root:    boardWith(pieceAt(0, 36), pieceAt(6, 35)),
			delta:   MoveDelta{MovingPlane: 0, From: 36, To: 43, HasCapture: true, CapturedPlane: 6, CaptureSquare: 35},
			post:    boardWith(pieceAt(0, 43)),
			updates: []featureUpdate{{feature: 36}, {feature: 6*64 + 35}, {feature: 43, add: true}},
		},
		{
			name: "promotion_capture",
			root: boardWith(pieceAt(0, 54), pieceAt(9, 63)),
			delta: MoveDelta{
				MovingPlane: 0, From: 54, To: 63,
				HasCapture: true, CapturedPlane: 9, CaptureSquare: 63,
				HasPromotion: true, PromotionPlane: 4,
			},
			post:    boardWith(pieceAt(4, 63)),
			updates: []featureUpdate{{feature: 54}, {feature: 9*64 + 63}, {feature: 4*64 + 63, add: true}},
		},
		{
			name:    "black_promotion",
			root:    boardWith(pieceAt(6, 9)),
			delta:   MoveDelta{MovingPlane: 6, From: 9, To: 1, HasPromotion: true, PromotionPlane: 7},
			post:    boardWith(pieceAt(7, 1)),
			updates: []featureUpdate{{feature: 6*64 + 9}, {feature: 7*64 + 1, add: true}},
		},
		{
			name:  "castle",
			root:  boardWith(pieceAt(5, 4), pieceAt(3, 7)),
			delta: MoveDelta{MovingPlane: 5, From: 4, To: 6, HasCastleRook: true, CastleRookFrom: 7, CastleRookTo: 5},
			post:  boardWith(pieceAt(5, 6), pieceAt(3, 5)),
			updates: []featureUpdate{
				{feature: 5*64 + 4},
				{feature: 5*64 + 6, add: true},
				{feature: 3*64 + 7},
				{feature: 3*64 + 5, add: true},
			},
		},
	}

	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			gotPost, gotUpdates, gotUpdateCount, err := deriveTransition(item.root, item.delta)
			if err != nil {
				t.Fatal(err)
			}
			if gotPost != item.post || !reflect.DeepEqual(gotUpdates[:gotUpdateCount], item.updates) {
				t.Fatalf("transition differs: post=%v updates=%v", gotPost, gotUpdates)
			}
			context, err := model.NewContext(item.root)
			if err != nil {
				t.Fatal(err)
			}
			wantAccumulator := context.accumulators[0]
			applyTestUpdates(model, &wantAccumulator, item.updates)
			if err := context.PushMove(item.delta, item.post); err != nil {
				t.Fatal(err)
			}
			if context.Depth() != 1 || context.boards[1] != item.post {
				t.Fatalf("context state depth=%d board=%v", context.Depth(), context.boards[1])
			}
			requireAccumulatorBits(t, context.accumulators[1], wantAccumulator)
			gotRaw := context.EvaluateRaw()
			wantRaw := model.evaluateAccumulator(&wantAccumulator)
			if math.Float32bits(gotRaw) != math.Float32bits(wantRaw) {
				t.Fatalf("raw bits=%08x want=%08x", math.Float32bits(gotRaw), math.Float32bits(wantRaw))
			}
		})
	}
}

func TestContextNullPopCapacityAndPublicBoardSize(t *testing.T) {
	model := deterministicContextModel()
	var full Board
	full[0] = ^uint64(0)
	context, err := model.NewContext(full)
	if err != nil {
		t.Fatalf("public 64-piece board rejected: %v", err)
	}
	rootAccumulator := context.accumulators[0]
	if err := context.PushNull(); err != nil {
		t.Fatal(err)
	}
	if context.boards[1] != full {
		t.Fatal("null changed board")
	}
	requireAccumulatorBits(t, context.accumulators[1], rootAccumulator)
	if err := context.Pop(); err != nil {
		t.Fatal(err)
	}
	requireAccumulatorBits(t, context.accumulators[0], rootAccumulator)
	if err := context.Pop(); err == nil {
		t.Fatal("root pop succeeded")
	}

	for context.Depth() < CompatibilityFrameCount-1 {
		if err := context.PushNull(); err != nil {
			t.Fatalf("push at depth %d: %v", context.Depth(), err)
		}
	}
	before := *context
	if err := context.PushNull(); err == nil {
		t.Fatal("overflow push succeeded")
	}
	if !reflect.DeepEqual(*context, before) {
		t.Fatal("overflow push mutated context")
	}
}

func TestContextRejectsLoaderCompatibleUnsafeIncrementalRange(t *testing.T) {
	cases := []struct {
		name   string
		values map[int]float32
	}{
		{
			name:   "accumulator",
			values: map[int]float32{hiddenWeightsStart: 2e35},
		},
		{
			name: "output",
			values: map[int]float32{
				hiddenWeightsStart: 1e20,
				outputWeightsStart: 2e15,
			},
		},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			model, _, err := LoadCounter55Legacy(bytes.NewReader(makeLegacyFile(item.values)))
			if err != nil {
				t.Fatalf("loader unexpectedly rejected full-refresh-safe model: %v", err)
			}
			if _, err := model.NewContext(Board{}); err == nil {
				t.Fatal("incremental context accepted unsafe maximum path")
			}
		})
	}
}

func TestContextRejectedTransitionsAreTransactional(t *testing.T) {
	model := deterministicContextModel()
	root := boardWith(pieceAt(0, 28), pieceAt(6, 35), pieceAt(5, 4), pieceAt(3, 7))
	validPost := boardWith(pieceAt(0, 35), pieceAt(5, 4), pieceAt(3, 7))
	valid := MoveDelta{MovingPlane: 0, From: 28, To: 35, HasCapture: true, CapturedPlane: 6, CaptureSquare: 35}
	cases := []struct {
		name  string
		delta MoveDelta
		post  Board
	}{
		{"missing_mover", MoveDelta{MovingPlane: 0, From: 27, To: 35}, root},
		{"same_color_capture", MoveDelta{MovingPlane: 0, From: 28, To: 7, HasCapture: true, CapturedPlane: 3, CaptureSquare: 7}, root},
		{"wrong_capture_square", MoveDelta{MovingPlane: 0, From: 28, To: 35, HasCapture: true, CapturedPlane: 6, CaptureSquare: 34}, root},
		{"nonpawn_displaced_capture", MoveDelta{MovingPlane: 3, From: 7, To: 15, HasCapture: true, CapturedPlane: 6, CaptureSquare: 35}, root},
		{"bad_promotion", MoveDelta{MovingPlane: 0, From: 28, To: 36, HasPromotion: true, PromotionPlane: 10}, root},
		{"bad_castle_piece", MoveDelta{MovingPlane: 0, From: 28, To: 30, HasCastleRook: true, CastleRookFrom: 7, CastleRookTo: 29}, root},
		{"post_mismatch", valid, root},
		{"overlapping_post", valid, overlapBoard(validPost)},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			context, err := model.NewContext(root)
			if err != nil {
				t.Fatal(err)
			}
			before := *context
			if err := context.PushMove(item.delta, item.post); err == nil {
				t.Fatal("invalid transition succeeded")
			}
			if !reflect.DeepEqual(*context, before) {
				t.Fatal("rejected transition mutated context")
			}
		})
	}

	context, err := model.NewContext(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := context.PushMove(valid, validPost); err != nil {
		t.Fatalf("valid transition rejected: %v", err)
	}
}

func deterministicContextModel() *Model {
	model := &Model{}
	for hidden := 0; hidden < HiddenSize; hidden++ {
		model.hiddenBiases[hidden] = float32((hidden%7)-3) * 0.25
		model.outputWeights[hidden] = float32((hidden%11)-5) * 0.03125
	}
	model.outputBias = 0.375
	for feature := 0; feature < InputSize; feature++ {
		for hidden := 0; hidden < HiddenSize; hidden++ {
			model.hiddenWeights[feature*HiddenSize+hidden] =
				float32(((feature*17+hidden*13)%41)-20) * 0.015625
		}
	}
	return model
}

func pieceAt(plane, square int) Board {
	var board Board
	board[plane] = uint64(1) << square
	return board
}

func boardWith(parts ...Board) Board {
	var board Board
	for _, part := range parts {
		for plane := range board {
			board[plane] |= part[plane]
		}
	}
	return board
}

func overlapBoard(board Board) Board {
	for plane, pieces := range board {
		if pieces != 0 {
			square := uint(bitsTrailingZeroes(pieces))
			board[(plane+1)%FeaturePlaneCount] |= uint64(1) << square
			return board
		}
	}
	return board
}

func bitsTrailingZeroes(value uint64) int {
	for square := 0; square < 64; square++ {
		if value&(uint64(1)<<square) != 0 {
			return square
		}
	}
	return 64
}

func applyTestUpdates(model *Model, accumulator *[HiddenSize]float32, updates []featureUpdate) {
	for _, update := range updates {
		base := update.feature * HiddenSize
		for hidden := 0; hidden < HiddenSize; hidden++ {
			if update.add {
				accumulator[hidden] += model.hiddenWeights[base+hidden]
			} else {
				accumulator[hidden] -= model.hiddenWeights[base+hidden]
			}
		}
	}
}

func requireAccumulatorBits(t *testing.T, got, want [HiddenSize]float32) {
	t.Helper()
	for lane := range got {
		if math.Float32bits(got[lane]) != math.Float32bits(want[lane]) {
			t.Fatalf("lane %d bits=%08x want=%08x", lane, math.Float32bits(got[lane]), math.Float32bits(want[lane]))
		}
	}
}

func TestContextArithmeticMutationWitnesses(t *testing.T) {
	model := &Model{}
	const captureFeature = 6*64 + 35
	const addFeature = 35
	model.hiddenWeights[captureFeature*HiddenSize] = 1e20
	model.hiddenWeights[addFeature*HiddenSize] = 1

	start := [HiddenSize]float32{1e20}
	correct := start
	applyTestUpdates(model, &correct, []featureUpdate{
		{feature: captureFeature},
		{feature: addFeature, add: true},
	})
	swapped := start
	applyTestUpdates(model, &swapped, []featureUpdate{
		{feature: addFeature, add: true},
		{feature: captureFeature},
	})
	if math.Float32bits(correct[0]) == math.Float32bits(swapped[0]) {
		t.Fatal("capture/add order mutant has no float32 witness")
	}

	model = &Model{}
	const destinationFeature = 12
	model.hiddenWeights[destinationFeature*HiddenSize] = 1e20
	parent := [HiddenSize]float32{1}
	child := parent
	applyTestUpdates(model, &child, []featureUpdate{{feature: destinationFeature, add: true}})
	inverseUnmake := child
	applyTestUpdates(model, &inverseUnmake, []featureUpdate{{feature: destinationFeature}})
	if math.Float32bits(inverseUnmake[0]) == math.Float32bits(parent[0]) {
		t.Fatal("inverse-arithmetic unmake mutant has no float32 witness")
	}
}
