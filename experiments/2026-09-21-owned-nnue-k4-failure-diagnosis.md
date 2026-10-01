# Owned K4 NNUE failure diagnosis and next-run plan

Status: **20M expansion remains HELD.** The next GPU training job should be a
schedule-isolation probe on the existing frozen 1M corpus, not the old 20M
recipe. Before that probe, finish the omitted teacher-quality check and measure
the selected network against the teacher, HCE and borrowed Rodent on the frozen
calibration positions. This document is a plan and evidence review; it did not
touch WSL, start a process, consume the GPU, or alter the Lean hopper.

## Bottom line

The 3.75% pilot result is a real model failure, not a flaky match. Its nominal
score corresponds to roughly -564 Elo, although 40 games are far too few for a
precise Elo estimate. The result is too large to dismiss as evaluator speed
alone.

The highest-probability diagnosis is **severe optimization/data starvation,
made opaque by an underpowered validation summary**:

- The network has 2,372,360 deployed parameters and 589,824 additional
  factorizer parameters during training, but saw only 1,000,000 unique training
  inputs and 1,024 optimizer updates. That is 0.338 unique inputs per trainable
  parameter and 16,777,216 total presentations.
- The selected checkpoint was the final scheduled checkpoint. Integer
  validation MSE fell from `0.0269574337493` to `0.0113931967397`, and early
  stopping did not fire. The run proved that learning and export worked; it did
  not show convergence or an adequate absolute error.
- The pinned Bullet bucketed example uses the same batch size but 6,104 batches
  per superbatch for 640 superbatches: 3,906,560 updates and
  64,005,079,040 presentations. The pilot was 3,815 times shorter in updates;
  even the old planned 20M schedule was 238.4 times shorter. This example is a
  scale reference, not a claim that its exact schedule is mandatory.
- Current official `nnue-pytorch` defaults provide a second scale sanity check:
  batch 16,384, 100,000,000 positions per epoch and up to 800 epochs. Defaults
  are not a production prescription, but they make a 1,024-step training horizon
  look like a smoke test rather than a serious convergence attempt.
- The selector optimized only global WDL-space mean squared error. It did not
  report sign accuracy, score calibration, rank correlation, move agreement, or
  errors by king bucket and score band. The validation head counts previously
  read from the frozen selector receipt were
  `[258, 10433, 19914, 20112, 18266, 17432, 11566, 2019]`; the lowest-material
  head therefore contributed only 0.258% of validation. A good global mean can
  hide a broken rare head or bucket.

There are two important co-factors rather than one clean root cause:

1. K4 evaluation costs roughly four times HCE at fixed nodes. That is a large
   real-clock handicap, but it is not a credible sole explanation for 37 losses,
   three draws and no wins.
2. Labels are 100% 5,000-node Stockfish score and 0% game result. That is a
   defensible first distillation target, but the prospectively required
   1,000-position 5k-versus-20k comparison was never completed. Label noise is
   therefore an unresolved precondition, not a theory to hand-wave away.

Do not simultaneously change architecture, labels, data balance, optimizer and
search. First use the cheap discriminators below to identify which axis failed.

## What the evidence rules down

| Explanation | Present assessment | Evidence or remaining falsifier |
| --- | --- | --- |
| Match/harness failure | Very unlikely | All 40 games completed; chess and operational audits passed; every decisive game ended in natural mate. |
| Gross color/sign inversion | Unlikely | HCE scored 19.0 points as White and 19.5 as Black; Bullet, independent-float, quantized-Go and incremental paths agree. Still run transformed-position invariants because parity can consistently reproduce a bad declared contract. |
| Export, transpose or feature-map corruption | Unlikely | Full-tensor quantization, strict model conversion, all-head fixtures and runtime parity passed. |
| “NGN search cannot use NNUE” | Ruled down strongly | The same NGN search with the imported Rodent V1.1 Anand evaluator beat the Counter backend by +96.19 Elo `[+69.53,+123.02]` in 400 audited games. |
| K4 architecture is inherently unusable | Not supported | It is a credible bucketed/factorized shape and its transport works. No architecture conclusion follows from a starved run. |
| Inference cost alone | Contributing, not sufficient | K4 is about four times HCE cost, but equal-node evidence is still missing and the real-clock loss is extreme. |
| Search/eval scale mismatch | Plausible secondary cause | NGN pruning and correction policy were tuned with HCE. Compare static scale and equal-node/equal-time behavior before retuning search. |
| Weak/noisy target | Plausible and unclosed | The mandated 5k/20k teacher comparison is outstanding; global MSE has no HCE or borrowed-net baseline. |
| Insufficient unique coverage | High probability | One million inputs train about 2.96M parameters; sparse material heads and likely sparse king buckets amplify the problem. |
| Insufficient optimizer horizon | Highest-probability actionable cause | Only 1,024 updates, final checkpoint selected, no early stop, and external trainer scale references are orders of magnitude larger. |

