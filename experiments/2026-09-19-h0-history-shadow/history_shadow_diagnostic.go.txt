package engine

const (
	h0NormalizedHistogramBuckets = 81
	h0NormalizedHistogramStepBP  = 250
	h0RankHistogramBuckets       = 7
)

// H0HistoryShadowCounters is the disposable passive-observer receipt. The
// observer is copied into an isolated worktree and is never linked into a
// release build.
type H0HistoryShadowCounters struct {
	CompletedInteriorNodes  uint64 `json:"completed_interior_nodes"`
	CompletedRootAttempts   uint64 `json:"completed_root_attempts"`
	QuietWinnerLabels       uint64 `json:"quiet_winner_labels"`
	QuietCutoffLabels       uint64 `json:"quiet_cutoff_labels"`
	QuietExactLabels        uint64 `json:"quiet_exact_labels"`
	RootQuietWinnerLabels   uint64 `json:"root_quiet_winner_labels"`
	MissedPVQuietLabels     uint64 `json:"missed_pv_quiet_labels"`
	CurrentEquivalentLabels uint64 `json:"current_equivalent_labels"`
	NewOnlyLabels           uint64 `json:"new_only_labels"`
	SingularQuietLabels     uint64 `json:"singular_quiet_labels"`
	NoisyAlphaRaiseNodes    uint64 `json:"noisy_alpha_raise_nodes"`
	NoisyCutoffNodes        uint64 `json:"noisy_cutoff_nodes"`
	WinnerMissingFromPrefix uint64 `json:"winner_missing_from_prefix"`

	PositiveMoves                 uint64 `json:"positive_moves"`
	NegativePrefixMoves           uint64 `json:"negative_prefix_moves"`
	PositiveRateUnits             uint64 `json:"positive_rate_units"`
	NegativeRateUnits             uint64 `json:"negative_rate_units"`
	NewOnlyPositiveRateUnits      uint64 `json:"new_only_positive_rate_units"`
	CurrentEquivalentRateUnits    uint64 `json:"current_equivalent_rate_units"`
	SuppressedNoisyNegativeMoves  uint64 `json:"suppressed_noisy_negative_moves"`
	SuppressedNoisyRateUnits      uint64 `json:"suppressed_noisy_rate_units"`
	ShadowEntryUpdates            uint64 `json:"shadow_entry_updates"`
	ShadowEntryAbsoluteDelta      uint64 `json:"shadow_entry_absolute_delta"`
	MainLabelUpdates              uint64 `json:"main_label_updates"`
	MainUpdatesOnMultiOriginSlots uint64 `json:"main_updates_on_multi_origin_slots"`
	MainCrossOriginSignConflicts  uint64 `json:"main_cross_origin_sign_conflicts"`

	QuietWinnerRankSamples              uint64                         `json:"quiet_winner_rank_samples"`
	LateQuietCutoffRankSamples          uint64                         `json:"late_quiet_cutoff_rank_samples"`
	LateQuietCutoffRankImproved         uint64                         `json:"late_quiet_cutoff_rank_improved"`
	LateQuietCutoffRankEqual            uint64                         `json:"late_quiet_cutoff_rank_equal"`
	LateQuietCutoffRankWorsened         uint64                         `json:"late_quiet_cutoff_rank_worsened"`
	LateQuietCutoffCurrentRankSum       uint64                         `json:"late_quiet_cutoff_current_rank_sum"`
	LateQuietCutoffShadowRankSum        uint64                         `json:"late_quiet_cutoff_shadow_rank_sum"`
	LateQuietCutoffRateWeight           uint64                         `json:"late_quiet_cutoff_rate_weight"`
	LateQuietCutoffWeightedRankDelta    int64                          `json:"late_quiet_cutoff_weighted_rank_delta"`
	LateQuietCutoffCurrentRankHistogram [h0RankHistogramBuckets]uint64 `json:"late_quiet_cutoff_current_rank_histogram"`
	LateQuietCutoffShadowRankHistogram  [h0RankHistogramBuckets]uint64 `json:"late_quiet_cutoff_shadow_rank_histogram"`

	HistoryPruneEligible             uint64 `json:"history_prune_eligible"`
	BaselineHistoryPrunes            uint64 `json:"baseline_history_prunes"`
	ShadowHistoryPrunes              uint64 `json:"shadow_history_prunes"`
	BaselinePruneShadowRescues       uint64 `json:"baseline_prune_shadow_rescues"`
	ShadowOnlyPrunes                 uint64 `json:"shadow_only_prunes"`
	ShadowOnlyPruneIllegalMoves      uint64 `json:"shadow_only_prune_illegal_moves"`
	ShadowOnlyPruneCompletedSearches uint64 `json:"shadow_only_prune_completed_searches"`
	ShadowOnlyPruneAlphaRaisers      uint64 `json:"shadow_only_prune_alpha_raisers"`
	ShadowOnlyPruneBetaCutoffs       uint64 `json:"shadow_only_prune_beta_cutoffs"`
	ShadowOnlyPruneBestMoves         uint64 `json:"shadow_only_prune_best_moves"`

	LMRDecisions                    uint64                               `json:"lmr_decisions"`
	LMRBaselineRecomputeMismatches  uint64                               `json:"lmr_baseline_recompute_mismatches"`
	LMRPolicy10Changed              uint64                               `json:"lmr_policy10_changed"`
	LMRPolicy01Changed              uint64                               `json:"lmr_policy01_changed"`
	LMRPolicy11Changed              uint64                               `json:"lmr_policy11_changed"`
	LMRPolicy11LessReduction        uint64                               `json:"lmr_policy11_less_reduction"`
	LMRPolicy11MoreReduction        uint64                               `json:"lmr_policy11_more_reduction"`
	LMRPolicy11WouldRemoveReduction uint64                               `json:"lmr_policy11_would_remove_reduction"`
	LMRPolicy11WouldAddReduction    uint64                               `json:"lmr_policy11_would_add_reduction"`
	LMRActualReducedSearches        uint64                               `json:"lmr_actual_reduced_searches"`
	LMRActualReducedFailHighs       uint64                               `json:"lmr_actual_reduced_fail_highs"`
	LMRVerificationConfirmed        uint64                               `json:"lmr_verification_confirmed"`
	LMRVerificationRefuted          uint64                               `json:"lmr_verification_refuted"`
	LMRVerificationBetaCutoffs      uint64                               `json:"lmr_verification_beta_cutoffs"`
	LMRPolicy11ChangedFailHighs     uint64                               `json:"lmr_policy11_changed_fail_highs"`
	LMRCurrentTermHistogram         [5]uint64                            `json:"lmr_current_term_histogram"`
	LMRPolicy10TermHistogram        [5]uint64                            `json:"lmr_policy10_term_histogram"`
	LMRPolicy01TermHistogram        [5]uint64                            `json:"lmr_policy01_term_histogram"`
	LMRPolicy11TermHistogram        [5]uint64                            `json:"lmr_policy11_term_histogram"`
	LMRPolicy11ReductionDelta       [7]uint64                            `json:"lmr_policy11_reduction_delta_histogram"`
	CurrentNormalizedHistogram      [h0NormalizedHistogramBuckets]uint64 `json:"current_normalized_histogram"`
	ShadowNormalizedHistogram       [h0NormalizedHistogramBuckets]uint64 `json:"shadow_normalized_histogram"`
}

