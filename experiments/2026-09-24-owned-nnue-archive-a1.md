# Owned K4 A1: train on the archive's own scores at full archive scale

Status, 2026-09-25 03:40 UTC: **PROMOTED.** The e10 archive-labeled K4 scored
116.5/200 against borrowed Rodent V1.1 Anand (+57.9 relative Elo [+24.4, +92.5]),
versus 34.5/200 for the 20M SF18-labeled K4 on the identical openings and
settings: paired score difference **+0.410 [+0.3475, +0.4725]**. Plan and gate
were frozen before any game. No absolute rating is claimed.

## Why

The owned K4 run is data-starved, not recipe-starved:

- Games vs same-code HCE climb steeply with distinct positions: 1M 8/40,
  5M 26.5/40, 20M 323.5/400 (+250). The curve has not flattened.
- Every run overfits early: 5M selected update 3,072 (~10 epochs), 20M selected
  update 20,480 (~17 epochs) of 131,072.
- Rodent V1.x, the −272 benchmark, reports 2.1B training positions; we used 20M.
- GPU training is not the constraint: the full 131k-update 20M run took about
  5 minutes on the RTX 5080. SF18 5k-node labeling (345 accepted/s, ~20 h for
  19M) is the only bottleneck.
- The pinned T80 archive already on WSL holds 3.80B decoded positions, 2.89B
  eligible, 2.45B unique K4 inputs, and each carries a Leela-derived search
  score. We used 1% of it and discarded its labels.

The user directed on 2026-09-24 to bootstrap from existing strong-engine
search labels rather than generating our own. The 2026-09-06 objection
(unproven producer transform) is answered empirically: the conversion factor
is fitted against our own SF18 labels on the frozen calibration positions.

## Pipeline

1. `ngnk4archive scatter` (new, `training/nnue/k4sample/src/bin/archive.rs`):
   one pass over the pinned archive. Keeps positions whose sampler-v2
   `rejection_reasons` are empty, whose chain split is `train` (same seed and
   split function as the SF18 corpus), and whose K4 key is not in the
   selection's `conflicts.bin` (all cross-split and historical-holdout keys).
   Packs Bullet `ChessBoard` records exactly as Bullet 629ee50's binpack loader
   does (STM-relative board, score and result), with raw archive scores, into
   256 randomly assigned buckets. Emits archive scores for the 107,303
   SF18-labeled calibration keys.
2. Calibration against the SF18 labels of the same calibration positions,
   plus board-byte parity against the Go packer's finalized calibration BF.
3. `ngnk4archive gather`: drop the `VALUE_NONE` sentinel, map raw scores to NGN
   units with the fitted monotone piecewise-linear map, shuffle each bucket,
   concatenate into one BF plus an `ngn-k4-archive-corpus-v1` manifest.
4. Training harness modes `archive-a1-e1` (163,840 updates ≈ 1 epoch) and
   `archive-a1-e10` (1,638,400 updates ≈ 10 epochs), 16,384-update
   checkpoints, cosine LR 0.001 → 0.00001, 100% score target, unchanged K4
   graph, seed, batch and quantisation. Bridge `select` scores checkpoints on
   the frozen SF18 validation BF (monitor only; label sources differ).

Holdouts: validation/calibration/reserved-test chains and every conflict key
are excluded, so the SF18 holdouts stay input-disjoint from training.
Position-level (not whole-chain) quarantine is used for conflict keys; games
on the fixed opening set are the strength evidence.

## Predeclared game gate

Match: archive K4 vs borrowed Rodent V1.1 Anand in the same NGN binary, the
exact audited 2026-09-23 clock-match configuration (engine
`1f2502844f0e…`, openings 4241–4340, seed 2026092301, 10+0.1, concurrency 2,
CPUs 12/14, 200 games). Only the K4 network changes.

Rule: promote the archive net over the 20M SF18 net only if the paired
per-opening score difference (archive minus 20M, same 100 pairs) has a 95%
bootstrap lower bound above zero (100,000 resamples, seed 2026092401). The
relative Elo against Rodent is reported but is not an absolute rating.

## Results

### Scatter (WSL, 652 s, cores 8-15, nice 10, hopper live)

Decoded 3,801,599,910 positions in 67,917,008 chains and rejected
675,697,898 non-quiet and 282,018,881 in-check positions, all identical to the
2026-09-21 scan. Train-split eligible 2,798,791,538; conflict-key exclusions
33,306,392; **emitted 2,765,485,146**. STM results: 660.4M loss, 1,451.7M draw,
653.3M win. The probe captured 107,980 calibration rows (107,303 keys).

### Calibration (104,278 SF18-labeled calibration keys)

- **Packing parity exact:** all 100,000 Go-packed calibration records match an
  archive-packed board byte for byte. A first-occurrence-per-key pass matched
  97,103, all with the same SF18 score; the other 2,897 are horizontally
  mirrored twins sharing a K4 key and match once every occurrence is included.
- **Sentinel found:** 10,704 rows (10.3%) carry raw score 32002, Stockfish's
  `VALUE_NONE`. SF18 rates them near level (mean +77). A unit conversion would
  have labeled them as forced wins. They are now dropped, as Stockfish's own
  trainer does.
- Without sentinels (93,574 rows), WDL-space MSE against the SF18 target:
  piecewise map **0.00124** (64 quantile knots, in-sample), best tanh 0.00172,
  best linear 0.00256 (factor 0.52). For scale, the owned 20M K4's validation
  MSE against SF18 labels is 0.00717. The map is compressive: slope about 0.8
  near zero, ±442 at raw ±930, clamped to −1432/+1415 at the tails.
  Spearman over all rows was 0.833, sign agreement 95.5% for |SF18| >= 50.

