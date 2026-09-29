package rodenteval

import (
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
	for feature := 0; feature < InputSize; feature++ {
		for hidden := 0; hidden < HiddenSize; hidden++ {
			value := uint32(feature*HiddenSize+hidden)*1664525 + 1013904223
			model.inputWeights[feature][hidden] = int16(uint16(value >> 8))
		}
	}
	for hidden := 0; hidden < HiddenSize; hidden++ {
		model.outputWeights[0][hidden] = int16(uint16(hidden*97 + 31))
		model.outputWeights[1][hidden] = int16(uint16(hidden*193 + 17))
	}
	model.outputBias = -117
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
	wantAccumulator := model.fullRefresh(post.Board)
	assertAccumulatorEqual(t, context.accumulators[1], wantAccumulator)
	wantRaw, err := model.EvaluateRaw(post)
	if err != nil {
		t.Fatal(err)
	}
	if gotRaw, err := context.EvaluateRaw(); err != nil || gotRaw != wantRaw {
		t.Fatalf("incremental raw = %d, %v; full refresh = %d", gotRaw, err, wantRaw)
	}
	wantStatic, err := model.EvaluateReleaseStatic(post)
	if err != nil {
		t.Fatal(err)
	}
	if gotStatic, err := context.EvaluateReleaseStatic(); err != nil || gotStatic != wantStatic {
		t.Fatalf("incremental static = %d, %v; full refresh = %d", gotStatic, err, wantStatic)
	}
	if err := context.Pop(); err != nil {
		t.Fatal(err)
	}
	if context.Depth() != 0 || context.Position() != root || context.accumulators[0] != parentAccumulator {
		t.Fatal("pop did not restore exact parent frame")
	}
}

