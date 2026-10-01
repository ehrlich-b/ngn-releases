# 2026-07-18 T1e — node-fraction (best-move node-effort) time scaling

Pre-registered BEFORE launch (CLAUDE.md work-loop step 5). Isolated single-change per-change gate: candidate
= HEAD engine (`c514a2b`) + `output/t1e.patch`, base = HEAD engine (`c514a2b`) unchanged, real-clock 10+0.1
c8 on the LAN 9800X3D box. This is the next ranked queue item after the Batch-1 certification
(`2026-07-18-batch1-cert.md`, CERTIFIED); base pointer now `c514a2b`. Full prep detail (edits, constants,
nodecheck identity, test output) in `output/t1e-notes.md`.

## Mechanism (one paragraph)

Adds a third soft-budget time signal alongside T1b (decision-stability scaling): the fraction of an
iteration's search nodes spent inside the best move's subtree. A best move that owns most of the tree is a
confident decision (shrink the soft target, bank the time); a best move that owns a small share is a contested
position where the search had to spread nodes across rivals (extend the soft target). Per-root-move node
attribution is snapshot-based (`info.Nodes` read before/after each root child; the best move's cost recorded
whenever a move becomes the new iteration best), the fraction is `bestMoveNodes / totalAttemptNodes`, fed to
the TimeManager on successful iteration completion via the existing `ReportCompletedIteration` call and
consulted only by the Tournament soft stop. The node-effort factor is
`nodeEffortMin + (nodeEffortMax - nodeEffortMin)*(1 - fraction)` with `nodeEffortMin=0.85`, `nodeEffortMax=1.25`
(neutral 1.0 at fraction=0.625; 1.0 on no data), composed multiplicatively with T1b's stability factor and
clamped to `[0.72, 1.40]` (the two signals are correlated, so the composed ceiling sits only modestly above
T1b's proven 1.30). The T1a-proven next-iteration projection, the hard ceiling, and the emergency floor are
UNTOUCHED, so the bound cannot raise flag risk; fixed-depth/fixed-nodes search is unaffected (nodecheck
byte-identical 230036 / 181982 / 587557).

```yaml
id: 2026-07-18-t1e-node-effort
date: 2026-07-18
change_class: search/eval heuristic (time management — real-clock games gate only)
hypothesis: >
  scaling the soft time budget by the best move's node-effort share (a third, SF-standard time signal
  composed with T1b's decision-stability scaling) spends time where the search is contested and banks it
  where the best move dominates, netting positive Elo at real clock without raising flag-out risk. H1:
  candidate >= +3 Elo over base; H0: candidate <= -3.
base_commit: c514a2b          # HEAD engine (certified base); HEAD 194d894 is doc-only above c514a2b
candidate_commit: c514a2b + output/t1e.patch   # engine == c514a2b + t1e; patch uncommitted per policy
base_binary_sha256: 40783f0444ed47cf50bf9ca0dfa5d7635536e1a1519fe47c902498665141a1f8      # on-box ngn_cert.exe (reused, hash-verified at batch-1 cert)
candidate_binary_sha256: f65a3c164987ef79adcf4062ae0f872a4c2c1f899c15ac052c183f2a75b30a1c  # output/ngn_t1e.exe
harness_commit: sprt.exe sha256 30c33e0512725b7f552d8a1cf72ba6f1e0deb4dcb122c6b6ba8f6433c686a762 (M5-aware validated mill, box-staged, untouched)
command: sprt.exe -new .\ngn_t1e.exe -base .\ngn_cert.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 8000 -mingames 300 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
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
aa_preflight: standing 2026-07-03 M5-enable adjudication-ON A/A (experiments/2026-07-03-m5-adjudication-aa.md) — same TC (10+0.1) / concurrency (8) / adjudication-flag set / machine (LAN 9800X3D) / openings config, penta -1.1 [-13,+11], pLLR -0.18, 0/0 flag-outs, 77.6% adjudicated, 961 g/hr, DONE_EXIT_0; the SAME convention adopted by the KEPT T1b/T4c and SHELVED T13/T1c/T16 runs, so no fresh A/A required.
decision_rule: >
  SPRT [-3,+3] on pentanomial pLLR, bounds +/-2.94, maxgames cap 8000; pLLR >= +2.94 -> KEEP; pLLR <= -2.94
  -> SHELVE; capped at 8000g with point est >= +1, pLLR > 0, and 0 excess flag-outs -> Batch-2 PROVISIONAL
  keep (CLAUDE.md batch-certification path); capped-nonpositive -> shelve; any candidate flag-out above the
  A/A baseline (0) -> HALT and investigate.
games_or_pairs:   4226 games / 2113 pairs
result:           penta +6.6 [-1,+14], pLLR peak +2.96 crossed the +2.94 bound at G4218 (first & only crossing), c8-drain to +2.85 at 4226g; trinomial +6.6 [-4,+17]
flags_errors:     0/0 flag-outs (new 0, base 0 of 4226); no crashes/illegal/no-move; 2308 adj-decisive + 1022 adj-draw
verdict:          keep (FULL H1 ACCEPTED; predeclared pLLR >= +2.94 fired — peak +2.96 at G4218)
next_action:      commit t1e.patch to main as Batch-2 keep #1 (certified base c514a2b); time-mgmt package now T1b+T1e; next box item T5
```

## Binaries

Cross-compiled with the record-convention command (Makefile `build` Windows line):
`GOOS=windows GOARCH=amd64 GOAMD64=v3 go build -o output/ngn_t1e.exe main.go`, go1.26.2, no ldflags/trimpath.

- **Candidate** `output/ngn_t1e.exe` sha256 `f65a3c164987ef79adcf4062ae0f872a4c2c1f899c15ac052c183f2a75b30a1c`
  — built from clean HEAD + `output/t1e.patch` (guard `go test -short ./engine -count=1` GREEN before build;
  nodecheck byte-identical 230036/181982/587557 per `output/t1e-notes.md`). Tree restored after build
  (`git checkout -- .`, `git apply --check output/t1e.patch` re-passes); patch stays uncommitted per policy.
- **Base** reuses the on-box `ngn_cert.exe` sha256 `40783f0444ed47cf50bf9ca0dfa5d7635536e1a1519fe47c902498665141a1f8`
  — the `c514a2b` engine binary already staged and hash-verified at the Batch-1 certification; no rebuild.
  (`c514a2b` == current HEAD `194d894` engine behavior; the delta above `c514a2b` is doc-only.)

## Bounds — [-3,+3] per-change gate (NOT the [0,+6] cert shape)

This is an isolated per-change SPRT, so `-elo0 -3 -elo1 3` (H0: elo<=-3, H1: elo>=3), same shape as the KEPT
T1b/T4c and SHELVED T13/T1c/T16 runs — NOT the batch-certification [0,+6]. `alpha=beta=0.05` yields pLLR
bounds +/-2.94. Expected header parse:
`H0: elo<=-3.0 H1: elo>=3.0 (alpha=0.05 beta=0.05 -> LLR bounds [-2.94, 2.94])`.

## Planned box run (staging + launch)

Stage the candidate (base already on box) and re-verify both on-box hashes before launch:

```bash
scripts/boxsprt.sh push output/ngn_t1e.exe ngn_t1e.exe    # candidate — new to box
scripts/boxsprt.sh hash ngn_t1e.exe        # expect F65A3C16…
scripts/boxsprt.sh hash ngn_cert.exe       # expect 40783F04…  (reused base)
```

Preflight: confirm an interactive session (`tasklist | findstr /I explorer.exe`) — boxsprt.sh's schtasks
launcher needs a logged-on session (locked is fine); NO session -> do NOT launch, report staged+blocked.

Launch (writes `run_t1e.bat`, out-file `t1e_out.txt`):

```bash
scripts/boxsprt.sh launch t1e -- -new '.\ngn_t1e.exe' -base '.\ngn_cert.exe' -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 8000 -mingames 300 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
```

At launch confirm: header parse, on-box SHA-256s, openings == 974e4b5a…, `ps` showing the t1e run as the ONLY
live worker (1 sprt + 8 ngn_t1e + 8 ngn_cert), then early health (~4 min: games completing, 0/0 flag-outs).

## Launch

CONFIRMED LIVE 2026-07-18 22:34:51 EDT via `scripts/boxsprt.sh launch t1e` (command exactly as above).
Topology verified: 1 sprt.exe + 8 ngn_t1e + 8 ngn_cert, all started 22:34:51; header matches the
predeclared rule (H0: elo<=-3.0 / H1: elo>=3.0, LLR bounds [-2.94, 2.94]); sole live worker on the box
(cert run had completed and drained). Early health at G27 (~10 min): 7W-12D-8L, pLLR +0.00, adjudication
firing (adj-win/adj-draw/max-moves), 0 forfeit/panic/illegal lines, flag-outs 0/0. Launched the same
evening the batch-1 cert certified (+43.2, 194d894); base ngn_cert.exe reused from that run, hashes
re-verified on box at stage time.

## Verdict

**KEEP** — FULL H1 ACCEPTED. The predeclared rule (`pLLR >= +2.94 -> KEEP`) fired: the pentanomial pLLR
**crossed +2.94 at G4218** (first and only crossing), peaking at **+2.96** (held G4218-G4220), then drained
over the 8 in-flight c8 games to **+2.85 at 4226g** (the known c8-drain pattern, T4c precedent — the peak
crosses the bound, the trailing pairs settle the stat lower). Independently re-verified on the box out-file
`t1e_out.txt` post-completion (all 4226 game-lines parsed): max pLLR +2.96 @G4218, first crossing @G4218; the
drain path G4218-4226 = +2.96, +2.96, +2.96, +2.92, +2.92, +2.92, +2.89, +2.89, +2.85. Hash-back:
re-cross-compiling clean HEAD + `output/t1e.patch` (`GOOS=windows GOARCH=amd64 GOAMD64=v3 go build`, go1.26.2)
reproduces the exact binary that played the run, sha256 `f65a3c16…` (byte-identical). Fixed-depth
node-identity re-verified on the patched tree (nodecheck 230036 / 181982 / 587557 all match baselines) — T1e is
a time-management-only change, so NO nodecheck re-lock. Tests green on the patched tree: `go test -short
./engine`, `go test -short ./...`, `go test -short -race ./engine` all pass.

Full RESULT block (verbatim, box `sprt.exe`):

```text
=== RESULT (4h21m6s) ===
Games: 4226   W-D-L: 1197-1912-1117   score: 50.9%
Elo(new - base): +6.6   95% CI [-4, +17]
LLR: +2.52   bounds [-2.94, 2.94]
Pentanomial [LL 118  LD 498  {LW,DD} 816  WD 548  WW 133] over 2113 pairs
Penta Elo: +6.6   95% CI [-1, +14]   pLLR +2.85  (THE decision stat)
Flag-outs (lost on time): new 0, base 0  of 4226 games  (goal: 0)
Adjudicated early: 2308 decisive, 1022 draw  of 4226 games
Verdict: H1 ACCEPTED: new is stronger (>= 3 ELO)
DONE_EXIT_0
```

Peak/crossing evidence (box out-file, G4215-G4226 verbatim):

```text
G4215 1196W 1906D 1113L   51.0%  elo   +6.8 [-4,+17]  LLR  +2.62  pLLR  +2.92  adj-win
G4216 1196W 1907D 1113L   51.0%  elo   +6.8 [-4,+17]  LLR  +2.62  pLLR  +2.92  adj-draw
G4217 1196W 1908D 1113L   51.0%  elo   +6.8 [-4,+17]  LLR  +2.62  pLLR  +2.92  draw-rule
G4218 1196W 1909D 1113L   51.0%  elo   +6.8 [-4,+17]  LLR  +2.62  pLLR  +2.96  adj-draw   <-- CROSS (peak)
G4219 1196W 1910D 1113L   51.0%  elo   +6.8 [-4,+17]  LLR  +2.62  pLLR  +2.96  adj-draw
G4220 1196W 1910D 1114L   51.0%  elo   +6.8 [-4,+17]  LLR  +2.59  pLLR  +2.96  adj-win
G4221 1196W 1910D 1115L   51.0%  elo   +6.7 [-4,+17]  LLR  +2.56  pLLR  +2.92  adj-win
G4222 1197W 1910D 1115L   51.0%  elo   +6.7 [-4,+17]  LLR  +2.59  pLLR  +2.92  adj-win
G4223 1197W 1910D 1116L   51.0%  elo   +6.7 [-4,+17]  LLR  +2.56  pLLR  +2.92  adj-win
G4224 1197W 1911D 1116L   51.0%  elo   +6.7 [-4,+17]  LLR  +2.56  pLLR  +2.89  max-moves
G4225 1197W 1911D 1117L   50.9%  elo   +6.6 [-4,+17]  LLR  +2.52  pLLR  +2.89  adj-win
G4226 1197W 1912D 1117L   50.9%  elo   +6.6 [-4,+17]  LLR  +2.52  pLLR  +2.85  adj-draw
```

Outcome: **T1e KEPT** — first Batch-2 keep (certified base `c514a2b`). Time-management package is now T1b
(stability scaling) + T1e (node-fraction effort scaling), composed multiplicatively under the T1b-proven
[0.72, 1.40] soft-budget clamp; projection / hard ceiling / emergency floor untouched (0/0 flag-outs confirms
no raised flag risk). Next box item: T5 (aspiration).
