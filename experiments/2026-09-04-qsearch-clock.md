# Qsearch clock regression repair

**Superseded by the [combined correctness repair](2026-09-04-correctness-repair.md).**
The clock-only candidate match was never launched. The old-harness A/A completed
200 games (61/93/46, penta +26.1 [-8,+61], zero flags), but subsequent discovery
of move-validation and repetition defects required a freshly validated harness.

The WSL checkout was clean at `7587eb6` (early June); the Mac source is
`b378c432cb34`. The native Windows mill was idle. T22 had completed, with
2108/3768/2124 W/D/L, penta -0.7 [-6,+4], 0/0 flag-outs over 8000 games:
it is shelved under its original rule, not incorporated here.

`zzz_pc_test.go` was an orphaned temporary probe referring to deleted globals.
It prevented even `go test -short` from compiling. Removed; its measurements
remain in the T22 run record and its source remains in git history.

## Mechanism and predeclared checks

`quiescenceWithDepth` honored external stop and node limits but never called
the time manager. Recursive captures/check evasions could therefore overrun
the deadline until control returned to alpha-beta. The regression sets an
expired clock and its next scheduled check: before the fix qsearch returned
89 with `Stopped=false`; after it returns the stop sentinel without a node.
Depth 0 checks the hard tournament deadline, without an iteration soft stop.

This is a correctness repair, not an Elo heuristic. Required: the regression,
full short/race suite, exact fixed-depth node/score/move identity, and a short
real-clock non-regression check. Games cannot establish a small Elo gain.

## Preflight and game manifest (written before launch)

- Source base: `b378c432cb34`, with only the orphan probe removed.
- Candidate: base plus qsearch time check in `engine/search.go`.
- Toolchain: go1.26.2 darwin/arm64 cross-compile, GOOS=windows,
  GOARCH=amd64, GOAMD64=v3; `go build -o <binary> .`.
- Base SHA-256: `8dd1f8a16bf3ae0e942c46e37ca9ca3dc95d7dcc389b5073110f407e1ce3a88d`.
- Candidate SHA-256: `c1f855c690529fb105fd99038a234eafc03b88c7a67ccb94ea8fc8c7e8042775`.
- Harness: unchanged validated `sprt.exe`, SHA-256
  `30c33e0512725b7f552d8a1cf72ba6f1e0deb4dcb122c6b6ba8f6433c686a762`.
- Machine: 9800X3D, 8c/16t, native Windows, `192.168.4.108`.
- Openings: `sprt_openings.txt`, 5000 lines, SHA-256
  `974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222`.
- TC 10+0.1, concurrency 8, default OS scheduling (no custom affinity).
- A/A: 200 games, same rebuilt base on both sides, refresh of the July
  1600-game validated mill preflight. Require 95% penta CI covering zero,
  no errors/flag-outs and both adjudication types. This short refresh cannot
  rule out a small bias; the larger original A/A remains supporting evidence.
- Candidate: 400 games, fixed cap (mingames 401 disables early stopping).
  Require no errors/flag-outs and no statistically significant negative
  penta result. An inconclusive small difference supports only non-regression,
  never an Elo-gain claim. Investigate any failure before shipping.

Commands (launched via `scripts/boxsprt.sh`):

```text
sprt.exe -new .\ngn_20260904_base.exe -base .\ngn_20260904_base.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 200 -mingames 201 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
sprt.exe -new .\ngn_20260904_qtime.exe -base .\ngn_20260904_base.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 400 -mingames 401 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
```

Poll names: `r0904aa`, then `r0904qtime`. Logs fetched to
`output/recovery-2026-09-04/`. Results to be appended after completion.
