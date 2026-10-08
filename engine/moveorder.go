package engine

// Correction-history tables are keyed by side to move and a hashed board feature.
const (
	corrHistSize  = 16384 // 2^14 slots per side
	corrHistMask  = corrHistSize - 1
	corrHistLimit = 1024 // clamp on the stored EMA entry (Stockfish CORRECTION_HISTORY_LIMIT)
)

// workerHistory owns the mutable search heuristics for one SearchEngine worker.
// Its tables persist across searches on that engine and are reset only at the same
// explicit lifecycle points as before. Keeping the complete heuristic family together
// prevents ordering, correction, killer, counter, and predecessor state from crossing
// engine instances.
type workerHistory struct {
	// Main quiet history indexed by [piece][to].
	historyTable [13][64]int
	// Continuation and follow-up histories retain the exact [previous move][move]
	// dimensions used by the package-owned implementation.
	continuationHistory [13][64][13][64]int
	followupHistory     [13][64][13][64]int

	pawnCorrectionHistory    [2][corrHistSize]int
	nonPawnCorrectionHistory [2][corrHistSize]int
	minorCorrectionHistory   [2][corrHistSize]int
	captureHistory           [13][64][13]int
	killerMoves              [MaximumDepth][2]Move
	counterMoves             [64][64]Move
	lastMovePlayed           Move
}

// pawnCorrectionIndex hashes both colors' pawn bitboards into a correction-table slot.
func pawnCorrectionIndex(whitePawns, blackPawns uint64) int {
	key := whitePawns*0x9E3779B97F4A7C15 ^ blackPawns*0xC2B2AE3D27D4EB4F
	key ^= key >> 29
	return int(key & corrHistMask)
}

// correctionValue returns the centipawn adjustment to add to the static eval. The
// NGN chooses 3/64 as a neutral round placeholder for later NGN tuning.
// A saturated entry (±corrHistLimit) yields at most ±48 cp.
func (h *workerHistory) correctionValue(stm Color, idx int) int {
	return h.pawnCorrectionHistory[stm][idx] * 3 / 64
}

// correctedStandPat is the raw eval plus the side-to-move's pawn-structure, non-pawn AND
// minor-piece corrections — the same backed-up static value interior nodes use (search.go ~1353). T4:
// qsearch applies it at the !inCheck stand-pat and the qDepth cap-return so the whole search
// backs up corrected statics (was: qsearch returned the raw eval, discarding the correction).
// The ply>=MaximumDepth emergency cap stays RAW — it fires before inCheck is known, and an
// in-check static must never be corrected. The eval cache is unaffected — the correction is
// added outside it.
func (h *workerHistory) correctedStandPat(pos *Position, evaluator *workerEvaluator) int {
	stm := pos.Turn()
	return evaluator.SearchSTM(pos) +
		h.correctionValue(stm, pawnCorrectionIndex(pos.Board.pieces[WhitePawn], pos.Board.pieces[BlackPawn])) +
		h.nonPawnCorrectionValue(stm, nonPawnCorrectionIndex(pos.Board.GetWhitePieces()&^pos.Board.pieces[WhitePawn], pos.Board.GetBlackPieces()&^pos.Board.pieces[BlackPawn])) +
		h.minorCorrectionValue(stm, minorCorrectionIndex(pos.Board.pieces[WhiteKnight]|pos.Board.pieces[WhiteBishop], pos.Board.pieces[BlackKnight]|pos.Board.pieces[BlackBishop]))
}

// updatePawnCorrection nudges a slot toward the observed (searchScore - staticEval) gap
// with a depth-weighted, gravity-damped EMA (Stockfish StatsEntry form) that keeps the
// entry bounded in roughly [-corrHistLimit, corrHistLimit].
func (h *workerHistory) updatePawnCorrection(stm Color, idx, diff, depth int) {
	bonus := diff * depth / 8
	if bonus > corrHistLimit/4 {
		bonus = corrHistLimit / 4
	} else if bonus < -corrHistLimit/4 {
		bonus = -corrHistLimit / 4
	}
	mag := bonus
	if mag < 0 {
		mag = -mag
	}
	e := &h.pawnCorrectionHistory[stm][idx]
	*e += bonus - (*e)*mag/corrHistLimit
}

// maybeUpdatePawnCorrection applies the correction update at a completed node when the
// search result is a consistent directional signal: worse than static eval (a true upper
// bound) or better with a real best move. Capture-driven results are skipped as noisy.
func (h *workerHistory) maybeUpdatePawnCorrection(stm Color, idx, staticEval, bestScore, beta, depth int, bestMove Move, inCheck bool) {
	if inCheck {
		return
	}
	if bestMove != EmptyMove && bestMove.IsCapture() {
		return
	}
	if !((bestScore < staticEval && bestScore < beta) || (bestScore > staticEval && bestMove != EmptyMove)) {
		return
	}
	h.updatePawnCorrection(stm, idx, bestScore-staticEval, depth)
}

