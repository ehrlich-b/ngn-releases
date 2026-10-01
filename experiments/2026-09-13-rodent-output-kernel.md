# Rodent AVX2 output kernel: accepted

The exact-integer output kernel passes its frozen speed gate: **45.55% less
time** on the six one-thread cold 400k-node search fixtures, with identical
cold/warm search results and allocations. Direct output calculation is **10.08x**
faster. This is a Rodent-backend speed result, not a rating or SMP gain.

## Scope and identity

Only the output dot product changes. The `amd64.v3` build uses AVX2; other builds
retain the portable implementation. The existing network, int32 wrapping,
truncating divisions, release-static scaling and engine clamp are unchanged.
No update-kernel, search-policy, default-evaluator or deployment change is included.

For clipped `v` in `[0,255]`, the kernel uses
`v² = v*floor(v/2) + v*ceil(v/2)`. Both partials fit signed int16; multiplying
their pair by duplicated signed weights reconstructs the original lane product.
Vector additions and horizontal reduction preserve the scalar sum modulo 2³².
Independent source review verified packing, bounds, extreme weights and ABI.

The accepted production patch is SHA-256
`2c3d77447b591329cbb45a479c62dd6493d2d01a3b2c52dc320a1df88b099f1a`,
on Go-source base `637043a` (same Go source as manifest base `20ee9c7`).
Eight production/regression files are integrated; private timing probes remain
experiment artifacts. Superseded executable-mode patches were not integrated.

## Frozen gate and result

WSL only, Go 1.25.5. Both search binaries use identical `GOAMD64=v3`, CGO-disabled
build settings; portable versus selected direct tests use the same candidate
binary. The separate v1 build checks fallback correctness, not comparative speed.

- Exact release and incremental oracles, kernel bit/layout/overflow tests and
  the v3 dispatch witness pass before timing.
- Six-fixture cold/warm node, score, PV and diagnostic trajectories are byte-identical:
  SHA-256 `07574e16014b30afbe99980e445e77f034255447d5504205f07074e6a6a86333`.
- Ten paired direct blocks alternate ordering; selected/portable median ratio
  **0.09919022376838979**, against a maximum of 2/3. Every record class and both
  order strata improve, with zero kernel allocations.
- Ten paired search blocks alternate ordering, each using all six frozen FENs,
  400k nodes, Hash128 and Threads1. Equal-weight median fixture ratio
  **0.5444771458731007**, against a maximum of 0.97. Fixture medians range
  0.529499–0.589700; order-stratum medians are 0.545963 and 0.546533.
  All 60 paired fixture rows favor the candidate, with exact allocation identity.
- All **44 stages** finish with zero command/supervisor return codes, empty
  stderr and no surviving owned process. Interference stays below one non-owned
  CPU core (observed maximum 0.435871); no timing evidence is rejected.
- Full short/race tests, v1/v3 exact-network oracles, configured engine tests,
  and Linux/Windows v3 builds pass. Raw validation outputs are retained.

An independent Sol audit recomputed the decision from raw benchmark output,
checked every stage receipt and rebuilt the inventory. Root inspected the actual
terminal/decision files and verified all eight integrated source hashes.

Run: `/home/ehrli/repos/ngn-rodent-output-gate-20260913/run-001`.
The [compact evidence archive](2026-09-13-rodent-anand-artifacts/output-kernel/)
retains the frozen contract and result; binaries and bulky telemetry stay on WSL.
Decision SHA-256:
`3ed26b49f261719e18e64c407d5376e64656110657187c8ecae49ab4c03a0b96`.

## Next

The next experiment compares the now-optimized Rodent backend with the accepted
Counter configuration in real-clock paired games. It needs its own fresh A/A,
preflight, legal replay and fixed decision rule. No additional absolute-rating
cycle, owned-network training or second speculative optimization is needed first.
The 3300 objective remains unverified.
