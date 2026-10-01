# NGN Engine Diagnosis: Why We're Stuck at 1400

**Date:** 2026-02-05
**Current ELO:** ~1400-1500 (65% vs Stockfish 1320)
**Expected ELO with implemented features:** 2000-2200+
**Gap explanation:** Critical bugs in search, broken heuristics, noisy evaluation

---

## Critical Bugs (Each one costs 50-200+ ELO)

### 0. No Draw Detection Inside the Search Tree (search.go:alphaBetaPV)

**Bug:** The recursive search function `alphaBetaPV` NEVER checks for draw by repetition or 50-move rule. Draw detection only happens at the root (line 475). The search uses `pos.MakeMove()` which doesn't update position history.

**Impact:** This is potentially the single most damaging bug. The engine:
- Cannot see that a line leads to draw by repetition
- Walks into drawn positions when winning
- Fails to find drawing resources when losing
- Plays aimlessly in many endgame scenarios

Every serious engine checks `if (isRepetition() || halfMoveClock >= 100) return 0;` at the start of each search node.

**Fix:** Check for repetition at the start of `alphaBetaPV`. This requires either using `GameMakeMove` in the search (expensive) or maintaining a lightweight repetition table.

---

### 1. TT Cutoffs in PV Nodes (search.go:666-670)

**Bug:** Returns exact TT scores in PV nodes without checking `isPV`.

```go
if ttHit && ttDepth >= int8(depth) {
    switch ttNodeType {
    case Exact:
        return int(ttEval)  // BUG: should check !isPV first
```

**Impact:** Corrupts the principal variation. The engine may return a cached score from a different search window, leading to incorrect best-move selection. This is one of the most well-known chess engine bugs.

**Fix:** Add `!isPV` guard: `if ttHit && !isPV && ttDepth >= int8(depth)`

---

### 2. SEE Pawn Bug (search.go:220-222)

**Bug:** `canPieceAttackSquare` returns `false` for pawns. Since `findLeastValuableAttacker` calls this function to verify attackers, pawns are NEVER selected as subsequent attackers in exchanges.

```go
case Pawn:
    return false // "We don't use this for pawns in SEE" -- BUT WE DO
```

