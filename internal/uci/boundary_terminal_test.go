package uci

import (
	"bufio"
	"io"
	"strings"
	"testing"
)

func TestTerminalStatesWinAtAndBeforeMaxMoves(t *testing.T) {
	mates := []struct {
		name, fen, move string
		newIsWhite      bool
		want            Outcome
	}{
		{"new white mates", "7k/5P2/6K1/8/8/8/8/8 w - - 0 1", "f7f8q", true, Win},
		{"base white mates", "7k/5P2/6K1/8/8/8/8/8 w - - 0 1", "f7f8q", false, Loss},
		{"new black mates", "8/8/8/8/8/6k1/5p2/7K b - - 0 1", "f2f1q", false, Win},
		{"base black mates", "8/8/8/8/8/6k1/5p2/7K b - - 0 1", "f2f1q", true, Loss},
	}
	for _, tc := range mates {
		t.Run(tc.name, func(t *testing.T) {
			newEng, baseEng := scriptedEngine(), scriptedEngine()
			moverIsWhite := strings.Contains(tc.fen, " w ")
			if moverIsWhite == tc.newIsWhite {
				newEng = scriptedEngine(tc.move)
			} else {
				baseEng = scriptedEngine(tc.move)
			}
			var played []string
			got := PlayGame(newEng, baseEng, []string{tc.fen}, tc.newIsWhite, GameConfig{NewDepth: 1, BaseDepth: 1, MaxMoves: 1, MovesOut: &played})
			if got.Res != tc.want || got.Reason != "checkmate" || len(played) != 1 {
				t.Fatalf("got=%+v moves=%v", got, played)
			}
		})
	}
	var played []string
	got := PlayGame(scriptedEngine(), scriptedEngine(), []string{"7k/6Q1/6K1/8/8/8/8/8 b - - 0 1"}, true, GameConfig{MaxMoves: 0, MovesOut: &played})
	if got.Res != Win || got.Reason != "checkmate" || len(played) != 0 {
		t.Fatalf("pre-existing cap-zero mate got=%+v moves=%v", got, played)
	}
}

func TestBoundaryStalemateRepetitionAndOrdinaryCap(t *testing.T) {
	var played []string
	got := PlayGame(scriptedEngine("c6c7"), scriptedEngine(), []string{"k7/8/2QK4/8/8/8/8/8 w - - 0 1"}, true, GameConfig{NewDepth: 1, BaseDepth: 1, MaxMoves: 1, MovesOut: &played})
	if got.Res != Draw || got.Reason != "stalemate" || len(played) != 1 {
		t.Fatalf("boundary stalemate got=%+v moves=%v", got, played)
	}
	played = nil
	got = PlayGame(scriptedEngine("g1f3", "f3g1", "g1f3", "f3g1"), scriptedEngine("g8f6", "f6g8", "g8f6", "f6g8"), nil, true, GameConfig{NewDepth: 1, BaseDepth: 1, MaxMoves: 8, MovesOut: &played})
	if got.Res != Draw || got.Reason != "draw-rule" || len(played) != 8 {
		t.Fatalf("boundary repetition got=%+v moves=%v", got, played)
	}
	played = nil
	got = PlayGame(scriptedEngine("a1a2"), scriptedEngine(), []string{"7k/8/8/8/8/8/6K1/R7 w - - 99 1"}, true, GameConfig{NewDepth: 1, BaseDepth: 1, MaxMoves: 1, MovesOut: &played})
	if got.Res != Draw || got.Reason != "draw-rule" || len(played) != 1 {
		t.Fatalf("final-ply fifty-move draw got=%+v moves=%v", got, played)
	}
	played = nil
	got = PlayGame(scriptedEngine("e2e4"), scriptedEngine(), nil, true, GameConfig{NewDepth: 1, BaseDepth: 1, MaxMoves: 1, MovesOut: &played})
	if got.Res != Draw || got.Reason != "max-moves" || len(played) != 1 {
		t.Fatalf("ordinary cap got=%+v moves=%v", got, played)
	}
}

func TestTerminalPrecedesEarlyAdjudication(t *testing.T) {
	// A mate on this ply must outrank a one-ply low-score adjudication.
	lines := "readyok\ninfo depth 1 score cp 0\nbestmove f7f8q\n"
	eng := &Engine{stdin: bufio.NewWriter(io.Discard), stdout: bufio.NewScanner(strings.NewReader(lines))}
	got := PlayGame(eng, scriptedEngine(), []string{"7k/5P2/6K1/8/8/8/8/8 w - - 0 1"}, true, GameConfig{NewDepth: 1, BaseDepth: 1, MaxMoves: 4, Adj: AdjConfig{DrawScore: 1, DrawPlies: 1}})
	if got.Res != Win || got.Reason != "checkmate" {
		t.Fatalf("terminal must precede adj, got=%+v", got)
	}
	lines = "readyok\ninfo depth 1 score cp 0\nbestmove c6c7\n"
	eng = &Engine{stdin: bufio.NewWriter(io.Discard), stdout: bufio.NewScanner(strings.NewReader(lines))}
	got = PlayGame(eng, scriptedEngine(), []string{"k7/8/2QK4/8/8/8/8/8 w - - 0 1"}, true, GameConfig{NewDepth: 1, BaseDepth: 1, MaxMoves: 4, Adj: AdjConfig{DrawScore: 1, DrawPlies: 1}})
	if got.Res != Draw || got.Reason != "stalemate" {
		t.Fatalf("stalemate must precede adj, got=%+v", got)
	}
}
