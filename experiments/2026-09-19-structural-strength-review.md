# Where the next substantial strength gains could come from

Research review, 2026-09-19. Source baseline: `f75b578`, with the accepted optimized
Rodent V1.1 Anand configuration distinguished from the newly integrated, opt-in
Rodent V1.2 backend. This is a research and prioritization report, not an
implementation change or an Elo claim. The Lean hopper was left alone.

## Bottom line

NGN is not demonstrably at a polish-only ceiling. It has a mature feature list,
but that is different from a jointly optimized search/evaluation system. The
largest credible opportunities are:

1. **Turn more clock expenditure into completed root decisions.** There is a
   concrete mismatch between the documented soft-limit semantics and where the
   limit is enforced, supported by current game logs.
2. **Finish realizing richer pretrained evaluation.** V1.2 is a substantial
   representation change already integrated behind an option, but its scalar
   implementation has not yet had a fair optimized strength comparison. Beyond
   it, a deeper threat-free network is a credible intermediate ambition before
   taking on all of Stockfish 18's threat machinery.
3. **Redesign history learning and its search consumers together.** Quiet-move
   learning, ordering, reductions and pruning remain coupled to choices made
   during the handcrafted-evaluation era. Failed isolated changes did not test
   this whole system on the present neural engine.

Demand-driven accumulator updates are a useful enabling project for richer
evaluation. A truly staged move picker is another open architectural experiment,
not a guaranteed node-identical speed patch. Neither has a defensible advance
Elo estimate.

The immediate recommendation is **a cheap clock-boundary mechanism test, then a
competitive V1.2 implementation and comparison**. Clock work takes first place
because the question is narrow and inexpensive to falsify, not because it has
the largest demonstrated upside. The conservative log analysis below argues
against hard cancellation explaining most of the tail. If separation fails to
improve completed decisions or only worsens clock distribution, V1.2 moves
ahead. Reserve a deliberate research lane for the
coupled search/history design. Do not consume the next cycle exclusively with
small SMP variants, tiny TT-lock changes, or another arbitrary margin sweep.

## First correct the starting point

The older “hundreds of Elo behind” results are real, but they describe a different
NGN configuration. They do not measure the strongest configuration now available.

| Evidence | What it establishes | What it does not establish |
|---|---|---|
| Older Counter-network NGN versus native Counter: −322.7 Elo at 30+0.3 | A large historical same-network implementation/search-system deficit | The present Rodent build's deficit |
| Rodent V1.1 Anand versus Counter evaluation inside the same NGN binary: +96.19 [69.53, 123.02], 400 games | A large realized evaluator change in NGN | An additive rating increment applicable to every other test |
| Rodent update kernel: 2.43× cold-search throughput, exact search identity | A large realized implementation improvement | An Elo conversion or a remaining 2.43× opportunity |
| Optimized Rodent NGN versus exact external Counter 5.5: +28.73 [6.08, 51.62], 400 games, 10+0.1 | NGN won that accepted local head-to-head | An official rating, longer-clock transfer, or the remaining gap to native Rodent/Stockfish |

Sources: [rating reassessment](2026-09-12-rating-reassessment.md),
[same-binary evaluation comparison](2026-09-13-rodent-counter-strength.md),
[update-kernel result](2026-09-13-rodent-update-kernel.md),
[external checkpoint](2026-09-13-rodent-external-counter-checkpoint.md).

We should not set priorities from the stale README rating headline. Nor should
we add 28.7 to an opponent's published rating and declare the target achieved.
The current absolute rating and current same-network gap to native Rodent remain
unestablished.

There is also a **configuration/adoption gap**, not a new search discovery.
Ordinary startup still defaults to HCE; the README's neural examples omit
Rodent, while TODO explicitly identifies optimized Rodent as the stronger
tested configuration. `make build-release` strips symbols but does not request
`GOAMD64=v3`, which gates the accepted Rodent AVX2 kernels. The Windows targets
request v3 but do not automatically select the neural backend/model. A 0.414-second
read-only hash check found that the three recorded WSL/Windows installation
paths still contain the exact September 5 classical `53e4d1b` binaries. That
installation is not the optimized Rodent configuration from the accepted games.
We did not inspect today's GUI selection, alternate launchers, or inherited
build environment, so this is not proof of which engine the user is actually
playing. It does establish a real delivery gap at those installation paths.
Before interpreting a future playing impression, bind it to the exact binary,
backend, model and build ISA. See the
[configuration addendum](2026-09-19-strategic-review-artifacts/evaluator-quality-screen-contract.md).
The exact [installation hash receipt](2026-09-19-strategic-review-artifacts/installed-binary-hashes.md)
records the check. Promoting an already-validated stronger configuration would
make earned gains available; it must not be counted as a new algorithmic gain,
and this read-only review does not authorize bypassing the existing release hold.

