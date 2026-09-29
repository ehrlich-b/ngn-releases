package engine

import (
	"math"
	"testing"
	"time"
)

func TestTimeManagerCreation(t *testing.T) {
	tm := NewTimeManager()

	if tm == nil {
		t.Fatal("TimeManager should not be nil")
	}

	if tm.emergencyTime != 100*time.Millisecond {
		t.Errorf("Expected emergency time 100ms, got %v", tm.emergencyTime)
	}

	if tm.moveOverhead != 50*time.Millisecond {
		t.Errorf("Expected move overhead 50ms, got %v", tm.moveOverhead)
	}
}

func TestFixedDepthTimeControl(t *testing.T) {
	tm := NewTimeManager()
	params := SearchParams{Depth: 6}

	tm.SetTimeControl(params, true)

	if tm.timeControl != FixedDepth {
		t.Errorf("Expected FixedDepth, got %v", tm.timeControl)
	}

	// Should not stop for fixed depth (time doesn't matter)
	if tm.ShouldStopSearch(5) {
		t.Error("Fixed depth search should not stop due to time")
	}
}

func TestInfiniteTimeControl(t *testing.T) {
	tm := NewTimeManager()
	params := SearchParams{Infinite: true}

	tm.SetTimeControl(params, true)

	if tm.timeControl != Infinite {
		t.Errorf("Expected Infinite, got %v", tm.timeControl)
	}

	// Should never stop for infinite search
	if tm.ShouldStopSearch(10) {
		t.Error("Infinite search should never stop due to time")
	}
}

func TestTimePerMoveControl(t *testing.T) {
	tm := NewTimeManager()
	params := SearchParams{MoveTime: 5000} // 5 seconds

	tm.SetTimeControl(params, true)

	if tm.timeControl != TimePerMove {
		t.Errorf("Expected TimePerMove, got %v", tm.timeControl)
	}

	expectedTime := 5000*time.Millisecond - tm.moveOverhead
	if tm.allocatedTime != expectedTime {
		t.Errorf("Expected allocated time %v, got %v", expectedTime, tm.allocatedTime)
	}
}

func TestTournamentTimeControl(t *testing.T) {
	tm := NewTimeManager()
	params := SearchParams{
		WhiteTime: 300000, // 5 minutes
		WhiteInc:  5000,   // 5 second increment
		MovesToGo: 0,      // No moves to go specified
	}

	tm.SetTimeControl(params, true)

	if tm.timeControl != Tournament {
		t.Errorf("Expected Tournament, got %v", tm.timeControl)
	}

	if tm.baseTime != 300*time.Second {
		t.Errorf("Expected base time 300s, got %v", tm.baseTime)
	}

	if tm.increment != 5*time.Second {
		t.Errorf("Expected increment 5s, got %v", tm.increment)
	}
}

func TestBlackTimeControl(t *testing.T) {
	tm := NewTimeManager()
	params := SearchParams{
		WhiteTime: 300000, // 5 minutes
		BlackTime: 250000, // 4 minutes 10 seconds
		WhiteInc:  5000,   // 5 second increment
		BlackInc:  3000,   // 3 second increment
	}

	// Test for black player
	tm.SetTimeControl(params, false)

	if tm.baseTime != 250*time.Second {
		t.Errorf("Expected base time 250s for black, got %v", tm.baseTime)
	}

	if tm.increment != 3*time.Second {
		t.Errorf("Expected increment 3s for black, got %v", tm.increment)
	}
}

func TestMovesToGoTimeControl(t *testing.T) {
	tm := NewTimeManager()
	params := SearchParams{
		WhiteTime: 300000, // 5 minutes
		WhiteInc:  0,      // No increment
		MovesToGo: 30,     // 30 moves to next time control
	}

	tm.SetTimeControl(params, true)

	// Soft target ~ the even share of the bank over the moves to go (300s/30 = 10s):
	// the sustainable per-move budget. Allow a band around it.
	evenShare := tm.baseTime / time.Duration(params.MovesToGo) // 10s
	if tm.softTime < evenShare/2 || tm.softTime > evenShare*6/5 {
		t.Errorf("soft time %v not a sane fraction of the even share %v", tm.softTime, evenShare)
	}

	// Hard ceiling must never exceed 30%% of the bank — the anti-flag guarantee.
	if tm.hardTime > tm.baseTime*3/10 {
		t.Errorf("hard time %v exceeds 30%% of bank %v (flag risk)", tm.hardTime, tm.baseTime)
	}

	// And still allocate a reasonable amount (at least 3 seconds).
	if tm.allocatedTime < 3*time.Second {
		t.Errorf("Time allocation too low, got %v", tm.allocatedTime)
	}
}