// Non-pawn correction history: identical mechanism to pawnCorrectionHistory but keyed on
// each side's NON-pawn piece placement (knights..king). The pawn key is blind to recurring
// eval errors that depend on piece configuration (e.g. a persistently mis-valued knight
// outpost or rook battery); this is an independent second corrector summed alongside it.
// It rides the SAME fixed T4 plumbing as the pawn term — applied at the qsearch stand-pat
// (via correctedStandPat) and the interior static-eval site, and learned from the pre-S7
// corrStaticEval — which is the whole point of the retest.

// nonPawnCorrectionIndex hashes both sides' non-pawn occupancy into a slot. Distinct
// mixing constants (murmur3 finalizers) from pawnCorrectionIndex so the two tables alias
// independently.
func nonPawnCorrectionIndex(whiteNonPawn, blackNonPawn uint64) int {
	key := whiteNonPawn*0xFF51AFD7ED558CCD ^ blackNonPawn*0xC4CEB9FE1A85EC53
	key ^= key >> 29
	return int(key & corrHistMask)
}

func (h *workerHistory) nonPawnCorrectionValue(stm Color, idx int) int {
	return h.nonPawnCorrectionHistory[stm][idx] * 3 / 64
}

func (h *workerHistory) updateNonPawnCorrection(stm Color, idx, diff, depth int) {
	bonus := diff * depth / 8
	if bonus > corrHistLimit/4 {
		bonus = corrHistLimit / 4
	} else if bonus < -corrHistLimit/4 {
		bonus = -corrHistLimit / 4
	}
	mag := bonus
	if mag < 0 {
		mag = -mag
	}
	e := &h.nonPawnCorrectionHistory[stm][idx]
	*e += bonus - (*e)*mag/corrHistLimit
}

func (h *workerHistory) maybeUpdateNonPawnCorrection(stm Color, idx, staticEval, bestScore, beta, depth int, bestMove Move, inCheck bool) {
	if inCheck {
		return
	}
	if bestMove != EmptyMove && bestMove.IsCapture() {
		return
	}
	if !((bestScore < staticEval && bestScore < beta) || (bestScore > staticEval && bestMove != EmptyMove)) {
		return
	}
	h.updateNonPawnCorrection(stm, idx, bestScore-staticEval, depth)
}

// Minor-piece correction history: same mechanism, keyed on each side's MINOR-piece
// (knight+bishop) placement. A finer key than nonPawn (which lumps every non-pawn into
// one slot): it sharpens corrections for recurring minor-piece configuration errors
// (outposts, bad bishops, knight-vs-bishop imbalances) that the coarse key aliases away.
// Independent third corrector summed alongside the pawn and non-pawn terms. Rides the SAME
// fixed T4 plumbing — applied at the qsearch stand-pat (via correctedStandPat) and the
// interior static-eval site, and learned from the pre-S7 corrStaticEval.

// minorCorrectionIndex hashes both sides' minor occupancy into a slot, with a third
// distinct constant pair (the other splitmix64 finalizers) so it aliases independently.
func minorCorrectionIndex(whiteMinors, blackMinors uint64) int {
	key := whiteMinors*0xBF58476D1CE4E5B9 ^ blackMinors*0x94D049BB133111EB
	key ^= key >> 29
	return int(key & corrHistMask)
}

func (h *workerHistory) minorCorrectionValue(stm Color, idx int) int {
	return h.minorCorrectionHistory[stm][idx] * 3 / 64
}

func (h *workerHistory) updateMinorCorrection(stm Color, idx, diff, depth int) {
	bonus := diff * depth / 8
	if bonus > corrHistLimit/4 {
		bonus = corrHistLimit / 4
	} else if bonus < -corrHistLimit/4 {
		bonus = -corrHistLimit / 4
	}
	mag := bonus
	if mag < 0 {
		mag = -mag
	}
	e := &h.minorCorrectionHistory[stm][idx]
	*e += bonus - (*e)*mag/corrHistLimit
}

func (h *workerHistory) maybeUpdateMinorCorrection(stm Color, idx, staticEval, bestScore, beta, depth int, bestMove Move, inCheck bool) {
	if inCheck {
		return
	}
	if bestMove != EmptyMove && bestMove.IsCapture() {
		return
	}
	if !((bestScore < staticEval && bestScore < beta) || (bestScore > staticEval && bestMove != EmptyMove)) {
		return
	}
	h.updateMinorCorrection(stm, idx, bestScore-staticEval, depth)
}

