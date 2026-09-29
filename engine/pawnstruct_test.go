package engine

import "testing"

// TestPawnStructureCounts checks the hand-optimized pawn-structure counters
// against crafted positions with known counts. The eval mirror-symmetry test
// catches color-ASYMMETRIC bugs; this catches a count that is simply wrong (a
// symmetric miscount). countPassedPawnsFast had exactly such a bug historically
// (the phantom passed-pawn mask), so both colors are exercised.
func TestPawnStructureCounts(t *testing.T) {
	sumDoubled := func(b *Bitboard, c Color) int {
		n := 0
		for f := 0; f < 8; f++ {
			n += countDoubledPawns(b, c, f)
		}
		return n
	}

	doubled := []struct {
		name string
		fen  string
		want int
	}{
		{"doubled e-file", "4k3/8/8/8/4P3/8/4P3/4K3 w - - 0 1", 1},
		{"tripled a-file", "4k3/8/8/8/P7/P7/P7/4K3 w - - 0 1", 2},
		{"no doubled", "4k3/8/8/8/3PP3/8/8/4K3 w - - 0 1", 0},
	}
	for _, c := range doubled {
		pos, err := ParseFEN(c.fen)
		if err != nil {
			t.Errorf("%s: bad FEN: %v", c.name, err)
			continue
		}
		if got := sumDoubled(&pos.Board, White); got != c.want {
			t.Errorf("doubled[%s] = %d, want %d", c.name, got, c.want)
		}
	}

	isolated := []struct {
		name string
		fen  string
		want int
	}{
		{"single isolated a-pawn", "4k3/8/8/8/P7/8/8/4K3 w - - 0 1", 1},
		{"connected d+e", "4k3/8/8/8/3PP3/8/8/4K3 w - - 0 1", 0},
		{"two isolated a+c", "4k3/8/8/8/P1P5/8/8/4K3 w - - 0 1", 2},
	}
	for _, c := range isolated {
		pos, err := ParseFEN(c.fen)
		if err != nil {
			t.Errorf("%s: bad FEN: %v", c.name, err)
			continue
		}
		if got := countIsolatedPawns(&pos.Board, White); got != c.want {
			t.Errorf("isolated[%s] = %d, want %d", c.name, got, c.want)
		}
	}

	passed := []struct {
		name  string
		fen   string
		color Color
		want  int
	}{
		{"white clear passer e5", "4k3/8/8/4P3/8/8/8/4K3 w - - 0 1", White, 1},
		{"white blocked by e7", "4k3/4p3/8/4P3/8/8/8/4K3 w - - 0 1", White, 0},
		{"white adjacent-file f6", "4k3/8/5p2/4P3/8/8/8/4K3 w - - 0 1", White, 0},
		{"black clear passer e4", "4k3/8/8/8/4p3/8/8/4K3 w - - 0 1", Black, 1},
		{"black blocked by e2", "4k3/8/8/8/4p3/8/4P3/4K3 w - - 0 1", Black, 0},
		{"black adjacent-file d3", "4k3/8/8/8/4p3/3P4/8/4K3 w - - 0 1", Black, 0},
	}
	for _, c := range passed {
		pos, err := ParseFEN(c.fen)
		if err != nil {
			t.Errorf("%s: bad FEN: %v", c.name, err)
			continue
		}
		own := pos.Board.GetBitboardOf(WhitePawn)
		opp := pos.Board.GetBitboardOf(BlackPawn)
		if c.color == Black {
			own, opp = opp, own
		}
		if got := countPassedPawnsFast(own, opp, c.color); got != c.want {
			t.Errorf("passed[%s] = %d, want %d", c.name, got, c.want)
		}
	}
}
