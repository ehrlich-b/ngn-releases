package engine

import (
	"testing"
)

func TestPassedPawnSlowVsFast(t *testing.T) {
	tests := []struct {
		name string
		fen  string
	}{
		{"start", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"},
		{"blocked_by_far", "k7/p7/8/8/8/8/P7/K7 w - - 0 1"},
		{"blocker_close", "k7/8/p7/8/8/P7/8/K7 w - - 0 1"},
		{"true_passed", "k7/8/8/8/8/p7/P7/K7 w - - 0 1"},
		{"adj_file_far", "k7/1p6/8/8/8/8/P7/K7 w - - 0 1"},
		{"adj_file_2ranks", "k7/8/1p6/8/8/8/P7/K7 w - - 0 1"},
		{"both_passed", "k7/8/8/2p5/3P4/8/8/K7 w - - 0 1"},
		{"endgame_passed", "8/8/2k5/3P4/8/8/3K4/8 w - - 0 1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pos, err := ParseFEN(tc.fen)
			if err != nil {
				t.Fatalf("ParseFEN: %v", err)
			}
			b := &pos.Board
			wp := b.GetBitboardOf(WhitePawn)
			bp := b.GetBitboardOf(BlackPawn)
			whiteSlow := countPassedPawns(b, White)
			whiteFast := countPassedPawnsFast(wp, bp, White)
			blackSlow := countPassedPawns(b, Black)
			blackFast := countPassedPawnsFast(bp, wp, Black)
			t.Logf("%s: white slow=%d fast=%d  black slow=%d fast=%d",
				tc.name, whiteSlow, whiteFast, blackSlow, blackFast)
			if whiteSlow != whiteFast {
				t.Errorf("WHITE mismatch slow=%d fast=%d", whiteSlow, whiteFast)
			}
			if blackSlow != blackFast {
				t.Errorf("BLACK mismatch slow=%d fast=%d", blackSlow, blackFast)
			}
		})
	}
}
