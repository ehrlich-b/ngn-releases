package nnue_test

import (
	"fmt"
	"testing"

	"github.com/ehrlich-b/ngn/engine"
	"github.com/ehrlich-b/ngn/nnue"
)

type engineUndo struct {
	move      engine.Move
	tag       engine.PositionTag
	enPassant engine.Square
	halfClock uint8
}

func fromEnginePosition(position *engine.Position) nnue.Position {
	all := position.Board.AllPieces()
	pieces := make([]nnue.PieceOnSquare, 0, len(all))
	for square := engine.Square(0); square < 64; square++ {
		enginePiece, ok := all[square]
		if !ok {
			continue
		}
		color := nnue.Black
		if enginePiece.Color() == engine.White {
			color = nnue.White
		}
		pieces = append(pieces, nnue.PieceOnSquare{
			Piece:  nnue.PieceType(enginePiece.Type() - engine.Pawn),
			Color:  color,
			Square: nnue.Square(square),
		})
	}
	side := nnue.Black
	if position.Turn() == engine.White {
		side = nnue.White
	}
	return nnue.Position{SideToMove: side, Pieces: pieces}
}

func pieceMap(position nnue.Position) ([64]nnue.PieceOnSquare, [64]bool) {
	var pieces [64]nnue.PieceOnSquare
	var present [64]bool
	for _, piece := range position.Pieces {
		pieces[piece.Square] = piece
		present[piece.Square] = true
	}
	return pieces, present
}

func sameEngineFixturePiece(a, b nnue.PieceOnSquare) bool {
	return a.Piece == b.Piece && a.Color == b.Color
}

func moveFirst(pieces []nnue.PieceOnSquare, square nnue.Square) {
	for i := range pieces {
		if pieces[i].Square == square {
			pieces[0], pieces[i] = pieces[i], pieces[0]
			return
		}
	}
}

func deltaFromEngineMove(t *testing.T, move engine.Move, before, after nnue.Position) nnue.Delta {
	t.Helper()
	beforeBoard, beforePresent := pieceMap(before)
	afterBoard, afterPresent := pieceMap(after)
	removed := make([]nnue.PieceOnSquare, 0, 2)
	added := make([]nnue.PieceOnSquare, 0, 2)
	for square := nnue.Square(0); square < 64; square++ {
		if beforePresent[square] && (!afterPresent[square] || !sameEngineFixturePiece(beforeBoard[square], afterBoard[square])) {
			removed = append(removed, beforeBoard[square])
		}
		if afterPresent[square] && (!beforePresent[square] || !sameEngineFixturePiece(beforeBoard[square], afterBoard[square])) {
			added = append(added, afterBoard[square])
		}
	}
	if len(removed) == 0 || len(removed) > nnue.MaxDeltaPieces || len(added) == 0 || len(added) > nnue.MaxDeltaPieces {
		t.Fatalf("unexpected board diff for %s: removed=%v added=%v", move.ToString(), removed, added)
	}
	moveFirst(removed, nnue.Square(move.Source()))
	moveFirst(added, nnue.Square(move.Destination()))

	kind := nnue.MoveNormal
	switch {
	case move.PromoType() != engine.NoType && move.IsCapture():
		kind = nnue.MovePromotionCapture
	case move.PromoType() != engine.NoType:
		kind = nnue.MovePromotion
	case move.IsCastle():
		kind = nnue.MoveCastle
	case move.IsEnPassant():
		kind = nnue.MoveEnPassant
	case move.IsCapture():
		kind = nnue.MoveCapture
	}
	return transition(t, kind, before, after, removed, added)
}

func findLegalMove(t *testing.T, position *engine.Position, notation string) engine.Move {
	t.Helper()
	for _, move := range engine.GenerateLegalMoves(position) {
		if move.ToString() == notation {
			return move
		}
	}
	t.Fatalf("legal move %s not found", notation)
	return engine.EmptyMove
}

func makeAndCheckEngineMove(t *testing.T, model *nnue.Model, context *nnue.Context, position *engine.Position, move engine.Move) engineUndo {
	t.Helper()
	before := fromEnginePosition(position)
	ep, tag, halfClock, legal := position.MakeMove(move)
	if !legal {
		t.Fatalf("MakeMove rejected generated move %s", move.ToString())
	}
	after := fromEnginePosition(position)
	delta := deltaFromEngineMove(t, move, before, after)
	if err := context.Push(delta, after); err != nil {
		t.Fatalf("Push %s: %v", move.ToString(), err)
	}
	requireContextMatchesReference(t, model, context, after)
	return engineUndo{move: move, tag: tag, enPassant: ep, halfClock: halfClock}
}

