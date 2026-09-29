package engine

import "testing"

// TestEndgameDrawScaling locks the generalized endgame draw-scale (Counter computeFactor
// family, eval lever E1): it must pull a provably-drawish ending toward the draw, reproduce
// the OCB case at /2, hit the pawnless-cannot-win case, and leave full material at identity.
func TestEndgameDrawScaling(t *testing.T) {
	mag := func(x int) int {
		if x < 0 {
			return -x
		}
		return x
	}
	scaleOf := func(fen string) int {
		p, err := ParseFEN(fen)
		if err != nil {
			t.Fatalf("bad FEN %q: %v", fen, err)
		}
		return drawScale(&p.Board, Evaluate(&p.Board))
	}

	// OCB ending, White up a pawn (Bd3 light vs Be7 dark) — the old ocbScale case, factor 2.
	if s := scaleOf("4k3/4bp2/8/8/8/3B4/P4P2/4K3 w - - 0 1"); s != 32 {
		t.Errorf("OCB ending: drawScale=%d, want 32 (factor 2)", s)
	}
	// KB vs bare K — pawnless, cannot win, factor 16.
	if s := scaleOf("4k3/8/8/8/8/3B4/8/4K3 w - - 0 1"); s != 4 {
		t.Errorf("KBvK: drawScale=%d, want 4 (factor 16)", s)
	}
	// Full material (startpos) — identity.
	if s := scaleOf("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"); s != 64 {
		t.Errorf("startpos: drawScale=%d, want 64 (identity)", s)
	}

	// A drawish minor-piece ending (KBPvKN) must have its eval magnitude pulled DOWN vs the
	// unscaled core — the whole point of E1 (NGN used to score these at full material).
	p, err := ParseFEN("4k3/8/4n3/8/8/3B4/4P3/4K3 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	raw := evaluateUnsafe(&p.Board)
	scaled := Evaluate(&p.Board)
	if mag(scaled) >= mag(raw) {
		t.Errorf("KBPvKN eval not pulled toward the draw: raw=%d scaled=%d", raw, scaled)
	}
}
