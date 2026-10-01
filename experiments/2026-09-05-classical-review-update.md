# Classical review: current evidence and next decisions

Latest follow-up: the [draw-conversion audit](2026-09-05-draw-conversion-audit.md)
proved an old pin's final-ply mate mislabeled as a draw and two carried-state
failures in the current engine. NGN stopped at depth1 on cached mate13/15 while
allowing immediate threefold. Counter's completed-depth/mate-distance guard
repairs both replayed continuations; root independently observed checkmate.
The [isolated candidate and prospective native gate](2026-09-05-mate-stop-confirmation.md)
are ready locally. The live Windows pin stays unchanged and requires the new
terminal audit. Before any candidate games, its separate gate was prospectively
moved to the Mac to run concurrently: fixed200 A/A then400 candidate,10+0.1/c4,
background scheduling, no score adjudication. Main integration waits for the
pin freeze; there is no later machine fallback or result pooling.
The default Hash128 follow-up reproduces one of the two immediate-draw choices;
both guarded continuations finish in mate. The
[live-feature activation check](2026-09-05-live-evaluation-features.md) corrects
the earlier claim that NGN's old mating helpers were active, and records the
absent bishop-pair feature as a separate future heuristic hypothesis.

Review source: `c76f7ed69107e083ba20d01bccbcbeb22334eb0a`, no playing-source
diff. This refreshes the [six-angle review prompt](2026-09-04-classical-review-prompt.md)
at the user's request. It includes a new tuning-fidelity diagnostic, a bounded
promotion audit, and a completed pure-speed experiment. It does not claim an
exhaustive clean bill of health or a new Elo result.

The latest absolute pin remains **2726 [2668,2784]** on the frozen local
historical calibration. The original 2600 gate passed; 2800 is unconfirmed.
The frozen classical fit completed its game gate and was shelved: paired+3.04
[−9.47,+15.56] over1,600 games, no acceptance and zero errors/flags. T17+T8
subsequently passed its completed734-game H1 at+38.02 [+20.77,+55.45] and is
integrated/deployed at5cf301a. No NNUE, fitted eval weights, singular-only policy
or picker change is installed. Detailed review evidence below predates T8 integration.

## What explains the tuning concern

The concern has a concrete basis. Earlier probes reproduced an omitted
backward-pawn feature, a quiet-data filter influenced by search/clock state,
quiet promotions admitted as stable data, and checkpoint selection that could
discard a better starting vector. Those defects are repaired, with regressions
and integer trace/live parity under perturbed weights. They explain specific
failure mechanisms, not every historical failed tune; their introduction dates
cannot explain earlier results. See the [repair record](2026-09-04-tuning-contract-repair.md).

The old optimizer also could not move the live mobility curves or threat
coefficients. The [current model](2026-09-05-classical-model-v2.md) exposes those
138 coefficients, retains 936 stored coordinates, and verifies actual source
import/rebuild. Its new corpus retains game/opening identities. Its modest
integer validation/test improvement did not earn acceptance in the completed
1,600-game gate; the fitted vector is shelved.

A new diagnostic checked 1,585 fresh positions from 16 seeded legal random walks,
using both original and actual exported fitted weights. These are not the fit's
holdouts. Both models had **zero integer trace/live mismatches**:

| Vector | Mean absolute float/live difference | 95th percentile | Maximum |
| --- | ---: | ---: | ---: |
| Base | 1.274 cp | 3.867 cp | 6.788 cp |
| Fitted | 1.110 cp | 3.208 cp | 7.406 cp |

This weakens the current-rounding explanation for a large strength deficit.
It does not prove a universal 8 cp bound: tapering scalar weights before
multiplying feature counts can amplify rounding. It also does not validate
search-only correction histories, clock damping, evaluation scale against
pruning margins, or the objective's relationship to winning games.

Reproducer (choose a new output directory for every run):

```sh
python3 experiments/2026-09-05-tuning-rounding-probe.py --model output/recovery-2026-09-04/classical-v2-r3-model.json --output-dir output/recovery-2026-09-04/tuning-rounding-rerun
```

The original `tuning-rounding-20260905/{manifest.json,result.txt}` records source,
model/probe hashes and command. Model SHA-256 is
`6057944aea194e815a64985727a27785c46db03194c04f18fc93007884fe35a2`.

## What strong classical Go engines actually do

