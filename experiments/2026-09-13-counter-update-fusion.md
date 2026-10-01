# Counter update-path speed experiment

## Goal and admission

Immediate base: `9ef265664499bc79fa129bf75adb7aaa52b7e5c6`, including the
accepted exact-order output kernel. The [competitor review](2026-09-13-competitor-review.md)
selects a current CPU/allocation profile before another implementation.

Profile both the existing six cold fixed-search fixtures and positions after
plies 23 and 48 of the first three accepted Counter1 games, retaining full legal
game history and one searcher per game across those two searches. The source PGN
SHA-256 is `35bdd00b28f585224e6c04392bdbf79cd4472c9ea7549b84acfc4379b07847d2`;
the first three games have 136, 115 and 142 plies, so none needs substitution.
This is sparse-prefix replay with persistent engine state, not timed-game replay.
Cap this diagnostic at one hour. Use the existing Counter network,
one worker, and the authorized WSL machine; no engine computation on the Mac.
Separate setup/allocation costs from hot search costs and avoid adding overlapping
cumulative profile percentages.

Admit only one concrete runtime mechanism: fused accumulator copying and ordered
feature updates if attributable profile cost makes a 3% whole-search improvement
plausible; otherwise consider redundant checked-transition work. If neither has
enough measured cost, record that result instead of implementing speculatively.

## Invariants

- No evaluator values, feature semantics, search/clock policy, network, or
  deployment changes. No training, new calibration games, or SMP redesign.
- Preserve the exact per-lane sequence of independently rounded float32 adds
  and subtracts. No reassociation, signed-weight shortcut or fused multiply-add.
- Preserve complete transition validation, error behavior, actual post-board
  checks, parent immutability, null/pop behavior and transactional growth.
- Retain the portable reference and non-v3 path. Require official full and
  incremental oracles, discriminating numerical tests and complete fixed-search
  identity before timing.
- Sol owns substantial profiling, implementation and independent review. Root
  controls admission, the predeclared performance protocol and integration.

## Performance boundary

Before candidate timing, freeze exact source, binaries, model, fixture inputs,
probe source, commands, environment and decision rule. Reuse the existing
supervisor and reciprocal-order measurement approach, with a fresh run directory.
Run ten reciprocal base/candidate paired blocks. The cold family contains the
six established fixtures; the persistent family has three per-game rows, each
timing both selected searches while excluding setup/replay/checks. Each family
separately requires a median of per-fixture paired median ratios ≤0.97, every
fixture/game median <1, and both order-stratum medians <1. Do not pool the two
families. Require exact snapshot identity at all six persistent roots in addition
to the existing cold/warm snapshots; search allocations must match.
Microbenchmarks diagnose the mechanism, not Elo or adoption, and have no separate
1.5x threshold for this candidate.

No score-conditioned extension, sample deletion or gate relaxation. A valid
negative or inconclusive result shelves the candidate. Operational failures
remain distinct from performance verdicts. No speculative production commit.

## Result: shelved

The fixed gate completed with `SHELVE_PERFORMANCE`: the optimization improved
every fixture but missed the predeclared minimum in both families. No production
source was integrated, no follow-up timing was run, and no deployment changed.
The unrelated untracked `SHA256SUMS` is preserved.

| Search family | Candidate/base time ratio | Less time | Required |
| --- | ---: | ---: | ---: |
| Six cold fixtures | 0.9725927134 | 2.7407% | ≥3% |
| Three persistent-game rows | 0.9712972769 | 2.8703% | ≥3% |

Both order strata and all nine fixture/game medians improve. Allocations match
in every pair. The exact cold/warm snapshot SHA-256 is
`2ffbc729295f3794883502e2c74779614623ef132844c795dc72c8d2e1dd1656`;
the six persistent roots match at
`8eb00369a0f89d8a9e60bd0116beb70115905aec3adeb89510eec80651c71591`.
All 42 supervised stages completed cleanly; interference rejection is false.
The [independent terminal audit](2026-09-13-counter-update-artifacts/gate/review-v1.json)
passes: it recomputes all raw rows, checks every receipt and input/inventory hash,
verifies exact snapshot bytes and allocation identity, and confirms no live
frozen processes. No threshold, sample or verdict changed during review.
This is a valid positive-but-below-bar speed result, not evidence of a regression
or a new Elo estimate. Do not round it into a pass or extend the sample.

The implementation combined copying and exact SA/SSA/SASA feature-row updates
into three fixed-width AVX2 loops. Portable behavior, transition checks, capacity
growth, fixed compatibility context and null/pop behavior remain unchanged.
Independent source and compiled-assembly review passed. Focused v1/v3 tests,
short/race engine tests, full repository short tests, official v1/v3 evaluator
oracles and exact search snapshots passed. The first full-short invocation
discovered archived probe copies as a partial Go package; that evidence-layout
failure is preserved separately, and renaming only the archive copies to inert
`.go.txt` names allowed the unchanged production/test sources to pass.

## Profile and admission evidence

Valid baseline source-line profiles attribute about 7.00% of cold CPU samples
and 5.23% in persistent searches to the accumulator-copy line plus the disjoint
ordered-update call. These are affected-path estimates, not projected savings
or sums of overlapping cumulative frames. The initial 6.44%/5.47% caller-level
proxy combined row-kernel samples with `duffcopy` directly under `PushMove`;
that caller also copies boards/arguments, so the proxy is not an exclusive
accumulator attribution. The source-line view still supports a plausible but
tight 3% target. Estimated memory traffic is not a measured speedup.

The persistent profile completed in 20.79 seconds with no stderr, survivors or
resource violation. Search labels cover 98.19% of sampled CPU; unlabelled
setup/replay accounts for the rest. Raw allocation profiles are dominated by
setup/TT creation and must not be presented as live memory or hot-search churn.
The existing output dot product remains the largest individual CPU consumer
(26.37% cold, 28.40% persistent); move selection and TT access are also prominent.

Two diagnostic harness failures are retained, excluded from attribution and
not counted as candidate verdicts: Windows command parsing stripped the first
cold run's caret selector, inadvertently selecting tests; a subsequent persistent
benchmark incorrectly required a populated PV with reporting disabled. The
corrected selector uses a literal impossible test name. The benchmark now checks
legal best move plus unchanged node/state/raw-value invariants; separate snapshot
tests still require complete legal PV identity. Neither correction changes
production search. The new labelled cold benchmark adds the same probe overhead
to both gate binaries; its timing is not directly comparable to the prior
output-kernel experiment's unlabelled timings.

## Durable evidence and next step

The [compact artifact map](2026-09-13-counter-update-artifacts/README.md) contains
the profile evidence, recoverable candidate patch, test logs, exact manifests,
driver, raw paired values and terminal hashes. Raw profiles, binaries, large
snapshots and complete supervised stage files remain at their frozen WSL paths.

Stop this speed candidate here. The next ranked goal is a bounded diagnostic of
the inherited search-policy/Counter-evaluator interaction, starting with the
three-term correction family on deterministic accepted-game prefixes with full
history. Measure whether it materially changes pruning/reduction and completed
decisions before admitting one playing-strength candidate. A subsequent game
gate would be separately declared; this result does not authorize another
absolute-rating campaign, broad tuning, or own-network training. Stronger
existing-network integration remains a separate follow-on.
