package engine

import (
	"fmt"
	"math"
	"time"
)

// TimeControl represents different types of time controls
type TimeControl int

const (
	FixedDepth TimeControl = iota
	FixedTime
	TimePerMove
	Tournament
	Infinite
)

// searchTerminationReason is an opt-in diagnostic classification of the first
// event that decisively ended a search. It is deliberately unexported: ordinary
// engine/UCI operation neither allocates an observer nor emits observer output.
type searchTerminationReason uint8

const (
	searchTerminationNone searchTerminationReason = iota
	searchTerminationSoftAdmission
	searchTerminationSoftAbort
	searchTerminationHardDeadline
	searchTerminationEmergencyReserve
	searchTerminationExternalStop
	searchTerminationNodeCap
	searchTerminationTerminal
	searchTerminationDepthComplete
)

func (reason searchTerminationReason) String() string {
	switch reason {
	case searchTerminationSoftAdmission:
		return "soft-admission"
	case searchTerminationSoftAbort:
		return "soft-abort"
	case searchTerminationHardDeadline:
		return "hard-deadline"
	case searchTerminationEmergencyReserve:
		return "emergency-reserve"
	case searchTerminationExternalStop:
		return "external-stop"
	case searchTerminationNodeCap:
		return "node-cap"
	case searchTerminationTerminal:
		return "terminal"
	case searchTerminationDepthComplete:
		return "depth-complete"
	default:
		return "none"
	}
}

type searchIterationObservation struct {
	Depth              int
	Duration           time.Duration
	CompletedAtElapsed time.Duration
	AspirationAttempts int
}

// searchTerminationObserver is fixed-capacity so enabling the C0 diagnostic
// adds no production hot-path allocation. The observer is worker-private, just
// like TimeManager. Overflow is explicit rather than silently reallocating.
type searchTerminationObserver struct {
	FirstReason       searchTerminationReason
	FirstDepth        int
	FirstElapsed      time.Duration
	Iterations        [MaximumDepth + 1]searchIterationObservation
	IterationCount    int
	IterationOverflow int
}

func (observer *searchTerminationObserver) reset() {
	if observer != nil {
		*observer = searchTerminationObserver{}
	}
}

type timeCheckSite uint8

const (
	timeCheckActiveSearch timeCheckSite = iota
	timeCheckIterationAdmission
)

// TimeManager handles time allocation and management during search
type TimeManager struct {
	timeControl   TimeControl
	allocatedTime time.Duration // Time allocated for this move (== hardTime for Tournament)
	softTime      time.Duration // Tournament soft budget: target before starting a new ID iteration
	hardTime      time.Duration // Tournament hard ceiling: never exceeded (the anti-flag guarantee)
	startTime     time.Time
	emergencyTime time.Duration // Reserve time for emergencies
	moveOverhead  time.Duration // Network/GUI overhead per move

	// Tournament time control parameters
	baseTime  time.Duration // Base time available
	increment time.Duration // Increment per move
	movesToGo int           // Moves until next time control

	// Game state for time allocation
	gamePhase   GamePhase
	movesPlayed int
	isEndgame   bool

	// Dynamic adjustment factors
	lastScoreChange    int
	positionComplexity float64

	// Internal rate limiting for frequent time checks
	checkCounter  uint64    // Time-check rate limiter (search goroutine only, non-atomic)
	lastTimeCheck time.Time // Last time we actually checked the clock
	shouldStop    bool      // Cached result of last time check

	// iterationStartTime is set by NewIteration at the top of each iterative
	// deepening depth and used by the soft-stop heuristic to estimate the
	// next iteration's cost. Previously each caller passed time.Now() in this
	// slot, which made iterationTime always ~0 and disabled soft-stop.
	iterationStartTime time.Time
	// lastIterationTime is the wall time the most recently COMPLETED iteration
	// took; the soft-stop projects the next iteration as a multiple of it.
	lastIterationTime time.Duration
	// stableIters counts consecutive COMPLETED iterations whose best move was
	// unchanged AND whose aspiration window held on the first attempt — the
	// "the decision is settled" signal read by stabilitySoftFactor. The soft budget
	// shrinks as it climbs (settled → spend less, bank time) and extends while it is
	// low (volatile → spend more), redistributing the bank toward genuinely hard moves.
	// Reset per move and whenever the best move changes or the window breaks (a
	// re-search) — that volatility is the sharpness flag that says "do NOT cut this
	// search short".
	stableIters int
	// bestMoveNodeFraction is the share of the most recently COMPLETED iteration's
	// search nodes that were spent inside the current best move's subtree (T1e),
	// fed by ReportCompletedIteration from the root loop. A high share means the
	// best move dominated the tree (a confident decision → shrink the soft target
	// via nodeEffortFactor); a low share means the search had to spread effort
	// across rival moves (contested → extend it). Zero means "no completed
	// iteration yet", which nodeEffortFactor maps to a neutral 1.0. Reset per move.
	bestMoveNodeFraction float64

	// now is injectable so admission-time accounting and setup expiry are
	// deterministic in lifecycle tests. It is immutable while a manager is in use.
	now func() time.Time

	// observer is nil in ordinary play. Tests and bounded diagnostics may attach
	// one to record the first decisive stop and completed-iteration timings.
	observer *searchTerminationObserver
}