// Killer moves table for move ordering heuristic.
// killerMoves[ply][slot] stores killer moves keyed by PLY (distance from root), 2 per ply.
// Keying by ply (not remaining depth) is what lets a quiet refutation found at one node be
// tried at its siblings/cousins at the same distance from root; keying by remaining depth
// instead aliased every same-draft node tree-wide (positions nothing alike sharing slots).

// Counter moves table for move ordering heuristic
// counterMoves[from][to] stores the best counter move to moves from 'from' to 'to'

// Track the last move played for counter move heuristic

// historyMax bounds every history entry. gravityUpdate asymptotes entries toward
// ±historyMax instead of letting them grow until a cap event; 8192 keeps the live
// scale identical to the old clamp, so the ordering bands (killers 25000, counters
// 30000) and the history-pruning/LMR thresholds stay calibrated.
const historyMax = 8192

// historyBonusCap bounds a single update step. Gravity is self-bounding for any
// |bonus| <= historyMax, but uncapped depth² steps (d12 → 144 vs d3 → 9 is fine;
// d40+ → 1600+ is not) would let one deep cutoff swamp accumulated signal;
// ~historyMax/4 is the reference engines' step ceiling.
const historyBonusCap = 2048

// gravityUpdate applies the standard bounded history update: the entry moves by
// bonus, decayed in proportion to its current magnitude. Equilibrium at ±historyMax,
// recent results outweigh stale ones, and there is no table-wide halving cliff (the
// old ageHistoryTable erased ALL relative ordering info whenever any single entry
// crossed the cap, and the per-entry /2 did the same locally).
func gravityUpdate(e *int, bonus int) {
	if bonus > historyBonusCap {
		bonus = historyBonusCap
	} else if bonus < -historyBonusCap {
		bonus = -historyBonusCap
	}
	mag := bonus
	if mag < 0 {
		mag = -mag
	}
	*e += bonus - (*e)*mag/historyMax
}

// UpdateHistoryTable increases the history score for a move that caused a cutoff
func (h *workerHistory) UpdateHistoryTable(move Move, prevMove Move, prev2 Move, depth int) {
	if move.IsCapture() || move.PromoType() != NoType {
		return
	}
	piece := move.MovingPiece()
	to := move.Destination()
	bonus := depth * depth
	gravityUpdate(&h.historyTable[piece][to], bonus)
	if prevMove != EmptyMove {
		gravityUpdate(&h.continuationHistory[prevMove.MovingPiece()][prevMove.Destination()][piece][to], bonus)
	}
	if prev2 != EmptyMove {
		gravityUpdate(&h.followupHistory[prev2.MovingPiece()][prev2.Destination()][piece][to], bonus)
	}
}

// PenalizeHistoryTable decreases the history score for moves that didn't cause cutoff
func (h *workerHistory) PenalizeHistoryTable(move Move, prevMove Move, prev2 Move, depth int) {
	if move.IsCapture() || move.PromoType() != NoType {
		return
	}
	piece := move.MovingPiece()
	to := move.Destination()
	penalty := depth * depth
	gravityUpdate(&h.historyTable[piece][to], -penalty)
	if prevMove != EmptyMove {
		gravityUpdate(&h.continuationHistory[prevMove.MovingPiece()][prevMove.Destination()][piece][to], -penalty)
	}
	if prev2 != EmptyMove {
		gravityUpdate(&h.followupHistory[prev2.MovingPiece()][prev2.Destination()][piece][to], -penalty)
	}
}

// GetHistoryScore returns the history score for a move, summing the main
// [piece][to] history with the continuation [prev-piece][prev-to][piece][to]
// and follow-up [prev2-piece][prev2-to][piece][to] tables when those moves are
// available. All terms get full weight — same magnitude updates means same ELO
// scale.
func (h *workerHistory) GetHistoryScore(move Move, prevMove Move, prev2 Move) int {
	if move.IsCapture() || move.PromoType() != NoType {
		return 0
	}
	piece := move.MovingPiece()
	to := move.Destination()
	score := h.historyTable[piece][to]
	if prevMove != EmptyMove {
		score += h.continuationHistory[prevMove.MovingPiece()][prevMove.Destination()][piece][to]
	}
	if prev2 != EmptyMove {
		score += h.followupHistory[prev2.MovingPiece()][prev2.Destination()][piece][to]
	}
	return score
}

// Capture history: indexed by [moving-piece][to][captured-piece]. The quiet history
// tables above skip captures, so without this captures are ordered by SEE alone. This
// adds a learned tie-breaker — captures that repeatedly cause cutoffs in this search are
// tried earlier among captures of similar SEE. Gravity-clamped like the quiet tables.

func (h *workerHistory) captureHistoryScore(move Move) int {
	return h.captureHistory[move.MovingPiece()][move.Destination()][move.CapturedPiece()]
}