This also updates the [September 13 competitor review](2026-09-13-competitor-review.md):
its larger evaluator/runtime recommendations were substantially acted on.
Rodent import and two major kernel steps are no longer hypothetical, and V1.2
has reached opt-in integration. The new review's clock mechanism, current
runtime ceiling, concrete history package and next-network alternatives should
not be confused with restarting the old Counter-kernel queue.

A future **same-exact-network native-Rodent control** is the cleanest next
calibration. No donor-search port is needed: NGN already has independently
checked compatibility with the donor evaluation. Equal model bytes, the named
raw-score contract, and the release-static material/scale adapter remove a major
confound, although the result still
combines search, execution speed, time management, corrections and surrounding
policy. It is not a pure search-only measurement. Match clock, threads, hash,
opening pairs, book/tablebase/contempt settings and exact retained donor artifact.

## 1. Clock/search interface: a concrete, previously untested mechanism

### Source evidence

`TimeManager` describes its soft budget as the point beyond which it should not
**start** another iterative-deepening iteration. But
[`ShouldStopSearch`](../engine/time.go) (lines 393–474) applies the condition
`elapsed + lastIterationTime > soft` inside the running recursive search too.
[`alphaBetaPV`](../engine/search.go) calls it at node entry and inside the move
loop (lines 1355 and 1804). The intended iteration-boundary call at lines
686–691 is itself subject to the every-1024-calls rate limiter.

When this interrupts an iteration, the root correctly keeps the previous
completed result (lines 871–879 and 958–967). That safeguard must remain:
interrupted child scores are not trustworthy replacement root results.

