package engine

import "testing"

// TestMoveIsPseudoLegalAcceptsGeneratedMoves: every NON-special pseudo-legal move
// the generator produces must be accepted (so the staged search never wrongly
// rejects a real TT move). Special moves (promo/castle/ep/double-push) are
// intentionally deferred and skipped here.
func TestMoveIsPseudoLegalAcceptsGeneratedMoves(t *testing.T) {
	fens := []string{
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",             // startpos
		"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1", // kiwipete
		"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R b KQkq - 0 1", // kiwipete, black
		"8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1",                            // endgame
		"r1bq1rk1/pp2bppp/2n2n2/2pp4/3P4/2N1PN2/PPQ1BPPP/R1B2RK1 w - - 0 10",   // middlegame
		"rnbqkbnr/ppp1pppp/8/3p4/4P3/8/PPPP1PPP/RNBQKBNR w KQkq d6 0 2",        // pawn capture available
	}
	for _, fen := range fens {
		pos, err := ParseFEN(fen)
		if err != nil {
			t.Fatalf("ParseFEN(%q): %v", fen, err)
		}
		for _, m := range GenerateMoves(pos) {
			special := m.PromoType() != NoType || m.IsCastle() || m.IsEnPassant()
			if m.MovingPiece().Type() == Pawn {
				if d := int(m.Destination()) - int(m.Source()); d == 16 || d == -16 {
					special = true // double push (deferred)
				}
			}
			if special {
				continue
			}
			if !moveIsPseudoLegal(pos, m) {
				t.Errorf("%s: rejected generated move src=%d dst=%d piece=%d", fen, m.Source(), m.Destination(), m.MovingPiece())
			}
		}
	}
}

// TestMoveIsPseudoLegalRejectsIllegal: crafted moves that are NOT pseudo-legal must
// be rejected, so a hash collision can never make the staged search play garbage.
func TestMoveIsPseudoLegalRejectsIllegal(t *testing.T) {
	pos, err := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	bad := []struct {
		m   Move
		why string
	}{
		{NewMove(D4, D5, WhiteQueen, NoPiece, NoType, 0), "no white queen on d4"},
		{NewMove(B1, D2, WhiteKnight, WhitePawn, NoType, Capture), "captures own pawn"},
		{NewMove(A1, A4, WhiteRook, NoPiece, NoType, 0), "rook through own a2 pawn"},
		{NewMove(C1, H6, WhiteBishop, NoPiece, NoType, 0), "bishop through own pawn"},
		{NewMove(B1, B3, WhiteKnight, NoPiece, NoType, 0), "not a knight move"},
		{NewMove(E2, E5, WhitePawn, NoPiece, NoType, 0), "pawn jumps three"},
		{NewMove(D1, D2, WhiteQueen, WhitePawn, NoType, Capture), "captured piece mismatch (d2 is a pawn, but its own)"},
	}
	for _, b := range bad {
		if moveIsPseudoLegal(pos, b.m) {
			t.Errorf("accepted illegal move (%s)", b.why)
		}
	}
}
