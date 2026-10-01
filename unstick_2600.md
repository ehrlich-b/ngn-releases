# Unstick 2600: adversarial review

I read `stuck_2600.md`, the governing top of `TODO.md`, the relevant memory files
(`project_meta_plan_v2`, `project_3000_mechanism`, `project_ebf_lane`,
`texel_pipeline`, `sprt_instrument`, `feedback_no_nnue`), `docs/11-road-to-3000-census.md`,
the current NGN search/eval/tuner/SPRT code, and CounterGo at `/tmp/countergo`
commit `b172b99`.

I did not run new games. This is a review of the code, logs, and process evidence.

## Verdict

The work is principled at the mechanics level and floundering at the thesis level.

Mechanically, this is not random hacking: there are node-identity checks, ACPL gates,
cloud SPRTs, batch certs, CounterGo comparisons, and increasingly good instrumentation.
That is real engineering.

Strategically, the loop keeps over-promoting the latest explanation into a governing
theory before it has survived falsification. The v2 claim "instrument-limited and
eval is the lever, not search" was too strong. The first half is supported. The second
half is not. The correct post-today statement is:

> NGN is instrument-limited, and current Texel-MSE tuning of static eval shapes is not
> transferring. Search geometry remains a live co-primary suspect, not a secondary lane.

"Eval-texel 0-for-3 -> pivot to search" is sound iteration only if it is framed that
way. If it becomes "eval is dead, now search is THE lever," that is the same lane-hop
failure mode with the sign flipped.

## What Is Actually Proven

1. The old per-change SPRT loop was not a truth instrument.

`cmd/sprt/main.go` now defaults to `-elo0 -3 -elo1 3`, `-mingames 200`, and
`-maxgames 20000`, and the stop rule now uses pentanomial `pLLR` instead of trinomial
LLR (`cmd/sprt/main.go:70-75`, `310-333`). That is a real improvement. The recorded
batch cert, `+55.7` Tier-1 sum becoming `+0.6` over 3877 games, is enough to distrust
Tier-1 magnitudes. Keep using Tier-1 as a filter, not as an Elo account.

2. MSE is not a strength objective.

This is not just a slogan. The code and logs show why. `IsQuietPosition` filters to
positions where static eval equals full-window qsearch (`engine/texel.go:427-452`).
That makes the corpus useful for quiet static calibration, but bad for tactical or
search-attractor terms. Mobility tuning moved MSE strongly in the recorded TODO log,
then worsened ACPL. That is exactly the search-argmax problem: the engine steers into
the eval's peaks, not into a random held-out sample.

3. "CounterGo has less machinery, therefore search is not the lever" is false.

That is feature-count reasoning. CounterGo is simpler, but its search geometry is
not equivalent. Counter root searches use PVS scout and root LMR on later quiet moves
(`/tmp/countergo/engine/search.go:137-160`). NGN root still full-windows every root move
at full depth (`engine/search.go:733-775`). Counter applies its shallow pruning block
to depth 8, including LMP, futility, and SEE (`/tmp/countergo/engine/search.go:327-361`).
NGN still excludes captures from LMR (`engine/search.go:1754-1755`), keeps SEE pruning
at depth <= 4 (`engine/search.go:1551-1569`), and caps qsearch at depth 6 with raw static
eval (`engine/search.go:1994-1996`).

So "NGN has more search features" does not imply "NGN has better search." It may have
more mechanisms but worse co-tuning, worse surfaces, or more defensive gates.

4. The eval story is mixed, not dead.

NGN already has a real eval-shape improvement in history: the per-count mobility
unfreeze was recorded as a keep before the later batch cert erased the apparent
batch magnitude. Current code has hand-seeded per-count mobility tables
(`engine/eval.go:2429-2455`) and folds them into the fixed gradient lump
(`engine/texel_gradient.go:134-175`). Today's failed experiment is specifically:
Texel-MSE tuning those tables on the quiet Lichess/SF corpus pulled away from the
hand seed and worsened ACPL.