// UpdateCaptureHistory rewards a capture that caused a beta cutoff.
func (h *workerHistory) UpdateCaptureHistory(move Move, depth int) {
	gravityUpdate(&h.captureHistory[move.MovingPiece()][move.Destination()][move.CapturedPiece()], depth*depth)
}

// PenalizeCaptureHistory decreases a tried capture that did not cause the cutoff.
func (h *workerHistory) PenalizeCaptureHistory(move Move, depth int) {
	gravityUpdate(&h.captureHistory[move.MovingPiece()][move.Destination()][move.CapturedPiece()], -(depth * depth))
}

// ClearHistoryTable resets both history tables (used between games)
func (h *workerHistory) ClearHistoryTable() {
	for i := range h.historyTable {
		for j := range h.historyTable[i] {
			h.historyTable[i][j] = 0
		}
	}
	for i := range h.continuationHistory {
		for j := range h.continuationHistory[i] {
			for k := range h.continuationHistory[i][j] {
				for l := range h.continuationHistory[i][j][k] {
					h.continuationHistory[i][j][k][l] = 0
				}
			}
		}
	}
	for i := range h.followupHistory {
		for j := range h.followupHistory[i] {
			for k := range h.followupHistory[i][j] {
				for l := range h.followupHistory[i][j][k] {
					h.followupHistory[i][j][k][l] = 0
				}
			}
		}
	}
	for i := range h.pawnCorrectionHistory {
		for j := range h.pawnCorrectionHistory[i] {
			h.pawnCorrectionHistory[i][j] = 0
		}
	}
	for i := range h.nonPawnCorrectionHistory {
		for j := range h.nonPawnCorrectionHistory[i] {
			h.nonPawnCorrectionHistory[i][j] = 0
		}
	}
	for i := range h.minorCorrectionHistory {
		for j := range h.minorCorrectionHistory[i] {
			h.minorCorrectionHistory[i][j] = 0
		}
	}
	for i := range h.captureHistory {
		for j := range h.captureHistory[i] {
			for k := range h.captureHistory[i][j] {
				h.captureHistory[i][j][k] = 0
			}
		}
	}
}

// UpdateKillerMoves stores a move that caused a beta cutoff as a killer move at the node's ply
func (h *workerHistory) UpdateKillerMoves(move Move, ply int) {
	// Only store quiet moves as killer moves
	if !move.IsCapture() && move.PromoType() == NoType && ply >= 0 && ply < MaximumDepth {
		// Check if this move is already stored as first killer
		if h.killerMoves[ply][0] == move {
			return
		}

		// Shift moves down: second killer becomes first, new move becomes second
		h.killerMoves[ply][1] = h.killerMoves[ply][0]
		h.killerMoves[ply][0] = move
	}
}

// IsKillerMove checks if a move is a killer move at the given ply
func (h *workerHistory) IsKillerMove(move Move, ply int) bool {
	if ply < 0 || ply >= MaximumDepth {
		return false
	}
	return h.killerMoves[ply][0] == move || h.killerMoves[ply][1] == move
}

// ClearKillerMoves resets the killer moves table
func (h *workerHistory) ClearKillerMoves() {
	for i := range h.killerMoves {
		h.killerMoves[i][0] = EmptyMove
		h.killerMoves[i][1] = EmptyMove
	}
}

// UpdateCounterMove stores a move as the best counter to the given previous move
func (h *workerHistory) UpdateCounterMove(counterMove Move, previousMove Move) {
	// Only store quiet moves as counter moves
	if !counterMove.IsCapture() && counterMove.PromoType() == NoType && previousMove != EmptyMove {
		from := previousMove.Source()
		to := previousMove.Destination()
		h.counterMoves[from][to] = counterMove
	}
}

// GetCounterMove returns the counter move for the given previous move
func (h *workerHistory) GetCounterMove(previousMove Move) Move {
	if previousMove == EmptyMove {
		return EmptyMove
	}
	from := previousMove.Source()
	to := previousMove.Destination()
	return h.counterMoves[from][to]
}

// IsCounterMove checks if a move is the counter move to the previous move
func (h *workerHistory) IsCounterMove(move Move, previousMove Move) bool {
	return h.GetCounterMove(previousMove) == move && move != EmptyMove
}

// SetLastMovePlayed sets the last move played for counter move heuristic
func (h *workerHistory) SetLastMovePlayed(move Move) {
	h.lastMovePlayed = move
}

// GetLastMovePlayed returns the last move played
func (h *workerHistory) GetLastMovePlayed() Move {
	return h.lastMovePlayed
}

// ClearCounterMoves resets the counter moves table
func (h *workerHistory) ClearCounterMoves() {
	for i := range h.counterMoves {
		for j := range h.counterMoves[i] {
			h.counterMoves[i][j] = EmptyMove
		}
	}
	h.lastMovePlayed = EmptyMove
}

