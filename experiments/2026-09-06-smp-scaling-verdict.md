# First HCE multicore scaling result

The real Lazy SMP implementation at 168385f passed the frozen 1/2/4/8-thread diagnostic. Root independently verified 92 artifacts and recomputed all 128 raw samples and six aggregate NPS ratios. Eight threads used about 7.82 physical cores and searched 6.41x (cold) / 6.69x (warm) the nodes per second of one thread. This supports an equal-wall playing test; it does not establish an Elo gain.

| Threads | Cold aggregate NPS / 1 thread | Warm aggregate NPS / 1 thread | Median primary depth gain cold / warm |
|---|---:|---:|---:|
| 2 | 1.779 | 1.877 | 0 / 0 |
| 4 | 3.458 | 3.567 | +1 / +1 |
| 8 | 6.405 | 6.688 | +2 / +1 |

The eight-thread primary search completed deeper than one thread in 14/16 cold probes (2 ties), and 10/16 warm probes (3 ties, 3 shallower). Additional aggregate nodes include helper work and do not mean that the primary searched an equivalent serial tree. Two threads produced more work without a median depth improvement in this sample.

The run used eight fixed FENs, a cold then warm 1,900 ms search for each, total Hash 128 per role independent of width, GOMAXPROCS equal to Threads, and physical CPU masks. Rotation A ran widths 1,2,4,8 from low-numbered cores; rotation B reversed width and fixture order using high-numbered cores. All eight row supervisors finished once with exit0, empty stdout/stderr, and no surviving descendants. Every best move and PV was legal, every root restored, every search joined, and configured/effective widths agreed. Maximum measured wall overshoot above allocation was 0.335 ms. Peak sampled process-tree RSS was about 487 MiB.

The accepted analyzer reports descriptive one-window fixed-probe bootstrap intervals, resampling eight named fixtures while preserving both rotations. They are not independent-machine, population-strength, or Elo confidence intervals. Eight-thread NPS intervals were [6.210,6.564] cold and [6.500,6.841] warm.

Evidence:
- Run: /home/ehrli/repos/ngn-smp-measure/output/smp-measure-m4c-20260906/hce-matrix-v1
- Time: 2026-09-06T08:50:55.937554Z through 08:55:02.381256856Z; other project compute held.
- Summary SHA: f85ec67470a22c60cf74c23419e262a5ee0b6b62f288b9d4824b43077aece2cd
- Final artifact manifest SHA: 066ad50952724765750790c33c35cfa066a487f7ff1a0dafd262566560befceb
- Root review: output/smp-matrix-root-review-20260906/receipt.json, SHA 814bf82fddff2f9858e70b569929e9ef6caed14b5d23d70ef1d2a40fcd8fb31e

Next is the narrowly extended HCE M4c match-runner profile: four-game 8v8 A/A integrity gate, then separately frozen 8v1 paired real-clock test. The runner must inherit all eight physical CPUs with concurrency 1 and omit fastchess's per-game single-CPU affinity. Exact per-go effective-width receipts are required. No game test or deployment is implied by this diagnostic.
