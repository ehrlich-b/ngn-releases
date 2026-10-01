# Zero-Allocation Search Design

## Executive Summary
This document outlines a comprehensive strategy to eliminate ALL memory allocations from the search hot path in the NGN chess engine. The goal is to achieve true zero-allocation search by converting all dynamic allocations to stack-based or pre-allocated buffers, ultimately targeting competitive 3M+ NPS performance.

## Current Allocation Analysis (From Memory Profiling)

### Memory Benchmark Results
```
BenchmarkSearchPerformance: 254,860 B/op, 336 allocs/op
BenchmarkSearch:            267,852 B/op, 336 allocs/op  
BenchmarkSearchDeep:        1,283,367 B/op, 1,682 allocs/op
```

### Allocation Breakdown by Function
- **quiescenceWithDepth**: 19.49GB (53.00%) - 33,131,991 objects (64.71%)
- **alphaBeta**: 17GB (46.24%) - 17,830,383 objects (34.83%)
- **Total Hot Path**: 36.49GB (99.23%) - 50,962,374 objects (99.54%)

### Critical Finding
The VAST MAJORITY of allocations (64.71%) come from quiescence search, despite our recent optimization! This suggests we're still allocating in quiescence, likely in move generation or buffering.

## Current Allocation Hotspots

### 1. Quiescence Search (PRIMARY BOTTLENECK - 64.71% of allocations!)

#### **Stack Arrays Escaping to Heap** (search.go:879, 889)
```go
var captureBuffer [64]Move      // Line 879: 4.04GB allocated!
var moveBuffer [256]Move        // Line 889: 15.45GB allocated!
```

**CRITICAL DISCOVERY**: These are declared as stack arrays but Go is escaping them to heap!
- **Impact**: 19.49GB total (53% of all allocations)
- **Frequency**: Called millions of times in quiescence
- **Root Cause**: Go's escape analysis moves large stack arrays to heap
- **Solution**: 
  1. Reduce array sizes (risky - might overflow)
  2. Pass buffers from caller (best solution)
  3. Use global/thread-local buffers

### 2. Search Path Allocations

#### **SEE Exchange Gain Array** (search.go:54)
```go
gain := make([]int, 32) // Exchange gains
```
- **Impact**: Allocated on EVERY capture evaluation  
- **Frequency**: Called from quiescence for EVERY capture
- **Memory**: 32*8 = 256 bytes per call
- **Solution**: Stack-allocated fixed array

#### **Move Ordering Scored Moves** (moveorder.go:152, 214)
```go
scoredMoves := make([]moveScore, len(moves))
orderedMoves := make([]Move, len(moves))
```
- **Impact**: Allocated for EVERY node in the search tree
- **Frequency**: ~100,000+ times per search
- **Solution**: Pre-allocated thread-local buffers

#### **Move Generation** (movegen.go:13, 68, 88, 101, 115, 221, 325, 449)
```go
moves := make([]Move, 0, 64)
legalMoves := make([]Move, 0, len(pseudoLegalMoves))
captures := make([]Move, 0, len(moves))
```
- **Impact**: Multiple allocations per move generation
- **Frequency**: ~200,000+ times per search
- **Solution**: Caller-provided buffers

#### **Evaluation Attack Arrays** (eval.go:1922, 1963, 1984, 2011)
```go
attacks := make([]int, 0, 27) // For king safety evaluation
```
- **Impact**: Allocated during position evaluation
- **Frequency**: ~100,000+ times per search  
- **Solution**: Stack arrays with count tracking

## Zero-Allocation Architecture

### Phase 1: Complete Elimination (No Allocations Whatsoever)

#### 1.1 Thread-Local Search Context
```go
type SearchContext struct {
    // Move buffers (max 256 moves per position)
    moveBuffer      [256]Move
    captureBuffer   [64]Move
    orderedBuffer   [256]Move
    scoredMoves     [256]MoveScore
    
    // SEE buffers
    seeGains        [32]int
    seeAttackers    [64]Square
    
    // Evaluation buffers  
    attackers       [32]int
    defenders       [32]int
    
    // Move generation scratch space
    pinMask         uint64
    checkMask       uint64
    
    // Killer/History tables (shared across search)
    killerMoves     [MaximumDepth][2]Move
    historyTable    [64][64]int
    counterMoves    [64][64]Move
}

// Global pool of search contexts (one per thread)
var searchContextPool = [MaxThreads]*SearchContext{}
```