That proves the current tuning objective is misaligned for that shape. It does not
prove eval is near globally optimal.

## The Real CounterGo Gap

The most likely reason NGN is far below CounterGo is not one thing. It is a bundle:

1. Search geometry and root waste.

The clearest direct code delta is root PVS plus root LMR. CounterGo has it. NGN lacks
it. This is not a risky "prune all sharp positions harder" transplant. It is a standard
root-search efficiency shape, and NGN currently pays full-window/full-depth cost on
every root move.

2. Selectivity surfaces are in the wrong places.

NGN has many pruning tools, but recent broad extensions keep filter-stopping on sharp
positions. That means the problem is probably not "prune more everywhere." It is "prune
more precisely": cut-node-specific IIR, root LMR, capture-history-gated capture LMR,
and better LMR calibration. The failed quick non-PV IIR experiment supports this:
without a `cutNode` dimension, it over-applies.

3. Eval endgame realism is still underbuilt.

CounterGo has drawishness divisors (`/tmp/countergo/eval/evaluation.go:413-438`),
rule-of-square passer logic (`/tmp/countergo/eval/evaluation.go:321-363`), own/enemy
king passer distances, and tempo disabled in endgames (`/tmp/countergo/eval/evaluation.go:386-393`).
NGN has some passer discounts and per-count mobility, but not the same drawishness
package. Those terms are unlikely to be found by generic quiet-position MSE tuning
because they are sparse, phase-specific, and game-outcome-shaped.

4. The exact "340 Elo" number is not yet a local fact.

CounterGo 3.8 is wired as a 2994 CCRL anchor in `opponents/ratings.json`, and the local
checkout is `b172b99`. That is enough as an existence proof. But the exact NGN-vs-Counter
gap on this hardware, TC, hash/options, adjudication, and anchor pool still needs a
direct local/cloud gauntlet. Treat 340 as order-of-magnitude target pressure, not a
precise diagnosis.

## Was v2 Right?

Instrument-limited: yes.

Eval-is-the-lever: wrong as stated.

It was a reasonable hypothesis because CounterGo's eval is jointly tuned and NGN had
frozen shapes. But it should have been stated as a hypothesis with a kill condition,
not "high confidence." The kill condition fired for the current Texel-MSE path:

- threats: wrong corpus/objective, no useful signal;
- unrestricted mobility: overfit sparse cells;
- restricted mobility: MSE improved but ACPL worsened.

The right update is not "never tune eval." It is:

> Do not trust quiet-position Texel MSE as a standalone eval-strength objective.
> Use it only with monotonic/geometric constraints, L2 to a prior, and an ACPL/game
> gate. For sparse endgame/draw terms, use targeted tests instead of global MSE.

## Structural Blind Spots

1. Governing notes are contradicting themselves.

The top of `TODO.md` still says eval is the high-confidence lever, while the live block
records 0-for-3 and recommends search. That is dangerous because future agents will
obey stale "governing" text. Promote the latest falsification, or mark the old thesis
as demoted. Do not leave both as governing.

2. The process keeps confusing "reasonable explanation" with "resolved diagnosis."

Counter eval richness, EBF gap, self-play mirage, and MSE misalignment are all real.
None alone resolves the 2800 path. The correct operating model is multiple live
suspects with ranked tests.

3. Incremental SPRT can reach 2800 only if the batch cert owns truth.

The old "keep positive sign at 600 games" loop can random-walk forever. The new
pentanomial mill is a real fix, but cloud defaults still need discipline. A 2000-game
cap per shard is fine for large effects; it is not a proof of +3. Quote pLLR and fixed-N
batch certs, not Tier-1 point estimates.

4. Static eval tuning is missing a search-aware outer loop.

The repeated failure mode is the same: static MSE likes a table; search exploits its
peaks; play worsens. ACPL catches some of that. Games catch the rest. Any future eval
tune needs constraints before the tune, not autopsy after the tune.

