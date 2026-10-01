# Owned NNUE: evidence and next training decision

Status, September 23: **keep the original 20M model as the owned baseline; do
not launch another long pass over the same 20M corpus.** Its confirmed win over
HCE is real, but the direct 200-game [Rodent comparison](2026-09-23-owned-nnue-k4-vs-rodent200-result.md)
places it at −272 Elo [−319, −230] relative to borrowed Rodent V1.1 Anand at
10+0.1 in the same NGN executable. No absolute 3000 rating has been measured.
The [short cosine experiment](2026-09-22-owned-nnue-main-m1-lr1-32k-result.md)
reduced held-out integer WDL MSE from 0.007170 to 0.007017, but its 32-position
equal-time Stockfish move agreement fell from 21 to 14. That panel is too small
to decide playing strength, and validation loss is a training signal rather
than an Elo proxy. A matched game test would be needed to promote the new net.

## Additional frozen-artifact diagnostics

The [scalar sweep](2026-09-23-owned-nnue-calibration-sweep.json), produced by
the [reproducible script](2026-09-23-owned-nnue-calibration-sweep.py), reads the
SHA-verified original 100k prediction CSV. Teacher `[-100, 99]` cp contains
43,524 positions. Original K4 has 78.52 cp MAE in this band, versus Rodent's
44.55. A predictor that always says zero has 40.18 cp MAE *within the same
target-selected band*, so band MAE cannot by itself be used to optimize chess
strength. Prediction-conditioned buckets are included in the JSON as another
descriptive view. The two evaluators use different score scales, so equal
predicted-score bins contain different populations and cannot be compared as
matched strata.

Multiplying every K4 score by 0.82 minimizes overall MAE on this same holdout
(125.30 versus 130.18 cp) but worsens WDL MSE (0.007390 versus 0.007166).
Multiplier 0.96 gives the smallest sampled WDL MSE, 0.007153. Multiplier 0.30
minimizes the target-selected central-band MAE but worsens overall MAE to
175.94 cp and WDL MSE to 0.01407. These multipliers were chosen *after looking
at the holdout*, so their small improvements are exploratory and require fresh
data and games. A scalar keeps the ordering of static evaluations; any playing
effect would depend on NGN's score-sensitive search behavior.

The [PGN throughput summary](2026-09-23-owned-nnue-k4-vs-rodent200-throughput.json),
produced by its [hash-checking script](2026-09-23-owned-nnue-k4-vs-rodent200-throughput.py),
uses the audited 200-game PGN. K4 searched 2.539 billion nodes in 3,136.7
summed search seconds, or 809k nodes per search-second; Rodent searched 3.110
billion in 3,088.3 seconds, or 1,007k nodes per search-second. The ratio is
0.804 and the median reported depths are 13 and 14. This is a measured
real-clock throughput disadvantage. Because evaluators shape the search tree,
it does not isolate evaluation-call cost or establish how much of −272 Elo is
caused by speed. The earlier 32-position panel used another model/build and
must not be substituted for these in-game numbers.

## External assessment and corrections

The [Opus 5.5 review](2026-09-23-owned-nnue-opus-review.md) and its
[exact prompt](2026-09-23-owned-nnue-opus-review-prompt.txt) are saved. It
also advises against simply increasing passes on the existing corpus. Its
fixed-node comparison and analysis of errors on reached positions are useful
next discriminants. The review's rough attribution of 30–40 Elo to throughput
is an unmeasured estimate and should not be treated as a result. Its suggested
new parity check has mostly been covered by the existing strict
Bullet/raw/integer/Go, independent full-refresh and incremental checks; a
specific unresolved transformed-position invariant could still be checked.
The 2026-09-20 [data plan](2026-09-20-owned-nnue-plan.md) explicitly says the
holdouts are encoded-chain and K4-input disjoint, **not proven original-game
disjoint**. Their loss is developmental evidence. Opus read an older
September 13 deferral in `CLAUDE.md`; the user's subsequent owned-NNUE task
supersedes that deferral. The Mac compute restriction remains in force.

## Existing corpora and a path to a larger run

We need positions with trusted scores, not necessarily newly generated games.
The [official Stockfish master binpacks](https://huggingface.co/datasets/official-stockfish/master-binpacks)
are a 287 GB collection already used for NNUE training. The
[Lichess open database](https://database.lichess.org/) offers about 410 million
positions with Stockfish evaluations, including FEN, search depth, node count
and PV, as well as separate PGN game archives. These are candidate sources,
not drop-in K4 training data: their score convention, quality, licensing,
deduplication, source distribution and conversion into the frozen BF contract
must be checked on a small shard first. Lichess reports varied browser
Stockfish versions/depths, so a fixed SF18 re-score of a sample is needed
before mixing its labels with our current labels. Raw PGN results alone are
weak position targets. [Stockfish's NNUE trainer documentation](https://github.com/official-stockfish/nnue-pytorch/blob/master/docs/nnue.md#loss-functions-and-how-to-apply-them)
describes using scored positions and optionally blending game results into a
WDL-space loss; that blend is a candidate experiment, not an automatic fix.

Next, first audit owned-versus-Rodent errors on positions reached in their
match: compare each engine's static evaluation and deeper SF18 moves at equal
nodes, especially near the first large swing. Freeze the sample before
examining its scores. The same-opening
[fixed-node match](2026-09-23-owned-nnue-k4-vs-rodent200-fixednodes-result.md)
has now completed: K4 scored 18.5% versus 17.25% under the clock, with a
paired score-difference interval spanning zero. The large Rodent gap remains
at equal nodes. Then test a
small, game-disjoint shard from one existing scored-position corpus against an
equal-size current-data control. Keep architecture, schedule and validation
rules fixed for that source-only comparison. Evaluate both on fresh held-out
positions and paired games against the original owned model and Rodent. A
larger data run becomes justified if the new source yields a playing gain at
equal size or a controlled learning curve shows further distinct data are
still improving play. Preserve a final real-clock game gate; lower MSE alone
does not promote a network.

The original and short-cosine `.nnue` files, both selected complete optimizer
checkpoints, the 100k predictions CSV, and both match archives have verified
off-worker copies under `output/nnue-owned-k4-backup-20260922/` in the main
checkout. The large artifacts remain ignored by Git; their hashes are in the
linked result records and diagnostic scripts.