#### 1.2 Buffer-Based APIs
```go
// All move generation returns count, uses caller buffer
func GenerateMovesIntoBuffer(pos *Position, out []Move) int
func GenerateCapturesIntoBuffer(pos *Position, out []Move) int  
func GenerateQuietMovesIntoBuffer(pos *Position, out []Move) int

// Move ordering in-place
func OrderMovesInPlace(moves []Move, scores []int, count int)

// SEE with provided workspace
func SEEWithBuffer(pos *Position, move Move, gains []int) int
```

#### 1.3 Stack-Based Small Arrays
```go
// Instead of make([]int, n) for small n
var attacks [32]int  // Stack allocated
attackCount := 0     // Track actual usage
```

### Phase 2: Pragmatic Scaling Back

After achieving zero allocations, we selectively reintroduce allocations where the performance benefit of dynamic sizing outweighs allocation cost:

#### 2.1 Keep Zero-Allocation
- **SEE gains array**: Fixed size, high frequency → KEEP STACK
- **Move buffers in search**: Bounded size, critical path → KEEP STACK  
- **Evaluation arrays**: Small, frequent → KEEP STACK
- **Move ordering workspace**: Performance critical → KEEP PRE-ALLOCATED

#### 2.2 Consider Dynamic (But Don't Implement Yet)
- **Transposition table**: One-time allocation → ACCEPTABLE
- **Opening book**: Loaded once → ACCEPTABLE
- **UCI position history**: Not in search path → ACCEPTABLE

## Implementation Strategy

### CRITICAL FIX: Quiescence Stack Escape (Highest Priority!)

The immediate issue is that Go is escaping our stack arrays to heap. This happens because:
1. Arrays are too large for Go's stack frame limits
2. The arrays might be captured by closures or passed to interfaces
3. Go's escape analysis is conservative

#### Immediate Fix - Pass Buffers Down
```go
// Add buffers to SearchInfo to pass through recursion
type SearchInfo struct {
    // ... existing fields ...
    
    // Shared buffers for zero allocation
    MoveBuffer        *[256]Move
    CaptureBuffer     *[64]Move  
    OrderedBuffer     *[256]Move
    SEEGains          *[32]int
}

// Modified quiescence to use passed buffers
func quiescenceWithDepth(pos *Position, alpha int, beta int, info *SearchInfo, qDepth int) int {
    // Use info buffers instead of stack arrays
    captureCount := GenerateCapturesIntoBuffer(pos, info.CaptureBuffer[:])
    
    // For check detection, use the main move buffer
    if qDepth < 2 {
        numMoves := GenerateMovesIntoBuffer(pos, info.MoveBuffer[:])
        // ... rest of logic
    }
}
```

### Step 1: SearchContext Introduction
```go
func alphaBeta(pos *Position, depth int, alpha int, beta int, info *SearchInfo, ctx *SearchContext) int {
    // Use ctx.moveBuffer instead of make([]Move, ...)
    numMoves := GenerateMovesIntoBuffer(pos, ctx.moveBuffer[:])
    moves := ctx.moveBuffer[:numMoves]
    
    // Use ctx.orderedBuffer for ordering
    orderedCount := orderMovesIntoBuffer(moves, ctx.orderedBuffer[:], ctx.scoredMoves[:])
    
    // SEE with workspace
    seeValue := SEEWithBuffer(pos, move, ctx.seeGains[:])
}
```

### Step 2: Convert Critical Functions

#### SEE Conversion
```go
func staticExchangeEvaluation(pos *Position, move Move) int {
    var gains [32]int  // Stack allocated
    return seeWithGains(pos, move, gains[:])
}

func seeWithGains(pos *Position, move Move, gains []int) int {
    // Implementation using provided buffer
}
```

#### Move Ordering Conversion  
```go
func orderMovesSinglePassInPlace(moves []Move, count int, scores []int, ...) int {
    // Score moves in-place using provided scores buffer
    for i := 0; i < count; i++ {
        scores[i] = calculateMoveScore(moves[i], ...)
    }
    
    // Sort in-place by scores
    insertionSortByScores(moves, scores, count)
    return count
}
```

### Step 3: Global Tables as Singletons