// GamePhase represents the current phase of the game
type GamePhase int

const (
	Opening GamePhase = iota
	Middlegame
	Endgame
)

// NewTimeManager creates a new time manager
func NewTimeManager() *TimeManager {
	return newTimeManager(time.Now)
}

const (
	minMoveOverheadMilliseconds = 0
	maxMoveOverheadMilliseconds = 5000
)

// SetMoveOverhead configures the fixed per-move reserve in milliseconds.
// TimeManager is worker-private and must be idle while it is reconfigured.
func (tm *TimeManager) SetMoveOverhead(milliseconds int) error {
	if milliseconds < minMoveOverheadMilliseconds || milliseconds > maxMoveOverheadMilliseconds {
		return fmt.Errorf("move overhead %dms outside [%d,%d]", milliseconds, minMoveOverheadMilliseconds, maxMoveOverheadMilliseconds)
	}
	tm.moveOverhead = time.Duration(milliseconds) * time.Millisecond
	return nil
}

func newTimeManager(now func() time.Time) *TimeManager {
	if now == nil {
		now = time.Now
	}
	return &TimeManager{
		timeControl:   FixedDepth,
		emergencyTime: 100 * time.Millisecond, // Reserve 100ms
		moveOverhead:  50 * time.Millisecond,  // 50ms overhead
		now:           now,
	}
}

// SetTimeControl configures the time manager for different time controls.
func (tm *TimeManager) SetTimeControl(params SearchParams, isWhite bool) {
	tm.SetTimeControlAt(params, isWhite, tm.nowTime())
}

// SetTimeControlAt configures a move whose clock began when go was received.
// Setup and worker creation therefore consume the same budget as the search.
func (tm *TimeManager) SetTimeControlAt(params SearchParams, isWhite bool, start time.Time) {
	tm.startTime = start
	tm.lastTimeCheck = tm.startTime
	tm.shouldStop = false
	tm.iterationStartTime = time.Time{} // Cleared until NewIteration is called
	tm.lastIterationTime = 0
	tm.stableIters = 0
	tm.bestMoveNodeFraction = 0
	tm.checkCounter = 0
	if tm.observer != nil {
		tm.observer.reset()
	}

	// Determine time control type and allocate time accordingly
	if params.Infinite {
		tm.timeControl = Infinite
		tm.allocatedTime = time.Hour * 24 // Effectively infinite
	} else if params.MoveTime > 0 {
		tm.timeControl = TimePerMove
		tm.allocatedTime = time.Duration(params.MoveTime) * time.Millisecond
	} else if params.Depth > 0 && (params.WhiteTime == 0 && params.BlackTime == 0) {
		tm.timeControl = FixedDepth
		tm.allocatedTime = time.Second * 30 // Max 30 seconds for fixed depth
	} else {
		// Tournament time control
		tm.timeControl = Tournament

		var myTime, myInc int
		if isWhite {
			myTime = params.WhiteTime
			myInc = params.WhiteInc
		} else {
			myTime = params.BlackTime
			myInc = params.BlackInc
		}

		tm.baseTime = time.Duration(myTime) * time.Millisecond
		tm.increment = time.Duration(myInc) * time.Millisecond
		tm.movesToGo = params.MovesToGo

		// Tournament computes its own soft/hard budgets (with built-in safety) and
		// returns; the shared cap/overhead below is for the simpler modes only.
		tm.computeTournamentLimits()
		return
	}

	// Non-tournament modes (TimePerMove/FixedDepth/Infinite): cap and account for
	// overhead. (Tournament returned above with its own remaining-relative caps.)
	tm.allocatedTime = tm.ensureSafeTimeAllocation()

	// Global maximum time limit for any move
	// Keep a 30 second maximum for expediency
	maxTime := time.Second * 30
	if tm.allocatedTime > maxTime {
		tm.allocatedTime = maxTime
	}
}

