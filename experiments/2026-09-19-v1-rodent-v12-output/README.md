# Rodent V1.2 AVX2 output kernel: accepted

The exact 768-lane output kernel passes its frozen correctness and speed gates.
On the six one-thread 400,000-node search fixtures it uses **38.85% less time**
than the immediate portable baseline. The isolated output dot product is
**11.51x faster**. This is a backend-speed result, not an Elo or default-network
decision.

## Scope and arithmetic

The production change only extracts the Rodent V1.2 output dot product and
selects an AVX2 implementation for `amd64.v3`; all other builds retain the
portable implementation. Model loading, accumulator updates and refreshes,
material-head selection, output bias/scaling, search and UCI behavior do not
change.

For each clipped lane `v` in `[0,255]`, the AVX2 kernel reconstructs
`v²*w` as `v*floor(v/2)*w + v*ceil(v/2)*w`. Both activation products fit signed
16 bits, including `v=255`; duplicated signed weights and `VPMADDWD` therefore
preserve `w=-32768`. Vector and horizontal additions wrap modulo 2^32. That is
the same ring as the released evaluator's int32 accumulation, so reassociating
the former four partial sums is exact. Bias insertion and both truncating
divisions remain in their original order in Go.

The kernel processes 48 blocks of 16 lanes, uses unaligned loads, has compile-
time guards for the 768-lane width and ends with `VZEROUPPER`. Tests cover zero,
signed extremes, positive/negative int32 overflow, 256 deterministic full-int16
random samples, unaligned inputs, both perspective orders, all eight output
heads, no mutation, zero kernel allocations and v1/v3 dispatch.

## Frozen performance result

The accepted run is
`/home/ehrli/v1-output-gate-run-20260919-003` on WSL, Go 1.25.5 and a Ryzen 7
9800X3D. Both engine binaries were built from source base `a83b449`; the
candidate differs only by this output-kernel ticket. The released V1.2 model is
4,744,768 bytes with SHA-256
`c35a1abc1b8c1cb1d5f4221454d494c1a6da1ed9088fd51ab27038bfa74b5053`.

- Base and candidate produced byte-identical cold and warm completed-depth
  trajectories, final nodes/scores/PVs/diagnostics, root restoration and
  incremental/fresh static scores. Receipt SHA-256:
  `c6d99d89dea6ea5f359a0f4c8dad3af13c488908627dcab98c388222d7b85bf7`.
- Ten alternating direct blocks give selected/portable median ratio
  **0.08688188540941194**. Portable-first and selected-first medians are
  0.0870361 and 0.0867277; both kernels allocate zero bytes.
- Ten clean alternating search blocks give equal-weight median candidate/base
  ratio **0.611458128958916**. Fixture medians span 0.601425–0.629194; the
  base-first and candidate-first strata are 0.612719 and 0.610594.
- All **60/60** accepted fixture pairs improve. Every row has exact allocation
  identity: 135,409,224 B/op and 12 allocs/op, dominated by the cold 128 MiB
  hash setup outside the timed search.
- A stage-local 250 ms monitor discarded six complete attempts at block 7 while
  an unrelated Python process occupied one core. The gate retried the same
  order and admitted only ten pairs with no sample above the frozen one-core
  ceiling. The discarded attempts and samples remain in the raw WSL receipt.

The compact `result.json` records all decision figures and hashes. The frozen
driver, manifest and inert private probe are retained here; bulky raw decision,
interference and identity receipts remain under the accepted WSL run root.
Two earlier runs are explicitly invalid: run 001 monitored untimed identity and
run 002 caught a one-core contaminant inside a timing block.

## Validation

The exact released-network raw/context oracle passes on Linux `amd64.v1` and
`amd64.v3`; the configured engine V1.2 oracle passes on both. The full short
suite, race-enabled `rodentv12eval` and `engine` suites, and Linux v1, Linux v3
and Windows v3 build-only gates all pass. No private probe is present in the
production tree.

V1 is accepted. The next bounded implementation is V2: exact 768-lane 2/3/4-row
incremental update kernels on this accepted base, followed by a re-profile
before considering refresh work.
