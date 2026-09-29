package engine

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

type ablationPos struct {
	fen string
	bm  string // best move in UCI (e.g. "d5f6")
}

// loadAblationEPD parses an EPD tactical suite. Lines look like:
//
//	<FEN> bm <UCI-move>; <comment>
func loadAblationEPD(t *testing.T, path string, limit int) []ablationPos {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("no EPD suite at %s: %v", path, err)
	}
	var out []ablationPos
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, " bm ")
		if idx < 0 {
			continue
		}
		fen := strings.TrimSpace(line[:idx])
		move := strings.TrimSpace(line[idx+4:])
		if c := strings.IndexAny(move, "; "); c >= 0 {
			move = move[:c]
		}
		if fen != "" && move != "" {
			out = append(out, ablationPos{fen, move})
		}
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// runAblationSuite searches every position to a fixed depth (deterministic: nil
// time manager + fresh global state per position) and returns solved count and
// total nodes.
func runAblationSuite(positions []ablationPos, depth int) (solved int, nodes uint64) {
	for _, p := range positions {
		defaultSearchEngine, _ = NewSearchEngineWithHash(DEFAULT_CACHE_SIZE)
		ClearHistoryTable()
		ClearKillerMoves()
		ClearCounterMoves()
		SetLastMovePlayed(EmptyMove)

		pos, err := ParseFEN(p.fen)
		if err != nil {
			continue
		}
		info := SearchIterativeDeepening(pos, depth, nil)
		if info.BestMove.ToString() == p.bm {
			solved++
		}
		nodes += info.Nodes
	}
	return
}

// TestPruningAblation is the search-correctness harness: it disables each
// pruning/reduction technique one at a time and re-solves a fixed-depth tactical
// suite. At fixed depth, removing a technique only makes the search MORE
// thorough, so a SOUND technique leaves the solved count unchanged (just more
// nodes). A technique whose removal makes the engine solve MORE positions is
// over-pruning real refutations — a bug, the same class as the +243-ELO
// partial-commit fix. Skipped in -short (it runs a few minutes).
func TestPruningAblation(t *testing.T) {
	if testing.Short() {
		t.Skip("ablation harness runs a few minutes; skipped in -short")
	}
	const depth = 7
	const limit = 120
	positions := loadAblationEPD(t, "../validated_tactical_positions.epd", limit)
	if len(positions) == 0 {
		t.Skip("no positions loaded")
	}

	baseSolved, baseNodes := runAblationSuite(positions, depth)
	t.Logf("baseline (all ON):  %3d/%d solved   %d nodes", baseSolved, len(positions), baseNodes)

	toggles := []struct {
		name string
		ptr  *bool
	}{
		{"no-NullMove", &SearchToggles.NullMove},
		{"no-Futility", &SearchToggles.Futility},
		{"no-RFP", &SearchToggles.RFP},
		{"no-Probcut", &SearchToggles.Probcut},
		{"no-LMP", &SearchToggles.LMP},
		{"no-SEEPrune", &SearchToggles.SEEPrune},
		{"no-LMR", &SearchToggles.LMR},
		{"no-HistPrune", &SearchToggles.HistPrune},
	}
	for _, tg := range toggles {
		*tg.ptr = false
		s, n := runAblationSuite(positions, depth)
		*tg.ptr = true // restore

		delta := s - baseSolved
		note := ""
		if delta > 0 {
			note = fmt.Sprintf("  <-- solves %d MORE with it OFF -> OVER-PRUNING (bug suspect)", delta)
		}
		t.Logf("%-13s %3d/%d solved  (%+d)  %d nodes (%.2fx)%s",
			tg.name, s, len(positions), delta, n, float64(n)/float64(baseNodes), note)
	}
}