// Single-pass move ordering with enhanced SEE integration
// Orders moves in one pass using scores, with captures ordered by SEE value
func (h *workerHistory) orderMovesSinglePass(moves []Move, depth int, previousMove Move, ttMove Move, ttHit bool, pos *Position) []Move {
	if len(moves) == 0 {
		return moves
	}

	// Use zero-allocation buffer version internally
	var buffer [256]Move
	count := h.orderMovesSinglePassIntoBuffer(moves, depth, previousMove, ttMove, ttHit, buffer[:], pos)

	// Return a copy of the ordered moves (only allocation, but tests need owned slice)
	result := make([]Move, count)
	copy(result, buffer[:count])
	return result
}

// Zero-allocation single-pass ordering into caller-provided buffer. Scores every move then
// fully sorts; used at the root and wherever the entire ordered list is consumed.
func (h *workerHistory) orderMovesSinglePassIntoBuffer(moves []Move, depth int, previousMove Move, ttMove Move, ttHit bool, out []Move, pos *Position) int {
	// Use a fixed-size array for scoring to avoid allocations
	var scores [256]int
	n, _ := h.scoreMovesIntoBuffer(moves, depth, previousMove, EmptyMove, ttMove, ttHit, out, scores[:], pos, false)
	for i := 0; i < n-1; i++ {
		selectNextMove(out, scores[:], n, i)
	}
	return n
}

// captureOrderScoreParts combines the SEE value and the capture-history
// tie-breaker into the capture ordering score: SEE-banded. Winning/equal
// captures sit at >= 80000 (above killers/counters); LOSING captures sit at
// ~-40000 — below every quiet (history sum bottoms out at -24576) — so a
// quiet with any history outranks a piece sacrifice that SEE already calls
// bad. All references order this way (Weiss/Ethereal/CounterGo/SF staged-gen).
// The history term stays secondary to SEE inside each band.
func captureOrderScoreParts(seeValue, chScore int) int {
	if seeValue >= 0 {
		return 80000 + seeValue + 1000 + chScore
	}
	return -40000 + seeValue + 1000 + chScore
}

// captureOrderScore is the eager form: both parts read at scoring time.
func (h *workerHistory) captureOrderScore(pos *Position, move Move) int {
	return captureOrderScoreParts(staticExchangeEvaluation(pos, move), h.captureHistoryScore(move)/8)
}

// unscoredCaptureBase marks a capture whose SEE swap loop was deferred (lazySEE).
// The capture-history tie-breaker is NOT deferred — it is read at scoring time and
// band-encoded next to the sentinel — because the table mutates as earlier moves'
// subtrees are searched (a deferred read would see fresher history than the eager
// path and change the order: not node-identical). SEE itself is a pure function of
// the position, which is unchanged at this node, so deferring it is exact.
// The band sits near -2^30, far below every real score (quiets bottom out near
// -24576; losing captures near -41000), so no real score can land in it.
const unscoredCaptureBase = -(1 << 30)

// materializeCaptureScores fills in the capture scores that
// scoreMovesIntoBuffer(lazySEE=true) deferred, running the SEE swap now and
// recombining it with the scoring-time history term parked in the sentinel band.
// It MUST run before the first selectNextMove comparison so selection never sees
// a sentinel — with that invariant the emitted order is identical to eager scoring.
func materializeCaptureScores(out []Move, scores []int, n int, pos *Position) {
	for i := 0; i < n; i++ {
		if s := scores[i]; s >= unscoredCaptureBase-(1<<16) && s <= unscoredCaptureBase+(1<<16) {
			chScore := s - unscoredCaptureBase
			scores[i] = captureOrderScoreParts(staticExchangeEvaluation(pos, out[i]), chScore)
		}
	}
}