## Deep Menu Of Next Moves

This is a menu, not a prophecy. Pick one lane at a time, use the execution protocol in
`TODO.md` lines 115-129, and cross off ideas aggressively when the fixed-N penta mill
or tactical probes disagree.

### A. Search Geometry / Width

1. W3 root PVS scout plus root LMR.

   Cross-ref: `TODO.md` line 161, `docs/11-road-to-3000-census.md` lines 65 and 117.
   This is still the cleanest CounterGo-vs-NGN search delta. Try root scout for moves
   after the first, then root LMR only for late quiet non-promotion root moves. Gate on
   no tactical drop, no sharp-position node inflation, and material root-node savings.

2. `cutNode` threading as a node-identical refactor, then W6 cut-node IIR.

   Cross-ref: `TODO.md` lines 100, 102, and 161; census lines 51 and 102-105. The
   quick non-PV IIR form was filter-stopped, and the notes identify missing `cutNode`
   state as the root cause. Do not combine the refactor and the behavior change. First
   prove identical nodes/scores. Then test cut-node IIR.

3. W14 LMR aggression bundle, but split it into named pieces.

   Cross-ref: `TODO.md` lines 102 and 147, census lines 46 and 99-102. Candidate pieces:
   reduce non-PV moves earlier, reduce killer/counter moves by one less instead of
   exempting them, and retune the divisor against live `statScore` distributions. Do
   not start with naked conthist/statScore LMR; `CH-statlmr` failed, `TODO.md` line
   102 says W14 needs Bryan's OK, and `TODO.md` lines 173 and 179 explicitly closed
   the broad conthist/SPSA path for now.

4. Capture LMR, reopened only as a gated variant.

   Cross-ref: census line 48, `TODO.md` line 147. The old W5 result was rejected, so
   retry only if the capture-history gate is narrow: losing/equal captures after many
   quiets, protected from checks, promotions, recaptures, and SEE-obvious tactics.

5. W10 PV pruning as a staged experiment.

   Cross-ref: `TODO.md` line 161, census line 55. Stage 1: remove the `!isPV` guard
   from one quiet-pruning mechanism only. Stage 2: add a tiny PV futility margin only if
   Stage 1 survives. This is higher-risk because previous broad pruning attempts inflated
   sharp nodes.

6. W12 LMR do-deeper.

   Cross-ref: `TODO.md` line 161, census line 62. This is a tactical-safety repair, not
   a free Elo item. It belongs after a live LMR change, where it can reduce false
   negatives from over-reduction.

7. W9 aspiration schedule revisit.

   Cross-ref: `TODO.md` lines 96 and 161, census line 52. The earlier aspiration lane was
   filter-stopped. Reopen only after W3 root PVS, because scout root search changes the
   cost profile of failed aspiration windows.

8. Reconcile W11/LMP state before touching LMP again.

   Cross-ref: `TODO.md` lines 95, 99, and 161; census line 67. The live log says W11
   re-bracket and the d8 row both already kept, while the older shelf still lists W11
   as remaining. The action is state cleanup or certification follow-through, not a new
   LMP patch, unless a cert/regression gives new evidence.

9. W2 SEE prune d4->8, but only if the diagnostic failure is explained.

   Cross-ref: `TODO.md` lines 101 and 151. Treat this as suspended, not dead. The live
   filter stop says sharp positions degraded. If later evidence identifies a units bug,
   margin sign error, or qsearch interaction, retest the minimal corrected patch.
   Otherwise leave it alone.

10. W7 CMH-prune upgrade.

    Cross-ref: `TODO.md` line 150, census line 49. Candidate shape: use continuation
    history only as a second veto for late quiets that already pass existing prune
    preconditions. Do not let it become another broad all-node pruning expansion.

11. O3 history-update discipline.

    Cross-ref: `TODO.md` lines 96 and 161. This is the safer fallback named after W9:
    no-bonus-first-quiet at shallow depth, butterfly add, and a saner depth-squared
    bonus cap. It changes ordering/learning rather than adding another broad prune.

