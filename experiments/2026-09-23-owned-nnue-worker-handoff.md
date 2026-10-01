# Owned NNUE worker handoff — 2026-09-23

This is a navigation and decision snapshot, not a new rating claim. Start with
the [evidence and next-step diagnosis](2026-09-23-owned-nnue-next-step-diagnosis.md),
then open the linked result receipts before changing the training recipe.

## State and saved work

- Research checkout: `/private/tmp/ngn-n1-sf18-worktree`, branch
  `research/n1-sf18-feasibility-20260920`. Before this handoff it was clean
  at `8204bca`, tracking `origin/research/n1-sf18-feasibility-20260920`.
- The original owned K4 20M model remains the baseline. Its selected `.nnue`
  SHA-256 is `cebec29cbab676af6a2ade2b67da5c9a01ddb7882ed604238c0e7d67893f72d6`.
  The 32k short-cosine model is diagnostic, not promoted; its SHA-256 is
  `6f9bd70e786f3ecb93c48e702404b00cfc1463417c7998478964035b7f0b6332`.
- Original and short-cosine models, selected complete optimizer checkpoints,
  the 100k calibration predictions, and the completed match archives have
  hash-verified off-worker copies under
  `/Users/ehrlich/repos/ngn/output/nnue-owned-k4-backup-20260922/`.
  This directory is intentionally ignored by Git. The large 20M BF, full
  checkpoint series and original receipts remain under
  `/home/ehrli/nnue-owned-k4-20260920/` on WSL. See the
  [original run](2026-09-22-owned-nnue-main-m1-lr1-result.md),
  [short-cosine run](2026-09-22-owned-nnue-main-m1-lr1-32k-result.md), and
  match result records for exact artifact hashes and paths.
- A one-time WSL `systemctl --user` check at 2026-09-23 22:53 UTC found no
  running NNUE user service. Several failed transient units from earlier
  launch/build attempts remain listed; later successful runs have passed
  terminal receipts and audits. A future worker should check live state again
  before allocating WSL compute.

## What was trained

The source is one pinned 10.81 GB T80 binpack from the public
[`official-stockfish/master-binpacks` dataset](2026-09-05-nnue-data-entry.md)
on Hugging Face, SHA-256
`0d22957b8d4f0f8e6f2913be7b744b2dab5178c3563e0f916312d0d94c28b92b`.
It supplies positions, not the training scores used for this K4 run. The
[sampler](../training/nnue/k4sample/README.md) filters and deduplicates by K4
input, producing 20 million distinct train **positions**, not games, and
100k each for validation, calibration and reserved test. Splits are encoded-
chain and K4-input disjoint; original-game disjointness is not proven.

Our [labeler](../training/nnue/k4label/README.md) re-scores positions with a
pinned Stockfish 18, 5,000 nodes per root. Training uses 100% fresh teacher
score, 0% inherited archive score or game result. Our packer/finalizer turns
accepted labels into Bullet records. The [K4 Bullet harness](../training/nnue/bullet/README.md)
defines the four mirrored king buckets, 768 hidden units per perspective,
SCReLU, eight material output heads, score transform, checkpoint/resume and
export. The weights start from random initialization. Bullet/CUDA supply GPU
training and optimizer primitives; the data pipeline, graph configuration,
quantized NGN evaluator integration and audits are project code.

The original full schedule completed 131,072 updates and 2,147,483,648
presentations, but validation selected update 20,480: integer WDL-space MSE
`0.007169997`. The short-cosine experiment selected update 28,672 at
`0.007016925` on the same holdout. Loss measures score imitation, not game
strength. Its small 32-position move panel regressed at equal time, and the
short-cosine net has not won a paired game promotion test.

## Playing evidence and open gap

- Original K4 versus same-code HCE: 323.5/400 (80.875%) at 30+0.3, with
  audited games and a predeclared paired interval above 50%.
  [Result](2026-09-22-owned-nnue-main-m1-lr1-result.md).
- Original K4 versus borrowed Rodent V1.1 Anand in the same NGN binary:
  34.5/200 (17.25%) at 10+0.1, relative Elo -272 with 95% interval
  [-319, -230]. [Result](2026-09-23-owned-nnue-k4-vs-rodent200-result.md).
- Same 100 openings at 160k commanded nodes per side: K4 scored 37/200
  (18.5%). The paired difference from the clock match spans zero; the
  substantial Rodent gap persists at equal nodes.
  [Result](2026-09-23-owned-nnue-k4-vs-rodent200-fixednodes-result.md).
- K4 measured about 80% of Rodent's in-game nodes per search-second in the
  clock match. Search-tree differences and evaluation cost are confounded; do
  not assign a precise Elo share to speed. The [Opus 5.5 review](2026-09-23-owned-nnue-opus-review.md)
  is saved, with corrections in the [diagnosis](2026-09-23-owned-nnue-next-step-diagnosis.md).

No absolute 3000 rating has been measured. The historical fixed 2800 result
is a separate earlier claim. Do not infer strength from validation loss or
static MAE alone: K4's overall teacher-score MAE beat borrowed Rodent's on
the inherited 100k panel, while its head-to-head game score was much worse.

## Next useful work

1. Freeze a reproducible sample of positions **reached in the K4–Rodent
   games** before examining evaluations. Compare K4, Rodent and deeper SF18
   scores and sibling-move rankings at equal nodes, particularly around the
   first large evaluation or result swing. Break down errors by game phase,
   king bucket and material head. Use the saved PGNs/EPDs and receipts.
2. Use that diagnosis to choose one small, controlled data-source experiment.
   Existing scored-position corpora are candidates; no new self-play is
   required just to obtain source positions. Check license, score convention,
   provenance, game-disjoint holdout and conversion on a small shard. Compare
   new-source and current-source models at **equal position count** with the
   same architecture and schedule, then test fresh holdouts and paired games.
3. Only scale labels/training if the controlled source test improves play or
   a learning curve shows more distinct data still help. Preserve a final
   real-clock game gate. Another long pass over the unchanged 20M corpus is
   not supported by current evidence.

Keep engine builds, matches, training and heavy chess validation on the
authorized Windows/WSL worker, not the battery-powered Mac. The old September
13 NNUE deferral in `CLAUDE.md` was superseded by the user's September 20–23
owned-NNUE direction; see its current working agreement.
