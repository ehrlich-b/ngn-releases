package engine

import (
	"fmt"
	"testing"
)

func TestQuiescenceTerminalPositions(t *testing.T) {
	const ply = 4
	for _, tc := range []struct {
		name, fen string
		want      int
	}{
		{"bare kings", "4k3/8/8/8/8/8/8/4K3 w - - 0 1", 0},
		{"bishop versus king", "4k3/8/8/8/8/2B5/8/4K3 w - - 0 1", 0},
		{"fifty moves", "4k3/8/8/8/8/8/8/R3K3 w - - 100 1", 0},
		{"fifty moves in check with evasion", "4k3/8/8/8/8/8/8/K3R3 b - - 100 1", 0},
		{"stalemate", "7k/5K2/6Q1/8/8/8/8/8 b - - 0 1", 0},
		{"mate at fifty moves", "7k/6Q1/6K1/8/8/8/8/8 b - - 100 1", -MATE_VALUE + ply},
	} {
		for _, qDepth := range []int{0, 1, 6} {
			t.Run(fmt.Sprintf("%s/q%d", tc.name, qDepth), func(t *testing.T) {
				pos, err := ParseFEN(tc.fen)
				if err != nil {
					t.Fatal(err)
				}
				ClearStop()
				ClearHistoryTable()
				defaultSearchEngine, _ = NewSearchEngineWithHash(1)
				info := newQSearchInfo()
				got := quiescenceWithDepth(pos, -INFINITY, INFINITY, ply, info, qDepth)
				if got != tc.want {
					t.Fatalf("score=%d, want %d", got, tc.want)
				}
			})
		}
	}
}

func TestMateOnFiftiethMove(t *testing.T) {
	for _, search := range []struct {
		name string
		run  func(*Position, int, *TimeManager) *SearchInfo
	}{
		{"iterative", SearchIterativeDeepening},
		{"fixed", SearchFixed},
	} {
		for _, depth := range []int{1, 3} {
			t.Run(fmt.Sprintf("%s/d%d", search.name, depth), func(t *testing.T) {
				pos, err := ParseFEN("7k/R7/6K1/8/8/8/8/8 w - - 99 1")
				if err != nil {
					t.Fatal(err)
				}
				ClearStop()
				ClearHistoryTable()
				ClearKillerMoves()
				ClearCounterMoves()
				defaultSearchEngine, _ = NewSearchEngineWithHash(1)
				info := search.run(pos, depth, nil)
				if info.BestMove.ToString() != "a7a8" || info.BestScore != MATE_VALUE-1 {
					t.Fatalf("move=%s score=%d, want Ra8# score=%d", info.BestMove.ToString(), info.BestScore, MATE_VALUE-1)
				}
			})
		}
	}
}

func TestQuiescenceRepetition(t *testing.T) {
	const fen = "4k3/8/8/8/8/8/8/R3K3 w - - 10 1"
	for _, gameCount := range []int{1, 2} {
		t.Run(fmt.Sprintf("game count %d", gameCount), func(t *testing.T) {
			pos, err := ParseFEN(fen)
			if err != nil {
				t.Fatal(err)
			}
			ClearStop()
			ClearHistoryTable()
			defaultSearchEngine, _ = NewSearchEngineWithHash(1)
			pos.Positions[pos.Hash()] = gameCount
			got := quiescenceWithDepth(pos, -INFINITY, INFINITY, 4, newQSearchInfo(), 1)
			if gameCount == 2 && got != 0 {
				t.Fatalf("third occurrence scored %d, want draw", got)
			}
			if gameCount == 1 && got < 300 {
				t.Fatalf("second occurrence scored %d, want the winning rook eval", got)
			}
		})
	}
	t.Run("search path", func(t *testing.T) {
		pos, err := ParseFEN(fen)
		if err != nil {
			t.Fatal(err)
		}
		ClearStop()
		defaultSearchEngine, _ = NewSearchEngineWithHash(1)
		info := newQSearchInfo()
		info.RepStack[0] = pos.Hash()
		info.RepStackLen = 4
		got := quiescenceWithDepth(pos, -INFINITY, INFINITY, 5, info, 1)
		if got != 0 {
			t.Fatalf("search repetition scored %d, want draw", got)
		}
		if info.RepStackLen != 4 {
			t.Fatal("qsearch did not restore the repetition stack")
		}
	})
}
