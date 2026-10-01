# 2026-07-19 T7 — history pruning modernization (depth<=6, depth-scaled threshold)

Pre-registered BEFORE launch (CLAUDE.md work-loop step 5). Isolated single-change per-change gate: candidate
= T5-keep HEAD engine (`5b93966`, == working-tree HEAD `c940fe8` — engine byte-identical, T10b reverted) +
`output/t7.patch`, base = T5-keep HEAD engine (`5b93966`) unchanged, reusing the on-box `ngn_t5.exe` (the exact
T5-keep binary), real-clock 10+0.1 c8 on the LAN 9800X3D box. Highest-ranked unblocked queue item after the
T10b shelve (biggest remaining measured prior, +23.7 [E] CMH-pruning class). Engine diff stays UNCOMMITTED per
policy (shelved as `output/t7.patch`, tree restored); commit only on a keep/provisional verdict.

## Mechanism (one paragraph)

NGN's quiet-move history pruning (search.go ~1730) was the flat form the 2026-07-01 audit flagged ("history
pruning depth<=3 flat"): a flat depth cap `depth <= 3`, a flat constant threshold `histScore < -1000`, the
`legalTried > 3` late-move requirement, guards `!isPV && !inCheck && !IsCapture && PromoType()==NoType && move
!= ttMove`, and NO killer/counter exemption. T7 modernizes that single block to the Ethereal-standard shape:
(1) the depth cap extends `depth <= 3` -> `depth <= 6`; (2) the flat threshold becomes depth-scaled
`histScore < -512*depth` (shallow nodes prune moderately-bad history, deep nodes demand a larger deficit — the
standard `margin*depth` form); (3) killers and the counter-move are exempted
(`!IsKillerMove(move, ply) && move != GetCounterMove(GetLastMovePlayed())`), mirroring the adjacent
futility-prune W1 guard, because now that pruning reaches depth 6, dropping a good ordering move here would fail
the node low and trigger a re-search (the closed-position tree inflation the futility W1 comment warns about).
The combined `GetHistoryScore(move, GetLastMovePlayed(), prev2)` main+continuation+followup score path, the
`legalTried > 3` guard, the `info.HistPrunes` counter, and the `SearchToggles.HistPrune` default-on are all
unchanged; the main move loop only runs at `depth >= 1` (depth 0 dispatches to quiescence at search.go:1269), so
`-512*depth >= -512` — no zero/negative-depth edge. This is a pruning (search-behavior) change: fixed-depth node
counts MOVE (nodecheck below), so it is gated by real-clock games, not proxies. The -512 coefficient and the
depth-6 cap are noted as future SPSA dims (T8); the revamp doc Part 3 T7 entry prescribes the shape ("depth<=6,
depth-scaled threshold") but no exact constants, so the Ethereal-standard linear form is used.

```yaml
id: 2026-07-19-t7-history-pruning
date: 2026-07-19
change_class: search/eval heuristic (search behavior — quiet-move pruning; real-clock games gate)
hypothesis: >
  modernizing NGN's flat history pruning (depth<=3, flat -1000 threshold, no killer/counter exemption) to the
  Ethereal-standard shape (depth<=6 cap, depth-scaled -512*depth threshold, killer/counter exempt) nets positive
  Elo at real clock by pruning more low-history quiets at shallow-to-mid depth without dropping good ordering
  moves. Measured prior: Ethereal 23b841e CMH prune+sort +23.7 / +34.5 LTC (the biggest single classical-Ethereal
  item); history pruning +4.1 [V]. H1: candidate >= +3 Elo over base; H0: candidate <= -3.
base_commit: 5b93966          # T5-keep HEAD engine (== working-tree HEAD c940fe8, engine byte-identical)
candidate_commit: 5b93966 + output/t7.patch   # engine == 5b93966 + t7; patch uncommitted per policy
base_binary_sha256: 0a8f65b75e59c376052f83b14787b0af2eba65552cedfee6f70ce576891ca721      # on-box ngn_t5.exe (reused T5-keep binary, hash-verified on box)
candidate_binary_sha256: 34bbfc66a70e121e187d571a213fbf46f3cf91aa0b2d31c429aa04fd29e3ecbf  # output/ngn_t7.exe (reproducible build, hash-verified on box)
harness_commit: sprt.exe sha256 30c33e0512725b7f552d8a1cf72ba6f1e0deb4dcb122c6b6ba8f6433c686a762 (M5-aware validated mill, box-staged, untouched); launcher scripts/boxsprt.sh untouched
command: sprt.exe -new .\ngn_t7.exe -base .\ngn_t5.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 8000 -mingames 300 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
machine: AMD Ryzen 7 9800X3D 8c/16t, native Windows (LAN box 192.168.4.108)
go_version: go1.26.2 (local cross-compile, darwin/arm64 host)
goarch_goamd64: windows/amd64 GOAMD64=v3
tc: 10+0.1 (seconds; bullet)
concurrency: 8
openings: canonical sprt_openings (5000 lines)
openings_sha256: 974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222
adjudication: STANDARD-ON (-resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80; M5-validated non-distorting)
maxgames: 8000
mingames: 300
aa_preflight: standing 2026-07-03 M5-enable adjudication-ON A/A (experiments/2026-07-03-m5-adjudication-aa.md) — same TC (10+0.1) / concurrency (8) / adjudication-flag set / machine (LAN 9800X3D) / openings config, penta -1.1 [-13,+11], pLLR -0.18, 0/0 flag-outs, 77.6% adjudicated, 961 g/hr, DONE_EXIT_0; the SAME convention adopted by the KEPT T1b/T4c/T1e/T5 and SHELVED T13/T1c/T16/T10b runs, so no fresh A/A required.
decision_rule: >
  SPRT [-3,+3] on pentanomial pLLR, bounds +/-2.94, maxgames cap 8000; pLLR >= +2.94 -> KEEP (Batch-2 keep per
  CLAUDE.md batch-certification path); pLLR <= -2.94 -> SHELVE and revert; capped at 8000g with point est >= +1,
  pLLR > 0, and 0 excess flag-outs -> Batch-2 PROVISIONAL keep; capped-nonpositive -> shelve; any candidate
  flag-out above the A/A baseline (0) -> HALT and investigate.
games_or_pairs: (pending)
result: (pending)
flags_errors: (pending)
verdict: (pending)
next_action: (pending)
```

## Nodecheck (node counts EXPECTED TO MOVE — pruning change, NOT re-locked)

Baselines (5b93966): 346662 / 149587 / 765656. Pruning change => counts MOVE (expected, not node-identical).
Deterministic (two identical runs produced identical counts):

| position | depth | baseline (5b93966) | T7     | delta |
|----------|-------|--------------------|--------|-------|
| kiwipete | d12   | 346662             | 346665 | +3    |
| mid      | d12   | 149587             | 149587 | 0     |
| end      | d16   | 765656             | 765654 | -2    |

Counts moved (confirming behavioral, not node-identical) but only slightly on these three positions: history
pruning is a rare event (needs `hist < -512*depth` AND `legalTried > 3`), and NGN's gravity-form history entries
cluster near 0, so the deeper depth-4..6 thresholds (-2048/-2560/-3072) seldom fire while the killer/counter
exemption removes a few prunes — the two roughly offset here. Single-position node shape is not a strength
signal — the real-clock SPRT owns strength. Baselines are NOT re-locked; that happens only on a keep verdict.

## Tests (patched tree, all GREEN — verbatim in output/t7-notes.md)

- `go test -short ./engine -count=1` -> ok
- `go test -short -race ./engine -count=1` -> ok
- `go test -short ./... -count=1` -> ok (engine, cmd/sprt, internal/uci)

No new regression test: the change reshapes an existing pruning threshold with no new seam a deterministic unit
test would pin better than nodecheck already does (the `HistPrunes` counter is exercised by the fixed-depth
search); a crafted "prune fires" position would be brittle and is not forced, per the work-loop guidance.

## Binaries

Cross-compiled with the record-convention command (Makefile `build` Windows line):
`GOOS=windows GOARCH=amd64 GOAMD64=v3 go build -o output/ngn_t7.exe main.go`, go1.26.2, no ldflags/trimpath,
from clean T5-keep HEAD `5b93966` + `output/t7.patch`. Tree restored after build; patch stays uncommitted.

- **Candidate** `output/ngn_t7.exe` sha256 `34bbfc66a70e121e187d571a213fbf46f3cf91aa0b2d31c429aa04fd29e3ecbf`
  (authority = output/t7-notes.md; reproducible — a second identical build reproduced the sha; local
  `shasum -a 256` and on-box `Get-FileHash` both match).
- **Base** reuses the on-box `ngn_t5.exe` sha256 `0a8f65b75e59c376052f83b14787b0af2eba65552cedfee6f70ce576891ca721`
  — the exact `5b93966` (T5 keep) engine binary already staged and hash-verified on the box; no rebuild.

Patch: `output/t7.patch` (git diff of engine/search.go only; single-block change).

## Bounds — [-3,+3] per-change gate (NOT the [0,+6] cert shape)

Isolated per-change SPRT: `-elo0 -3 -elo1 3` (H0: elo<=-3, H1: elo>=3), same shape as the KEPT T1b/T4c/T1e/T5
and SHELVED T13/T1c/T16/T10b runs. `alpha=beta=0.05` yields pLLR bounds +/-2.94. Expected header:
`H0: elo<=-3.0 H1: elo>=3.0 (alpha=0.05 beta=0.05 -> LLR bounds [-2.94, 2.94])`.

## Planned box run (staging + launch)

Base already on box; candidate staged and both on-box hashes re-verified before launch:

```bash
scripts/boxsprt.sh push output/ngn_t7.exe ngn_t7.exe   # candidate — new to box (DONE)
scripts/boxsprt.sh hash ngn_t7.exe          # 34BBFC66…  (verified)
scripts/boxsprt.sh hash ngn_t5.exe          # 0A8F65B7…  (verified, reused base)
```

Launch (writes `run_t7.bat`, out-file `t7_out.txt`):

```bash
scripts/boxsprt.sh launch t7 -- -new '.\ngn_t7.exe' -base '.\ngn_t5.exe' -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 8000 -mingames 300 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
```

## Launch

CONFIRMED LIVE 2026-07-19 20:48:11 EDT via `scripts/boxsprt.sh launch t7` (command exactly as the planned block
above). Topology verified: 1 `sprt.exe` (PID 30792, 20:48:11) + 8 `ngn_t7` (PIDs 15204, 15448, 18312, 19704,
21496, 24696, 31012, 31708) + 8 `ngn_t5` (PIDs 2660, 5980, 13396, 22344, 22872, 23240, 24816, 30436), all
started 20:48:12 EDT — the sole live workers on the box (idle before launch, `ps` empty). Out-file:
`C:\Users\ehrli\ngn\sprt\t7_out.txt` (fetch via `scripts/boxsprt.sh fetch t7`).

On-box hashes verified at stage time: candidate `ngn_t7.exe`
`34BBFC66A70E121E187D571A213FBF46F3CF91AA0B2D31C429AA04FD29E3ECBF`, base `ngn_t5.exe`
`0A8F65B75E59C376052F83B14787B0AF2EBA65552CEDFEE6F70CE576891CA721` — both full-hash matches. Interactive
session preflight passed (`explorer.exe` PID 2060, since 2026-07-18 20:41 — the same session T10b used). Header
echoes the predeclared binaries/args/bounds exactly (verbatim from the out-file):

```text
new=.\ngn_t7.exe  base=.\ngn_t5.exe
mode: real clock 10s+0.1s (concurrency 8) | openings: 5000 (x2 colors) | concurrency: 8
H0: elo<=-3.0   H1: elo>=3.0   (alpha=0.05 beta=0.05 -> LLR bounds [-2.94, 2.94])
adjudication: resign>=900cp/5p  draw<=10cp/10p>=80p  (opt-in; A/A-gate before trusting a verdict)
```

Early games flowing G1->G10 (out-file 1110->1288 B while polling), adjudication firing (adj-win/adj-draw/
draw-rule), 0 forfeit/flag-out/panic/illegal/crash lines:

```text
G1     1W  0D  0L  100.0%  elo +800.0 [-234,+800]  LLR  +0.09  pLLR  +0.00  adj-win
G5     2W  2D  1L   60.0%  elo  +70.4 [-209,+350]  LLR  +0.03  pLLR  +0.02  adj-win
G10    4W  4D  2L   60.0%  elo  +70.4 [-137,+278]  LLR  +0.06  pLLR  +0.07  draw-rule
```

(The transient G4 pLLR +4.32 is single-pair early noise — the CI spans ±hundreds at <10 games; it settles to
+0.07 by G10.) Poll with `scripts/boxsprt.sh tail t7` / `scripts/boxsprt.sh ps`; verdict is a separate agent
per the predeclared decision_rule (the run drives to the 8000g cap or a +/-2.94 pLLR crossing).

## RESULT — SHELVED (capped-nonpositive), 2026-07-24

Run COMPLETED to the 8000-game cap and self-cleaned; box idle, `boxsprt.sh ps` empty. Verdict applied per the
predeclared `decision_rule` (capped-nonpositive -> shelve and revert). Out-file retained locally as
`output/t7_out.txt` sha256 `71e68dc3e5bae269fb738f41798c7ac54b9f2e6fa5cff527a2553f791a6d5fc2` (box copy
`C:\Users\ehrli\ngn\sprt\t7_out.txt`, 728,284 B, mtime 2026-07-20 05:07).

```text
=== RESULT (8h19m28s) ===
Games: 8000   W-D-L: 2139-3718-2143   score: 50.0%
Elo(new - base): -0.2   95% CI [-8, +7]
LLR: -0.13   bounds [-2.94, 2.94]
Pentanomial [LL 224  LD 946  {LW,DD} 1649  WD 972  WW 209] over 4000 pairs
Penta Elo: -0.2   95% CI [-5, +5]   pLLR -0.15  (THE decision stat; trinomial above is secondary)
Flag-outs (lost on time): new 0, base 0  of 8000 games  (goal: 0)
Adjudicated early: 4272 decisive, 1909 draw  of 8000 games
Verdict: INCONCLUSIVE (ran out of games - add openings or raise -maxgames)
DONE_EXIT_0
```

```yaml
games_or_pairs: 8000 games / 4000 pairs (maxgames cap reached; 8h19m28s)
result: >
  penta Elo -0.2, 95% CI [-5, +5], pLLR -0.15 (final); trinomial elo -0.2 [-8, +7], LLR -0.13.
  Pentanomial [LL 224, LD 946, {LW,DD} 1649, WD 972, WW 209]. W-D-L 2139-3718-2143 = 50.0%.
  Adjudicated early 4272 decisive / 1909 draw of 8000 (77.3%, in line with the 77.6% A/A baseline).
flags_errors: 0 flag-outs (new 0, base 0), 0 panics, 0 illegal moves, 0 disconnects, 0 no-move, 0 crashes; DONE_EXIT_0
verdict: >
  SHELVED — capped-nonpositive. At the 8000g cap the point estimate is -0.2 (< the +1 provisional floor) and
  pLLR -0.15 (< 0), so the batch-certification provisional path does NOT apply; the predeclared rule sends this
  to shelve-and-revert. Engine diff reverted, shelf copy retained as output/t7.patch.
next_action: >
  Revert engine/search.go to HEAD (done), keep output/t7.patch as the shelf copy, mark T7 shelved in TODO.md,
  and launch the next queued item. REOPEN only as a re-derived variant that first passes a pre-launch
  behavioral-delta gate (see Reopen condition below).
```

### pLLR envelope (full-run scan, 8001 sampled lines)

| scope | max | min | final |
|---|---|---|---|
| whole run | **+4.32** (G4 only) | -0.92 | -0.15 |
| post-mingames (G >= 300, n=7701) | **+1.92** | **-0.92** | -0.15 |

The single +4.32 sample is the G4 line already flagged as early noise in the Launch section above
(`G4  2W 2D 0L  75.0%  elo +190.8 [-147,+528]  LLR +0.14  pLLR +4.32`) — one pair at a ±hundreds CI, below the
`-mingames 300` floor, so the harness correctly did not stop there. The stop rule was never armed by it, and no
post-mingames sample came within a full unit of either +/-2.94 bound. This is a flat null, not a truncated
trend: the decision statistic spent the whole powered portion of the run inside [-0.92, +1.92].

### Interpretation — the null is on a near-inert change, so the LANE is not closed

The pre-launch nodecheck already showed the candidate was barely moving the tree (kiwipete +3 of 346662 =
+0.001%, mid 0 of 149587 = 0.000%, end -2 of 765656 = -0.0003%), and the manifest reasoned why: the depth-scaled
threshold is a MIXED edit, not a uniform loosening, and the exemption claws back more.

| depth | old threshold | T7 threshold (-512*d) | direction |
|---|---|---|---|
| 1 | -1000 | -512   | MORE pruning |
| 2 | -1000 | -1024  | ~neutral |
| 3 | -1000 | -1536  | LESS pruning |
| 4-6 | (not pruned) | -2048 / -2560 / -3072 | newly enabled, but a very large deficit |

Entries are gravity-bounded at `historyMax = 8192` per table (moveorder.go:219) and `GetHistoryScore` sums
main + continuation + followup, so -2048..-3072 is *reachable* in principle but sits far out in the tail of a
distribution that clusters near 0 — so the depth-4..6 extension seldom fires, while the new
killer/counter exemption removes prunes at every depth. The three effects roughly cancel, which is exactly what
both instruments reported: ~0.001% node delta and -0.2 +/- 5 Elo.

So the honest reading is that this run measured a parameterization that barely changes behavior, NOT the
hypothesis "modernizing history pruning is worth Elo in NGN". The +23.7 [E] CMH-pruning prior is untested here,
and T7's null is weak evidence about the lane. It IS strong evidence about this constant set.

### Reopen condition (and a process gate this run earned)

REOPEN as a re-derived variant only, never a re-run of these constants, and only after a **pre-launch
behavioral-delta gate**: a pruning/reduction candidate must move fixed-depth nodecheck by a materially
non-trivial margin (proposed: >= 1% on at least one of the three canonical positions) BEFORE it is worth 8+ hours
of box time. T7 burned 8h19m28s to measure a 0.001% tree change; the nodecheck table in this very manifest
predicted that and the run was launched anyway. Compare the KEPT T5 (nodecheck moved +51%/-18%/+30%) and the
SHELVED-but-genuinely-tested T16 (+30%/-31%/-17%) — both actually exercised the mechanism they claimed.

Concrete re-derivation candidates, in the order they should be tried if the lane is revisited:
1. A materially smaller coefficient (e.g. `-256*depth`, or a two-term `-1000 - 256*depth`) so the depth-4..6
   extension actually fires at a rate the nodecheck can see;
2. the depth-cap extension ALONE (`depth <= 6`, flat -1000, no exemption) — the single-axis version, which is
   unambiguously a loosening and would move nodes;
3. the killer/counter exemption as its own isolated change, since here it was confounded into a package.
Route any of these through the nodecheck gate first, and prefer whichever shows the largest tree delta per line
of diff. Best handled as SPSA dims under T8 rather than more hand-picked constants.