The exact [Counter 5.5 time manager](https://github.com/ChizhovVadim/CounterGo/blob/63c487ca724c620f71c129d62129c6fb9109c872/pkg/engine/timemanager.go#L71)
instead considers its optimum budget on iteration completion; its deadline
mechanism handles hard interruption. This is a
different division of responsibilities, not merely a different time constant.

### Existing-log evidence, independently reproduced

Two independent read-only scans examined the already completed external-Counter
checkpoint's `run-003/candidate/fastchess.log`. Across **32,347 NGN searches**:

- summed go-to-bestmove duration was 6,786,594 ms;
- summed time after the last completed-depth report was 1,732,567 ms;
- their ratio was **25.529%**;
- filtering to searches of at least 100 ms still gave **26.192%**;
- the reported PV's first move matched bestmove in every search.

These are summed per-search durations across concurrent games, not the job's
wall time. The independent parser found no unmatched, duplicate, negative-time,
or pending searches. Ordinary reporting/log delay was small: the median
difference between logged elapsed time and the info line's reported time was
about 0.54 ms.

**This is not “25% wasted,” “25% recoverable,” or an Elo estimate.** Unfinished
iterations can leave useful TT entries, history updates and completed subtrees.
The observation is that a substantial share of expenditure did not culminate
in another completed root decision. Counter's near-zero apparent tail is not a
valid negative control: its final-report convention differs and includes 3,572
repeated final depths.

A second, conservative bound analysis rules out an important alternative
explanation. Using the *smaller* of the two logged banks/increments, the maximum
40-move estimate, verified reserves and a 5 ms safety margin, **90.61% of
substantial nonmate tail time** occurred in searches finishing before both the
minimum possible hard deadline and the emergency deadline. Match and review
`time.go` are byte-identical. This is evidence against hard-deadline exhaustion
explaining most of the tail, not proof that every remaining millisecond is a
soft abort or that an alternative policy wins. The inference depends on the
logged timestamp alignment; 5 ms is a sensitivity margin, not an independently
measured worst-case logging bound.

Reproduction and interpretation:
[clock audit](2026-09-19-strategic-review-artifacts/clock-tail-audit.md),
[parser](2026-09-19-strategic-review-artifacts/clock_tail_audit.py),
[hard-deadline exclusion and results](2026-09-19-strategic-review-artifacts/clock-hard-bound-audit.md).

### Candidate and falsifier

Separate an **unthrottled iteration-boundary “may start?” decision** from
**rate-limited in-node hard/emergency cancellation**. Initially retain the
existing soft-budget formula, stability/effort factors and next-iteration
projection. Retain external-stop, node-limit and interrupted-result safeguards.

This is not cost-free: letting an admitted iteration finish can increase clock
spending and alter the bank trajectory. The prior iteration's duration is only
an estimate of the next one's cost. Measure completed decision depth, hard-stop
frequency, tail, and the game's remaining-clock trajectory; do not optimize the
tail percentage in isolation. A mechanism pass only admits real-clock games.

A final adversarial scan makes this risk concrete. Among 30,205 searches with
an eligible last completed growth observation, the median next/previous
iteration-cost ratio was **2.114×**, and 52.2% exceeded 2×. Among 20,898
qualifying unfinished tails, 36.1% had already consumed more than twice the
preceding completed iteration's duration. These are completion-selected and
censored samples, not counterfactual completion times or a reason to fit a new
multiplier from this log. They do show why retaining the current 1× forecast
while allowing completion is **not budget-neutral**. Predeclare spending,
overshoot and bank-trajectory guardrails before admitting the experiment;
do not “repair” its budget after looking at game results. Full filters and
limitations: [iteration-growth audit](2026-09-19-strategic-review-artifacts/clock-iteration-growth-audit.md).

The old [T1a test](2026-07-02-t1a-softstop.md) genuinely lost −7.8 Elo when it
removed the next-iteration projection and spent more of the soft budget. It did
**not** test this boundary-versus-abort separation. Do not rerun T1a under a new
name, remove the interrupted-iteration guard, or claim that spending more is
automatically stronger.

## 2. Richer evaluation, with production-quality execution

### V1.2 is a representation step, not a personality tweak

| Property | Adopted V1.1 Anand architecture | Integrated V1.2 architecture |
|---|---:|---:|
| King buckets | 1 | 4 |
| Accumulator width per perspective | 512 | 768 |
| Material-dependent output heads | 1 | 8 |
| Input weights | 393,216 | 2,359,296 |
| Two-perspective numeric accumulator | 2,048 bytes | 3,072 bytes |

That is six times the input-weight capacity for 1.5 times the ordinary lane
width; only the selected output head needs evaluation. Exact loader, inference,
incremental and engine-integration work is already accepted. The missing
strength evidence cannot be replaced by donor marketing or a scalar NPS result.

The [Rodent V1.2 release](https://github.com/nescitus/Rodent-V/releases/tag/Rodent_v_1.2)
reports +66.19 ±15.61 over Rodent Base, but its release also changes search, SEE,
TT and execution. That is not a net-only result, and the comparison is not
against NGN's adopted V1.1 Anand configuration. There is no established +66 Elo
gain waiting to be collected here.

**Next bounded implementation project:** production 768-lane output/update
kernels preserving the exact arithmetic contract, then a representative
whole-search cost check and an equal-clock V1.2-versus-Anand match. Preserve the
scalar oracle. Stop polishing once the major scalar costs are removed and the
implementation is competitive enough to learn from games. Investigate king
refresh caching only if its measured contribution justifies it.

V1.2-first is a marginal-cost and information-value decision: most compatibility
work is already paid for. It is not a finding that this net is stronger than
Anand. A credible same-NGN equal-node quality diagnostic can lower its priority;
a competitive equal-clock loss can reject it. Neither outcome licenses an
indefinite attempt to optimize away a disappointing result.

The old `acpl.sh` is not a turnkey version of that diagnostic: it launches a
binary without evaluator arguments/options, and an ordinary NGN binary starts
in HCE. Its top-eight reference labels also assign unlisted moves the worst
listed score rather than evaluating them. Explicit backend identity, complete
chosen-move labeling or censoring, and valid reference provenance are required
before that screen can inform triage of a richer net. Evaluator rejection still
requires real-clock games under the project contract. See the
[quality-screen contract](2026-09-19-strategic-review-artifacts/evaluator-quality-screen-contract.md).

Sources: [Slice A](2026-09-19-rodent-v12-slice-a.md),
[Slice B](2026-09-19-rodent-v12-slice-b.md),
[Slice C](2026-09-19-rodent-v12-slice-c.md),
[`rodentv12eval`](../rodentv12eval).

### Demand-driven accumulators are a reusable enabling change

NGN currently copies and updates both perspectives on every real move before
the child discovers whether a draw, TT cutoff, or other early return makes
evaluation unnecessary. Null moves also copy numeric state. Stockfish and
Berserk retain transition metadata and realize accumulator state when needed.

The saved September 19 one-thread optimized-Anand profile puts Rodent
`SearchContext.PushMove` at **21.60% cumulative CPU**, including `applyUpdates`
at 12.59% and `refreshPerspective` at 5.15%. Output evaluation is 7.15%
cumulative. These nested shares must not be added together. The profile is six
short fixtures, not a workload-universal estimate or a fresh benchmark run.

The smallest honest preflight is a **shadow demand trace**, not
`1 - evalCalls/pushes`. An unevaluated ancestor can still supply the accumulator
needed by an evaluated descendant. Track which per-perspective frame/delta
chains are actually required, which are abandoned on pop, and which null frames
can alias an existing accumulator. Implement lazy materialization only if this
demonstrates material avoidable work. Preserve push/pop, bucket-crossing,
refresh, model-identity and SMP ownership invariants.

The old [Counter fusion experiment](2026-09-13-counter-update-fusion.md)
computed every update eagerly; its roughly 2.8% result did not test deferred
computation. Conversely, eliminating all current refresh cost would still be
only a modest V1.1 speed ceiling. Finny refresh caches and lazy materialization
are distinct ideas; neither should be sold as another automatic 2.43× win.

Primary comparisons:
[Stockfish lazy accumulator](https://github.com/official-stockfish/Stockfish/blob/cb3d4ee9b47d0c5aae855b12379378ea1439675c/src/nnue/nnue_accumulator.cpp),
[Berserk accumulator](https://github.com/jhonnold/berserk/blob/8ae895a6151695be4a50d4fb65b0c131659c513a/src/nn/accumulator.c).
Profile provenance: [saved profile extraction](2026-09-19-strategic-review-artifacts/current-profile-extract.md)
and [September 19 profiling work](2026-09-19-smp-tt-read-publication.md).

### The ambitious next representation need not be “port all of Stockfish”

Stockfish 18 BIG/dual is a legitimate larger program, but its hard part is not
just a wider dot product. It adds occupied-target threat features, discovered
slider relationships, separate base/threat accumulators, PSQT, dual selection
and fallback, and a more elaborate score adapter. The existing
[BIG/dual plan](2026-09-06-sf18-big-dual-plan.md) is explicit about this.

The existing SMALL implementation is an **oracle**, not a ready production hot
path: `EvaluateAll` computes all eight heads and tracks wide intermediate
arithmetic; reset also invokes the trace path. A production integration needs a
selected-head, trace-free path validated against that oracle. Otherwise a slow
reference implementation could make a good evaluator appear weak.

Before full search integration, cost the threat-feature/update mechanism on
legal move traces and implement selected-head propagation. An eager dual frame
contains at least 8,896 numeric bytes before metadata; copying that at every
edge would compound today's avoidable-work problem. Do not reject a richer net
merely because NPS is lower, but do reject an unbounded optimization project
whose reference costs obscure any useful comparison.

**A credible intermediate candidate is Berserk 14**, pinned to
`8ae895a6151695be4a50d4fb65b0c131659c513a`: 16 king buckets, 1,024 accumulator
lanes and a 2,048 → 16 → 32 → 1 deep network, without explicit threat features.
Its input weights occupy 24 MiB. It can reuse the kind of piece-delta
infrastructure NGN already owns, though its quantization and sparse affine
kernels require a new exact implementation. The source constants, not its
stale README width, establish this architecture.

This is an option to scope **after V1.2**, not another simultaneous port. The
official Makefile identifies `berserk-9b84c340af7e.nn`; full artifact identity and
separate model-license/provenance admission remain unresolved in this review.
Donor strength does not establish this network's gain inside NGN.

Primary sources: [Berserk architecture](https://github.com/jhonnold/berserk/blob/8ae895a6151695be4a50d4fb65b0c131659c513a/src/types.h),
[feature indexing](https://github.com/jhonnold/berserk/blob/8ae895a6151695be4a50d4fb65b0c131659c513a/src/board.h),
[inference](https://github.com/jhonnold/berserk/blob/8ae895a6151695be4a50d4fb65b0c131659c513a/src/nn/evaluate.c),
[model selection](https://github.com/jhonnold/berserk/blob/8ae895a6151695be4a50d4fb65b0c131659c513a/src/makefile).

Own-network training is not the next move. It introduces another large data,
training and compute problem before we have established how well NGN can realize
already-trained models with richer representations.

## 3. Search/history co-design: escape the isolated-knob local optimum

The present engine has history, continuation history, killers, countermoves,
LMR, correction history and other modern feature names. What matters is how
they jointly allocate search.

Examples in the current source:

- Quiet history learns on quiet beta cutoffs, not every completed quiet PV
  winner. Exact Counter 5.5 and Rodent V1.2 source also learn from a completed
  node's final quiet best move when it raises the original alpha. Counter does
  not reward every intermediate alpha raise; its negative examples stop at the
  winner rather than including later PV alternatives.
- NGN's principal history keys on piece/to; its one-ply and two-ply continuation
  histories are separate. Counter's keying and shared continuation structure
  differ, as do its normalized learning rate and consumers.
- NGN's ordinary bonus is depth squared against gravity limit 8,192. Its LMR
  consumer uses coarse thresholds at −500 and +1,000. Changing the bonus alone
  changes what those thresholds mean.
- Quiet ordering, killer exemptions, futility, late-move pruning and reductions
  share signals. A move ranked late is not just visited later: it may be searched
  less deeply or not at all.

See [`moveorder.go`](../engine/moveorder.go), especially lines 15–30 and 216–285,
and [`search.go`](../engine/search.go), especially lines 2029–2090 and 2219.
Compare [Counter's learning rule](https://github.com/ChizhovVadim/CounterGo/blob/63c487ca724c620f71c129d62129c6fb9109c872/pkg/engine/history.go#L28)
and the integrated consumers in
[pinned Stockfish search](https://github.com/official-stockfish/Stockfish/blob/cb3d4ee9b47d0c5aae855b12379378ea1439675c/src/search.cpp).

This does **not** justify copying a hundred Stockfish constants. It supports one
predeclared, internally consistent experiment covering the definition of a
learning event, learning-rate/scale normalization, and the corresponding
ordering/reduction consumer. A passive warm-game observer should first reveal
the missing quiet-PV learning opportunities, history/rank distributions and
LMR fail-high rescues. Cold six-position first-move-cutoff percentage alone is
not an adequate objective.

The [bounded candidate design](2026-09-19-strategic-review-artifacts/search-history-design.md)
makes this actionable: final-winner labels, a donor-normalized response rate,
and the corresponding history-to-LMR scale, while retaining NGN's table
topology and other search policy. It explicitly accounts for noisy winners,
root-loop learning and unchanged history-pruning thresholds. A small factorial
is a screen for a large interaction, not a way to certify a few Elo from noisy
cells. This is a proposed experiment, not an already-admitted change.

### The strongest evidence against blanket lane closure is already ours

The [T17 improving-reference repair](2026-07-25-t17-improving-reference.md)
lost −6.4 [−14, +1] by itself. The later predeclared
[T17 plus T8 consumer-margin interaction](2026-09-05-t8-qcap-confirmation.md)
won **+38.02 [20.77, 55.45]** over 734 games. These used different historical
baselines (July `734a8ee` versus September qcap `1387fd6`), and the latter used
the older adjudicating harness. This is not a same-baseline factorial proof
that T8 rescued T17, nor an individual-knob estimate or NNUE promise. It is
an accepted historical example of a useful coherent bundle, inconsistent with
a blanket policy of closing such designs from one earlier isolated result.

Likewise, the [killer-reset study](2026-08-02-t10a-killer-reset.md) documented
effects on ordering, futility exemptions and LMR, yet its result was generalized
into a closure until first-move cutoffs approached 90%. That percentage is a
diagnostic, not a universal admission threshold for a stronger search.

There is a source-level reason nominal depth is not a quality ruler. NGN,
Counter and Rodent assign different extension and reduced-research depths;
their pruning gates count different move populations; and depth zero enters
different selective quiescence policies. A policy can spend more on a useful
tactical verification and complete a lower nominal depth. Discovering a later
refutation can also lower first-move-cutoff percentage. Conversely, pruning
that refutation can improve the statistic. Six-root summed plies are a coarse
tree-cost warning, not a common tactical horizon or decision-quality measure.
For a genuinely node-identical runtime change, these confounds disappear:
same nodes/scores/PVs plus less time remains the appropriate test. None of this
reopens the already rejected extension/scout patch by itself.

**Stop rule:** reject correctness failures immediately; reject a mechanism when
its promised learning/selection behavior does not appear. Test the coherent
candidate against the current baseline in paired real-clock games. Use a
predeclared small factorial only where it distinguishes the proposed
interaction; do not optimize by selecting the prettiest cell after many noisy
tests. Historical failed constants remain failed constants.

## 4. True staged move generation remains an architectural experiment

NGN generates and scores the entire move list before trying the TT move. Its
lazy-SEE path defers capture scoring, not quiet generation. A staged picker can
try a legal TT move first, generate captures and quiets only as needed, and
stop preparing moves once the search has enough information.

The saved current one-thread profile gives `selectNextMove` **11.59% flat CPU**
and `scoreMovesIntoBuffer` **5.15% cumulative**. These make the lane worth
considering, not a hundreds-of-Elo diagnosis. Even removing selection entirely
would imply only about 1.13× throughput on that profile; a real replacement is
not free. Generation and search-behavior changes require separate reasoning.

Historical evidence must be read accurately:

- Commit `6bf83e9` did test genuine TT-first deferred generation, on the June
  pre-reset engine. Three fixed-depth examples expanded nodes; no games ran.
- The June 23 [sentinel experiment](speed-log.md) generated all moves and
  deferred quiet-history scoring. It was not a full staged generator. Box d14
  slowed and expanded nodes; Mac d13's node change had the opposite sign. No
  strength test ran.
- The later block selector really did lose its node-identical speed screen;
  that specific implementation remains rejected.

Deferring quiet scoring changes which history values are observed after earlier
subtrees update them. Modern engines deliberately accept that behavior. Hence
a new staged picker must be admitted as a **search-and-speed candidate**, not
required to reproduce all original node counts. This does not mean overlooking
slowdowns or reinstating old failed patches.

Primary comparator:
[Stockfish staged picker](https://github.com/official-stockfish/Stockfish/blob/cb3d4ee9b47d0c5aae855b12379378ea1439675c/src/movepick.cpp).

## Narrow correctness findings: real, but not the main strength thesis

Two small deterministic witnesses reproduced current search-contract defects in
one bounded WSL compile-and-test invocation taking **4.027 seconds**:

1. A singular-verification search that excluded capture `a1a7` still searched
   that capture through ProbCut and used it to certify a cutoff. The main move
   loop's exclusion does not cover the earlier ProbCut loop.
2. ProbCut advanced the board and evaluator without updating `MoveStack[ply]`.
   A grandchild then consumed a stale two-ply move-history context.

The witnesses use a deterministic zero-weight test model; they establish the
contracts, not their frequency or Elo cost in actual Rodent games. No fix was
applied. The inert `.go.txt` tests deliberately assert the defects' presence and
must not be installed unchanged as production regression tests. Full evidence
is in [the review artifacts](2026-09-19-strategic-review-artifacts).

These deserve focused fixes and regression tests in a separate implementation
task. They are not a responsible explanation for hundreds of missing Elo.

## Testing changes needed to learn from bigger ideas

The current harness has important protections: full opening pairs in the
bootstrap estimator, artifact identity, operational audits, and no game
adjudication in the accepted modern gates. Preserve those.

Three limits affect interpretation:

1. Reusing the first 200 paired six-ply openings at 10+0.1 measures a particular
   cell. A random seed does not randomize a sequential book. Finalists need a
   frozen held-out opening/clock confirmation before a broad rating claim.
2. The cold exact-kernel speed gate is appropriate for the candidate it was
   designed to test. It does not establish the value of a new network, a
   behavior-changing picker, or warm-state lazy execution. Choose mechanism
   diagnostics that match the actual proposal; real-clock games decide strength.
3. A 400-game ADOPT/SHELVE gate is useful for screening large gains. A result
   like +15.65 [−2.61, +33.98] properly fails adoption but does not establish
   zero potential or close an architectural family. Do not extend the same
   observed candidate opportunistically; distinguish the candidate decision
   from the scientific conclusion about the whole lane.

Details and exact harness references:
[strength-infrastructure review](2026-09-19-strategic-review-artifacts/strength-infrastructure-review.md).

## What I would deliberately deprioritize

- More small SMP helper variants immediately: current aggregate width-eight
  throughput was about 6.705× width one, while the recent helper game gate
  consumed about 3 hours 42 minutes and remained inconclusive. That is not
  evidence that SMP is solved, but it is a poor default use of the next cycle.
- The rejected TT read-publication protocol: removing its lock cost produced a
  measured **−1.464%** whole-search result. Do not revive it from a profile alone.
- TT clustering without new pressure evidence. The old experiment found too few
  harmful collisions to support its premise; current pressure might differ,
  but that must be demonstrated before a redesign.
- Global output scaling as a claimed units bug. It also changes correction
  learning, SEE's relative scale, aspiration behavior and clamping; a result
  would be an empirical coupled policy. Runtime score policy must enter model
  identity or invalidate TT/history.
- Language rewrites, GPU search, tablebase expansion, or own-network training
  without evidence that they address the dominant remaining problem.

## Concrete next decisions

| Order | Work to admit | Evidence required before claiming success |
|---|---|---|
| Separate delivery task | A Rodent-specific release handoff for already-earned gains | Exact accepted binary/model/configuration; explicit disposition of the old hold; amended installation/rollback and startup/stop checks |
| 1 | Clock boundary versus recursive cancellation | Deterministic clock/lifecycle tests, changed completion behavior, safe bank trajectory, paired clock games |
| 2 | Efficient Rodent V1.2 versus optimized Anand | Exact arithmetic/lifecycle parity, credible runtime, same-search equal-clock strength |
| 3 | Shadow accumulator-demand study, then lazy execution if supported | Actual abandoned per-perspective work, exact push/pop parity, representative whole-search improvement |
| 4 | One coherent NNUE-era history/selection package | Warm-state mechanism evidence and a predeclared interaction test, then strength |
| 5 | One richer-model bridge: costed Berserk or SF18, not both at once | Model provenance, independent oracle, production-cost feasibility, then realized strength |

A same-model native-Rodent comparison should accompany the next accepted
checkpoint to tell us whether the remaining deficit is mostly outside the base
evaluator. Do not spend this review hour launching it: it needs a real match,
not a two-minute surrogate.

The delivery task is distinct from research acceptance. The Rodent strength,
kernel and external-checkpoint records explicitly excluded deployment. Current
TODO still requires resolving or explicitly closing the older external V1.2
reporting boundary and amending the release contract. That opponent-reporting
issue is not proof of a defect in NGN's imported V1.1 evaluator, but a successful
new strength test did not automatically remove the hold. Do not reuse the stale
Counter installation runbook or silently replace the old binaries.

## What would make this diagnosis wrong?

There is evidence of unfinished architectures, **not evidence of another
already-existing large Elo gain**. More network capacity is not a stronger-model
theorem. Current runtime ceilings are much smaller than before the accepted
kernels: even magically eliminating all Rodent `PushMove` cost in the saved
profile would yield about 1.28× throughput, and lazy execution can remove only
part of that work. That hypothetical cannot be added to the picker ceiling.
The history package is an unmeasured hypothesis, not a discovered calibration
failure.

One future same-model control and the V1.2 comparison should change priorities:

| Current NGN versus native Anand | V1.2 versus NGN Anand | Consequence |
|---|---|---|
| Clear material deficit | Clear gain | Take V1.2; emphasize realizing its evaluation through search/runtime before another port |
| Clear material deficit | Clear loss/no worthwhile gain | Keep optimized Anand; search/history/runtime leads |
| Material deficit excluded | Clear gain | Richer evaluation leads; demote the claimed large missing search package |
| Material deficit excluded | Clear loss/no worthwhile gain | Neither large-gain thesis is established; cost one richer-model alternative or accept smaller increments |

“Material deficit excluded” requires a predeclared meaningful threshold and a
sufficiently tight interval, not an interval that merely includes zero.
Inconclusive results leave the branch unresolved. This is a decision framework,
not a request to launch another rating campaign now.

The larger change in approach is to admit **coherent mechanisms with explicit
failure tests**, not to lower adoption standards or rename rejected patches.
The recent evaluator and kernel wins show that large steps are possible. The
next large step is more likely to come from how evaluation, learning and search
fit together than from another isolated constant.