### B. Qsearch / TT / Correctness

1. S1 qsearch cap-6 fix.

   Cross-ref: `TODO.md` line 148, census line 63, attribution in `TODO.md` line 168.
   This is closer to bug-channel than style-channel: qsearch currently has a raw-eval
   return risk mid-sequence. Gate with tactics, qnode traces, and games.

2. Qsearch TT all-bounds store, split into return policy and store policy.

   Cross-ref: `TODO.md` lines 109 and 149, census lines 54 and 63. Because
   `QSEARCH-FAILSOFT` already failed, do not ship a combined qsearch TT rewrite. Test
   exact/lower/upper store first, then retrieval semantics.

3. QSEARCH-CORRHIST as a narrow ACPL-first retry.

   Cross-ref: `TODO.md` lines 109 and 158. This was previously ACPL-skipped, so it needs new
   evidence: positions where stand-pat is consistently wrong and correction history
   would alter qsearch cutoffs without masking tactics.

4. S3 TT correctness bundle.

   Cross-ref: `TODO.md` line 161. Keep this as a correctness audit: bound type, depth,
   ply-adjusted mate scores, replacement, and root/probe edge cases. Do not count it as
   a strength lane unless games actually say so.

5. TT age refresh only.

   Cross-ref: `TODO.md` line 135 and census line 66. The larger retention bundle had
   rejected siblings, so isolate TT age refresh from killer/history-on-TT-cutoff.

6. TT fail-low margin.

   Cross-ref: `TODO.md` line 159. This is a search-stability item. Pair it with probes
   on positions where TT fail-lows cause re-searches or unstable principal variations.

7. S5 probcut refinements.

   Cross-ref: `TODO.md` line 161, census line 64. Probcut is tempting because it can
   save real nodes, but it is a tactical-risk lever. Only try SEE-gated or TT-gated
   refinements with explicit false-cut probes.

### C. Eval / Endgame / Static Shapes

1. B3 drawishness divisors.

   Cross-ref: `TODO.md` line 141, census line 80, zzbias notes in `TODO.md` line 165.
   Directly attacks the +97 centipawn overoptimism channel. Test on low-material and
   pawn-only suites before self-play.

2. B4 winnable-endgame terms.

   Cross-ref: `TODO.md` line 142, census lines 81-82. Candidate terms: rule-of-square,
   own-king escort distance, enemy-king stop distance, and passer race realism. This is
   more defensible than global MSE because it has concrete chess semantics.

3. B5 tempo off in endgames.

   Cross-ref: `TODO.md` line 140, census line 83. Cheap, interpretable, and directly
   tied to zugzwang/low-material weirdness. Verify on endgame probes, not just Texel MSE.

4. Passer rank table unfreeze.

   Cross-ref: `TODO.md` line 155. Do not free-tune all passer terms at once. Add a
   monotonic rank table with prior/L2 constraints and then use ACPL plus games.

5. Pawn phalanx per-rank.

   Cross-ref: `TODO.md` line 155. This is a Counter-like shape that can help connected
   passers without pretending the whole eval is solved. Keep it monotonic and small.

6. Bad bishop versus rammed same-color pawns.

   Cross-ref: `TODO.md` line 155. This is a classic static blind spot and should be
   evaluated on hand-selected structures plus ACPL. It should not need 132 free cells.

7. Own rook behind passer, but not standalone unless evidence changes.

   Cross-ref: `TODO.md` line 155, historical rejection around `TODO.md` line 104.
   Reopen only inside a richer passer package or targeted endgame suite. The standalone
   attempt already had a bad result.

8. Threat EG twins and pawn-push threat.

   Cross-ref: `TODO.md` line 156. Earlier threat work failed on quiet corpus, so these
   need tactical/ACPL targeting. Treat them as "does the search see this one ply too
   late?" features, not as global quiet-MSE candy.

