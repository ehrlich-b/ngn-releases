# Search, UCI, and match-rule correctness repair

## Proved failures

1. `zzz_pc_test.go` referred to deleted experiment globals: the checked-in
   short test suite could not compile. Removed the orphan probe.
2. Quiescence ignored the time manager. An expired-clock regression returned
   score 89 with `Stopped=false`. It now observes the hard deadline and unwinds.
3. Quiescence skipped dead material, fifty-move and repetition draws; stalemate
   was checked only at qDepth 0. Repros scored a fifty-move rook ending +348,
   a stalemate at qDepth 1/6 -1111, and a third occurrence +544. Shared draw
   detection and a balanced qsearch repetition stack now cover those paths.
4. Root and alpha-beta draw shortcuts hid checkmate at the fifty-move boundary.
   Both root drivers chose Ra1 (0) over Ra8# (29999) at HMC 99. They now find
   mate. Checkmate terminates play before a draw claim: [FIDE rules, 5.1.1 and
   9.3](https://handbook.fide.com/chapter/E012023).
5. The slice move generator checked rank 2 for the white pawn captured by
   black en passant, instead of rank 4. The search's buffer generator found
   the move, but UCI output validation discarded it. An only-evasion regression
   covers this through UCI; the match harness also recognizes that evasion.
6. `ParseUCIMove` decoded coordinates without validating legality. The external
   match harness accepted blocked rook jumps, wrong-color moves, empty sources,
   impossible pawn moves, and invalid promotion suffixes. It now resolves the
   exact move in the legal list; a scripted match proves illegal moves forfeit.
7. The harness incremented the initial repetition count after `ParseFEN` had
   already seeded it. One knight round-trip was adjudicated threefold at ply 4.
   Scripted matches now continue at the second occurrence and draw at ply 8.

The existing Texel quiet-position guard was retained: terminal draws remain
quiet even when the raw eval contains tempo/PST terms and differs from zero.

Independent audit: Stockfish depth-2 divides over **2293 positions / 48926
root moves**, deterministic seed 2600, covering castling, promotions, en passant
and pinned en passant. The audit found bug 5 and passed after its repair. Every
move also checks incremental board/hash/check-tag state against FEN reconstruction
and exact make/unmake restoration. Re-run:

```sh
NGN_MOVEGEN_ORACLE=/path/to/stockfish go test ./engine -run TestMovegenOracle -v
```

## Scope and acceptance (declared before the new harness run)

This is an explicitly combined correctness integration experiment: legal move
generation feeds UCI output and authoritative match adjudication, while the
quiescence repairs change how the newly available moves terminate and unwind.
Each defect has a direct regression. The combined games check integration and
gross non-regression; they do **not** attribute marginal Elo to individual fixes.
No tuning vector, pruning heuristic, or eval coefficient is changed.

The originally declared clock-only match is superseded, **never launched**.
Its old-harness A/A completed 200 games, 61/93/46 W/D/L, penta +26.1 [-8,+61],
0/0 flags, 107 decisive/52 draw adjudications. This passes that short refresh
rule but cannot validate the subsequently repaired harness. It is not pooled
with the new preflight or candidate games.

Acceptance requires direct regressions, full short and race suites, independent
move audit, a fresh same-binary A/A on the repaired harness, and a completed
400-game candidate-vs-immediate-base check. Any illegal move, crash, no-move,
watchdog/voided game or flag-out halts acceptance. A statistically significant
negative penta result requires investigation. An inconclusive result supports
only the correctness repair's non-regression check, not an Elo improvement claim.

## Manifest

- Base source: `b378c432cb34` (certified T12+T19 engine, before these repairs).
- Source patch: `output/recovery-2026-09-04/repair.patch`, SHA-256
  `e35836336f921d39632418d729db4939b8651c0a8be4a64d53293a8bfc220ee6`.
- Base Windows binary: `ngn_20260904_base.exe`, SHA-256
  `8dd1f8a16bf3ae0e942c46e37ca9ca3dc95d7dcc389b5073110f407e1ce3a88d`.
- Candidate Windows binary: `ngn_20260904_repair.exe`, SHA-256
  `6c9af9928d6c24c5ba6a798d424d5f998e04d837c77de8654daf6924ca99e6e9`.
- Corrected harness: `sprt_20260904.exe`, SHA-256
  `63ef86d9b986fdb2c260782149af259e278845d89595d010df3970e6698cc9ed`.
  Original `sprt.exe` preserved; the new harness is separately named.
- Toolchain: go1.26.2 darwin/arm64, GOOS=windows GOARCH=amd64 GOAMD64=v3.
  Commands: `go build -o <engine> .`, `go build -o <harness> ./cmd/sprt`.
- Machine: AMD 9800X3D, native Windows, LAN `192.168.4.108`; concurrency 8,
  default OS scheduling/no custom affinity; TC 10+0.1 seconds.
- Openings: `sprt_openings.txt`, 5000 lines, SHA-256
  `974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222`.
- Fresh A/A: **200 games**, candidate against itself. Require penta 95% CI
  covering 0, clean errors/clocks, both adjudication kinds. This short check
  cannot exclude a small scheduling bias; report its uncertainty.
- Candidate: **400 games**, candidate vs rebuilt immediate base, fixed cap.
- Poll names: `r0904repairaa`, then `r0904repair`.

```text
sprt_20260904.exe -new .\ngn_20260904_repair.exe -base .\ngn_20260904_repair.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 200 -mingames 201 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
sprt_20260904.exe -new .\ngn_20260904_repair.exe -base .\ngn_20260904_base.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 400 -mingames 401 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
```

Raw logs: `output/recovery-2026-09-04/`. Results appended after completion.
The historical 2704 [2665,2743] rating is not a new rating for this patch.

## Local validation

- Full short suite and full race suite pass; `go vet ./...` passes.
- The independent audit also compares encoded legal moves from the search's
  buffer generator against the slice generator. Both paths match all 2293
  oracle positions; the audit passes under the race detector.
- Reference nodes: kiwipete d12 295507 and middlegame d12 112109 are unchanged;
  rook ending d16 changes 667703 -> 858052 as draw handling changes. This is
  a tree-shape observation, not evidence of strength.
- End-to-end UCI, depth 1: on the only-en-passant-evasion FEN, the old binary's
  PV contains e4d3 but final output is `bestmove (none)`; the repair emits
  `bestmove e4d3`. At the fifty-move mate FEN, old output is `bestmove a7a1`,
  score 0; repaired output is `bestmove a7a8`, score mate 1.
- Transcripts: `black_ep_only_evasion_{base,repair}.txt` and
  `mate_at_fifty_{base,repair}.txt` in the run-log directory.
- Overhead measurement on the Mac E-cores, six samples per binary/position in
  alternating ABBA order: kiwipete median 571 -> 632.5 ms (+10.8%); middlegame
  213 -> 224 ms (+5.2%), with identical node counts. Added terminal checks have
  a measurable cost. The game check, not these timings, decides gross strength
  non-regression. Raw samples: `matched-tree-speed.json` in the run directory.

## Corrected-harness A/A: PASS

`r0904repairaa` completed 200 games in 12m45s: W/D/L **47/93/60**;
penta buckets **6/34/32/23/5**, estimate **-22.6 [-57,+11]**, pLLR -0.45.
Zero flags, illegal moves, crashes, no-move/disconnect, watchdog or voided-game
lines. Adjudication fired 107 decisive / 53 draw times. `DONE_EXIT_0`.
The CI covers zero, so the predeclared short A/A gate passes. Its width still
cannot exclude a small bias; this result is not an engine-strength verdict.

## Original comparison invalidated; compatible control required

`r0904repair` was stopped and is **INVALID, not a strength verdict**. At games
24, 60 and 71 the old baseline forfeited with an illegal move (each increments
the candidate win count). None of these games will be pooled into another run.

A direct UCI reproduction proves a baseline receiver defect: after
`position fen 4k3/8/8/8/2pP4/8/8/4K3 b - d3 0 1 moves c4d3`, the old binary
prints `invalid move: c4d3` and searches the stale black-to-move board, emitting
`c4c3` when White should move. The repaired engine accepts the move and emits
the legal White reply `e1d2`. The original baseline cannot interoperate with an
opponent that plays black en passant. The three individual games lack move
traces, so their exact move sequences are unproved; all are baseline-side
errors consistent with this independently reproduced incompatibility.

**Replacement control, declared before relaunch:** `b378c432cb34` plus ONLY
the one-line en-passant victim-rank repair in `generateBlackPawnMoves`.
Its search/eval code is byte-for-byte the original base. This is a necessary
protocol compatibility repair on both sides, not tuning the control after a
loss. The comparison isolates the remaining search/clock repairs; it cannot
assign Elo to the en-passant fix itself.

Built in `output/_compat_base` (isolated `git archive b378c43`, one-line patch),
same Go/compiler/target flags as before. `ngn_20260904_compat.exe` SHA-256:
`98f6d7667039f3b913cd92b4a97bb3ef739fa92ee6336c34ffd0111d37318d02`.
Candidate and harness remain the frozen, already A/A-tested binaries. Reuse
`r0904repairaa`: same machine, TC, concurrency, harness and candidate.

New run **`r0904repair2`**, same predeclared 400-game cap and error/non-regression
rule, with `-base .\ngn_20260904_compat.exe`. All other arguments unchanged.

## Compatible-control comparison: significant regression; deployment held

`r0904repair2` completed **400 games in 25m06s**, W/D/L **95/173/132**;
penta buckets **15/56/86/37/6**, **-32.2 Elo [−55,−10]**, pLLR −1.50.
Zero errors and zero flags on either side; 227 decisive and 79 draw adjudications.
`DONE_EXIT_0`. Although the sequential test says inconclusive at cap, the
predeclared fixed-cap paired interval excludes zero on the negative side.
**The non-regression gate does not pass.** The fresh rating run and deployment
are held while the new legality-check cost is investigated. These games are
complete evidence and will not be extended or pooled into a replacement run.

The correctness regressions remain proved. The next step is an exact search
optimization to reduce the new stalemate-probe cost, with node/score/move
identity against the repair, an independent legality audit, repeated timing,
and then a separate fresh game check against the compatible control.
