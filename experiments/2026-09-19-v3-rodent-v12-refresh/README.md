# Rodent V1.2 AVX2 king-perspective refresh: accepted

The exact variable-row refresh kernel passes its correctness and whole-search
speed gates. On top of accepted V2, it reduces the six-fixture 400,000-node
search time by **21.55%**. This is a backend-speed result, not an Elo or
model-strength decision.

## Why this ticket was admitted

The fresh post-V2 profile used the exact accepted V2 engine binary
`53efe6c1975684379d98497c7f00b41c8e766a0764f0744a13709c8402211a06` and a
six-fixture, three-repetition, 400,000-node one-thread workload. Of 10.75 seconds
of labeled search CPU, `rodentv12eval.(*Model).refreshPerspective` consumed
2.52 seconds flat (**23.44%**, 24.65% cumulative), the largest remaining flat
cost. Output had fallen to 5.86%; the update kernels together were 3.53%.
The retained profile is
`/home/ehrli/post-v2-profile-20260919-001/cpu.pprof`, SHA-256
`cc80ba89c3dfe888825f9696f2e9573a6753f7da14f2bd8a55833698ad1b6356`.

## Scope and exactness

`refreshPerspective` gathers selected input-row pointers in the existing
plane-then-ascending-square order. Validated piece planes are disjoint, so a
fixed 64-row array represents every admitted board. The selected portable or
amd64.v3 kernel initializes each lane from its bias and adds rows in exactly
that gathered order. Go int16 addition and AVX2 `VPADDW` are the same modular
16-bit operation, including overflow.

The AVX2 implementation processes 48 unaligned 32-byte blocks, retains the row
order independently for every lane, has compile-time guards for 768 lanes and
64 rows, and ends with `VZEROUPPER`. Model layout, feature indices, transition
classification, frame copies, update/output kernels and search are unchanged.

Tests compare every lane with the portable oracle for 0, 1, 2, 16, 32 and 64
rows; adversarial and deterministic random full-int16 values; boundary lanes;
unaligned bias/destination/rows; immutable inputs; and zero allocations. A
fully occupied valid 64-square board is compared with scalar full refresh for
both perspectives. Existing context/oracle tests independently cover king
bucket/mirror changes and compare every incremental frame, raw score and static
score with fresh full refresh.

## Performance and identity

The accepted paired run is
`/home/ehrli/v3-refresh-gate-run-20260919-002` on WSL, Go 1.25.5 and a Ryzen 7
9800X3D. Its immediate base is accepted V2 commit `aca6525`; the released V1.2
model is 4,744,768 bytes with SHA-256
`c35a1abc1b8c1cb1d5f4221454d494c1a6da1ed9088fd51ab27038bfa74b5053`.

- Base and candidate cold/warm completed-depth trajectories, final nodes,
  scores, PVs, diagnostics, root restoration and fresh/incremental scores are
  byte-identical. Receipt SHA-256:
  `c6d99d89dea6ea5f359a0f4c8dad3af13c488908627dcab98c388222d7b85bf7`.
- Ten clean alternating blocks give equal-weight median candidate/base ratio
  **0.7845098284113629**, or **21.55% less time**. Fixture medians span
  0.673927–0.840175; base-first and candidate-first medians are 0.788543 and
  0.785729.
- All **60/60** paired fixture cells improve, with exact allocation identity at
  135,409,224 B/op and 12 allocs/op. No accepted timing pair was discarded.
- A clean seven-repetition direct benchmark measured median portable/selected
  times of 567.8/45.65 ns for two rows, 3,493/176.5 ns for 16 rows and
  7,001/415.8 ns for 32 rows: **12.44x, 19.79x and 16.84x** faster,
  respectively, with zero bytes and zero allocations per operation.
- The shared driver's unchanged output-dot control remained stable at median
  selected/portable ratio 0.086294. It is a control, not V3 direct evidence.

Multiplying the independently accepted V1, V2 and V3 ratios gives 0.175416, an
inferred **5.70x** search speedup over the original portable V1.2 backend. This
product is a sanity estimate across separate experiments, not a fresh direct
comparison or Elo claim.

## Shared-host disposition and validation

Three preflight-only launches refused to start while an unrelated GPU-training
workflow consumed one CPU core. A first unattended attempt used an incorrect
waiter pattern; the gate's stage-local 250 ms monitor then rejected every noisy
search pair and failed closed after its fixed attempt cap. That invalid run is
preserved at `/home/ehrli/v3-refresh-gate-run-20260919-001` and is not used.
The corrected silent waiter neither stopped nor reprioritized any other job;
accepted run 002 began only after the workflow ended and completed with zero
discarded attempts. The Lean hopper remained live and untouched throughout.

Released-network raw/context and scoped engine oracles pass on Linux amd64.v1
and amd64.v3. The full short suite, race-enabled evaluator suite, plan-specified
`-short -race` engine suite, and Linux v1, Linux v3 and Windows v3 build-only
gates pass; package vet, including assembly declarations, also passes. A
deliberately overbroad race command without `-short` is excluded:
it admitted the repository's intentional shared-position concurrency stress
test (which reports its known race) and depth-100 stress test (which hits Go's
10-minute alarm). The correct prescribed command passes. No private probe is
present in the production tree.

The compact `result.json` records decision figures and raw hashes. Bulky raw
receipts remain on WSL. V3 is accepted. Re-profile the exact accepted backend
next, then complete G0b and the fair V1.2-versus-optimized-Anand game gate unless
the profile reveals another major exact backend cost.