type H0HistoryShadowTableSummary struct {
	Nonzero   uint64 `json:"nonzero"`
	Positive  uint64 `json:"positive"`
	Negative  uint64 `json:"negative"`
	Saturated uint64 `json:"saturated"`
	Minimum   int    `json:"minimum"`
	Maximum   int    `json:"maximum"`
}

type H0HistoryShadowTableState struct {
	Main                      H0HistoryShadowTableSummary `json:"main"`
	Continuation              H0HistoryShadowTableSummary `json:"continuation"`
	Followup                  H0HistoryShadowTableSummary `json:"followup"`
	MainSlotsTouched          uint64                      `json:"main_slots_touched"`
	MainSlotsWithManyOrigins  uint64                      `json:"main_slots_with_many_origins"`
	MainSlotsWithSignConflict uint64                      `json:"main_slots_with_sign_conflict"`
}

type H0HistoryShadowSnapshot struct {
	Counters H0HistoryShadowCounters   `json:"counters"`
	Tables   H0HistoryShadowTableState `json:"tables"`
}

type h0HistoryShadowObserver struct {
	main         [13][64]int
	continuation [13][64][13][64]int
	followup     [13][64][13][64]int
	mainOrigins  [13][64]uint64
	mainPositive [13][64]uint64
	mainNegative [13][64]uint64
	counters     H0HistoryShadowCounters
}

