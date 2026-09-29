//go:build rodentv12contextoracle

package rodentv12eval

import (
	"os"
	"testing"
)

func TestV12DefaultExactModelIncrementalPaths(t *testing.T) {
	modelPath := os.Getenv("RODENT_V12_DEFAULT_MODEL")
	if modelPath == "" {
		t.Fatal("RODENT_V12_DEFAULT_MODEL is required")
	}
	model, err := LoadV12Default(modelPath)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("bucket_mirror_en_passant_promotion_and_sibling", func(t *testing.T) {
		root := contextPosition(White,
			contextPiece{WhiteKing, 2}, contextPiece{BlackKing, 60},
			contextPiece{WhiteRook, 7}, contextPiece{WhitePawn, 36}, contextPiece{WhitePawn, 54},
			contextPiece{BlackRook, 63}, contextPiece{BlackPawn, 35}, contextPiece{BlackKnight, 57})
		context, err := model.NewSearchContext(root)
		if err != nil {
			t.Fatal(err)
		}
		assertContextMatchesRefresh(t, context)
		push := func(delta MoveDelta, post Position) {
			t.Helper()
			if err := context.PushMove(delta, post); err != nil {
				t.Fatal(err)
			}
			if context.Position() != post {
				t.Fatal("context position differs from expected checkpoint")
			}
			assertContextMatchesRefresh(t, context)
		}

		post := contextPost(root, []contextPiece{{WhiteKing, 2}}, []contextPiece{{WhiteKing, 1}})
		push(MoveDelta{MovingPlane: WhiteKing, From: 2, To: 1}, post)
		post = contextPost(post, []contextPiece{{BlackKnight, 57}}, []contextPiece{{BlackKnight, 42}})
		push(MoveDelta{MovingPlane: BlackKnight, From: 57, To: 42}, post)
		post = contextPost(post,
			[]contextPiece{{WhitePawn, 36}, {BlackPawn, 35}},
			[]contextPiece{{WhitePawn, 43}})
		push(MoveDelta{MovingPlane: WhitePawn, From: 36, To: 43, HasCapture: true, CapturedPlane: BlackPawn, CaptureSquare: 35}, post)
		post = contextPost(post, []contextPiece{{BlackKing, 60}}, []contextPiece{{BlackKing, 59}})
		push(MoveDelta{MovingPlane: BlackKing, From: 60, To: 59}, post)
		post = contextPost(post,
			[]contextPiece{{WhitePawn, 54}, {BlackRook, 63}},
			[]contextPiece{{WhiteQueen, 63}})
		push(MoveDelta{MovingPlane: WhitePawn, From: 54, To: 63, HasCapture: true, CapturedPlane: BlackRook, CaptureSquare: 63, HasPromotion: true, PromotionPlane: WhiteQueen}, post)
		post = contextPost(post, []contextPiece{{BlackKing, 59}}, []contextPiece{{BlackKing, 50}})
		push(MoveDelta{MovingPlane: BlackKing, From: 59, To: 50}, post)

		beforeNull := context.Position()
		beforeNullAccumulator := context.accumulators[context.depth]
		if err := context.PushNull(); err != nil {
			t.Fatal(err)
		}
		if context.Position().Board != beforeNull.Board || context.Position().SideToMove == beforeNull.SideToMove ||
			context.accumulators[context.depth] != beforeNullAccumulator {
			t.Fatal("null checkpoint changed board/accumulator or failed to flip side")
		}
		assertContextMatchesRefresh(t, context)
		if err := context.Pop(); err != nil {
			t.Fatal(err)
		}
		if context.Position() != beforeNull || context.accumulators[context.depth] != beforeNullAccumulator {
			t.Fatal("pop after null failed to restore exact parent")
		}
		post = contextPost(beforeNull, []contextPiece{{WhiteRook, 7}}, []contextPiece{{WhiteRook, 15}})
		push(MoveDelta{MovingPlane: WhiteRook, From: 7, To: 15}, post)
	})

	t.Run("both_castles_and_ordinary_updates", func(t *testing.T) {
		root := contextPosition(White,
			contextPiece{WhiteKing, 4}, contextPiece{WhiteRook, 7},
			contextPiece{BlackKing, 60}, contextPiece{BlackRook, 56},
			contextPiece{WhiteKnight, 18}, contextPiece{BlackBishop, 45})
		context, err := model.NewSearchContext(root)
		if err != nil {
			t.Fatal(err)
		}
		assertContextMatchesRefresh(t, context)
		push := func(delta MoveDelta, post Position) {
			t.Helper()
			if err := context.PushMove(delta, post); err != nil {
				t.Fatal(err)
			}
			assertContextMatchesRefresh(t, context)
		}

		post := contextPost(root,
			[]contextPiece{{WhiteKing, 4}, {WhiteRook, 7}},
			[]contextPiece{{WhiteKing, 6}, {WhiteRook, 5}})
		push(MoveDelta{MovingPlane: WhiteKing, From: 4, To: 6, HasCastleRook: true, CastleRookFrom: 7, CastleRookTo: 5}, post)
		post = contextPost(post,
			[]contextPiece{{BlackKing, 60}, {BlackRook, 56}},
			[]contextPiece{{BlackKing, 58}, {BlackRook, 59}})
		push(MoveDelta{MovingPlane: BlackKing, From: 60, To: 58, HasCastleRook: true, CastleRookFrom: 56, CastleRookTo: 59}, post)
		post = contextPost(post, []contextPiece{{WhiteKnight, 18}}, []contextPiece{{WhiteKnight, 35}})
		push(MoveDelta{MovingPlane: WhiteKnight, From: 18, To: 35}, post)
		post = contextPost(post, []contextPiece{{BlackBishop, 45}}, []contextPiece{{BlackBishop, 28}})
		push(MoveDelta{MovingPlane: BlackBishop, From: 45, To: 28}, post)
	})
}
