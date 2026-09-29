package engine

import "testing"

func TestLegalPawnPushProbe(t *testing.T) {
	cases := []struct {
		name, fen string
		want      bool
	}{
		{"white", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1", true},
		{"black", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR b KQkq - 0 1", true},
		{"white pinned", "4k3/8/8/8/8/3b4/4P3/5K2 w - - 0 1", false},
		{"black pinned", "5k2/4p3/3B4/8/8/8/8/4K3 b - - 0 1", false},
		{"rank pinned", "4k3/8/8/8/8/8/r2PK3/8 w - - 0 1", false},
		{"file block preserved", "k3r3/8/8/8/8/8/4P3/4K3 w - - 0 1", true},
		{"unrelated check", "k3r3/8/8/8/8/8/P7/4K3 w - - 0 1", false},
		{"white promotion", "4k3/P7/8/8/8/8/8/4K3 w - - 0 1", true},
		{"black promotion", "4k3/8/8/8/8/8/p7/4K3 b - - 0 1", true},
		{"blocked", "4k3/8/8/8/4p3/4P3/8/4K3 w - - 0 1", false},
		{"stalemate", "7k/5K2/6Q1/8/8/8/8/8 b - - 0 1", false},
		{"only EP evasion", "2B5/8/8/2K1k3/3Pp3/8/8/5R2 b - d3 0 1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := ParseFEN(tc.fen)
			if err != nil {
				t.Fatal(err)
			}
			board, hash, tag, clock, ep := p.Board, p.Hash(), p.Tag, p.HalfMoveClock, p.EnPassant
			if got := hasLegalPawnPush(p); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			if p.Board != board || p.Hash() != hash || p.Tag != tag || p.HalfMoveClock != clock || p.EnPassant != ep {
				t.Fatal("probe changed the position")
			}
		})
	}
}