type h0HistoryShadowMoveScore struct {
	move    Move
	current int
	shadow  int
}

type h0HistoryShadowNode struct {
	count int
	prev  Move
	prev2 Move
	moves [256]h0HistoryShadowMoveScore
}

type h0HistoryShadowLMRObservation struct {
	active          bool
	actualReduction int
	policy11        int
}

func (h *workerHistory) enableH0HistoryShadow() {
	h.h0Shadow = new(h0HistoryShadowObserver)
}

func (h *workerHistory) h0HistoryShadowSnapshotAndResetCounters() H0HistoryShadowSnapshot {
	if h == nil || h.h0Shadow == nil {
		return H0HistoryShadowSnapshot{}
	}
	o := h.h0Shadow
	snapshot := H0HistoryShadowSnapshot{Counters: o.counters, Tables: o.tableState()}
	o.counters = H0HistoryShadowCounters{}
	return snapshot
}

func (o *h0HistoryShadowObserver) clearTables() {
	if o == nil {
		return
	}
	clear(o.main[:])
	clear(o.continuation[:])
	clear(o.followup[:])
	clear(o.mainOrigins[:])
	clear(o.mainPositive[:])
	clear(o.mainNegative[:])
}

func (o *h0HistoryShadowObserver) captureNode(node *h0HistoryShadowNode, moves []Move, current *workerHistory, prev, prev2 Move) {
	if o == nil || node == nil {
		return
	}
	node.count = 0
	node.prev = prev
	node.prev2 = prev2
	for _, move := range moves {
		if move.IsCapture() || move.PromoType() != NoType || node.count >= len(node.moves) {
			continue
		}
		node.moves[node.count] = h0HistoryShadowMoveScore{
			move: move, current: current.GetHistoryScore(move, prev, prev2), shadow: o.score(move, prev, prev2),
		}
		node.count++
	}
}

func (o *h0HistoryShadowObserver) score(move, prev, prev2 Move) int {
	if o == nil || move.IsCapture() || move.PromoType() != NoType {
		return 0
	}
	piece := move.MovingPiece()
	to := move.Destination()
	score := o.main[piece][to]
	if prev != EmptyMove {
		score += o.continuation[prev.MovingPiece()][prev.Destination()][piece][to]
	}
	if prev2 != EmptyMove {
		score += o.followup[prev2.MovingPiece()][prev2.Destination()][piece][to]
	}
	return score
}

func h0HistoryRate(depth int) int {
	if depth < 0 {
		depth = -depth
	}
	rate := depth * depth
	if rate > 400 {
		rate = 400
	}
	return rate
}

func h0HistoryResponse(entry, target, rate int) int {
	return entry + int((int64(target)-int64(entry))*int64(rate)/512)
}

