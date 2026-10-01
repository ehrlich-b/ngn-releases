# 2026-08-04 T8 SPSA round 1 (T17b fixed base) — RUN COMPLETE. Converged vector, NOT a verdict.

**T8 IS A TUNER, NOT A VERDICT.** No keep/shelve may be read from this file. The vector below requires its
own real-clock SPRT before it means anything — see the post-run runbook in
`experiments/2026-08-01-t8-base-decision.md`.

## Run

- Started 2026-08-03 07:19:34, finished 2026-08-04 13:26:48 — **30h07m**, matching the corrected ~30h
  estimate (807 g/hr measured) and **not** the ~25h originally assumed nor the ~57h I briefly and wrongly
  published.
- `engine=ngn_t8base.exe` `c67200c1…` (HEAD = T12+T19, **plus `output/t17.patch`**), tuner `spsa.exe`
  `f26a2814…`, **`params=14`**, 1500 iters x 8 pairs x 2 games = **24000 games** (clears the >=20k policy
  floor), 60 checkpoints, **0 forfeits throughout**, DONE_EXIT_0.

## Converged vector

| param | default | converged | change |
|---|---|---|---|
| **ReverseFutilityMargin** | 120 | **88** | **-27%** |
| **SingularMargin** | 64 | **48** | **-25%** |
| LMPBase | 5 | **4** | -20% |
| NullMoveMinDepth | 3 | **2** | -33% |
| DeltaMargin | 100 | **96** | -4% |
| FutilityMargin | 100 | **103** | +3% |
| ExtensionBudget | 24 | **26** | +8% |
| AspInit | 12 | **13** | +8% |
| AspMult | 150 | **154** | +2.7% |
| NullMoveR | 3 | 3 | — |
| LMRFullDepth | 3 | 3 | — |
| LMRMoveThreshold | 2 | 2 | — |
| FutilityMaxDepth | 8 | 8 | — |
| SingularDepth | 6 | 6 | — |

**9 of 14 moved.**

## Both pre-registered predictions held

1. **"The vector will NOT land within rounding of the defaults"** (recorded at iter 475, when only 4 rounded
   values had moved). **Confirmed** — 9 of 14 moved, two by 25-27%. So the pre-registered condition that
   would have closed the lane without a second round — *"if the converged vector lands within rounding of
   the defaults, the finding is already-near-a-local-optimum"* — **did not fire.**
2. **"`NullMoveR`'s converged value is noise"** (pre-registered after measuring its **97x sensitivity
   collapse** post-T12: span 45.0% -> 0.46% of default). **It converged to exactly its default, 3.** That is
   what a dim with no gradient does. **Do not report `NullMoveR = 3` as a tuned optimum — it is the absence
   of a result**, and it is the one value in this table that carries no information.

**The `AspMult` caution is NOT triggered.** It moved 150 -> 154 (+2.7%), well inside the range where T5's
games-validated aspiration shape is undisturbed. Its trajectory efficiency was 0.13 (random-walk-like), so
the small move is consistent with noise rather than a real pull.

**The two big movers are the two that showed directed trajectories throughout** (`ReverseFutilityMargin`
efficiency 0.84, `SingularMargin` 0.63 at iter 475). Both are **margin-loosening**: RFP prunes more
aggressively at 88 than 120, and a smaller singular margin makes more moves qualify as singular. Whether
that is real is exactly what the confirmation SPRT decides.

September 4 clarification: the earlier text reversed RFP's direction. Its gate
is `staticEval - margin >= beta`; decreasing the margin makes the cutoff easier.
This corrects the mechanism description, not the recorded tuning result.

## Next step and a sequencing note against my own rule

**The confirmation SPRT is: candidate = HEAD + `output/t17.patch` + this vector, base = unmodified HEAD.**
Do **not** compare tuned-vs-untuned on the T17 base — that measures the tune alone and misses the interaction
test the fixed base was chosen for. The bundle starts ~6.4 Elo down (T17 standalone = -6.4).

**T22 was launched first, which departs from the rule I wrote — and the rule's premise was wrong.** The
post-run runbook said *"if the vector moved materially, confirm it first, because leaving an unconfirmed
tuned vector in the tree would contaminate every later candidate's base."* **The vector is NOT in the tree.**
HEAD is untouched (T12+T19), so T22's base is clean and its measurement is unaffected. The contamination
concern I wrote was imagining an applied vector; it does not apply to a vector that exists only as numbers in
a log.

The practical case for T22 first is also stronger: **T22 was already staged and gated, while the T8
confirmation still needs building** (apply `t17.patch`, edit 14 constants, test, cross-compile, push). Doing
that carefully while T22 runs is the efficient order.

**One real consequence, recorded: if T22 KEEPS, HEAD changes and the T8 confirmation candidate must be
rebuilt on the new HEAD.** That is extra work, not invalidation.

```yaml
id: 2026-08-04-t8-spsa-result
date: 2026-08-04
change_class: tuner run (NOT a verdict)
result: >
  Converged vector, 9 of 14 params moved. Largest: ReverseFutilityMargin 120->88, SingularMargin 64->48,
  LMPBase 5->4, NullMoveMinDepth 3->2. 24000 games, 30h07m, 0 forfeits, DONE_EXIT_0.
predictions_held: >
  (1) vector did NOT land within rounding of defaults, so the close-the-lane condition did not fire;
  (2) NullMoveR converged to its exact default, consistent with its measured 97x sensitivity collapse -- its
  value carries no information.
next_action: >
  Build the confirmation candidate = HEAD + t17.patch + this vector; SPRT [-3,+3] vs unmodified HEAD.
  Rebuild on the new HEAD if T22 keeps first.
```
