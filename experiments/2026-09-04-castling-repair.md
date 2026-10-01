# Internal castling legality can produce a losing UCI fallback

Status: **ACCEPTED correctness repair**, completed predeclared game gate;
deployed on WSL, Windows and Mac defaults. This takes precedence over
the unfinished T8 heuristic confirmation, which was cancelled without a verdict.
Reviewed/played source: `71399c0`; main's later changes are offline tuner/docs only.

## Mechanism evidence from an actual loss

Fresh gauntlet GAME170, ply16:

```text
r1bqk2r/pppnnpp1/3p3p/4p3/3P1PPb/1P2P2P/P1P1N1B1/RNBQK2R w KQkq - 3 9
```

The black bishop on h4 checks the white king along g3–f2–e1. Independent Stockfish
perft lists exactly **Ng3, Kf1, Kd2**. Castling is illegal. The released engine
produces this result from either the FEN or the complete game history:

```text
info depth 1 ... score cp 3  ... pv e1g1
info depth 2 ... score cp -17 ... pv e1g1
info depth 3 ... score cp -3 ... pv e1g1
bestmove e2g3
```

After Ng3, NGN itself finds **...Bxg3+**, winning the knight, at depth 1. At depth
3 it scores Black +439. The internal root search optimizes an illegal castle;
the existing UCI legality guard substitutes the first legal fallback, Ng3.
Therefore legal emitted moves and a clean game replay do not establish internal
search legality. The search's chosen best move and PV need their own checks.

In the targeted diagnostic, 400k nodes and each of ten individually disabled
search heuristics still produced Ng3. Complete SF depth14 prefers Kf1 (root −105)
and scores Ng3's continuation around −461. This is a demonstrated search-rule
failure, not evidence that a new evaluation model is required.

Raw full-history/FEN/independent perft transcript:
`output/recovery-2026-09-04/illegal-castle-repro.txt`.
Targeted ablations and exact prefixes: `targeted-ablation.json`.

## Repair scope and validation requirements

Trace the shared and duplicated castle checks. The iterative root's fast
make/final-king-check path does not prove starting/transit square safety.
The in-check qsearch and offline quiet-search consumers need the same audit.
The main alpha-beta loop already has special castling checks; prefer consistent
semantics over another independent partial implementation.

The isolated Terra checkout is `output/castle-repair-20260904`. Root reviews and
controls integration, game tests and deployment. Cover both colors/wings,
starting check, attacked transit squares, final-square safety, valid castles,
repeated searches and caller-state preservation. Test the actual internal best
move/PV before UCI fallback, and the full-history case above. Run short/race
suites and the independent move-generation oracle as relevant.

No speculative vector or new weights are included. A source proof and regression
suite establish legality; a bounded real-clock comparison with the frozen release
checks cost and playing behavior. Record its exact manifest before launch. The
existing accepted release stays deployed until the new repair is validated.

## Predeclared game gate

Run name `r0904castle`, after the isolated repair's tests and independent oracle
pass, source/patch/binary hashes are filled in, and the box is verified idle.
Compare the correction alone against the frozen `ngn_20260904_fast.exe`
(`7b5af06a4a0ff02a0d9c51aa5eb041dd8ddf9935bbe4691d65dd76146a515828`).
The existing corrected SPRT binary remains frozen at
`63ef86d9b986fdb2c260782149af259e278845d89595d010df3970e6698cc9ed`.

**Fixed 400 paired games**, 10+0.1/c8, native 9800X3D Windows, default affinity,
`-lowpower=false`; same pinned openings SHA-256
`974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222`.
Preflight basis is the completed same-day `r0904fastaa`: same base binary,
harness, TC, concurrency, affinity, power option and machine; 200 games,
−13.9 [−44,+16], zero errors/flags. No harness/scheduling change has occurred.

Require proved internal legality, all regression/oracle checks, and zero
flags/illegal or missing moves/crashes/disconnects/watchdogs in the comparison.
A significantly negative paired interval rejects this implementation pending
investigation. An inconclusive non-regression result can support the proved
rule correction, but is not a measured gain or an H1 claim. No early stopping
or extension based on the score sign. A future absolute rating must use a new
predeclared match and does not follow by adding self-play Elo to 2726.

```text
sprt_20260904.exe -new .\ngn_20260904_castle.exe -base .\ngn_20260904_fast.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 400 -mingames 401 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
```

Candidate playing artifacts frozen before launch, Go1.26.2, windows/amd64/v3;
committed runtime is `70eb527`:

- Source parent: `c53484b`, plus `output/recovery-2026-09-04/castle-runtime.patch`
  across `search.go`, `movegen.go`, `has_legal_moves.go`, `texel.go`.
  Runtime patch SHA-256: `ffb8396c7995b9dd51e03e7ecb7f3593e29d8a9865ea38743bb0d6cee56b9ff5`.
