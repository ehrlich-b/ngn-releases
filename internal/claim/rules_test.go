package claim

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ehrlich-b/ngn/engine"
)

func testPosition(t *testing.T, fen string) *engine.Position {
	t.Helper()
	p, e := engine.ParseFEN(fen)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func terminalPair(t *testing.T, p *engine.Position, seen map[string]int, half, total, cap int, wantResult, wantReason string) {
	t.Helper()
	result, reason := AuditTerminal(p, seen, half, total, cap)
	if result != wantResult || reason != wantReason {
		t.Fatalf("audit got %s/%s want %s/%s", result, reason, wantResult, wantReason)
	}
	result, reason = RunnerTerminal(p, total, cap)
	if result != wantResult || reason != wantReason {
		t.Fatalf("runner got %s/%s want %s/%s", result, reason, wantResult, wantReason)
	}
}
func TestTerminalPriorityAndCapBoundary(t *testing.T) {
	p := testPosition(t, StartFEN)
	seen := map[string]int{PositionKey(p): 1}
	half := 0
	terminalPair(t, p, seen, half, 399, 400, "", "")
	terminalPair(t, p, seen, half, 400, 400, "1/2-1/2", "ply-cap")
	for _, token := range []string{"f2f3", "e7e5", "g2g4", "d8h4"} {
		if e := advanceAudit(p, token, seen, &half); e != nil {
			t.Fatal(e)
		}
	}
	terminalPair(t, p, seen, half, 400, 400, "0-1", "checkmate")
	// Mate beats the halfmove/repetition boundaries as well as the cap.
	p.HalfMoveClock = 100
	p.Positions[p.Hash()] = 3
	seen[PositionKey(p)] = 3
	terminalPair(t, p, seen, 100, 400, 400, "0-1", "checkmate")
	p = testPosition(t, "k7/8/1QK5/8/8/8/8/8 b - - 100 1")
	seen = map[string]int{PositionKey(p): 3}
	p.Positions[p.Hash()] = 3
	terminalPair(t, p, seen, 100, 400, 400, "1/2-1/2", "stalemate")
}
func TestRepetitionAtCapAndBeforeBoundary(t *testing.T) {
	p := testPosition(t, StartFEN)
	seen := map[string]int{PositionKey(p): 1}
	half := 0
	for i, token := range []string{"g1f3", "g8f6", "f3g1", "f6g8", "g1f3", "g8f6", "f3g1", "f6g8"} {
		if e := advanceAudit(p, token, seen, &half); e != nil {
			t.Fatal(e)
		}
		if i < 7 {
			terminalPair(t, p, seen, half, i+1, 400, "", "")
		}
	}
	terminalPair(t, p, seen, half, 400, 400, "1/2-1/2", "repetition")
}
func TestFiftyMoveBoundaryAndCaptureReset(t *testing.T) {
	p := testPosition(t, "7k/p7/8/8/8/8/8/R6K w - - 99 1")
	seen := map[string]int{PositionKey(p): 1}
	half := 99
	terminalPair(t, p, seen, half, 399, 400, "", "")
	if e := advanceAudit(p, "a1a2", seen, &half); e != nil {
		t.Fatal(e)
	}
	terminalPair(t, p, seen, half, 400, 400, "1/2-1/2", "fifty-move")
	p = testPosition(t, "7k/p7/8/8/8/8/8/R6K w - - 99 1")
	seen = map[string]int{PositionKey(p): 1}
	half = 99
	if e := advanceAudit(p, "a1a7", seen, &half); e != nil {
		t.Fatal(e)
	}
	if half != 0 {
		t.Fatal("capture did not reset halfmove")
	}
	terminalPair(t, p, seen, half, 1, 400, "", "")
	p = testPosition(t, "7k/p7/8/8/8/8/P7/7K w - - 99 1")
	seen = map[string]int{PositionKey(p): 1}
	half = 99
	if e := advanceAudit(p, "a2a3", seen, &half); e != nil {
		t.Fatal(e)
	}
	if half != 0 {
		t.Fatal("pawn move did not reset halfmove")
	}
}
func TestInsufficientMaterialPolicy(t *testing.T) {
	cases := []struct {
		fen  string
		dead bool
	}{
		{"7k/8/8/8/8/8/8/7K w - - 0 1", true},
		{"7k/8/8/8/8/8/8/B6K w - - 0 1", true},
		{"7k/8/8/8/8/8/8/N6K w - - 0 1", true},
		{"7k/8/8/8/8/4b3/8/B6K w - - 0 1", true},
		{"7k/8/8/8/8/3b4/8/B6K w - - 0 1", false},
		{"7k/8/8/8/8/8/8/NN5K w - - 0 1", false},
		{"7k/8/8/8/8/8/P7/7K w - - 0 1", false},
	}
	for _, tc := range cases {
		p := testPosition(t, tc.fen)
		if Insufficient(p) != tc.dead || auditDead(p) != tc.dead {
			t.Fatalf("material policy %s", tc.fen)
		}
		seen := map[string]int{PositionKey(p): 1}
		if tc.dead {
			terminalPair(t, p, seen, 0, 400, 400, "1/2-1/2", "insufficient-material")
		}
	}
}
func TestRepetitionKeyLegalEPAndRights(t *testing.T) {
	// e5xd6 exposes White's king e1 to Black's rook e8: EP is not legal.
	a := testPosition(t, "k3r3/8/8/3pP3/8/8/8/4K3 w - d6 0 1")
	b := testPosition(t, "k3r3/8/8/3pP3/8/8/8/4K3 w - - 0 1")
	if PositionKey(a) != PositionKey(b) {
		t.Fatal("pinned EP distinguished repetition")
	}
	a = testPosition(t, "k7/8/8/3pP3/8/8/8/4K3 w - d6 0 1")
	b = testPosition(t, "k7/8/8/3pP3/8/8/8/4K3 w - - 0 1")
	if PositionKey(a) == PositionKey(b) {
		t.Fatal("legal EP lost from repetition key")
	}
	a = testPosition(t, StartFEN)
	b = testPosition(t, strings.Replace(StartFEN, "KQkq", "-", 1))
	if PositionKey(a) == PositionKey(b) {
		t.Fatal("castling rights lost")
	}
	b = testPosition(t, strings.Replace(StartFEN, " w ", " b ", 1))
	if PositionKey(a) == PositionKey(b) {
		t.Fatal("STM lost")
	}
}
func TestLoadOpeningSliceAndNegativeFixtures(t *testing.T) {
	cases := []struct {
		name, bytes string
		pass        bool
	}{
		{"first-80", "# owned fixture\n\n" + strings.Repeat("e2e4 e7e5\r\n", 80) + "bad trailing line\n", true},
		{"79-lines", strings.Repeat("e2e4 e7e5\n", 79), false},
		{"illegal-opening", strings.Repeat("e2e5\n", 80), false},
		{"fen-only", strings.Repeat(StartFEN+"\n", 80), false},
		{"repetition-terminal", strings.Repeat("g1f3 g8f6 f3g1 f6g8 g1f3 g8f6 f3g1 f6g8\n", 80), false},
		{"past-checkmate", strings.Repeat("f2f3 e7e5 g2g4 d8h4 a2a3\n", 80), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "openings")
			if e := os.WriteFile(path, []byte(tc.bytes), 0600); e != nil {
				t.Fatal(e)
			}
			ops, slice, e := LoadOpenings(path)
			if (e == nil) != tc.pass {
				t.Fatalf("pass=%v error=%v", tc.pass, e)
			}
			if tc.pass {
				if len(ops) != 80 || ops[79].Index != 79 || ops[0].FEN != StartFEN || string(slice) != strings.Repeat("e2e4 e7e5\n", 80) {
					t.Fatal("incorrect frozen slice")
				}
			}
		})
	}
}