9. Minor protected / minor behind pawn / rook-on-7th conditioned.

   Cross-ref: `TODO.md` line 156. These are plausible shape additions, but each should
   be one isolated switch with an explanatory test book. Rook-on-7th especially needs
   conditioning to avoid fake activity.

10. B2 king-danger model.

    Cross-ref: `TODO.md` lines 157 and 161. Prior safe-check work was bad, so first
    log king-danger miss cases. Candidate terms: queen-king tropism, king-line danger,
    and central congestion. No broad attack retune until the misses are classified.

11. Blocked passer per-rank and threats on pawns.

    Cross-ref: `TODO.md` line 157. These are good "why did we overvalue this?" features
    for zzbias and slow-bleed losses. Use them as anti-overoptimism constraints.

12. Tune Run #1 only after shape additions.

    Cross-ref: `TODO.md` line 143 and closed-lane warning at `TODO.md` line 175. Do
    not retune the current 796 basis again. Retune only after adding two to four richer
    shapes, with monotonic/geometric constraints and ACPL/game gates.

### D. Measurement / Harness / Calibration

1. Edit the governing top of `TODO.md`.

   Cross-ref: `TODO.md` lines 8-15 versus lines 23-35. The file still says eval is the
   high-confidence lever while the live checkpoint says eval failed and search should
   be primary. Fixing this is not bureaucracy; it prevents future agents from following
   stale doctrine.

2. Make the penta mill the default verdict.

   Cross-ref: `TODO.md` lines 13, 23, 26, 33, and 115-129. Every candidate should report
   pLLR, fixed-N central interval, games, and stop reason. Do not promote old sign-only
   SPRT summaries to truth.

3. Run CounterGo on the same local diagnostic ruler.

   Cross-ref: `TODO.md` line 163, census lines 121-185. Run NGN and CounterGo through
   the same fixed-depth, fixed-node, qnode, and EBF probes. This turns "Counter did it"
   from mythology into measured deltas.

4. D1 gauntlet harness fixes.

   Cross-ref: `TODO.md` line 161. Normalize Hash/Threads, MaxMoves 400, per-move clocks,
   adjudication, and opponent set. Add CounterGo/Fruit-style references only after the
   harness is deterministic enough to compare runs.

5. Keep the three-tier gating discipline.

   Cross-ref: `TODO.md` lines 57-61. Use Tier 0 for code/probe sanity, Tier 1 for
   cheap sign, Tier 2 for certification. Most false confidence in the notes came from
   treating Tier 1 as if it were Tier 2.

6. Build a "failed idea ledger" queryable by feature family.

   Cross-ref: closed lanes in `TODO.md` lines 170-182. The same ideas keep returning
   under new names. A ledger with feature, patch, failure mode, and reopen condition
   would save cycles.

7. Reconcile live log versus older shelf.

   Cross-ref: `TODO.md` lines 95-102 versus line 161. W11, W6, W9, W2, and singular
   work have newer verdicts above the older shelf. Update the shelf or annotate it so
   future work does not pull stale "remaining" items.

### E. Time Management / Real-Clock Strength

1. D3 time-management trio.

   Cross-ref: `TODO.md` line 161, census line 47. This must be real-clock tested, not
   fixed-node tested. Candidate pieces: better panic time, PV stability weighting, and
   fail-low allocation.

2. D4 repetition contempt.

   Cross-ref: `TODO.md` line 161 and census line 57. This is likely small but can matter
   in drawish self-play. Test against positions where the engine repeats from optimism
   rather than necessity.

3. Add root stability telemetry.

   Cross-ref: `TODO.md` lines 115-129 and D3/D4 at line 161. Log best-move changes,
   aspiration failures, fail-low cascades, and time spikes. This makes time-management
   changes testable instead of vibes-based.

### F. Speed / Cleanup / Infrastructure

1. Go PGO.

   Cross-ref: `TODO.md` line 163. Low conceptual risk. It should be node-identical and
   measured as NPS/bench, not Elo first.

