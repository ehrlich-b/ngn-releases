# 2026-07-26 T11a pre-flight — TT pressure is not the constraint, and `ttlist` availability is not a retention problem

Measured locally while T20 held the box. **Zero box cost. T11a never reached the box.** Instrumentation
fully reverted; nodecheck baselines restore exactly (346662 / 149587 / 765656); `go test -short ./engine`
green. Shelf copies: `output/t11a-ttprobe.patch`, `output/t11a-ttprobe_test.go.txt`.

## Premise verified from code

`engine/cache.go:124-157`. NGN's TT is **direct-mapped, 1-way**:

```go
func (c *Cache) index(hash uint64) uint { return uint(hash) & c.mask }   // single slot, no bucket
```

Replacement on a store: empty ⇒ take it; same key ⇒ `depth >= oldDepth-3 || nodeType == Exact`;
**different key (collision) ⇒ `oldAge != age || depth >= oldDepth`** — so a same-age, deeper occupant
rejects the new entry outright. `CachedEval` is 16 B, so a 4-entry cluster is exactly one 64 B cache line;
4-way clustering is the natural transplant and the premise is real.

T11a was queued because the plateau diagnosis identified low TT-move availability (`ttlist`) as the specific
deficit, and the 2026-07-26 queue audit ranked it above the other node-level items on the mechanism argument
that it *retains information* rather than adding another rule competing to cut the same nodes.

## Method

Full self-play games sharing one TT — the only realistic model of table pressure, since a single search
from a fixed position never loads the table. 70 plies per game from corpus openings, default 128 MB
(8388608 slots — the SPRT command sets no `Hash`, so this is the gate's real configuration).

**A first pass at depth 11 was discarded as unrepresentative and is recorded here so it is not reused:**
it stored only ~908K entries per game (~13K nodes/move), roughly a tenth of real per-move search, and
reported 6.5% occupancy / 0.08% harmful collisions. Drawing "zero headroom" from that would have been wrong.
The reported run below uses **depth 14**, calibrated against ebfprobe (which measures depth 13-14 at 400K
nodes) so per-move node counts match 10+0.1 play on the box. Cross-check: 107K main-search probes/move plus
qsearch ≈ 300-400K total nodes/move, consistent with the ebfprobe calibration.

## Result (depth 14, 128 MB, 70 plies)

| | game 1 | game 2 |
|---|---|---|
| stores | 1828726 | 3141865 |
| end-of-game occupancy | **13.65%** | **21.96%** |
| collision rate | 4.83% | 7.74% |
| collisions evicting **stale** (old-age) entries | 83173 (**94.1%**) | 232457 (**95.6%**) |
| collisions **rejected** (new entry lost) | 2430 | 5009 |
| collisions evicting a **same-age** entry | 2761 | 5757 |
| **harmful collisions as a share of all stores** | **0.28%** | **0.34%** |
| `ttMoveAvail` (gets returning a hit with a move) | **27.49%** | **28.43%** |

Occupancy over the game (game 2): 3.2% @ply10 → 10.8% @20 → 13.5% @30 → 17.1% @40 → 17.9% @50 →
19.9% @60 → 22.0% @70.

## FINDING — `ttMoveAvail` is INVARIANT to TT pressure, so clustering cannot move it

This is the load-bearing result and it is what kills T11a:

| condition | occupancy | collision rate | `ttMoveAvail` |
|---|---|---|---|
| depth 11, ply 10 | 0.82% | 0.29% | 29.93% |
| depth 11, ply 70 | 6.49% | 2.03% | 32.40% |
| depth 14, ply 10 | 3.22% | 1.19% | 26.01% |
| depth 14, ply 70 | 21.96% | 7.74% | 28.43% |

Availability sits at **25-32% across a 27x range of occupancy and a 27x range of collision rate**. It does
not rise as the table fills; it does not fall as collisions climb. **TT-move availability is not limited by
eviction.** It is limited by how often the search revisits a position at all — a property of the search tree,
not of the table.

4-way clustering addresses exactly one thing: destructive collisions. Those are **0.34% of stores**, and 95%
of all collisions are correctly evicting stale entries from earlier moves, which is the intended behavior of
the aging scheme. So clustering would pay a 4-slot probe on every access to recover a third of one percent of
stores, targeting a metric it provably cannot move.

**Additional check: `hits == hitsWithMove` exactly, in every game at every depth.** Every TT hit already
carries a move. There is no "hit but no usable move" loss to recover either.

## Verdict

**T11a REJECTED on pre-flight. Do not build 4-way TT clustering.** Grounds:

1. The table is **78-86% empty** at end of game in the gate's own configuration; it is not under pressure.
2. Harmful collisions are **0.34% of stores**; 95% of collisions are the aging scheme working as designed.
3. The metric it was queued to raise, `ttMoveAvail`, is **invariant to both occupancy and collision rate**,
   so the mechanism cannot operate.

**This also corrects the queue audit and the plateau diagnosis.** The audit ranked T11a above T15/T12/T19
on the argument that it "raises TT-move availability... information retention, not another rule competing to
cut the same nodes." The retention framing is **falsified**: nothing is failing to be retained. The measured
availability (~27%) is even lower than the diagnosis's quoted 34-47%, but its cause is transposition
frequency, not eviction — and **a low `ttlist` number should no longer be used to motivate candidates**
unless a mechanism is shown that changes how often the search revisits positions.

**REOPEN CONDITION (predeclared).** Reopen TT clustering only if a configuration is demonstrated where the
table actually saturates — e.g. a much longer TC, a much smaller `Hash`, or a confirmed measurement that
real box games store several times more than this simulation. The pre-flight to re-run first is this same
probe; clustering earns box time only once end-of-game occupancy exceeds ~70% **and** harmful collisions
exceed ~5% of stores. Note the opposite lever is also now measurable and currently pointless: raising `Hash`
above 128 MB cannot help a table that is 80% empty (and T14's small-TT/large-pages idea is the only variant
of this neighborhood that the data does not contradict).

## Caveats

1. Fixed-depth self-play on a local machine, fresh TT per game, 2 games. Real box games vary in length and
   the 8 concurrent SPRT workers are separate processes with independent tables, so per-process pressure is
   modeled correctly, but game-to-game variance is undersampled (game 2 stored 72% more than game 1).
2. Depth 14 was calibrated to the ebfprobe node count, not measured against real box logs. If real games
   store materially more than 3.1M per game, occupancy rises — but availability's invariance across the
   whole measured range is the finding that does not depend on the absolute level.
3. `ttMoveAvail` here counts main-search probes (search.go:1303) only, not qsearch probes.
