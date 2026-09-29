package engine

import (
	"testing"
)

// TestHashSymmetricUnderMakeUnmake walks a deep tree of moves applying
// MakeMove then UnMakeMove. After every unmake, the position hash must
// equal the hash before the corresponding make.
func TestHashSymmetricUnderMakeUnmake(t *testing.T) {
	fens := []string{
		"rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
		"r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1", // Kiwipete
		"r1bqk2r/ppp2ppp/2n2n2/2b1p3/2B1P3/3P1N2/PPP2PPP/RNBQK2R w KQkq - 4 6",
		"4k3/8/8/8/8/8/4P3/4K3 w - - 0 1",                             // K+P endgame
		"r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1",                        // Castle test
		"rnbq1bnr/ppp1pppp/8/3pP3/8/8/PPPP1PPP/RNBQKBNR b KQkq - 0 2", // EP available
	}
	for _, fen := range fens {
		t.Run(fen[:20], func(t *testing.T) {
			pos, err := ParseFEN(fen)
			if err != nil {
				t.Fatal(err)
			}
			walk(t, pos, 3)
		})
	}
}

func walk(t *testing.T, pos *Position, depth int) {
	if depth == 0 {
		return
	}
	hashBefore := pos.Hash()
	var buf [256]Move
	n := GenerateMovesIntoBuffer(pos, buf[:])
	for i := 0; i < n; i++ {
		m := buf[i]
		ep, tag, hc, _ := pos.MakeMove(m)
		if isInCheck(pos, m.MovingPiece().Color()) {
			pos.UnMakeMove(m, tag, ep, hc)
			if pos.Hash() != hashBefore {
				t.Errorf("hash mismatch on illegal-move undo: move=%s before=%x after=%x",
					m.ToString(), hashBefore, pos.Hash())
				return
			}
			continue
		}
		walk(t, pos, depth-1)
		pos.UnMakeMove(m, tag, ep, hc)
		if pos.Hash() != hashBefore {
			t.Errorf("hash mismatch after unmake: move=%s before=%x after=%x",
				m.ToString(), hashBefore, pos.Hash())
			return
		}
	}
}
