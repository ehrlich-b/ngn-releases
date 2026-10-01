# 2026-07-26 T6 pre-flight — the live history distribution, and why the revamp doc's T6 spec was wrong

Measured locally while T20 held the box. **Zero box cost.** This is a pre-flight measurement, not a game
result. Instrumentation fully reverted; `go test -short ./engine` green on the restored tree. Shelf copies:
`output/t6-histprobe.patch`, `output/t6-histprobe.go.txt`, `output/t6-histprobe_test.go.txt`.

## Why this was run

The revamp doc (`2026-07-01-method-revamp.md:265-272`) specifies T6 as:

> CounterGo uses `r -= clamp(histSum/5000, -2, +2)` where its EMA-form history saturates entries toward
> ±16384 (sum range ±49k). **NGN's gravity-form entries cluster near 0 (equilibrium), so the tested `/2048`
> truncated to 0 almost everywhere** (the recorded "int-trunc no-op") and `/256` over-swung — **the
> 2026-06-28 rejection never tested a divisor matched to NGN's actual value distribution WITH a ±2 clamp.**
> Retest: measure the live histSum distribution, **pick divisor ≈ P90/2**, clamp ±2.

The spec names its own prerequisite — measure the distribution — so that was done first.

## Premise verified from code

`search.go:1944-1954` — NGN's current history-LMR term is a crude asymmetric step function:

```go
histScore := GetHistoryScore(move, savedLast, prev2)   // main + continuation + followup
if histScore < -500 {
    reduction++          // bad history -> reduce MORE
} else if histScore > 1000 {
    reduction--          // good history -> reduce LESS
}
```

`GetHistoryScore` (moveorder.go:284) sums three gravity-bounded tables, each capped at
`historyMax = 8192` ⇒ theoretical range **±24576**.

## Measurement

200 corpus openings replayed to their end position, one fixed-depth-11 search each,
**3.8M LMR history evaluations** sampled at the decision point itself.

### The current rule is highly active — not a dead term

| bucket | count | share |
|---|---|---|
| `histScore < -500` (reduce MORE) | 1931916 | **50.76%** |
| `histScore > +1000` (reduce LESS) | 928573 | **24.40%** |
| dead zone (no adjustment) | 945827 | 24.85% |

### Live |histScore| distribution

| percentile | value |
|---|---|
| P50 | 1450 |
| P75 | 2550 |
| P90 | **3950** |
| P95 | 5650 |
| P99 | 9550 |

## FINDING 1 — the revamp doc's stated rationale for reopening T6 is FALSE on both counts

**(a) `/2048` did NOT "truncate to 0 almost everywhere."** Measured, at divisor 2048 with the ±2 clamp the
continuous scheme produces a **nonzero adjustment on 37.20%** of LMR evaluations and **differs from the
current rule on 47.44%** of them. That is a live, large behavioral change — not an int-truncation no-op.

**(b) The 2026-06-28 test DID use the ±2 clamp.** Reading the actual run record
(`experiments/2026-06-28-lmr-conthist.md`) rather than the summary of it, the tested formula was:

```text
reduction -= clamp(histScore/2048, -2, +2)
```

— character-for-character the change T6 proposes. The claim that the prior rejection "never tested a divisor
matched to NGN's actual value distribution WITH a ±2 clamp" is wrong about the clamp, and the distribution
measurement above shows /2048 was in fact well inside the live value range.

**Both reopen rationales in the T6 spec are therefore void.** The "int-trunc no-op" phrase originated in the
2026-06-28 result line itself and was carried forward into the revamp doc as if it were a diagnosis; it was a
guess, and it is now measured false.

## FINDING 2 — but the prior evidence is statistically empty, so the lane is still open

The 2026-06-28 verdict reads "REJECT ... worse both ways", but the numbers behind it are:

| variant | result | games |
|---|---|---|
| `/2048` | -8 Elo **[-44, +28]** | **358** |
| `/256` | -45 Elo [-88, -2] | **254** |

A ±36-wide CI at 358 games cannot distinguish a +5 keep from a -20 loss; it is compatible with essentially
any outcome in the gate's range. It is also **pre-reset** (base `2dadef8`, dated the reset day), and per
CLAUDE.md pre-reset game verdicts are historical evidence only and cannot close a lane.

So T6 stays open — **not for the reasons the revamp doc gave, but because the prior test measured nothing.**
That is a weaker and more honest reopen than the one on the books.

## FINDING 3 — the doc's prescribed divisor would have silently reversed the term's direction

This is the load-bearing result. Mean signed reduction delta, positive = reduce MORE = shallower:

```text
CURRENT rule (+/-1 @ -500/+1000) : +0.2635 plies per LMR node
```

