package engine

import (
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

// These tests LOCK the three texel-tuner data-pipeline fixes (#21) so they cannot
// silently regress: the prior re-tune overfit (a 3x strength regression) because
// the data pipeline trained on tactical noise and leaked positions across the
// train/test split. If any of these fail, the tuner is unsafe to run again.

// TestIsQuietPositionFiltersTactics locks defect #1 (quiet filter): a position with
// a pending tactic must be rejected; a calm one accepted; an in-check one rejected.
func TestIsQuietPositionFiltersTactics(t *testing.T) {
	savedTT := defaultSearchEngine
	defaultSearchEngine, _ = NewSearchEngineWithHash(DEFAULT_CACHE_SIZE)
	ClearStop()
	defer func() { defaultSearchEngine = savedTT }()

	cases := []struct {
		name string
		fen  string
		want bool
	}{
		{"startpos is quiet", "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1", true},
		{"bare-king endgame is quiet", "7k/8/8/8/8/8/8/K7 w - - 0 1", true},
		{"hanging queen is NOT quiet", "4k3/8/8/3q4/4P3/8/8/4K3 w - - 0 1", false}, // exd5 wins the queen
		{"in check is NOT quiet", "4k3/8/8/8/8/8/4r3/4K3 w - - 0 1", false},        // Re2+ on the white king
		{"white quiet promotion is NOT quiet", "6k1/7P/8/8/8/8/8/6K1 w - - 0 1", false},
		{"black quiet promotion is NOT quiet", "7k/8/8/8/8/8/p7/6K1 b - - 0 1", false},
		{"blocked promotion is quiet", "6k1/7p/7P/8/8/8/8/6K1 w - - 0 1", true},
		// Moving g7-g8 opens the h6-g7-f8 diagonal onto White's king, so the
		// geometrically available promotion is illegal.
		{"pinned promotion is quiet", "k4K2/6P1/7b/8/8/8/8/8 w - - 0 1", true},
	}
	for _, c := range cases {
		pos, err := ParseFEN(c.fen)
		if err != nil {
			t.Fatalf("%s: bad FEN %q: %v", c.name, c.fen, err)
		}
		before := EvaluateForPlayer(&pos.Board, pos.Turn())
		if got := IsQuietPosition(pos); got != c.want {
			t.Errorf("%s: IsQuietPosition=%v, want %v", c.name, got, c.want)
		}
		// The filter must not corrupt the position (qsearch make/unmake balanced).
		if after := EvaluateForPlayer(&pos.Board, pos.Turn()); after != before {
			t.Errorf("%s: IsQuietPosition mutated the position (eval %d -> %d)", c.name, before, after)
		}
	}
}

// BenchmarkIsQuietPositionRandomWalkCorpus measures the data-filter cost on a
// deterministic, varied 1,600-position corpus instead of a synthetic single FEN.
func BenchmarkIsQuietPositionRandomWalkCorpus(b *testing.B) {
	rng := rand.New(rand.NewSource(2600))
	positions := make([]*Position, 0, 1600)
	for game := 0; game < 16; game++ {
		pos, err := ParseFEN("rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1")
		if err != nil {
			b.Fatal(err)
		}
		for ply := 0; ply < 100; ply++ {
			positions = append(positions, pos.Copy())
			moves := GenerateLegalMoves(pos)
			if len(moves) == 0 {
				break
			}
			pos.GameMakeMove(moves[rng.Intn(len(moves))])
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsQuietPosition(positions[i%len(positions)])
	}
}

// TestIsQuietPositionIsolatedFromSearchState proves that data generation cannot
// change classification by inheriting a search's clock damping, TT/correction data,
// stop state, or game history. It also locks caller-state preservation.
func TestIsQuietPositionIsolatedFromSearchState(t *testing.T) {
	savedTT := defaultSearchEngine
	savedStopped := IsStopRequested()
	defer func() {
		defaultSearchEngine = savedTT
		if savedStopped {
			RequestStop()
		} else {
			ClearStop()
		}
	}()

	pos, err := ParseFEN("4k3/8/8/8/8/8/4P3/4K3 w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	pos.Positions = map[uint64]int{pos.Hash(): 3}
	before := pos.Copy()
	defaultSearchEngine, _ = NewSearchEngineWithHash(DEFAULT_CACHE_SIZE)
	defaultSearchEngine.TTStore(pos.Hash(), EmptyMove, 1234, 0, Exact, false)
	corrIndex := pawnCorrectionIndex(pos.Board.pieces[WhitePawn], pos.Board.pieces[BlackPawn])
	oldCorrection := defaultSearchEngine.worker.history.pawnCorrectionHistory[pos.Turn()][corrIndex]
	defaultSearchEngine.worker.history.pawnCorrectionHistory[pos.Turn()][corrIndex] = 8192
	defer func() {
		defaultSearchEngine.worker.history.pawnCorrectionHistory[pos.Turn()][corrIndex] = oldCorrection
	}()
	RequestStop()

	if got := IsQuietPosition(pos); !got {
		t.Fatal("quiet position changed classification under populated search state")
	}
	if !IsStopRequested() {
		t.Fatal("IsQuietPosition cleared the caller's stop request")
	}
	if !PositionEqual(pos, before) || pos.Positions[pos.Hash()] != before.Positions[before.Hash()] {
		t.Fatal("IsQuietPosition mutated caller position or game history")
	}
	for _, clock := range []uint8{0, 20, 99} {
		probe := pos.Copy()
		probe.HalfMoveClock = clock
		if got := IsQuietPosition(probe); !got {
			t.Fatalf("halfmove clock %d changed quiet classification", clock)
		}
	}
}

// TestIsQuietPositionRejectsHorizonExchange locks the conservative horizon rule.
// The forced alternating capture sequence reaches the sixth q-ply; treating that
// leaf's static evaluation as final would admit an unresolved exchange as quiet.
func TestIsQuietPositionRejectsHorizonExchange(t *testing.T) {
	pos, err := ParseFEN("7k/8/8/8/q7/rb6/rQ6/RBN4K w - - 0 1")
	if err != nil {
		t.Fatal(err)
	}
	if _, resolved := texelQuietSearch(pos.Copy(), -INFINITY, INFINITY, 0); resolved {
		t.Fatal("six-ply capture horizon was treated as a resolved quiet search")
	}
	if IsQuietPosition(pos) {
		t.Fatal("unresolved horizon exchange was admitted as quiet")
	}
}

// TestLoadTexelSamplesDedupes locks defect #2 (dedup): repeated positions collapse
// to one sample (counted), and side-to-move is part of the key (same board, other
// side = a distinct position, kept).
func TestLoadTexelSamplesDedupes(t *testing.T) {
	const (
		a  = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
		b  = "7k/8/8/8/8/8/8/K7 w - - 0 1"
		bb = "7k/8/8/8/8/8/8/K7 b - - 0 1" // same board as b, black to move = distinct
	)
	lines := []string{
		a + " 0.5",
		b + " 1.0",
		a + " 1.0", // duplicate of a (different result, still same position)
		b + " 0.5", // duplicate of b
		bb + " 0.0",
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "data.txt")
	var buf []byte
	for _, l := range lines {
		buf = append(buf, l...)
		buf = append(buf, '\n')
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}

	samples, nDup, err := LoadTexelSamples(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(samples) != 3 { // a, b, bb
		t.Errorf("got %d unique samples, want 3", len(samples))
	}
	if nDup != 2 { // the second a and the second b
		t.Errorf("got nDup=%d, want 2", nDup)
	}
}

// TestSplitTrainTestPartitions locks defect #3 (leak-free split): the two halves
// partition the input with no shared element, at the requested fraction.
func TestSplitTrainTestPartitions(t *testing.T) {
	const n = 100
	samples := make([]TexelSample, n)
	for i := range samples {
		samples[i].Result = float64(i) // unique tag per element to detect leakage
	}
	train, test := SplitTrainTest(samples, 0.2, 42)

	if len(train)+len(test) != n {
		t.Errorf("lengths sum to %d, want %d", len(train)+len(test), n)
	}
	if len(test) != 20 {
		t.Errorf("test size %d, want 20 (20%% of %d)", len(test), n)
	}
	inTrain := make(map[float64]bool, len(train))
	for _, s := range train {
		inTrain[s.Result] = true
	}
	seen := make(map[float64]bool, n)
	for _, s := range append(append([]TexelSample{}, train...), test...) {
		seen[s.Result] = true
	}
	for _, s := range test {
		if inTrain[s.Result] {
			t.Errorf("element %v leaked into both train and test", s.Result)
		}
	}
	if len(seen) != n {
		t.Errorf("union covers %d distinct elements, want %d", len(seen), n)
	}
}