// scoreMovesIntoBuffer writes each move and its ordering score into the parallel out/scores
// buffers WITHOUT sorting. The search drives the sort incrementally via selectNextMove so the
// O(n^2) selection tail past a beta cutoff is never paid. Returns the number of moves written
// and the index of the TT move within out[] (or -1 if absent), so the caller can hoist it to
// the front in O(1) rather than rescanning for it in selectNextMove(oi=0).
func (h *workerHistory) scoreMovesIntoBuffer(moves []Move, depth int, previousMove Move, prev2 Move, ttMove Move, ttHit bool, out []Move, scores []int, pos *Position, lazySEE bool) (int, int) {
	n := len(moves)
	if n == 0 || len(out) < n || len(scores) < n {
		return 0, -1
	}

	// Hoist the continuation-history sub-table for previousMove out of the per-move
	// loop (previousMove is constant for the whole list), so each quiet move pays one
	// 2-D index instead of the full 4-D continuationHistory index. Bit-identical to
	// the GetHistoryScore path below. Same hoist for the follow-up table on prev2.
	usePrev := previousMove != EmptyMove
	var contHist *[13][64]int
	if usePrev {
		contHist = &h.continuationHistory[previousMove.MovingPiece()][previousMove.Destination()]
	}
	usePrev2 := prev2 != EmptyMove
	var fuHist *[13][64]int
	if usePrev2 {
		fuHist = &h.followupHistory[prev2.MovingPiece()][prev2.Destination()]
	}

	// Hoist the loop-invariant counter move and killer slots out of the per-move loop.
	// previousMove and depth are constant for the whole list, so each quiet move becomes
	// two equality tests instead of re-indexing counterMoves[from][to] and the killer
	// table (with its bounds check) every iteration. EmptyMove never matches a real
	// generated quiet move, so the guards below are bit-identical to IsCounterMove /
	// IsKillerMove.
	counterMove := h.GetCounterMove(previousMove)
	var killer0, killer1 Move
	if depth >= 0 && depth < MaximumDepth {
		killer0 = h.killerMoves[depth][0]
		killer1 = h.killerMoves[depth][1]
	}

	// Single pass: score all moves. Record where the TT move lands so the caller can hoist
	// it to the front in O(1) — it always carries the unique top score (100000).
	ttIndex := -1
	for i, move := range moves {
		score := 0

		// TT move gets highest priority
		if ttHit && move == ttMove {
			score = 100000
			ttIndex = i
		} else {
			// Calculate score based on move type and SEE
			if move.IsCapture() || move.IsEnPassant() {
				if lazySEE {
					// Defer the SEE swap loop: the caller materializes the real
					// capture scores (materializeCaptureScores) immediately before
					// its FIRST selection compare, so nodes that cut on the hoisted
					// TT move never pay any ordering SEE at all. The history
					// tie-breaker is read NOW (the table mutates as earlier moves'
					// subtrees are searched; a deferred read would not be
					// node-identical) and parked in the sentinel band.
					score = unscoredCaptureBase + h.captureHistoryScore(move)/8
				} else {
					score = h.captureOrderScore(pos, move)
				}
			} else if move.PromoType() != NoType {
				promoWeight := int(GetPiece(move.PromoType(), move.MovingPiece().Color()).Weight())
				if move.PromoType() == Queen {
					// Non-capture queen promotion: gains ~800cp; rank above winning
					// captures (which max around 82000) so we explore it before any
					// capture or quiet alternative.
					score = 90000 + promoWeight
				} else {
					// Under-promotions are rare and usually inferior; keep them
					// in the original priority band, ordered by promoted piece value.
					score = 70000 + promoWeight
				}
			} else if move.IsCastle() {
				// Castling: moderate priority
				score = 50000
			} else {
				// Quiet moves: use heuristics
				score = 0

				// Counter moves get highest priority among quiet moves
				if move == counterMove && counterMove != EmptyMove {
					score += 30000
				}

				// Killer moves get high priority
				if move == killer0 || move == killer1 {
					score += 25000
				}

				// History heuristic for remaining quiet moves; continuation
				// history adds context-aware ordering when previousMove != EmptyMove.
				// Inlined GetHistoryScore using the hoisted contHist: this branch only
				// runs for quiet moves, so that function's capture/promo guard (which
				// would return 0) is always false here — identical result.
				piece := move.MovingPiece()
				to := move.Destination()
				hs := h.historyTable[piece][to]
				if usePrev {
					hs += contHist[piece][to]
				}
				if usePrev2 {
					hs += fuHist[piece][to]
				}
				score += hs
			}
		}

		scores[i] = score
		out[i] = move
	}

	return n, ttIndex
}

// selectNextMove performs one selection-sort step: it scans out[i:n] for the highest score
// and swaps that entry (and its score) into position i. Calling it for i = 0,1,2,... yields
// moves in the exact order a full selection sort produces, so the search can stop as soon as
// a beta cutoff fires instead of sorting the whole list. Strict > keeps ties at their earlier
// index, matching the full-sort order — so the incremental path is node-identical.
func selectNextMove(out []Move, scores []int, n, i int) {
	maxIdx := i
	maxScore := scores[i] // hoist out of the loop: avoids re-loading scores[maxIdx] each iteration
	for j := i + 1; j < n; j++ {
		if scores[j] > maxScore {
			maxIdx = j
			maxScore = scores[j]
		}
	}
	if maxIdx != i {
		scores[i], scores[maxIdx] = scores[maxIdx], scores[i]
		out[i], out[maxIdx] = out[maxIdx], out[i]
	}
}

