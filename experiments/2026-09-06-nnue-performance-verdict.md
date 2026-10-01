# Initial optimized NNUE performance verdict

These are WSL timing diagnostics on source 4bd54add with the accepted 100k-data pilot network abd956983274bae1d91b522cd7ca3a1b2fa91ef3e69051ccb414b9adf0a5c67e. They establish a runtime improvement over the retained scalar reference; no playing-strength improvement is inferred.

The direct bounded output dot measured a median 121.15 ns portable versus 13.90 ns AVX2, an 8.716x speedup in the same amd64.v3 test binary. Context evaluation across six identical positions measured 8.45–8.82x speedup. Both paths allocated zero bytes per evaluation.

| Position | Cold whole-search speedup | Warm whole-search speedup |
|---|---:|---:|
| Start position | 1.651x | 1.517x |
| Kiwipete | 1.769x | 1.658x |
| Middlegame | 1.689x | 1.558x |
| Pawn endgame | 1.938x | 1.657x |
| En passant | 1.873x | 1.573x |
| Promotion | 1.885x | 1.612x |

Whole-search comparisons force the scalar reference or select production Int32Context in one binary, with the same model and exact search outcomes. Both evaluator contexts are prepared outside timing. Cold includes the common first-search TT allocation; warm repeats the fixed root with retained TT/history. Node counts are identical for every compared fixture/lifetime. Only the direct portable-dot/AVX2 comparison isolates the vector kernel; whole-search gains also include int32 accumulator/storage effects.

The declared reciprocal ABBA order produced 28 direct-dot rows, 168 context rows and 336 whole-search rows. Each reported mode has 14 observations across reciprocal positions and seven repetitions. The numbers above are descriptive within-run medians; timing variation and repeated-sample dependence remain visible in the retained raw data and per-mode ranges. They are not an independent-machine confidence claim.

All three bounded stages completed with zero exit status, empty stderr, no timeout or termination and no surviving descendants. Engines/test binaries used CPU4 and GOMAXPROCS1; monitoring used CPU6. CPU4–7 were idle before/after. Source/network/capability and iteration counts were frozen before execution. Root verified all 36 timing-artifact hashes and independently reparsed every raw row, checking exact iteration counts, zero inference allocations and equal whole-search node counts.

Evidence root: /home/ehrli/repos/ngn-nnue-perf-harness-v2/output/nnue-performance-harness-20260906/go-timing-v1. Timing receipt SHA fa5e4b786a50450ccab30e9b029fab0554163f3f0f99b0fc11fd8180f9422ddd. Artifact list SHA3d8baa82ef0b1b6a881b7a0cc08febc208fbab9cede8a4096ce1867692414997. Root calculation: root-recalculation-v1.json, preserving medians and full min/max ranges.

External B0/current HCE timing is a separate pending check for evaluator-plumbing overhead. Profiling is separate from unprofiled speed comparisons. Real-clock NNUE playing, multicore scaling and final accepted-release comparison remain required.

## Corrected HCE wall-time comparison

The v5 external sampler inserts position/isready/readyok before each timed go. Root verified all 103 final artifacts and independently reproduced all 700 measured outcomes (650 cross-session comparisons, zero differences). All 910 warmup/measured searches crossed the barrier.

Current/B0 median wall-time ratios were kiwipete 1.0404, middlegame 1.3248, endgame 1.1531, en-passant 1.2555, promotion 1.0138. Values above 1 are slower. These short fixed-node, warm-session results compare entire revisions in one WSL window; they do not isolate N3 overhead or establish an Elo change. The slowdown is material enough to prioritize CPU profiling.

The earlier external-v2 ratios are preserved as UCI pipeline diagnostics: B0 sleeps 10 ms after launching search, delaying consumption of the next command when the prior search finishes early. Those values must not be cited as HCE search speedups. The direct Go NNUE benchmarks are unaffected.

Evidence: /home/ehrli/repos/ngn-nnue-perf-harness-v2/output/nnue-performance-harness-20260906/external-hce-timing-v3; root receipt output/nnue-performance-20260906/root-external-hce-v3-review.json.
