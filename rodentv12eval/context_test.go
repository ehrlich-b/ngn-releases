package rodentv12eval

import (
	"math"
	"strings"
	"testing"
)

type contextPiece struct {
	plane  int
	square int
}

func contextPosition(side Color, pieces ...contextPiece) Position {
	var position Position
	position.SideToMove = side
	for _, piece := range pieces {
		position.Board[piece.plane] |= uint64(1) << piece.square
	}
	return position
}

func contextPost(root Position, remove, add []contextPiece) Position {
	post := root
	for _, piece := range remove {
		post.Board[piece.plane] &^= uint64(1) << piece.square
	}
	for _, piece := range add {
		post.Board[piece.plane] |= uint64(1) << piece.square
	}
	post.SideToMove ^= 1
	return post
}

func incrementalTestModel() *Model {
	model := testModel()
	for hidden := 0; hidden < HiddenSize; hidden++ {
		model.inputBiases[hidden] = int16(uint16(hidden*251 + 0x8001))
	}
	for feature := 0; feature < TotalInputFeatures; feature++ {
		for hidden := 0; hidden < HiddenSize; hidden++ {
			value := uint32(feature*HiddenSize+hidden)*1664525 + 1013904223
			model.inputWeights[feature][hidden] = int16(uint16(value >> 8))
		}
	}
	for bucket := 0; bucket < OutputBuckets; bucket++ {
		for perspective := 0; perspective < 2; perspective++ {
			for hidden := 0; hidden < HiddenSize; hidden++ {
				value := bucket*1009 + perspective*503 + hidden*97 + 31
				model.outputWeights[bucket][perspective][hidden] = int16(uint16(value))
			}
		}
		model.outputBiases[bucket] = int16(bucket*37 - 117)
	}
	return model
}

func poisonContextFrame(context *SearchContext, frame int) {
	for perspective := White; perspective <= Black; perspective++ {
		for hidden := 0; hidden < HiddenSize; hidden++ {
			context.accumulators[frame][perspective][hidden] = int16(uint16(0x5a5a + hidden + int(perspective)))
		}
	}
	context.positions[frame] = Position{SideToMove: Color(7)}
}

func assertAccumulatorEqual(t *testing.T, got, want accumulator) {
	t.Helper()
	if got == want {
		return
	}
	for perspective := White; perspective <= Black; perspective++ {
		for hidden := 0; hidden < HiddenSize; hidden++ {
			if got[perspective][hidden] != want[perspective][hidden] {
				t.Fatalf("accumulator perspective %d lane %d = %d, want %d", perspective, hidden, got[perspective][hidden], want[perspective][hidden])
			}
		}
	}
}

func assertContextMatchesRefresh(t *testing.T, context *SearchContext) {
	t.Helper()
	position := context.positions[context.depth]
	assertAccumulatorEqual(t, context.accumulators[context.depth], context.model.fullRefresh(position.Board))
	wantRaw, err := context.model.EvaluateRaw(position)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := context.EvaluateRaw(); err != nil || got != wantRaw {
		t.Fatalf("incremental raw = %d, %v; full refresh = %d", got, err, wantRaw)
	}
	wantStatic, err := context.model.EvaluateReleaseStatic(position)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := context.EvaluateReleaseStatic(); err != nil || got != wantStatic {
		t.Fatalf("incremental static = %d, %v; full refresh = %d", got, err, wantStatic)
	}
}

func runContextTransition(t *testing.T, model *Model, root Position, delta MoveDelta, post Position) {
	t.Helper()
	context, err := model.NewSearchContext(root)
	if err != nil {
		t.Fatal(err)
	}
	parentAccumulator := context.accumulators[0]
	parentPosition := context.positions[0]
	poisonContextFrame(context, 1)

	if err := context.PushMove(delta, post); err != nil {
		t.Fatal(err)
	}
	if context.Depth() != 1 || context.Position() != post {
		t.Fatalf("child state depth/position = %d/%+v, want 1/%+v", context.Depth(), context.Position(), post)
	}
	if context.accumulators[0] != parentAccumulator || context.positions[0] != parentPosition {
		t.Fatal("push mutated parent frame")
	}
	assertContextMatchesRefresh(t, context)
	if err := context.Pop(); err != nil {
		t.Fatal(err)
	}
	if context.Depth() != 0 || context.Position() != root || context.accumulators[0] != parentAccumulator {
		t.Fatal("pop did not restore exact parent frame")
	}
}