func (tm *TimeManager) setSearchTerminationObserver(observer *searchTerminationObserver) {
	tm.observer = observer
	if observer != nil {
		observer.reset()
	}
}

func (tm *TimeManager) observeTermination(reason searchTerminationReason, depth int, elapsed time.Duration) {
	observer := tm.observer
	if observer == nil || observer.FirstReason != searchTerminationNone {
		return
	}
	observer.FirstReason = reason
	observer.FirstDepth = depth
	observer.FirstElapsed = elapsed
}

func (tm *TimeManager) observeControlTermination(reason searchTerminationReason, depth int) {
	if tm.observer == nil {
		return
	}
	tm.observeTermination(reason, depth, tm.nowTime().Sub(tm.startTime))
}

func (tm *TimeManager) observeCompletedIteration(depth, attempts int) {
	observer := tm.observer
	if observer == nil || tm.iterationStartTime.IsZero() {
		return
	}
	now := tm.nowTime()
	record := searchIterationObservation{
		Depth:              depth,
		Duration:           now.Sub(tm.iterationStartTime),
		CompletedAtElapsed: now.Sub(tm.startTime),
		AspirationAttempts: attempts,
	}
	if observer.IterationCount < len(observer.Iterations) {
		observer.Iterations[observer.IterationCount] = record
		observer.IterationCount++
		return
	}
	observer.IterationOverflow++
}

// computeTournamentLimits sets the soft and hard per-move budgets from an
// increment-aware, remaining-relative formula. The old allocation stacked
// game-phase × complexity × score × time multipliers on top of an even share,
// so it could plan several× a sustainable per-move slice and bleed the bank far
// faster than the increment refilled. This version is sustainable at any clock:
//   - soft = (usable bank / moves-to-go) + 0.8·increment  — the normal target;
//     spending ~the increment each move keeps the bank roughly flat.
//   - hard = min(4·soft, 0.3·usable bank)                 — the rare critical-move
//     ceiling; the 0.3 cap means one move can never blow the bank (anti-flag).
func (tm *TimeManager) computeTournamentLimits() {
	moves := tm.movesToGo
	if moves <= 0 {
		moves = tm.estimateMovesRemaining()
	}
	if moves < 1 {
		moves = 1
	}
	// Reserve so we never plan into the emergency buffer or the GUI/move overhead.
	usable := tm.baseTime - tm.emergencyTime - tm.moveOverhead
	if usable < 0 {
		usable = 0
	}
	uf := float64(usable)

	soft := uf/float64(moves) + 0.8*float64(tm.increment)
	hard := math.Min(soft*4, uf*0.3)
	if hard < soft {
		hard = soft // tiny banks: don't let the 0.3 cap fall below the soft target
	}

	tm.softTime = time.Duration(soft)
	tm.hardTime = time.Duration(hard)
	tm.allocatedTime = tm.softTime // the normal per-move target (hard is the rare ceiling)
}

// estimateMovesRemaining estimates how many moves are left in the game
func (tm *TimeManager) estimateMovesRemaining() int {
	// Be much more optimistic about remaining moves to allow more thinking time
	remaining := 40 - tm.movesPlayed // Assume shorter games for better time usage
	switch tm.gamePhase {
	case Opening:
		remaining = 30 - tm.movesPlayed // Shorter estimate in opening
	case Middlegame:
		remaining = 25 - tm.movesPlayed // Focus time on critical middle game
	case Endgame:
		remaining = 20 - tm.movesPlayed // Endgame needs precise calculation
	}

	// Never estimate more than 40 moves remaining, even early in game
	if remaining > 40 {
		remaining = 40
	}
	// Always assume at least 10 moves remaining for safety
	if remaining < 10 {
		remaining = 10
	}

	return remaining
}

