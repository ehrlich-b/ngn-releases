# Rodent V1.2 cost preflight (V0)

V0 found a large, concentrated implementation opportunity rather than a field
of small search tweaks. At equal fixed-node budgets, the exact portable Rodent
V1.2 default evaluator is 6.214x slower than the optimized V1.1 Anand evaluator
across the six cold fixtures and 6.489x slower across the retained persistent
game-prefix workload. This result is a cost diagnosis, not a strength rejection:
the trees and scores differ between networks, and lower scalar NPS is not a
valid reason to discard V1.2.

## Pinned setup

- Source: `5254b09971af9ffc8ae51b321accbc842fd52bfd`.
- Linux/amd64.v3, Ryzen 7 9800X3D, `GOMAXPROCS=1`, engine Threads 1,
  Hash 128 MiB.
- Six cold positions at 400,000 nodes/search and four persistent positions at
  200,000 nodes/search, three benchmark repetitions.
- V1.1 Anand model: SHA-256
  `5f7480b56538e9e64ee02fb5b4842bf0394ee44b707dae3a63ec7f902abd0afb`,
  789,568 bytes.
- V1.2 default non-Tal model: SHA-256
  `c35a1abc1b8c1cb1d5f4221454d494c1a6da1ed9088fd51ab27038bfa74b5053`,
  4,744,768 bytes.
- The supervised run completed normally in 63.122 seconds, peaked at 449,564
  KiB RSS, did not time out or exceed its 1 GiB limit, and left no survivors.
  The live hopper remained PID 3099092 and was neither stopped nor reconfigured.

## Equal-budget timings

| Workload | V1.1 | V1.2 | V1.2 / V1.1 |
|---|---:|---:|---:|
| Cold, start/white | 275.932 ms | 1,591.886 ms | 5.769x |
| Cold, start/black | 259.129 ms | 1,530.957 ms | 5.908x |
| Cold, Kiwipete/white | 314.438 ms | 1,993.325 ms | 6.339x |
| Cold, Kiwipete/black | 297.608 ms | 1,938.612 ms | 6.514x |
| Cold, asymmetric/white | 256.320 ms | 1,615.292 ms | 6.302x |
| Cold, asymmetric/black | 256.433 ms | 1,643.883 ms | 6.411x |
| Persistent four-prefix sequence | 500.749 ms | 3,249.564 ms | 6.489x |

All searches reached their requested node floor, returned a legal move, kept
one effective thread, restored the root position and evaluator stack, and
matched a fresh static evaluation after unwind. The separate V1.2 coverage gate
also passed every output bucket, every king bucket, and white/black mirror and
bucket-crossing king transitions.

## CPU attribution

The CPU profile contains 54.05 seconds of labeled V1.2 search. Two portable
kernels account for 83.90% of it:

| V1.2 operation | CPU seconds | Share of labeled V1.2 search |
|---|---:|---:|
| `applyUpdates` cumulative | 24.93 | 46.12% |
| `evaluateAccumulator` cumulative | 20.42 | 37.78% |
| `refreshPerspective` (inside updates) | 4.00 | 7.40% |
| accumulator copy at `PushMove` | 0.14 | 0.26% |

The source-line profile places 20.79 seconds flat in the portable nested update
loops and 12.69 seconds flat in `clippedSquaredWeighted`. The copy hypothesis is
falsified: the 3 KiB parent-to-child copy is measurable but immaterial.

## Transition incidence

The counter observer was compiled in a second, disposable build so it could not
perturb the CPU profile. Its patch is retained here and is explicitly not a
production change.

| Workload | Move pushes | Null pushes | 2-row | 3-row | 4-row | Perspective refreshes | Refresh / move |
|---|---:|---:|---:|---:|---:|---:|---:|
| Cold | 1,915,563 | 173,464 | 1,191,606 | 668,391 | 55,566 | 136,311 | 7.116% |
| Persistent | 626,799 | 63,556 | 465,092 | 156,373 | 5,334 | 69,344 | 11.063% |

The update-count partitions sum exactly to the move-push totals. Most pushes
therefore execute the regular 2-row or 3-row path for both perspectives; only
7.1% to 11.1% replace one perspective with a full refresh. This confirms that
specialized exact 2/3/4-row SIMD kernels target the dominant work, while a
refresh kernel is a later bounded optimization.

## Decision and implementation order

1. **V1:** add an amd64.v3 AVX2 V1.2 clipped-squared output dot product, with the
   portable implementation retained as the arithmetic oracle and fallback.
2. **V2:** add amd64.v3 AVX2 V1.2 2/3/4-row update kernels and dispatch the three
   observed transition shapes, again retaining the portable oracle/fallback.
3. **V3 only after re-profile:** vectorize king-perspective refresh if it remains
   material after V1 and V2. Do not spend work on accumulator copying.

V1 comes first even though total update time is slightly larger: the proven
V1.1 output kernel has identical clipped-square arithmetic and needs only a
768-lane trip count plus V1.2 bucket-selected weights. V2 then reuses the proven
V1.1 update-kernel design with the same 768-lane adjustment. After exactness,
oracle, race, and bounded performance gates, the optimized V1.2 network can be
strength-tested; the portable profile alone makes no Elo claim.

Machine-readable figures and artifact hashes are in `result.json`. The inert
probe is `rodent_v0_profile_test.go.txt`; `v12-transition-diagnostics.patch` is
the diagnostic-only observer.