func TestSearchContextMoveFamiliesMatchFullRefreshAllLanes(t *testing.T) {
	model := incrementalTestModel()
	tests := []struct {
		name  string
		root  Position
		delta MoveDelta
		post  Position
	}{
		{
			name: "white quiet",
			root: contextPosition(White,
				contextPiece{WhiteKing, 4}, contextPiece{BlackKing, 60}, contextPiece{WhiteKnight, 1}),
			delta: MoveDelta{MovingPlane: WhiteKnight, From: 1, To: 18},
			post: contextPosition(Black,
				contextPiece{WhiteKing, 4}, contextPiece{BlackKing, 60}, contextPiece{WhiteKnight, 18}),
		},
		{
			name: "black double pawn",
			root: contextPosition(Black,
				contextPiece{WhiteKing, 4}, contextPiece{BlackKing, 60}, contextPiece{BlackPawn, 51}),
			delta: MoveDelta{MovingPlane: BlackPawn, From: 51, To: 35},
			post: contextPosition(White,
				contextPiece{WhiteKing, 4}, contextPiece{BlackKing, 60}, contextPiece{BlackPawn, 35}),
		},
		{
			name: "capture crosses output bucket threshold",
			root: contextPosition(White,
				contextPiece{WhiteKing, 4}, contextPiece{BlackKing, 60},
				contextPiece{WhiteBishop, 26}, contextPiece{BlackKnight, 35},
				contextPiece{WhitePawn, 8}, contextPiece{BlackPawn, 48}),
			delta: MoveDelta{MovingPlane: WhiteBishop, From: 26, To: 35, HasCapture: true, CapturedPlane: BlackKnight, CaptureSquare: 35},
			post: contextPosition(Black,
				contextPiece{WhiteKing, 4}, contextPiece{BlackKing, 60},
				contextPiece{WhiteBishop, 35}, contextPiece{WhitePawn, 8}, contextPiece{BlackPawn, 48}),
		},
		{
			name: "white en passant",
			root: contextPosition(White,
				contextPiece{WhiteKing, 4}, contextPiece{BlackKing, 60}, contextPiece{WhitePawn, 36}, contextPiece{BlackPawn, 35}),
			delta: MoveDelta{MovingPlane: WhitePawn, From: 36, To: 43, HasCapture: true, CapturedPlane: BlackPawn, CaptureSquare: 35},
			post: contextPosition(Black,
				contextPiece{WhiteKing, 4}, contextPiece{BlackKing, 60}, contextPiece{WhitePawn, 43}),
		},
		{
			name: "black en passant",
			root: contextPosition(Black,
				contextPiece{WhiteKing, 4}, contextPiece{BlackKing, 60}, contextPiece{BlackPawn, 27}, contextPiece{WhitePawn, 28}),
			delta: MoveDelta{MovingPlane: BlackPawn, From: 27, To: 20, HasCapture: true, CapturedPlane: WhitePawn, CaptureSquare: 28},
			post: contextPosition(White,
				contextPiece{WhiteKing, 4}, contextPiece{BlackKing, 60}, contextPiece{BlackPawn, 20}),
		},
		{
			name: "white king castle",
			root: contextPosition(White,
				contextPiece{WhiteKing, 4}, contextPiece{BlackKing, 60}, contextPiece{WhiteRook, 7}),
			delta: MoveDelta{MovingPlane: WhiteKing, From: 4, To: 6, HasCastleRook: true, CastleRookFrom: 7, CastleRookTo: 5},
			post: contextPosition(Black,
				contextPiece{WhiteKing, 6}, contextPiece{BlackKing, 60}, contextPiece{WhiteRook, 5}),
		},
		{
			name: "white queen castle",
			root: contextPosition(White,
				contextPiece{WhiteKing, 4}, contextPiece{BlackKing, 60}, contextPiece{WhiteRook, 0}),
			delta: MoveDelta{MovingPlane: WhiteKing, From: 4, To: 2, HasCastleRook: true, CastleRookFrom: 0, CastleRookTo: 3},
			post: contextPosition(Black,
				contextPiece{WhiteKing, 2}, contextPiece{BlackKing, 60}, contextPiece{WhiteRook, 3}),
		},
		{
			name: "black king castle",
			root: contextPosition(Black,
				contextPiece{WhiteKing, 4}, contextPiece{BlackKing, 60}, contextPiece{BlackRook, 63}),
			delta: MoveDelta{MovingPlane: BlackKing, From: 60, To: 62, HasCastleRook: true, CastleRookFrom: 63, CastleRookTo: 61},
			post: contextPosition(White,
				contextPiece{WhiteKing, 4}, contextPiece{BlackKing, 62}, contextPiece{BlackRook, 61}),
		},
		{
			name: "black queen castle",
			root: contextPosition(Black,
				contextPiece{WhiteKing, 4}, contextPiece{BlackKing, 60}, contextPiece{BlackRook, 56}),
			delta: MoveDelta{MovingPlane: BlackKing, From: 60, To: 58, HasCastleRook: true, CastleRookFrom: 56, CastleRookTo: 59},
			post: contextPosition(White,
				contextPiece{WhiteKing, 4}, contextPiece{BlackKing, 58}, contextPiece{BlackRook, 59}),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runContextTransition(t, model, test.root, test.delta, test.post)
		})
	}
}