| divisor | nonzero% | differs from current% | mean delta | vs current |
|---|---|---|---|---|
| 512 | 79.49% | 67.40% | +0.3629 | +0.0993 |
| 650 | 75.86% | 61.27% | +0.3122 | +0.0487 |
| 700 | 72.47% | 60.22% | +0.2837 | +0.0201 |
| **750** | **72.47%** | **57.55%** | **+0.2701** | **+0.0066** |
| 800 | 69.24% | 56.41% | +0.2410 | -0.0226 |
| 1024 | 63.07% | 49.29% | +0.1568 | -0.1067 |
| **1975 (doc's P90/2)** | ~38% | ~47% | **~-0.07** | **~-0.33** |
| 2048 | 37.20% | 47.44% | -0.0692 | -0.3328 |
| 5000 (Counter's) | 6.42% | 69.53% | -0.0720 | -0.3356 |

**The doc's prescribed `divisor ≈ P90/2 = 1975` lands essentially on the already-tested /2048** — and at that
divisor the change is not a smoothness refinement at all. It **reverses the net direction** of NGN's
history-LMR term, from net-reduce-more (+0.264) to net-reduce-less (-0.069): a -0.33 plies/LMR-node swing.

So T6 as specified conflates two independent changes:

- **(a) shape** — step function to continuous ramp (the stated hypothesis);
- **(b) level** — a large net shift in how much LMR reduces overall (unstated, and the dominant effect).

Run together, neither a positive nor a negative verdict is interpretable. This is the same failure mode
T18b and T20 both taught: porting a donor formula imports the donor's calibration along with it. Counter's
`/5000` is matched to Counter's ±49k EMA range, its own LMR base table, and its own ordering quality.

**Both prior data points are consistent with the T3a mechanism, which explains the old result without any
int-truncation story:** `/256` (far MORE reduction than current) measured **-45**, and T3a (more reduction
via cutNode) measured **-7.9**, both matching the recorded diagnosis that at FMC ~84% extra reduction
over-reduces mis-ordered moves that then fail low silently. `/2048` (net LESS reduction) measured -8, i.e.
flat within its huge CI. The old experiment bracketed the *level* axis and never isolated the *shape* axis.

## The re-derived candidate — T6a, level-matched continuous scaling

**`reduction -= clamp(histScore/750, -2, +2)`**, replacing the -500/+1000 step rule.

Divisor **750** is chosen because it holds the mean reduction delta at **+0.2701** against the current
**+0.2635** — a residual of +0.0066 plies/LMR-node, i.e. the net reduction level is preserved and the
experiment isolates **shape alone**. It is not a donor constant and not a percentile heuristic; it is fitted
to NGN's own measured distribution to null out the confound.

Behavioral delta is large — nonzero on **72.47%** of LMR evaluations, differing from the current rule on
**57.55%** — so this is nowhere near the T7/T1f inertness failure mode, and unlike a time-management change
it **is** measurable by the standard fixed-depth nodecheck gate (LMR runs at fixed depth). Nodecheck must be
run and must move ≥~1% before this earns box time.

**Honest expectation: low single digits, plausibly zero.** Smoothing a step function that already fires on
75% of nodes is a second-order refinement, the donor prior does not transfer (different history form,
different LMR base, different ordering quality), and the one prior measurement — however underpowered —
did not point positive. It ranks where it does because the alternatives ranked lower after the audit
correction, not because it has a strong prior.

**A cheap companion falls out of the same table and should NOT be bundled:** the *level* axis has never been
tested in the de-reduction direction on a post-reset base. T3a proved more reduction hurts at 84% FMC; the
complement — less reduction — is a one-constant edit to the existing thresholds. If T6a is inconclusive, that
is the follow-up, as a separate change.

## Caveats

1. Fixed depth 11 on 200 corpus positions, single-threaded, local machine. Real games at 10+0.1 reach
   varying depths and the history tables warm differently across a full game; the distribution could shift.
   The direction-reversal result is driven by the bulk of the distribution and is unlikely to flip, but the
   exact level-matching divisor (750) is a point estimate from this sample, not a tuned constant.
2. History tables here warm from a single search per position rather than across a game's move sequence, so
   early-search evaluations are over-represented relative to real play.
3. Mean reduction delta is a per-LMR-node average; it does not account for where in the tree the change
   lands, and a ply of reduction at depth 3 is not worth a ply at depth 12.

---

# T6a VERDICT — **PROXY REJECT, 2026-07-26.** Never reached the box. Zero box cost.

T6a was built (`output/t6a-lmr-conthist750.patch`, `LMR_HIST_DIV = 750`), gated locally, and **rejected on
the proxy before earning box time.** CLAUDE.md sanctions a proxy rejection here: this is a pure
ordering/EBF-mechanism change (LMR reduction shape — no eval values, no time, no TC behavior), judged on
depth-at-fixed-nodes, which is the instrument this lane is explicitly steered by. Reopen condition recorded
below, as required.

## Gate 1 — behavioral delta: PASSES (>= ~1%)

Fixed-depth nodecheck vs the HEAD lock (346662 / 149587 / 765656): **-5.0% / +35.9% / +21.0%**
(329180 / 203292 / 926167). Nowhere near the T7/T1f inertness failure mode.

## Gate 2 — depth at 400K fixed nodes: **FAILS, -4 net plies**

| position | base | T6a | delta |
|---|---|---|---|
| quiet-mg-QGD | 14 | 14 | 0 |
| quiet-mg-closed | 12 | **13** | **+1** |
| lateMg-ph6-11 | 13 | 13 | 0 |
| najdorf-mg | 14 | 13 | **-1** |
| rook-eg-R4P | 17 | 16 | **-1** |
| pawn-eg | 32 | 29 | **-3** |

**Net -4 plies.** This is the same figure that deprioritized T19 (commit `78b9ebb`: "T19 costs 4 net plies
versus T20's plus 3"), so the pre-set ranking rule applies unchanged.

**It also contradicts T6's founding hypothesis directly.** The 2026-06-28 manifest stated the aim as
"finer history-scaled reductions **search deeper at the same time** ⇒ NEW > BASE." Measured, it searches
**shallower**.

## Why the level-matching repair does not exist — the load-bearing negative

The mean-reduction-matched divisor (750) still grew the tree, because **tree size is convex in reduction**:
at a constant *mean*, a higher-*variance* reduction schedule grows the tree, since reducing one ply less
near the root costs far more than reducing one ply more near the leaves saves. The continuous scheme applies
±2 where the step rule only ever applied ±1, so variance rises even with the mean held.

The obvious repair is to level-match on **tree size** (the confound that actually matters) rather than on
mean reduction. Swept, `LMR_HIST_DIV` 400 → 1400, nodecheck % vs the HEAD lock:

| divisor | kiwipete | mid | **end** |
|---|---|---|---|
| 400 | +8.1% | -20.9% | **+10.2%** |
| 512 | +13.0% | +86.8% | **+38.5%** |
| 650 | +3.6% | -2.4% | **+23.1%** |
| 750 | -5.0% | +35.9% | **+21.0%** |
| 900 | +10.0% | +23.4% | **+19.2%** |
| 1024 | -1.7% | -15.1% | **+24.9%** |
| 1400 | +8.5% | -36.6% | **+102.9%** |

**No divisor holds tree size neutral. The endgame tree grows at EVERY setting across a 3.5x divisor range
(+10% to +103%).** So the shape axis cannot be isolated from the level axis by choosing a divisor — the
±2 spread grows the tree at every level. The repair proposed mid-investigation is not achievable.

(The kiwipete/mid columns swing non-monotonically and are NOT fitted to: that is the known artifact class
recorded in `project_endgame_tree_diagnostic` — single-position node counts are unreliable for pruning
knobs. The endgame column's uniform sign across the whole sweep is what carries the conclusion.)

## Verdict

**REJECT T6a on proxy.** Four independent strands, all negative, all at zero box cost:

1. Depth at fixed nodes **-4 net plies** — the hypothesis's own stated metric, falsified.
2. Tree grows at **every** divisor 400-1400; no level-neutral configuration exists.
3. Prior game evidence, though underpowered, was negative in sign both ways (-8 at /2048, -45 at /256).
4. Both of the revamp doc's rationales for reopening the lane are measured false (Findings 1 and 3 above).

**REOPEN CONDITION (predeclared, required by CLAUDE.md).** This rejects the **shape** axis — continuous
±2 scaling — on a measured mechanism: the ±2 spread inflates the tree irrespective of level. It does **not**
close the history-LMR term as a whole. The **level** axis is untouched and untested post-reset: the
`-500 / +1000` thresholds are unmeasured constants, and T3a (-7.9) plus `/256` (-45) together establish only
that *more* reduction is harmful at FMC ~84% — the *de-reduction* direction has never been tried on a
post-reset base. That is a one-constant edit, nodecheck-gateable, and is the surviving candidate from this
lane. Reopen the shape axis only if an ordering fix raises FMC toward ~90% (which would change what
over-reduction costs), or if a variant constrains the adjustment to ±1 so tree variance is not increased —
noting that ±1 continuous is very close to the existing rule with moved thresholds, i.e. it collapses into
the level axis anyway.

Engine tree reverted; all three nodecheck baselines restore exactly (346662 / 149587 / 765656);
`go test -short ./engine` green.