func TestTournamentSoftStop(t *testing.T) {
	tm := NewTimeManager()

	// ReportCompletedIteration still TRACKS decision stability (a changed best move or
	// a broken window resets it); the counter is kept for diagnostics even though the
	// soft budget no longer shrinks on it.
	tm.ReportCompletedIteration(true, true, 0.5) // best move changed
	if tm.stableIters != 0 {
		t.Fatalf("a changed best move must reset stability, got %d", tm.stableIters)
	}
	tm.ReportCompletedIteration(false, false, 0.5) // window broke (re-search)
	if tm.stableIters != 0 {
		t.Fatalf("a window break must reset stability, got %d", tm.stableIters)
	}
	for i := 0; i < 6; i++ {
		tm.ReportCompletedIteration(false, true, 0.5) // settled iterations
	}
	if tm.stableIters != 6 {
		t.Fatalf("six settled iterations should give stability 6, got %d", tm.stableIters)
	}

	// T1b: stableIters now SCALES the soft target via stabilitySoftFactor — a volatile
	// decision (low stableIters) extends it, a settled one (high stableIters) shrinks it
	// and banks the time. This is the symmetric replacement for the removed one-sided
	// "easy move" shrink; only the soft target flexes — the ×1 projection and the hard
	// ceiling do not. The soft stop fires when elapsed+lastIter(=elapsed+500) > soft*factor.
	// factor = max(0.85, 1.30 - 0.10*stableIters): si=0 -> 1.30 (thresh 5200ms),
	// si=3 -> 1.00 (4000ms), si=6 -> 0.85 floor (3400ms).
	tm.timeControl = Tournament
	tm.baseTime = 120 * time.Second
	tm.softTime = 4000 * time.Millisecond
	tm.hardTime = 16000 * time.Millisecond
	tm.lastIterationTime = 500 * time.Millisecond
	// Neutralize the T1e node-effort signal (fraction 0 → nodeEffortFactor 1.0) so this
	// test measures the T1b stability thresholds alone; T1e is covered separately.
	tm.bestMoveNodeFraction = 0
	const depth = 10

	// At elapsed 4200ms (elapsed+lastIter = 4700): a VOLATILE search keeps going
	// (4700 < 5200) while a neutral/settled one stops (4700 > 4000, > 3400). Pre-T1b the
	// soft budget was a flat 4000 so ALL THREE stopped here — the extend side (red->green).
	tm.stableIters = 0
	if tm.shouldStopTournamentSearch(depth, 4200*time.Millisecond) {
		t.Error("volatile search (stableIters=0) must extend past the base soft budget at 4200ms")
	}
	tm.stableIters = 3
	if !tm.shouldStopTournamentSearch(depth, 4200*time.Millisecond) {
		t.Error("neutral search (stableIters=3) must stop at ~the base soft budget at 4200ms")
	}
	tm.stableIters = 6
	if !tm.shouldStopTournamentSearch(depth, 4200*time.Millisecond) {
		t.Error("settled search (stableIters=6) must stop below the base soft budget at 4200ms")
	}

	// At elapsed 3000ms (elapsed+lastIter = 3500): only the SETTLED search stops early
	// (3500 > 3400); volatile and neutral keep searching (3500 < 5200, 3500 < 4000).
	// Pre-T1b all three kept going here (3500 < 4000) — the shrink side (red->green).
	tm.stableIters = 6
	if !tm.shouldStopTournamentSearch(depth, 3000*time.Millisecond) {
		t.Error("settled search (stableIters=6) must bank time by stopping early at 3000ms")
	}
	tm.stableIters = 3
	if tm.shouldStopTournamentSearch(depth, 3000*time.Millisecond) {
		t.Error("neutral search (stableIters=3) must not stop early at 3000ms")
	}
	tm.stableIters = 0
	if tm.shouldStopTournamentSearch(depth, 3000*time.Millisecond) {
		t.Error("volatile search (stableIters=0) must not stop early at 3000ms")
	}

	// The hard ceiling must NOT move with stability — the anti-flag guarantee. Even a
	// maximally-volatile search (max extend) stops at the hard ceiling.
	tm.stableIters = 0
	if !tm.shouldStopTournamentSearch(depth, tm.hardTime) {
		t.Error("hard ceiling must still stop the search regardless of stability")
	}
}

