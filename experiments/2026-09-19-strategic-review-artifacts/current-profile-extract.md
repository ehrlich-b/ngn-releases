# Existing optimized-Anand profile used by the review

This is a read-only extraction from the already completed September 19 profile,
not a new benchmark. Profile provenance and the associated rejected TT candidate
are in [the original record](../2026-09-19-smp-tt-read-publication.md).

The current review read the **one-thread base** CPU profile, not the rejected
publication candidate's profile. Six one-second fixture searches are a
prioritization sample, not a general playing workload or a speed comparison.

```text
File: engine-profile.test
Build ID: 1a4bc6426dddc67d9bc2d99ccaeb870f5a611812
Type: cpu
Time: 2026-09-19 06:45:52 EDT
Duration: 7.12s, Total samples = 6.99s (98.21%)
```

Selected entries from the saved profile:

| Function | Flat CPU | Cumulative CPU |
|---|---:|---:|
| engine.selectNextMove | 11.59% | 11.59% |
| engine.CachedEval.LoadKey | 9.73% | 9.73% |
| engine.Cache.Get | 0.43% | 10.73% |
| engine.workerHistory.scoreMovesIntoBuffer | 4.15% | 5.15% |
| engine.workerEvaluator.PushMove | 0.29% | 26.18% |
| rodenteval.SearchContext.PushMove | 0.86% | 21.60% |
| rodenteval.Model.applyUpdates | 0.86% | 12.59% |
| rodenteval.Model.refreshPerspective | 5.01% | 5.15% |
| rodenteval.Model.evaluateAccumulator | 0.43% | 7.15% |
| rodenteval.rodentOutputDotAVX2 | 6.72% | 6.72% |
| runtime.duffcopy | 7.15% | 7.15% |
| engine.Position.makeMoveHelper | 0.72% | 3.29% |
| engine.hasLegalPawnPush | 0.43% | 0.57% |

Nested cumulative entries overlap: `applyUpdates` includes refresh work and is
itself inside `SearchContext.PushMove`. Do not add these percentages. The
`duffcopy` share has multiple callers and is not all NNUE accumulator copying.
`LoadKey` samples do not establish a mutex problem in the direct one-worker
table. Removing all move selection would cap that isolated thought experiment
near `1/(1-.1159) = 1.13x`; a real replacement still costs time and can change
the search tree.

The successful extraction command completed in 0.473 seconds (remote command
time), with no engine, build or hopper operation:

```sh
ssh ehrli@192.168.4.108 'wsl -d Ubuntu -- /usr/local/go/bin/go tool pprof -top -nodecount=65 /home/ehrli/repos/ngn-tt-profile-20260919/output/tt-contention-profile-20260919/engine-profile.test /home/ehrli/repos/ngn-tt-profile-20260919/output/tt-contention-profile-20260919/w1.cpu.pb.gz'
```

Two earlier command-form attempts failed before analysis: nested Windows/bash
quoting, then an unqualified `go` absent from the noninteractive PATH. Neither
launched an engine or changed state. No raw profile or binary is copied into
the repository.