- Windows `build/ngn_20260904_castle.exe`:
  `131b9bbf1cb064bd6c1edd5513db8e6c670eba154231f3aadd66cc1942a85e7d`.
- Mac `build/ngn_20260904_castle`:
  `1010178789b5560583c0ee430934dc4e12473a7dbbdbf07598a30bb3185a3a0f`.

## Repair and pre-game checks

All pseudo-legal castle consumers now use the same direct king-path check:
iterative/fixed roots, alpha-beta, qsearch evasion/stalemate probes, `HasLegalMove`
and the offline quiet filter. Root drivers score zero **legal** moves as mate or
stalemate, including when their pseudo-legal list is nonempty. Interrupted roots
are not classified as terminal merely because no move has finished.

New regression tests fail against the old runtime: GAME170 chooses illegal e1g1;
`HasLegalMove` invents a castle evasion in the double-check mate fixture; and a
stalemated root returns −2147483647. On the repaired build, GAME170 depth3 returns
legal PV/bestmove **e1f1**. Tests include both colors/wings, source/transit/final
attacks, safe castles, full history, repeated search, offline/qsearch mate,
caller-state preservation, and mate/stalemate scores in both root drivers.

`go test -short ./...`, `go test -short -race ./...`, `go vet ./...` all pass.
Stockfish oracle: **2,293 positions, 48,926 divides, 128 internal search roots**.
The oracle checks legal-set membership even when a legal mating move has zero
replies. Two canonical runs give identical node/score/PV/bestmove results:
Kiwipete d12 301843 nodes; middlegame d12 112109; rook endgame d16 858052.
The latter two are identical to the prior release; Kiwipete changes under the
rule repair. UCI en-passant and fifty-move mate smoke cases still pass.

Raw evidence is under `output/recovery-2026-09-04/castle-*`. Test builds carry
the pre-commit parent plus dirty metadata; the exact runtime patch and hashes
above identify what was tested. No T17/T8 parameters or new eval weights enter
this candidate. Completed result follows.

## Incidence in the complete fresh gauntlet

An offline replay of all 200 games inspected **13,181 NGN-to-move positions**
after the six opening plies. There were **39 positions in 25 games** where the
generator offered a castle rejected by the legal filter; 17 were in check.
Fresh full-history depth3 searches with the frozen release selected an illegal
PV head in **eight positions across seven games**: two wins, three draws, two
losses. In all eight, the legal emitted bestmove matched the actual recorded
move. GAME170 contributes two positions.

This establishes recurrence beyond the initial example. It is shallow, reset
search-state evidence consistent with a fallback; it does not reconstruct the
original timed searches or count seven games lost to the defect. Most susceptible
positions did not produce an illegal depth3 choice, and winning/drawing examples
provide useful counterevidence to attributing every affected game to the bug.

Probe/method/raw transcripts: `output/recovery-2026-09-04/illegal_castle_incidence.*`.
All game moves were independently audited previously; this probe also verifies
full legal replay with the corrected local engine. Frozen input/release hashes
are stored in its JSON.

## Completed game verdict

`r0904castle` completed **400 games / 200 pairs in 25m31s**, DONE_EXIT_0.
W/D/L **99/205/96**, pentanomial **8/54/74/55/9**. Paired Elo **+2.6
[−20,+25]**, pLLR **+0.12**. Zero flags on either side; zero recorded illegal
moves, missing moves, crashes, disconnects or watchdogs. All 400 sequential game
records have normal adjudication/checkmate/draw/max-move endings. Native workers exited.

The strength verdict is **INCONCLUSIVE**, with no H1 claim and no extension of
the fixed cap. The predeclared correctness gate **PASSES**: the rule defect is
proved and regressed; tests/oracle pass; no significant negative paired interval
or operational failure is present. This is not a measured strength gain or a new
absolute rating. The last absolute pin remains 2726 [2668,2784].

Raw log SHA-256: `7c9f76ab15bb163886af9253bb94b2278a9f20fea899c3663fa961c9f5353f44`.
Structured completion/reason audit: `output/recovery-2026-09-04/r0904castle-audit.json`.

Linux deployment artifact, Go1.26.2 linux/amd64/v3, built from later docs-only
commit `6175b48` with runtime identical to `70eb527`:
`build/ngn_20260904_castle_linux`, SHA-256
`80b7488a16ad66f80ff0c618b64baa6445d4c24cdedb78e5d2e87c3a57d32b3e`.
Windows/Mac activation uses the frozen tested artifacts above.

Deployment completed and smoke-verified on all three platforms, with rollback
copies and native WSL short/race tests. [Installed paths and hashes](2026-09-04-deployment.md).
