package engine

import "testing"

// TestEnPassantHashCapturabilityGate guards both EP contracts. The raw target is
// retained only with an adjacent pawn, for FEN and move generation. The ordinary Zobrist/repetition key is stricter and includes that target
// only when at least one EP capture is legal. FEN and incremental paths must
// agree under both rules.
func TestEnPassantHashCapturabilityGate(t *testing.T) {
	// Play one UCI move from fen via the search make path (incremental hash).
	play := func(fen, uci string) (uint64, Square) {
		pos, err := ParseFEN(fen)
		if err != nil {
			t.Fatalf("ParseFEN(%q): %v", fen, err)
		}
		mv, err := ParseUCIMove(pos, uci)
		if err != nil {
			t.Fatalf("ParseUCIMove(%q): %v", uci, err)
		}
		pos.MakeMove(mv)
		return pos.Hash(), pos.EnPassant
	}
	// Full from-scratch hash of a position loaded from FEN.
	full := func(fen string) uint64 {
		pos, err := ParseFEN(fen)
		if err != nil {
			t.Fatalf("ParseFEN(%q): %v", fen, err)
		}
		return pos.Hash()
	}

	// Uncapturable: e2-e4 with no black pawn on d4/f4. The EP target must be
	// dropped, so the position hashes identically to the same placement with no EP.
	hUnc, epUnc := play("4k3/8/8/8/8/8/4P3/4K3 w - - 0 1", "e2e4")
	if epUnc != NoSquare {
		t.Errorf("uncapturable e2-e4: EnPassant=%v, want NoSquare (no adjacent enemy pawn)", epUnc)
	}
	if hUnc != full("4k3/8/8/8/4P3/8/8/4K3 b - - 0 1") {
		t.Errorf("uncapturable EP changed the hash: the phantom EP square was not gated out")
	}

	// Capturable: a black pawn on d4 can take e3. EP is a real distinguishing
	// feature: it must change the hash, and incremental (played) must equal full.
	hCap, epCap := play("4k3/8/8/8/3p4/8/4P3/4K3 w - - 0 1", "e2e4")
	if epCap == NoSquare {
		t.Errorf("capturable e2-e4: EnPassant dropped, want a real EP square")
	}
	if hCap == full("4k3/8/8/8/3pP3/8/8/4K3 b - - 0 1") {
		t.Errorf("capturable EP did not affect the hash (it should distinguish the position)")
	}
	if hCap != full("4k3/8/8/8/3pP3/8/8/4K3 b - e3 0 1") {
		t.Errorf("capturable EP: incremental hash != full hash from FEN")
	}

	// Pinned: an adjacent capturer exists but the EP capture is pin-illegal
	// (a5 king and h5 rook share the rank with the e5/d5 pawns). The raw target
	// remains for FEN and move generation. Stockfish 18's
	// real-move repetition key instead applies legal-EP normalization (its FEN
	// loader retains the pseudo target); NGN's full and incremental ordinary keys
	// must agree on that legal identity.
	hPin, _ := play("6k1/3p4/8/K3P2r/8/8/8/8 b - - 0 1", "d7d5")
	if hPin != full("6k1/8/8/K2pP2r/8/8/8/8 w - d6 0 1") {
		t.Errorf("pinned EP: incremental hash != full hash from FEN")
	}
}
