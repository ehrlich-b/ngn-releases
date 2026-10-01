# N1 richer-model feasibility: select Stockfish 18 BIG/dual

Status: **N1a through N1h complete; exact SF18 BIG/dual cleared its throughput
gate, gained exact dirty-threat transitions, and passed both its bounded screen
and its larger pre-registered equal-time strength gate.** This does not expose
the backend through the normal NGN UCI process, change the default, establish
an absolute rating, or authorize promotion.

The immediate question after the rejected P1 staged-picker candidate was which
single richer pretrained evaluator deserved the next architectural lane.
Berserk 14 and Stockfish 18 BIG/dual were the two predeclared alternatives. The
decision is now evidence-bound:

- Berserk 14's pinned source names `berserk-9b84c340af7e.nn`. The exact release
  asset is 25,201,924 bytes and hashes to
  `9b84c340af7e45f6e07f0046235ccb327f4ae0840c8ee2c4b97b99121e5c5084`.
  Its engine source is GPLv3-or-later, but the separate
  `jhonnold/berserk-networks` repository contains model blobs without a license
  or model-provenance statement. That fails this project's pre-incorporation
  gate. It is not a technical rejection of the architecture and can reopen if
  the model owner supplies an explicit artifact license/provenance binding.
- Stockfish's official network archive pins `nn-c288c895ea92.nnue` at
  revision `dd5c7f74c073a7d0bcbb52648d26c537a3d04cf4` with the full SHA-256
  `c288c895ea924429ea9092e3f36b2b3c1f00f2a3a4c759ff7e57e79e3b43e4a7`
  and size 108,919,594 bytes. The archive states that all uploaded networks are
  under CC0. NGN's existing strict BIG loader already passed its official-file,
  independent tensor-spot, focused, race, vet and build gates. This clears the
  provenance and loader prerequisites without reopening them.