// TestNodeEffortFactor covers the T1e node-effort soft-target multiplier: default
// 1.0 with no data, a neutral crossover, monotone-decreasing in the node fraction,
// and clamped into [nodeEffortMin, nodeEffortMax].
func TestNodeEffortFactor(t *testing.T) {
	tm := NewTimeManager()

	// No completed iteration yet (fraction 0) → neutral 1.0.
	if f := tm.nodeEffortFactor(); f != 1.0 {
		t.Errorf("no-data node-effort factor must default to 1.0, got %v", f)
	}

	// A best move that owns the whole tree (fraction 1.0) → maximum shrink (min).
	tm.bestMoveNodeFraction = 1.0
	if f := tm.nodeEffortFactor(); f != nodeEffortMin {
		t.Errorf("fraction 1.0 must give the min factor %v, got %v", nodeEffortMin, f)
	}

	// A best move that owns almost none of the tree → maximum extend (max).
	tm.bestMoveNodeFraction = 0.0001
	if f := tm.nodeEffortFactor(); f <= 1.0 || f > nodeEffortMax+1e-9 {
		t.Errorf("a tiny fraction must extend up toward the max %v, got %v", nodeEffortMax, f)
	}

	// Crossover: with these constants the factor passes through 1.0 at fraction 0.625.
	tm.bestMoveNodeFraction = 0.625
	if f := tm.nodeEffortFactor(); math.Abs(f-1.0) > 1e-9 {
		t.Errorf("fraction 0.625 must be the neutral 1.0 crossover, got %v", f)
	}

	// Monotone non-increasing across the (0,1] fraction range, always clamped. Seed
	// above the max so the first real sample passes; fraction 0 is the no-data
	// sentinel (1.0), not part of the continuous map, so it is excluded here.
	prev := nodeEffortMax + 1.0
	for frac := 0.05; frac <= 1.0+1e-9; frac += 0.05 {
		f := tm.factorAt(frac)
		if f < nodeEffortMin-1e-9 || f > nodeEffortMax+1e-9 {
			t.Errorf("factor at fraction %.2f = %v outside clamp [%v,%v]", frac, f, nodeEffortMin, nodeEffortMax)
		}
		if f > prev+1e-9 {
			t.Errorf("factor must be monotone non-increasing in fraction: %.2f gave %v > prev %v", frac, f, prev)
		}
		prev = f
	}

	// Defensive: a fraction above 1.0 is clamped to the min, not extrapolated below it.
	tm.bestMoveNodeFraction = 1.5
	if f := tm.nodeEffortFactor(); f != nodeEffortMin {
		t.Errorf("fraction > 1 must clamp to the min %v, got %v", nodeEffortMin, f)
	}
}

// factorAt is a test helper: the node-effort factor for a given fraction.
func (tm *TimeManager) factorAt(frac float64) float64 {
	tm.bestMoveNodeFraction = frac
	return tm.nodeEffortFactor()
}

