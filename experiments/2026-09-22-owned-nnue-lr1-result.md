# K4-LR1 learning-rate isolation result

Status: **positive static and small-panel diagnostic; real-clock screen failed
its 35% gate.** The 5M unique-data probe continues independently. This is not
a 3k rating result; the 20M expansion remains held.

## Run identity

[The prospectively frozen LR1 plan](2026-09-22-owned-nnue-lr1-plan.md) changed
only the S1 cosine learning-rate endpoints, from `0.001 -> 0.00005` to
`0.0002 -> 0.00001`. Architecture, seed, exact 1M pilot BF, target, batch size,
16,384-update horizon, 512-update checkpoint interval and validation set were
unchanged. The Rust harness SHA-256 is
`ad17bc5a6b4e576372a252e0cc04dcba0e1634fe68d23b0a2cfc04b8b98ef769`.

The first isolated build omitted Bullet's `cuda` feature. Its systemd attempt
failed immediately in the mock GPU runtime with no trained checkpoint; that
attempt remains at `runs/k4-pilot-lr1-20260922`. The [corrected launcher](2026-09-22-owned-nnue-lr1-run.sh)
uses the CUDA-enabled trainer SHA-256
`34d49b9dd175a15e622aa6ac8781817606914899f8b609976806424a0f14e187`
and a new no-clobber run at `runs/k4-pilot-lr1-20260922-retry1`. Its
[run receipt](2026-09-22-owned-nnue-lr1-run-receipt.txt) binds the pilot BF,
finalizer manifest, trainer, bridge and completed training. The retry completed
all 16,384 updates, 268,435,456 presentations and 33 checkpoint receipts.

## Validation selection

[The complete selector receipt](2026-09-22-owned-nnue-lr1-selection.json) has
SHA-256 `4ad114f4f6ea91d46f63097e50be23f61e7e10e14cbfdf16bb5e934b3fb5d4ae`.
It selected update 1,024, integer MSE **0.011008**, versus **0.011393** for the
original pilot and **0.012274** for S1's selected model. The retrospective
early stop was update 5,120. Update 512 had lower raw/integer loss but failed
the predeclared quantization-error gates, so it was not eligible. Loss rose
again through later checkpoints, which argues against more presentations on
this same 1M BF at these learning rates.

## Frozen static calibration

The selected K4 file is SHA-256
`47127943aaaa7e1db7e4e0efeb5f88716aea0936b8306be68f341830308e6cb4`.
The [calibration launcher](2026-09-22-owned-nnue-lr1-calibrate.sh) used the
same frozen 100k positions and label shards as the original K4/HCE/Rodent
comparison. Its [receipt](2026-09-22-owned-nnue-lr1-calibration-receipt.txt)
binds the prediction CSV and [full analysis](2026-09-22-owned-nnue-lr1-static-analysis.json),
whose SHA-256 is `7810dd0746a74b17c86eb432e62b9d267425a27d20389d31b6b7d4cbb3a57178`.

| Evaluator | MAE (cp) | WDL MSE | Spearman | Sign accuracy, teacher magnitude ≥50 |
| --- | ---: | ---: | ---: | ---: |
| Original K4 pilot | 157.92 | 0.011457 | 0.6691 | 80.13% |
| K4-LR1 | **150.55** | **0.010957** | **0.6848** | **81.05%** |
| HCE | 152.70 | 0.011296 | 0.6845 | 80.83% |
| Borrowed Rodent Anand | 133.13 | 0.008559 | 0.8352 | 89.30% |

LR1's paired MAE advantage over HCE is 2.15 cp on this set (position-bootstrap
95% interval 1.40 to 2.96 cp). Related positions can share source chains, so
that interval does not measure independent-game uncertainty. LR1 remains well
behind the borrowed Rodent net on these static labels.

The global MAE hides a chess-relevant deficit. Combining the frozen
teacher-score bands from -100 through +99 cp (43,524 positions), LR1 has
**94.12 cp MAE** and **0.005579 WDL MSE**, versus HCE's **68.74 cp** and
**0.003195**. LR1's overall lead comes from positions with larger teacher
scores: for the 15,716 positions at or below -401 cp or at or above +400 cp,
its MAE is 329.18 cp versus HCE's 385.89 cp. These are retrospective slices
of a pre-existing calibration, not a new selection rule. The near-equal
deficit is a plausible contributor to poor game results and a useful
diagnostic for the 5M models; it is not a causal proof from this static set.