// ensureSafeTimeAllocation ensures we don't allocate too much time
func (tm *TimeManager) ensureSafeTimeAllocation() time.Duration {
	if tm.timeControl == Tournament {
		// Scale maximum allocation with available time
		var maxAllocation time.Duration
		if tm.baseTime > time.Minute*10 {
			// With lots of time, can use up to 1/2 of remaining time
			maxAllocation = tm.baseTime / 2
		} else if tm.baseTime > time.Minute*5 {
			// With moderate time, use up to 2/5 of remaining time
			maxAllocation = tm.baseTime * 2 / 5
		} else {
			// With little time, be conservative - 1/3 of remaining time
			maxAllocation = tm.baseTime / 3
		}
		if tm.allocatedTime > maxAllocation {
			tm.allocatedTime = maxAllocation
		}

		// Always keep emergency reserve
		if tm.allocatedTime > tm.baseTime-tm.emergencyTime {
			tm.allocatedTime = tm.baseTime - tm.emergencyTime
		}
	}

	// Account for move overhead
	if tm.allocatedTime > tm.moveOverhead {
		tm.allocatedTime -= tm.moveOverhead
	}

	return tm.allocatedTime
}

// ShouldStopSearch determines if search should stop based on time
// This method can be called frequently without performance penalty - it uses
// internal rate limiting to only do expensive time checks every N calls
// NewIteration is called by iterative deepening at the top of each new depth
// so the soft-stop heuristic can measure how long the current iteration took
// and project the next one. Without this, ShouldStopSearch's nextIterationEstimate
// would be zero.
func (tm *TimeManager) NewIteration() {
	// Record how long the iteration that just finished took, so the soft-stop can
	// project the next one. NewIteration runs at the top of each ID depth, so when
	// called for depth N the previous (N-1) iteration's elapsed is measured here.
	if !tm.iterationStartTime.IsZero() {
		tm.lastIterationTime = tm.nowTime().Sub(tm.iterationStartTime)
	}
	tm.iterationStartTime = tm.nowTime()
}

// ReportCompletedIteration feeds the root's per-iteration signals to the time
// manager: the decision-stability flags (T1b) and the best-move node fraction
// (T1e). A completed iteration that kept the same best move AND held its aspiration
// window on the first try means the choice is settling, so stableIters climbs and
// shouldStopTournamentSearch shrinks the soft budget. Any best-move change or window
// break (a fail-high/fail-low re-search) resets it — that volatility is exactly when
// the search must NOT be cut short. bestMoveNodeFraction (the share of the iteration's
// nodes spent inside the best move's subtree) is stored unconditionally — it reflects
// the node distribution of the completed iteration regardless of stability — and is
// read by nodeEffortFactor. Only the soft target flexes; the hard ceiling never does,
// so this cannot increase flag risk.
func (tm *TimeManager) ReportCompletedIteration(bestMoveChanged, windowHeldFirstTry bool, bestMoveNodeFraction float64) {
	tm.bestMoveNodeFraction = bestMoveNodeFraction
	if bestMoveChanged || !windowHeldFirstTry {
		tm.stableIters = 0
		return
	}
	tm.stableIters++
}

// Soft-budget stability scaling (T1b). stabilitySoftFactor maps the
// consecutive-stable-iteration count (stableIters, maintained by
// ReportCompletedIteration) to the multiplier the tournament soft stop applies to
// softTime. It starts at stabilitySoftMax while the decision is volatile
// (stableIters == 0) and decays by stabilitySoftSlope per settled iteration, floored
// at stabilitySoftMin — so a churning root extends the soft target and a settled one
// shrinks it and banks the difference for later hard moves. It is symmetric about 1.0
// (crossover at stableIters == 3 with these constants), NOT the one-sided shrink
// removed 2026-06-03. Only the soft target flexes; the hard ceiling, the emergency
// floor, and the next-iteration projection never do, so this cannot raise flag risk.
const (
	stabilitySoftMax   = 1.30 // volatile (stableIters==0): extend the soft target
	stabilitySoftMin   = 0.85 // settled: shrink it and bank the time for harder moves
	stabilitySoftSlope = 0.10 // decay per stable iteration (1.0 crossover at stableIters==3)
)

func (tm *TimeManager) stabilitySoftFactor() float64 {
	// stableIters is >= 0, so the factor starts at the max and only decreases; only the
	// lower clamp is needed.
	f := stabilitySoftMax - stabilitySoftSlope*float64(tm.stableIters)
	if f < stabilitySoftMin {
		f = stabilitySoftMin
	}
	return f
}