Source references remain Counter 3.8 `b172b99b3128d82c44ef1d8d8a278aa09f2d721b`,
Chess-3 v4.0 `a33531629cbe82eef6810f982ec814e79d52a3b3`, and Blunder v8.5.5
`89230a74a966b3400610271eca49c4be241e94cd`. Local revisions were reverified.
The current [Chess-3 author README](https://github.com/paulsonkoly/chess-3#ccrl)
reports v4.0 at 3033; its pinned v4.0 evaluator is handcrafted. That published
listing and NGN's local calibration are distinct instruments. The pinned release
README itself leaves the v4.0 rating blank; do not attribute today's number to it.

| Mechanism | Concrete comparison | Consequence for NGN |
| --- | --- | --- |
| Evaluation/model agreement | Chess-3 uses generic `eval.Eval` for integer play and float fitting (`eval/eval.go:15`); Counter applies weights then calls its evaluator (`tests/tuner.go:112`). NGN's hand-maintained trace is now checked against its live evaluator. | Keep the parity/import contract. More data cannot repair a different trained model. |
| Trainable information | Chess-3 fits mobility, threats and individual check/shelter/storm inputs (`tools/tuner/tuning/tuning.go:28`). NGN now fits mobility/threats, while king danger mainly exposes aggregate scaling. | Individual danger inputs remain a specific gap, but the frozen loss study found limited safe-check relevance; do not assume this explains the whole gap. |
| Child-depth policy | Counter derives `newDepth=depth-1+extension`, reduces from it, then verifies at it (`engine/search.go:392–403`). NGN computes `nextDepth`, but initial nonfirst scouts use `depth-1-reduction` (`engine/search.go:1994–2029`). | Known J1. The old patch expanded the tree, but its negative estimate was only 65 interrupted pre-reset games, not a completed rejection. See the [current interaction diagnostic](2026-09-05-depth-policy.md). |
| Qsearch move classes | NGN includes all four capture promotions but no quiet promotions (`engine/search.go:2405`, `engine/movegen.go:86`). Counter includes quiet queen promotions, but only queen promotions in its qsearch generator (`common/movegen.go:232`). Chess-3 includes all promotion types (`movegen/movegen.go:235`). | A concrete tactical-completeness gap. Reference engines make different compromises; neither is an oracle for all qsearch policy. |
| Complexity and speed | Counter also uses a full move list and direct-mapped TT. NGN's new block picker preserved exact search behavior but cost more time. | Extra tables or a fancier picker are not established prerequisites. Consider removing coupled heuristics when traces support that experiment. |

Blunder's own [testing record](https://github.com/deanmchris/blunder/blob/89230a74a966b3400610271eca49c4be241e94cd/docs/testing.md#re-tune-evaluation-using-gradient-descent-tuner)
reports a completed gradient-tuning comparison at +60.9 ±17.5 Elo, H1 accepted,
1,164 games. A later bug-fixed-tuner comparison is around zero. Classical tuning
can help a Go engine; these records neither predict NGN's gain nor establish
why its earlier experiments failed. NNUE would change representation while
leaving search, labels, integration and measurement to validate.

## New bounded promotion and speed results

A subsequent [quiet-promotion witness](2026-09-05-quiet-promotion-witness.md)
now reproduces a missed mate at the qsearch horizon in both colors: accepted T8
returns+464 while the isolated move-class diagnostic returns mate in one.
Normal depth-one root search already finds those mates, so no root blunder or
Elo gain is claimed. Root verified source identity, reran/asserted the scores,
and checked synchronized independent mate proof. Actual quiet promotions occur
206 times in135 of the old200 rated games; this is prevalence, not causation.
No playing change is installed or added to either frozen live experiment.

The promotion trace found no omitted capture-promotion type or missing promotion
material in SEE/delta. NGN generates Q/R/B/N capture promotions, and both delta
gain and full SEE include the actual promoted piece. The known missing class is
quiet promotions. A controlled-window probe also confirms lower-value capture
underpromotions can hit generic delta pruning. It does **not** show a missed
mate or wrong root choice, so it does not justify a correctness patch. Chess-3
also uses delta after ordering; copying it does not resolve that question.
Detailed lines and the decisive witness design are in
`output/recovery-2026-09-04/qsearch-promotion-review-20260905.md`; probe source,
overlay and passing output are adjacent. No production source changed.

The block picker used lists of at least 32 moves, initialized at selection 4,
with eight-score blocks and exact current-index tie/swap semantics. Differential
tests, three canonical roots and 23 frozen roots matched. Across 230 interleaved
fresh-process search pairs on the Mac, candidate/base time ratio averaged
**1.04169**, median **1.04479**. It is shelved; the patch and raw identity/timing
records remain in `output/recovery-2026-09-04/picker-blocks-*`. There is no native
Windows speed claim and no further picker iteration is queued.

## Three next decisions

1. The classical-fit gate is complete: capped inconclusive, vector shelved under
   its original rule. No refit, favorable-sign extension or NNUE pivot follows.
   Lower validation/test MSE did not establish better play in this comparison.
2. T17+T8 confirmation is complete and accepted. Its relative+38.02 gain finishes
   the existing24,000-game investment; an external absolute pin is still required.
3. The four-arm child-depth diagnostic is complete, and one singular-only
   candidate was selected. Its [frozen game gate](2026-09-05-singular-t8-confirmation.md)
   on accepted T8 completed H0: paired−12.26 [−27.56,+2.99] over794 games,
   zero errors/flags. The candidate is rejected and accepted T8 remains installed.
   A scalar-margin tweak or another futility-off test needs a new premise:
   current controls show harm and direct watched-reply skips were not found.

That larger search simplification did not earn acceptance. Quiet-promotion
completeness is a separate reproduced horizon gap; an
[efficient candidate](2026-09-05-quiet-promotion-candidate.md) now passes local
correctness checks and a bounded Mac timing probe, and cannot alter the current
frozen pin. No game acceptance or strength gain is claimed. The
[external400-game rating run](2026-09-05-2800-pin-plan.md) is now measuring accepted
T8. Self-play, lower MSE and summed selected gains do not establish 2800.

## Bounded null-move follow-up

A source-only follow-up at7bfba89 examines a five-legal-ply king triangle plus
an artificial null move. NGN's repetition scan can include the initial legal
ancestor through that null edge. A small deferred probe is staged in
`output/null-boundary-20260905/`; it has not been compiled or run while the Mac
clocked gate is active. No wrong root move, actual game loss or Elo effect is
shown, and no production patch is selected.

The apparent reference gap narrowed on closer inspection. Counter stops scans
at older null ancestors, but checks ancestor hash before that boundary and
does not first exclude the current null node. Blunder's null halfmove-clock
reset does not constrain its separate full history scan. Chess-3 appends the
null hash and scans earlier hashes without a null boundary. All three pinned
revisions were reverified. Their live predicates do not establish that this
immediate null-cycle fixture is prevented. Treat it as a shared search
approximation to investigate only with a meaningful root/loss witness, not as
another established missing prerequisite for classical strength. The mate-stop
and quiet-promotion candidates retain priority.
