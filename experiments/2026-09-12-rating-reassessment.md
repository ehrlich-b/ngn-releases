# Current rating: separate configurations, then resume improvement

September 12 user direction: reassess with Astra coordinating Sol subagents,
harden the rating understanding without another long calibration campaign,
then continue improving the engine.

## Decision

Run **zero new calibration games** now. The completed evidence supports a
one-thread orientation around **3000–3100** on the pinned published labels.
It establishes **−186 relative Elo against Counter8**, not an eight-thread
absolute rating. Retire the former 3130 headline: it averaged a one-thread
Rodent mapping and an eight-thread Counter mapping.

The limiting uncertainty is transfer between opponent ratings and local test
conditions, not simply game count. Rodent V1.2 is an unrated target in the frozen
inventory and its incomplete cell cannot solve the anchor problem. Future
independent rating-list testing is more useful for a formal rating than repeatedly
measuring these mismatched anchors.

## What was measured

All three accepted cells used the same candidate binary, built from `a51233b`
(tree `622b3f8da8a1af9abfd7da3ee8942593373a0624`):

- NGN SHA-256: `437981db2753e2e9b46fc7185754da63969f284792ed0a6b5182dc192565f910`.
- Counter network SHA-256: `3488baed71f4d432d028e05b65e2ff13f8c9afe7f91974d1a3c34a0691d6670c`.
- Build: `/home/ehrli/repos/ngn-final-integration-gate/output/final-integration-gate-20260906/attempt-001/build/ngn_linux_amd64_v3`.
- Current `b7ffdc8` changes only documentation relative to that behavior source.
- The installed historical `build/ngn` remains a different, classical binary.

Each cell contains 100 completed games, 50 reversed-color opening pairs,
30+0.3, concurrency one, Hash 128 MiB per engine, and no adjudication on WSL.
The independent audit's `pair_audit[].half_points` is the sampling unit.
Pentanomial categories count pair totals 0, 0.5, 1, 1.5 and 2 points.
Intervals are percentile intervals from 100,000 complete-pair bootstrap samples,
transformed by `400 log10(score / (1 − score))`. They quantify match sampling
uncertainty conditional on this test, not rating-list transfer uncertainty.

The [WSL-only replay script](2026-09-12-rating-reassessment.py) verifies the
exact binary, model, build receipt, three accepted audit/comparison/manifest
sets and recorded anchor sources before reproducing the stored bootstrap
results with seed `0x4e474e4558543236`. The [checked result](2026-09-12-rating-reassessment.json)
contains every source path and digest. Root reviewed the script, required an
actual model-byte check, and verified the copied source/result SHA-256s:

- Script: `408b5782bd28b72c405bf1038282c072f39085d6e8d84f178e51c1ad5fbe2b2e`.
- Result: `c00d2b22ba5d0bada0111cee7ee2b3295444576cebb7b22a1be04bd348d92230`.

From the WSL checkout, replay into a new output directory (existing output is
refused):

```sh
python3 experiments/2026-09-12-rating-reassessment.py --output /home/ehrli/repos/ngn-rating-audit/output/new-replay/result.json
```

The script needs the preserved WSL evidence trees named in the receipt. It
does not require the old runner worktree to become a passing development tree,
and it launches no engines or games. Original v1 and corrected v2 analysis
artifacts remain under `/home/ehrli/repos/ngn-rating-audit/output/`.

| Opponent / matched width | NGN W/D/L | Pentanomial | Relative Elo [paired 95%] |
|---|---:|---|---:|
| Counter 5.5 / 1 | 1/25/74 | [27,19,4,0,0] | −322.7 [−401.9,−263.4] |
| Rodent V1.1 Anand/testers / 1 | 1/15/84 | [35,13,2,0,0] | −412.8 [−511.5,−338.0] |
| Counter 5.5 / 8 | 1/49/50 | [10,30,9,1,0] | −186.2 [−230.2,−143.1] |

Counter cells have accepted recovery receipts after separately proved reporting
classifications. Rodent V1.1 width one completed directly. Rodent V1.1 width eight
was unavailable, and the failed/partial V1.2 games are excluded. Original results
and failure records remain unchanged; see the [release audit](2026-09-12-release-state-audit.md).

## Absolute-rating sensitivity

These are alternative maps, not independent estimates to average:

| Local cell | Published label applied | Mapped point [match-only interval] |
|---|---:|---:|
| Counter1 | 3333, author-reported 40/15 context | 3010 [2931,3070] |
| Counter1 | 3359, pinned September 5 Blitz snapshot | 3036 [2957,3096] |
| Rodent V1.1 / 1 | 3522, pinned September 5 Blitz snapshot | 3109 [3010,3184] |
| Counter8 | 3333, unmatched rating-list/width label | 3147 [3103,3190] |
| Counter8 | 3359, unmatched rating-list/width label | 3173 [3129,3216] |

The snapshot reports anchor uncertainty of about ±10 for Counter and ±16 for
Rodent. Extending Rodent's interval outward by 16 yields [2994,3200]; this
sensitivity is not a joint 95% confidence interval. Across one-thread mappings,
roughly 2930–3200 describes the span of these conditional intervals, not a
precisely calibrated probability statement.

The exact Rodent testers/Anand artifact has not been proved equivalent to the
CCRL rated entry. The local clock, book, tablebase configuration and hardware
also differ. These systematic effects are not captured by the bootstrap.
The width-one/width-eight Counter gap cannot isolate NGN's SMP gain because
Counter also changes width. Never combine these rows into a single rating.

The accepted historical **2884 [2834,2934]** result measures classical `53e4d1b`
on the separate native-Windows 400-game, 120+1 fixed-anchor calibration. Internal
NNUE/SMP SPRTs establish relative improvements under their own contracts; their
Elo estimates cannot be added to that historical number.

## Next experiment

Test the narrowly scoped quiet-promotion qsearch candidate against this accepted
Counter baseline. Preserve a fresh A/A, a frozen paired schedule, complete legal
and terminal audits, and a predeclared acceptance rule. Do not extend the run
after seeing its score. An inconclusive result is useful only as a recorded
decision to move on; it does not authorize a speculative keep.

The prior release contract remains held. This assessment changes the work queue
under the user's current instruction; it does not reinterpret failed games or
claim that the old deployment gate passed.
