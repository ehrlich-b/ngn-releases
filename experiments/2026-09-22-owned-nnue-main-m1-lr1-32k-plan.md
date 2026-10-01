# K4 20M short cosine schedule experiment

Status: completed through the fixed search panel. The
[result](2026-09-22-owned-nnue-main-m1-lr1-32k-result.md) records improved
validation and static error but worse equal-time move agreement. This remains
a diagnostic candidate, not a replacement for the selected 131,072-update
`main-m1-lr1` model or its predeclared 400-game confirmation.

## Question and controlled change

The prior 20M run's selected update 20,480 occurred while its 131,072-update
cosine schedule still had about 94% of its initial learning rate. Its training
loss continued to fall, while held-out validation integer MSE rose after the
selected checkpoint. Test whether a shorter learning-rate wind-down improves
generalization using the exact same finalized 20M BF, seed, architecture,
batch size, AdamW settings, score-only SF18 targets, and 100k validation and
calibration holdouts.

`main-m1-lr1-32k` is a cold start with 32,768 updates, 4,096-update
checkpoints, and cosine endpoints `0.0002 -> 0.00001`. It has eight
superbatches. The selector admits updates at or after 8,192; uses integer
validation MSE with the existing quantization and optimizer gates; breaks
ties by earlier update; and has patience eight so every checkpoint can be
ranked. The training corpus and validation set are unchanged. The reserved
100k test set stays sealed.

## Decision before games

Compare with the frozen original selection: update 20,480, integer validation
MSE `0.007169997081706572`, calibration MAE `130.18` cp, and the existing
per-band and per-output-head calibration. A meaningful schedule signal needs
at least `0.00001` lower integer validation MSE and no quantization/export
gate failure. Inspect calibration and the 32-position fixed search panel for
regressions before spending match time. If it remains viable, freeze the
selected model and run a disjoint, paired same-code match directly against the
original K4 model. Neither validation loss nor the search panel alone proves
playing-strength improvement. The original 400-game HCE confirmation remains
separate; promotion still requires its declared paired-bootstrap gate and
external opponent calibration for a 3k claim.

Record all checkpoints and results, including a failed hypothesis. Do not
select a different training schedule or game opening after viewing match
outcomes.