// Soft-budget node-effort scaling (T1e, the Stockfish-style third time signal).
// nodeEffortFactor maps bestMoveNodeFraction — the share of the last completed
// iteration's nodes spent inside the best move's subtree (maintained by
// ReportCompletedIteration) — to the multiplier the tournament soft stop applies to
// softTime. When the best move owns most of the tree the decision is confident, so
// the factor shrinks the soft target toward nodeEffortMin and banks the time; when the
// search had to spread nodes across rival moves the position is contested, so it
// extends the target toward nodeEffortMax. The map is linear in (1 - fraction):
//
//	factor = nodeEffortMin + (nodeEffortMax - nodeEffortMin)*(1 - fraction)
//
// which already lands in [nodeEffortMin, nodeEffortMax] for fraction in [0,1] (the
// crossover to 1.0 is at fraction == 0.625 with these constants; the explicit clamp is
// a guard). fraction == 0 means "no completed iteration yet" → neutral 1.0, so the
// first iteration and every non-tournament search are unaffected. Like T1b this only
// flexes the soft target; the hard ceiling, the emergency floor, and the
// next-iteration projection never do, so it cannot raise flag risk.
const (
	nodeEffortMin = 0.85 // best move dominates the tree (fraction→1): confident, shrink the soft target
	nodeEffortMax = 1.25 // best move is a small share (fraction→0): contested, extend the soft target
)

func (tm *TimeManager) nodeEffortFactor() float64 {
	f := tm.bestMoveNodeFraction
	if f <= 0 {
		return 1.0 // no completed iteration yet (or a degenerate one): stay neutral
	}
	if f > 1 {
		f = 1
	}
	factor := nodeEffortMin + (nodeEffortMax-nodeEffortMin)*(1.0-f)
	if factor < nodeEffortMin {
		factor = nodeEffortMin
	}
	if factor > nodeEffortMax {
		factor = nodeEffortMax
	}
	return factor
}

// Bounds on the composed T1b×T1e soft-target multiplier. stabilitySoftFactor and
// nodeEffortFactor are correlated — a settled best move usually also owns most of the
// tree — so their raw product (up to 1.30×1.25 = 1.625, down to 0.85×0.85 ≈ 0.7225) is
// clamped here to stop the two signals from double-counting into a budget that
// overshoots T1b's proven 1.30 max or undercuts a safe floor. The ceiling sits only a
// little above 1.30 so node effort adds modest headroom, not a second full extension;
// the floor sits just below the natural 0.7225 min so the most-confident case still
// keeps a sane fraction of the budget. The T1a-proven next-iteration projection, the
// hard ceiling, and the emergency floor are untouched, so the bound cannot flag.
const (
	softComposedMin = 0.72 // never spend below ~72% of the soft target, even both-signals-confident
	softComposedMax = 1.40 // never spend above ~140% of it, even both-signals-volatile
)

func (tm *TimeManager) ShouldStopSearch(depth int) bool {
	return tm.shouldStopSearchAt(depth, timeCheckActiveSearch)
}

func (tm *TimeManager) shouldStopSearchAt(depth int, site timeCheckSite) bool {
	// Plain counter: only the search goroutine calls this, and SetTimeControl's
	// reset happens-before the goroutine launch — the old atomic.AddUint64 was a
	// per-node RMW bought for nothing (single-threaded search).
	tm.checkCounter++

	// Only do expensive time checking every 1024 calls (or immediately if we should stop)
	if tm.shouldStop || tm.checkCounter%1024 == 0 {
		now := tm.nowTime()
		elapsed := now.Sub(tm.startTime)
		tm.lastTimeCheck = now

		switch tm.timeControl {
		case FixedDepth:
			tm.shouldStop = false // Time doesn't matter for fixed depth
		case Infinite:
			tm.shouldStop = false // Never stop for infinite search
		case TimePerMove, FixedTime:
			// Hard time limit - stop when time is up
			tm.shouldStop = elapsed >= tm.allocatedTime
			if tm.shouldStop {
				tm.observeTermination(searchTerminationHardDeadline, depth, elapsed)
			}
		case Tournament:
			reason := tm.tournamentStopReason(depth, elapsed)
			tm.shouldStop = reason != searchTerminationNone
			if reason == searchTerminationSoftAbort && site == timeCheckIterationAdmission {
				reason = searchTerminationSoftAdmission
			}
			if tm.shouldStop {
				tm.observeTermination(reason, depth, elapsed)
			}
		}
	}

	return tm.shouldStop
}

