# T17 + completed T8 vector, rebuilt on accepted qcap

**Completed H1; integrated and deployed on Mac/WSL/Windows.** This continues the previously authorized
[interrupted confirmation](2026-09-04-t8-confirmation.md), with its fixed
[24,000-game vector](2026-08-04-t8-spsa-result.md). No new tuning or heuristic
is included. No speculative source or playing defaults changed on main.

The preceding `r0905texelv2` completed1,600 games and was shelved under its
original inconclusive-cap rule: paired+3.04 [−9.47,+15.56], zero errors/flags.
Qcap remains the accepted immediate base, so these exact artifacts and the
completed same-configuration A/A remain applicable. This candidate is not an
absolute2800 rating confirmation.

## Scope and artifacts

Isolated source `output/t8-qcap-confirm-20260905`, parent `1387fd6`. Its diff
changes only `engine/search.go` and the existing T17 regression in
`engine/reentrancy_test.go`. The search change seeds the improving reference,
handles its unknown sentinel and applies the exact nine converged defaults plus
their UCI registry defaults. All qcap/draw/castling and offline changes are
inherited. The evaluation model and coefficients remain the accepted baseline.

Raw artifact directory `output/recovery-2026-09-04/`:

- `t8-qcap-candidate.diff` SHA-256
  `d3c20857c7bfa308eb9f11471a0b6d4d5e31bdeacd78097c4c7266ac8cdfc62c`.
- Mac `build/ngn_20260905_t8qcap`:
  `0a42c12d287269251946f4bcbd016a497f200965a445b9806c22d57a982b4345`.
- Windows `build/ngn_20260905_t8qcap.exe`:
  `43b2a2c93d9a773b0a08d8017d611edb7b39d63efe071cb7366c1ac4c11f92fc`.
- Accepted Windows base `ngn_20260904_qcap.exe`:
  `bc84fe0d298a32ff644775b2892ee43356bd94f9e22b6a527dc99cdbb29556f2`.

Go1.26.2, Mac arm64 and cross-compiled Windows amd64/v3. Root reviewed the
diff and logs; full short/race/vet pass. Two fresh-process fixed-depth runs
agree exactly on score/nodes/PV/bestmove: Kiwipete d12=426957/−90/e2a6,
middlegame d12=227604/+6/d4c5, rook ending d16=853016/+103/b4f4. These differ
from the base and establish repeatability only. All14 UCI defaults match the
vector; black EP evasion, forbidden castling, fifty-move mate and clean quit
smoke checks pass on Mac. Root also ran the independent SF oracle:2293 positions,
48926 depth-2 root divides and128 internal search roots pass with state restoration.
Logs are `t8-qcap-{short,race,vet,oracle}.txt`,
`t8-qcap-identity.json`, and `t8-qcap-release-smoke.txt`.

Root pushed the Windows executable to the native SPRT directory and verified
its remote SHA-256. It is staged alongside the installed engine; no default
artifact was replaced and no game process for this candidate was started.

## Frozen game rule for these qcap-based artifacts

Retain the original interaction confirmation rule: paired SPRT **[−3,+3]**,
alpha/beta .05, minimum200 games, maximum1600. Accept only completed H1 and
zero operational failures/flags. H0 rejects; capped inconclusive shelves. No
extension or provisional keep on score. Do not pool the interrupted279 games.

Native Windows Ryzen9800X3D, concurrency8, default affinity, lowpower=false,
10+0.1. Harness `sprt_20260904.exe`:
`63ef86d9b986fdb2c260782149af259e278845d89595d010df3970e6698cc9ed`.
Opening file `sprt_openings.txt`,5000 lines:
`974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222`.

```text
BOXSPRT_BIN=sprt_20260904.exe scripts/boxsprt.sh launch r0905t8qcap -- -new '.\ngn_20260905_t8qcap.exe' -base '.\ngn_20260904_qcap.exe' -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 1600 -mingames 200 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
```

Launch only after
root review of the preceding completed match, verification of immediate base,
all remote hashes and idle native chess workers. Run the native candidate UCI
smoke then, before games; it is not run alongside the active timed match.
The completed same-qcap A/A
`r0905qbaseaa` used precisely these settings and passed; reuse is valid only
while baseline/harness/machine configuration remains unchanged. A new accepted
base needs its corresponding A/A control. No autonomous handoff currently
launches this T8 candidate.

