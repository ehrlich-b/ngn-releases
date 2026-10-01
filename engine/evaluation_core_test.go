package engine

import "testing"

type testPiecePlacement struct {
	square Square
	piece  Piece
}

func boardWithPieces(placements ...testPiecePlacement) Bitboard {
	var board Bitboard
	for _, placement := range placements {
		board.UpdateSquare(placement.square, placement.piece, NoPiece)
	}
	return board
}

func reflectedColourBoard(source *Bitboard) Bitboard {
	var reflected Bitboard
	for square, piece := range source.mailbox {
		if piece == NoPiece {
			continue
		}
		file, rank := square&7, square>>3
		reflectedSquare := Square((7-rank)*8 + file)
		reflectedPiece := GetPiece(piece.Type(), piece.Color().Other())
		reflected.UpdateSquare(reflectedSquare, reflectedPiece, NoPiece)
	}
	return reflected
}

func rebuildFromMailbox(source *Bitboard) Bitboard {
	var rebuilt Bitboard
	for square, piece := range source.mailbox {
		if validPiece(piece) {
			rebuilt.UpdateSquare(Square(square), piece, NoPiece)
		}
	}
	return rebuilt
}

func TestEvaluationStartAndReflectionSymmetry(t *testing.T) {
	start := StartingBoard()
	if score := evaluateRaw(&start); score != 0 {
		t.Fatalf("starting position score = %d, want zero", score)
	}

	board := boardWithPieces(
		testPiecePlacement{A1, WhiteKing},
		testPiecePlacement{G8, BlackKing},
		testPiecePlacement{D4, WhiteQueen},
		testPiecePlacement{B6, BlackRook},
		testPiecePlacement{C3, WhitePawn},
		testPiecePlacement{F6, BlackPawn},
		testPiecePlacement{G2, WhiteKnight},
		testPiecePlacement{C7, BlackBishop},
	)
	reflected := reflectedColourBoard(&board)
	want := -evaluateRaw(&board)
	if got := evaluateRaw(&reflected); got != want {
		t.Fatalf("colour/reflection score = %d, want %d", got, want)
	}
	before := board
	_ = evaluateRaw(&board)
	if board != before {
		t.Fatal("evaluation mutated its board")
	}
}

func TestEvaluationDeadMaterialControls(t *testing.T) {
	cases := []Bitboard{
		boardWithPieces(testPiecePlacement{A1, WhiteKing}, testPiecePlacement{H8, BlackKing}),
		boardWithPieces(testPiecePlacement{C1, WhiteKing}, testPiecePlacement{H8, BlackKing}, testPiecePlacement{B2, WhiteBishop}),
		boardWithPieces(testPiecePlacement{A1, WhiteKing}, testPiecePlacement{G8, BlackKing}, testPiecePlacement{F3, WhiteKnight}),
		boardWithPieces(testPiecePlacement{A1, WhiteKing}, testPiecePlacement{H8, BlackKing}, testPiecePlacement{G7, BlackBishop}),
		boardWithPieces(testPiecePlacement{A1, WhiteKing}, testPiecePlacement{H8, BlackKing}, testPiecePlacement{B7, BlackKnight}),
	}
	for index := range cases {
		if score := evaluateRaw(&cases[index]); score != 0 {
			t.Fatalf("dead-material case %d score = %d, want zero", index, score)
		}
	}
}

func TestEvaluationMaterialPhaseAndEndgameRules(t *testing.T) {
	whiteQueen := boardWithPieces(
		testPiecePlacement{E1, WhiteKing},
		testPiecePlacement{E8, BlackKing},
		testPiecePlacement{D4, WhiteQueen},
	)
	if score := evaluateRaw(&whiteQueen); score <= 0 {
		t.Fatalf("white queen material score = %d, want positive", score)
	}
	if !hasNonPawnMaterial(&whiteQueen, White) || hasNonPawnMaterial(&whiteQueen, Black) {
		t.Fatal("non-pawn material ownership was misclassified")
	}
	if got := gamePhase(&whiteQueen); got != piecePhaseInc[WhiteQueen] {
		t.Fatalf("queen phase = %d, want %d", got, piecePhaseInc[WhiteQueen])
	}

	blackQueen := boardWithPieces(
		testPiecePlacement{E1, WhiteKing},
		testPiecePlacement{E8, BlackKing},
		testPiecePlacement{D5, BlackQueen},
	)
	if score := evaluateRaw(&blackQueen); score >= 0 {
		t.Fatalf("black queen material score = %d, want negative", score)
	}

	start := StartingBoard()
	if got := gamePhase(&start); got != totalPhase {
		t.Fatalf("starting phase = %d, want %d", got, totalPhase)
	}
	bare := boardWithPieces(testPiecePlacement{A1, WhiteKing}, testPiecePlacement{H8, BlackKing})
	if got := gamePhase(&bare); got != 0 {
		t.Fatalf("bare-king phase = %d, want zero", got)
	}
	if isEndgame(&start) || !isEndgame(&bare) || isEndgame(&whiteQueen) {
		t.Fatal("material endgame criterion disagrees with its documented shapes")
	}
}