func TestContextEngineLegalSpecialMoves(t *testing.T) {
	cases := []struct {
		name, fen, move string
		kind            nnue.MoveKind
	}{
		{"white normal", "4k3/8/8/8/8/8/4P3/4K3 w - - 0 1", "e2e4", nnue.MoveNormal},
		{"black normal", "4k3/4p3/8/8/8/8/8/4K3 b - - 0 1", "e7e5", nnue.MoveNormal},
		{"white king normal", "4k3/8/8/8/8/8/8/4K3 w - - 0 1", "e1d1", nnue.MoveNormal},
		{"black king normal", "4k3/8/8/8/8/8/8/4K3 b - - 0 1", "e8d8", nnue.MoveNormal},
		{"white capture", "4k3/8/8/3p4/4P3/8/8/4K3 w - - 0 1", "e4d5", nnue.MoveCapture},
		{"black capture", "4k3/8/8/3p4/4P3/8/8/4K3 b - - 0 1", "d5e4", nnue.MoveCapture},
		{"white en passant", "4k3/8/8/3pP3/8/8/8/4K3 w - d6 0 1", "e5d6", nnue.MoveEnPassant},
		{"black en passant", "4k3/8/8/8/3pP3/8/8/4K3 b - e3 0 1", "d4e3", nnue.MoveEnPassant},
		{"white castle", "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1", "e1g1", nnue.MoveCastle},
		{"black castle", "r3k2r/8/8/8/8/8/8/R3K2R b KQkq - 0 1", "e8c8", nnue.MoveCastle},
		{"white promotion queen", "4k3/P7/8/8/8/8/8/4K3 w - - 0 1", "a7a8q", nnue.MovePromotion},
		{"white promotion rook", "4k3/P7/8/8/8/8/8/4K3 w - - 0 1", "a7a8r", nnue.MovePromotion},
		{"white promotion bishop", "4k3/P7/8/8/8/8/8/4K3 w - - 0 1", "a7a8b", nnue.MovePromotion},
		{"white promotion knight", "4k3/P7/8/8/8/8/8/4K3 w - - 0 1", "a7a8n", nnue.MovePromotion},
		{"black promotion queen", "4k3/8/8/8/8/8/p7/4K3 b - - 0 1", "a2a1q", nnue.MovePromotion},
		{"black promotion rook", "4k3/8/8/8/8/8/p7/4K3 b - - 0 1", "a2a1r", nnue.MovePromotion},
		{"black promotion bishop", "4k3/8/8/8/8/8/p7/4K3 b - - 0 1", "a2a1b", nnue.MovePromotion},
		{"black promotion knight", "4k3/8/8/8/8/8/p7/4K3 b - - 0 1", "a2a1n", nnue.MovePromotion},
		{"white promotion capture", "1r2k3/P7/8/8/8/8/8/4K3 w - - 0 1", "a7b8q", nnue.MovePromotionCapture},
		{"black promotion capture", "4k3/8/8/8/8/8/p7/1R2K3 b - - 0 1", "a2b1q", nnue.MovePromotionCapture},
	}
	model := contextModel(t)
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			position, err := engine.ParseFEN(test.fen)
			if err != nil {
				t.Fatalf("ParseFEN: %v", err)
			}
			root := fromEnginePosition(position)
			context, err := nnue.NewContext(model)
			if err != nil {
				t.Fatal(err)
			}
			if err := context.Reset(root); err != nil {
				t.Fatal(err)
			}
			move := findLegalMove(t, position, test.move)
			before := fromEnginePosition(position)
			ep, tag, halfClock, legal := position.MakeMove(move)
			if !legal {
				t.Fatal("legal move rejected")
			}
			after := fromEnginePosition(position)
			delta := deltaFromEngineMove(t, move, before, after)
			if delta.Kind != test.kind {
				t.Fatalf("derived kind = %d, want %d", delta.Kind, test.kind)
			}
			if err := context.Push(delta, after); err != nil {
				t.Fatalf("Push: %v", err)
			}
			requireContextMatchesReference(t, model, context, after)
			position.UnMakeMove(move, tag, ep, halfClock)
			if err := context.Pop(); err != nil {
				t.Fatal(err)
			}
			unmade := fromEnginePosition(position)
			requireContextMatchesReference(t, model, context, unmade)
			if fmt.Sprint(unmade) != fmt.Sprint(root) {
				t.Fatalf("engine undo did not restore converted root: got %v want %v", unmade, root)
			}
		})
	}
}