## Frozen search panel

The [LR1 search launcher](2026-09-22-owned-nnue-lr1-search-panel.sh) reused the
preselected 32 FENs, SF18 reference, HCE/Rodent comparators and search budgets,
with the optimized K4 engine and selected LR1 model. The [complete result](2026-09-22-owned-nnue-lr1-search-panel-result.json)
has SHA-256 `7ec9ba99ffdf07a008bcfcb543d9c07fc3315e47ffd67b6171ea777060a56dcd`.

| Same 32 positions | Original K4 on optimized engine | K4-LR1 | HCE |
| --- | ---: | ---: | ---: |
| 100k node request: SF18 move agreement | 13/32 | **15/32** | 15/32 |
| 250 ms request: SF18 move agreement | 14/32 | **15/32** | 16/32 |
| 250 ms request: mean reported nodes | 120,109 | 121,793 | 201,793 |

The two extra equal-node matches split into five LR1-only and three original
pilot-only positions. Thirty-two positions cannot prove a playing-strength
gain. LR1 still searches fewer nodes than HCE at equal clock, though the AVX2
inference work greatly narrowed that cost.

## Real-clock gate

The 40-game, 20 reversed-color-pair HCE screen uses the clean-source AVX2
engine from commit `7f6b1f4`, the selected LR1 model and the accepted 10+0.1
runner. [The build receipt](2026-09-22-owned-nnue-lr1-engine-build.json) binds
the Linux binary SHA-256
`1f2502844f0eab41832cd0818f57c81d59ec66a774e1a09756e0cc72c0c6d1be`.
The [HELD review subject](2026-09-22-owned-nnue-lr1-screen-held.json) hashes to
`26cbee7a64c1548861b209815a74050500e85563c0b552603910d9b8e4c1b9ce`;
the approved WSL manifest is SHA-256
`a46e70e2d7e7af00861ec60f9bb7ceb08a8d7b6be87b5c29930dcfa3bb586eba`.
The match reuses the prior screen's 20 opening pairs for direct comparability;
no formal rating claim will use that reuse as independent evidence.

The match unit `ngn-k4-lr1-screen-40-20260922.service` ran from about 02:43
to 03:04 UTC. The [terminal receipt](2026-09-22-owned-nnue-lr1-screen-terminal.json)
has SHA-256 `50028c27ac47ef9c21a4cc7d0dbf5d0d9be3af28f41dcd4d9a7341e7e9840ae6`.
The [independent chess audit](2026-09-22-owned-nnue-lr1-screen-audit.json)
has SHA-256 `9c04e9626e6eeefb94b29725d5ce4a3f940116c132332ce46c5c97cd812033cb`;
the [operational audit](2026-09-22-owned-nnue-lr1-screen-audit-operational.json)
has SHA-256 `f44f8e2d45a9914856285737dc4366da15cf9e40eb02b0cacd1b7fe569e5ba67`.
Both passed, with 40 complete games, 20 opening pairs and 5,471 legal plies.
The [run state](2026-09-22-owned-nnue-lr1-screen-state.json) records the
final file digest; the chess audit binds the PGN SHA-256
`edaf29fea682ec849f4375f9f744a601ab8b23dc80e309976670977633544d62`.

LR1 scored **8/40 (20%)** against HCE: 5 wins, 6 draws and 29 losses. The
original pilot scored 1.5/40 (3.75%) on the reused opening pairs. LR1's gain
is real in this screen, but 20% misses the predeclared **35%** gate. The 20M
expansion stays held, and neither screen is a formal rating estimate.

The PGN's per-move search telemetry shows LR1 averaged about 1.05M reported
nodes/second and depth 12.7 over 2,446 non-book moves; HCE averaged 1.76M
nodes/second and depth 15.5 over 2,399. In the first 40 plies, the respective
figures were 1.00M/depth 12.6 and 1.46M/depth 14.5. These are descriptive
averages across varying positions and clock budgets, so they do not isolate
the contribution of search speed from evaluation errors. They do show why
near-parity on static labels and 15/32 equal-node panel agreement did not
establish real-clock strength.
