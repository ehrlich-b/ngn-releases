# 2026-07-26 Corrector key ranking — one measurement that orders every remaining corrhist candidate

Measured locally while T20 held the box. **Zero box cost.** Instrumentation fully reverted; nodecheck
baselines restore exactly; suite green. Shelf copies: `output/t4-keyrank.patch`, `output/t4-keyrank.go.txt`,
`output/t4-keyrank_test.go.txt`.

Extends the T4d pre-flight instrument (`2026-07-26-t4d-continuation-corrhist-preflight.md`) from one
candidate key to **nine**, so a single run ranks every plausible fifth/sixth corrector instead of building
and gating them one at a time.

## Method

Same gate the real correctors use, same clamp (`corrHistLimit` = 1024), 5 games x 60 plies at depth 12,
**1351031 residual observations** of `bestScore - corrStaticEval` (i.e. what the three shipped correctors
have already failed to absorb). Statistic: one-way-ANOVA R² of per-bucket means.

**Two controls at different cardinalities both land at exactly 1.0x** the analytic chance floor
`(B-1)/N` — 832 buckets → 0.00059 vs 0.00062 predicted; 16384 buckets → 0.01214 vs 0.01213 predicted.
The floor model is validated at both ends, so cross-key ratios are trustworthy.

## Result

| key | R² | buckets | chance floor | ratio | status |
|---|---|---|---|---|---|
| **material signature** | **0.29025** | 7129 | 0.00528 | 55.0x | **untried — strongest total signal** |
| material x phase (joint) | 0.29233 | 7060 | 0.00522 | 55.9x | confound probe |
| cont12 prev+prev2 combined | 0.13828 | 16166 | 0.01196 | 11.6x | T4d reopen #2 |
| nonPawn WHITE only | 0.07607 | 15845 | 0.01173 | 6.5x | untried split |
| **PHASE alone (accPhase)** | **0.06608** | **26** | 0.00002 | **3571.1x** | **see below** |
| cont1 prevMove (T4d as built) | 0.06515 | 708 | 0.00052 | 124.5x | **BUILT + STAGED** |
| cont2 prev2 | 0.05731 | 703 | 0.00052 | 110.3x | T4d reopen #1 |
| nonPawn BLACK only | 0.03369 | 2500 | 0.00185 | 18.2x | untried split |
| both king squares | 0.01829 | 1188 | 0.00088 | 20.8x | new idea |
| CONTROL (832) | 0.00059 | 832 | 0.00062 | **1.0x** | floor |
| CONTROL (16384) | 0.01214 | 16384 | 0.01213 | **1.0x** | floor |

## Findings

### 1. T4d's built form is the right choice among continuation variants — its own reopen list is now ranked

