# Rodent incremental update AVX2

The single frozen speed gate completed **PASS_PERFORMANCE** and independent
source and final-evidence reviews **accepted** the candidate. No new Elo claim
follows from this benchmark. The base is accepted `7623779`, including the
Rodent output kernel.

## Mechanism and correctness

The post-output profile put incremental updates at 58.21% flat CPU. This candidate
replaces only the lane arithmetic for the existing add/subtract patterns: quiet
and promotion (AS), capture/en passant (ASS), and castling (ASAS). Three AVX2
kernels perform 32 blocks of 16 int16 lanes, using modular VPADDW/VPSUBW in the
original order. Portable kernels and disjoint build tags preserve fallback.
Frame copy, king-mirror refresh, validation, stack ownership, null/pop and search
policy remain unchanged. The generic loop retains other private-helper patterns.

Independent source review accepted bounds, ABI, dispatch and arithmetic. WSL
focused v1/v3 tests, full release/transition oracles v1/v3, configured Rodent
engine integration, full short suite and full short race suite all passed.
Linux and Windows amd64-v3 packaging also passed with empty build diagnostics;
the [packaging receipt](2026-09-13-rodent-anand-artifacts/update-kernel/candidate/packaging/packaging-receipt.json)
records exact commands and binary hashes. All eight integrated source/test files
match the frozen candidate receipt; the private benchmark is not production code.
Adversarial tests cover wraparound, signed extremes, random lanes, boundaries,
unaligned operands, immutable model rows and zero kernel allocations.

## Fixed performance gate

Matched Linux/amd64 v3 base/candidate builds use the existing six fixtures,
Hash128, one thread and 400,000 nodes. Ten alternating paired blocks were fixed
before timing. Acceptance requires search ratio at most 0.97, every fixture and
both order strata improving, direct classes improving without allocations, exact
cold/warm search identity and clean supervision. No extension or rerun occurred.

- Search candidate/base median: **0.411622**, or **58.84% less time** (~2.43x).
- All six fixture medians improve, ranging from 0.386867 to 0.442859.
- Base-first / candidate-first medians: 0.411658 / 0.414145.
- Direct selected/original-generic median: 0.023058; AS, ASS and ASAS each improve.
  This is an isolated arithmetic microbenchmark, not a whole-engine multiplier.
- Exact cold/warm identity SHA-256:
  `07574e16014b30afbe99980e445e77f034255447d5504205f07074e6a6a86333`.
- Search allocations remain exactly 135409224 B/op and 12 allocs/op; these cold
  rows include lazy TT allocation. Direct kernels allocate zero bytes/objects.
- All 44 stage receipts are clean. Interference monitor recorded 19 samples,
  maximum 0.502 non-owned cores; no rejection.

The [candidate receipt](2026-09-13-rodent-anand-artifacts/update-kernel/candidate/receipt.json)
pins source, patches, models, oracle fixtures and raw test output. Frozen run:
`/home/ehrli/repos/ngn-rodent-update-gate-20260913/run-001`.
Decision SHA-256:
`9bfb8330c5c7764539b8729de08f6b2df6c7e9b8050123e9924072b085acc1b0`.

The [compact gate archive](2026-09-13-rodent-anand-artifacts/update-kernel/gate/results.json)
preserves the frozen manifests, driver, build receipt, decision and inventory;
larger raw measurements remain in WSL at their recorded paths/checksums. The
independent review reparsed all 40 timing outputs, all 44 receipts, both identity
snapshots and the complete inventory, reproducing the verdict exactly.

This gain is relative to the already output-optimized Rodent engine. It does not
establish an absolute rating, an eight-thread speedup or deployment readiness.
Existing external network selection and runtime defaults remain unchanged.