func TestSearchContextMoveFamiliesMatchFullRefreshAllLanes(t *testing.T) {
	model := incrementalTestModel()
	modelBefore := *model
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
			name: "ordinary capture",
			root: contextPosition(White,
				contextPiece{WhiteKing, 4}, contextPiece{BlackKing, 60}, contextPiece{WhiteBishop, 26}, contextPiece{BlackKnight, 35}),
			delta: MoveDelta{MovingPlane: WhiteBishop, From: 26, To: 35, HasCapture: true, CapturedPlane: BlackKnight, CaptureSquare: 35},
			post: contextPosition(Black,
				contextPiece{WhiteKing, 4}, contextPiece{BlackKing, 60}, contextPiece{WhiteBishop, 35}),
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
			name: "white queen castle crosses mirror",
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
			name: "black queen castle crosses mirror",
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
	if *model != modelBefore {
		t.Fatal("incremental transitions mutated immutable model")
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

func TestSearchContextKingMirrorBoundaryBothDirectionsAndColors(t *testing.T) {
	model := incrementalTestModel()
	tests := []struct {
		name         string
		color        Color
		from, to     int
		movingPlane  int
		friendlyPawn int
	}{
		{name: "white d to e", color: White, from: 3, to: 4, movingPlane: WhiteKing, friendlyPawn: 10},
		{name: "white e to d", color: White, from: 4, to: 3, movingPlane: WhiteKing, friendlyPawn: 10},
		{name: "black d to e", color: Black, from: 59, to: 60, movingPlane: BlackKing, friendlyPawn: 50},
		{name: "black e to d", color: Black, from: 60, to: 59, movingPlane: BlackKing, friendlyPawn: 50},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pieces := []contextPiece{{WhiteKing, 4}, {BlackKing, 60}, {WhiteKnight, 18}, {BlackBishop, 45}}
			pieces[test.color] = contextPiece{test.movingPlane, test.from}
			pawnPlane := WhitePawn
			if test.color == Black {
				pawnPlane = BlackPawn
			}
			pieces = append(pieces, contextPiece{pawnPlane, test.friendlyPawn})
			root := contextPosition(test.color, pieces...)
			post := contextPost(root,
				[]contextPiece{{test.movingPlane, test.from}},
				[]contextPiece{{test.movingPlane, test.to}})
			delta := MoveDelta{MovingPlane: uint8(test.movingPlane), From: uint8(test.from), To: uint8(test.to)}
			runContextTransition(t, model, root, delta, post)
		})
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
	positions := []Position{root}
	accumulators := []accumulator{context.accumulators[0]}
	push := func(delta MoveDelta, post Position) {
		t.Helper()
		if err := context.PushMove(delta, post); err != nil {
			t.Fatal(err)
		}
		assertAccumulatorEqual(t, context.accumulators[context.depth], model.fullRefresh(post.Board))
		for depth := range positions {
			if context.positions[depth] != positions[depth] || context.accumulators[depth] != accumulators[depth] {
				t.Fatalf("push at depth %d mutated ancestor frame %d", context.depth, depth)
			}
		}
		positions = append(positions, post)
		accumulators = append(accumulators, context.accumulators[context.depth])
	}
	post := contextPost(root, []contextPiece{{WhiteKing, 3}}, []contextPiece{{WhiteKing, 4}})
	push(MoveDelta{MovingPlane: WhiteKing, From: 3, To: 4}, post)
	post = contextPost(post, []contextPiece{{BlackKnight, 57}}, []contextPiece{{BlackKnight, 42}})
	push(MoveDelta{MovingPlane: BlackKnight, From: 57, To: 42}, post)
	branchRoot := post
	post = contextPost(post, []contextPiece{{WhiteKnight, 1}}, []contextPiece{{WhiteKnight, 18}})
	push(MoveDelta{MovingPlane: WhiteKnight, From: 1, To: 18}, post)
	post = contextPost(post, []contextPiece{{BlackKing, 60}}, []contextPiece{{BlackKing, 59}})
	push(MoveDelta{MovingPlane: BlackKing, From: 60, To: 59}, post)
	post = contextPost(post, []contextPiece{{WhitePawn, 12}}, []contextPiece{{WhitePawn, 28}})
	push(MoveDelta{MovingPlane: WhitePawn, From: 12, To: 28}, post)

	for context.Depth() > 2 {
		if err := context.Pop(); err != nil {
			t.Fatal(err)
		}
		positions = positions[:len(positions)-1]
		accumulators = accumulators[:len(accumulators)-1]
		if context.Position() != positions[len(positions)-1] {
			t.Fatalf("unwind depth %d restored wrong position", context.Depth())
		}
		assertAccumulatorEqual(t, context.accumulators[context.depth], accumulators[len(accumulators)-1])
	}

	// First sibling overwrites the old depth-3/4 frames with a pawn branch.
	post = contextPost(branchRoot, []contextPiece{{WhitePawn, 12}}, []contextPiece{{WhitePawn, 20}})
	push(MoveDelta{MovingPlane: WhitePawn, From: 12, To: 20}, post)
	post = contextPost(post, []contextPiece{{BlackPawn, 52}}, []contextPiece{{BlackPawn, 36}})
	push(MoveDelta{MovingPlane: BlackPawn, From: 52, To: 36}, post)
	for context.Depth() > 2 {
		if err := context.Pop(); err != nil {
			t.Fatal(err)
		}
		positions = positions[:len(positions)-1]
		accumulators = accumulators[:len(accumulators)-1]
	}

	// A different sibling reuses the same poisoned-by-history child frames.
	post = contextPost(branchRoot, []contextPiece{{WhiteKnight, 1}}, []contextPiece{{WhiteKnight, 16}})
	push(MoveDelta{MovingPlane: WhiteKnight, From: 1, To: 16}, post)
	post = contextPost(post, []contextPiece{{BlackKing, 60}}, []contextPiece{{BlackKing, 59}})
	push(MoveDelta{MovingPlane: BlackKing, From: 60, To: 59}, post)
}

func TestApplyUpdatesRefreshesOnlyCrossingKingPerspective(t *testing.T) {
	model := incrementalTestModel()
	post := contextPosition(Black,
		contextPiece{WhiteKing, 4}, contextPiece{BlackKing, 60},
		contextPiece{WhiteKnight, 18}, contextPiece{BlackBishop, 45})
	var destination accumulator
	for perspective := White; perspective <= Black; perspective++ {
		for hidden := 0; hidden < HiddenSize; hidden++ {
			destination[perspective][hidden] = int16(uint16(0x4000 + hidden*17 + int(perspective)))
		}
	}
	blackBefore := destination[Black]
	updates := [4]featureUpdate{
		{plane: WhiteKing, square: 4, add: true},
		{plane: WhiteKing, square: 3},
	}

	model.applyUpdates(&destination, post.Board, &updates, 2, White)
	wantRefresh := model.fullRefresh(post.Board)
	if destination[White] != wantRefresh[White] {
		t.Fatal("crossing White perspective was not fully refreshed")
	}
	blackKingSquare := 60
	toRow := &model.inputWeights[featureIndex(White, kingTypePlane, 4, blackKingSquare, Black)]
	fromRow := &model.inputWeights[featureIndex(White, kingTypePlane, 3, blackKingSquare, Black)]
	for hidden := 0; hidden < HiddenSize; hidden++ {
		want := blackBefore[hidden] + toRow[hidden] - fromRow[hidden]
		if destination[Black][hidden] != want {
			t.Fatalf("incremental Black perspective lane %d = %d, want sentinel delta %d", hidden, destination[Black][hidden], want)
		}
	}
	if destination[Black] == wantRefresh[Black] {
		t.Fatal("non-crossing Black perspective was unexpectedly refreshed")
	}
}

func TestSearchContextIncrementalArithmeticWrapsInt16(t *testing.T) {
	model := testModel()
	root := contextPosition(White,
		contextPiece{WhiteKing, 4}, contextPiece{BlackKing, 60}, contextPiece{WhiteKnight, 1})
	post := contextPosition(Black,
		contextPiece{WhiteKing, 4}, contextPiece{BlackKing, 60}, contextPiece{WhiteKnight, 18})
	for perspective := White; perspective <= Black; perspective++ {
		kingSquare := 4
		if perspective == Black {
			kingSquare = 60
		}
		from := featureIndex(White, 1, 1, kingSquare, perspective)
		to := featureIndex(White, 1, 18, kingSquare, perspective)
		model.inputBiases[0] = 32760
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
			t.Fatalf("wrapped perspective %d lane 0 = %d, want -32676", perspective, got)
		}
	}
}

