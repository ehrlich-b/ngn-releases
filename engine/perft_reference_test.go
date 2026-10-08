package engine

import "testing"

// Public perft results are behavioral oracles, never engine implementation/data.
// https://www.chessprogramming.org/Perft_Results (positions 1 through 6).
func TestPerftReferenceSuite(t *testing.T) {
	checks := []struct {
		name, fen string
		depth     int
		nodes     uint64
	}{
		{"start", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1", 5, 4865609},
		{"kiwipete", "r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1", 4, 4085603},
		{"rook-ending", "8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1", 4, 43238},
		{"promotions-castling", "r3k2r/Pppp1ppp/1b3nbN/nP6/BBP1P3/q4N2/Pp1P2PP/R2Q1RK1 w kq - 0 1", 4, 422333},
		{"tactical-promotions", "rnbq1k1r/pp1Pbppp/2p5/8/2B5/8/PPP1NnPP/RNBQK2R w KQ - 1 8", 4, 2103487},
		{"middlegame", "r4rk1/1pp1qppp/p1np1n2/2b1p1B1/2B1P1b1/P1NP1N2/1PP1QPPP/R4RK1 w - - 0 10", 4, 3894594},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			pos, err := ParseFEN(check.fen)
			if err != nil {
				t.Fatal(err)
			}
			fen, hash := GenerateFEN(pos), pos.Hash()
			got := RunPerftTest(pos, check.depth).Nodes
			if got != check.nodes {
				t.Fatalf("depth %d nodes=%d want=%d", check.depth, got, check.nodes)
			}
			if GenerateFEN(pos) != fen || pos.Hash() != hash {
				t.Fatal("perft failed to restore its root")
			}
			t.Logf("depth=%d nodes=%d", check.depth, got)
		})
	}
}