func (h *workerHistory) orderMoves(moves []Move, pos *Position) []Move {
	return h.orderMovesWithDepth(moves, 0, pos)
}

func (h *workerHistory) orderMovesWithDepth(moves []Move, depth int, pos *Position) []Move {
	return h.orderMovesWithDepthAndPrevious(moves, depth, h.GetLastMovePlayed(), pos)
}

func (h *workerHistory) orderMovesWithDepthAndTTMove(moves []Move, depth int, ttMove Move, ttHit bool, pos *Position) []Move {
	return h.orderMovesWithDepthPreviousAndTTMove(moves, depth, h.GetLastMovePlayed(), ttMove, ttHit, pos)
}

func (h *workerHistory) orderMovesWithDepthPreviousAndTTMove(moves []Move, depth int, previousMove Move, ttMove Move, ttHit bool, pos *Position) []Move {
	return h.orderMovesSinglePass(moves, depth, previousMove, ttMove, ttHit, pos)
}

// Zero-allocation ordering into caller-provided buffer
// Returns number of moves written to out (equals len(moves))
func (h *workerHistory) orderMovesIntoBufferWithDepthPreviousAndTTMove(moves []Move, depth int, previousMove Move, ttMove Move, ttHit bool, out []Move, pos *Position) int {
	return h.orderMovesSinglePassIntoBuffer(moves, depth, previousMove, ttMove, ttHit, out, pos)
}

func (h *workerHistory) orderMovesIntoBufferWithDepth(moves []Move, depth int, out []Move, pos *Position) int {
	return h.orderMovesIntoBufferWithDepthPreviousAndTTMove(moves, depth, h.GetLastMovePlayed(), EmptyMove, false, out, pos)
}

func (h *workerHistory) orderMovesWithDepthAndPrevious(moves []Move, depth int, previousMove Move, pos *Position) []Move {
	return h.orderMovesWithDepthPreviousAndTTMove(moves, depth, previousMove, EmptyMove, false, pos)
}