func TestSearchContextAllPromotionsMatchFullRefreshAllLanes(t *testing.T) {
	model := incrementalTestModel()
	for _, color := range []Color{White, Black} {
		for pieceType := 1; pieceType <= 4; pieceType++ {
			for _, capture := range []bool{false, true} {
				name := []string{"white", "black"}[color] + "_type_" + string(rune('0'+pieceType))
				if capture {
					name += "_capture"
				}
				t.Run(name, func(t *testing.T) {
					movingPlane := int(color) * colorPlanes
					targetPlane := movingPlane + pieceType
					from, to := 48, 56
					rootPieces := []contextPiece{{WhiteKing, 4}, {BlackKing, 60}, {movingPlane, from}}
					remove := []contextPiece{{movingPlane, from}}
					delta := MoveDelta{MovingPlane: uint8(movingPlane), From: uint8(from), To: uint8(to), HasPromotion: true, PromotionPlane: uint8(targetPlane)}
					if color == Black {
						from, to = 8, 0
						rootPieces[2].square = from
						remove[0].square = from
						delta.From, delta.To = uint8(from), uint8(to)
					}
					if capture {
						to ^= 1
						delta.To = uint8(to)
						capturedPlane := BlackRook
						if color == Black {
							capturedPlane = WhiteRook
						}
						rootPieces = append(rootPieces, contextPiece{capturedPlane, to})
						remove = append(remove, contextPiece{capturedPlane, to})
						delta.HasCapture = true
						delta.CapturedPlane = uint8(capturedPlane)
						delta.CaptureSquare = uint8(to)
					}
					root := contextPosition(color, rootPieces...)
					post := contextPost(root, remove, []contextPiece{{targetPlane, to}})
					runContextTransition(t, model, root, delta, post)
				})
			}
		}
	}
}

