# Owned K4 frozen static calibration

Status: **K4 model-quality deficit confirmed, but its size alone does not
explain the 3.75% game score.** Equal-node and equal-time search diagnostics
remain the next discriminant. The 20M data expansion stays held.

## Inputs and method

This audit used all 100,000 accepted positions of the previously finalized
calibration corpus. It verified the finalizer manifest, calibration BF, both
label shards and the exact selected model files before scoring. All three
evaluators ran through NGN's `EvaluateSelected` base SearchSTM API on the same
halfmove-reset FENs, without search correction history. Teacher scores are the
frozen 5,000-node SF18 labels in NGN score units. [The independent teacher
quality audit](2026-09-21-owned-nnue-teacher-quality.md) bounded the 5k/20k
mean target difference below its predeclared threshold.

- Finalized pilot manifest SHA-256:
  `753e8ed1fbe09bef290fcba7209dfb77dce0170e80a5a09a22a1c3c12599755e`.
- Calibration BF SHA-256:
  `98de4eb3382c6face5cd92d9194d3271ec0fb6aff1ee30405f94846ddfaceb60`.
- Selected K4 model SHA-256:
  `034559653a83a7e64d4407badff334f66e3eae6a3a1f33147e3516b3c88c3e69`.
- Borrowed Rodent V1.1 Anand network SHA-256:
  `5f7480b56538e9e64ee02fb5b4842bf0394ee44b707dae3a63ec7f902abd0afb`.
- Authoritative WSL directory:
  `/home/ehrli/nnue-owned-k4-20260920/runs/k4-static-calibration-20260921`.
  `predictions.csv` SHA-256 is
  `e6964525ae130e4024421eadfaf3d163eb8a6e017aa71a23a69483c44fb5ed29`;
  its receipt SHA-256 is
  `35b8b2fd19a267368c034bfbe6704e7fdc15c8ba1ca9ebb303311c6f976e6d97`.
  All 100,000 prediction rows were written and independently read back.
- [Statistical analysis JSON](2026-09-21-owned-nnue-static-calibration-analysis.json) SHA-256:
  `401587081ef52e3425038f4405aa20844da8f7bcf92d9acdf64c9de846a44b14`.
  Its script and method are in [the static-dump directory](../cmd/ngnk4staticdump/README.md).

The scorer used one low-priority WSL CPU core, completed in about 3.2 seconds,
and did not use the GPU or interrupt the Lean hopper.

## Overall comparison

| Evaluator | MAE (cp) | RMSE (cp) | WDL MSE | Score Spearman | Sign accuracy, `|teacher| >= 50` |
| --- | ---: | ---: | ---: | ---: | ---: |
| HCE | 152.70 | 216.30 | 0.011296 | 0.6845 | 80.83% |
| Owned K4 pilot | 157.92 | 230.80 | 0.011457 | 0.6691 | 80.13% |
| Borrowed Rodent Anand | 133.13 | 195.19 | 0.008559 | 0.8352 | 89.30% |

K4's paired MAE is 5.22 cp worse than HCE (position-bootstrap 95% interval
4.43 to 6.05) and 24.80 cp worse than Rodent (23.94 to 25.67). These are
descriptive intervals: calibration positions can share an encoded source chain,
so position bootstrap understates independent-data uncertainty. The signs and
gaps are still useful for this frozen diagnostic set. Neither static error nor
correlation is a playing-strength estimate.

K4 is less reliable where search must distinguish near-balanced positions. In
the teacher `[-50, 49]` score band (27,479 positions), MAE is 96.7 cp for K4,
66.6 for HCE and 40.4 for Rodent. K4's Huber fit of predicted score on teacher
score has slope 0.640 and intercept +7.5 cp, versus HCE's 0.469/-9.0 and
Rodent's 0.476/-10.8. A single slope does not capture the nonlinear score
behavior: K4 is comparatively noisy near zero and has severe individual
overshoots in won/lost positions.

## Concentration of K4 errors

| Output head (piece-count bucket) | Positions | K4 MAE | HCE MAE | K4 minus HCE |
| ---: | ---: | ---: | ---: | ---: |
| 0 | 237 | 416.8 | 249.7 | +167.1 |
| 1 | 10,204 | 213.7 | 187.4 | +26.3 |
| 2 | 20,201 | 163.2 | 159.1 | +4.1 |
| 3 | 21,090 | 163.1 | 158.9 | +4.2 |
| 4 | 18,514 | 156.3 | 152.6 | +3.7 |
| 5 | 17,073 | 141.1 | 138.3 | +2.8 |
| 6 | 10,858 | 118.8 | 125.2 | -6.4 |
| 7 | 1,823 | 101.5 | 102.0 | -0.5 |

Head 0 had only 237 calibration examples. Its +167 cp paired deficit has a
wide position-bootstrap interval of about +120 to +215 cp, which is descriptive
because the examples are not independent games. Head 1 is also clearly weak.
At source ply 120 or later, K4 MAE is 207.3 cp versus HCE's 173.1 cp. King
bucket pair `3-1` is 197.8 versus 166.3 cp; `0-3` is 193.8 versus 170.4 cp.
No side-to-move-only sign inversion appears: White and Black K4 MAE are 159.3
and 156.6 cp respectively.

The largest K4 excess-error record had teacher +2,347 cp, K4 +5,117, HCE
+2,183 and Rodent +1,793. Other large K4 overshoots occur in low-material
heads. The per-position CSV and analysis JSON retain IDs for loss autopsy.

## Decision

The selected K4 model is worse than HCE on the frozen static labels and much
worse than borrowed Rodent. Sparse low-material coverage and central-score
noise are concrete model deficits. Yet an overall 5 cp MAE gap and 0.00016 WDL
MSE gap against HCE do not, by themselves, account for 37 losses and three
draws in 40 games. The approximately fourfold evaluation cost, score/search
interaction, or game-state-specific errors may amplify this deficit.

Next run a hash-frozen 32-position search panel at equal nodes and equal wall
time with the same engine code and exact networks. Compare moves with a deeper
SF18 reference, reached depth, nodes and elapsed time; inspect disagreements
and sampled pilot losses. If K4 is weak even at equal nodes, proceed to the
frozen 1M longer-schedule K4-S1 probe. If it is competitive at equal nodes but
falls apart at equal time, optimize inference before spending on more labels.