`cont1` (opponent's previous move, the form built and staged) at **124.5x** beats `cont2` (our own previous
move, reopen variant #1) at 110.3x on both raw R² and ratio. `cont12` (combined, reopen variant #2) has
higher raw R² (0.138) but that is a cardinality artifact — at 16166 buckets its ratio collapses to 11.6x,
i.e. most of its apparent signal is chance. **T4d's predeclared reopen order should be revised: variant #2
(combined) is worse per parameter than variant #1 (prev2), and neither beats what is already built.**

### 2. Material signature is the strongest untried key — and it is NOT a phase proxy

Material explains **29.0%** of the post-correction residual, **4.4x the continuation key's excess over
chance** (0.285 vs 0.065). The obvious alternative explanation is that material merely encodes game phase,
and residual size varies by phase. **Tested and rejected**: phase alone explains 6.6%, and the joint
`material x phase` key scores 0.29233 versus material's 0.29025 — **an increase of 0.002**, confirming phase
is fully determined by material (as it must be, being computed from it). So material's signal *contains*
phase's, leaving **~22.4 points of genuine material-configuration structure beyond phase** — i.e. learned
imbalance correction, which is what strong engines get from an explicit imbalance table and NGN does not have
(`TODO.md` lists "material imbalance/pawn-count scaling" as an untried eval-shape item).

Material corrhist was in the 2026-06-23 aux bundle, but that run was INVALIDATED mid-flight by the
`ClearHistoryTable` leak and sat on the mis-plumbed pre-T4 base, so it has **never been validly tested**.

### 3. The most interesting result is PHASE — and it probably should NOT be a corrector

**6.6% of residual variance from 26 buckets (3571x chance).** That is as much total signal as the
continuation key now staged, from **27x fewer parameters**. Because the statistic measures between-bucket
**mean** shift (not variance), this says NGN's static eval carries a **systematic, phase-dependent bias**.

A 26-bucket corrector would warm up almost instantly and absorb it — but that is papering over the symptom.
A systematic per-phase mean error is more naturally a **calibration defect in the mg/eg taper**, which is a
direct eval fix rather than a fifth table. **This should be investigated as an eval-calibration question
first**, and only turned into a corrector if the taper turns out to be correct and the bias real.

### 4. The non-pawn split idea is dead

`nonPawnW` 6.5x and `nonPawnB` 18.2x are both far below the combined non-pawn key already shipped. Splitting
the current combined key into per-colour tables is **not** promising and should be dropped from the queue.

## Caveats

1. **R² is not Elo.** These rank *available signal*, not strength gain. The corrhist family's realised series
   is +9.1 → +6.4 → +3.7, strictly diminishing, and nothing here predicts that trend breaks.
2. Every figure is *post*-correction residual — signal the three shipped correctors did not absorb. A key
   with high residual R² may still be partly redundant with corrections already applied.
3. 5 games at fixed depth 12 on a local machine, fresh tables per game. The update gate is asymmetric
   (fail-highs and fail-lows enter on different conditions), which inflates the overall mean residual
   (+51.73 cp) and could contribute apparent structure; the **controls bound the chance component but not
   this gate asymmetry**.
4. The king-square key was initially measured with a broken bitboard-to-square extraction (33 live buckets,
   a meaningless 69x); it was fixed to `TrailingZeros64` and re-measured at 20.8x. The broken figure is
   recorded here only so it is not mistaken for a result.

## Revised queue for the corrhist family

1. **T4d — continuation (cont1)** — BUILT, GATED, STAGED. Runs next when T20 resolves.
2. **T4e — material corrhist** — strongest untried key by a wide margin, mechanism now separated from phase.
3. **Phase-bias investigation** — NOT a corrector first: check the mg/eg taper for a calibration defect.
4. ~~non-pawn W/B split~~ — DROPPED, measured weak.
5. King-square corrector — weak (20.8x, R² 0.018), park it.
6. T4d reopen variants (cont2, cont12) — both measurably worse than what is built; only if T4d fails.

---

# ADDENDUM — the shape of the phase bias, and a CORRECTION to Finding 3

Finding 3 above proposed that PHASE's 3571x signal points at an **mg/eg taper calibration defect**.
**Measured, and that framing is wrong.** Mean residual per phase bucket (cp, buckets with n >= 1000):

| phase | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 | 11 | 12 |
|---|---|---|---|---|---|---|---|---|---|---|---|
| mean cp | 44 | **120** | 40 | 52 | 84 | **124** | 35 | 35 | 96 | **131** | 33 |

| phase | 13 | 14 | 15 | 16 | 17 | 18 | 19 | 20 | 21 | 22 | 23 | 24 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| mean cp | **161** | 26 | **155** | 77 | 24 | **120** | 31 | 46 | **125** | 24 | **125** | 20 |

Grand mean 51 cp, N = 1351031.

**A taper miscalibration would produce a smooth monotonic drift with phase. This is non-monotonic with a
clear parity structure**: odd-phase buckets average **~99 cp** residual, even-phase **~54 cp** — nearly 2x.

With the standard phase weights (minor = 1, rook = 2, queen = 4), **an odd phase means an odd number of
minor pieces on the board, i.e. a material imbalance.** So the eval is roughly twice as wrong when material
is imbalanced as when it is symmetric. That is not a taper problem — it is a **missing imbalance term**,
which is precisely what a material-keyed corrector absorbs and precisely the gap `TODO.md` already lists
("material imbalance/pawn-count scaling", untried).

**Consequences:**

- **Finding 3 is corrected**: do NOT open an mg/eg taper investigation on this evidence. The bias is
  material-structured, not phase-monotonic.
- **T4e (material corrhist) is strengthened**, not merely confirmed: the mechanism by which material carries
  ~22.4 points of beyond-phase signal now has a concrete shape — imbalanced material.
- A **direct imbalance eval term** becomes a sibling candidate to T4e (learn it vs encode it). T4e is the
  cheaper first test because it needs no new eval tuning and rides proven corrhist plumbing.

**Caveats — this is suggestive, not established.** The parity pattern has real exceptions (phase 17 and 19
are odd and LOW at 24/31 cp; phase 18 is even and HIGH at 120 cp), so parity is not the whole story. The
residual gate is asymmetric (fail-highs and fail-lows enter on different conditions), which biases the
overall level (+51 cp) and is uncontrolled here — the controls bound chance variance, not gate asymmetry.
Low-residual buckets also carry far more samples (phase 24: 235727; phase 3: 11380), so the high-residual
buckets are the noisier ones. Treat this as a mechanism hint that ranks T4e, not as a measured eval bug.
