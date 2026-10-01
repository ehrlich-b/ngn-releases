# Rodent V1.2 post-V3 profile: stop backend kernel work

The fresh profile of accepted V3 shows that the three exact AVX2 tickets have
removed the concentrated evaluator bottleneck. King-perspective refresh fell
from 23.44% flat CPU after V2 to **1.58%** in its AVX2 body (**2.44%** cumulative
through the wrapper). No evaluator function now exceeds 10% flat CPU. The next
step is G0b/E0, the fair V1.2-versus-optimized-Anand strength gate, not another
backend microkernel.

## Pinned workload

- Source: accepted V3 commit `4d42639`.
- Linux/amd64.v3, Go 1.25.5, Ryzen 7 9800X3D, `GOMAXPROCS=1`, engine Threads 1,
  Hash 128 MiB.
- Six fixed positions, 400,000 nodes/search, three benchmark repetitions, with
  setup excluded and CPU labels `scope=search`.
- Released V1.2 model: 4,744,768 bytes, SHA-256
  `c35a1abc1b8c1cb1d5f4221454d494c1a6da1ed9088fd51ab27038bfa74b5053`.
- Profiled binary SHA-256:
  `665f76e7891e4f735160021fda2a1a819882e946ffe6bfcba57fa8fd3fce6fe4`.
- CPU profile: `/home/ehrli/post-v3-profile-20260919-001/cpu.pprof`, SHA-256
  `3d5719788b4e64daaca4912092963b1aaa4403f19f736f4a08a7255e8969e8b8`.

All 18 searches reached the node floor and returned normally. The profile has
6.98 seconds of labeled search samples.

## Cost distribution

| Operation | Flat CPU | Share | Cumulative share |
|---|---:|---:|---:|
| `engine.selectNextMove` | 0.64 s | 9.17% | 9.31% |
| `CachedEval.LoadKey` | 0.62 s | 8.88% | 8.88% |
| V1.2 output AVX2 | 0.60 s | 8.60% | 8.60% |
| `runtime.duffcopy` (all callers) | 0.48 s | 6.88% | 6.88% |
| `alphaBetaPV` self time | 0.45 s | 6.45% | 97.56% |
| V1.2 two-row update AVX2 | 0.20 s | 2.87% | 2.87% |
| V1.2 three-row update AVX2 | 0.16 s | 2.29% | 2.29% |
| V1.2 refresh AVX2 | 0.11 s | 1.58% | 1.58% |

The wrapper-level evaluator totals are also bounded: `applyUpdates` is 10.46%
cumulative, `evaluateAccumulator` is 9.89%, and `refreshPerspective` is 2.44%.
The `SearchContext.PushMove` source-line profile isolates the 3 KiB accumulator
copy at 0.14 seconds, **2.01%** of labeled CPU; the larger aggregate
`runtime.duffcopy` row therefore cannot be attributed to that copy alone.
Transition derivation is 5.01% cumulative and validation fragments are smaller.

## Decision

The former scalar backend had two dominant loops and one justified refresh
tail. Those have been addressed. Output remains visible because it is called
frequently, but its body is already exact AVX2 and no new portable-ISA design is
supported by this profile. Update, refresh, copy, validation or transition
micro-optimizations individually offer only small upper bounds and carry
increasing correctness risk.

Stop backend kernel work before it turns into polishing. The independently
accepted V1/V2/V3 ratios imply a 5.70x speedup over the original portable V1.2
backend; this profile supplies the planned stopping evidence. Complete the
narrow harness admission and run E0 under the frozen same-search/equal-clock
protocol. Only game evidence can decide whether V1.2 is the stronger evaluator.
