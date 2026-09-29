package ngnk4

import (
	"bytes"
	"testing"
)

func transitionTestModel(t *testing.T) *Model {
	t.Helper()
	data := makeTestFile(t, func(payload []byte) {
		for feature := 0; feature < TotalInputFeatures; feature++ {
			for hidden := 0; hidden < 8; hidden++ {
				value := int16((feature*17+hidden*29)%101 - 50)
				putI16(payload, inputWeightOffset(feature, hidden), value)
			}
		}
		for bucket := 0; bucket < OutputBuckets; bucket++ {
			for perspective := 0; perspective < 2; perspective++ {
				for hidden := 0; hidden < 8; hidden++ {
					putI16(payload, outputWeightOffset(bucket, perspective, hidden), int16(7+bucket*3+perspective*5+hidden))
				}
			}
		}
	})
	model, err := Load(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return model
}

func assertContextMatchesRefresh(t *testing.T, context *SearchContext) {
	t.Helper()
	position := context.Position()
	wantAccumulator := context.model.fullRefresh(position.Board)
	if got := context.accumulators[context.depth]; got != wantAccumulator {
		t.Fatal("incremental accumulator differs from full refresh")
	}
	wantScore, err := context.model.EvaluateRaw(position)
	if err != nil {
		t.Fatal(err)
	}
	gotScore, err := context.EvaluateRaw()
	if err != nil || gotScore != wantScore {
		t.Fatalf("incremental score = %d, %v; full refresh = %d", gotScore, err, wantScore)
	}
}

func pushFixture(t *testing.T, model *Model, root Position, delta MoveDelta, post Position) {
	t.Helper()
	context, err := model.NewSearchContext(root)
	if err != nil {
		t.Fatal(err)
	}
	rootAccumulator := context.accumulators[0]
	if err := context.PushMove(delta, post); err != nil {
		t.Fatal(err)
	}
	assertContextMatchesRefresh(t, context)
	if err := context.PushNull(); err != nil {
		t.Fatal(err)
	}
	assertContextMatchesRefresh(t, context)
	if err := context.Pop(); err != nil {
		t.Fatal(err)
	}
	if err := context.Pop(); err != nil {
		t.Fatal(err)
	}
	if context.Depth() != 0 || context.Position() != root || context.accumulators[0] != rootAccumulator {
		t.Fatal("full unwind did not restore root frame")
	}
}

func TestIncrementalSpecialMovesAndKingRefresh(t *testing.T) {
	model := transitionTestModel(t)
	t.Run("capture", func(t *testing.T) {
		root := kingsOnly(White, 4, 60)
		root.Board[WhiteKnight] = 1 << 1
		root.Board[BlackBishop] = 1 << 18
		post := kingsOnly(Black, 4, 60)
		post.Board[WhiteKnight] = 1 << 18
		pushFixture(t, model, root, MoveDelta{MovingPlane: WhiteKnight, From: 1, To: 18, HasCapture: true, CapturedPlane: BlackBishop, CaptureSquare: 18}, post)
	})
	t.Run("en passant", func(t *testing.T) {
		root := kingsOnly(White, 4, 60)
		root.Board[WhitePawn] = 1 << 36
		root.Board[BlackPawn] = 1 << 35
		post := kingsOnly(Black, 4, 60)
		post.Board[WhitePawn] = 1 << 43
		pushFixture(t, model, root, MoveDelta{MovingPlane: WhitePawn, From: 36, To: 43, HasCapture: true, CapturedPlane: BlackPawn, CaptureSquare: 35}, post)
	})
	t.Run("promotion capture", func(t *testing.T) {
		root := kingsOnly(White, 4, 60)
		root.Board[WhitePawn] = 1 << 48
		root.Board[BlackRook] = 1 << 57
		post := kingsOnly(Black, 4, 60)
		post.Board[WhiteQueen] = 1 << 57
		pushFixture(t, model, root, MoveDelta{MovingPlane: WhitePawn, From: 48, To: 57, HasCapture: true, CapturedPlane: BlackRook, CaptureSquare: 57, HasPromotion: true, PromotionPlane: WhiteQueen}, post)
	})
	t.Run("castling bucket crossing", func(t *testing.T) {
		root := kingsOnly(White, 4, 60)
		root.Board[WhiteRook] = 1 << 7
		post := kingsOnly(Black, 6, 60)
		post.Board[WhiteRook] = 1 << 5
		pushFixture(t, model, root, MoveDelta{MovingPlane: WhiteKing, From: 4, To: 6, HasCastleRook: true, CastleRookFrom: 7, CastleRookTo: 5}, post)
	})
	t.Run("black mirror crossing", func(t *testing.T) {
		root := kingsOnly(Black, 4, 60)
		post := kingsOnly(White, 4, 59)
		pushFixture(t, model, root, MoveDelta{MovingPlane: BlackKing, From: 60, To: 59}, post)
	})
}

func kingsOnly(side Color, whiteKing, blackKing int) Position {
	var position Position
	position.SideToMove = side
	position.Board[WhiteKing] = uint64(1) << whiteKing
	position.Board[BlackKing] = uint64(1) << blackKing
	return position
}

func TestRejectedPushIsTransactionalAndStackGrows(t *testing.T) {
	model := transitionTestModel(t)
	root := kingsOnly(White, 4, 60)
	root.Board[WhiteKnight] = 1 << 1
	context, err := model.NewSearchContext(root)
	if err != nil {
		t.Fatal(err)
	}
	depth, capacity, accumulator := context.Depth(), context.FrameCapacity(), context.accumulators[0]
	wrongPost := root
	wrongPost.SideToMove = Black
	if err := context.PushMove(MoveDelta{MovingPlane: WhiteKnight, From: 1, To: 18}, wrongPost); err == nil {
		t.Fatal("mismatched post position accepted")
	}
	if context.Depth() != depth || context.FrameCapacity() != capacity || context.Position() != root || context.accumulators[0] != accumulator {
		t.Fatal("rejected push changed context")
	}
	for i := 0; i < initialSearchFrameCount; i++ {
		if err := context.PushNull(); err != nil {
			t.Fatal(err)
		}
	}
	if context.FrameCapacity() != 2*initialSearchFrameCount || context.Depth() != initialSearchFrameCount {
		t.Fatalf("growth = capacity %d depth %d", context.FrameCapacity(), context.Depth())
	}
	for context.Depth() > 0 {
		if err := context.Pop(); err != nil {
			t.Fatal(err)
		}
	}
	if context.Position() != root || context.accumulators[0] != accumulator {
		t.Fatal("growth/unwind changed root")
	}
}