func TestDeriveTransitionKingRefreshClassification(t *testing.T) {
	tests := []struct {
		name        string
		color       Color
		from, to    int
		wantRefresh Color
	}{
		{name: "white bucket zero to one", color: White, from: 2, to: 1, wantRefresh: White},
		{name: "white bucket one to two", color: White, from: 0, to: 8, wantRefresh: White},
		{name: "white bucket two to three", color: White, from: 8, to: 16, wantRefresh: White},
		{name: "white mirror d to e", color: White, from: 3, to: 4, wantRefresh: White},
		{name: "white mirror e to d", color: White, from: 4, to: 3, wantRefresh: White},
		{name: "black bucket zero to one", color: Black, from: 58, to: 57, wantRefresh: Black},
		{name: "black bucket one to two", color: Black, from: 56, to: 48, wantRefresh: Black},
		{name: "black bucket two to three", color: Black, from: 48, to: 40, wantRefresh: Black},
		{name: "black mirror d to e", color: Black, from: 59, to: 60, wantRefresh: Black},
		{name: "black mirror e to d", color: Black, from: 60, to: 59, wantRefresh: Black},
		{name: "white same bucket and mirror", color: White, from: 16, to: 24, wantRefresh: Color(2)},
		{name: "black same bucket and mirror", color: Black, from: 47, to: 39, wantRefresh: Color(2)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pieces := []contextPiece{{WhiteKing, 4}, {BlackKing, 60}, {WhiteKnight, 18}, {BlackBishop, 45}}
			movingPlane := WhiteKing
			pieces[0].square = test.from
			if test.color == Black {
				movingPlane = BlackKing
				pieces[0].square = 4
				pieces[1].square = test.from
			}
			root := contextPosition(test.color, pieces...)
			post, _, _, refresh, err := deriveTransition(root, MoveDelta{MovingPlane: uint8(movingPlane), From: uint8(test.from), To: uint8(test.to)})
			if err != nil {
				t.Fatal(err)
			}
			if refresh != test.wantRefresh {
				t.Fatalf("refresh = %d, want %d", refresh, test.wantRefresh)
			}
			want := contextPost(root, []contextPiece{{movingPlane, test.from}}, []contextPiece{{movingPlane, test.to}})
			if post != want {
				t.Fatal("derived position differs")
			}
		})
	}
}

func TestSearchContextKingViewChangesMatchFullRefreshAllLanes(t *testing.T) {
	model := incrementalTestModel()
	tests := []struct {
		name     string
		color    Color
		from, to int
	}{
		{name: "white bucket zero to one", color: White, from: 2, to: 1},
		{name: "white bucket one to two", color: White, from: 0, to: 8},
		{name: "white bucket two to three", color: White, from: 8, to: 16},
		{name: "white mirror", color: White, from: 3, to: 4},
		{name: "white same view", color: White, from: 16, to: 24},
		{name: "black bucket zero to one", color: Black, from: 58, to: 57},
		{name: "black bucket one to two", color: Black, from: 56, to: 48},
		{name: "black bucket two to three", color: Black, from: 48, to: 40},
		{name: "black mirror", color: Black, from: 60, to: 59},
		{name: "black same view", color: Black, from: 47, to: 39},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pieces := []contextPiece{{WhiteKing, 4}, {BlackKing, 60}, {WhiteKnight, 18}, {BlackBishop, 45}, {WhitePawn, 10}, {BlackPawn, 50}}
			movingPlane := WhiteKing
			pieces[0].square = test.from
			if test.color == Black {
				movingPlane = BlackKing
				pieces[0].square = 4
				pieces[1].square = test.from
			}
			root := contextPosition(test.color, pieces...)
			post := contextPost(root, []contextPiece{{movingPlane, test.from}}, []contextPiece{{movingPlane, test.to}})
			runContextTransition(t, model, root, MoveDelta{MovingPlane: uint8(movingPlane), From: uint8(test.from), To: uint8(test.to)}, post)
		})
	}
}

func TestApplyUpdatesRefreshesOnlyChangedKingPerspective(t *testing.T) {
	model := incrementalTestModel()
	post := contextPosition(Black,
		contextPiece{WhiteKing, 8}, contextPiece{BlackKing, 60},
		contextPiece{WhiteKnight, 18}, contextPiece{BlackBishop, 45})
	var destination accumulator
	for perspective := White; perspective <= Black; perspective++ {
		for hidden := 0; hidden < HiddenSize; hidden++ {
			destination[perspective][hidden] = int16(uint16(0x4000 + hidden*17 + int(perspective)))
		}
	}
	blackBefore := destination[Black]
	updates := [4]featureUpdate{
		{plane: WhiteKing, square: 8, add: true},
		{plane: WhiteKing, square: 0},
	}

	model.applyUpdates(&destination, post.Board, &updates, 2, White)
	wantRefresh := model.fullRefresh(post.Board)
	if destination[White] != wantRefresh[White] {
		t.Fatal("changed White perspective was not fully refreshed")
	}
	blackKingSquare := 60
	toRow := &model.inputWeights[featureIndex(White, kingTypePlane, 8, blackKingSquare, Black)]
	fromRow := &model.inputWeights[featureIndex(White, kingTypePlane, 0, blackKingSquare, Black)]
	for hidden := 0; hidden < HiddenSize; hidden++ {
		want := blackBefore[hidden] + toRow[hidden] - fromRow[hidden]
		if destination[Black][hidden] != want {
			t.Fatalf("incremental Black perspective lane %d = %d, want %d", hidden, destination[Black][hidden], want)
		}
	}
	if destination[Black] == wantRefresh[Black] {
		t.Fatal("unchanged Black perspective was unexpectedly refreshed")
	}
}

