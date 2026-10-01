# 2026-07-26 Ordering-miss diagnostic — where NGN's move ordering actually fails, and T10d rejected

Measured locally while T20 held the box. **Zero box cost.** No instrumentation was written: the counters
already existed (`sd_order` / `sd_miss` / `sd_band`, docs/11 §3, behind UCI `debug on`) and had simply never
been read across a position set. Engine tree clean; nodecheck baselines restore exactly; suite green.

## Why this lane

The **T3 LMR-reduction family is GATED behind move ordering** — T3a measured -7.9 with the recorded
diagnosis that at FMC ~84% extra reduction over-reduces mis-ordered moves that then fail low silently, and
T10's first FMC-raising attempt (T10b, 4-ply conthist) failed. So an ordering fix is a **multiplier**: it
unlocks a whole gated family rather than adding one more increment. That makes it higher leverage than
another corrector, which is why it was worth measuring before queueing anything else.

## HOW TO RUN THIS (the instrument was nearly mis-read — record the trap)

Drive the engine through a **fifo, waiting for `bestmove` before sending `quit`** (the pattern
`scripts/nodecheck.sh:128-132` uses). Piping all commands at once with `printf '...go depth 13\nquit\n'`
makes `quit` arrive **mid-search**: the engine stops early (reached depth 6 of 13), the dump reports a
fraction of the real counts, and successive runs differ (mln 1927 vs 1920) — which also falsely looks like
search non-determinism. The FEN must include the move counters or `position fen` is rejected as invalid and
the engine silently falls back to the start position and plays a **book move**.

## Result — cut-class of the move that actually cut, among NON-first cutoffs

`sd_miss` records, for every beta cutoff that was *not* on the first move tried, which class the cutting move
belonged to. That is precisely "what deserved to be ordered earlier".

| position | ttlist | FMC | misses | cap | qh | kill | cntr | **tt** |
|---|---|---|---|---|---|---|---|---|
| kiwipete (tactical) | 47.0% | 88.4% | 4155 | **67.9%** | 16.6% | 12.5% | 3.0% | **0** |
| quiet-mg-QGD | 37.1% | 87.2% | 1277 | 12.7% | 41.5% | 40.5% | 5.3% | **0** |
| quiet-mg-closed | 28.5% | 81.9% | 8348 | 28.6% | 33.0% | 33.4% | 5.0% | **0** |
| najdorf-mg | 35.4% | 85.5% | 3363 | 27.1% | 28.4% | 37.0% | 7.5% | **0** |
| rook-eg-R4P | 37.3% | 80.6% | 5296 | 2.5% | 64.7% | 29.3% | 3.5% | **0** |

### Finding 1 — kiwipete is NOT representative of the ordering deficit

Measured on kiwipete alone, captures are **67.9%** of ordering misses and the obvious conclusion is "fix
capture ordering." Across the quiet/endgame set captures are **2.5-28.6%**, and in the endgame they are
essentially absent. **Killers + quiet history are 74-94% of misses in every non-tactical position.** Any
ordering work steered by kiwipete alone would have been aimed at the wrong class. (Same lesson as
`project_endgame_tree_diagnostic`: single-position measurement misleads on tree-shape questions.)

### Finding 2 — the TT side of ordering is already perfect

**TT misses are exactly 0 in every position measured.** When a hash move exists it is tried first and never
"deserved to be earlier." Combined with the T11a pre-flight (`ttMoveAvail` invariant to TT pressure, and
every TT hit already carrying a move), this closes the TT direction of the ordering lane: **the entire
remaining deficit is in quiet-move ordering**, not in hash-move availability or retention.

### Finding 3 — FMC is 80.6-88.4%, and the endgame is the worst

The recorded "~84% FMC" is confirmed as an average, but the spread matters: the rook endgame is **80.6%**
with 64.7% of its misses in quiet history. The gate on the T3 family binds hardest in the endgame.

## T10d — killer/counter band swap — PROXY REJECTED

The ordering bands, read from code (`moveorder.go:575-625`):

```text
TT move -> queen promo 90000 -> winning captures ~81000+ -> under-promo 70000
        -> castle 50000 -> COUNTER +30000 -> KILLER +25000 -> history quiets -> losing captures -41000
```

NGN already correctly sinks losing captures (-41000) **below** all quiets, so bad-captures-before-killers is
not a defect here. The only quiet-class move ordered ahead of a killer is a **counter-move**.

**Hypothesis:** counters outrank killers (30000 vs 25000) despite killers cutting far more often — kiwipete
`cutkill 1735` vs `cutcntr 384` (~4.5x), and killers are 29-41% of misses against counters' 3.0-7.5%. Swap
the bands so killers lead.

**Measured — the hypothesis is FALSE.** `killer 30000 / counter 25000`, FMC vs base:

| position | base FMC | T10d FMC | delta |
|---|---|---|---|
| quiet-mg-QGD | 87.20% | 85.38% | **-1.82** |
| quiet-mg-closed | 81.93% | 84.24% | +2.31 |
| najdorf-mg | 85.45% | 85.29% | -0.16 |
| kiwipete | 88.44% | 87.87% | -0.57 |
| rook-eg-R4P | 80.64% | 79.07% | **-1.57** |

**Net -1.81 FMC across five positions, four of five worse.** Nodecheck moved decisively
(+7.0% / -44.6% / +74.8%), so the change was live — this is a real measurement, not an inert patch.

**REJECT T10d on proxy.** Sanctioned under CLAUDE.md: this is a pure ordering change and FMC is the matching
instrument. Patch shelved at `output/t10d-killer-counter-swap.patch`; tree reverted; baselines restore.

**Why the hypothesis was wrong (mechanism, for the next agent):** cut *frequency* is the wrong ranking
criterion. A counter-move is the refutation of **the specific previous move**, so when one exists it is
context-matched to this node; a killer is ply-global and fires often precisely because it is broad. The
existing order encodes **context-specificity over frequency**, and the measurement says that is the better
trade. Do not re-derive "order the more frequent class first" — it is falsified.

**REOPEN CONDITION (predeclared).** This rejects the full swap. Untested neighbours: equal bands (both
25000 or both 30000, removing the priority distinction rather than inverting it); and a killer bonus made
conditional on the counter being absent. Given 4-of-5 worse, the direction is not promising and neither
variant should be run unless the ordering lane is re-motivated by other evidence.

## What survives for the ordering lane

The deficit is **quiet-move ordering (history + killers), worst in the endgame**, not captures, not the TT.
That is the target any future T10-lane candidate must hit, and it should be measured with **this five-position
FMC table**, never a single position. Note T10b (4-ply conthist, uniform weight) already attacked quiet
ordering and capped null at -2.4; its recorded reopen (non-uniform ply weights, or 3-ply only) is aimed at
the right class per this diagnostic, which is a mild upgrade to that reopen's standing.