```go
// Instead of passing around, use global singletons
var globalHistoryTable [64][64]int
var globalKillerMoves [MaximumDepth][2]Move
var globalCounterMoves [64][64]Move

// Access directly, no allocation needed
func UpdateHistoryTable(move Move, depth int) {
    globalHistoryTable[move.Source()][move.Destination()] += depth * depth
}
```

## Performance Impact Analysis

### Current State (With Allocations)
- **NPS**: 2.64M (378μs per node)
- **Allocations**: 255KB/op, 336 allocs/op
- **GC Pressure**: Moderate, periodic pauses

### Projected Zero-Allocation State
- **NPS**: 3.0M - 3.3M (303-333μs per node)
- **Allocations**: 0KB/op, 0 allocs/op
- **GC Pressure**: None in hot path
- **Benefits**:
  - 15-25% performance improvement
  - Predictable latency (no GC pauses)
  - Better CPU cache utilization
  - Reduced memory bandwidth usage

## Risk Mitigation

### Correctness Risks
1. **Buffer Overflow**: Use bounds checking in debug builds
2. **Thread Safety**: Each thread gets its own SearchContext
3. **Recursion Depth**: Stack arrays sized for MaximumDepth

### Performance Risks  
1. **Stack Size**: Monitor stack usage, adjust array sizes
2. **Cache Pollution**: Keep frequently accessed data together
3. **False Sharing**: Align SearchContext to cache lines

## Testing Strategy

1. **Correctness Tests**: 
   - Perft validation before/after
   - Identical search results on test positions
   - Stress testing with deep searches

2. **Performance Tests**:
   - Benchmark before/after each change
   - Profile to confirm zero allocations
   - Measure NPS improvement

3. **Memory Tests**:
   - Confirm zero allocations with `-benchmem`
   - Check stack usage doesn't exceed limits
   - Verify no memory leaks

## Implementation Priority (Based on Profiling Data)

### IMMEDIATE - Critical Performance Blockers
1. **Quiescence stack arrays escaping to heap** (19.49GB, 64.71% of allocations!)
   - Lines 879, 889 in search.go
   - Fix: Pass buffers through SearchInfo
   - Impact: Eliminate 64.71% of all allocations

2. **SEE gains array** (Part of remaining 35% allocations)
   - Line 54 in search.go  
   - Fix: Stack array or passed buffer
   - Impact: Called for every capture evaluation

### HIGH - Secondary Hotspots  
3. **Move ordering allocations** (moveorder.go:152, 214)
   - Fix: Use in-place sorting with pre-allocated scores
   - Impact: Called at every search node

4. **Move generation internal allocations** (Various make() calls)
   - Fix: Convert all to buffer-based APIs
   - Impact: Reduce function call overhead

### MEDIUM - Worth Fixing
5. **Evaluation arrays** (eval.go:1922, 1963, 1984, 2011)
   - Fix: Stack arrays with count tracking
   - Impact: Moderate frequency

### LOW - Keep As-Is (Not in Hot Path)
6. **UCI initialization** - One-time
7. **Position history map** - Not performance critical  
8. **Cache initialization** - One-time

## Conclusion

### Key Findings from Memory Profiling
1. **Major Discovery**: Stack arrays in Go can escape to heap when too large
2. **Primary Bottleneck**: Quiescence search accounts for 64.71% of all allocations
3. **Simple Fix Available**: Passing buffers eliminates most allocations

### Expected Impact
- **Current**: 254,860 B/op, 336 allocs/op, 2.64M NPS
- **After Quiescence Fix**: ~90,000 B/op, ~100 allocs/op (64% reduction)
- **After Full Implementation**: <1,000 B/op, <10 allocs/op
- **Performance Target**: 3.0M+ NPS (14%+ improvement)

### Implementation Plan
1. **Phase 1** (Immediate): Fix quiescence heap escape → 64% allocation reduction
2. **Phase 2** (Next): Fix SEE and move ordering → 90% allocation reduction  
3. **Phase 3** (Future): Complete zero-allocation with SearchContext → 99%+ reduction

The design prioritizes fixing the highest-impact allocations first (quiescence arrays) which alone will eliminate 64.71% of allocations. Combined with SEE fixes, we can achieve 90%+ reduction with minimal code changes.

### Next Steps
1. Implement SearchInfo buffer passing for quiescence
2. Convert SEE to use stack arrays
3. Benchmark and verify improvements
4. Proceed with remaining optimizations if needed