func (o *h0HistoryShadowObserver) updateEntry(entry *int, target, rate int) {
	before := *entry
	*entry = h0HistoryResponse(before, target, rate)
	delta := *entry - before
	if delta < 0 {
		delta = -delta
	}
	o.counters.ShadowEntryUpdates++
	o.counters.ShadowEntryAbsoluteDelta += uint64(delta)
}

func (o *h0HistoryShadowObserver) updateMove(move, prev, prev2 Move, depth, target int) {
	if o == nil || move.IsCapture() || move.PromoType() != NoType {
		return
	}
	rate := h0HistoryRate(depth)
	piece := move.MovingPiece()
	to := move.Destination()
	originBit := uint64(1) << uint(move.Source())
	otherOrigins := o.mainOrigins[piece][to] &^ originBit
	if otherOrigins != 0 {
		o.counters.MainUpdatesOnMultiOriginSlots++
	}
	if target > 0 {
		if o.mainNegative[piece][to]&^originBit != 0 {
			o.counters.MainCrossOriginSignConflicts++
		}
		o.mainPositive[piece][to] |= originBit
	} else {
		if o.mainPositive[piece][to]&^originBit != 0 {
			o.counters.MainCrossOriginSignConflicts++
		}
		o.mainNegative[piece][to] |= originBit
	}
	o.mainOrigins[piece][to] |= originBit
	o.counters.MainLabelUpdates++
	o.updateEntry(&o.main[piece][to], target, rate)
	if prev != EmptyMove {
		o.updateEntry(&o.continuation[prev.MovingPiece()][prev.Destination()][piece][to], target, rate)
	}
	if prev2 != EmptyMove {
		o.updateEntry(&o.followup[prev2.MovingPiece()][prev2.Destination()][piece][to], target, rate)
	}
}

func h0QuietMove(move Move) bool {
	return move != EmptyMove && !move.IsCapture() && move.PromoType() == NoType
}

func (o *h0HistoryShadowObserver) observeCompletedNode(
	node *h0HistoryShadowNode,
	searchedQuiets []Move,
	bestMove Move,
	depth, origAlpha, bestScore, beta, legalTried int,
	isPV, isRoot, inSingular bool,
) {
	if o == nil || node == nil {
		return
	}
	if isRoot {
		o.counters.CompletedRootAttempts++
	} else {
		o.counters.CompletedInteriorNodes++
	}
	if bestScore <= origAlpha || bestMove == EmptyMove {
		return
	}
	rate := h0HistoryRate(depth)
	if !h0QuietMove(bestMove) {
		o.counters.NoisyAlphaRaiseNodes++
		if bestScore >= beta {
			o.counters.NoisyCutoffNodes++
			o.counters.SuppressedNoisyNegativeMoves += uint64(len(searchedQuiets))
			o.counters.SuppressedNoisyRateUnits += uint64(rate * len(searchedQuiets))
		}
		return
	}

	o.counters.QuietWinnerLabels++
	newOnly := isRoot || bestScore < beta
	if bestScore >= beta {
		o.counters.QuietCutoffLabels++
		if !isRoot {
			o.counters.CurrentEquivalentLabels++
			o.counters.CurrentEquivalentRateUnits += uint64(rate)
		}
	} else {
		o.counters.QuietExactLabels++
	}
	if isRoot {
		o.counters.RootQuietWinnerLabels++
	}
	if !isRoot && isPV && bestScore < beta {
		o.counters.MissedPVQuietLabels++
	}
	if newOnly {
		o.counters.NewOnlyLabels++
		o.counters.NewOnlyPositiveRateUnits += uint64(rate)
	}
	if inSingular {
		o.counters.SingularQuietLabels++
	}

	currentRank, shadowRank, foundRank := node.ranks(bestMove)
	if foundRank {
		o.counters.QuietWinnerRankSamples++
		if bestScore >= beta && legalTried >= 2 {
			o.counters.LateQuietCutoffRankSamples++
			o.counters.LateQuietCutoffCurrentRankSum += uint64(currentRank)
			o.counters.LateQuietCutoffShadowRankSum += uint64(shadowRank)
			o.counters.LateQuietCutoffRateWeight += uint64(rate)
			o.counters.LateQuietCutoffWeightedRankDelta += int64(currentRank-shadowRank) * int64(rate)
			o.counters.LateQuietCutoffCurrentRankHistogram[h0RankBucket(currentRank)]++
			o.counters.LateQuietCutoffShadowRankHistogram[h0RankBucket(shadowRank)]++
			switch {
			case shadowRank < currentRank:
				o.counters.LateQuietCutoffRankImproved++
			case shadowRank > currentRank:
				o.counters.LateQuietCutoffRankWorsened++
			default:
				o.counters.LateQuietCutoffRankEqual++
			}
		}
	}

	winnerIndex := -1
	for index, move := range searchedQuiets {
		if move == bestMove {
			winnerIndex = index
			break
		}
	}
	if winnerIndex < 0 {
		o.counters.WinnerMissingFromPrefix++
		return
	}
	o.counters.PositiveMoves++
	o.counters.PositiveRateUnits += uint64(rate)
	o.updateMove(bestMove, node.prev, node.prev2, depth, historyMax)
	for _, move := range searchedQuiets[:winnerIndex] {
		o.counters.NegativePrefixMoves++
		o.counters.NegativeRateUnits += uint64(rate)
		o.updateMove(move, node.prev, node.prev2, depth, -historyMax)
	}
}

