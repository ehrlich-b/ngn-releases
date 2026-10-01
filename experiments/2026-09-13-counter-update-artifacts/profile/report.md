# Counter post-output profile evidence (2026-09-13)

## Decision

Admit exactly one identity-preserving candidate: fuse the destination-frame copy
and all ordered feature-row updates in `SearchContext.PushMove` into one
per-lane pass.  Preserve update order and float32 add/sub operations exactly;
only remove intermediate destination loads/stores.  Do not combine this with a
transition-validation or output-dot change.

This is an admission to the already frozen correctness/performance gate, not a
performance claim.  Stop after that gate: adopt only if both cold and persistent
families independently meet their predeclared whole-search thresholds; otherwise
shelve with no result-conditioned repeat or variant shopping.

## Frozen identity and inputs

- Accepted base commit: `9ef265664499bc79fa129bf75adb7aaa52b7e5c6`
- Go: `go1.25.5 linux/amd64`; builds use `CGO_ENABLED=0 GOAMD64=v3 GOMAXPROCS=2`, `go test -p 2 -c`.
- Search execution: CPU 4, `GOMAXPROCS=1 GOGC=100 GOMEMLIMIT=off GODEBUG=`, Hash 128 MiB, one thread, 400,000 minimum nodes/search.
- Model SHA-256: `3488baed71f4d432d028e05b65e2ff13f8c9afe7f91974d1a3c34a0691d6670c`.
- Accepted Counter1 PGN SHA-256: `35bdd00b28f585224e6c04392bdbf79cd4472c9ea7549b84acfc4379b07847d2`.
- Cold six-fixture probe SHA-256: `8623a3f8d486f9fcd3231b835469890ece52e79a9cabcf5bb6d5562d664dea4c`.
- Persistent probe v2 SHA-256: `7883232be0870b80531d5a2f7532e5355ab207e61af42a05f3f7c042002fb308`.
- Preserved persistent probe v1 SHA-256: `3f1b89bf3151843eb75af577248b756afafc4d4d775ddb8c41ead262a16f580d`.
- Prefix JSON SHA-256: `067e3cc3bb62b5d544d0bf0ce9d18c2de7c94e46bf721e443b53f9a64b47303e`.
- Prefix extractor SHA-256: `a1a6a242c151aa18ab20d48cb85841765f61475745ba9a049dfb1a75b60f3272`.
- Process monitor SHA-256: `4771585ee567615467cbf5eb3255a78dd234efe6f8fc9fe02885bb9815a7209f`.

The persistent corpus is the first three games in the immutable accepted PGN,
without result or position selection.  Each has at least 48 plies.  Full legal
history is replayed; roots are selected after fixed plies 23 and 48.  One engine
is created per game/benchmark operation and retained across the two searches,
including TT, history and evaluator state.  Replay, root restoration and exact
state checks are outside the Go benchmark timer.  The six-root snapshot is SHA
`8eb00369a0f89d8a9e60bd0116beb70115905aec3adeb89510eec80651c71591`.

## Valid profiles

Cold-v2 completed in 22.03 s with return code 0, empty stderr, no survivors and
317,440 KiB peak sampled tree RSS.  Receipt SHA-256 is
`e1eca67e3c191d9eb78368dd06f65654eacdaab5a456c571c5e249adbbd8fe41`.
Its six 10x rows range from 303.83 to 353.36 ms/search.  The unfiltered 21.73 s
CPU sample gives 260 ms on the accumulator-copy source line plus 1.26 s
cumulative in `applyFeatureUpdates`: 1.52/21.73 = 7.00% of total samples.
Within that update path, AVX2 add/sub rows are 1.22 s flat.  This profile has no
scope labels, so its denominator honestly includes setup as well as search.

Persistent-v2 completed in 20.79 s with return code 0, empty stderr, no
survivors and 318,028 KiB peak sampled tree RSS.  Receipt SHA-256 is
`9837a3154f4155588d47bc9fad551fe0b9a9e604098c7b33207e72f59aecd48e`.
All rows used 10 operations, two searches and at least 800,000 total nodes/op:

| row | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| game_1 | 579,427,816 | 2,397,913 | 26 |
| game_2 | 606,569,636 | 2,399,142 | 27 |
| game_3 | 643,805,764 | 2,402,836 | 27 |

Search labels cover 20.09/20.46 s = 98.19% of total profile samples.  Percentages
below retain the full 20.46 s denominator rather than renormalizing the filtered
profile.  Search-only attribution gives 160 ms on the accumulator-copy line plus
910 ms cumulative in `applyFeatureUpdates`: 1.07/20.46 = 5.23% of all samples.
The add/sub AVX2 rows are 860 ms flat (4.20%) within that path.  Unlabelled
setup/replay accounts for 370 ms (1.81%) and is reported separately.

