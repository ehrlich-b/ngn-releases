package engine

import (
	"fmt"
	"math/bits"
	"math/rand"
	"testing"
)

func TestMaterialEvaluation(t *testing.T) {
	// Test basic material counting
	board := Bitboard{}

	// Starting position should be roughly balanced
	board = StartingBoard()
	eval := Evaluate(&board)

	// Starting position should be close to 0 (may not be exactly 0 due to PST values)
	if eval > 100 || eval < -100 {
		t.Errorf("Starting position evaluation should be close to 0, got %d", eval)
	}

	// Test position with material advantage for white
	board = Bitboard{}
	board.UpdateSquare(E1, WhiteKing, NoPiece)
	board.UpdateSquare(E8, BlackKing, NoPiece)
	board.UpdateSquare(D1, WhiteQueen, NoPiece) // White has extra queen

	eval = Evaluate(&board)
	if eval <= 900 { // Should be strongly positive for white (queen = 1000 - some PST penalty)
		t.Errorf("White should have large material advantage, got %d", eval)
	}
}

func TestPieceSquareTables(t *testing.T) {
	// Test that central squares are valued higher for pieces
	board := Bitboard{}
	board.UpdateSquare(E1, WhiteKing, NoPiece)
	board.UpdateSquare(E8, BlackKing, NoPiece)

	// Knight on edge vs center
	board.UpdateSquare(A1, WhiteKnight, NoPiece)
	evalEdge := Evaluate(&board)

	board.UpdateSquare(A1, NoPiece, WhiteKnight)
	board.UpdateSquare(E4, WhiteKnight, NoPiece) // Central knight
	evalCenter := Evaluate(&board)

	if evalCenter <= evalEdge {
		t.Errorf("Central knight should be valued higher than edge knight. Edge: %d, Center: %d", evalEdge, evalCenter)
	}
}

func TestPawnStructureEvaluation(t *testing.T) {
	// Test doubled pawns penalty
	board := Bitboard{}
	board.UpdateSquare(E1, WhiteKing, NoPiece)
	board.UpdateSquare(E8, BlackKing, NoPiece)

	// Two pawns on same file (doubled)
	board.UpdateSquare(E2, WhitePawn, NoPiece)
	board.UpdateSquare(E3, WhitePawn, NoPiece)
	evalDoubled := Evaluate(&board)

	// Two pawns on different files (better structure)
	board.UpdateSquare(E3, NoPiece, WhitePawn) // Remove doubled pawn
	board.UpdateSquare(D2, WhitePawn, NoPiece) // Add pawn on different file
	evalNormal := Evaluate(&board)

	// Normal structure should be better than doubled pawns
	if evalDoubled >= evalNormal {
		t.Errorf("Doubled pawns should be worse than normal pawn structure. Doubled: %d, Normal: %d", evalDoubled, evalNormal)
	}

	// Test isolated pawn penalty
	board = Bitboard{}
	board.UpdateSquare(E1, WhiteKing, NoPiece)
	board.UpdateSquare(E8, BlackKing, NoPiece)

	// Connected pawns
	board.UpdateSquare(D2, WhitePawn, NoPiece)
	board.UpdateSquare(E2, WhitePawn, NoPiece)
	evalConnected := Evaluate(&board)

	// Isolated pawn
	board.UpdateSquare(D2, NoPiece, WhitePawn)
	evalIsolated := Evaluate(&board)

	if evalIsolated >= evalConnected {
		t.Errorf("Isolated pawn should be penalized. Connected: %d, Isolated: %d", evalConnected, evalIsolated)
	}
}

func TestPassedPawnBonus(t *testing.T) {
	// Test passed pawn bonus
	board := Bitboard{}
	board.UpdateSquare(E1, WhiteKing, NoPiece)
	board.UpdateSquare(E8, BlackKing, NoPiece)

	// White pawn with black pawn blocking
	board.UpdateSquare(E4, WhitePawn, NoPiece)
	board.UpdateSquare(E5, BlackPawn, NoPiece) // Blocking pawn
	evalBlocked := Evaluate(&board)

	// Remove blocking pawn - now it's passed
	board.UpdateSquare(E5, NoPiece, BlackPawn)
	evalPassed := Evaluate(&board)

	if evalPassed <= evalBlocked {
		t.Errorf("Passed pawn should get bonus. Blocked: %d, Passed: %d", evalBlocked, evalPassed)
	}
}