func (node *h0HistoryShadowNode) lookup(move Move) (current, shadow int, ok bool) {
	if node == nil {
		return 0, 0, false
	}
	for index := 0; index < node.count; index++ {
		if node.moves[index].move == move {
			entry := node.moves[index]
			return entry.current, entry.shadow, true
		}
	}
	return 0, 0, false
}

func (node *h0HistoryShadowNode) ranks(move Move) (currentRank, shadowRank int, ok bool) {
	targetIndex := -1
	for index := 0; index < node.count; index++ {
		if node.moves[index].move == move {
			targetIndex = index
			break
		}
	}
	if targetIndex < 0 {
		return 0, 0, false
	}
	target := node.moves[targetIndex]
	currentRank, shadowRank = 1, 1
	for index := 0; index < node.count; index++ {
		entry := node.moves[index]
		if entry.current > target.current || (entry.current == target.current && index < targetIndex) {
			currentRank++
		}
		if entry.shadow > target.shadow || (entry.shadow == target.shadow && index < targetIndex) {
			shadowRank++
		}
	}
	return currentRank, shadowRank, true
}

func h0RankBucket(rank int) int {
	switch {
	case rank <= 1:
		return 0
	case rank == 2:
		return 1
	case rank == 3:
		return 2
	case rank == 4:
		return 3
	case rank <= 8:
		return 4
	case rank <= 16:
		return 5
	default:
		return 6
	}
}

func h0CurrentHistoryTerm(score int) int {
	if score < -500 {
		return 1
	}
	if score > 1000 {
		return -1
	}
	return 0
}

func h0NormalizedHistoryTerm(score int) int {
	value := score / 2500
	if value < -2 {
		value = -2
	} else if value > 2 {
		value = 2
	}
	return -value
}

func h0FinalizeReduction(reduction, depth int, nodeTTPv, killer bool) int {
	if nodeTTPv {
		reduction--
		if reduction < 0 {
			reduction = 0
		}
	}
	if killer {
		reduction--
		if reduction < 0 {
			reduction = 0
		}
	}
	if reduction >= depth-1 {
		reduction = depth - 2
	}
	if reduction < 0 {
		reduction = 0
	}
	return reduction
}