func TestSearchContextRejectsTransitionsTransactionally(t *testing.T) {
	model := incrementalTestModel()
	root := contextPosition(White,
		contextPiece{WhiteKing, 4}, contextPiece{BlackKing, 60}, contextPiece{WhiteKnight, 1}, contextPiece{BlackPawn, 35})
	validDelta := MoveDelta{MovingPlane: WhiteKnight, From: 1, To: 18}
	validPost := contextPost(root, []contextPiece{{WhiteKnight, 1}}, []contextPiece{{WhiteKnight, 18}})
	tests := []struct {
		name  string
		delta MoveDelta
		post  Position
		want  string
	}{
		{name: "mover side", delta: MoveDelta{MovingPlane: BlackPawn, From: 35, To: 27}, post: Position{Board: root.Board, SideToMove: Black}, want: "mover color"},
		{name: "expected side", delta: validDelta, post: Position{Board: validPost.Board, SideToMove: White}, want: "differs from expected"},
		{name: "expected board", delta: validDelta, post: Position{Board: root.Board, SideToMove: Black}, want: "differs from expected"},
		{name: "capture fields", delta: MoveDelta{MovingPlane: WhiteKnight, From: 1, To: 18, CapturedPlane: BlackPawn}, post: validPost, want: "without capture"},
		{name: "own capture", delta: MoveDelta{MovingPlane: WhiteKnight, From: 1, To: 4, HasCapture: true, CapturedPlane: WhiteKing, CaptureSquare: 4}, post: validPost, want: "invalid captured"},
		{name: "king capture", delta: MoveDelta{MovingPlane: WhiteKnight, From: 1, To: 60, HasCapture: true, CapturedPlane: BlackKing, CaptureSquare: 60}, post: validPost, want: "invalid captured"},
		{name: "bad promotion", delta: MoveDelta{MovingPlane: WhiteKnight, From: 1, To: 18, HasPromotion: true, PromotionPlane: WhiteQueen}, post: validPost, want: "invalid promotion"},
		{name: "bad castle", delta: MoveDelta{MovingPlane: WhiteKnight, From: 1, To: 18, HasCastleRook: true, CastleRookFrom: 7, CastleRookTo: 5}, post: validPost, want: "invalid castling"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			context, err := model.NewSearchContext(root)
			if err != nil {
				t.Fatal(err)
			}
			beforeAccumulator := context.accumulators[0]
			beforePosition := context.positions[0]
			beforeCapacity := context.FrameCapacity()
			if err := context.PushMove(test.delta, test.post); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("PushMove error = %v, want substring %q", err, test.want)
			}
			if context.Depth() != 0 || context.FrameCapacity() != beforeCapacity ||
				context.accumulators[0] != beforeAccumulator || context.positions[0] != beforePosition {
				t.Fatal("rejected transition mutated context or capacity")
			}
		})
	}
}

