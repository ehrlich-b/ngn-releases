package engine

import (
	"strings"
	"testing"
)

// game46History is the recorded game through the position before 96.e5e8.
// It is a regression witness, not a synthetic chess position.
const game46History = `c2c3 b7b5 b2b3 e7e5 f2f4 c8a6 f4e5 d7d6 e5e6 f7e6 g1f3 g8f6 e2e3 f8e7 a2a4 b5a4 f1a6 b8a6 b3b4 e8g8 d1a4 a6b8 e1g1 d6d5 f3g5 d8c8 a4c2 h7h6 f1f6 h6g5 f6f8 c8f8 e3e4 f8f7 d2d4 b8d7 c2e2 d7f6 b1d2 f7g6 e4e5 f6h5 d2f1 h5f4 e2g4 g6h5 g4h5 f4h5 g2g3 g8f7 f1e3 f7e8 a1a6 e8d7 e3g4 c7c6 g1g2 g7g6 g2f1 e7d8 f1e2 d8e7 e2f2 a8f8 f2e3 f8a8 e3e2 e7d8 e2f3 d8e7 f3e2 e7d8 e2d3 d8e7 c1e3 e7d8 d3e2 d8e7 e2d3 e7d8 a6a2 a7a6 a2a1 d7e8 g4f2 d8e7 f2h3 g5g4 h3f2 h5g7 f2g4 g7f5 e3d2 e8f7 g4e3 f5e3 d2e3 g6g5 h2h3 f7g6 a1a4 e7d8 a4a1 d8e7 e3d2 g6h5 c3c4 d5c4 d3c4 g5g4 a1h1 a8e8 h3g4 h5g4 h1h6 e7d8 h6g6 g4f3 c4d3 d8b6 g3g4 e8d8 d2c3 a6a5 b4a5 b6d4 c3d4 c6c5 a5a6 d8d4 d3c3 d4a4 g6e6 f3g4 e6b6 g4f5 e5e6 f5f6 c3b3 a4a1 b3c4 a1a5 c4d5 c5c4 d5c4 f6e7 c4b4 a5a1 b4b5 e7f6 b6c6 a1b1 b5a5 b1b8 a6a7 b8e8 a5a6 f6f5 a6b7 f5e5 a7a8q e8a8 b7a8 e5f6 a8b7 f6e7 b7c7 e7f6 c7b7 f6e7 b7c7 e7f6 c7d6 f6f5 e6e7 f5e4 e7e8q e4f4 d6d7 f4f5 c6g6 f5f4 g6f6 f4g5 e8e5 g5g4 e5e8 g4g5 e8e5 g5g4`
const game46FEN = "8/3K4/5R2/4Q3/6k1/8/8/8 w - - 13 96"

func replayGame46(t *testing.T) *Position {
	t.Helper()
	pos, err := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	for ply, text := range strings.Fields(game46History) {
		m, err := ParseUCIMove(pos, text)
		if err != nil {
			t.Fatalf("ply %d %s: %v", ply+1, text, err)
		}
		if _, _, _, ok := pos.GameMakeMove(m); !ok {
			t.Fatalf("ply %d %s illegal", ply+1, text)
		}
	}
	want, err := ParseFEN(game46FEN)
	if err != nil {
		t.Fatal(err)
	}
	if pos.Hash() != want.Hash() {
		t.Fatalf("history hash=%x want=%x fen=%s", pos.Hash(), want.Hash(), GenerateFEN(pos))
	}
	return pos
}

func resetMateProbe() {
	defaultSearchEngine, _ = NewSearchEngineWithHash(DEFAULT_CACHE_SIZE)
	ClearHistoryTable()
	ClearKillerMoves()
	ClearCounterMoves()
	ClearStop()
}

func TestGame46HistoryFreeMateAndRecordedThreefold(t *testing.T) {
	// The current board alone makes e5e8 look mating. The full game history is
	// deliberately absent from this child search, matching the TT/qsearch risk.
	free, err := ParseFEN(game46FEN)
	if err != nil {
		t.Fatal(err)
	}
	move, err := ParseUCIMove(free, "e5e8")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, ok := free.GameMakeMove(move); !ok {
		t.Fatal("e5e8 illegal")
	}
	resetMateProbe()
	child := SearchIterativeDeepening(free, 8, nil)
	if child.BestScore > -MATE_IN_MAX {
		t.Fatalf("history-free e5e8 child score=%d, want mate", child.BestScore)
	}

	// On the actual recorded history, e5e8 followed by the recorded reply ends
	// in a third occurrence. This is the concrete draw that a depth-one cached
	// mate must not settle before the root has verified it.
	history := replayGame46(t)
	for _, text := range []string{"e5e8", "g4g5"} {
		m, err := ParseUCIMove(history, text)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, _, ok := history.GameMakeMove(m); !ok {
			t.Fatalf("%s illegal", text)
		}
	}
	if !history.IsFIDEDrawRule() || history.Positions[history.Hash()] < 3 {
		t.Fatalf("recorded pair did not yield threefold: occurrences=%d", history.Positions[history.Hash()])
	}
}
