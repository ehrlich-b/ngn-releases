package engine

import "testing"

// TestEnPassantHashCapturabilityGate guards the en-passant hashing fix: the EP
// square must enter the zobrist hash ONLY when an enemy pawn can actually capture
// en passant. A phantom EP target (set after every double push regardless of
// capturability) split the hash — and therefore the repetition signature — of
// otherwise-identical positions, and fragmented the transposition table. The gate
// lives at the two EP set-sites (makeMoveHelper's double push and ParseFEN), so
// the full hash (generateZobristHash) and the incremental hash (updateHash) stay
// in agreement.
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
	// (a5 king and h5 rook share the rank with the e5/d5 pawns). Adjacency gating
	// retains the EP square (consistent with Stockfish/Polyglot); the property
	// under test is that full and incremental hash still agree.
	hPin, _ := play("6k1/3p4/8/K3P2r/8/8/8/8 b - - 0 1", "d7d5")
	if hPin != full("6k1/8/8/K2pP2r/8/8/8/8 w - d6 0 1") {
		t.Errorf("pinned EP: incremental hash != full hash from FEN")
	}
}