Primary bindings:
[Berserk 14 makefile](https://github.com/jhonnold/berserk/blob/8ae895a6151695be4a50d4fb65b0c131659c513a/src/makefile),
[Berserk network repository](https://github.com/jhonnold/berserk-networks),
[Stockfish network archive](https://github.com/official-stockfish/networks/tree/dd5c7f74c073a7d0bcbb52648d26c537a3d04cf4),
[archive license](https://github.com/official-stockfish/networks/blob/dd5c7f74c073a7d0bcbb52648d26c537a3d04cf4/LICENSE), and
[pinned SF18 source](https://github.com/official-stockfish/Stockfish/tree/cb3d4ee9b47d0c5aae855b12379378ea1439675c).

## Slice N1a: selected-head propagation

The existing SMALL implementation was deliberately a diagnostic oracle: every
call propagated all eight material heads and retained wide intermediate traces.
That would make any production comparison artificially slow. N1a adds a
separate trace-free `EvaluateSelected` path for both full refresh and the
worker-private incremental context. It propagates exactly one material-selected
head; `EvaluateAll` remains unchanged as the oracle. Nothing calls the new path
from engine search.

Correctness passed at three independent levels:

- synthetic all-bucket, both-side, defined-wrapping and real-transition tests;
- all 145 pinned full-refresh upstream Stockfish fixtures; and
- all 703 pinned upstream incremental rows, including real/null transitions,
  refresh boundaries, stack growth and unwind.

The configured package race gate and repository-wide short suite passed on WSL
using Go 1.25.5, `GOAMD64=v3`, bounded `GOMAXPROCS=2`, and the exact official
SMALL model. The source hashes and commands are retained in `result-v1.json`.

On one pinned Ryzen 7 9800X3D core, ten one-second blocks measured:

| Path | All-stack median | Selected median | Ratio | Reduction |
| --- | ---: | ---: | ---: | ---: |
| Full refresh + output | 30,825 ns | 5,223.5 ns | 0.16946 | 83.05% |
| Existing incremental state + output | 21,619 ns | 1,420 ns | 0.06568 | 93.43% |

This proves that the all-head diagnostic path is not a valid production-cost
proxy and removes that known confounder before BIG work. It does not predict
BIG cost: BIG has a 1,024-byte transformed input and FullThreats state, while
SMALL has a 128-byte transformed input and no threat features.

## N1b gate definition

The exact eager dual numeric footprint remains 137,844,032 shared model bytes
before object overhead and at least 8,896 bytes per live ply per worker. At 128
frames that is 1,138,688 numeric bytes per worker. This is feasible in memory,
but blindly copying the full dual frame at every edge would repeat a design that
the richer model makes much more expensive. Stockfish's demand-driven
accumulator stack is therefore part of the cost model, not optional polish.

N1b is one bounded, non-search-integrated BIG reference slice:

1. Freeze legal move sequences before looking at timing. Reuse the accepted
   SF18 incremental action corpus and add representative game prefixes only if
   needed to cover occupied slider targets.
2. Generate an independent oracle from pinned Stockfish for both perspectives:
   active FullThreats indices, exact before/after added and removed sets,
   refresh decisions, base/threat accumulators, PSQT, transformed input and the
   material-selected head.
3. Implement only scalar exact BIG full refresh plus a simple sorted-set threat
   diff. Measure active threats, changed rows, refresh incidence, enumeration,
   1,024-lane update, transform and selected-head costs separately. Do not port
   Stockfish's fused/double update machinery in this slice.
4. Report the observed distribution and an explicit optimized-cost ceiling.
   No search/UCI wiring, games or default change may occur until this result is
   reviewed. Failure of exact parity or an unbounded need for donor machinery
   shelves the route; lower NPS by itself does not.

## Slice N1b: exact BIG reference and cost boundary

N1b implements the bounded scalar reference without crossing the search
boundary. The BIG package now has separate HalfKAv2 and FullThreats accumulator
state, allocation-free active-threat enumeration, a simple sorted-set
transition diff, exact 1,024-lane base/threat row updates, transform, and the
material-selected output head. It does not have a worker context, accumulator
stack, SIMD, UCI option, search adapter, default change, or game authorization.

The pinned Stockfish diagnostic was built independently and run on the already
frozen core/growth legal corpus plus 12 frozen 64-ply real-game prefixes. Every
one of 1,203 rows matches for both perspectives across active threats,
before/after threat sets, refresh decisions, base/threat accumulators and PSQT,
transformed input, raw selected outputs, and divided components. Each upstream
incremental row is also compared with a fresh Stockfish stack/cache before it
is admitted. Exact model/oracle/source hashes and commands are in
`result-v2.json`.

The representative 780-row corpus prevents the 128 sparse king-coverage cases
from distorting the cost model:

| Distribution | Median | P95 | Maximum | Mean |
| --- | ---: | ---: | ---: | ---: |
| Active threats / perspective | 36 | 48 | 54 | 33.18 |
| Changed threat rows / perspective-transition | 7 | 13 | 82 | 7.24 |
| Changed threat rows / nonrefresh move, both perspectives | 14 | 26 | 36 | 14.10 |
| Base rows / perspective | 2 | 3 | 4 | 2.27 |

Only 6/1,536 perspective-transitions cross the FullThreats king-file boundary;
47/1,536 move their own king and require a base refresh. The large threat-diff
maxima are those refresh cases, not unbounded ordinary updates.

Ten one-second blocks on one pinned Ryzen 7 9800X3D core measured these scalar
medians, all with zero allocations:

| Kernel | Median ns/op |
| --- | ---: |
| FullThreats enumeration, one perspective | 2,786.5 |
| Sorted threat diff, one perspective | 126.3 |
| One threat row, 1,024 lanes + PSQT | 536.95 |
| One base row, 1,024 lanes + PSQT | 451.85 |
| Transform | 5,519.5 |
| Material-selected head | 6,073.5 |
| Complete two-perspective full refresh | 57,791 |
| Complete full refresh + selected output | 69,694 |

Charging observed changed rows, both enumerations/diffs, and conservatively
replacing every own-king move with an entire two-perspective refresh gives a
measured scalar-kernel ceiling of 15.15 us median and 17.93 us mean for
maintenance. Adding transform/head gives 26.74 us median and 29.53 us mean;
the P95 is 69.38 us because own-king moves make up about 6% of moves. This
is a bounded reference ceiling, not an acceptable eager production cost and not
a search-NPS prediction. Stack bookkeeping and evaluated-node incidence remain
for N1c.

N1b therefore passes rather than shelves the route: exactness is complete,
feature/update counts are bounded, and refresh incidence is low. The next gate
is N1c only: implement a non-search-integrated demand-driven BIG context that
stores dirty transitions, computes from a usable donor only when evaluated,
does not copy 8,896-byte frames on every edge, and matches the same independent
oracle. It must measure evaluated-node cost before any UCI/search/default/game
work.

## Slice N1c: demand-driven BIG context

N1c adds the worker-private accumulator stack that N1b intentionally omitted.
Every push validates and stores only compact board facts plus the semantic
transition, clears four computed bits, and leaves the 9,408-byte numeric frame
untouched. It neither copies the parent frame nor clears the destination
payload. Evaluation finds the nearest usable donor independently for base and
FullThreats state, respects the two feature sets' different king refresh
boundaries, materializes only the missing path, and caches the result. Pop
reconstructs the private board from the stored transition.

The scalar implementation needs one deliberate departure from Stockfish's
always-replay policy. On the representative corpus, evaluating after two dirty
plies costs about 51.94 us and after three costs 66.77 us, but blindly replaying
eight costs 138.95 us. The same-position full-refresh baseline is 72.51 us.
The context therefore replays through three dirty plies and refreshes the
current frame at four or more. This cut the eight-ply case to 76.74 us, within
5.8% of the full-refresh baseline including all eight pushes and pops. The
cutoff is an explicit measured scalar policy; SIMD or fused updates can move it.

Correctness extends the same independent pinned Stockfish oracle rather than
introducing a context-authored oracle. All 1,203 positions match exactly for
both perspectives after ordinary one-ply donors. A second lane evaluates only
every eighth ply, forcing adaptive refreshes, and every case is then unwound to
its root while cached states and selected output are checked again. This covers
real and null pushes, compact-delta and full-position adapters, refresh
boundaries, donors, adaptive refresh, pop, stack reuse, all accumulator/PSQT
lanes, active threats, and final output. The focused race gate and the
repository-wide short suite also pass.

Ten one-second blocks on the same pinned Ryzen 7 9800X3D core measured the
following medians; every path reports zero allocations:

| Demand-context path | Median ns |
| --- | ---: |
| Dirty push + matching pop, per transition | 208.75 |
| Evaluate every ply, per evaluated node | 28,668 |
| Evaluate after 2 dirty plies | 51,942.5 |
| Evaluate after 3 dirty plies | 66,772.5 |
| Evaluate after 4 dirty plies, adaptive refresh | 77,729.5 |
| Evaluate after 8 dirty plies, adaptive refresh | 76,740.5 |
| Re-evaluate already-computed frame | 7,254.5 |
| Same-position selected full refresh | 72,512 |

The one-ply donor path is 60.5% cheaper than the same-corpus full-refresh
baseline (2.53x throughput), while the adaptive cutoff bounds long unevaluated
chains instead of paying work proportional to their length. The fixed
257-frame context occupies 2,418,152 bytes; frames are 9,408 bytes each. That
is a bounded memory trade: the 137,844,032-byte model remains shared, while
each search worker owns about 2.31 MiB of mutable stack.

N1c passes its gate. It proves an exact, allocation-free and bounded scalar
search-state design, not playing strength. No engine search, UCI option,
default, match, or game path uses BIG yet. The next gate is a research-only
search adapter with fixed-position/fixed-node NPS and profile evidence before
any Elo match is authorized. At roughly 20x the existing SMALL selected-output
cost, BIG must demonstrate that evaluation quality pays for its node loss or
identify a SIMD/fused-update prerequisite.

## Slice N1d: research search adapter and node-cost gate

N1d crosses the search boundary only through an explicit programmatic backend,
`sf18-big-research`. It is absent from UCI, is never selected by default, and
does not authorize games. The adapter reuses NGN's already-tested semantic
`nnue.Delta` bridge for every real/null move type, gives each search worker a
private BIG context, and participates in evaluator identity, TT/history
invalidation, stop/unwind, and fresh-evaluation emergency semantics.

Score ownership is explicit. The adapter reproduces Stockfish 18's selected
component blend, zero-optimism complexity correction, and material scaling
using its exact pawn/knight/bishop/rook/queen constants. It then converts the
208-unit Stockfish pawn scale to NGN centipawns. NGN's existing rule-50 and
non-mate clamp boundary is applied once; Stockfish's outer rule-50 adjustment is
not duplicated. Focused tests cover positive/negative truncation, material and
complexity scaling, damping, clamping, transactional selection, backend
identity, and an incremental depth-three search exactly matching a fresh BIG
evaluation at every score read.

The fixed-node cost result rejects the scalar backend for an Elo match. Ten
independent one-iteration samples searched the same middlegame, Kiwipete and
endgame basket for 5,000 nodes each, on one pinned Ryzen 7 9800X3D core:

| Backend | Median ns/node | Implied nodes/s | Relative throughput |
| --- | ---: | ---: | ---: |
| HCE | 598.95 | 1,669,588 | 100% |
| Scalar SF18 BIG | 19,079 | 52,414 | 3.14% |

BIG is therefore 31.85x slower per node. This comparison includes identical
search admission and fixed node budgets; different evaluator decisions are
allowed, but both lanes complete exactly 15,000 nodes per operation.

A final 20-iteration CPU profile localizes the gap inside BIG rather than in
the adapter. `ensureThreats` accounts for 34.0% cumulative time;
`updateThreatFeature` 17.9%, `affineRowValue` 20.4%, transform 16.5%, active
threat enumeration 13.3%, and base row updates 8.1%. Adapter score conversion
does not appear in the top 20. The first optimization target is therefore
AVX2/amd64-v3 numerical kernels for 1,024-lane row updates, transform, and the
selected affine head, followed by threat-enumeration/fused-update work only if
the new profile demands it.

N1d passes as a measurement gate and fails as a match gate. No BIG games should
run at 3.14% of baseline throughput. The next bounded slice is N1e: exact
portable scalar fallbacks plus independently tested amd64-v3 vector kernels,
then repeat this same fixed-node gate. The goal remains active; neither exact
integration nor model pedigree establishes a 3300 rating.

## Slice N1e: exact vector kernels and FullThreats hot path

N1e replaces the measured scalar numerical bottlenecks with fixed-shape AVX2
kernels selected only by `amd64.v3`; every kernel retains a portable scalar
fallback. The kernels cover 1,024-lane base and signed-int8 threat row updates,
the clipped-product transform, the 1,024-wide first affine layer, and the
32-by-32 plus 32-wide affine tail. Their exactness tests include signed limits,
defined int16/int32 wrapping, clipping boundaries, add/subtract paths, and
build-selection witnesses.

The post-kernel profile exposed a second architectural bottleneck rather than a
SIMD one. The reference enumerator repeatedly scanned the 64-square board,
walked slider rays, and recomputed FullThreats feature ranks. N1e adopts the
same broad layout as Stockfish's
[FullThreats implementation](https://github.com/official-stockfish/Stockfish/blob/master/src/nnue/features/full_threats.cpp):
piece bitboards, precomputed attack/ray tables, and split feature-index lookup
tables. NGN still sorts its active set because its deliberately simple
transition contract consumes sorted diffs. The independent pinned oracle, not
the upstream implementation shape, remains the correctness authority.

All 1,203 pinned Stockfish rows still match for both perspectives, including
active and changed threats, accumulators, transformed bytes and selected
outputs. The sparse every-eighth-ply context lane, complete unwind, amd64-v1
fallback build, amd64-v3 build, focused search parity, race gates, vet, and the
repository-wide short suite also pass.

Ten one-second blocks on the same pinned Ryzen 7 9800X3D core measured:

| Path | Scalar median ns | N1e median ns | Speedup |
| --- | ---: | ---: | ---: |
| One threat row | 536.95 | 23.025 | 23.32x |
| One base row | 451.85 | 21.615 | 20.90x |
| Transform | 5,519.5 | 140.45 | 39.30x |
| Material-selected head | 6,073.5 | 255.55 | 23.77x |
| FullThreats enumeration, one perspective | 2,786.5 | 618.35 | 4.51x |
| Complete two-perspective full refresh | 57,791 | 4,657.5 | 12.41x |
| Complete full refresh plus selected output | 69,694 | 4,899.5 | 14.22x |

The scalar context cutoff also became stale. Replaying three dirty plies now
costs about 8.01 us, while rebuilding the current position after the same
pushes costs about 5.87 us. The adaptive policy therefore retains at most two
dirty plies. Fresh full refresh now uses the vector row dispatch too; the
reference path had retained one scalar threat-update loop until this gate.

The unchanged 15,000-node basket gives the decisive end-to-end result:

| Backend | Median ns/node | Implied nodes/s | Relative throughput |
| --- | ---: | ---: | ---: |
| HCE control | 630.0 | 1,587,302 | 100% |
| N1e SF18 BIG | 2,770.5 | 360,946 | 22.74% |

This is a 6.89x speedup over the 19,079 ns/node scalar BIG result and raises
relative throughput from 3.14% to 22.74%. A separate three-operation-per-sample
check measured 715.7 ns/node for HCE and 3,074.5 ns/node for BIG, or 23.28%
relative throughput, so the conclusion is not dependent on the one-operation
sample shape.

The final 300-iteration profile measures 2,874 ns/node. `ensureThreats` is now
45.3% cumulative; active-set enumeration is 19.9%, sorting contributes about
8.2%, and accumulator copies contribute 10.3%. The scalar transform and affine
loops are gone from the profile. This selects an exact Stockfish-style dirty-
threat transition path as the next performance architecture, not more kernel
polish.

N1e passes its cost gate. At roughly 361k nodes/s on the fixed basket, a small
research-only BIG-vs-HCE strength screen is now warranted to learn whether the
richer model pays for its remaining 4.40x node cost. This authorizes only that
screen and the adapter work needed to run it; it does not authorize UCI
exposure, a default change, a wide Elo campaign, or promotion. Dirty-threat
updates remain the next engineering gate if the quality signal is positive.

## Slice N1f: locked research UCI and bounded strength screen

N1f adds a separate `cmd/sf18big-research` process solely for the authorized
screen. It loads the exact official BIG model at startup, identifies itself as
`ngn-sf18-big-research`, hides `EvalBackend` and `EvalFile` from its UCI
handshake, rejects attempts to mutate them, and retains BIG across
`ucinewgame`. The normal `ngn` command, normal UCI option list and HCE default
remain unchanged.

The screen used the existing paired-opening SPRT harness in capped observation
mode: serial execution, 20 ms/move for both roles, 160-ply cap, no score
adjudication, and 20 openings played with reversed colors. Candidate, control,
harness and model hashes are pinned in `result-v6.json`. An 8-game HCE/HCE smoke
first finished 2-4-2 with paired Elo 0 and no operational failures.

BIG then scored **22-16-2 in 40 games (75.0%)** against the same-tree HCE
control. The paired estimate is **+190.8 Elo**, with penta 95% CI
**[+125, +272]** and pentanomial `[0,0,5,10,5]`. Twenty-four games ended by
checkmate, fifteen at the neutral ply cap, and one by the engine's draw rule.
There were no startup, protocol, crash, illegal-move or no-move failures.

This is a deliberately short screen, not a promotion-grade verdict. The
harness reports its narrow `[-3,+3]` SPRT as inconclusive because the run was
pre-capped at 40 games, but the screen question was broader: whether BIG's
evaluation quality could plausibly repay its measured node loss. The positive
paired confidence interval answers that question decisively enough to continue
engineering.

N1f passes. The next bounded slice is N1g: replace per-materialized-ply
FullThreats enumerate-sort-diff with independently verified dirty-threat
updates, then repeat the fixed-node gate and a larger pre-registered equal-time
match before considering normal UCI exposure or a default change.

## Slice N1g: exact dirty-threat transitions

N1g removes full FullThreats enumeration from ordinary materialized plies. A
transition's changed-square set is the union of its removed and added piece
squares. The candidate takes the closure of that set under attackers in both
the before and after boards, using reverse pawn, knight and king tables plus the
nearest diagonal and orthogonal slider on every changed ray. It then compares
only the before/after threat pairs produced by those affected attackers.

That closure is sufficient for every possible changed feature: either the
attacker changed, the occupied target changed, or occupancy on a slider ray
changed. The corresponding attacker is respectively on a changed square,
attacks a changed square, or is the nearest slider found through that square in
one of the two boards. King-orientation half changes remain explicit refresh
boundaries. Per-frame piece bitboards avoid rebuilding twelve bitboards from
the 64-square board, and the sorted active-threat multiset is updated in place.

The permanent regression battery covers quiet pawn and knight moves, a double
pawn step that opens a diagonal, a capture that opens a rook ray, duplicate
feature multiplicity, and canonical active-list tails. More importantly, all
1,203 independent pinned Stockfish rows remain exact for both perspectives,
including sparse every-eighth-ply materialization and complete unwind. Focused
search against fresh full refresh is exact. amd64-v1, amd64-v3, vet, ordinary
race, all official oracles under race, focused engine race, and the
repository-wide short suite pass.

The first subset-enumeration version was rejected after contemporaneous A/B
measurement showed a regression. Direct pair comparison, cached piece
bitboards, precomputed attacker index origins, and an in-place multiset merge
turned the lane into a repeatable win. Twelve alternating samples used prebuilt
candidate and untouched `497dfa5` binaries, the same model, the same pinned
Ryzen 7 9800X3D core, one benchmark operation, and 15,000 searched nodes per
sample. The candidate won all twelve adjacent pairs:

| Variant | Median ns/node | Implied nodes/s |
| --- | ---: | ---: |
| `497dfa5` enumerate-sort-diff baseline | 2,921.0 | 342,349 |
| N1g exact dirty-threat candidate | 2,681.5 | 372,926 |

The ratio-of-medians reduction is **8.20%**; the median of the twelve paired
reductions is **8.10%**. The cached bitboards add 147,456 bytes to the benchmark
operation's roughly 105 MB searcher allocation, with no additional allocation
count. That is an intentional approximately 0.14% memory trade for less work at
every materialized node.

The final 300-operation profile measures 2,664 ns/node. `ensureThreats` falls
from N1e's 45.3% cumulative to 40.8%; dirty transition discovery is 16.3%, while
sorting falls from about 8.2% to 0.69%. Accumulator copies are now the largest
single flat cost at 15.1%. Three-ply replay still loses decisively: median 8.009
us versus 5.993 us for rebuilding after the same pushes, so the measured
two-dirty-ply cutoff remains unchanged.

N1g passes. The next evidence gate is the already authorized larger,
pre-registered equal-time BIG-vs-HCE match. Normal UCI exposure, a default
change, an absolute-rating claim, and promotion remain unauthorized. If more
cost work is interleaved with that match, the profile selects accumulator-stack
copy traffic and fused threat-row updates—not more active-set sorting—as the
next engineering targets.

## Slice N1h: larger pre-registered equal-time match

N1h was frozen in `n1h-match-contract.json` before any game. Candidate, HCE
control and harness were clean `e75533a` builds and were pinned by SHA-256. The
opening set was the first 200 lines of the canonical 5,000-opening corpus
(`974e4b5a...`), itself frozen as a 200-line slice with SHA-256
`2257ecfb...`. These openings are independent of N1f's small built-in set.

The contract kept the earlier 20 ms/move, serial, reversed-color-pair and
160-ply-cap shape, with no score adjudication. It required all games to finish:
the minimum-game thresholds were set one game beyond each cap, so the harness's
narrow `[-3,+3]` SPRT could not stop the run early. The pre-registered strength
rule was instead a positive lower bound on the final pentanomial 95% Elo
interval.

The required 20-game HCE/HCE smoke completed 4-10-6 with paired Elo -34.9 and
95% CI `[-102,+30]`. More importantly for that stage, all games completed and
the log contained no startup, protocol, crash, illegal-move, no-move, watchdog
or timeout failure.

BIG then scored **199-176-25 in 400 games (71.8%)**. Across the 200 opening
pairs, the pentanomial was `[1,8,56,86,49]`; paired Elo was **+161.9** with 95%
CI **[+138,+188]** and paired LLR **+8.43**. The secondary trinomial interval
was `[+124,+200]`. All 400 games completed in 15m59s: 224 checkmates, 164 neutral
ply-cap draws, 11 draw-rule finishes and one stalemate, with zero operational
failure. All artifact hashes matched again after the run.

The harness prints `INCONCLUSIVE` only because the contract deliberately set
`mingames=401` for a 400-game cap. Under the decision rule frozen before the
match, N1h is a decisive **PASS**: the paired lower confidence bound is +138,
far above zero. The independent larger corpus also narrows and confirms N1f's
positive quality signal rather than merely repeating its openings.

N1h proves a large relative gain over same-tree HCE; it still does not prove an
absolute 3300 rating. The next bounded slice is N1i: expose the already locked
BIG backend through a production-compatible opt-in UCI lane, then run a pinned
external-anchor gauntlet. Only that cross-engine calibration can decide whether
BIG is already at the target and whether a default change should be proposed.