func TestSearchContextNullGrowthResetAndIndependentContexts(t *testing.T) {
	model := incrementalTestModel()
	root := contextPosition(White,
		contextPiece{WhiteKing, 4}, contextPiece{BlackKing, 60},
		contextPiece{WhiteKnight, 1}, contextPiece{BlackKnight, 57})
	first, err := model.NewSearchContext(root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := model.NewSearchContext(root)
	if err != nil {
		t.Fatal(err)
	}
	rootAccumulator := first.accumulators[0]
	for push := 1; push <= initialSearchFrameCount; push++ {
		parent := first.accumulators[first.depth]
		if err := first.PushNull(); err != nil {
			t.Fatal(err)
		}
		if first.accumulators[first.depth] != parent || first.positions[first.depth].Board != root.Board {
			t.Fatalf("null push %d changed board or accumulator", push)
		}
	}
	if first.Depth() != initialSearchFrameCount || first.FrameCapacity() != 2*initialSearchFrameCount {
		t.Fatalf("growth depth/capacity = %d/%d", first.Depth(), first.FrameCapacity())
	}
	if second.Depth() != 0 || second.Position() != root || second.accumulators[0] != rootAccumulator {
		t.Fatal("first context growth mutated independent context")
	}
	for first.Depth() > 0 {
		if err := first.Pop(); err != nil {
			t.Fatal(err)
		}
	}
	newRoot := contextPosition(Black,
		contextPiece{WhiteKing, 3}, contextPiece{BlackKing, 56}, contextPiece{WhiteQueen, 27})
	if err := first.Reset(newRoot); err != nil {
		t.Fatal(err)
	}
	if first.Depth() != 0 || first.FrameCapacity() != 2*initialSearchFrameCount || first.Position() != newRoot {
		t.Fatal("reset did not replace root while retaining capacity")
	}
	assertContextMatchesRefresh(t, first)
	beforePosition := first.Position()
	beforeAccumulator := first.accumulators[0]
	beforeCapacity := first.FrameCapacity()
	invalid := newRoot
	invalid.Board[WhiteKing] = 0
	if err := first.Reset(invalid); err == nil {
		t.Fatal("invalid reset unexpectedly succeeded")
	}
	if first.Position() != beforePosition || first.accumulators[0] != beforeAccumulator || first.FrameCapacity() != beforeCapacity {
		t.Fatal("invalid reset mutated context")
	}
}

func TestSearchContextMultiPlyUnwindAndSiblingReuse(t *testing.T) {
	model := incrementalTestModel()
	root := contextPosition(White,
		contextPiece{WhiteKing, 3}, contextPiece{BlackKing, 60},
		contextPiece{WhiteKnight, 1}, contextPiece{BlackKnight, 57},
		contextPiece{WhitePawn, 12}, contextPiece{BlackPawn, 52})
	context, err := model.NewSearchContext(root)
	if err != nil {
		t.Fatal(err)
	}
	push := func(delta MoveDelta, post Position) {
		t.Helper()
		if err := context.PushMove(delta, post); err != nil {
			t.Fatal(err)
		}
		assertContextMatchesRefresh(t, context)
	}
	post := contextPost(root, []contextPiece{{WhiteKing, 3}}, []contextPiece{{WhiteKing, 4}})
	push(MoveDelta{MovingPlane: WhiteKing, From: 3, To: 4}, post)
	post = contextPost(post, []contextPiece{{BlackKnight, 57}}, []contextPiece{{BlackKnight, 42}})
	push(MoveDelta{MovingPlane: BlackKnight, From: 57, To: 42}, post)
	branchRoot := post
	post = contextPost(post, []contextPiece{{WhiteKnight, 1}}, []contextPiece{{WhiteKnight, 18}})
	push(MoveDelta{MovingPlane: WhiteKnight, From: 1, To: 18}, post)
	if err := context.Pop(); err != nil {
		t.Fatal(err)
	}
	if context.Position() != branchRoot {
		t.Fatal("pop did not restore branch root")
	}
	post = contextPost(branchRoot, []contextPiece{{WhitePawn, 12}}, []contextPiece{{WhitePawn, 20}})
	push(MoveDelta{MovingPlane: WhitePawn, From: 12, To: 20}, post)
	post = contextPost(post, []contextPiece{{BlackPawn, 52}}, []contextPiece{{BlackPawn, 36}})
	push(MoveDelta{MovingPlane: BlackPawn, From: 52, To: 36}, post)
	assertContextMatchesRefresh(t, context)
}

func TestSearchContextRejectedMovesAreTransactional(t *testing.T) {
	model := incrementalTestModel()
	root := contextPosition(White,
		contextPiece{WhiteKing, 4}, contextPiece{BlackKing, 60},
		contextPiece{WhiteKnight, 1}, contextPiece{BlackKnight, 18})
	tests := []struct {
		name  string
		delta MoveDelta
		post  Position
		want  string
	}{
		{name: "wrong mover", delta: MoveDelta{MovingPlane: BlackKnight, From: 18, To: 33}, post: root, want: "mover color"},
		{name: "missing mover", delta: MoveDelta{MovingPlane: WhiteBishop, From: 2, To: 11}, post: root, want: "has no piece"},
		{name: "occupied destination", delta: MoveDelta{MovingPlane: WhiteKnight, From: 1, To: 18}, post: root, want: "occupied"},
		{name: "capture fields without flag", delta: MoveDelta{MovingPlane: WhiteKnight, From: 1, To: 16, CapturedPlane: BlackKnight, CaptureSquare: 18}, post: root, want: "without capture"},
		{name: "invalid promotion", delta: MoveDelta{MovingPlane: WhiteKnight, From: 1, To: 16, HasPromotion: true, PromotionPlane: WhiteQueen}, post: root, want: "invalid promotion"},
		{name: "mismatched expected", delta: MoveDelta{MovingPlane: WhiteKnight, From: 1, To: 16}, post: root, want: "derived post"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			context, err := model.NewSearchContext(root)
			if err != nil {
				t.Fatal(err)
			}
			beforeAccumulator := context.accumulators[0]
			beforeCapacity := context.FrameCapacity()
			if err := context.PushMove(test.delta, test.post); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("PushMove error = %v, want substring %q", err, test.want)
			}
			if context.Depth() != 0 || context.Position() != root || context.accumulators[0] != beforeAccumulator || context.FrameCapacity() != beforeCapacity {
				t.Fatal("rejected move mutated context")
			}
		})
	}
	context, err := model.NewSearchContext(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := context.Pop(); err == nil || !strings.Contains(err.Error(), "cannot pop root") {
		t.Fatalf("root Pop error = %v", err)
	}
}