func TestKingSafety(t *testing.T) {
	// Test king safety with pawn shield
	board := Bitboard{}

	// King without pawn shield
	board.UpdateSquare(G1, WhiteKing, NoPiece)
	board.UpdateSquare(E8, BlackKing, NoPiece)
	evalNoShield := Evaluate(&board)

	// King with pawn shield
	board.UpdateSquare(F2, WhitePawn, NoPiece)
	board.UpdateSquare(G2, WhitePawn, NoPiece)
	board.UpdateSquare(H2, WhitePawn, NoPiece)
	evalWithShield := Evaluate(&board)

	if evalWithShield <= evalNoShield {
		t.Errorf("King with pawn shield should be safer. No shield: %d, With shield: %d", evalNoShield, evalWithShield)
	}
}

func TestEndgameDetection(t *testing.T) {
	// Test endgame detection
	board := Bitboard{}
	board.UpdateSquare(E1, WhiteKing, NoPiece)
	board.UpdateSquare(E8, BlackKing, NoPiece)

	// Middlegame - lots of pieces
	board.UpdateSquare(D1, WhiteQueen, NoPiece)
	board.UpdateSquare(A1, WhiteRook, NoPiece)
	board.UpdateSquare(C1, WhiteBishop, NoPiece)
	board.UpdateSquare(B1, WhiteKnight, NoPiece)

	board.UpdateSquare(D8, BlackQueen, NoPiece)
	board.UpdateSquare(A8, BlackRook, NoPiece)
	board.UpdateSquare(C8, BlackBishop, NoPiece)
	board.UpdateSquare(B8, BlackKnight, NoPiece)

	if isEndgame(&board) {
		t.Error("Should not detect endgame with many pieces")
	}

	// Remove most pieces for endgame
	board.UpdateSquare(D1, NoPiece, WhiteQueen)
	board.UpdateSquare(A1, NoPiece, WhiteRook)
	board.UpdateSquare(C1, NoPiece, WhiteBishop)

	board.UpdateSquare(D8, NoPiece, BlackQueen)
	board.UpdateSquare(A8, NoPiece, BlackRook)
	board.UpdateSquare(C8, NoPiece, BlackBishop)

	if !isEndgame(&board) {
		t.Error("Should detect endgame with few pieces")
	}
}

func TestMobilityEvaluation(t *testing.T) {
	// Test mobility evaluation
	// Create position with different mobility
	pos := &Position{}
	pos.Board = Bitboard{}
	pos.Board.UpdateSquare(E1, WhiteKing, NoPiece)
	pos.Board.UpdateSquare(E8, BlackKing, NoPiece)

	// White queen with lots of mobility
	pos.Board.UpdateSquare(D4, WhiteQueen, NoPiece) // Central queen

	// Black queen trapped in corner
	pos.Board.UpdateSquare(A8, BlackQueen, NoPiece)
	pos.Board.UpdateSquare(A7, BlackPawn, NoPiece) // Blocking pawn
	pos.Board.UpdateSquare(B8, BlackRook, NoPiece) // Blocking rook

	pos.SetTag(WhiteToMove)
	pos.EnPassant = NoSquare

	eval := Evaluate(&pos.Board)

	// The evaluation should run without crashing and give some reasonable value
	// (The exact sign depends on many factors including PST values and material)
	if eval == 0 {
		t.Errorf("Evaluation should not be exactly zero with pieces on board, got %d", eval)
	}
}

