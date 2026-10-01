# Review prompt: explain NGN's classical strength gap

Review evidence was refreshed September5 against `c76f7ed`. The playing release
now includes accepted T17+T8 at `5cf301a`, inheriting the qsearch corrections:
completed734-game H1, paired+38.02 [+20.77,+55.45], zero flags/errors. The latest
completed absolute pin is **2726 [2668,2784]** on the historical local scale.
The active objective is at least 2800 without NNUE. Investigate substantial
mechanism changes when evidence supports them, including replacing or removing
a coupled subsystem. There is no 10–20 Elo ceiling on the ambition of this work.

First pass reviewed runtime `71399c063ee9688f499e71e1253ebaf81b211813`.
For a new pass, record the actual source commit, working diff and deployed binary
hash before starting. The uncapped-qsearch correction, committed at `ec9d264`,
is now accepted under its fixed correctness game gate; frozen patch/artifacts
and the inconclusive strength result are in `2026-09-04-qsearch-budget-repair.md`.
Later source `f556aef` includes the offline classical model/corpus implementation
without new playing weights. Check `TODO.md` for the latest deployed state.
The objective is to identify reproducible correctness defects and substantial
implementation gaps relative to successful classical engines, especially Go
engines. Do not propose NNUE, a language rewrite, or novel chess research as the
escape route. The question is what NGN implements incorrectly, incompletely,
inefficiently, or with poorly validated interactions.

Read `TODO.md` and `CLAUDE.md`, then inspect executable code before accepting
claims in comments, reviews, or experiment summaries. Earlier audits declared
areas clean that subsequently contained proven defects. Historical evidence is
useful but is not an invariant. Preserve the deployed release and live matches.
Use isolated probes; do not apply speculative production changes during review.

Read the [current evidence and decisions](2026-09-05-classical-review-update.md)
before repeating a prior experiment. Repaired trace omissions, quiet filtering,
checkpoint selection, castling and qsearch-cap/TT reuse are regression targets,
not new discoveries. The current 936-entry model makes mobility/threat terms
trainable. Its frozen fit completed1,600 games at paired+3.04 [−9.47,+15.56] and
was shelved under its inconclusive-cap rule; T17+T8 passed and is deployed.
The selected singular-only policy completed its T8 game gate at paired
−12.26 [−27.56,+2.99], H0 over794 games, and was rejected under the
[frozen manifest](2026-09-05-singular-t8-confirmation.md). The external400-game
pin of accepted T8 is running. Do not alter its inputs, launch competing box
work, or refit on the used holdout.
Known J1 scout-depth inconsistency needs an interaction hypothesis: its old
repair expanded the tree, but the negative game estimate came from only 65
interrupted pre-reset games, not a completed verdict. Distinguish a repaired defect from proof
that the surrounding subsystem is correct.

## Reference sources

Use primary source code pinned to a revision. Local copies are gitignored under
`output/review-2026-09-04/`; retrieve these revisions if copies are unavailable:

- CounterGo `v1.38.0` / Counter 3.8:
  `b172b99b3128d82c44ef1d8d8a278aa09f2d721b`,
  https://github.com/ChizhovVadim/CounterGo
- Chess-3 `v4.0`: `a33531629cbe82eef6810f982ec814e79d52a3b3`,
  https://github.com/paulsonkoly/chess-3
- Blunder `v8.5.5`: `89230a74a966b3400610271eca49c4be241e94cd`,
  https://github.com/deanmchris/blunder

Verify evaluator type and engine version in source. Distinguish published
ratings, authors' estimates, and our local calibration. Do not compare ratings
across lists as if they shared a scale. Use other classical engines only when
they answer a specific unresolved question; pin their pre-NNUE revisions.

## Angle 1: chess rules and state integrity