The copy and update source lines are sequential and non-overlapping.  The
5.23%/7.00% figures are affected-path shares, not projected savings: the fused
kernel retains every arithmetic operation.  They nevertheless provide a
specific, independently repeated mechanism with enough headroom to justify one
candidate against a 3% whole-search gate.

## Allocation evidence

Persistent timer-scoped rows allocate about 2.40 MB and 26--27 objects per two
searches.  Cold timer-scoped rows allocate about 135.42 MB and 10 objects per
search because each iteration includes the lazily materialized 128 MiB TT.
The raw persistent allocation profile totals 4,713.93 MB: 89.61% is
`newCacheWithMode`, 8.24% `NewSearchEngineWithHash`, and 1.55%
`searchIterativeDeepeningWorkerUnsafe`.  Raw heap profiles are setup-dominated
and do not carry the CPU scope labels; they must not be interpreted as search
CPU or added to CPU percentages.  The candidate gate requires identical
timer-scoped allocations rather than claiming this optimization removes heap
allocation.

## Excluded operational failures

- `cold-v1` is invalid and excluded.  Windows command processing stripped the
  caret from `-test.run=^$`, actual argv became `-test.run=$`, unrelated tests
  ran, and the owned process group was terminated.  Receipt SHA-256
  `84b1bdb0a039b3edc8e3562c93fa79bdd94da71c52bdf5b04c0083c1c756ed8f`;
  stdout SHA-256
  `ac5255e7c7c80e347ca86877b79e728f35eec55492a9570ee213626f281578df`.
- `persistent-v1` is invalid and excluded.  A nil callback intentionally avoids
  reporting-only `refreshSearchReport`, so the interrupted raw result can have
  an empty PV even with a legal best move.  Its benchmark-only assertion was
  changed to legal-best-move validation; the strict callback snapshot PV check
  remained unchanged.  Receipt SHA-256
  `56d1ae25b06cd8d6a865ab3c503e39e09318e25c8e7e18eccc48041c97becd1f`;
  stdout SHA-256
  `b0e3245baedac90c2b5c9c15aa8da40b6547955b7716c59c0016f2925e78dd80`.

No timing result influenced either repair, and neither failed attempt supplies
profile or benchmark evidence.

## Raw evidence and extraction commands

Raw profiles remain in WSL and are not copied into the compact archive:

- Cold CPU: `output/profile/cold-v2/cpu.pb.gz`, SHA-256 `bfe16d91735762c23ca2280a6b7ac37da10a8be976eaa0835c7ebabe073a4bee`
- Cold alloc: `output/profile/cold-v2/alloc.pb.gz`, SHA-256 `34b8ef7853ca001f71beb7fe831f77e53f64d0c30d8d76c7e3e389d57745cca9`
- Persistent CPU: `output/profile/persistent-v2/cpu.pb.gz`, SHA-256 `a0363f29fd42b7011fecbe5f060f026d565505bd3760713c22c41450627c33f8`
- Persistent alloc: `output/profile/persistent-v2/alloc.pb.gz`, SHA-256 `10a5c576c1040b25493ea5b39363d156312eb76655815aff776fbfbd1b484153`

From the worktree root, the principal extraction commands are:

```sh
go tool pprof -top output/build/engine-profile-v2.test output/profile/persistent-v2/cpu.pb.gz
go tool pprof -top -tagfocus=scope=search output/build/engine-profile-v2.test output/profile/persistent-v2/cpu.pb.gz
go tool pprof -top -tagignore=scope=search output/build/engine-profile-v2.test output/profile/persistent-v2/cpu.pb.gz
go tool pprof -tree -nodefraction=0 -focus=runtime.duffcopy output/build/engine-profile-v2.test output/profile/persistent-v2/cpu.pb.gz
go tool pprof -source_path=/home/ehrli -list=PushMove output/build/engine-profile-v2.test output/profile/persistent-v2/cpu.pb.gz
go tool pprof -source_path=/home/ehrli -list=applyFeatureUpdates output/build/engine-profile-v2.test output/profile/persistent-v2/cpu.pb.gz
go tool pprof -top -alloc_space output/build/engine-profile-v2.test output/profile/persistent-v2/alloc.pb.gz
```

The exact executed argv and clean environment are recorded in each supervisor
receipt.  Valid runners use the shell-safe impossible selector
`-test.run=CounterProfileNoTestsSelected`.
