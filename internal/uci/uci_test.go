package uci

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestStartWithOptionsBeforeWarmup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test subprocess uses a POSIX shell")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "fake-uci")
	receipt := filepath.Join(dir, "commands")
	t.Setenv("NGN_UCI_TEST_LOG", receipt)
	script := `#!/bin/sh
configured=no
while IFS= read -r line; do
  printf '%s\n' "$line" >> "$NGN_UCI_TEST_LOG"
  case "$line" in
    uci) printf 'uciok\n' ;;
    'setoption name UseNNUE value true') configured=yes ;;
    isready)
      if [ "$configured" = yes ]; then printf 'readyok\n';
      else printf 'info string error missing options\n'; fi ;;
    'go movetime 200') printf 'bestmove e2e4\n' ;;
    quit) exit 0 ;;
  esac
done
`
	if err := os.WriteFile(path, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	options := []string{"setoption name EvalFile value /net dir/pilot.nnue", "setoption name UseNNUE value true"}
	e := StartWithOptions(path, "test", false, options)
	if e == nil {
		t.Fatal("configured fake engine failed startup")
	}
	defer Stop(e)
	commands, err := os.ReadFile(receipt)
	if err != nil {
		t.Fatal(err)
	}
	want := "uci\n" + strings.Join(options, "\n") + "\nisready\nposition startpos\ngo movetime 200\n"
	if string(commands) != want {
		t.Fatalf("startup commands = %q, want %q", commands, want)
	}
}

func TestReadyRejectsConfigurationError(t *testing.T) {
	for _, tc := range []struct {
		output string
		want   bool
	}{
		{"info string option set: UseNNUE = true\nreadyok\n", true},
		{"info string error eval option: CRC mismatch; using hce\nreadyok\n", false},
	} {
		e := &Engine{stdout: bufio.NewScanner(strings.NewReader(tc.output))}
		if got := WaitFor(e, "readyok", time.Second); got != tc.want {
			t.Fatalf("WaitFor(%q) = %v, want %v", tc.output, got, tc.want)
		}
	}
}

func TestParseScore(t *testing.T) {
	cases := []struct {
		line   string
		want   int
		wantOK bool
	}{
		{"info depth 10 seldepth 12 score cp 34 nodes 1000 pv e2e4", 34, true},
		{"info depth 8 score cp -250 lowerbound pv d2d4", -250, true},
		{"info depth 3 score mate 2 pv f1c4", mateAdjScore, true},
		{"info depth 3 score mate -1 pv e8g8", -mateAdjScore, true},
		{"info string using 4 threads", 0, false},
		{"info depth 5 nodes 100 nps 2000", 0, false},
		{"bestmove e2e4 ponder e7e5", 0, false},
	}
	for _, c := range cases {
		got, ok := parseScore(c.line)
		if ok != c.wantOK || (ok && got != c.want) {
			t.Errorf("parseScore(%q) = (%d,%v), want (%d,%v)", c.line, got, ok, c.want, c.wantOK)
		}
	}
}

func TestAdjWinTwoSided(t *testing.T) {
	cfg := AdjConfig{ResignScore: 600, ResignPlies: 4}
	var a adjState
	for i, s := range []int{700, 720, 690} { // three decisive plies: not enough yet
		if fired, _, _ := a.record(cfg, s, i+1); fired {
			t.Fatalf("fired too early at ply %d", i+1)
		}
	}
	fired, whiteWins, isDraw := a.record(cfg, 800, 4)
	if !fired || !whiteWins || isDraw {
		t.Fatalf("want white win at ply 4; got fired=%v whiteWins=%v draw=%v", fired, whiteWins, isDraw)
	}
}

func TestAdjWinResetsOnSignFlip(t *testing.T) {
	cfg := AdjConfig{ResignScore: 600, ResignPlies: 4}
	var a adjState
	a.record(cfg, 700, 1)  // W1
	a.record(cfg, 700, 2)  // W2
	a.record(cfg, -700, 3) // flip -> B1
	if fired, _, _ := a.record(cfg, -700, 4); fired {
		t.Fatal("should not fire at B2 (streak restarted on the sign flip)")
	}
	if fired, _, _ := a.record(cfg, -700, 5); fired {
		t.Fatal("should not fire at B3")
	}
	fired, whiteWins, _ := a.record(cfg, -700, 6) // B4
	if !fired || whiteWins {
		t.Fatalf("want black win at B4; fired=%v whiteWins=%v", fired, whiteWins)
	}
}

func TestAdjDrawPastFloor(t *testing.T) {
	cfg := AdjConfig{DrawScore: 20, DrawPlies: 8, DrawMinPlies: 80}
	var a adjState
	for ply := 70; ply < 78; ply++ { // eight near-zero plies, but before the floor
		if fired, _, _ := a.record(cfg, 5, ply); fired {
			t.Fatalf("draw fired before floor at ply %d", ply)
		}
	}
	if fired, _, _ := a.record(cfg, 300, 78); fired { // decisive ply resets the streak
		t.Fatal("unexpected fire on a decisive ply")
	}
	var fired, isDraw bool
	for ply := 81; ply <= 88; ply++ { // fresh 8-ply near-zero streak past the floor
		fired, _, isDraw = a.record(cfg, -10, ply)
	}
	if !fired || !isDraw {
		t.Fatalf("want draw at ply 88; fired=%v isDraw=%v", fired, isDraw)
	}
}

func TestAdjZeroValueDisabled(t *testing.T) {
	var cfg AdjConfig // zero value must be fully off for every non-sprt caller
	var a adjState
	for ply := 1; ply <= 300; ply++ {
		s := 0
		if ply%2 == 0 {
			s = 5000 // wildly decisive, alternating with exact 0
		}
		if fired, _, _ := a.record(cfg, s, ply); fired {
			t.Fatalf("zero-value AdjConfig must never adjudicate (fired at ply %d)", ply)
		}
	}
}