func (o *h0HistoryShadowObserver) observeLMRDecision(
	node *h0HistoryShadowNode,
	move Move,
	currentScore, baseReduction, actualReduction, depth int,
	nodeTTPv, killer bool,
) h0HistoryShadowLMRObservation {
	if o == nil || node == nil || !h0QuietMove(move) {
		return h0HistoryShadowLMRObservation{}
	}
	shadowScore := o.score(move, node.prev, node.prev2)
	term00 := h0CurrentHistoryTerm(currentScore)
	term10 := h0CurrentHistoryTerm(shadowScore)
	term01 := h0NormalizedHistoryTerm(currentScore)
	term11 := h0NormalizedHistoryTerm(shadowScore)
	policy00 := h0FinalizeReduction(baseReduction+term00, depth, nodeTTPv, killer)
	policy10 := h0FinalizeReduction(baseReduction+term10, depth, nodeTTPv, killer)
	policy01 := h0FinalizeReduction(baseReduction+term01, depth, nodeTTPv, killer)
	policy11 := h0FinalizeReduction(baseReduction+term11, depth, nodeTTPv, killer)
	o.counters.LMRDecisions++
	if policy00 != actualReduction {
		o.counters.LMRBaselineRecomputeMismatches++
	}
	if policy10 != actualReduction {
		o.counters.LMRPolicy10Changed++
	}
	if policy01 != actualReduction {
		o.counters.LMRPolicy01Changed++
	}
	if policy11 != actualReduction {
		o.counters.LMRPolicy11Changed++
	}
	if policy11 < actualReduction {
		o.counters.LMRPolicy11LessReduction++
	} else if policy11 > actualReduction {
		o.counters.LMRPolicy11MoreReduction++
	}
	if actualReduction > 0 && policy11 == 0 {
		o.counters.LMRPolicy11WouldRemoveReduction++
	}
	if actualReduction == 0 && policy11 > 0 {
		o.counters.LMRPolicy11WouldAddReduction++
	}
	o.counters.LMRCurrentTermHistogram[term00+2]++
	o.counters.LMRPolicy10TermHistogram[term10+2]++
	o.counters.LMRPolicy01TermHistogram[term01+2]++
	o.counters.LMRPolicy11TermHistogram[term11+2]++
	delta := policy11 - actualReduction
	if delta < -3 {
		delta = -3
	} else if delta > 3 {
		delta = 3
	}
	o.counters.LMRPolicy11ReductionDelta[delta+3]++
	terms := 1
	if node.prev != EmptyMove {
		terms++
	}
	if node.prev2 != EmptyMove {
		terms++
	}
	o.counters.CurrentNormalizedHistogram[h0NormalizedBucket(currentScore, terms)]++
	o.counters.ShadowNormalizedHistogram[h0NormalizedBucket(shadowScore, terms)]++
	return h0HistoryShadowLMRObservation{active: true, actualReduction: actualReduction, policy11: policy11}
}

func h0NormalizedBucket(score, terms int) int {
	if terms < 1 {
		terms = 1
	}
	basisPoints := score * 10000 / (historyMax * terms)
	if basisPoints < -10000 {
		basisPoints = -10000
	} else if basisPoints > 10000 {
		basisPoints = 10000
	}
	index := (basisPoints + 10000) / h0NormalizedHistogramStepBP
	if index < 0 {
		return 0
	}
	if index >= h0NormalizedHistogramBuckets {
		return h0NormalizedHistogramBuckets - 1
	}
	return index
}

func (o *h0HistoryShadowObserver) observeLMROutcome(observation h0HistoryShadowLMRObservation, firstScore, finalScore, alpha, beta int) {
	if o == nil || !observation.active || observation.actualReduction <= 0 {
		return
	}
	o.counters.LMRActualReducedSearches++
	if firstScore <= alpha {
		return
	}
	o.counters.LMRActualReducedFailHighs++
	if observation.policy11 != observation.actualReduction {
		o.counters.LMRPolicy11ChangedFailHighs++
	}
	if finalScore > alpha {
		o.counters.LMRVerificationConfirmed++
		if finalScore >= beta {
			o.counters.LMRVerificationBetaCutoffs++
		}
	} else {
		o.counters.LMRVerificationRefuted++
	}
}