## Launch verification

Root reviewed the completed fit verdict, confirmed idle native workers, and
reverified all four remote candidate/base/harness/book hashes above. Native
Windows smoke checked all14 advertised defaults, black EP evasion, forbidden
castling, mate at the fifty-move boundary, and clean quits. All passed; log
`output/recovery-2026-09-04/t8-qcap-windows-smoke.txt`. The script itself checks
idle workers and candidate identity and bounds each engine output read.

The exact frozen command launched at02:58:59 local September5. Root observed
one expected versioned harness and eight workers for each of the two frozen
binaries, with completed games flowing. No defaults were replaced.

`output/recovery-2026-09-04/watch_r0905t8qcap.py` is a bounded collector: it
checks once a minute, fetches the completed log and records remaining processes.
It never launches another run, selects a verdict, commits or deploys. State is
`t8-qcap-games-controller.json`. Root reviews any acceptance before integration.
This is the only active native match. The prepared singular-only candidate
follows this decision and must rebuild if T8 changes the accepted base.

The [conditional successor controller](2026-09-05-singular-only-confirmation.md#conditional-unchanged-base-handoff)
may audit a completed, clean T8 H0/cap result and launch the already frozen
singular-only candidate after all unchanged-base prerequisites pass. It halts
on H1 or any failed prerequisite. This automation changes neither the original
game rule nor the installed engine, and does not extend or restart T8.

## Completed result and accepted source integration

`r0905t8qcap` completed734 games in46m20s: **240W/334D/160L**,
penta12/70/144/108/33 over367 pairs. Paired **+38.0187 Elo
[+20.7712,+55.4549]**, pLLR+2.977535. The first reported boundary crossing was
G726 (+2.95); already assigned games drained to734. The frozen harness reports
**H1 ACCEPTED** and a single terminal `DONE_EXIT_0`. Zero flag-outs or operational
failures;400 adjudicated wins,184 adjudicated draws,77 rule draws,72 max-move
draws and one stalemate. Root independently recomputed the paired statistics and
reviewed the settings, complete audit and idle native process list.

Raw log SHA-256:
`4ca1adc3c4b18f4171359a1ea8897b6a8078b8ab8b51b5bac1d6df3c2cd42888`.
Raw/audit files are `output/recovery-2026-09-04/r0905t8qcap_out.txt` and
`r0905t8qcap-audit.json`. No interrupted games were pooled.

**Accept the T17+T8 interaction as tested.** This is a relative short-TC gain,
not an absolute2800 result and not separate estimates for T17 and each margin.
The conditional controller stopped on H1 without launching the old qcap-based
singular-only candidate. That candidate must now rebuild on T8 and use a fresh
baseline A/A before its existing game gate.

Root applied the exact reviewed patch. Every tracked Go/module file equals the
game-tested isolated T8 source. Full main short/race/vet and independent oracle
pass (2293 positions,48926 divides,128 internal search roots); the release smoke
verifies all14 defaults and EP/castling/fifty-move-mate behavior. Initial main
short/vet failures came solely from old mutually exclusive diagnostic `.go`
files being discovered beneath `output/depth-policy-20260905`. An output-only
module boundary preserves all frozen bytes/paths while excluding those raw
artifacts from the parent package walk. Full checks then passed; both initial
failure and corrected logs are retained as `t8-integrated-*`.

For WSL deployment, root cross-built the same accepted source with Go1.26.2,
Linux amd64/v3, CGO disabled. `build/ngn_20260905_t8qcap_linux` SHA-256:
`4478de8c648d9dceac19ce9b517dc18b90ffeda27bdaf43706beceff5e7c327c`.
Native WSL short/race checks and staged/installed smoke pass. Root promoted the
accepted defaults on all three platforms, preserving rollback; Windows and Mac
retain the exact game-tested hashes above. Source is committed at5cf301a.
[Paths, final hashes and deployment logs](2026-09-04-deployment.md).
The [fresh T8 A/A and rebuilt singular-only gate](2026-09-05-singular-t8-confirmation.md)
now follow this completed release.