The borrowed Rodent control is especially useful. It shows that NGN's current
search can convert a strong NNUE into strong play. K4-specific scale/cost coupling
can still matter, but a general search-versus-NNUE incompatibility cannot explain
the owned result.

## Why validation gave false comfort

`0.01139` is not “low” without a meaningful baseline. It is merely lower than a
randomly initialized model on the same target. Near a 0.5 target, its probability
RMSE of about 0.107 corresponds, by local linearization, to roughly 170 score
units. That conversion is only an intuition because errors are not all central,
but it shows why improvement over initialization is not evidence of a useful
evaluator.

The current mean also gives abundant middlegame heads almost all the vote. It
does not answer any of these chess questions:

- Does the model usually get the sign right outside the drawish band?
- Does it order positions and candidate moves like the teacher?
- Is its score slope compatible with NGN's pruning margins?
- Do low-material heads, king buckets, sides to move, or large-score tails
  collapse?
- Does K4 beat HCE at the same node budget and lose only after paying its speed
  cost?

Those omissions are why the selector can be mechanically correct while the
selected net loses essentially every game.

## Stage A: no-training autopsy when WSL becomes available

These checks are bounded and mostly CPU work. Run them before occupying the GPU
or labeling another 19M positions.

1. **Complete the skipped label-quality gate.** On the frozen 1,000-position
   calibration sample, produce exact SF18 labels at 5,000 and 20,000 nodes.
   Report WDL-target mean/median/p90 absolute difference, sign disagreement at
   `|teacher| >= 50` and `>= 100`, and Spearman rank correlation. Preserve the
   original plan's hold at mean absolute target difference above 0.05. A failure
   means the next pilot is relabeled at a justified node budget before any
   schedule experiment.
2. **Static calibration audit on all 100,000 frozen calibration positions.** For
   candidate 0, selected candidate 1,024, HCE and borrowed Rodent, report score
   MAE/RMSE, bias, robust slope/intercept, Pearson/Spearman correlation and sign
   accuracy against the existing teacher label. Break each metric down by side,
   output head, the STM/NTM king-bucket pair, piece count, source ply and
   teacher-score bin.
   Include bootstrap uncertainty; do not rank 258 examples as if they were
   20,000.
3. **Semantic invariants.** Evaluate paired original and color-swapped/rank-flipped
   states under the actual score contract. Check Bullet float, independent float,
   quantized refresh and incremental outputs. This is a final falsifier for a
   consistently implemented but wrongly specified orientation.
4. **Cost/quality separation.** On 32 frozen positions, compare HCE, K4 and
   borrowed Rodent under equal nodes and equal wall time. Record best move,
   teacher-best-move agreement, score, reached depth, nodes, elapsed time,
   evaluator calls and ns/node. This is diagnostic, not fixed-node Elo.
5. **Loss autopsy.** From a score-unselected sample of 12 decisive pilot games,
   locate the first large K4-versus-teacher divergence and classify it by head,
   bucket, material and whether HCE/Rodent agreed with the teacher. Do not use
   these positions for checkpoint selection.

Decision after Stage A:

- Any semantic failure: repair only that contract, retrain the 1M pilot from
  scratch, and repeat parity before games.
- 5k/20k gate failure: relabel the pilot; do not test a longer schedule on a
  target already shown unstable.
- K4 competitive at equal nodes but disastrous at equal time: optimize the K4
  evaluator before buying data.
- K4 poor at equal nodes and static metrics: proceed to the schedule-isolation
  probe below.

## Stage B: the next GPU run — K4-S1 schedule-isolation probe

Use the exact existing 1M training BF, 100k validation BF, architecture, seed,
target, batch size, AdamW parameters, clipping, quantization and data order.
Cold-start from the same initialization. Change exactly one recipe family: the
optimization horizon and its cosine schedule.

| Setting | K4 pilot (failed) | K4-S1 probe |
| --- | ---: | ---: |
| Unique training inputs | 1,000,000 | 1,000,000, identical bytes |
| Batch size | 16,384 | 16,384 |
| Maximum updates | 1,024 | 16,384 |
| Maximum presentations | 16,777,216 | 268,435,456 |
| Presentations per unique input | 16.8 | 268.4 |
| Validation interval | 128 | 512 updates |
| Initial/final LR | .001 / .00005 | .001 / .00005 over the longer horizon |
| Minimum before early stop | 256 | 4,096 updates |
| Early-stop patience | 4 checks | 8 checks |

At the measured 6.94M presentations/second, raw training work is about 39
seconds; validation/export overhead will dominate. Keep the existing resource
supervisor and wall cap. This is a cheap causal experiment, not a promotion
candidate by default.

Select by integer validation MSE as before, but produce the full Stage-A metric
panel for every retained checkpoint. A global-MSE win accompanied by a sparse
head/bucket collapse is not improvement. Compare the selected K4-S1 checkpoint
with candidate 1,024 on the same 32-position equal-node/equal-time panel.