2. Speed slivers.

   Cross-ref: `TODO.md` line 163. Candidate slivers: attack detection hot paths, SEE
   swap loop, move encoding helpers, TT probe layout, and qsearch move gen. Require
   node-identical output unless the change intentionally alters ordering.

3. Dead-code deletes.

   Cross-ref: `TODO.md` line 163. This will not solve 2800, but it lowers cognitive
   load and reduces accidental interactions before larger search work.

4. Hash and Move Overhead UCI stubs.

   Cross-ref: `TODO.md` line 163. Useful before external gauntlets and GUI testing.
   Keep them behavior-neutral.

5. Bench reproducibility package.

   Cross-ref: `TODO.md` lines 115-129. Capture commit, bench hash, nodes, NPS, qnodes,
   TT hits, and probe suite outputs for every candidate. The notes are already rich; the
   missing piece is machine-readable repeatability.

### G. Things To Avoid Unless New Evidence Appears

1. Generic eval-basis retune of the current 796 features.

   Cross-ref: `TODO.md` line 175. Closed. Reopen only after richer shapes are added.

2. Conthist/statScore LMR as a standalone 2800 lever.

   Cross-ref: `TODO.md` lines 162 and 173. Shelved to the 3200 era unless cut-node
   threading, LMR telemetry, or live score distributions change the premise.

3. Search-parameter SPSA over reductions.

   Cross-ref: `TODO.md` line 179. Closed until margins/shapes change. It is too easy to
   tune into the current blind spots.

4. Broad pruning extensions after W9/W6/W2-style filter stops.

   Cross-ref: `TODO.md` lines 102, 151, and 161. Reopen as narrow, diagnosed patches
   only. The failure mode is sharp-position inflation.

5. NNUE.

   Cross-ref: `TODO.md` line 181. Reserved by choice. It is not the present blocker,
   and using it now would hide unresolved classical search/eval defects.

If a first sweep is needed from this menu, without treating it as the only path, use:

1. Fix `TODO.md` governing text.
2. Reconcile `TODO.md` live log versus shelf.
3. W3 root PVS plus root LMR.
4. S1 qsearch cap-6 fix.
5. B3/B4/B5 endgame realism mini-batch.
6. CounterGo local diagnostic ruler.
7. W6 `cutNode` refactor, then cut-node IIR.
8. Tune Run #1 only after the richer eval shapes exist.

## NNUE Constraint

"No NNUE until 2800" is not the blocker.

NNUE is the obvious shortcut, but CounterGo 3.8 makes pure-Go HCE to 2800 realistic.
The actual blocker is process resolution plus choosing tests that attack the real
Counter/NGN deltas. Do not use NNUE as an escape hatch for a classical-engine process
that still has root search waste, incomplete endgame realism, and unresolved selectivity
geometry.

## Operating Reset

1. Edit the top of `TODO.md`: demote "eval is THE lever" to "Texel-MSE eval tuning
   failed its current checkpoint; eval remains targeted/endgame only."

2. Stop quoting Tier-1 Elo sums as banked strength. Quote them as filter signs only.

3. Keep eval work, but only in two forms:
   - targeted correctness/endgame terms with ACPL or special books;
   - constrained low-dimensional shapes, not free 132-cell MSE fitting.

4. Choose from the menu above by lane: one search-geometry item, one qsearch/TT item,
   one targeted eval/endgame item, and one measurement item. If the next item is search,
   prefer root PVS+LMR or `cutNode` over another broad pruning extension.

5. Run CounterGo on the same diagnostic ruler as NGN. The existence proof is local now;
   use it as a measuring stick, not just a motivational paragraph.

Bottom line: you are not doomed, and you are not just pattern-matching. But the current
writeup is overconfident in its lane labels. The way out is to downgrade theories faster
and promote only falsified-surviving mechanisms. Right now the surviving mechanism is:
better instrument, targeted search geometry, targeted endgame eval, and no trust in
quiet-MSE unless ACPL/games agree.