// shouldStopTournamentSearch decides, from the soft/hard budgets set by
// computeTournamentLimits, whether to stop searching. The OLD version layered
// absolute minThinkTime (0.5-8s) and minDepth (4-12) floors that ignored the bank
// — the bullet-killers that forced spending a fixed wall-time regardless of how
// little time was left. This version is purely bank-relative.
func (tm *TimeManager) shouldStopTournamentSearch(depth int, elapsed time.Duration) bool {
	return tm.tournamentStopReason(depth, elapsed) != searchTerminationNone
}

func (tm *TimeManager) tournamentStopReason(depth int, elapsed time.Duration) searchTerminationReason {
	// Hard ceiling — never exceeded. computeTournamentLimits caps this at 30% of the
	// usable bank, so a single move can never flag us (the anti-flag guarantee).
	if elapsed >= tm.hardTime {
		return searchTerminationHardDeadline
	}
	// Emergency: down to the reserve, stop regardless of the budget arithmetic.
	if tm.baseTime-elapsed <= tm.emergencyTime {
		return searchTerminationEmergencyReserve
	}
	// Always finish at least one iteration so the move is real (never a depth-0
	// blunder); the search's clock-interrupt guard keeps the last completed depth
	// if a later iteration is cut off mid-flight.
	if depth < 1 {
		return searchTerminationNone
	}
	// Soft limit: don't START another iteration we can't finish within the soft budget.
	//
	// The soft target is scaled by two composed signals — decision stability (T1b,
	// stabilitySoftFactor) and best-move node effort (T1e, nodeEffortFactor): while the
	// root's best move is still churning OR the search is spreading nodes across rivals the
	// position is genuinely hard and earns MORE of the bank; once the choice has settled AND
	// the best move owns most of the tree it earns LESS and banks the difference for later
	// hard moves. This REDISTRIBUTES the clock toward the moves that decide games instead of
	// moving total spend uniformly — both uniform directions are measured regressions: the
	// one-sided "easy move" shrink removed 2026-06-03 (×0.5/×0.7 on stability) drained whole
	// games to ~25% of the clock, and spending the FULL soft budget every move measured -7.8
	// Elo (T1a, 2026-07-02). The bounded scaling here is neither; even the most volatile move
	// (composed max factor 1.40) still stops below that budget once the ×1 projection is
	// added. The next-iteration projection, the hard ceiling, and the emergency floor above
	// are the flag protection and are UNTOUCHED, so this cannot flag.
	// Compose the two soft-target signals multiplicatively: decision stability (T1b,
	// stabilitySoftFactor) and node effort (T1e, nodeEffortFactor). Because they are
	// correlated the raw product is bounded to [softComposedMin, softComposedMax] so
	// the pair cannot double-count into a budget that overshoots T1b's proven 1.30 max
	// or undercuts a safe floor. The next-iteration projection, the hard ceiling, and
	// the emergency floor above are UNCHANGED, so this cannot flag.
	composed := tm.stabilitySoftFactor() * tm.nodeEffortFactor()
	if composed < softComposedMin {
		composed = softComposedMin
	}
	if composed > softComposedMax {
		composed = softComposedMax
	}
	soft := time.Duration(float64(tm.softTime) * composed)
	if tm.lastIterationTime == 0 {
		if elapsed >= soft {
			return searchTerminationSoftAbort
		}
		return searchTerminationNone
	}
	if elapsed+tm.lastIterationTime > soft {
		return searchTerminationSoftAbort
	}
	return searchTerminationNone
}

// UpdateGameState updates the time manager with current game state
func (tm *TimeManager) UpdateGameState(movesPlayed int, lastScore, currentScore int, position *Position) {
	tm.movesPlayed = movesPlayed / 2 // D2: movesPlayed is a ply count (uci.go:411); halve to full-moves so the full-move constants in estimateMovesRemaining/determineGamePhase aren't decremented 2x too fast (was front-loading the clock, moves ~8-15)
	tm.lastScoreChange = currentScore - lastScore

	// Determine game phase
	tm.gamePhase = tm.determineGamePhase(position)

	// Calculate position complexity
	tm.positionComplexity = tm.calculatePositionComplexity(position)
}