func (o *h0HistoryShadowObserver) observeHistoryPrune(node *h0HistoryShadowNode, move Move, currentScore, legalTried int) (baselinePrune, shadowOnly bool) {
	if o == nil || legalTried <= 3 {
		return currentScore < -1000 && legalTried > 3, false
	}
	if node == nil || !h0QuietMove(move) {
		return currentScore < -1000, false
	}
	shadowScore := o.score(move, node.prev, node.prev2)
	baselinePrune = currentScore < -1000
	shadowPrune := shadowScore < -1000
	o.counters.HistoryPruneEligible++
	if baselinePrune {
		o.counters.BaselineHistoryPrunes++
	}
	if shadowPrune {
		o.counters.ShadowHistoryPrunes++
	}
	if baselinePrune && !shadowPrune {
		o.counters.BaselinePruneShadowRescues++
	}
	if shadowPrune && !baselinePrune {
		o.counters.ShadowOnlyPrunes++
		shadowOnly = true
	}
	return baselinePrune, shadowOnly
}

func (o *h0HistoryShadowObserver) observeShadowPruneIllegal(shadowOnly bool) {
	if o != nil && shadowOnly {
		o.counters.ShadowOnlyPruneIllegalMoves++
	}
}

func (o *h0HistoryShadowObserver) observeShadowPruneOutcome(shadowOnly bool, score, alpha, beta int, becameBest bool) {
	if o == nil || !shadowOnly {
		return
	}
	o.counters.ShadowOnlyPruneCompletedSearches++
	if score > alpha {
		o.counters.ShadowOnlyPruneAlphaRaisers++
	}
	if score >= beta {
		o.counters.ShadowOnlyPruneBetaCutoffs++
	}
	if becameBest {
		o.counters.ShadowOnlyPruneBestMoves++
	}
}

func (o *h0HistoryShadowObserver) tableState() H0HistoryShadowTableState {
	var state H0HistoryShadowTableState
	for piece := range o.main {
		for square, value := range o.main[piece] {
			updateH0TableSummary(&state.Main, value)
			origins := o.mainOrigins[piece][square]
			if origins != 0 {
				state.MainSlotsTouched++
				if origins&(origins-1) != 0 {
					state.MainSlotsWithManyOrigins++
				}
				if o.mainPositive[piece][square] != 0 && o.mainNegative[piece][square] != 0 {
					state.MainSlotsWithSignConflict++
				}
			}
		}
	}
	for previousPiece := range o.continuation {
		for previousTo := range o.continuation[previousPiece] {
			for piece := range o.continuation[previousPiece][previousTo] {
				for _, value := range o.continuation[previousPiece][previousTo][piece] {
					updateH0TableSummary(&state.Continuation, value)
				}
			}
		}
	}
	for previousPiece := range o.followup {
		for previousTo := range o.followup[previousPiece] {
			for piece := range o.followup[previousPiece][previousTo] {
				for _, value := range o.followup[previousPiece][previousTo][piece] {
					updateH0TableSummary(&state.Followup, value)
				}
			}
		}
	}
	return state
}

func updateH0TableSummary(summary *H0HistoryShadowTableSummary, value int) {
	if value == 0 {
		return
	}
	if summary.Nonzero == 0 || value < summary.Minimum {
		summary.Minimum = value
	}
	if summary.Nonzero == 0 || value > summary.Maximum {
		summary.Maximum = value
	}
	summary.Nonzero++
	if value > 0 {
		summary.Positive++
	} else {
		summary.Negative++
	}
	if value == historyMax || value == -historyMax {
		summary.Saturated++
	}
}
