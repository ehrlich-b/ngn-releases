# One bounded Counter-inspired history/search experiment

Source-only design, 2026-09-19; not implemented or admitted to the hopper.
NGN reviewed at `f75b578`. Donor: Counter 5.5, exact revision
[`63c487ca724c620f71c129d62129c6fb9109c872`](https://github.com/ChizhovVadim/CounterGo/tree/63c487ca724c620f71c129d62129c6fb9109c872),
read from the already-pinned oracle checkout. This is a deliberately limited
learning/consumer package, **not a complete Counter search or history port**.

## Signal, normalization, consumer

Let `H` be an individual entry's saturation magnitude, `h/H` its normalized
value, and `s` be the sum of main and available predecessor-context histories.

| Aspect | Current NGN | Exact Counter 5.5 |
| --- | --- | --- |
| Quiet winner target | Quiet beta-cutoff move only | Final quiet best move when the completed node raises original alpha, including PV nodes |
| Negative examples | Tried quiets preceding cutoff, including a noisy cutoff | Quiet prefix through the winning quiet move; stop at winner, do not penalize later PV alternatives or update when winner is noisy |
| Main key | Colored piece / destination | Side / origin / destination |
| Predecessors | Separate one-ply continuation and two-ply follow-up arrays | One colored-piece/destination continuation array read/updated for both predecessor distances |
| Entry bound | `H=8192` | `H=16384` |
| Normalized response rate | `eta=min(d*d,2048)/8192` | `eta=min(d*d,400)/512` |
| History LMR term | Add 1 if `s < -500`; subtract 1 if `s > 1000` | Subtract `clamp(s/5000,-2,+2)` |
| Other consumers | Quiet ordering plus shallow history pruning (`s < -1000`, depth <=3, after >3 legal moves) | Quiet ordering; no matching explicit history-prune clause in the inspected search |

NGN evidence: `engine/moveorder.go:15-21,203-285`,
`engine/search.go:1865-1870,2053-2062,2219-2232,2260-2282`.
Counter evidence: [history.go:14-62](https://github.com/ChizhovVadim/CounterGo/blob/63c487ca724c620f71c129d62129c6fb9109c872/pkg/engine/history.go#L14),
[search.go:263-284](https://github.com/ChizhovVadim/CounterGo/blob/63c487ca724c620f71c129d62129c6fb9109c872/pkg/engine/search.go#L263),
[search.go:329-332](https://github.com/ChizhovVadim/CounterGo/blob/63c487ca724c620f71c129d62129c6fb9109c872/pkg/engine/search.go#L329).

Both update equations are bounded exponential responses after normalization;
"EMA versus gravity" is not itself a qualitative distinction. At depth 4 the
normalized rates are `1/512` versus `1/32`: Counter learns 16 times faster per
update. Its raw initial increment is 32 times larger because its bound is also
twice as large. These statements should not be conflated.

## Three coupled changes, fixed before measurement

1. **Winner labels:** use the donor's completed-node quiet-winner rule, including
   PV winners, and its prefix-only negative examples. A final noisy winner does
   not update quiet history under this rule; that explicitly replaces NGN's
   present quiet penalties on a noisy cutoff. Update only once per completed
   node, never once for every intermediate alpha raise. Do not learn an
   interrupted result. NGN's root loops are separate from `alphaBetaPV`: cover
   completed root attempts as well as interior nodes, comparing to the
   attempt's original alpha and not double-counting an in-window completion.
   A completed fail-high attempt may supply a valid label; an interrupted
   attempt may not. Leave killer/counter updates on their current policy.
2. **Response rate:** adopt `eta=min(d*d,400)/512`, but retain NGN's `H=8192`
   storage range. Use widened arithmetic for
   `h += (target - h)*min(d*d,400)/512`, where target is `+H` or `-H`.
   Do not route this through the old step cap of 2048: that silently changes
   the intended response at deeper nodes. Entry bounds, ordering bands and
   capture-history arithmetic remain unchanged.
3. **Normalized LMR consumer:** use
   `reduction -= clamp(s/2500,-2,+2)` with NGN's `H=8192`. This is the
   Counter `/5000` rule expressed at half its storage scale, not a fitted
   divisor. Keep the current LMR base table, PV/improving/killer modifiers,
   extension flow, history-prune threshold and eligibility gates unchanged.

Retain NGN's current main key and its separate continuation/follow-up arrays in
this first bounded package. This sacrifices fidelity to the complete donor in
exchange for an interpretable test of labels/response/consumer interaction.
Faster response on a coarser main key can amplify aliasing and noise; a negative
result would reject this package, not Counter's complete topology. Do not add
butterfly indexing, compact storage, extra continuation plies, capture-history
changes, or a new picker during this experiment.

History pruning remains an important **explicit** side effect of changing the
producer even when its threshold is numerically unchanged. Record its firing
rate and rescued-move errors; do not attribute all behavior to LMR. Likewise,
retaining the raw entry range does not guarantee the live distribution stays
the same.

## Why this is not the old isolated tweak

The [June amplitude screen](../2026-06-29-fmc-ordering.md) changed `d*d` to
`4*d*d` with the old labels and consumers and was rejected on six cold
positions. The [T6 record](../2026-07-26-t6-history-distribution-preflight.md)
shows the present LMR term was already active on about 75% of decisions; the
"dead term" premise is false. Its level-matched `/750` candidate lost four
summed nominal depth plies on the six-position screen, and several divisors
expanded its endgame tree. Those are real negative priors, not permission to
retry the same constants.

The new hypothesis is narrower and testable: **a differently trained, more
responsive quiet signal may support a different selective-search consumer**.
It is not "more history is always better" or "smooth LMR must search deeper."
The producer change is a prerequisite to reopening the rejected consumer as an
interaction test; a consumer-only arm is a control, not an adoption candidate.

## Small predeclared factorial, with honest limits

First run a future passive/shadow observer on frozen, held-out game prefixes,
preserving each game's history between positions. Measure weighted missed
quiet-PV learning events, live normalized distributions, rank of later cutting
quiets, history-prune frequency, and reduced-search fail-high/rescue behavior.
Cold single-position reset is not the intended workload. If the new labels
carry negligible update mass or the claimed rank/selection change is absent,
stop before games. Shadow results diagnose the mechanism; their fixed search
tree is not evidence of playing strength under the new policy.

Then use four immutable policies:

| Policy | A: winner labels + response rate | B: normalized LMR consumer |
| --- | --- | --- |
| 00 | Current | Current |
| 10 | New | Current |
| 01 | Current | New |
| 11 | New | New |

For a bounded exploratory pilot, predeclare two paired-opening comparisons:
`10 versus 00` and `11 versus 01`, with the same 64 held-out opening pairs and
color reversal in each (256 games total). Their difference estimates whether
the learning package's effect changes under the new consumer. A separately
predeclared `11 versus 00` comparison on 64 pairs adds 128 games to check the
actual complete package. Use opening-block resampling, keep exact binaries and
clock settings fixed, and do not extend a favorable cell after looking.

This pilot can expose a **large** interaction or a clearly bad package; it
cannot resolve a few Elo or establish that interaction is necessary merely
because individual arms have wide intervals. The difference of Elo estimates
also assumes approximately additive/transitive latent strengths; report the
paired scores and uncertainty, not only an interaction point estimate. A small
inconclusive pilot stays inconclusive. Only the predeclared full-package
strength gate can admit a change. No constants are chosen from the prettiest
cell, and no whole-family closure follows from one negative package.

## Adversarial clock note

Moving soft checks to completed-iteration boundaries does not create free
thinking time. With identical soft budgets it can let an admitted iteration
spend substantially longer, particularly through aspiration re-searches.
Consequently it can lower the last-incomplete-iteration fraction while draining
the bank earlier and making later decisions worse. The last completed
iteration's duration is an imperfect next-iteration estimate; unchanged hard
limits do not establish unchanged game-level time allocation or flag risk.

Keep boundary admission unthrottled, keep the existing projection and
stability/effort factors initially, and preserve hard/emergency/node/external
cancellation plus last-completed-result safeguards. Record stop reasons,
completed decision depth, hard aborts, retained bank at matched game plies,
and final move quality; tail fraction alone is not the target. The existing
25.529% NGN tail is not 25.529% wasted: useful TT/history/completed subtrees can
survive it. Counter's final-info convention prevents its near-zero log tail
from being a valid negative control.

The prior T1a projection-removal loss is particularly relevant contrary
evidence: more expenditure can lose even with zero flag-outs. Boundary-only
soft stopping is a distinct, untested architecture, not a demonstrated gain.
