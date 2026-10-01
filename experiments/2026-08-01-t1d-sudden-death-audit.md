# 2026-08-01 T1d — sudden-death "10-move floor leak" — AUDITED, NO CORRECTNESS DEFECT FOUND

**STATUS: DISPROVED as a correctness item. Zero box hours, zero code changed, tree never dirty.**

T1d was the **last open item in the time-management lane** (TODO: "time management is TAPPED, only T1d
(correctness) remains") and had been carried since 2026-07-10 with **no written spec** beyond the phrase
"sudden-death 10-move floor leak still open". It is audited here so the lane can be closed or the item
properly specified, rather than carried indefinitely as an unexamined claim.

## What the mechanism actually is

In sudden death the GUI sends no `movestogo`, so `computeTournamentLimits` (`time.go:158`) falls back to
`estimateMovesRemaining`, which is clamped:

```
remaining = {30,25,20,40}[phase] - movesPlayed,  clamped to [10, 40]
soft = usable/remaining + 0.8*increment
hard = min(4*soft, 0.3*usable)
```

Past ~move 20 the estimate pins at the **floor of 10** and stays there for the rest of the game, so the
engine spends ~1/10 of its remaining bank every move, forever.

**That is geometric decay, not a leak.** The bank is multiplied by ~0.9 per move and never reaches zero, so
the floor cannot by itself cause a flag. The "leak" framing is not supported by the arithmetic.

## The one genuine bug candidate found — and it is NOT reachable

Reading `shouldStopTournamentSearch` (`time.go:392`) surfaced a real ordering hazard:

```go
if elapsed >= tm.hardTime { return true }          // line 395 -- fires FIRST
...
if depth < 1 { return false }                       // line 405 -- "always finish one iteration"
```

The hard-ceiling check precedes the guard whose stated purpose is *"Always finish at least one iteration so
the move is real (never a depth-0 blunder)"*. And `hardTime` **can compute to exactly 0**: when
`baseTime <= emergencyTime + moveOverhead` (150 ms) then `usable` clamps to 0, and with **zero increment**
`soft = 0`, so `hard = min(0, 0) = 0` and the `if hard < soft` rescue does not fire. At that point
`elapsed >= 0` is true immediately, so on paper the search stops before searching anything and returns an
empty best move.

**Tested empirically — it does not happen.** The engine returns a legal move at every bank size down to 50 ms
(below the 150 ms reserve):

| wtime (no inc) | 5000 | 1000 | 200 | 150 | 100 | 50 |
|---|---|---|---|---|---|---|
| bestmove | g1f3 | g1f3 | g1f3 | g1f3 | g1f3 | g1f3 |

**Why it is protected:** `ShouldStopSearch` (`time.go:359`) only evaluates the clock when
`tm.shouldStop || tm.checkCounter%1024 == 0`. The first ~1023 calls return the initial `shouldStop == false`
without looking at the time at all. A depth-1 root loop calls it **once per root move**, and the maximum legal
move count in any chess position is **218 — far below 1024** — so **depth 1 always completes** and
`info.BestMove` is always populated before any clock check can fire.

**Record this honestly: the protection is incidental.** The 1024-call throttle exists as a performance
optimisation (its own comment describes it as avoiding an expensive per-node time check), not as a
correctness guarantee. It happens to bound the failure. The 218 < 1024 margin is large and structural, so
this is not worth a defensive patch on current evidence — but if that throttle is ever tuned down, the
ordering hazard at `time.go:395` becomes live. **Noted for whoever touches it.**

## Bank overspend — checked, none

Measured engine-reported search time against the bank across sudden-death banks 8000 -> 100 ms: the engine
never approaches, let alone exceeds, its bank. No flag risk was found in this path.

## Soft-budget usage — measured, and it is EXPECTED, not a regression

| bank (no inc) | 8000 | 4000 | 2000 | 1000 | 500 | 200 |
|---|---|---|---|---|---|---|
| used / unscaled soft | 26% | 24% | 27% | 25% | 23% | 20% |

| bank (with inc) | 8000+80 | 4000+40 | 2000+20 | 1000+10 |
|---|---|---|---|---|
| used / unscaled soft | 35% | 32% | 28% | 23% |

**There is no sudden-death-specific asymmetry** — both modes sit at ~25-35%, so the hypothesis that the
no-increment path behaves differently from the increment path is **not supported**.

And ~25-35% is the *designed* behaviour, not a defect. Two load-bearing mechanisms produce it:

1. **The projection** — "don't START another iteration we can't finish within the soft budget". At NGN's
   measured mg EBF of ~2.4-2.7, an iteration costs ~2.5x its predecessor, so refusing to start one you cannot
   finish stops you at roughly **1/EBF ≈ 40%** of the budget. **T1a proved this projection is LOAD-BEARING:
   removing it to spend the full soft budget measured -7.8 Elo.**
2. **T1b + T1e composed scaling**, which deliberately shrinks the target (to as low as 0.72x) once the best
   move has settled and owns most of the tree.

**Caution on one number, flagged rather than asserted:** TODO's time-mgmt entry records the 2026-06-03 fix as
"25%->57% soft usage, +37". The measurement above is a **per-move fraction of the unscaled soft target from a
cold TT at move 4**, which is almost certainly **not the same quantity** — 57% most plausibly refers to
whole-game consumption of the game clock. **These numbers are not comparable and the discrepancy is NOT being
claimed as a regression.** If anyone wants to settle it, the instrument must be a full-game clock trace, not
a single `go` command.

## Verdict and disposition

- **T1d is NOT a correctness defect.** No mechanism proof survives contact with the code: the floor produces
  geometric decay, the depth-0 hazard is unreachable, there is no overspend, and there is no sudden-death
  asymmetry.
- Per CLAUDE.md's decision table there is **no "correctness-flavored" class** — so what remains is at most a
  *heuristic* question (is a floor of 10 the best estimate for a long sudden-death game?), which lands
  squarely in the **time-management lane that T1f already measured as TAPPED**, and which would require a
  real-clock games gate to answer.
- **Therefore the time-management lane is now fully closed**: T1a rejected, T1b kept, T1c shelved, T1e kept,
  T1f closed by measurement, **T1d disproved as correctness and demoted into the tapped heuristic lane**.
- **Residual weakness worth naming without chasing:** with the floor pinned at 10, a sudden-death game that
  runs long has the engine playing its last ~20 moves at millisecond speed (0.9^n decay). That is a real
  strength cost in X+0 games — but NGN's entire measurement stack (10+0.1 SPRT, 120+1 gauntlet, CCRL 40/15)
  uses increment or movestogo, so **no instrument currently in use can see it, and no queued verdict depends
  on it.** Do not spend box time here before something measures it.

```yaml
id: 2026-08-01-t1d-sudden-death-audit
date: 2026-08-01
change_class: correctness audit (no change made)
result: >
  No correctness defect. Floor produces geometric decay not a leak; the depth-0 no-move hazard at time.go:395
  is unreachable because the 1024-call throttle guarantees depth 1 completes (218 max root moves << 1024);
  no bank overspend; soft usage ~25-35% in BOTH sudden-death and increment modes, which is the designed
  consequence of the load-bearing projection (T1a: -7.8 to remove it).
box_cost: zero
code_changed: none
next_action: >
  Close the time-management lane. T1d demoted from open correctness item to a tapped-lane heuristic question.
  Flag for anyone tuning the ShouldStopSearch throttle: the hardTime check at time.go:395 precedes the
  always-finish-one-iteration guard at 405, so a smaller throttle would make that hazard live.
```