### Corpus

`gather` kept **2,374,290,033** records (76.0 GB BF, SHA-256
`270f65bdf9f5c948160cae5330030a60f99e94383de2a290b565ae7aad7cde26`) after
dropping 391,195,113 `VALUE_NONE` sentinels (14.1% of emitted). No mapped score
exceeded the ±10,000 cap. About 119x the 20M SF18 corpus.

### e1 training (163,840 updates, 2.68B presentations, ~1.13 epochs)

34 s per 16,384-update checkpoint (~7.9M positions/s); disk kept up. The bridge
selected the final checkpoint (update 163,840): integer validation MSE
**0.005065** on the frozen SF18-labeled validation BF, still falling at the end.
The 20M SF18-trained K4 selected 0.007170 and the short-cosine run 0.007017 on
the same holdout. Model SHA-256
`a4b127eb646d9c7f6d822a0c9ee068664917cd9305b3cc77f5bd09f3ed5b298f`.

Static calibration on the 100k SF18-labeled calibration positions (none of which
were trained on; their archive scores did inform the 64-knot score map):

| Evaluator | MAE cp | WDL MSE | Spearman | Sign acc. \|t\|>=50 |
| --- | ---: | ---: | ---: | ---: |
| HCE | 152.7 | 0.01130 | 0.685 | 80.8% |
| Owned K4 20M (SF18 labels) | 130.2 | 0.00717 | 0.809 | 88.5% |
| Borrowed Rodent V1.1 Anand | 133.1 | 0.00856 | 0.835 | 89.3% |
| **Owned K4 archive e1** | **97.6** | **0.00512** | **0.865** | **91.9%** |

Static metrics are not a strength claim; the paired game gate decides.

### e10 training (1,638,400 updates, 26.8B presentations, ~11.3 epochs)

~8.77M positions/s, about 57 minutes on the RTX 5080. Validation MSE fell
steadily (0.00583 at 16k, 0.00540 at 344k, 0.00510 at 836k, 0.00497 at 1.16M,
0.00488 at 1.49M); the bridge selected update **1,523,712** (~0.00487), no early
stop. Model SHA-256
`7e7fe59ba477b12041f010d7a1c2851d728ef84c8a761961a5e2e4b77776cfaf`.

### Gate match

`integration-v1/k4-archive-a1-e10-vs-rodent-200-v1`, HELD SHA-256
`e7ae0d1e00e1445f9320e36e08b706f1b08f51a6cad163857d3614fcd0c1658b`, review
subject `9e3e98161fbe8c22854175939cfd7010a5ab0b81fab02634c2d2318c675e909d`,
exclusive window, 02:38–03:36 UTC on 2026-09-25, hopper live. Identical engine
(`1f2502844f0e…`), Rodent network, openings 4241–4340, seed 2026092301, 10+0.1,
concurrency 2, CPUs 12/14 as the audited 20M clock match; only the K4 network
changed.

- Runner state COMPLETE; operational audit PASS; independent chess audit PASS,
  200 games, 100 pairs, 34,517 legal plies, zero embedded-book signature plies.
  Final-files SHA-256 `7a1442dd76fff6e2861cb382421c8719eaa02e8f74b71f67bd51a6fcafb079b0`.
- Archive K4: **72W / 89D / 39L = 116.5/200 (58.25%)**, penta
  `[2, 19, 33, 36, 10]`; 95% paired-bootstrap score [0.535, 0.630], relative
  Elo vs Rodent V1.1 **+57.9 [+24.4, +92.5]**.
- Same openings, 20M SF18 K4: 34.5/200 (17.25%). Paired difference
  **+0.410 [+0.3475, +0.4725]** (seed 2026092401, 100,000 resamples).
- **Verdict under the predeclared rule: promote** the archive e10 net over the
  20M SF18 net. [Report](2026-09-24-owned-nnue-archive-a1-vs-rodent200-report.json),
  produced by [the report script](2026-09-24-owned-nnue-archive-a1-vs-rodent200-report.py).
  PGN SHA-256 `4a5c81e9d825ca563469989c25db223869aa1290010c2e084ad430c2fb5d2be6`.

Artifacts: WSL `/home/ehrli/nnue-owned-k4-20260920/archive-a1/` (scatter
buckets, corpus BF, runs, nets); hash-verified off-worker copies of both nets,
selections, receipts and the PGN under
`/Users/ehrlich/repos/ngn/output/nnue-owned-k4-archive-a1-20260924/` (ignored
by Git). Small receipts are in this directory with the `2026-09-24-owned-nnue-archive-a1-` prefix.

Limits: one opponent, one clock, 200 games. This is relative Elo against the
same-binary Rodent V1.1 evaluator, not an absolute rating and not a comparison
with Rodent V1.2 (the current competitive evaluator), which is untested.

## Next

1. Same-binary gate against Rodent V1.2 (the adopted competitive evaluator),
   then a longer-clock check. Both are cheap; the net and harness exist.
2. More data from the same dataset: about 106 GB more Leela-derived binpacks
   (farseerT74/T75/T76, T60T70wIsRightFarseer) and about 107 GB of Stockfish
   5k-node self-play binpacks. The scatter/gather path takes about 35 minutes
   per 10 GB archive; each new file needs its own sentinel/score-map check.
3. Then capacity: with ~100x more data the 768-wide K4 may be the limit;
   a wider hidden layer needs matching engine inference work.
4. Housekeeping: the 88 GB of scatter buckets can be deleted once the corpus is
   no longer needed for reruns (user decision).