func TestEvaluationBoundedAndIndependentOfAccumulator(t *testing.T) {
	var crowded Bitboard
	for square := 0; square < 64; square++ {
		piece := WhiteQueen
		if square&1 != 0 {
			piece = BlackQueen
		}
		crowded.UpdateSquare(Square(square), piece, NoPiece)
	}
	score := evaluateRaw(&crowded)
	if score <= -25000 || score >= 25000 {
		t.Fatalf("crowded score = %d, outside strict bound", score)
	}

	board := StartingBoard()
	want := evaluateRaw(&board)
	board.accMG = 123456
	board.accEG = -654321
	board.accPhase = 777
	if got := evaluateRaw(&board); got != want {
		t.Fatalf("stale accumulators changed score: got %d want %d", got, want)
	}
	if board.accMG != 123456 || board.accEG != -654321 || board.accPhase != 777 {
		t.Fatal("evaluation repaired or mutated cached accumulators")
	}
	if allocations := testing.AllocsPerRun(1000, func() {
		_ = evaluateRaw(&board)
	}); allocations != 0 {
		t.Fatalf("evaluation allocated %v times per run", allocations)
	}
}

func TestRebuildEquivalenceAndLegalMutationRestoration(t *testing.T) {
	board := StartingBoard()
	original := board
	beforeScore := evaluateRaw(&board)
	RebuildPST()
	if got := evaluateRaw(&board); got != beforeScore {
		t.Fatalf("score changed across deterministic rebuild: got %d want %d", got, beforeScore)
	}
	if board != original {
		t.Fatal("table rebuild changed an existing board")
	}
	rebuilt := rebuildFromMailbox(&board)
	if got := evaluateRaw(&rebuilt); got != evaluateRaw(&board) {
		t.Fatalf("mailbox rebuild changed score: got %d want %d", got, evaluateRaw(&board))
	}

	board.Move(E2, E4, WhitePawn, NoPiece)
	board.Move(E4, E2, WhitePawn, NoPiece)
	if board != original {
		t.Fatal("quiet legal-shape move did not restore the board")
	}

	capture := boardWithPieces(
		testPiecePlacement{E1, WhiteKing},
		testPiecePlacement{E8, BlackKing},
		testPiecePlacement{A1, WhiteRook},
		testPiecePlacement{A8, BlackKnight},
	)
	captureOriginal := capture
	capture.UpdateSquare(A1, NoPiece, WhiteRook)
	capture.UpdateSquare(A8, WhiteRook, BlackKnight)
	capture.UpdateSquare(A8, BlackKnight, WhiteRook)
	capture.UpdateSquare(A1, WhiteRook, NoPiece)
	if capture != captureOriginal {
		t.Fatal("capture-shaped updates did not restore accumulators and masks")
	}

	promotion := boardWithPieces(
		testPiecePlacement{E1, WhiteKing},
		testPiecePlacement{E8, BlackKing},
		testPiecePlacement{A7, WhitePawn},
	)
	promotionOriginal := promotion
	promotion.UpdateSquare(A7, WhiteQueen, WhitePawn)
	promotion.UpdateSquare(A7, WhitePawn, WhiteQueen)
	if promotion != promotionOriginal {
		t.Fatal("promotion-shaped updates did not restore accumulators")
	}

	castle := boardWithPieces(
		testPiecePlacement{E1, WhiteKing},
		testPiecePlacement{H1, WhiteRook},
		testPiecePlacement{E8, BlackKing},
	)
	castleOriginal := castle
	castle.Move(E1, G1, WhiteKing, NoPiece)
	castle.Move(G1, E1, WhiteKing, NoPiece)
	castle.Move(F1, H1, WhiteRook, NoPiece)
	if castle != castleOriginal {
		t.Fatal("castling-shaped updates did not restore accumulators")
	}
}

func TestRebuildIsStable(t *testing.T) {
	RebuildPST()
	mgBefore, egBefore, phaseBefore := mgPST, egPST, piecePhaseInc
	RebuildPST()
	if mgPST != mgBefore || egPST != egBefore || piecePhaseInc != phaseBefore {
		t.Fatal("rebuilding analytic tables was not deterministic")
	}
	for pieceType := Pawn; pieceType <= King; pieceType++ {
		white := GetPiece(pieceType, White)
		black := GetPiece(pieceType, Black)
		for square := 0; square < 64; square++ {
			reflectedSquare := (7-(square>>3))*8 + (square & 7)
			if mgPST[white][square] != -mgPST[black][reflectedSquare] || egPST[white][square] != -egPST[black][reflectedSquare] {
				t.Fatalf("table reflection mismatch type=%d square=%d", pieceType, square)
			}
		}
	}
}
