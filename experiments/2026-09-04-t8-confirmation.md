# T17 + T8 vector confirmation on the repaired release

Predeclared before launch. This closes the unfinished confirmation step from
the [24,000-game SPSA run](2026-08-04-t8-spsa-result.md); it is not another tune.
The interaction candidate combines the original T17 improving-reference repair
with its converged vector, compared directly with the accepted release.

```yaml
id: 2026-09-04-t8-confirmation
date: 2026-09-04
change_class: tune / search interaction
hypothesis: T17 plus the existing T8 vector improves the repaired release in real-clock games
base_commit: 71399c063ee9688f499e71e1253ebaf81b211813
candidate_commit: e23f3bb plus t8-confirm.patch
candidate_patch_sha256: 50c02d57bd484b5c63293083522187b5eaae4ce7ec81821f795d37ab2ba964a6
base_binary_sha256: 7b5af06a4a0ff02a0d9c51aa5eb041dd8ddf9935bbe4691d65dd76146a515828
candidate_binary_sha256: f8d59a73c1fb1e7713251749f1c64b8100af16d6f4fbcfc091576fb1e720eda4
harness_commit: 71399c0
harness_binary_sha256: 63ef86d9b986fdb2c260782149af259e278845d89595d010df3970e6698cc9ed
command: >-
  sprt_20260904.exe -new .\ngn_20260904_t8confirm.exe -base .\ngn_20260904_fast.exe
  -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt
  -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 1600 -mingames 200
  -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
machine: Ryzen 9800X3D, native Windows, 8 cores / 16 threads, default affinity
go_version: go1.26.2 cross-compiled on darwin/arm64
goarch_goamd64: windows/amd64/v3
tc: 10+0.1
concurrency: 8
openings: output/sprt_openings.txt, 5000 lines
openings_sha256: 974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222
aa_preflight: r0904fastaa, same release binary/configuration, 200 games, penta -13.9 [-44,+16], zero errors/flags, PASS
decision_rule: >-
  Pentanomial SPRT [-3,+3], alpha/beta 0.05, minimum 200 games, cap 1600.
  Keep only on completed H1 and zero errors/flags. H0 rejects. Inconclusive
  at cap shelves; no extension or provisional keep on favorable sign.
games_or_pairs: 279 completed games at cancellation, with unfinished pairs
result: 77 W / 146 D / 56 L snapshot only; pLLR +0.86; not a verdict
flags_errors: no recorded flag/error game before cancellation; final normal integrity summary unavailable
verdict: cancelled without a strength verdict after an independently reproduced illegal root-castling defect
next_action: repair search legality first, then rebuild and predeclare any T8 confirmation on the corrected base
```

## Verification and scope

The source checkout is `output/t8-confirm-20260904`. Frozen patch and verification
logs are in `output/recovery-2026-09-04/`. Main remains free of speculative edits.
T17 was manually rebased around `searchDrawScore`; no release correctness fix was
removed. All fourteen parameter consumers are live reads in search; the nine
changed literals and UCI registry defaults match the converged vector.

Full short suite, full short/race suite, and `go vet ./...` pass. UCI verifies all
fourteen advertised defaults and the black EP / mate-on-fiftieth-move release
regressions. Two fresh-process searches each agree exactly on nodes, score, PV,
and best move: Kiwipete d12 = 303176, middlegame d12 = 329560, rook ending d16 =
853015. These differ from the base and do not establish strength.

Mac inspection binary SHA-256:
`b9f27babdf952538e7235d24120b16ef4040446d101be32e0c986be8be946d24`.
Build commands: `go build -o <binary> .`, with Windows GOOS=windows GOARCH=amd64
GOAMD64=v3. The candidate contains no tuner repair or new evaluation weights.

The same-day `r0904fastaa` preflight uses precisely this TC, concurrency, default
affinity, power option, machine, harness and base binary. No harness or scheduling
change has occurred; that completed preflight is the basis for this run. Its
small sample does not exclude small systematic bias. No earlier games are pooled.

Run name: `r0904t8confirm`, native directory `C:\Users\ehrli\ngn\sprt`.
If queued during the gauntlet, an unattended handoff waits for gauntlet PID 21900,
requires its `DONE_EXIT_0` marker and no remaining match workers, then runs the
exact command above. It must abort if either check fails. The absolute-rating
calculation is independent of this self-play result. A self-play keep is not
evidence that the new 2800 target has been reached.

The handoff is queued and its waiting message was verified. Remote candidate
SHA-256 matches. Launch used `BOXSPRT_BIN='call run_t8_afterpin.bat'` with
`scripts/boxsprt.sh launch r0904t8confirm --` and the exact flags above. The
batch wrapper runs `wait_r0904pin.ps1`, then the frozen SPRT; errors propagate
to the outer `DONE_EXIT_*` marker. Driver files are in the raw artifact directory.

## Cancellation for a newly proved search defect

Before completion, the targeted loss analysis reproduced an illegal root PV:
NGN searches `e1g1` while White is checked by Bh4, then its UCI legality fallback
returns `e2g3`, losing the knight to `h4g3`. This occurs from both the FEN and the
complete game history; Stockfish perft independently confirms only Ng3/Kf1/Kd2
are legal. Repro: `output/recovery-2026-09-04/illegal-castle-repro.txt`.

Stop this unfinished comparison to prioritize the correction and its game gate.
The cancellation is due to the independently proved defect, not the score sign.
There is no completed SPRT verdict, no keep and no lane closure. Preserve the
T17/vector source and hashes, but rebuild on the corrected search before a new
confirmation; these interrupted games must not be pooled with that run.

Cancellation verified: `DONE_EXIT_-1`, 279 games (77/146/56), pLLR +0.86;
`scripts/boxsprt.sh ps` shows no remaining workers. There was no H1/H0 crossing.