// TestComposedSoftFactorBound verifies the T1b×T1e composed soft-target multiplier is
// bounded to [softComposedMin, softComposedMax] so the two correlated signals cannot
// double-count. The soft stop with lastIterationTime==0 fires exactly at elapsed >=
// softTime*composed, so the fire threshold reveals the composed factor.
func TestComposedSoftFactorBound(t *testing.T) {
	tm := NewTimeManager()
	tm.timeControl = Tournament
	tm.baseTime = 120 * time.Second
	tm.hardTime = 60 * time.Second
	tm.softTime = 1000 * time.Millisecond
	tm.lastIterationTime = 0 // fire threshold is exactly softTime*composed
	const depth = 10

	// Both signals maximally volatile: stability 1.30 × node-effort ~1.25 = ~1.625 raw,
	// which MUST be capped to softComposedMax (1.40) → threshold 1400ms, not ~1625ms.
	tm.stableIters = 0
	tm.bestMoveNodeFraction = 0.0001
	if tm.shouldStopTournamentSearch(depth, 1350*time.Millisecond) {
		t.Error("composed soft target must extend to ~1400ms; stopped early at 1350ms")
	}
	if !tm.shouldStopTournamentSearch(depth, 1450*time.Millisecond) {
		t.Error("composed soft target must be capped at ~1400ms; did not stop at 1450ms (uncapped ~1625ms)")
	}

	// Both signals maximally confident: stability 0.85 × node-effort 0.85 = ~0.7225,
	// at/above the softComposedMin floor (0.72) → threshold ~722ms.
	tm.stableIters = 10 // floors stabilitySoftFactor at 0.85
	tm.bestMoveNodeFraction = 1.0
	if tm.shouldStopTournamentSearch(depth, 700*time.Millisecond) {
		t.Error("both-confident soft target must reach ~722ms; stopped early at 700ms")
	}
	if !tm.shouldStopTournamentSearch(depth, 740*time.Millisecond) {
		t.Error("both-confident soft target must be ~722ms; did not stop at 740ms")
	}
}

// TestNodeEffortPlumbing confirms the per-root-move node attribution is wired end to
// end: after a real short search the best move's node fraction is positive and does
// not exceed the iteration total (bestMoveNodes/totalNodes in (0,1]) — the sum
// identity holds by construction (info.Nodes only advances inside the root move loop,
// so the total is the exact sum of the per-move deltas).
func TestNodeEffortPlumbing(t *testing.T) {
	tm := NewTimeManager()
	pos := newUCIStartingPosition()
	info := SearchIterativeDeepening(pos, 6, tm)
	if info.BestMove == EmptyMove {
		t.Fatal("search returned no best move")
	}
	frac := tm.bestMoveNodeFraction
	if frac <= 0.0 {
		t.Errorf("best-move node fraction must be positive after a real search, got %v", frac)
	}
	if frac > 1.0+1e-9 {
		t.Errorf("best-move node fraction must not exceed 1.0 (best move nodes <= total), got %v", frac)
	}
}

func TestGamePhaseDetection(t *testing.T) {
	tm := NewTimeManager()

	// Test opening position
	pos := newUCIStartingPosition()
	tm.movesPlayed = 5
	phase := tm.determineGamePhase(pos)
	if phase != Opening {
		t.Errorf("Expected Opening phase, got %v", phase)
	}

	// Test endgame position - create a position with minimal material
	pos = &Position{}
	pos.Board = Bitboard{}
	pos.Board.UpdateSquare(E1, WhiteKing, NoPiece)
	pos.Board.UpdateSquare(E8, BlackKing, NoPiece)
	pos.Board.UpdateSquare(A7, WhitePawn, NoPiece)
	pos.Board.UpdateSquare(H7, BlackPawn, NoPiece)

	tm.movesPlayed = 50
	phase = tm.determineGamePhase(pos)
	if phase != Endgame {
		t.Errorf("Expected Endgame phase, got %v", phase)
	}
}

func TestComplexityCalculation(t *testing.T) {
	tm := NewTimeManager()

	// Test starting position complexity
	pos := newUCIStartingPosition()
	complexity := tm.calculatePositionComplexity(pos)

	if complexity < 0.3 || complexity > 0.8 {
		t.Errorf("Starting position complexity should be moderate, got %v", complexity)
	}

	// Test empty position
	pos = &Position{}
	pos.Board = Bitboard{}
	complexity = tm.calculatePositionComplexity(pos)

	if complexity != 0.0 {
		t.Errorf("Empty position should have zero complexity, got %v", complexity)
	}
}