// Compatibility wrappers retain the package API and its single default heuristic
// lifetime. Search hot paths use their SearchInfo owner directly.
func correctionValue(stm Color, idx int) int {
	return defaultSearchEngine.worker.history.correctionValue(stm, idx)
}
func correctedStandPat(pos *Position) int {
	defaultSearchEngine.sessionMu.Lock()
	defer defaultSearchEngine.sessionMu.Unlock()
	generation := mustAcquireHCEModelUse()
	defer releaseHCEModelUse()
	evaluator := defaultSearchEngine.mustPreparePrimaryEvaluator(pos, generation)
	return defaultSearchEngine.worker.history.correctedStandPat(pos, evaluator)
}
func updatePawnCorrection(stm Color, idx, diff, depth int) {
	defaultSearchEngine.worker.history.updatePawnCorrection(stm, idx, diff, depth)
}
func maybeUpdatePawnCorrection(stm Color, idx, staticEval, bestScore, beta, depth int, bestMove Move, inCheck bool) {
	defaultSearchEngine.worker.history.maybeUpdatePawnCorrection(stm, idx, staticEval, bestScore, beta, depth, bestMove, inCheck)
}
func nonPawnCorrectionValue(stm Color, idx int) int {
	return defaultSearchEngine.worker.history.nonPawnCorrectionValue(stm, idx)
}
func updateNonPawnCorrection(stm Color, idx, diff, depth int) {
	defaultSearchEngine.worker.history.updateNonPawnCorrection(stm, idx, diff, depth)
}
func maybeUpdateNonPawnCorrection(stm Color, idx, staticEval, bestScore, beta, depth int, bestMove Move, inCheck bool) {
	defaultSearchEngine.worker.history.maybeUpdateNonPawnCorrection(stm, idx, staticEval, bestScore, beta, depth, bestMove, inCheck)
}
func minorCorrectionValue(stm Color, idx int) int {
	return defaultSearchEngine.worker.history.minorCorrectionValue(stm, idx)
}
func updateMinorCorrection(stm Color, idx, diff, depth int) {
	defaultSearchEngine.worker.history.updateMinorCorrection(stm, idx, diff, depth)
}
func maybeUpdateMinorCorrection(stm Color, idx, staticEval, bestScore, beta, depth int, bestMove Move, inCheck bool) {
	defaultSearchEngine.worker.history.maybeUpdateMinorCorrection(stm, idx, staticEval, bestScore, beta, depth, bestMove, inCheck)
}
func UpdateHistoryTable(move Move, prevMove Move, prev2 Move, depth int) {
	defaultSearchEngine.worker.history.UpdateHistoryTable(move, prevMove, prev2, depth)
}
func PenalizeHistoryTable(move Move, prevMove Move, prev2 Move, depth int) {
	defaultSearchEngine.worker.history.PenalizeHistoryTable(move, prevMove, prev2, depth)
}
func GetHistoryScore(move Move, prevMove Move, prev2 Move) int {
	return defaultSearchEngine.worker.history.GetHistoryScore(move, prevMove, prev2)
}
func captureHistoryScore(move Move) int {
	return defaultSearchEngine.worker.history.captureHistoryScore(move)
}
func UpdateCaptureHistory(move Move, depth int) {
	defaultSearchEngine.worker.history.UpdateCaptureHistory(move, depth)
}
func PenalizeCaptureHistory(move Move, depth int) {
	defaultSearchEngine.worker.history.PenalizeCaptureHistory(move, depth)
}
func ClearHistoryTable() { defaultSearchEngine.ClearHistoryTable() }
func UpdateKillerMoves(move Move, ply int) {
	defaultSearchEngine.worker.history.UpdateKillerMoves(move, ply)
}
func IsKillerMove(move Move, ply int) bool {
	return defaultSearchEngine.worker.history.IsKillerMove(move, ply)
}
func ClearKillerMoves() { defaultSearchEngine.ClearKillerMoves() }
func UpdateCounterMove(counterMove Move, previousMove Move) {
	defaultSearchEngine.worker.history.UpdateCounterMove(counterMove, previousMove)
}
func GetCounterMove(previousMove Move) Move {
	return defaultSearchEngine.worker.history.GetCounterMove(previousMove)
}
func IsCounterMove(move Move, previousMove Move) bool {
	return defaultSearchEngine.worker.history.IsCounterMove(move, previousMove)
}
func SetLastMovePlayed(move Move) { defaultSearchEngine.SetLastMovePlayed(move) }
func GetLastMovePlayed() Move     { return defaultSearchEngine.GetLastMovePlayed() }
func ClearCounterMoves()          { defaultSearchEngine.ClearCounterMoves() }
func orderMovesSinglePass(moves []Move, depth int, previousMove Move, ttMove Move, ttHit bool, pos *Position) []Move {
	return defaultSearchEngine.worker.history.orderMovesSinglePass(moves, depth, previousMove, ttMove, ttHit, pos)
}
func orderMovesSinglePassIntoBuffer(moves []Move, depth int, previousMove Move, ttMove Move, ttHit bool, out []Move, pos *Position) int {
	return defaultSearchEngine.worker.history.orderMovesSinglePassIntoBuffer(moves, depth, previousMove, ttMove, ttHit, out, pos)
}
func captureOrderScore(pos *Position, move Move) int {
	return defaultSearchEngine.worker.history.captureOrderScore(pos, move)
}
func scoreMovesIntoBuffer(moves []Move, depth int, previousMove Move, prev2 Move, ttMove Move, ttHit bool, out []Move, scores []int, pos *Position, lazySEE bool) (int, int) {
	return defaultSearchEngine.worker.history.scoreMovesIntoBuffer(moves, depth, previousMove, prev2, ttMove, ttHit, out, scores, pos, lazySEE)
}
func orderMoves(moves []Move, pos *Position) []Move {
	return defaultSearchEngine.worker.history.orderMoves(moves, pos)
}
func orderMovesWithDepth(moves []Move, depth int, pos *Position) []Move {
	return defaultSearchEngine.worker.history.orderMovesWithDepth(moves, depth, pos)
}
func orderMovesWithDepthAndTTMove(moves []Move, depth int, ttMove Move, ttHit bool, pos *Position) []Move {
	return defaultSearchEngine.worker.history.orderMovesWithDepthAndTTMove(moves, depth, ttMove, ttHit, pos)
}
func orderMovesWithDepthPreviousAndTTMove(moves []Move, depth int, previousMove Move, ttMove Move, ttHit bool, pos *Position) []Move {
	return defaultSearchEngine.worker.history.orderMovesWithDepthPreviousAndTTMove(moves, depth, previousMove, ttMove, ttHit, pos)
}
func orderMovesIntoBufferWithDepthPreviousAndTTMove(moves []Move, depth int, previousMove Move, ttMove Move, ttHit bool, out []Move, pos *Position) int {
	return defaultSearchEngine.worker.history.orderMovesIntoBufferWithDepthPreviousAndTTMove(moves, depth, previousMove, ttMove, ttHit, out, pos)
}
func orderMovesIntoBufferWithDepth(moves []Move, depth int, out []Move, pos *Position) int {
	return defaultSearchEngine.worker.history.orderMovesIntoBufferWithDepth(moves, depth, out, pos)
}
func orderMovesWithDepthAndPrevious(moves []Move, depth int, previousMove Move, pos *Position) []Move {
	return defaultSearchEngine.worker.history.orderMovesWithDepthAndPrevious(moves, depth, previousMove, pos)
}