Trace FEN/UCI input through move generation, make/unmake, search, and game
adjudication. Exercise castling through check, EP discovered checks, promotions,
underpromotions, fifty-move mate priority, repetition, and insufficient material.
Compare legal moves and rebuilt position/hash/accumulators with an independent
oracle after random legal sequences, not just a handful of perft roots.

Audit game-history versus search-history counts, irreversible boundaries and
null moves. Search-cycle draws are a heuristic distinct from game threefold.
Check every final board regardless of its recorded label: terminal verification
must work in both directions. The September5 follow-up proved an actual
final-ply checkmate hidden by `max-moves` that the earlier legal audit accepted.
Check same-position reentry through IID/singular search, array limits, and every
early return. A perft pass does not establish search correctness.

Compare **internal search BestMove and every completed root iteration** against
the independent legal set, before any UCI fallback or PV filtering. Audit every
consumer of pseudo-legal moves, not just `GenerateLegalMoves`. The September 4
loss study proved a castle-out-of-check root choice that a legal emitted-move
audit missed: the UCI guard substituted a legal move that hung a knight. Include
terminal positions with a nonempty pseudo-legal list and zero legal moves.

Required artifact: an independent invariant test across every implicated
consumer, with a failing old-runtime witness where possible. A legal UCI fallback
or tests that call the same implementation twice do not certify internal choices.

## Angle 2: search semantics and interactions

Audit alpha-beta/PVS windows, bound types, mate-distance encoding, aspiration
retry behavior, qsearch horizons, in-check behavior and terminal precedence.
Trace actual depth at each recursive call: reductions, all extension types,
full-depth verification, excluded moves, and depth/ply caps. Check whether the
stored TT depth describes the search actually performed. Inspect TT use across
different halfmove clocks/history, and qsearch reuse at differing capture caps.

Trace history/correction learning inputs and consumers, stop propagation, and
state restoration across normal children, null moves and same-ply searches.
Include state carried between moves: fresh FEN and full-history searches both
missed a reproduced depth1 cached-mate early-stop failure. See the
[draw-conversion evidence](2026-09-05-draw-conversion-audit.md) before repeating
it. Compare actual mate termination guards with Counter's depth/distance rule.
Demonstrate an error with a concrete FEN/window/state or label it a heuristic
hypothesis. Differences from another engine alone do not establish a bug.

Review the complete child-search decision: base depth, extensions, reductions,
scout depth, verification trigger, verification depth and final TT depth. Compare
Counter's coherent child-depth policy with NGN's actual calls, not its comments.
Test captures/promotions/checks separately, including promotion value in SEE and
delta pruning. If proposing simplification, state all redundant compensations it
removes and the smallest explicit interaction experiment. Do not merely disable
futility: the frozen controls already contain regressions and direct traces did
not implicate skipped reference replies.

## Angle 3: why evaluation tuning failed

Reconstruct the real pipeline: data generation, quiet filtering, outcome labels,
deduplication, split, feature extraction, optimizer, integer export, loading and
the evaluator invoked by search. Verify exact trace/live-evaluator agreement on
diverse positions and perturbed parameters, not only original weights. Check
finite-difference gradients against the *playing model*, not merely against
another implementation of the same possibly incomplete surrogate.

Check dormant tunables; omitted features; frozen terms; material/PST
identifiability; phase interpolation; integer rounding amplified by feature
counts; draw scaling; score perspective; cache invalidation; and K calibration.
Check whether related positions from one game leak across train/validation and
whether holdouts were reused. Separate trainable feature limitations from data,
optimizer, integration, speed and search-interaction failures.

For each historical tune, recover its revision, dataset/objective, actual exported
and tested vector, and completed game evidence where available. Date any newly
found bug before blaming it for an older failure. Do not infer that classical
tuning cannot work, or that NNUE would work, from these failed attempts.

Separate exact integer reconstruction from the differentiable approximation.
Measure both at original and exported weights. Integer tapering before feature
multiplication can amplify rounding; a constant residual bound needs proof.
Check evaluation-scale changes against centipawn pruning thresholds, SEE piece
values, aspiration windows and clock score regimes. A fixed K and an improved
static objective do not prove these search interactions improved. Propose a
scale diagnostic on training or fresh positions before another expensive fit.