func TestContextEngineHistoriesMakeAndUnmake(t *testing.T) {
	roots := []string{
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		"r3k2r/p1ppqpb1/bn2pnp1/2pP4/1p2P3/2N2N2/PPQBBPPP/R3K2R w KQkq - 0 1",
		"4k3/ppp2ppp/8/3pP3/8/8/PPP2PPP/4K3 w - d6 0 1",
	}
	model := contextModel(t)
	for rootIndex, fen := range roots {
		t.Run(fmt.Sprintf("history-%d", rootIndex), func(t *testing.T) {
			position, err := engine.ParseFEN(fen)
			if err != nil {
				t.Fatal(err)
			}
			root := fromEnginePosition(position)
			context, _ := nnue.NewContext(model)
			if err := context.Reset(root); err != nil {
				t.Fatal(err)
			}
			requireContextMatchesReference(t, model, context, root)
			undos := make([]engineUndo, 0, 40)
			for ply := 0; ply < 40; ply++ {
				moves := engine.GenerateLegalMoves(position)
				if len(moves) == 0 {
					break
				}
				index := (rootIndex*23 + ply*17 + 5) % len(moves)
				undos = append(undos, makeAndCheckEngineMove(t, model, context, position, moves[index]))
			}
			if len(undos) < 10 {
				t.Fatalf("history too short: %d plies", len(undos))
			}
			for i := len(undos) - 1; i >= 0; i-- {
				undo := undos[i]
				position.UnMakeMove(undo.move, undo.tag, undo.enPassant, undo.halfClock)
				if err := context.Pop(); err != nil {
					t.Fatalf("Pop ply %d: %v", i, err)
				}
				requireContextMatchesReference(t, model, context, fromEnginePosition(position))
			}
			if context.Depth() != 0 {
				t.Fatalf("final depth = %d", context.Depth())
			}
		})
	}
}

func TestContextEngineRepetitionCycle(t *testing.T) {
	position, err := engine.ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	model := contextModel(t)
	context, _ := nnue.NewContext(model)
	root := fromEnginePosition(position)
	if err := context.Reset(root); err != nil {
		t.Fatal(err)
	}
	rootScore := requireContextMatchesReference(t, model, context, root)
	var undos []engineUndo
	for _, notation := range []string{"g1f3", "g8f6", "f3g1", "f6g8"} {
		move := findLegalMove(t, position, notation)
		undos = append(undos, makeAndCheckEngineMove(t, model, context, position, move))
	}
	if got := requireContextMatchesReference(t, model, context, fromEnginePosition(position)); got != rootScore {
		t.Fatalf("repeated board score = %d, root score = %d", got, rootScore)
	}
	for i := len(undos) - 1; i >= 0; i-- {
		undo := undos[i]
		position.UnMakeMove(undo.move, undo.tag, undo.enPassant, undo.halfClock)
		if err := context.Pop(); err != nil {
			t.Fatal(err)
		}
		requireContextMatchesReference(t, model, context, fromEnginePosition(position))
	}
}

func TestContextEngineNullMove(t *testing.T) {
	position, err := engine.ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	model := contextModel(t)
	context, _ := nnue.NewContext(model)
	before := fromEnginePosition(position)
	if err := context.Reset(before); err != nil {
		t.Fatal(err)
	}
	ep := position.MakeNullMove()
	after := fromEnginePosition(position)
	if err := context.PushNull(after); err != nil {
		t.Fatalf("PushNull: %v", err)
	}
	requireContextMatchesReference(t, model, context, after)
	position.UnMakeNullMove(ep)
	if err := context.Pop(); err != nil {
		t.Fatal(err)
	}
	requireContextMatchesReference(t, model, context, fromEnginePosition(position))
}