// mirrorBoard returns the color-and-vertical flip of src: every piece moves to
// sq^56 with its color swapped. Evaluate() is a pure white-perspective function
// with no side-to-move dependence, so the EXACT invariant
// Evaluate(P) == -Evaluate(mirrorBoard(P)) must hold for any position. Any
// violation is an asymmetry bug in some eval term.
func mirrorBoard(src *Bitboard) Bitboard {
	var dst Bitboard
	pairs := [6][2]Piece{
		{WhitePawn, BlackPawn}, {WhiteKnight, BlackKnight},
		{WhiteBishop, BlackBishop}, {WhiteRook, BlackRook},
		{WhiteQueen, BlackQueen}, {WhiteKing, BlackKing},
	}
	for _, pr := range pairs {
		for bb := src.GetBitboardOf(pr[0]); bb != 0; bb &= bb - 1 {
			dst.UpdateSquare(Square(bits.TrailingZeros64(bb)^56), pr[1], NoPiece)
		}
		for bb := src.GetBitboardOf(pr[1]); bb != 0; bb &= bb - 1 {
			dst.UpdateSquare(Square(bits.TrailingZeros64(bb)^56), pr[0], NoPiece)
		}
	}
	return dst
}

// randomEvalPosition builds a random (not necessarily legal) board with one
// king per side and a scatter of other pieces. Legality is irrelevant to a
// static-eval symmetry check; coverage of eval code paths is what matters.
// Pawns are kept off ranks 1 and 8.
func randomEvalPosition(rng *rand.Rand) Bitboard {
	var b Bitboard
	var occ [64]bool
	wk, bk := rng.Intn(64), rng.Intn(64)
	for bk == wk {
		bk = rng.Intn(64)
	}
	b.UpdateSquare(Square(wk), WhiteKing, NoPiece)
	b.UpdateSquare(Square(bk), BlackKing, NoPiece)
	occ[wk], occ[bk] = true, true
	choices := []Piece{
		WhitePawn, WhiteKnight, WhiteBishop, WhiteRook, WhiteQueen,
		BlackPawn, BlackKnight, BlackBishop, BlackRook, BlackQueen,
	}
	for n := rng.Intn(22); n > 0; n-- {
		p := choices[rng.Intn(len(choices))]
		sq := rng.Intn(64)
		if occ[sq] {
			continue
		}
		if (p == WhitePawn || p == BlackPawn) && (sq/8 == 0 || sq/8 == 7) {
			continue
		}
		occ[sq] = true
		b.UpdateSquare(Square(sq), p, NoPiece)
	}
	return b
}

func dumpBoard(b *Bitboard) string {
	s := ""
	for sq := 0; sq < 64; sq++ {
		if p := b.PieceAt(Square(sq)); p != NoPiece {
			s += fmt.Sprintf("%d:%d ", sq, int(p))
		}
	}
	return s
}

// TestEvaluationSymmetry sweeps thousands of random positions asserting the
// exact color-mirror invariant Evaluate(P) == -Evaluate(mirror(P)). (Replaces
// an earlier one-position test that only checked the two evals had opposite
// signs — which proved essentially nothing.)
func TestEvaluationSymmetry(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	const iters = 20000
	fails := 0
	for i := 0; i < iters; i++ {
		p := randomEvalPosition(rng)
		m := mirrorBoard(&p)
		ep, em := Evaluate(&p), Evaluate(&m)
		if ep+em != 0 {
			fails++
			if fails <= 8 {
				t.Errorf("asymmetry #%d: Evaluate(P)=%d Evaluate(mirror)=%d sum=%d\n  P: %s\n  M: %s",
					i, ep, em, ep+em, dumpBoard(&p), dumpBoard(&m))
			}
		}
	}
	if fails > 0 {
		t.Fatalf("%d/%d random positions had asymmetric evaluation", fails, iters)
	}
}

