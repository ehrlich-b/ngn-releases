# Exact stalemate-probe optimization and replacement game gate

## Why this exists

The [first repair](2026-09-04-correctness-repair.md) proved seven defects but
lost its 400-game paired check: −32.2 [−55,−10]. It is not accepted as a release.
A 50-iteration CPU profile of the repaired search attributed 2.40s / 20.41s
(~12%) to full move generation at the non-check quiescence stalemate probe.
The new draw/repetition helper itself used 0.11s in qsearch. This supports
investigating the legality probe, rather than dropping correct terminal rules.

## Exact change

Before the complete stalemate test, look for a legal single-step pawn push.
An empty forward square is sufficient when the safe king and pawn source do
not share a rank or diagonal: moving the pawn cannot uncover a slider attack,
and a push along the king's file preserves any file block. Potential rank or
diagonal pins, and all in-check positions, use make/check/unmake. A failed pawn
probe always falls back to the full legal-move check. Promotions are included.
There is no approximate material threshold, removed draw rule, or new pruning.

This is node-identical to the first repair, not to the original engine.
The first implementation made/unmade every candidate push; Mac timings were
mixed, so it was not accepted. The refined version is the one tested below.

## Validation and measurements

- Full short suite, full race suite, and `go vet ./...`: PASS.
- Pawn cases: both colors, blocked pawns, rank/diagonal pins, a preserved file
  block, unrelated check, promotions, stalemate, and sole EP evasion: PASS.
- The independent Stockfish oracle now checks the pawn witness and exact state
  restoration as well: 2293 positions / 48926 root moves, under race: PASS.
- Native Windows, same 9800X3D box, engine process affinity mask 16 (one logical
  core), otherwise idle. Eight samples per binary/position, ABBA repeated four
  times; UCI go-depth clocks, fresh engine per sample. All samples agree exactly
  on cumulative nodes, seldepth, score, PV and bestmove between repair and fast.

| Position | Nodes | Median repair ms | Median fast ms | Speed gain |
|---|---:|---:|---:|---:|
| Kiwipete d12 | 295507 | 190.5 | 163 | 16.9% |
| Middlegame d12 | 112109 | 81.5 | 67 | 21.6% |
| Rook ending d16 | 858052 | 269.5 | 261.5 | 3.1% |

Timing does not establish Elo. Raw samples: `r0904fast_bench.json`, driver
`bench_fast.ps1`, and profile in `output/recovery-2026-09-04/`.

## Frozen artifacts

- Source: `b378c432cb34` plus `output/recovery-2026-09-04/fast.patch`, SHA-256
  `f6051e9ae2a74895d4e3f1b6ba4534edb1a1c4ac4d1432f828c200cba65f9ebf`.
- Windows candidate `ngn_20260904_fast.exe`, SHA-256
  `7b5af06a4a0ff02a0d9c51aa5eb041dd8ddf9935bbe4691d65dd76146a515828`.
- Linux candidate `ngn_20260904_fast_linux`, SHA-256
  `5f3b6078ba62afb86cc4f95ead318f64b5adf04c2fed8c2680fdc63c3ee423e0`.
- Compatible control and corrected SPRT harness unchanged; see the repair
  manifest for source and hashes. Go1.26.2, amd64/v3, `go build -o <file> .`.

## Game manifest (before launch)

Native Windows 9800X3D, **10+0.1, concurrency 8**, default scheduling/affinity,
`-lowpower=false`. The single-core benchmark affinity does NOT apply to games.
The same pinned 5000-line openings, SHA-256
`974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222`.

1. **`r0904fastaa`**, 200 games, new candidate vs itself. Require paired 95% CI
   covering zero, zero flags/errors, and both decisive/draw adjudications.
2. **`r0904fast`**, fixed 400 games, new candidate vs the compatible original
   control. Same acceptance as before: zero flags, illegal/missing moves,
   crashes, disconnects, or watchdog/voided games. A significantly negative
   paired interval requires further investigation. An inconclusive interval
   can support the proved correctness repair, but cannot establish an Elo gain.

No earlier games are pooled. Neither run stops on favorable mid-run signs.
Only after this replacement gate passes will the fresh 200-game absolute
[2600 check](2026-09-04-strength-check.md) run using this new frozen candidate.

```text
sprt_20260904.exe -new .\ngn_20260904_fast.exe -base .\ngn_20260904_fast.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 200 -mingames 201 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
sprt_20260904.exe -new .\ngn_20260904_fast.exe -base .\ngn_20260904_compat.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 400 -mingames 401 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
```

## Fresh A/A: PASS

`r0904fastaa` completed **200 games in 12m31s**, W/D/L **44/104/52**;
penta buckets **4/28/44/20/4**, **−13.9 [−44,+16]**, pLLR −0.35.
The interval covers zero. Zero flags, illegal/missing moves, crashes,
disconnects, watchdog/voided games. Adjudications: 96 decisive / 42 draw.
`DONE_EXIT_0`. The short A/A passes its rule; small bias remains unexcluded.

## Replacement comparison: PASS; optimized correctness repair accepted

`r0904fast` completed **400 games in 25m25s**, W/D/L **116/195/89**;
penta buckets **9/39/75/70/7**, **+23.5 [+1,+46]**, pLLR +1.10.
Zero flags, illegal/missing moves, crashes, disconnects, watchdog/voided games.
Adjudications: 205 decisive / 102 draw. `DONE_EXIT_0`.

The predeclared fixed-cap correctness/non-regression gate passes. The paired
interval is positive; the sequential SPRT boundary was not reached. This is
a short combined-repair result, not a per-fix Elo attribution or a large powered
heuristic certification. The earlier −32.2 result remains recorded separately.

Deployment will use the idle gap before the absolute gauntlet: commit the
accepted source, preserve old source/binaries, run WSL short/race tests, install
the exact frozen Linux/Windows artifacts, and verify UCI on the live files.
No deployment tests or builds will contend with the subsequent rating games.
The 2600 target remains unconfirmed until that independent gauntlet completes.

The frozen binaries were built before the source commit, so Go VCS metadata
reports `b378c43` with `vcs.modified=true`. The source patch and binary SHA-256s
above identify the tested release; that metadata alone does not make it stale.
