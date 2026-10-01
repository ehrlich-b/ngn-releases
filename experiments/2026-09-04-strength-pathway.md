# Pathway to larger strength gains

Requested September 4 after the correctness release. This is a development
plan, not an Elo forecast or a game-run manifest. Deployed runtime is `71399c0`;
the proved internal-castling/root-terminal repair `70eb527` is under game test.

## Evidence and scope

Historical batch comparisons measured +43.2, +35.1, then +9.9 self-play Elo;
Batch 4 has no keeps ([ledger](batch-ledger.md)). These used the old harness.
They suggest diminishing returns from that queue, not a strength ceiling or
a transferable sum. Today's repair passed its separate 400-game check at
+23.5 [+1,+46], with zero errors/flags. Its completed absolute pin is **2726
[2668,2784]**, not yet the active 2800 objective.

Evaluation has already been tuned: `cmd/texel` supports material/PST/weights
and gradient fitting; `cmd/evaltune` fits auxiliary weights using move quality.
Recommending "add Texel" would repeat existing work. `cmd/lossxray` already
locates disagreements with Stockfish, but score lag alone cannot establish
whether their cause is evaluation, pruning, or insufficient search.

## Ranked work

1. Finish `r0904pin`, audit legal game replay, disclose excluded anchors, and
   publish the rating. No competing box CPU work or extension on favorable sign.
2. Confirm the existing [T8 experiment](2026-08-04-t8-spsa-result.md). Its 24,000
   tuning games finished August 4 but the vector never got a confirmation match.
   Rebase the T17 improving-reference repair plus vector onto the repaired base
   in an isolated checkout, verify consumers/tests, and run the predeclared
   paired comparison. Preserve the fix-plus-vector interaction scope. No new
   tuning round is prerequisite; parameter movement is not a strength verdict.
3. Use fresh games to select one substantial change. Cap the initial diagnostic
   pass at 24 positions from distinct games and two hours of compute. Freeze a
   deterministic sample of losses and draw/win controls before interventions.
   Use existing loss analysis to locate candidate positions, validate moves
   independently, and preserve game history when replaying. Compare a base
   search budget with 4x that budget; test selected pruning/reduction families
   individually; inspect valuation of the reference engine's alternatives.
   Report mixed/unknown causes. This small study prioritizes work; it neither
   establishes Elo nor estimates the entire loss population.
4. Follow the evidence with one candidate: a measured runtime bottleneck if
   extra search rescues choices; a pruning/reduction redesign if an intervention
   rescues them; or a classical evaluation feature change with joint fitting if
   valuation errors persist. Removing a harmful mechanism is a valid candidate.
   For evaluation, use fresh diverse data, split by game/opening family, and
   retain untouched openings for confirmation. Re-fitting the same small corpus
   is not the proposed change. Test mechanism/threshold interactions explicitly.
5. Target a material campaign gain, such as +50 Elo over the repaired release,
   without promising that effect exists. Confirm the resulting package directly
   against a frozen release; do not sum selected estimates. Every search/eval
   candidate still requires a completed game gate. Cap failed investigations and
   change direction when their specific hypothesis fails.

## Architectural alternative and operation

The active objective is at least 2800 without NNUE. Classical reference engines
guide feature and search choices; neural evaluation is outside this campaign.
The [tuning-contract repair](2026-09-04-tuning-contract-repair.md) is complete,
the [T8 confirmation](2026-09-04-t8-confirmation.md) was cancelled without a
verdict after the [completed fresh-game diagnostic](2026-09-04-loss-study.md)
exposed illegal internal castling. The [castling repair](2026-09-04-castling-repair.md)
takes precedence; any T8 confirmation must rebuild on its accepted base.

Long matches run unattended. Agent activity should prepare candidates, inspect
completed evidence, and make decisions. No countdown messages or repeated
short-interval polling; report completed stages and material failures/decisions.