func TestSearchContextNullPopResetAndCapacityGrowth(t *testing.T) {
	model := incrementalTestModel()
	root := contextPosition(White,
		contextPiece{WhiteKing, 4}, contextPiece{BlackKing, 60}, contextPiece{WhiteKnight, 1}, contextPiece{BlackKnight, 57})
	context, err := model.NewSearchContext(root)
	if err != nil {
		t.Fatal(err)
	}
	rootAccumulator := context.accumulators[0]
	if err := context.PushNull(); err != nil {
		t.Fatal(err)
	}
	if context.Position().Board != root.Board || context.Position().SideToMove != Black {
		t.Fatalf("null position = %+v, want unchanged board and Black STM", context.Position())
	}
	assertAccumulatorEqual(t, context.accumulators[1], rootAccumulator)
	if err := context.Pop(); err != nil {
		t.Fatal(err)
	}
	if context.Position() != root || context.accumulators[0] != rootAccumulator {
		t.Fatal("pop after null did not restore root")
	}
	if err := context.Pop(); err == nil || !strings.Contains(err.Error(), "cannot pop root") {
		t.Fatalf("root Pop error = %v", err)
	}

	for context.Depth() < initialSearchFrameCount-1 {
		if err := context.PushNull(); err != nil {
			t.Fatal(err)
		}
	}
	if context.FrameCapacity() != initialSearchFrameCount || context.Position().SideToMove != Black {
		t.Fatalf("boundary state depth/capacity/STM = %d/%d/%d", context.Depth(), context.FrameCapacity(), context.Position().SideToMove)
	}
	boundaryAccumulator := context.accumulators[context.depth]
	boundaryPosition := context.positions[context.depth]
	basePointer := &context.accumulators[0]
	badDelta := MoveDelta{MovingPlane: WhiteKnight, From: 1, To: 18}
	badPost := contextPost(boundaryPosition, []contextPiece{{WhiteKnight, 1}}, []contextPiece{{WhiteKnight, 18}})
	if err := context.PushMove(badDelta, badPost); err == nil || !strings.Contains(err.Error(), "mover color") {
		t.Fatalf("boundary rejected PushMove error = %v", err)
	}
	if context.Depth() != initialSearchFrameCount-1 || context.FrameCapacity() != initialSearchFrameCount ||
		&context.accumulators[0] != basePointer || context.accumulators[context.depth] != boundaryAccumulator ||
		context.positions[context.depth] != boundaryPosition {
		t.Fatal("rejected boundary push mutated frames or grew capacity")
	}

	validDelta := MoveDelta{MovingPlane: BlackKnight, From: 57, To: 42}
	validPost := contextPost(boundaryPosition, []contextPiece{{BlackKnight, 57}}, []contextPiece{{BlackKnight, 42}})
	if err := context.PushMove(validDelta, validPost); err != nil {
		t.Fatal(err)
	}
	if context.Depth() != initialSearchFrameCount || context.FrameCapacity() != initialSearchFrameCount*2 {
		t.Fatalf("successful growth depth/capacity = %d/%d, want %d/%d", context.Depth(), context.FrameCapacity(), initialSearchFrameCount, initialSearchFrameCount*2)
	}
	if context.accumulators[initialSearchFrameCount-1] != boundaryAccumulator ||
		context.positions[initialSearchFrameCount-1] != boundaryPosition {
		t.Fatal("growth mutated parent frame")
	}
	assertAccumulatorEqual(t, context.accumulators[context.depth], model.fullRefresh(validPost.Board))

	beforeDepth := context.Depth()
	beforeCapacity := context.FrameCapacity()
	beforePosition := context.Position()
	beforeAccumulator := context.accumulators[context.depth]
	invalidRoot := root
	invalidRoot.SideToMove = Color(9)
	if err := context.Reset(invalidRoot); err == nil {
		t.Fatal("Reset accepted invalid root")
	}
	if context.Depth() != beforeDepth || context.FrameCapacity() != beforeCapacity ||
		context.Position() != beforePosition || context.accumulators[context.depth] != beforeAccumulator {
		t.Fatal("rejected Reset mutated context")
	}

	newRoot := contextPosition(Black,
		contextPiece{WhiteKing, 3}, contextPiece{BlackKing, 59}, contextPiece{WhiteRook, 0}, contextPiece{BlackRook, 63})
	if err := context.Reset(newRoot); err != nil {
		t.Fatal(err)
	}
	if context.Depth() != 0 || context.FrameCapacity() != beforeCapacity || context.Position() != newRoot {
		t.Fatalf("Reset state depth/capacity/position = %d/%d/%+v", context.Depth(), context.FrameCapacity(), context.Position())
	}
	assertAccumulatorEqual(t, context.accumulators[0], model.fullRefresh(newRoot.Board))
}

