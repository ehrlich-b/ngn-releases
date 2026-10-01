# K4 AVX2 inference probe (2026-09-21)

## Scope

The K4 search panel had shown a 6.7x mean wall-time cost relative to HCE at
the same node budget. This probe changes only K4 inference: signed 16-bit
feature-row updates and the clipped-squared output dot use AVX2 when built for
`amd64.v3`. Other builds retain the portable Go implementation. The wider
`int64` output path is unchanged. The old and new engines load the same frozen
K4 network (SHA-256 `034559653a83a7e64d4407badff334f66e3eae6a3a1f33147e3516b3c88c3e69`).

## Correctness checks

- The portable and AVX2 kernels matched on 128 fixed-seed randomized trials,
  including extreme signed accumulator and weight values, for each output and
  feature-update operation.
- The full K4 package and focused engine K4 tests passed on the WSL host with
  `GOAMD64=v3`; the portable K4 tests passed on macOS. The scalar `int64` path
  remains covered by the existing K4 tests.
- On the frozen 32-position panel, the two engines produced identical best
  moves, depths, and node counts on every equal-node search. Both agreed with
  the teacher on 13/32 positions. This is a useful search-level parity check,
  not a proof for every possible position.

## Timings

WSL AMD Ryzen 7 9800X3D, one pinned CPU lane, nice 10, while the 5M label job
used two other lanes. The Go microbenchmarks used the same pinned K4 network:

| Benchmark | Prior engine | AVX2 engine | Speedup |
| --- | ---: | ---: | ---: |
| Incremental evaluation | 618 ns | 100.5 ns | 6.1x |
| Push/pop quiet move | 670 ns | 184.4 ns | 3.6x |
| Full accumulator refresh | 10,246 ns | 2,125 ns | 4.8x |

The search-level comparison reran both exact engines serially on the frozen
panel, rotating order between positions. The old engine was SHA-256
`3c3257bf14b43883ef7c88608fdb06eefe2379928119154ea6d38571c05e61b8`;
the AVX2 engine was SHA-256
`d17740084a5d7c96c3bb150ef55f8be3567779de33dddcb2faa95956458c475b`.
The [raw result](2026-09-21-owned-nnue-avx2-search-panel-result.json) is SHA-256
`ccfbe04b2d68848497b128543c761272eda000e9a9477c45c23bdcd1c8ae6b4c`.

| Frozen budget (32 positions) | Prior engine | AVX2 engine | Ratio |
| --- | ---: | ---: | ---: |
| 100k node cap: mean reported nodes | 67,389 | 67,389 | 1.00x |
| 100k node cap: mean wall time | 373.3 ms | 94.4 ms | 3.95x faster |
| 250 ms clock: mean reported nodes | 29,516 | 120,109 | 4.07x more |
| 250 ms clock: mean depth | 8.63 | 11.03 | +2.41 plies |
| 250 ms clock: teacher best-move agreement | 14/32 | 14/32 | unchanged |

The clock searches chose the same best move on 22/32 positions. That is expected
when the faster engine searches deeper. Teacher best-move agreement on 32
positions is too small a sample to infer playing-strength change. The prior
HCE search run took 50.9 ms at the same node budget, so optimized K4 is still
slower, and K4's model quality remains the larger open issue. The 5M unique
position probe is the next quality gate; 20M expansion remains held.

## Reproduction

`scripts/k4_speed_panel.py` pins the panel, prior result, UCI runner, baseline
engine, and K4 model by SHA-256. It refuses to overwrite a result and records
the optimized engine SHA-256, CPU affinity, and nice value. The panel SHA-256 is
`96de37a99f6c7649a1bf297329e49ce20490aa1c0c7c1e935620ce67ad92e50f`.
The runner SHA-256 is
`37effc57e156f90f3537f3784cd4e9c277c227c4521dedd9e730335f7ab0d300`.