func TestSearchContextIncrementalArithmeticWrapsInt16(t *testing.T) {
	model := testModel()
	root := contextPosition(White,
		contextPiece{WhiteKing, 4}, contextPiece{BlackKing, 60}, contextPiece{WhiteKnight, 1})
	post := contextPosition(Black,
		contextPiece{WhiteKing, 4}, contextPiece{BlackKing, 60}, contextPiece{WhiteKnight, 18})
	model.inputBiases[0] = math.MaxInt16 - 7
	for perspective := White; perspective <= Black; perspective++ {
		kingSquare := 4
		if perspective == Black {
			kingSquare = 60
		}
		from := featureIndex(White, 1, 1, kingSquare, perspective)
		to := featureIndex(White, 1, 18, kingSquare, perspective)
		model.inputWeights[from][0] = -20
		model.inputWeights[to][0] = 100
	}
	context, err := model.NewSearchContext(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := context.PushMove(MoveDelta{MovingPlane: WhiteKnight, From: 1, To: 18}, post); err != nil {
		t.Fatal(err)
	}
	assertAccumulatorEqual(t, context.accumulators[1], model.fullRefresh(post.Board))
	for perspective := White; perspective <= Black; perspective++ {
		if got := context.accumulators[1][perspective][0]; got != -32676 {
			t.Fatalf("perspective %d wrapped lane = %d, want -32676", perspective, got)
		}
	}
}
