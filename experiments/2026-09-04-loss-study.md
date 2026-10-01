# Bounded fresh-game diagnostic study

Protocol frozen before running interventions. This is a prioritization study,
not a strength verdict or a claim to explain the entire loss population.

Input must contain all **200** `r0904pin` games. Exclude the entire Blunder 6.1
block for this diagnostic sample because of its clock loss. This is stricter
than the gauntlet's actual rule, which retains occasional real clock losses;
the primary rating still includes that block. Within each NGN-result
stratum rank full `GAME` lines by SHA-256, then choose **12 losses, 6 draws, 6
wins** from distinct games. Write the selected game hashes and source-corpus
hash to `loss_study_sample.json` before starting either analysis engine.

For every sampled game:

1. Scan NGN turns after ply 16 with Stockfish at 3000 nodes before/after the
   played move. Preserve the full startpos move prefix. Ignore missing/bounded
   scores; clamp extreme and mate scores to ±1500. Exclude decisions whose
   pre-move NGN score is already below −300 cp. Retain all scan records.
2. Select the largest observed drop, breaking ties at the earlier ply; refine
   that decision at 30000 SF nodes before/after. A game with no eligible decision
   remains unknown in the sample and is not replaced by another game.
3. At the selected prefix compare frozen released NGN at **100000 vs 400000
   nodes**. Evaluate each chosen continuation with SF at **100000 nodes**.
   Every search starts from cleared game/TT state with Hash=64 and Threads=1,
   followed by the full move history; the larger budget inherits no warm search.
4. Record unchanged moves, changed moves scoring at least 50 cp better at 4x
   nodes, and mixed/unknown outcomes. The word "rescued" in JSON means this
   specific diagnostic threshold, not that the original timed game is reproduced
   or that an evaluation-versus-search cause has been established.

The independent ruler is local Stockfish, not an NGN dependency. Its binary and
the NGN binary are hashed in the report. Frozen NGN Mac SHA-256:
`d8f99879b13d9a9a512865f0f918a422f6e505cab29cb35e9b2b0871f64d7b8b`.
Driver `output/recovery-2026-09-04/loss_study.py`, initial reviewed SHA-256:
`95b7b3dfdef16bcedb6f7a9d7d1c7f49d58059b3fbbeaa8954cff21fabe7ba84`.

```sh
python3 output/recovery-2026-09-04/loss_study.py --max-runtime-seconds 7200
```

Local compute only, hard two-hour limit. Parser/sampling smoke and an actual
Stockfish/NGN handshake, two-budget and alternative-scoring smoke passed. Root
review required fixing buffered UCI reads, eliminating warm-state bias and
handling unusable scores before allowing the full study.

Limitations: shallow SF scores are noisy; selecting a maximum exaggerates its
apparent drop. Refinement may dispute that first ranking. Game-result strata are
deliberately oversampled and cannot estimate population frequencies. The gauntlet
uses twenty paired six-ply opening prefixes from the pinned synthetic opening
file, reused across anchors; it is not a representative official-rating corpus.
These positions can prioritize a controlled ablation or feature study, but cannot
by themselves justify a transplant, a new fitted evaluator, or an Elo claim.

## Initial study completed

Completed all 24 selected games in **154 seconds** locally. Sample SHA-256:
`57258086f69b6ea562833c754a252bcbd7c0bef8e212dea60445e6e2f076412f`.
The corpus hash matches the completed gauntlet record.

- **19 unchanged moves**, five changed moves, zero demonstrated 50-cp rescues.
- Both scored continuations are usable in 18 cases. Among the five changed
  choices, only two have both reference scores usable; they worsen by 86 and
  62 cp. The other three are unknown because a reference score is bounded.
- Only 13 selected decisions have usable before/after refinement scores.
  A maximum shallow drop is therefore an unreliable locator in a substantial
  fraction of this sample. This result does not close the runtime lane or prove
  an evaluation cause.
- Several fresh fixed-node choices differ from the actual timed-game move.
  The controlled probes retain game history but do not recreate the timed game's
  clock allocation or accumulated search learning. No such difference alone is
  labeled a playing bug.

Next diagnostic: one-family-at-a-time search ablations on selected cases with
large score disagreement, checking reference scores at a complete fixed depth.
The cases are targeted falsification attempts selected after this study, not
another random population sample. The search candidate under game test stays
frozen while diagnostics use a separate local build.

## Classical reference throughput

CounterGo v1.38.0 at pinned commit `b172b99` was compiled with the same Go1.26.2
for Mac arm64 and its default handcrafted evaluator. Four fresh-process samples
per engine/position, interleaved order, Hash64/Threads1, 2M requested nodes:

| Position | NGN median reported NPS | Counter median reported NPS | Ratio |
|---|---:|---:|---:|
| Kiwipete | 682589 | 1206773 | 1.77x |
| Middlegame | 710065 | 1153125 | 1.62x |
| Rook ending | 1230981 | 1714866 | 1.39x |

Raw samples and hashes: `reference-throughput.json`; driver:
`reference_throughput.py`, in the same raw artifact directory. Runs completed
before the loss study began. Cross-engine node accounting and searched work
differ, so these ratios establish neither equal-work speed nor an Elo gain.
They justify profiling NGN's costs rather than attributing a language ceiling
to Go. Fixed-node diagnostic failure to change a choice does not rule out
time-management or broader throughput gains.
