# 2026-07-26 T4d pre-flight — continuation correction history carries real, unabsorbed signal

Measured locally while T20 held the box. **Zero box cost.** Instrumentation fully reverted; nodecheck
baselines restore exactly (346662 / 149587 / 765656); `go test -short ./engine` green. Shelf copies:
`output/t4d-corrprobe.patch`, `output/t4d-corrprobe.go.txt`, `output/t4d-corrprobe_test.go.txt`.

**This is the first POSITIVE pre-flight of the session** — T1f, T6a and T11a were all killed by their own
pre-flights on the same day.

## Why this candidate

Verified from code: NGN has exactly **three** correctors, all keyed on **position features**:

| table | key | status |
|---|---|---|
| `pawnCorrectionHistory` (moveorder.go:37) | both sides' pawn bitboards | KEPT **+9.1** |
| `nonPawnCorrectionHistory` (moveorder.go:109) | both sides' non-pawn occupancy | KEPT **+6.4** |
| `minorCorrectionHistory` (moveorder.go:159) | both sides' knight\|bishop occupancy | KEPT **+3.7** |

**The corrhist family is 3-for-3 — the only family in the campaign that has never missed** — and every
member keys on *what the position looks like*. The reference set's fourth corrector keys on something
categorically different: **the move sequence that reached the position** (previous move's piece × destination).
It is the one structurally distinct corrector, and it has never been tried.

**Checked against the ledger for a prior attempt, and there is none.** The 2026-06-23 "aux corrhist
(C-vs-B)" run was **material + nonPawnW + nonPawnB**, not continuation (`2026-06-23-corrhist-cvsb.md`), and
that run was in any case INVALIDATED mid-flight by a `ClearHistoryTable` cross-game leak and sat on the
mis-plumbed pre-T4 base. Two *other* untried variants also fall out of that record and are NOT bundled here:
material corrhist, and splitting the currently-combined non-pawn key into separate W/B tables.

## The question the pre-flight had to answer

The three existing correctors are already applied to `corrStaticEval`. So: **does the residual
`bestScore - corrStaticEval` still carry structure that a continuation key could learn**, or has the
position-feature family already absorbed it? If not, a fourth corrector has nothing to learn.

## Method

Instrumented both corrhist update sites (search.go:2121, 2154) under **the exact gate the real correctors
use** (`!inCheck`, best move not a capture, and the same bound condition), accumulating the residual under:

- the **continuation key** (`prevPiece*64 + prevTo` from `info.MoveStack[ply-1]`), 832 buckets;
- a **control key** of identical cardinality, derived from a counter — uncorrelated with the position;
- the **three existing corrector keys** (16384 buckets each), for calibration.

Statistic: fraction of total residual sum-of-squares explained by per-bucket means (a one-way-ANOVA R²).
4 games x 60 plies at depth 12, **1145273 residual observations**.

**The control is load-bearing**: with B buckets and N samples, chance R² ≈ (B-1)/N even with no real signal.
For B=832, N=1145273 that predicts 0.00073 — and the measured control was 0.00068-0.00071 across three
independent runs. The control construction is therefore validated, and every ratio below is against its own
analytically-correct floor.

## Result 1 — the signal is real and is NOT outlier-driven

Residuals include mate scores, so raw variance is dominated by tails. The real corrector clamps
(`corrHistLimit = 1024`), so the honest test is at that scale:

| residual clamp | residual SD | continuation R² | control R² | ratio |
|---|---|---|---|---|
| none | 1965.09 cp | 0.02304 | 0.00069 | 33.4x |
| **±1024 (`corrHistLimit`)** | 154.38 cp | **0.07122** | 0.00071 | **100.2x** |
| ±256 | 89.85 cp | 0.08227 | 0.00068 | 120.5x |

**The signal STRENGTHENS under clamping** — 33x → 100x → 121x. It is the opposite of an outlier artifact:
clamping removes mate-score noise and the continuation structure becomes clearer. At the corrector's own
operating scale the continuation key explains **7.1% of residual variance against a 0.07% chance floor**.

## Result 2 — the calibration against the three correctors that WORKED

Same clamped residual (±1024), same run:

| key | R² | live buckets | chance floor | **ratio over chance** |
|---|---|---|---|---|
| pawn (KEPT +9.1) | 0.05767 | 12787 | 0.01116 | 5.2x |
| non-pawn (KEPT +6.4) | 0.07200 | 16382 | 0.01430 | 5.0x |
| minor (KEPT +3.7) | 0.06689 | 12832 | 0.01120 | 6.0x |
| **continuation (UNTRIED)** | **0.07122** | **707** | 0.00062 | **115.5x** |

Two readings, and the second is the important one:

1. **Magnitude**: continuation's 0.0712 is squarely in the range of all three kept correctors (0.058-0.072).
   It carries a comparable amount of signal.
2. **Efficiency**: it does so with **20x fewer buckets** — 707 vs 12787-16382. Its ratio over chance is
   **115x versus their 5-6x.** The same explained variance from a twentieth of the parameters.

That efficiency is not cosmetic at this time control. With ~1.1M observations, a 707-bucket key averages
~1600 samples per bucket while a 16384-bucket key averages ~70. **A corrhist table only helps once its
entries have warmed up**, and at 10+0.1 the tables barely warm at all — the same "structure never gets
loaded" effect the T11a probe measured for the TT. A key that reaches useful occupancy 20x faster is
materially better suited to the gate's TC.

## Honest caveats

1. **The comparison is not apples-to-apples.** The existing keys' figures are *post*-correction residual —
   what they failed to absorb — while continuation's is entirely unabsorbed, since no such corrector exists.
   Continuation's number is therefore flattered. The mitigating fact is that all four sit in the same regime:
   the existing three still show 5-6x over chance on their own keys, because the gravity EMA and the
   6245/131072 weight deliberately under-correct rather than fully absorbing.
2. **R² is not Elo.** Explaining 7% of static-eval residual variance does not map linearly to strength. The
   diminishing series (+9.1 → +6.4 → +3.7) is the better guide to magnitude.
3. Fixed depth 12, 4 games, local machine, fresh tables per game. Game-to-game variance undersampled.
4. The continuation key here is `[prevPiece][prevTo]`. Alternative keyings (including prev2, or combining
   with stm differently) are not tested and could be better or worse.

## Verdict and expectation

**PROCEED to build T4d.** It is the best-motivated candidate remaining: the only family that has never
missed, the one structurally distinct corrector left in it, never attempted, with measured unabsorbed signal
at 100x its chance floor and 20x the parameter efficiency of the tables that already work.

**Honest expectation: +2 to +4.** The family's series is strictly diminishing (+9.1 → +6.4 → +3.7) and there
is no reason to expect a fourth corrector to break that trend; the parameter-efficiency argument is a reason
to think it lands at the top of that range rather than below it, not a reason to expect a large number.
**A +2-4 change most likely CAPS rather than crossing +2.94** (T4c at +3.7 only crossed at G7840), so the
probable outcome is a **capped-positive PROVISIONAL keep** — which is exactly what the batch-certification
path exists for, and **Batch 3 currently has 0 rows and is due 2026-08-08**.

Build gates before it earns box time: **nodecheck must move ≥~1%** (corrhist changes eval values, so it is
not node-identical and this gate applies), the `-short` and `-race` engine suites must be green, and
`ClearHistoryTable` must zero the new table — that omission is exactly what invalidated the 2026-06-23 aux
corrhist run mid-flight, and it needs a regression test.