## Angle 4: compare complete classical mechanisms

Follow live evaluator call paths, not helper definitions. The
[bounded activation check](2026-09-05-live-evaluation-features.md) found that the
old mating-bonus helpers are dormant and corrected an earlier audit claim.
All three references explicitly score bishop pairs; NGN removed that feature
and its tuning coordinate on a pre-reset inconclusive result. Treat reopening
it as a heuristic experiment, not an automatic correctness keep.

Build a small matrix with NGN and reference file:line evidence. Compare formulas,
units, gates, update rules, representations and interactions—not feature names.
Prioritize move ordering and LMR together, TT replacement and reuse, quiescence,
time allocation, king danger, mobility, pawn structure, threats, and draw scaling.

For each proposed transplant, explain the missing behavior, why it matters in
NGN, prerequisites in the reference engine, and which existing NGN mechanism it
would replace. Include subtraction/simplification as candidates. Check
`experiments/TRIED-LEDGER.md` and later run records before proposing old work.
An old negative experiment may be reopened only with a specific new falsifiable
premise, not because its verdict is inconvenient.

Required artifact: one complete mechanism comparison and a deletion candidate.
Explain why the reference's prerequisite holds in NGN before copying a rule.
Do not copy an entire foreign engine into the NGN binary to meet a rating target.
Preserve provenance and the relevant license for any code actually reused.

## Angle 5: Go runtime and usable search

Compare work per node, allocation/GC, frame storage, move generation/staging,
SEE work, hashing, caches, locks, diagnostics and time checks. Inspect hardcoded
node/depth limits and root exits that may waste clock at longer time controls.
Measure on equivalent hardware/settings; raw NPS and nominal depth across
engines are not strength measurements. Distinguish exact speed improvements
from pruning that merely makes the tree smaller. Check current native workers
before remote compute; do not load the Windows/WSL box during timed matches.

## Angle 6: the measurement and development process

Independently inspect the match harness, score POV, clocks, process failures,
opening/color pairing, terminal adjudication, PGN completeness, and exclusions.
Check whether self-play conclusions transfer to external opponents and longer
clocks. Identify completed experiments left unconfirmed, stale binaries, tests
that mirror implementation, and lane closures made from invalid evidence.
Do not turn every review into a new testing framework or another long tune.

Require terminal-log, binary, command, full game-count and color-pair agreement.
An occurrence of a success marker somewhere in a log is not a terminal check.
Exercise validators through their real public entry point with corrupted inputs;
handwritten tests of the expected answer do not prove the validator rejects it.
Preserve historical rating labels and exclusions; bootstrap sensitivity cannot
retroactively change the acceptance gate or convert this local scale to CCRL.

## Required output

1. Ranked findings with classification: reproduced defect, code-proven mismatch,
   measured performance issue, heuristic hypothesis, or falsified suspicion.
2. Each finding includes NGN file:line, exact reproduction or falsifier, reference
   source/revision when relevant, impact and limits, and whether it was already
   known. Distinguish newly discovered bugs from the unshipped T17 repair.
3. A compact mechanism comparison, including what stronger engines *omit*.
4. A tuning failure account that distinguishes proved causes from missing records.
5. At most three next experiments, with prerequisites, the cheapest decisive
   check, game acceptance rule, compute cap and stopping condition. No invented
   Elo priors, summed gains, or unsupported claim that the engine is now clean.

Give the strongest evidence against your preferred explanation. A useful review
narrows the next implementation decision and supplies executable evidence.

Bound each initial angle to 20 minutes of code/probe work and return its best
finding or negative result. Allocate further time only to a concrete unresolved
mechanism. At most three experiments leave the review. Long games run unattended;
do not spend active turns on repeated polling or countdowns.
