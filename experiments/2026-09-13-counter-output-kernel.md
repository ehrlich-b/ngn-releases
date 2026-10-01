# Exact-order Counter output kernel

## Reassessment and selected goal

The September 13 inspection found both main checkouts at
`a12e1976981fbf89b148a5b229689bf06916a8ba`, with no engine-source change since
the accepted Counter/AVX2 baseline. The Mac's unrelated untracked `SHA256SUMS`
is preserved. WSL main is clean. The previous quiet-promotion experiment is
complete and shelved, not a pending or provisional gain.

The [rating reassessment](2026-09-12-rating-reassessment.md) remains the current
orientation: roughly 3000–3100 at one thread, and −186 [−230,−143] relative to
Counter8 in the matched eight-thread test. No additional rating games are
justified by this source state. The packaged Counter release remains held;
this experiment does not authorize deployment or reopen the external matrix.

The most reasonable next goal is to test the exact-order Counter output kernel
already selected in [the previous run](2026-09-12-qpromo-gate.md#next-experiment-selected-before-this-verdict).
The existing `evaluateAccumulator` loop still performs 512 scalar comparisons,
multiplications and ordered additions. Earlier accepted profiling identified
scalar evaluation as a major cost; the feature-row AVX2 optimization left this
output loop unchanged. This is a plausible opportunity, not a measured current
speedup. The subsequent September 13 direction explicitly defers owned training
and data work. Existing-network integration and broad search-policy changes
remain larger projects with higher confirmation cost.

**Goal:** prove exact numerical and fixed-search identity for one small AVX2
output kernel, then integrate only if it clears the predeclared speed gates;
otherwise archive the measured rejection and advance the queue.

## Scope and fixed acceptance boundaries

- Immediate base: `a12e1976981fbf89b148a5b229689bf06916a8ba`.
- Candidate: isolated WSL worktree `ngn-counter-output-20260913`; no speculative
  production changes on main and no stacking with the shelved promotion patch.
- AVX2 handles positive-lane comparison and independently rounded float32
  multiplication. Scalar additions consume active lanes in ascending original
  order. Initial sum is positive zero; output bias is added exactly once.
- No fused multiply-add, horizontal reduction, evaluator-policy, search,
  time-management or model change. The portable scalar implementation remains
  the reference and non-v3 fallback.
- Require exact raw bits on official full/incremental reference vectors and
  discriminating finite-input tests, including all eight-lane activity masks,
  signed zeros, subnormals, cancellation and rounding-order witnesses.
- Require zero kernel allocations, no input mutation, correct build-tag
  selection, GNU disassembly proving vector products without fused operations,
  and exact fixed-search node/score/PV identity on the existing six fixtures.
- Direct-kernel speedup must be at least 1.5x. Whole-search equal-weight median
  time reduction must be at least 3%, every fixture median must improve, and
  both reciprocal execution-order strata must improve. No acceptance on sign.
- WSL only for implementation execution, builds, tests, profiles and timing.
  Preserve unrelated jobs; coordinate all owned tests/builds before timing.
- This is a pure-speed gate, not an Elo estimate. No real-clock game campaign
  is needed for an exactly behavior-identical, repeatably faster change.

Sol owns implementation; a separate Sol agent identifies the existing timing
tools and frozen reference inputs; another independently reviews numerical
correctness and the final evidence. Root reviews the contracts, freezes timing
details before measurement and controls the acceptance decision.

## Status

**Accepted.** The fixed run completed `PASS_PERFORMANCE`, all 42 stages clean.
Independent reconstruction verified all 275 inventoried files, all 15 frozen
inputs at both original and copied paths, exact raw timing calculations,
source/binary bindings, and cold/warm search identity. Root accepts the exact
kernel and its tests; the shared engine benchmark probe stays research-only.

| Measurement | Result | Predeclared requirement |
|---|---:|---:|
| Direct selected/portable median time ratio | 0.494535 (2.0221x speedup) | ≤0.666667 |
| Whole-search equal-weight median time ratio | 0.721547 (27.85% less time) | ≤0.97 |
| Six fixture median ratios | 0.711213–0.751216 | Every fixture <1 |
| Search order-stratum medians | 0.722055 / 0.722723 | Both <1 |
| Kernel allocations | 0 B/op, 0 allocs/op | Zero |
| Search allocations, both binaries | 135,409,280 B/op, 9 allocs/op | Exact match |

All 60 paired fixture timing ratios favor the candidate (0.688332–0.762081).
Both large correctness snapshots are byte-identical, SHA-256
`2ffbc729295f3794883502e2c74779614623ef132844c795dc72c8d2e1dd1656`.
The prelaunch non-owned load was 0.20394 CPU cores; the maximum of 14 monitored
samples was 0.23195, below the fixed one-core rejection threshold. No stage had
stderr, an operational failure, or a surviving child.

These are one-thread cold 400k-node fixture measurements on WSL, including lazy
TT allocation. They are not a steady-game, eight-thread, or Elo result. The
direct corpus and real search encounter different activation distributions;
their speedups are separate measurements, not additive or interchangeable.
No new rating estimate follows. No network, search policy, clock policy or
deployment default changed; `build/ngn` remains the previously installed release.

Short engine, race engine, full short suite, official Counter oracle at
GOAMD64 v1 and v3, and fixed-search correctness all passed before timing.
Adversarial tests cover all eight-lane masks, signed zeros, subnormals,
non-fused product rounding, cross-block and partial-sum counterexamples,
bias-last ordering, finite randomized inputs, mutation and allocation.
Disassembly confirms ordered comparison, premasking, independent vector
products and ascending scalar addition; no FMA or horizontal sum.

The integrated ten `countereval` files match the tested candidate's SHA-256s
exactly. Subsequent Linux amd64-v3 and Windows amd64-v3 packaging builds both
pass with Go 1.25.5; formatting checks are clean. Their commands, source hashes
and output hashes are in `integration-receipt.json`. These binaries remain
versioned research artifacts, not installed defaults.

The [evidence directory](2026-09-13-counter-output-artifacts/README.md) preserves
the exact candidate patch, build receipts, verification logs, held/released/frozen
manifests, driver, raw timing decision, inventory and independent review.
Original binaries, model and full snapshots remain on WSL. The prelaunch
file-mode correction is documented; there was no performance-dependent rerun.
The independent verifier's first version had a command-wrapper comparison bug;
its corrected version passed on the unchanged run. That was a reviewer error,
not a benchmark failure or discarded timing sample.

**Next:** use the bounded competitor review to select a fresh CPU/allocation
profile of the accepted faster engine and one attributable runtime improvement.
Do not reuse old hotspot percentages as current or restart an inconclusive
experiment merely because its point estimate was positive. Existing networks
only; no owned NNUE training or data campaign.

## Timing protocol frozen before measurement

The unchanged six-fixture probe is
`/home/ehrli/repos/ngn-counter-transition-fastpath-v1/engine/counter_transition_profile_test.go`,
SHA-256 `8623a3f8d486f9fcd3231b835469890ece52e79a9cabcf5bb6d5562d664dea4c`.
Both engine test binaries compile those exact bytes. It first emits cold/warm
node, score, PV, callback and root-restoration snapshots; the candidate and base
JSON must be byte-identical. Its six fixtures are start, kiwipete and asymmetric
positions, each with both sides to move.

1. Direct gate: ten paired blocks, alternating portable-first and selected-first.
   The same candidate test binary runs one separately filtered implementation
   per process, `250ms` per benchmark. Inputs are all captured accumulator records
   in the frozen official oracle JSON with the pinned Counter model. Every row
   must report zero allocations. Primary ratio is the median of ten
   selected/portable ns-per-operation ratios; require at most `1/1.5`, and each
   order stratum's median ratio below one. Otherwise stop and shelve.
2. Search gate, only after the direct gate passes: ten paired blocks, alternating
   base-first and candidate-first. Each process runs all six unchanged fixtures
   once at 400,000 nodes, Hash128, Threads1. Require the median of six per-fixture
   median candidate/base time ratios to be at most `0.97`; each fixture median
   and both order-stratum medians must be below one. Allocation counts must
   match. These are cold first searches, including lazy transposition-table
   allocation, not a measurement of steady-game speed or Elo.

Child processes run on physical-core representative CPU4; the supervising driver
and monitoring thread run on CPU6. Runtime is pinned to GOMAXPROCS1, GOGC100,
GOMEMLIMIT=off, and empty GODEBUG. Each stage uses the existing process supervisor,
a 300-second timeout and a 1-GiB memory cap. Before launch and throughout the run,
five-second samples reject timing evidence above one non-owned CPU core.
All owned builds and correctness suites stop before timing begins. Unrelated
jobs are neither stopped nor modified.

The driver refuses an existing output directory, freezes and hashes every
declared input, and records original logs, commands, process receipts, an
inventory and a terminal decision. Resource/parser/monitor failures are not
speed verdicts. No timing sample is dropped or schedule extended based on
observed performance.