K4-S1 outcomes:

- **Clear continued learning:** materially better global and stratified static
  metrics, with improved equal-node behavior. Run a fixed-node paired screen
  before real-clock games. If it then reaches the existing 35% real-clock gate,
  authorize the 20M data expansion.
- **Validation overfit/plateau:** the best checkpoint occurs early and static
  chess metrics do not improve. The bottleneck is unique data/coverage or target,
  not optimizer steps. Label a 5M intermediate corpus rather than spending 20
  hours immediately.
- **Static improvement without game improvement:** investigate score calibration,
  move ordering and inference cost. Do not “solve” it with more positions.

## Stage C: data-isolation fallback if K4-S1 plateaus

Use 5M unique accepted inputs: the existing 1M prefix plus 4M new inputs under
the unchanged accepted-set contract. At the measured label rate this is roughly
3.2 hours plus frozen slack, far cheaper than the 19M expansion. Train for at
most 32,768 updates (536,870,912 presentations, 107.4 presentations per unique
input), validate every 1,024 updates and retain the same target and architecture.

This stage asks one clean question: does broader unique coverage fix the static
and fixed-node deficit when the optimization budget is no longer trivial? If it
does not, stop scaling this recipe and revisit target construction or data
balance. If failures concentrate in rare heads/buckets, a prospectively frozen
stratified sampler or loss weighting is justified as a later single-axis test.

## Stage D: serious 20M candidate after a passing discriminator

The old 16,384-update main schedule is withdrawn. Once Stage A passes and either
K4-S1 or the 5M fallback supplies positive static/fixed-node evidence, use:

| Setting | K4-M1 |
| --- | ---: |
| Unique training inputs | 20,000,000 |
| Batch size | 16,384 |
| Maximum updates | 131,072 |
| Maximum presentations | 2,147,483,648 |
| Presentations per unique input | 107.4 |
| Validation/export interval | 4,096 updates |
| Early stop | after 6 non-improving checks, not before 32,768 updates |
| LR | AdamW cosine, .001 to .00005 |

This is eight times the old main optimization budget and still conservative
relative to the pinned Bullet scale reference. At measured smoke throughput the
raw GPU work is only about 5.2 minutes; the approximately 20-hour incremental
teacher-label run, not GPU training, is the expensive decision. The validation
curve may stop the run early.

Keep 100% teacher score for K4-S1 and K4-M1. Official NNUE documentation and the
Bullet example both make result blending a credible later experiment, but the
current BF encodes a constant draw because result was intentionally ignored.
Changing the blend now would silently train toward fake draws. A result-blend
experiment requires trustworthy game-result provenance and a new frozen target
contract.

## Strength gates

1. Static and semantic audits pass; no hidden head/bucket collapse.
2. Teacher-label quality gate passes, or the corpus is prospectively relabeled.
3. Equal-node evidence improves enough to justify timing-sensitive play.
4. A 40-game paired HCE screen uses the existing honest cutoff: below 35% holds
   the recipe; at least 35% permits the next expenditure but does not claim
   superiority.
5. A 20M candidate scoring at least 50% proceeds to the already specified
   400-game confirmation. The owned-network milestone still requires a paired
   lower confidence bound above 50%, then comparison with borrowed Rodent/SF18.

Do not retune search margins to rescue a weak evaluator before equal-node model
quality is established. Do not change target blend, architecture or sampling in
the schedule probe. Do not discard the 3.75% result or choose a runner-up after
seeing games.

## Source cross-checks

- [Pinned Bullet bucket/factorizer example](https://github.com/jw1912/bullet/blob/629ee50000b2afb7b3337595401c830d3b1e0f42/examples/progression/3_input_buckets.rs)
  supplies the 6,104-by-640 schedule scale, batch 16,384, factorizer merge,
  material heads and SCReLU reference.
- [Official Stockfish nnue-pytorch configuration](https://github.com/official-stockfish/nnue-pytorch/blob/master/config.py)
  supplies the current 100M-position epoch, batch-16,384 and 800-epoch defaults.
- [Official NNUE loss documentation](https://github.com/official-stockfish/nnue-pytorch/blob/master/docs/nnue.md#loss-functions-and-how-to-apply-them)
  explains engine/data-dependent WDL scaling, evaluation/result interpolation
  and why the useful mixture is empirical rather than universal.
- Local evidence: [pilot selection](2026-09-21-owned-nnue-pilot-selection.md),
  [failed strength screen](2026-09-21-owned-nnue-k4-pilot-screen.md),
  [shared-machine smoke](2026-09-20-owned-nnue-smoke.md),
  [teacher throughput gate](2026-09-20-owned-nnue-label-throughput.md), and
  [borrowed Rodent strength control](2026-09-13-rodent-counter-strength.md).
