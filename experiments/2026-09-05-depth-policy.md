# Child-depth and extension interaction diagnostic

Parent `98968fc`, playing source unchanged. Root specified the four arms before
delegating implementation to Terra. All work is isolated and Mac-only; the native
Windows `r0905texelv2` real-clock match remains undisturbed. This is a bounded
diagnostic, not a playing-strength verdict or authorization to install a variant.

## Correct the historical premise

The repeated claim that the original J1 scout-base repair "lost 59 Elo" is too
strong. Root recovered `output/j1_sprt.log`: it stops after 65 games, 18W/18D/29L,
point estimate −59.4 [−144,+25], pLLR −0.28 against bounds ±2.94. There is no
terminal verdict. Settings were 800ms movetime, concurrency 4 on Mac efficiency
cores; the harness itself warns about contention. `docs/09-invariants-and-joints.md`
already says it was interrupted for another run and not a formal verdict.

The measured tree expansion remains evidence of cost, not a closed lane. This
trial predates the June 28 reset, so it cannot decide current playing strength
under the active policy. It also predates the recapture-extension removal
`1f7f539` (June 12). Preserve the old results and their uncertainty; do not erase
the risk merely because the experiment was incomplete.

## Hypothesis and fixed design

NGN computes `nextDepth` with extensions/negative extensions, then initial
nonfirst scouts use `depth-1-reduction`. Counter 3.8 derives one child depth for
both reduced search and verification (`engine/search.go:392–403`). Chess-3 v4.0
uses ordinary `d-1` children without NGN's check/passed/singular extension stack
(`search/search.go:205–430`). These implementations motivate a simplification
experiment; they do not prove a particular variant stronger in NGN.

| Arm | Scout depth | Positive check/passed-pawn extensions |
| --- | --- | --- |
| A | Current | Current |
| B | `nextDepth-reduction`, removing the defeating reclamp | Current |
| C | Current | Disabled |
| D | `nextDepth-reduction`, removing the defeating reclamp | Disabled |

Preserve singular positive/negative extensions, extension budget, stop/draw/state
handling, evaluation, ordering and all other search rules in every arm. Inspect
nonnegative child-depth handling explicitly. No threshold grid or additional
variant may be added from the observed results. D tests a coherent policy with
singular extensions only; C isolates the contribution of the removed extensions.

Instrument actual initial scout depth versus intended `nextDepth-reduction`.
Aggregate by check/passed/singular-positive/singular-negative/overlap, PV status
and zero versus positive reduction. Save a few FEN/move/depth witnesses. Counters
of computed extensions alone are insufficient. Instrumented A must match
uninstrumented A exactly on nodes, scores, full PV and best move.

Run the three canonical fixed-depth roots and every one of the 23 previously
frozen safe-study selected roots at 400k nodes, preserving full original move
history, fresh engine state, Hash 64 / Threads 1. These are reused diagnostic cases,
not a new independent holdout. Record all outcomes including win/draw controls,
nodes, completed depth and actual search limits. Preserve source diffs, binary
hashes, commands, selection hashes and raw results. Initial implementation and
diagnostics are capped at 25 minutes; return completed work if the cap is reached.

Root then checks distinct candidate moves with the existing Stockfish annotation
and independent legality tools. Fewer nodes, nominal depth and a selected-sample
score improvement cannot establish Elo. At most one candidate proceeds to a
separate predeclared real-clock game gate against the then-accepted immediate
base, after the current fit and T17+T8 decisions. No speculative main changes.

## Completed diagnostic and candidate choice

All four overlays passed the short engine suite. B/C/D passed targeted search,
stop, same-position reentry, fixed-search parity and qsearch tests. Instrumented
A matches uninstrumented A on all26 searches. Root independently matches A to
the frozen qcap binary on the three canonical roots with identical Hash64
settings, and to the prior study's full final UCI output on all23 game roots.

Across the23 game roots, A made4,145,882 initial nonfirst scouts;151,388 used a
depth different from `nextDepth-reduction`. Check-only accounts for93,029,
passed-pawn-only for58,359. Of those mismatches,137,487 were non-PV and13,901 PV;
147,856 had zero reduction and3,532 positive reduction. B/C/D had zero mismatches.

C and D are identical on nodes, scores, full PVs and moves in this sample.
Source inspection explains this: `singularMove` is the TT move, which is hoisted
first. If legal, it is the first legal child and never takes the nonfirst scout;
if illegal, it is skipped. Thus singular-positive/negative masks are zero in
these scout traces. The interaction grid effectively has three distinct policies
for the observed searches; no fourth variant was invented to force a difference.

| Arm | Changed moves /23 | SF child score improves≥50cp | Worsens≥50cp | Any negative change |
| --- | ---: | ---: | ---: | ---: |
| B, consistent extended scouts | 9 | 3 | 0 | 2 (−6,−16cp) |
| C, remove check/passed extensions | 6 | 2 | 0 | 0 |
| D, C plus consistent scouts | 6 | 2 | 0 | 0 |

Root scored all36 distinct choices at Stockfish completed depth16, Hash64,
Threads1, preserving full history. No incomplete/bounded score was accepted.
D's six changed-choice differences are+34,+11,+92,+62,+2,+35cp; all other
choices are identical to A. This selected sample does not estimate Elo or a
population error rate. Its win/draw controls are retained. Independent perft
checks validate every chosen move, all92 NGN PVs and all36 reference PVs:
1,880 legal PV plies, all23 root FENs reproduced exactly.

Work is not uniformly reduced by deleting extensions. Canonical fixed-depth
nodes A→B→D are463,288→568,111→373,138 (Kiwipete d12),
138,322→379,970→292,644 (middlegame d12), and605,563→1,070,281→671,475
(rook ending d16). These searches all finish their requested depth; all keep the
same canonical best move. They are tree-cost observations, not timing or Elo.

**Root selects D for isolated candidate preparation**, preserving the coherent
child-depth expression while removing the two positive extension classes. B
remains uncertain rather than rejected on this diagnostic. The prepared candidate
must strip every instrumentation switch/counter/UCI modification, prove exact
identity with D, and pass full checks before its separate game manifest.
No candidate is installed, committed as playing code, or launched here.

Artifacts:

- `output/depth-policy-20260905/result.json`: SHA-256
  `868e6a18b1c50a3488e5b7f47c7d7b9baf987d874337521dc23d9ebaa7f79303`.
  Exact A–D overlays/diffs, binaries/hashes, original and refined runs, tests
  and bounded actual-call witnesses are adjacent.
- `output/recovery-2026-09-04/depth-policy-scored-20260905/result.json`: SHA-256
  `f219996b3519d0ee2dda54293d3214622dd74026fc9ac2065235fd4f624d205a`.
  Reproducer: `2026-09-05-depth-policy-score.py --input INPUT --output-dir NEWDIR`.
- `output/depth-policy-20260905/root-pv-and-score-audit.json` and
  `root-frozen-baseline-identity.json`: independent root checks above.
- Historical `output/j1_sprt.log`: SHA-256
  `289940502ac678c9797dffe6da175987835410a441c7e03863f73f7c58f1027e`.