// determineGamePhase determines the current game phase
func (tm *TimeManager) determineGamePhase(position *Position) GamePhase {
	if position == nil {
		return Middlegame // Default
	}

	// Count material to determine phase
	totalMaterial := 0
	queens := PopCount(position.Board.GetBitboardOf(WhiteQueen)) + PopCount(position.Board.GetBitboardOf(BlackQueen))
	rooks := PopCount(position.Board.GetBitboardOf(WhiteRook)) + PopCount(position.Board.GetBitboardOf(BlackRook))
	minors := PopCount(position.Board.GetBitboardOf(WhiteBishop)) + PopCount(position.Board.GetBitboardOf(BlackBishop)) +
		PopCount(position.Board.GetBitboardOf(WhiteKnight)) + PopCount(position.Board.GetBitboardOf(BlackKnight))

	totalMaterial = queens*9 + rooks*5 + minors*3

	if tm.movesPlayed < 15 && totalMaterial > 60 {
		return Opening
	} else if totalMaterial < 20 {
		return Endgame
	}

	return Middlegame
}

// calculatePositionComplexity calculates a complexity score for the position
func (tm *TimeManager) calculatePositionComplexity(position *Position) float64 {
	if position == nil {
		return 0.5 // Default complexity
	}

	allPiecesBitboard := position.Board.GetWhitePieces() | position.Board.GetBlackPieces()
	totalPieces := PopCount(allPiecesBitboard)
	if totalPieces == 0 {
		return 0.0
	}

	complexity := 0.0

	// Piece count: middlegame (15-25 pieces) is most complex, opening and endgame less so.
	// Triangle peak at 20 pieces.
	pieceCountFactor := 1.0 - math.Abs(float64(totalPieces)-20.0)/20.0
	if pieceCountFactor < 0 {
		pieceCountFactor = 0
	}
	complexity += pieceCountFactor * 0.3

	// More piece-type variety = more complex
	pieceTypes := 0
	for _, piece := range []Piece{WhiteQueen, BlackQueen, WhiteRook, BlackRook,
		WhiteBishop, BlackBishop, WhiteKnight, BlackKnight} {
		if PopCount(position.Board.GetBitboardOf(piece)) > 0 {
			pieceTypes++
		}
	}
	complexity += float64(pieceTypes) / 8.0 * 0.3

	return math.Min(complexity, 1.0)
}

// GetTimeControlString returns a string describing the current time control
func (tm *TimeManager) GetTimeControlString() string {
	switch tm.timeControl {
	case FixedDepth:
		return "Fixed Depth"
	case FixedTime:
		return "Fixed Time"
	case TimePerMove:
		return "Time Per Move"
	case Tournament:
		return "Tournament"
	case Infinite:
		return "Infinite"
	}
	return "Unknown"
}

// GetAllocatedTime returns the time allocated for the current move
func (tm *TimeManager) GetAllocatedTime() time.Duration {
	return tm.allocatedTime
}

// GetElapsedTime returns the time elapsed since search started
func (tm *TimeManager) GetElapsedTime() time.Duration {
	return tm.nowTime().Sub(tm.startTime)
}

// SetupDeadlineExceeded is a direct, unrated hard-deadline check used around
// pre-search setup. Fixed-depth and infinite searches have no wall-clock setup
// deadline. Tournament setup uses the existing hard ceiling; it does not alter
// the soft iteration policy.
func (tm *TimeManager) SetupDeadlineExceeded() bool {
	elapsed := tm.GetElapsedTime()
	switch tm.timeControl {
	case TimePerMove, FixedTime:
		if elapsed >= tm.allocatedTime {
			tm.observeTermination(searchTerminationHardDeadline, 0, elapsed)
			return true
		}
		return false
	case Tournament:
		if elapsed >= tm.hardTime {
			tm.observeTermination(searchTerminationHardDeadline, 0, elapsed)
			return true
		}
		if tm.baseTime-elapsed <= tm.emergencyTime {
			tm.observeTermination(searchTerminationEmergencyReserve, 0, elapsed)
			return true
		}
		return false
	default:
		return false
	}
}

func (tm *TimeManager) nowTime() time.Time {
	if tm.now == nil {
		return time.Now()
	}
	return tm.now()
}

// IsEmergencyTime returns true if we're in emergency time
func (tm *TimeManager) IsEmergencyTime() bool {
	if tm.timeControl != Tournament {
		return false
	}

	elapsed := tm.nowTime().Sub(tm.startTime)
	remaining := tm.baseTime - elapsed

	return remaining <= tm.emergencyTime*2 // Emergency when 2x emergency time left
}
