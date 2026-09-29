package engine

import "testing"

// TestMoveToSAN covers the SAN rules the PGN exporter relies on: pawn pushes,
// piece moves, castling, captures, promotion, check, and disambiguation.
func TestMoveToSAN(t *testing.T) {
	cases := []struct {
		name, fen, uci, want string
	}{
		{"pawn push", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1", "e2e4", "e4"},
		{"knight", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1", "g1f3", "Nf3"},
		{"castle kingside", "r1bqk1nr/pppp1ppp/2n5/1Bb1p3/4P3/5N2/PPPP1PPP/RNBQK2R w KQkq - 0 1", "e1g1", "O-O"},
		{"pawn capture", "rnbqkbnr/ppp1pppp/8/3p4/4P3/8/PPPP1PPP/RNBQKBNR w KQkq - 0 1", "e4d5", "exd5"},
		{"queen check", "4k3/8/8/8/8/8/4Q3/4K3 w - - 0 1", "e2e7", "Qe7+"},
		{"promotion", "8/P7/8/4k3/8/8/8/6K1 w - - 0 1", "a7a8q", "a8=Q"},
		{"knight file disambig", "4k3/8/8/8/3N1N2/8/8/4K3 w - - 0 1", "d4e6", "Nde6"},
	}
	for _, c := range cases {
		pos, err := ParseFEN(c.fen)
		if err != nil {
			t.Fatalf("%s: ParseFEN: %v", c.name, err)
		}
		m, err := ParseUCIMove(pos, c.uci)
		if err != nil {
			t.Fatalf("%s: ParseUCIMove(%s): %v", c.name, c.uci, err)
		}
		if got := MoveToSAN(m, pos); got != c.want {
			t.Errorf("%s: MoveToSAN(%s) = %q, want %q", c.name, c.uci, got, c.want)
		}
	}
}