// TestEvalSymmetryByTerm isolates WHICH eval term breaks mirror symmetry by
// checking each term's white-perspective net contribution independently.
func TestEvalSymmetryByTerm(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	type term struct {
		name string
		net  func(b *Bitboard) int
	}
	terms := []term{
		{"pesto", func(b *Bitboard) int {
			mg, eg, ph := evaluatePeSTO(b)
			return (mg*ph + eg*(totalPhase-ph)) / totalPhase
		}},
		{"passed", func(b *Bitboard) int {
			wp, bp := b.GetBitboardOf(WhitePawn), b.GetBitboardOf(BlackPawn)
			return (countPassedPawnsFast(wp, bp, White) - countPassedPawnsFast(bp, wp, Black)) * 20
		}},
		{"doubled", func(b *Bitboard) int {
			w, bl := 0, 0
			for f := 0; f < 8; f++ {
				w += countDoubledPawns(b, White, f)
				bl += countDoubledPawns(b, Black, f)
			}
			return -(w - bl) * 20
		}},
		{"isolated", func(b *Bitboard) int {
			return -(countIsolatedPawns(b, White) - countIsolatedPawns(b, Black)) * 12
		}},
		{"chains", func(b *Bitboard) int {
			return evaluatePawnChains(b, White) - evaluatePawnChains(b, Black)
		}},
		{"bishoppair", func(b *Bitboard) int {
			return evaluateBishopPair(b, White) - evaluateBishopPair(b, Black)
		}},
		{"mobility", func(b *Bitboard) int {
			return evaluateMobility(b, White) - evaluateMobility(b, Black)
		}},
		{"rookopen", func(b *Bitboard) int {
			return evaluateRookOnOpenFile(b, White) - evaluateRookOnOpenFile(b, Black)
		}},
		{"outposts", func(b *Bitboard) int {
			wp, bp := b.GetBitboardOf(WhitePawn), b.GetBitboardOf(BlackPawn)
			return evaluateOutpostSquares(b.GetBitboardOf(WhiteKnight), b.GetBitboardOf(WhiteBishop), wp, bp, White) -
				evaluateOutpostSquares(b.GetBitboardOf(BlackKnight), b.GetBitboardOf(BlackBishop), bp, wp, Black)
		}},
		{"kingsafety", func(b *Bitboard) int {
			var sliderAtt [64]uint64
			fillSliderAttacks(b, &sliderAtt)
			return evaluateKingSafety(b, White, &sliderAtt) - evaluateKingSafety(b, Black, &sliderAtt)
		}},
		{"kingactivity", func(b *Bitboard) int {
			return evaluateKingActivity(b)
		}},
	}
	const iters = 20000
	counts := make([]int, len(terms))
	for i := 0; i < iters; i++ {
		p := randomEvalPosition(rng)
		m := mirrorBoard(&p)
		for j, tm := range terms {
			if tm.net(&p)+tm.net(&m) != 0 {
				counts[j]++
			}
		}
	}
	for j, tm := range terms {
		status := "ok"
		if counts[j] > 0 {
			status = "ASYMMETRIC"
		}
		t.Logf("%-14s %-11s (%d/%d)", tm.name, status, counts[j], iters)
	}
}

func BenchmarkEvaluation(b *testing.B) {
	board := StartingBoard()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Evaluate(&board)
	}
}

func BenchmarkEvaluationComplex(b *testing.B) {
	// More complex position with many pieces
	board := Bitboard{}
	board.UpdateSquare(E1, WhiteKing, NoPiece)
	board.UpdateSquare(D1, WhiteQueen, NoPiece)
	board.UpdateSquare(A1, WhiteRook, NoPiece)
	board.UpdateSquare(H1, WhiteRook, NoPiece)
	board.UpdateSquare(C1, WhiteBishop, NoPiece)
	board.UpdateSquare(F1, WhiteBishop, NoPiece)
	board.UpdateSquare(B1, WhiteKnight, NoPiece)
	board.UpdateSquare(G1, WhiteKnight, NoPiece)

	for file := 0; file < 8; file++ {
		board.UpdateSquare(Square(8+file), WhitePawn, NoPiece)
		board.UpdateSquare(Square(48+file), BlackPawn, NoPiece)
	}

	board.UpdateSquare(E8, BlackKing, NoPiece)
	board.UpdateSquare(D8, BlackQueen, NoPiece)
	board.UpdateSquare(A8, BlackRook, NoPiece)
	board.UpdateSquare(H8, BlackRook, NoPiece)
	board.UpdateSquare(C8, BlackBishop, NoPiece)
	board.UpdateSquare(F8, BlackBishop, NoPiece)
	board.UpdateSquare(B8, BlackKnight, NoPiece)
	board.UpdateSquare(G8, BlackKnight, NoPiece)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Evaluate(&board)
	}
}
