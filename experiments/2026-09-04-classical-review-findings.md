# Classical engine review: first evidence pass

Reviewed runtime: `71399c063ee9688f499e71e1253ebaf81b211813`. Production engine
files and deployed binaries were not changed. The reusable six-angle
[review prompt](2026-09-04-classical-review-prompt.md) includes the broader
remaining sweep; this report does not declare every listed area clean.

**Follow-up:** the [tuning-contract repair](2026-09-04-tuning-contract-repair.md)
resolves findings 1–3 with regression tests and adds a reproduced optimizer
checkpoint-selection fix. The evidence below describes the reviewed release;
the search findings remain open. Deployed playing binaries are unchanged by
these offline repairs. The subsequent loss study also reproduced an internal
castle-out-of-check search defect: the UCI fallback hid the illegal choice and
substituted a legal move that hung a knight. That proved rule correction now
takes priority over the original experiment ordering below; see the
[castling evidence and validation gate](2026-09-04-castling-repair.md). T8's
confirmation was cancelled without a verdict and must be rebuilt after repair.

## Most actionable findings

### 1. Gradient tuning omits a live evaluation term — reproduced defect

`engine/eval.go:3011` subtracts the backward-pawn penalty. The independent
feature reconstruction in `engine/texel_gradient.go:124-175` never includes it.
The trace therefore optimizes a different raw evaluator from the one played.

On 16 deterministic legal random walks, seed 2600, 100 positions per walk:
**542/1600 positions disagree**, maximum absolute difference **24 cp**. Every
difference is exactly explained by the omitted backward-pawn term; unexplained
differences: zero. This is a coverage sample, not a tournament distribution.

First example:

```text
rnb1kb1r/ppp2p2/3pqnpp/4p1P1/4BP2/2N1P2N/PPPP3P/R1BQ1K1R w kq - 0 1
trace = -66; live raw evaluation = -58; backward pawns white/black = 0/1
```

The existing `TestEvalTraceReproducesEval`, float-fidelity test, numeric-gradient
test and MSE-improvement test all pass on the same source. The numeric gradient
test checks the surrogate against itself; the small parity corpus misses this
feature. Required repair: restore parity and test diverse positions plus weight
perturbations. Do not launch another gradient tune first.

**Attribution limit:** the penalty entered at `1435127`, June 21. This cannot
explain earlier failed tunes. It affects the gradient path; coordinate descent
in `texelMSE` invokes the live raw evaluator and does include the term.

### 2. Quiet-data filtering confuses clock damping with tactics — reproduced defect

`engine/texel.go:440` takes an undamped static score, then compares it for exact
equality with qsearch at line 445. Qsearch's stand-pat uses the clock-damped
`EvaluateForPlayerCached` (`engine/eval.go:2817`) plus correction histories.
Consequently the predicate can change without introducing any tactic.

```text
4k3/8/8/8/8/8/4P3/4K3 w - - 0 1
captures = 0; static = 97; qsearch = 97; quiet = true

4k3/8/8/8/8/8/4P3/4K3 w - - 20 1
captures = 0; static = 97; qsearch = 90; quiet = false
```

Both probes clear TT/history first. The only changed FEN field is the halfmove
clock, well short of a draw. This biases generated training data according to
clock state; its frequency in historical corpora is not yet measured. The
damping entered at `d46c48c`, June 22, after the quiet filter's June 8 addition.
This finding applies to self-play data generation through `IsQuietPosition`,
not automatically to externally supplied datasets. Warm TT/correction-history
dependence also deserves a dedicated test before this predicate is trusted.

### 3. An immediate promotion is admitted as quiet — reproduced pipeline defect

```text
7k/8/8/8/8/8/p7/6K1 b - - 0 1
captures = 0; static = 441; qsearch = 441; quiet = true
```