func TestSearchContextsOwnIndependentFrames(t *testing.T) {
	model := incrementalTestModel()
	root := contextPosition(White,
		contextPiece{WhiteKing, 4}, contextPiece{BlackKing, 60}, contextPiece{WhiteKnight, 1})
	left, err := model.NewSearchContext(root)
	if err != nil {
		t.Fatal(err)
	}
	right, err := model.NewSearchContext(root)
	if err != nil {
		t.Fatal(err)
	}
	if &left.accumulators[0] == &right.accumulators[0] || &left.positions[0] == &right.positions[0] {
		t.Fatal("contexts share frame backing storage")
	}
	rightAccumulator := right.accumulators[0]
	rightPosition := right.positions[0]
	rightCapacity := right.FrameCapacity()
	post := contextPost(root, []contextPiece{{WhiteKnight, 1}}, []contextPiece{{WhiteKnight, 18}})
	if err := left.PushMove(MoveDelta{MovingPlane: WhiteKnight, From: 1, To: 18}, post); err != nil {
		t.Fatal(err)
	}
	if right.Depth() != 0 || right.FrameCapacity() != rightCapacity ||
		right.positions[0] != rightPosition || right.accumulators[0] != rightAccumulator {
		t.Fatal("pushing one worker context mutated another")
	}
}

func TestSearchContextRejectsUnvalidatedModelAndPosition(t *testing.T) {
	var zero Model
	if context, err := zero.NewSearchContext(minimalPosition(White)); err == nil || context != nil {
		t.Fatalf("zero model context = %v, %v", context, err)
	}
	model := testModel()
	invalid := minimalPosition(White)
	invalid.Board[WhiteQueen] = invalid.Board[WhiteKing]
	if context, err := model.NewSearchContext(invalid); err == nil || context != nil {
		t.Fatalf("invalid root context = %v, %v", context, err)
	}
}
