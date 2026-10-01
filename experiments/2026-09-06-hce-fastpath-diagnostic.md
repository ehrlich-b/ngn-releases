# HCE transition fast path: source accepted, performance unresolved

The fast path bypasses NNUE-only transition preparation/push/pop for HCE. Root reviewed the source, compiler lowering and correctness evidence. A first scratch design was rejected because recursive invocation depth can exceed physical NNUE move depth; v3 grows private NNUE scratch on that rare overflow and tests actual alpha-beta/quiescence re-entry. Full short/race/vet/build and corrected-B0 snapshots pass; focused optimized-NNUE tests also pass under GOAMD64=v3. The patch remains unintegrated.

Matched v3/trimpath binaries were compared with the unchanged readyok-barrier UCI sampler. Root verified all 256 final artifacts and recomputed zero normalized outcome mismatches over 700 samples per comparison. Both supervisors finished normally with no survivors.

The original timing gate required reported nodes to equal the requested node cap. That gate FAILED: ordinary UCI emits the last completed-depth node count, not the final visited count. The frozen plan and failed postcheck remain unchanged. An additive review downgrades the runs to wall-time/last-completed-result diagnostics; neither exact cap consumption nor ns/node is established.

| Probe | Fast path / exact 168385f parent median wall | Fast path / corrected B0 median wall |
|---|---:|---:|
| Kiwipete | 0.975 | 1.024 |
| Middlegame | 0.922 | 1.697 |
| Endgame | 0.986 | 1.084 |
| En passant | 1.003 | 1.231 |
| Promotion | 0.964 | 1.009 |

These are ratios of pooled role medians for each named fixture. The retained analysis also provides session, cycle, paired-row and reciprocal-order distributions; those metrics must not be conflated. The bypass has a modest fixture-dependent effect and has not repaired the full short-search regression. The middlegame wall gap is not proof that the recursive search itself became 70% slower: setup, pacing and background-runtime work still need separation.

The follow-up compared retained pre-M4c 4bd54add against exact 168385f with the unchanged sampler. Root independently verified 210 artifacts (109,138,731 bytes), 700 short rows and 42 larger middlegame rows, zero normalized outcome mismatches, and both actual wrapper/supervisor exits with no survivors. No debug formatting was added inside the timed interval.

| M4c boundary probe | 168385f / 4bd54add pooled median wall |
|---|---:|
| Short Kiwipete | 0.976 |
| Short middlegame | 0.986 |
| Short endgame | 0.964 |
| Short en passant | 1.022 |
| Short promotion | 1.003 |
| Middlegame, requested 524,288 nodes | 1.012 |

The larger probe has seven paired-session median ratios with median 0.996 and range 0.970–1.014. These observations do not reproduce the earlier large short middlegame gap at the M4c boundary and provide no basis for an SMP rollback. They do not resolve the earlier B0-to-current difference or establish a playing-strength gain. The inherited raw label accepted-b0 refers to the exact 4bd54add binary in these two manifests, not historical B0.

Full follow-up artifact manifest SHA 755b785e4a9638d0a348cf16f5acebc620192d9f95875bf0af8fea3cd482e236. Analysis SHA 0c10e2159ff266b4d92f21f4fca42a2114897f443b949de33817b6ad66461f3a. Root receipt output/hce-fastpath-root-review-20260906/m4c-diagnostic-v1.json SHA c882205723d3ab214e7f235234a9d6a721c5ce87af404e3ceb84215e7c3ed3d4.

Evidence root: output/hce-transition-fastpath-timing-20260906. Full 256-file manifest SHA c44a5b67ea5ad645bf7bca89bcd1447d77f79c159210d3464970dc251c7a49a1. Observability amendment SHA be0e609155a7f792db94ef7a8f29f257e91c079195c8721f69929c93f105d462. Final distribution analysis SHA 5308935fbdaefa248f1310397bac6b9c549df6292dd78f75f7afe26f1bf53044. Root review: output/hce-fastpath-root-review-20260906/diagnostic-v1.json. No deployment or playing-strength gain is implied.

## Direct public-search comparison, B0 to 4bd

The test-only v2 harness is accepted after root source/build review, 20 correctness rows and the single authorized ABBA14 timing run. It captures nonempty legal completed-iteration PVs and actual final SearchInfo.Nodes. The failed exact-cap preflight remains preserved: actual final counts can overshoot the requested cap by up to two nodes, identically on both revisions. Empty-PV v1 evidence is not used for the completed-PV claim.

All 56 worker sessions and 140 searches completed with 112 measured rows, exact normalized move/score/PV/node trajectories, restored roots, empty stderr, initialization-only crash logs, actual outer exit zero and no monitor violations/survivors. Root verified 249 final artifacts totaling 395,137 bytes, in addition to the separately verified source and binary packet.

Using seven adjacent pairs of session medians (4bd/B0), ratios were 1.0203 cold and 1.0339 warm at requested 16,384 nodes; 1.0232 cold and 1.0272 warm at requested 524,288 nodes. At the larger budget, ranges were 1.0063–1.0452 cold and 1.0103–1.0479 warm. The retained analysis also reports individual warm row-pair ratios; those repeated observations are dependent and have a different median from the seven session pairs.

This one-position public-call measurement includes admission, root refresh, TT-PV reconstruction and callback copy, while excluding UCI command pacing, construction, result formatting and replay. It finds a small direct gap, far below some earlier short-UCI ratios; it is not a general speed claim or evidence that recursion alone slowed. A source-only proposal to equalize post-bestmove timing outside the next measured UCI search remains held. No production optimization or rollback follows from this diagnostic alone.

Root receipt: output/hce-direct-diagnosis-20260906/root-timing-review-v2.json, SHA df4a97fdd967b9f9f7bf6bc235ff269e490be6842e42f60f8339c705a172321b.