Black can play `a2a1q`. NGN's non-check qsearch uses captures only
(`engine/search.go:2414`; `engine/movegen.go:887`), so the quiet filter admits a
position with an immediate material-changing move. Counter's correspondingly
named `GenerateCaptures` explicitly includes non-capturing queen promotions:
[pinned move generator](https://github.com/ChizhovVadim/CounterGo/blob/b172b99b3128d82c44ef1d8d8a278aa09f2d721b/common/movegen.go#L207).

The missing qsearch move class was already noted in the July method review.
The executable demonstration of its **training-filter consequence** is the new
finding here. Repairing the data predicate and changing playing qsearch have
different scopes: the latter needs a separate real-clock game verdict, including
promotion stalemates, underpromotions and capture-cap behavior.

### 4. Extensions are computed but discarded on some scouts — known, reproduced

`engine/search.go:1987` computes the scout depth from `depth - 1 - reduction`,
not the already computed `nextDepth`. When reduction is zero, the full-depth
LMR retry does not run; only a qualifying PV retry may restore the extension.
Instrumentation counted **113** non-first scouts with `reduction == 0` and
`nextDepth > reducedDepth` in a Kiwipete depth-6 search of 27,472 nodes.
These are omitted extensions on the initial scout, not necessarily 113 final
wrong scores. Counter uses `newDepth = depth - 1 + extension` consistently in
[its child searches](https://github.com/ChizhovVadim/CounterGo/blob/b172b99b3128d82c44ef1d8d8a278aa09f2d721b/engine/search.go#L372).

**Do not sell this as a new one-line fix.** It is J1 in `docs/09` and the tried
ledger records a previous naive repair at -59 Elo with endgame tree growth.
That negative result opposes blindly restoring every accumulated extension.
Reopening requires a new hypothesis about the extension policy as a whole,
including simplifying/removing extensions and controlling their cumulative cost.

### 5. Search has an unrequested 50-million-node ceiling — code/probe verified

`engine/search.go:1216` stops above 50M regardless of the requested node or time
budget. A direct probe with Nodes=50,000,001, requested budget=100,000,000 and no
external stop sets `Stopped=true`. This proves the branch, not its frequency in
real games. Measure its incidence in longer-clock search before assigning Elo
or prioritizing removal. Counter's node stop instead consults the caller's
`Limits.Nodes` in `engine/timemanager.go:57`.

## Concrete comparison with classical Go engines

Reference commits and download instructions are in the review prompt. Chess-3's
current [project rating table](https://github.com/paulsonkoly/chess-3#ccrl) lists
4.0 around 3033 and 3.0 around 2994. Its pinned evaluator is visibly handcrafted
and its tuner identifies itself as HCE. Counter's release likewise constructs
`eval.NewEvaluationService`, with handcrafted features and Texel tuning. These
are evidence that Go and classical evaluation are viable, not a causal
decomposition of either engine's rating.

| Mechanism | NGN | Reference implementation and implication |
|---|---|---|
| Tuner/model fidelity | Separate hand-maintained trace; live-term omission reproduced | Chess-3 uses the same generic `eval.Eval` for integer play and floating-point tuning (`eval/eval.go:17`, `tools/tuner/tuning/vector.go:72`); Counter's tuner calls its evaluator after applying weights (`tests/tuner.go:112`). Shared model code reduces this particular drift risk; integer/float equivalence still requires checks. |
| Trainable evaluation features | Mobility tables and threats are fixed inside the gradient trace; advertised mobility scalars have zero feature coefficient | Chess-3 exposes per-piece mobility, safe/unsafe checks, shelter/storm, threats, passers and other context-dependent coefficients (`tools/tuner/tuning/tuning.go:28`). Re-fitting NGN's current vector cannot change missing or frozen feature behavior. |
| King danger | Existing nonlinear attack-unit curve, hand-set inputs, no separate safe-check feature | Chess-3 has separate safe/unsafe checks, storm/shelter coefficients, and a nonlinear danger curve (`eval/king_attacks.go:19`, `eval/eval.go:143`). The gap is specific information and tunability; NGN already has nonlinear king safety. |
| Horizon moves | Captures plus in-check evasions; six-ply capture cap | Counter's capture generator also generates queen pushes on the promotion rank. Its qsearch runs to the search-stack limit rather than a six-capture cap (`engine/search.go:438`). Compare tactical completeness and cost separately. |
| Extension/search depth | J1 inconsistency; old direct-fix test was interrupted after 65 games, with a negative estimate and no verdict | Counter propagates one computed child depth. Investigate the current interaction; [historical evidence correction](2026-09-05-depth-policy.md). |
| Move generation and ordering | Generates the full list, scores it, then lazily selects; capture SEE partly deferred | Chess-3 has a staged picker with delayed quiet generation (`picker/picker.go:48-112`). Counter also generates full lists, so staging alone cannot explain its advantage. Previous NGN ordering attempts must be considered before another transplant. |
| Complexity that is not necessary | Several continuation/capture/correction histories, ProbCut, direct-mapped TT | Counter 3.8 has a smaller history/search stack and a direct-mapped TT too (`engine/movesort.go`, `engine/transtable.go`). A missing clustered TT or another history table is not a demonstrated prerequisite for 3k-class play. |

The defensible inference is that integration, informative features and effective
search matter more than a checklist of technique names. This source comparison
has not measured their individual contribution to NGN's strength.

## What the tuning history does and does not establish

- Raw logs exist for million-position and augmented three-million-position
  gradient tunes; this was not merely a lack of data volume. Several show better
  train and validation objectives. That proves optimizer progress on that
  objective, not improved decisions under real clocks.
- `output/sprt_retune.log` ends at 68 games, 23/18/27, -20.5 [-102,+62], without
  a completion footer. That file alone cannot establish a failed tuning lane.
  Other historical experiments may have valid negative verdicts; they must be
  mapped to their tested binary/dataset rather than generalized from this file.
- The June 21/22 defects above cannot retrospectively explain pre-June-21
  failures. Related-game leakage, frozen features, score-model differences and
  unrecorded experiment provenance remain hypotheses or known limitations, not
  proved causes of every failed tune.
- Blunder documents a gradient-evaluation tune at **+60.9 +/-17.5**, H1 accepted,
  1164 games, and a later bug-fixed-tuner comparison around zero. Its own
  [test record](https://github.com/deanmchris/blunder/blob/89230a74a966b3400610271eca49c4be241e94cd/docs/testing.md#re-tune-evaluation-using-gradient-descent-tuner)
  is a concrete counterexample to "classical tuning never helps Go engines."
  Its effect size is not a prediction for NGN.

NNUE would change the evaluator's representation. It would leave search-depth,
terminal-rule, clock, data-label and deployment errors to be solved. The failures
here establish neither that NNUE would work nor that it would fail. It is outside
this review's requested classical scope.

## Next work, limited to three experiments

1. **Repair and verify the tuning contract.** Trace parity and a state-independent
   quiet predicate are prerequisites to a fresh Texel candidate. Use the saved
   failures, weight perturbations and a fresh held-out position sample. Cap the
   initial repair/validation at two compute hours; unresolved model differences
   block a new tune. Tuner-only fixes make no playing-strength claim.
2. **Confirm T17 + the completed T8 search vector.** This is search SPSA, not
   Texel, so the gradient defects do not invalidate it. Build on the repaired
   release; verify parameters/tests; use a new predeclared paired real-clock
   manifest, capped at 1600 games (~two box hours at recent throughput).
   Proposed gate: SPRT [-3,+3], alpha/beta .05, H1 required; inconclusive shelves
   the candidate. Do not start a new tuning marathon to avoid this verdict.
3. **Test promotion-complete qsearch in isolation.** First require correct mate,
   stalemate and promotion behavior, then use the same proposed game gate/cap.
   An all-extension restoration or a broad king-danger transplant is not bundled
   into this candidate. Those need the loss-based prioritization already planned.

These are ranked proposals; each game launch still needs its exact source,
binary hashes, opening checksum and A/A basis recorded before starting. The
existing release gauntlet remains undisturbed.

## Reproduction and limits

```sh
python3 experiments/2026-09-04-review-probes.py
```

The runner creates a Go overlay under `output/review-2026-09-04/` and leaves
production source untouched. The probe source is committed beside this report.
**Expected exit on the reviewed release: 1**, with trace and quiet-filter
failures. Scout and node-cap probes pass by confirming the described behavior.
Raw output: `output/review-2026-09-04/review-probes.txt`. Local Go1.26.2/arm64;
no box CPU work was used. Existing gradient tests passed independently.

No new Elo verdict, full runtime profile, all-game loss analysis, or exhaustive
clean bill of health is claimed by this first pass.
