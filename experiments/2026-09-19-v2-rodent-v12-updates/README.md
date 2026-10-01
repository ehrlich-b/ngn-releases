# Rodent V1.2 AVX2 incremental updates: accepted

The exact 768-lane AS, ASS and ASAS update kernels pass their correctness and
whole-search speed gates. On top of accepted V1, they reduce the six-fixture
400,000-node search time by **63.43%**. This is a backend-speed result, not an
Elo or model-strength decision.

## Scope and exactness

The production change adds portable and `amd64.v3` implementations for the
three ordered update shapes emitted by `deriveTransition`:

- AS: add destination feature, subtract source feature;
- ASS: AS plus a captured feature subtraction;
- ASAS: AS plus castling-rook destination/source updates.

`applyUpdates` dispatches only those exact patterns. Its existing generic loop
remains the fallback for any future pattern, and king-bucket/mirror refreshes
still take the unchanged full-refresh path. Model layout, frame copying,
transition validation, null/pop ownership, output evaluation and search are
unchanged.

Every lane operation is modular int16 arithmetic. AVX2 `VPADDW`/`VPSUBW`
matches Go's wrapping add/subtract lane-for-lane and preserves the semantic
operation order. Each kernel processes 48 unaligned 32-byte blocks, has
compile-time 768-lane width guards and ends with `VZEROUPPER`.

Tests compare all lanes against the portable oracle across adversarial values
and 64 deterministic full-int16 random trials, explicitly touch the 512/768
boundary lanes, exercise unaligned operands, prove weight-row immutability and
zero allocations, and check v1/v3 dispatch. Existing V1.2 context tests cover
quiet/capture, both en-passant directions, all castling sides, every promotion
and capture-promotion, king bucket/mirror changes, rejected transitions, deep
sibling reuse, nulls, reset and independent contexts. Every incremental frame,
raw score and release-static score is compared with a fresh full refresh.

## Performance and identity

The accepted whole-search run is
`/home/ehrli/v2-update-gate-run-20260919-001` on WSL, Go 1.25.5 and a Ryzen 7
9800X3D. The immediate base is accepted V1 commit `9f5f498`. The released V1.2
model is 4,744,768 bytes with SHA-256
`c35a1abc1b8c1cb1d5f4221454d494c1a6da1ed9088fd51ab27038bfa74b5053`.

- Base and candidate cold/warm completed-depth trajectories, final nodes,
  scores, PVs, diagnostics, root restoration and fresh/incremental scores are
  byte-identical. Receipt SHA-256:
  `c6d99d89dea6ea5f359a0f4c8dad3af13c488908627dcab98c388222d7b85bf7`.
- Ten clean alternating search blocks give equal-weight median candidate/base
  ratio **0.36568307213089213**, or **63.43% less time**. Fixture medians span
  0.342409–0.391493; base-first and candidate-first medians are 0.365093 and
  0.373790.
- All **60/60** accepted fixture pairs improve, with exact allocation identity
  at 135,409,224 B/op and 12 allocs/op (the cold 128 MiB hash setup).
- One complete block-9 attempt was discarded when the stage-local 250 ms
  monitor caught an unrelated Python process above the one-core ceiling. The
  same order was replayed cleanly; the rejected attempt remains in the raw WSL
  receipt.
- A separate three-repetition, 500 ms direct preflight measured median
  portable/selected times of 300.4/17.09 ns for AS, 459.2/22.31 ns for ASS and
  617.0/26.04 ns for ASAS: **17.58x, 20.58x and 23.69x** faster, respectively,
  with zero bytes and zero allocations per operation.

The shared search driver also reran V1's output-dot control; those figures are
not used as V2 evidence. The compact `result.json` records V2's decision and
raw artifact hashes. The unchanged driver and inert search probe are retained
once in the V1 evidence directory; bulky raw receipts remain on WSL.

Multiplying the independently accepted V1 and V2 ratios gives 0.22360, an
inferred **4.47x** speedup over the original portable V1.2 implementation.
That is a useful sanity estimate only: the next action is a fresh post-V2
profile, not treating multiplied medians as a new measurement.

## Validation and next step

The released raw/context oracle and configured engine oracle pass on Linux
`amd64.v1` and `amd64.v3`. The full short suite, race-enabled
`rodentv12eval`/`engine` suites, and Linux v1, Linux v3 and Windows v3 build-only
gates all pass. No private probe is present in the production tree.

V2 is accepted. Re-profile the exact accepted backend next. Implement a refresh
kernel only if refresh remains materially dominant; otherwise advance directly
to the fair V1.2-versus-optimized-Anand game gate.