func TestEmergencyTimeDetection(t *testing.T) {
	tm := NewTimeManager()
	params := SearchParams{WhiteTime: 1000} // 1 second

	tm.SetTimeControl(params, true)

	// Initially should not be emergency time
	if tm.IsEmergencyTime() {
		t.Error("Should not be emergency time initially")
	}

	// Simulate time passing
	time.Sleep(500 * time.Millisecond)
	tm.startTime = tm.startTime.Add(-600 * time.Millisecond) // Fake elapsed time

	// Now should be emergency time
	if !tm.IsEmergencyTime() {
		t.Error("Should be emergency time with little time remaining")
	}
}

func TestTimeAllocationSafety(t *testing.T) {
	tm := NewTimeManager()
	params := SearchParams{WhiteTime: 10000} // 10 seconds

	tm.SetTimeControl(params, true)

	// Should never allocate more than 1/3 of total time
	maxAllowed := tm.baseTime / 3
	if tm.allocatedTime > maxAllowed {
		t.Errorf("Allocated time %v exceeds maximum allowed %v", tm.allocatedTime, maxAllowed)
	}

	// Should always keep emergency reserve
	emergencyReserve := tm.emergencyTime
	if tm.allocatedTime > tm.baseTime-emergencyReserve {
		t.Errorf("Allocated time %v doesn't leave emergency reserve %v", tm.allocatedTime, emergencyReserve)
	}
}

func TestTimeControlStringRepresentation(t *testing.T) {
	tm := NewTimeManager()

	testCases := []struct {
		params   SearchParams
		expected string
	}{
		{SearchParams{Depth: 6}, "Fixed Depth"},
		{SearchParams{Infinite: true}, "Infinite"},
		{SearchParams{MoveTime: 5000}, "Time Per Move"},
		{SearchParams{WhiteTime: 300000}, "Tournament"},
	}

	for _, tc := range testCases {
		tm.SetTimeControl(tc.params, true)
		result := tm.GetTimeControlString()
		if result != tc.expected {
			t.Errorf("Expected time control string %s, got %s", tc.expected, result)
		}
	}
}

func TestGameStateUpdate(t *testing.T) {
	tm := NewTimeManager()
	pos := newUCIStartingPosition()

	tm.UpdateGameState(10, 50, 30, pos) // 10 plies; score dropped from 50 to 30

	if tm.movesPlayed != 5 { // D2: arg is a ply count, halved to full-moves (time.go:334)
		t.Errorf("Expected moves played 5, got %d", tm.movesPlayed)
	}

	if tm.lastScoreChange != -20 {
		t.Errorf("Expected score change -20, got %d", tm.lastScoreChange)
	}

	if tm.gamePhase != Opening {
		t.Errorf("Expected Opening phase, got %v", tm.gamePhase)
	}
}

func BenchmarkTimeAllocation(b *testing.B) {
	tm := NewTimeManager()
	params := SearchParams{WhiteTime: 300000, WhiteInc: 5000}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tm.SetTimeControl(params, true)
	}
}

func BenchmarkShouldStopSearch(b *testing.B) {
	tm := NewTimeManager()
	params := SearchParams{MoveTime: 5000}
	tm.SetTimeControl(params, true)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tm.ShouldStopSearch(6)
	}
}

func TestTimeManagerSetMoveOverheadChecksBounds(t *testing.T) {
	tm := NewTimeManager()
	if tm.moveOverhead != 50*time.Millisecond {
		t.Fatalf("standalone default overhead=%v, want 50ms", tm.moveOverhead)
	}
	for _, milliseconds := range []int{0, 5000} {
		if err := tm.SetMoveOverhead(milliseconds); err != nil {
			t.Fatalf("SetMoveOverhead(%d): %v", milliseconds, err)
		}
		if got := tm.moveOverhead; got != time.Duration(milliseconds)*time.Millisecond {
			t.Fatalf("SetMoveOverhead(%d) stored %v", milliseconds, got)
		}
	}
	before := tm.moveOverhead
	for _, milliseconds := range []int{-1, 5001} {
		if err := tm.SetMoveOverhead(milliseconds); err == nil {
			t.Fatalf("SetMoveOverhead(%d) succeeded outside UCI range", milliseconds)
		}
		if tm.moveOverhead != before {
			t.Fatalf("failed SetMoveOverhead(%d) changed overhead from %v to %v", milliseconds, before, tm.moveOverhead)
		}
	}
}