**Impact:** SEE values are wrong for most exchanges. Example: If white plays BxN and black can recapture with a pawn, SEE won't see the pawn recapture. This means:
- Good captures get pruned (SEE thinks they lose material)
- Bad captures survive (SEE thinks they're safe)
- Affects both main search pruning and quiescence pruning

**Fix:** Implement proper pawn attack checking in `canPieceAttackSquare`.

---

### 3. Counter Move / Previous Move is Broken (moveorder.go:16, search.go:964, 1119)

**Bug:** `lastMovePlayed` is a **global variable** set only from the UCI handler. It's NEVER updated during search tree traversal.

```go
// Only called from uci.go, never from search:
func SetLastMovePlayed(move Move) {
    lastMovePlayed = move
}
```

**Impact:**
- **Counter move heuristic is 100% broken** - always looks up the same move
- **Recapture extensions** (search.go:964-970) compare against the wrong move
- **Move ordering** passes stale data to `orderMovesIntoBufferWithDepthPreviousAndTTMove`

**Fix:** Pass the previous move through the search recursion as a parameter.

---

### 4. moveCount Includes Illegal Moves (search.go:865 vs 941)

**Bug:** `moveCount++` is incremented at line 865 BEFORE the legality check at line 941. LMR and LMP use `moveCount` for thresholds.

```go
moveCount++          // line 865 - counted before legality check
// ... pruning decisions based on moveCount ...
ep, tag, hc, _ := pos.MakeMove(move)
if isInCheck(pos, movingColor) {   // line 941 - legality check
    pos.UnMakeMove(move, tag, ep, hc)
    continue  // illegal, but moveCount already incremented
}
legalTried++         // line 945 - the CORRECT counter
```

**Impact:** LMR triggers 2-4 moves too early. LMP prunes legal moves that should be searched. In a typical position with ~30 pseudo-legal moves where 5-8 are illegal, this shifts all thresholds.

**Fix:** Use `legalTried` for LMR/LMP thresholds instead of `moveCount`.

---

### 5. LMR Minimum Reduction Forced to 1 (search.go:1068)

**Bug:** After killer/history adjustments can reduce `reduction` to 0, it's clamped back to 1.

```go
if reduction < 1 {
    reduction = 1  // Forces minimum reduction even for killer moves
}
```

**Impact:** Killer moves with good history still get reduced. This defeats the purpose of the adjustment. The engine searches moves it knows are good at reduced depth.

**Fix:** Allow `reduction = 0` (effectively no reduction for good moves).

---

### 6. Extension Guardrail Kills Shallow Extensions (search.go:1006-1008)

**Bug:** `if nextDepth >= depth { nextDepth = depth - 1 }` prevents extensions from ever exceeding the original depth.

**Impact:** At depth=1, check extensions are disabled (nextDepth goes 0->1->0). At depth=2, only one extension can apply. Extensions are most valuable at shallow depths for finding tactical shots.

**Fix:** Allow controlled extension (e.g., limit total extensions to +2 or use an extension budget).

---

### 7. Check Extension Applied Twice (search.go:957-961 and 1080-1082)

**Bug:** Check extension modifies `nextDepth` at line 958, then the LMR code also extends for checks in `reducedDepth` at line 1080.

**Impact:** Check sequences get double-extended, causing search explosion. This wastes time on long checking sequences that don't lead to anything.

**Fix:** Only apply check extension in one place.

---

### 8. isInCheck Pawn File Masks Swapped for White King (movegen.go:792-793)

**Bug:** The file-wrapping masks for detecting black pawn checks on the white king are reversed.

```go
// BUG: FileA should be FileH, FileH should be FileA
leftAttack := (SquareMask[kingSquare] << 7) & ^FileMasks[FileA]  // wraps A→H, should mask FileH
rightAttack := (SquareMask[kingSquare] << 9) & ^FileMasks[FileH] // wraps H→A, should mask FileA
```

Compare with move generation (correct): `(whitePawns & ^FileMasks[FileA]) << 7`
Compare with black king case (correct): `(SquareMask[kingSquare] >> 7) & ^FileMasks[FileA]`

**Impact:** White king on FileA/FileH can get false "in check" from pawns on the opposite file. This corrupts legality testing everywhere - moves that are legal get rejected. Rare but real.

---

### 9. Losing Captures Ordered Above Killer/Counter Moves (moveorder.go:197-199)

**Bug:** Losing captures get base score ~60000, which is above killers (~25000-30000).

**Impact:** The engine wastes time searching known-bad captures before trying proven good quiet moves.

---

### 10. Futility Pruning Doesn't Exclude TT Move (search.go:879)

**Bug:** Futility pruning skips ALL quiet moves, including the TT move (the best move from a previous search).

```go
if futilityPrune && !move.IsCapture() {
    continue  // Prunes TT move too!
}
```

**Impact:** The most important move in the position gets pruned. Should be:
`if futilityPrune && !move.IsCapture() && move != ttMove`

---

## Evaluation Bugs

### 8. Knight PST Penalizes Development (eval.go:44-46)

The knight PST assigns penalties to f3/c3/f6/c6 (the natural developing squares):
- Rank 3 (f3/c3): -15 to -5 (should be +5 to +15)
- Rank 2 (starting rank area): -25 to -15

Standard CPW values have +5 to +15 for these squares. The engine actively discourages correct knight development.

### 9. Outpost Detection Backwards (eval.go ~line 1808)

The check for whether enemy pawns can threaten an outpost uses the wrong direction comparison. It marks squares as outposts when enemy pawns ARE approaching, and rejects them when enemy pawns have already passed.

### 10. Passed Pawn Detection Bug (eval.go:353)

`countPassedPawnsFast` uses the same upward mask for same-file blocking regardless of color. For black passed pawn detection, the same-file check should use a downward mask.

### 11. No Tapered Eval for Non-King Pieces

Only king PSTs use the tapered MG/EG system. All other pieces use a single PST. This means rook placement doesn't change between opening and endgame, pawn values don't increase in endgame, etc. This is a significant limitation.

### 12. Hand-Tuned Evaluation Values

Comments throughout the code like "Reduced from 50 to prevent over-penalization" and "Reduced from 15 to prevent evaluation inflation" indicate that values were hand-adjusted to fix observed problems. Without Texel tuning or similar automated optimization, these values are almost certainly wrong - and wrong values make features cost ELO rather than gain it.

---

## Performance Issues

### 13. Expensive Quiescence Search

- **Mate detection at qDepth==0** (search.go:1182-1226): Generates ALL moves and tries them to detect checkmate. Called millions of times.
- **Check generation** (search.go:1257-1274): Generates ALL moves to find quiet checks. Extremely expensive for marginal benefit.
- **No TT probing in quiescence**: Standard engines probe TT in qsearch.

### 14. Evaluation Function Complexity

- 2554 lines, 12+ evaluation terms with nested loops
- `evaluatePieceProtection` has O(n^2) piece interactions
- `evaluateKingAttackPatterns` checks a 5x5 area
- Many features slow down NPS without adding accuracy

---

## Root Cause Analysis

The engine is at 1400 because **bugs are causing features to subtract ELO**.

A minimal correct engine (material + PST + alpha-beta + TT + LMR) should be ~1800-2000. This engine has all the advanced features but:

1. **Search bugs prune good moves** (SEE bug, moveCount bug, forced LMR)
2. **TT bug corrupts the principal variation**
3. **Counter move heuristic is non-functional** (global lastMovePlayed)
4. **Extensions are disabled at shallow depth** (guardrail)
5. **Evaluation adds noise** (wrong outposts, wrong PSTs, untuned values)

Each bug doesn't just fail to help - it actively hurts. The engine is fighting itself.

---

## Recommended Fix Order

### Phase 1: Fix Search Correctness (Expected: +200-400 ELO)
1. Add draw detection (repetition + 50-move) inside search tree
2. Fix TT cutoffs in PV nodes (add `!isPV` guard)
3. Fix SEE pawn bug (`canPieceAttackSquare` must handle pawns)
4. Fix isInCheck pawn file masks for white king
5. Fix moveCount vs legalTried for LMR/LMP
6. Fix LMR minimum reduction (allow reduction=0)
7. Fix extension guardrail (allow nextDepth = depth, not depth-1)
8. Remove double check extension
9. Pass previous move through search recursion (fix counter moves)
10. Don't futility-prune the TT move
11. Order losing captures below killer/counter moves

### Phase 2: Simplify Evaluation (Expected: +100-200 ELO)
1. Remove all eval features except: material, PST, passed pawns, bishop pair, basic king safety
2. Use PeSTO or CPW standard PST values (with separate MG/EG)
3. Implement proper tapered eval for all pieces
4. This gives a clean, correct baseline

### Phase 3: Performance (Expected: +50-100 ELO)
1. Remove mate detection from quiescence
2. Remove quiet check generation from quiescence (or limit to qDepth 0)
3. Add TT probing to quiescence
4. This should roughly double NPS

### Phase 4: Add Features Back One at a Time (Expected: +100-200 ELO per feature)
1. Add each evaluation feature back individually
2. Test ELO gain/loss after each addition
3. Only keep features that gain ELO
4. Consider Texel tuning for parameter optimization

### Phase 5: Advanced Tuning
1. Texel tuning of all parameters
2. LMR table tuning
3. Time management tuning

**Total expected gain: 500-900+ ELO (reaching 1900-2300+)**

---

## Reference Points

| Engine | Language | ELO | Eval Type | Notes |
|--------|----------|-----|-----------|-------|
| Blunder | Go | ~2400 | Classical | AdaGrad tuning |
| Zurichess | Go | ~2500 | Classical | Tuned MG/EG PSTs |
| Fruit 2.1 | C | ~2700 | Classical | Clean, simple eval |
| Crafty | C | ~2600 | Classical | Decades of refinement |

All achieved their ELO with classical eval and correct, well-tuned implementations.
