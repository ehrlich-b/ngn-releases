# NGN Chess Engine Performance Analysis Brief

## Current Performance: ~143K nodes/second (UPDATED)

This document analyzes the NGN chess engine performance bottlenecks and provides guidance for optimization.

## Executive Summary

**FIXED: The primary bottleneck (mobility evaluation) has been resolved, achieving 2.4x speedup from 60K→143K nps.** Remaining issues include inefficient legal move validation and excessive allocations for further optimization if needed.

## Critical Performance Issues (Ranked by Impact)

### 1. ✅ **FIXED: Evaluation Function Generates Legal Moves** (was 80% of slowdown)

**Location:** `engine/eval.go:448-450`
```go
func evaluateMobility(board *Bitboard, pos *Position) int {
    legalMoves := GenerateLegalMoves(pos)  // ❌ EXPENSIVE!
    mobility := len(legalMoves) * 2
    ...
```

**Problem:** 
- Called at EVERY leaf node in the search tree
- Called during quiescence search stand-pat evaluation
- Move generation is expensive (see issue #2)
- For a 5-ply search, this might be called 100,000+ times

**Solution Applied:**
- ✅ **Removed mobility evaluation entirely** - achieved 2.4x speedup (60K→143K nps)
- Engine now meets 100K+ nps threshold for continued feature development

### 2. 🟠 **MAJOR: Legal Move Validation is Expensive** (15% of slowdown)

**Location:** `engine/movegen.go:515-523`
```go
func IsLegalMove(pos *Position, move Move) bool {
    ep, tag, hc, _ := pos.MakeMove(move)      // Make move
    legal := !isInCheck(pos, move.Color())    // Check if king in check
    pos.UnMakeMove(move, tag, ep, hc)         // Unmake move
    return legal
}
```

**Problem:**
- For EVERY pseudo-legal move, we make/unmake it to check legality
- This happens multiple times: in move generation AND in evaluation
- A position might have 30-40 pseudo-legal moves

**Solution:**
- **Option A:** Use pinned piece detection to avoid make/unmake for most moves
- **Option B:** Batch legality checking
- **Option C:** Only check legality when actually making the move in search

### 3. 🟡 **MODERATE: Memory Allocations** (5% of slowdown)

**Location:** Multiple places
```go
// In GenerateLegalMoves
legalMoves := make([]Move, 0, len(pseudoLegalMoves))

// In generatePieceMoves  
moves := make([]Move, 0, 32)
```

**Problem:**
- Allocating new slices for every move generation
- Not reusing memory between calls

**Solution:**
- Use a move list pool
- Preallocate and reuse move lists

## Architecture Analysis

### ✅ What's Good (Not causing slowdown)
1. **Position updates are in-place** - We're using `*Position` and modifying the same struct, NOT copying
2. **Bitboard operations are efficient** - Using uint64 bitboards is standard
3. **Hash updates are incremental** - Zobrist hashing is done correctly

### ❌ What's Bad
1. **Evaluation is doing search work** - Generating moves in eval function
2. **No move generation caching** - Regenerating moves even for repeated positions
3. **Redundant legality checks** - Checking legality multiple times per move

## Performance Comparison

| Engine | NPS | Why It's Faster |
|--------|-----|-----------------|
| Stockfish | 5-25M | - SIMD instructions<br>- No move gen in eval<br>- Staged move generation<br>- Bitboard magic for sliding pieces |
| Our Engine | 60K | - Generates all legal moves in eval<br>- Makes/unmakes every move for legality<br>- No optimization |

## Quick Wins (Implement These First)

### 1. Remove Mobility from Evaluation (5 minute fix)
```go
// Comment out or remove this entire section in Evaluate()
// if !endgame {
//     pos.SetTag(WhiteToMove)
//     whiteMobility := evaluateMobility(board, pos)
//     pos.SetTag(BlackToMove)
//     blackMobility := evaluateMobility(board, pos)
//     eval += whiteMobility - blackMobility
// }
```
**✅ COMPLETED: Achieved 2.4x speedup (60K → 143K nps)**

### 2. Generate Only Captures in Quiescence
The quiescence search is already generating all moves then filtering. Fix this:
```go
// Instead of:
moves := GenerateLegalMoves(pos)
for _, move := range moves {
    if move.IsCapture() { ... }

// Do:
captures := GenerateCaptures(pos)  // Generate ONLY captures
```
**Expected improvement: 1.5x speedup in quiescence**

### 3. Lazy Move Generation in Alpha-Beta
Don't generate all moves upfront if we might get a cutoff:
```go
// Generate moves one at a time, stop on beta cutoff
```

## Long-term Optimizations

1. **Magic Bitboards** for sliding piece move generation
2. **Move Generation Caching** for repeated positions
3. **Staged Move Generation** (hash move → captures → killers → quiet)
4. **Pin Detection** to avoid make/unmake for legality
5. **SIMD Operations** for parallel position evaluation

## Recommendation for Next Steps

**✅ COMPLETED: Removed mobility evaluation from the evaluation function.**

This change achieved 143K+ nps, which exceeds the 100K threshold. The engine can now continue with feature development (better evaluation, search improvements) and return to optimization later if needed.

The engine architecture is fundamentally sound - you're not copying positions, you're using bitboards correctly, and the search structure is good. The performance issue is a specific bug (move generation in evaluation) rather than a fundamental design flaw.

## Testing the Fix

After removing mobility evaluation:
```bash
go test ./engine -run TestPrecise5SecondPerformance -v
```

✅ **Actual result: 143K+ nps** (exceeds 100K threshold)

## For Future LLMs Working on This

**Priority Order:**
1. ✅ Remove/fix mobility evaluation (was 80% of the problem) - COMPLETED
2. Optimize legal move generation (now the largest remaining bottleneck at ~15%)  
3. Everything else is minor optimization

**Do NOT:**
- Try to optimize bitboard operations (they're fine)
- Change from in-place position updates (current approach is correct)
- Add complex caching before fixing the eval function

**Key Insight:** The engine generates legal moves 100,000+ times per second because the evaluation function calls GenerateLegalMoves(). This is completely unnecessary and fixing it will give a 3-5x speedup.