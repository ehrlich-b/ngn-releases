package uci

import (
	"bufio"
	"io"
	"strings"
	"testing"
)

func scriptedEngine(moves ...string) *Engine {
	lines := "readyok\n"
	for _, move := range moves {
		lines += "bestmove " + move + "\n"
	}
	return &Engine{stdin: bufio.NewWriter(io.Discard), stdout: bufio.NewScanner(strings.NewReader(lines))}
}

func TestPlayGameRejectsIllegalMove(t *testing.T) {
	var played []string
	got := PlayGame(scriptedEngine("a1a8"), scriptedEngine(), nil, true,
		GameConfig{NewDepth: 1, BaseDepth: 1, MaxMoves: 1, MovesOut: &played})
	if got.Res != Loss || got.Reason != "illegal-move" || len(played) != 0 {
		t.Fatalf("illegal rook jump was applied: result=%+v moves=%v", got, played)
	}
}

func TestPlayGameCountsInitialPositionOnce(t *testing.T) {
	for _, tc := range []struct {
		name       string
		max, plies int
		reason     string
	}{
		{"second occurrence continues", 5, 5, "max-moves"},
		{"third occurrence draws", 9, 8, "draw-rule"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var played []string
			got := PlayGame(scriptedEngine("g1f3", "f3g1", "g1f3", "f3g1", "e2e4"),
				scriptedEngine("g8f6", "f6g8", "g8f6", "f6g8"), nil, true,
				GameConfig{NewDepth: 1, BaseDepth: 1, MaxMoves: tc.max, MovesOut: &played})
			if got.Reason != tc.reason || len(played) != tc.plies {
				t.Fatalf("result=%+v plies=%d, want %s after %d plies", got, len(played), tc.reason, tc.plies)
			}
		})
	}
}

func TestPlayGameAllowsBlackEnPassantEvasion(t *testing.T) {
	var played []string
	got := PlayGame(scriptedEngine("e4d3"), scriptedEngine(),
		[]string{"2B5/8/8/2K1k3/3Pp3/8/8/5R2 b - d3 0 1"}, false,
		GameConfig{NewDepth: 1, BaseDepth: 1, MaxMoves: 1, MovesOut: &played})
	if got.Reason != "max-moves" || len(played) != 1 || played[0] != "e4d3" {
		t.Fatalf("legal check evasion was adjudicated as mate: result=%+v moves=%v", got, played)
	}
}
