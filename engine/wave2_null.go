package engine

// W2NullVerify keeps deep null cutoffs provisional until a real-position search
// also fails high. Adoption requires a completed strength gate; shipped off.
var W2NullVerify = 0

// W2NullMargin adds a bounded surplus reduction only with null verification.
// Adoption requires a completed strength gate; shipped off.
var W2NullMargin = 0

func wave2NullMargin(correctedStaticEval, beta int) int {
	if W2NullMargin == 0 || W2NullVerify == 0 ||
		correctedStaticEval == unknownStaticEval ||
		correctedStaticEval <= -MATE_IN_MAX || correctedStaticEval >= MATE_IN_MAX ||
		beta <= -MATE_IN_MAX || beta >= MATE_IN_MAX {
		return 0
	}
	return min(2, max(0, (correctedStaticEval-beta)/200))
}

func (info *SearchInfo) allowNullMove(alpha, beta int) bool {
	if info.nullVerification {
		return false
	}
	return W2NullVerify == 0 || (info.ExcludedMove == EmptyMove &&
		abs(alpha) < MATE_IN_MAX && abs(beta) < MATE_IN_MAX)
}

func (info *SearchInfo) searchNull(pos *Position, depth, ply, alpha, beta int, isPV, canNull, cutNode bool) int {
	if info.nullSearch != nil {
		return info.nullSearch(pos, depth, ply, alpha, beta, isPV, canNull, cutNode, info)
	}
	return alphaBetaPV(pos, depth, ply, alpha, beta, isPV, canNull, cutNode, info)
}

// Preserve path slots even when the speculative null subtree explores real moves.
// The default-off path keeps the original search without the array snapshots.
func (info *SearchInfo) searchNullChild(pos *Position, depth, ply, alpha, beta int, cutNode bool) int {
	if W2NullVerify == 0 {
		return info.searchNull(pos, depth, ply, alpha, beta, false, false, cutNode)
	}
	savedMoves, savedEvals := info.MoveStack, info.StaticEvalStack
	defer func() { info.MoveStack, info.StaticEvalStack = savedMoves, savedEvals }()
	return info.searchNull(pos, depth, ply, alpha, beta, false, false, cutNode)
}

// Called only after undoing the null move and restoring evaluator and predecessor.
func (info *SearchInfo) verifyNullCutoff(pos *Position, depth, ply, beta int, cutNode bool) bool {
	if W2NullVerify == 0 || depth < 8 {
		return true
	}
	verificationDepth := depth - NULL_MOVE_R
	if verificationDepth < 1 {
		verificationDepth = 1
	}

	// Same-ply re-entry must not see its own repetition entry. Save live path
	// context as well: this search can overwrite the outer node's static eval and
	// move slots. Suppression covers every descendant, even when canNull is true.
	savedSuppression := info.nullVerification
	savedRepLen := info.RepStackLen
	savedMoves, savedEvals := info.MoveStack, info.StaticEvalStack
	savedLast := info.history.GetLastMovePlayed()
	defer func() {
		info.nullVerification = savedSuppression
		info.RepStackLen = savedRepLen
		info.MoveStack, info.StaticEvalStack = savedMoves, savedEvals
		info.history.SetLastMovePlayed(savedLast)
	}()
	info.nullVerification = true
	info.RepStackLen--
	info.NullVerifySearches++
	score := info.searchNull(pos, verificationDepth, ply, beta-1, beta, false, false, cutNode)
	if info.Stopped {
		return false
	}
	if score >= beta {
		info.NullVerifyCutoffs++
		return true
	}
	info.NullVerifyFailLows++
	return false
}
