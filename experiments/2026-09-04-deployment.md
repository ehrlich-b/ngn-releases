# Current accepted mate-stop deployment

Deployed September5: playing source **53e4d1b9bf4006a39c0327a7016faba7f2b8f82e**,
accepted T17+T8 plus the proved mate-stop correction. Fixed400-game native
comparison91/242/67, paired+20.87 [+3.56,+38.29], zero flags/errors; accepted
under the original correctness rule. The corrected400-game external run is now
fully validated at **2884 [2834,2934]**, clearing the original2800 lower-bound
goal. [Completed result and integrity audits](2026-09-05-2800-result.md).
[Acceptance and integration evidence](2026-09-05-mate-stop-windows-recovery.md).

| Installed engine | SHA-256 |
|---|---|
|WSL Linux amd64/v3|0c067627404ab67a5f4a57fef63be024fd84e9248423286ef3dfc103041d266e|
|Windows amd64/v3|2c29f19652ce0bfa2d068af093e83c4eeca4f5f1907bc5dba7475162439a3c33|

WSL `/home/ehrli/repos/ngn` fast-forwarded cleanly from5cf301a to53e4d1b.
All162 source/data files matched the isolated integration package, whose full
short/race/vet and independent oracle passed on WSL Go1.25.5. The oracle compared
2293 positions,48926 root moves and128 raw search roots. The exact frozen
Go1.26.2/CGO0 binaries were installed as WSL `build/ngn` / `build/ngn.exe` and
Windows `C:\Users\ehrli\ngn\ngn.exe` / `repin\ngn.exe`. Installed Linux and
both Windows engine paths pass Hash128, all14 defaults, black EP, illegal-castle
rejection, fifty-move mate and clean-quit checks.

Corrected Windows harness defaults now use the terminal-precedence repair:
`ngn\sprt.exe` and `ngn\sprt\sprt.exe`, SHA62e6ffd96d57558938d45eaf0a123ee57dff1a7dec596a768738a0680b262e2a;
`ngn\repin\gauntlet.exe`, SHA499946e16f9758f2dd52d9760e1752565d98438b221c24bb451e46cd8a58a247.
All replaced defaults retain `.pre-mateguard-20260905` rollback siblings; WSL
also retains `backup/pre-mateguard-20260905`. Source, binary and post-install
hash checks pass. The user-prohibited Mac engine execution was not performed;
Mac executable defaults were not replaced.

Reviewed scripts, exact bundle/file hashes and raw deployment/check outputs:
`output/mate-history-20260905/integration/`; final Windows installed/startup
checks: `output/corrected-pin-20260905/final-startup.*`.
The [corrected external rating protocol](2026-09-05-corrected-mate-pin.md)
now measures this accepted release separately from its self-play comparison.

## Prior T17+T8 deployment

**Deployed September5:** accepted T17 plus the completed T8 vector, source
`5cf301a870f6cd9218d8085166993f7ad5364aee`. Its completed734-game gate reached
H1, paired+38.02 [+20.77,+55.45], with zero flags/errors. No fitted evaluation
weights or singular-only policy are installed. The latest absolute pin is still
the earlier2726 [2668,2784]; the relative gain does not establish2800.
[Completed game and integration evidence](2026-09-05-t8-qcap-confirmation.md).

| Installed platform | SHA-256 |
|---|---|
| Linux amd64/v3 | `4478de8c648d9dceac19ce9b517dc18b90ffeda27bdaf43706beceff5e7c327c` |
| Windows amd64/v3 | `43b2a2c93d9a773b0a08d8017d611edb7b39d63efe071cb7366c1ac4c11f92fc` |
| Mac arm64 | `0a42c12d287269251946f4bcbd016a497f200965a445b9806c22d57a982b4345` |

Paths: Mac and WSL `build/ngn` plus `build/ngn.exe`; Windows
`C:\Users\ehrli\ngn\ngn.exe` and `repin\ngn.exe`. WSL was clean atbf5c6b9
and fast-forwarded from a verified incremental bundle to5cf301a. Native WSL
short/race suites pass; staged and installed Linux smoke checks pass. Installed
Mac and native Windows smoke checks also pass, including all14 T8 defaults,
EP/castling/fifty-move-mate behavior and clean quits. Windows/Mac retain the exact
game-tested hashes; Linux is cross-built from identical source with Go1.26.2/v3.

Rollback copies use `.pre-t8-20260905`; WSL also retains
`backup/pre-t8-20260905`. The scripts require a clean fast-forward checkout,
idle native workers for Windows promotion, and exact hashes before replacing
defaults. All WSL/native smoke work finished before the new T8 A/A launch.

Logs/scripts under `output/recovery-2026-09-04/`: `t8-wsl-deploy.txt`,
`t8-windows-deploy.txt`, `t8-windows-installed-smoke.txt`,
`t8-mac-installed-smoke.txt`, `deploy_t8_wsl.sh`, `activate_t8_windows.ps1` and
the T8 verification scripts. WSL reported `DEPLOYMENT_VERIFIED 5cf301a...` and
Windows reported `WINDOWS_T8_DEFAULT_VERIFIED`.

## Prior qsearch correctness deployment

**Deployed September 5:** accepted uncapped production qsearch, runtime patch
committed at `ec9d264`. Fixed 400-game gate: 106/184/110 W/D/L, paired
−3.47 [−27.36,+20.38], zero flags/errors; accepted as a proved correction with
inconclusive strength. Absolute rating remains **2726 [2668,2784]**.
[Exact acceptance, artifacts and A/A investigation](2026-09-04-qsearch-budget-repair.md).

| Installed platform | SHA-256 |
|---|---|
| Linux amd64/v3 | `372c896b7c8ed23454d99e7315e4a960d7030131293cc3a5fe54314f75342867` |
| Windows amd64/v3 | `bc84fe0d298a32ff644775b2892ee43356bd94f9e22b6a527dc99cdbb29556f2` |
| Mac arm64 | `24d71a1ac381320d2ab790b1a88a7fdb30761bcedf573f9b98973b5a1e7eb702` |

WSL source fast-forwarded cleanly to `bf5c6b9`, including the offline classical
model/corpus implementation but no changed evaluation weights. Native Go1.25.5
full short and short/race suites pass (engine 6.287s / 24.270s). Frozen Linux
and Windows artifacts were hash-checked before installation. Installed Linux,
Windows and Mac binaries pass EP evasion, castle-out-of-check rejection,
fifty-move mate precedence and clean-quit smoke checks.

Defaults: WSL `build/ngn` / `build/ngn.exe`; Windows `ngn/ngn.exe` and
`ngn/repin/ngn.exe`; Mac `build/ngn` / `build/ngn.exe`. All replaced defaults
have `.pre-qcap-20260905` rollback siblings; WSL also retains branch
`backup/pre-qcap-20260905`. Earlier backups remain. No classical tuned vector
or T8 vector was installed. Native chess workers were verified idle after
installation; corpus generation runs only on the Mac.

Logs/scripts in `output/recovery-2026-09-04/`: `qcap-wsl-deploy.txt`,
`qcap-windows-deploy.txt`, `qcap-{windows,mac}-smoke.txt`,
`deploy_qcap_wsl.sh`, `activate_qcap_windows.ps1`, and `verify_qcap_*`.

# Earlier castling correctness deployment

**Deployed playing source: `70eb527`**, the proved internal-castling/root-terminal
repair. The completed 400-game gate passed its predeclared correctness rule:
99/205/96 W/D/L, paired +2.6 [−20,+25], zero flags/errors. Strength is inconclusive;
the last absolute pin remains **2726 [2668,2784]**. See the
[full acceptance record](2026-09-04-castling-repair.md).

| Installed platform | SHA-256 |
|---|---|
| Linux amd64/v3 | `80b7488a16ad66f80ff0c618b64baa6445d4c24cdedb78e5d2e87c3a57d32b3e` |
| Windows amd64/v3 | `131b9bbf1cb064bd6c1edd5513db8e6c670eba154231f3aadd66cc1942a85e7d` |
| Mac arm64 | `1010178789b5560583c0ee430934dc4e12473a7dbbdbf07598a30bb3185a3a0f` |

WSL `/home/ehrli/repos/ngn` fast-forwarded cleanly from `71399c0` to `e02be60`
(runtime `70eb527` plus offline tuning repairs and review/acceptance records).
Defaults are WSL `build/ngn` and `build/ngn.exe`, Windows
`C:\Users\ehrli\ngn\ngn.exe` and `ngn\repin\ngn.exe`, and Mac `build/ngn` /
`build/ngn.exe`. The frozen native Windows game-tested binary was installed
unchanged. The existing corrected harness binaries remain installed.

Native WSL Go1.25.5 full short and short/race suites passed (engine packages
5.835s and 23.765s). Installed Linux, Windows and Mac binaries passed black EP
evasion, illegal-castle rejection (Kf1), and fifty-move mate-priority UCI checks,
including clean quit. Native test workers exited before deployment and remain idle.

Rollback: WSL branch `backup/pre-castle-20260904`; all replaced defaults have
`.pre-castle-20260904` siblings. The earlier `.pre-repair-20260904` backups remain.
Logs and reviewed installation scripts are `output/recovery-2026-09-04/castle-*`,
`deploy_castle_wsl.sh`, `activate_castle_windows.ps1` and verification scripts.
No qsearch prototype or T8 vector is installed.

The earlier deployment below is retained as historical provenance.

# Earlier September correctness deployment

Accepted source commit: **`71399c063ee9688f499e71e1253ebaf81b211813`**.
The committed runtime diff matches the exact tested patch SHA-256 in the
[validation record](2026-09-04-fast-correctness.md). The short fixed-cap check
passed at +23.5 [+1,+46] over 400 games, with zero errors/flags. The separate
absolute gauntlet has now completed at **2726 [2668,2784]**, clearing the
original 2600 gate. [Full result and limitations](2026-09-04-strength-check.md).
Subsequent offline tuning repairs do not change these frozen playing artifacts.

## Installed artifacts

| Platform | SHA-256 |
|---|---|
| Linux amd64/v3 | `5f3b6078ba62afb86cc4f95ead318f64b5adf04c2fed8c2680fdc63c3ee423e0` |
| Windows amd64/v3 | `7b5af06a4a0ff02a0d9c51aa5eb041dd8ddf9935bbe4691d65dd76146a515828` |
| Mac arm64 | `d8f99879b13d9a9a512865f0f918a422f6e505cab29cb35e9b2b0871f64d7b8b` |

All are the frozen Go1.26.2 artifacts built before the source commit. Go VCS
metadata therefore names `b378c43+dirty`; verify this release using the hashes
and source-patch mapping, not that metadata alone.

- WSL source: `/home/ehrli/repos/ngn`, fast-forwarded from `7587eb6` to `71399c0`.
  Default binaries: `build/ngn` (Linux), `build/ngn.exe` (Windows).
- Windows default engine: `C:\Users\ehrli\ngn\ngn.exe`; also installed as
  `C:\Users\ehrli\ngn\repin\ngn.exe`.
- Corrected SPRT (`63ef86d9b986fdb2c260782149af259e278845d89595d010df3970e6698cc9ed`)
  installed at `C:\Users\ehrli\ngn\sprt.exe` and `ngn\sprt\sprt.exe`.
- Corrected gauntlet (`5d1b6ca586420c04bffed729f5e48c775105c2fb04b2c267a15bd6b6a6769b9a`)
  installed at `C:\Users\ehrli\ngn\repin\gauntlet.exe`.
- Mac defaults: `/Users/ehrlich/repos/ngn/build/ngn` and `build/ngn.exe`.

The WSL checkout and binaries had been from June 7. The root Windows engine
and SPRT were from June 28. The root `ngn_crashes.log` contained initialization
messages only, not an actual crash record.

## Rollback

WSL retains branch `backup/pre-repair-20260904` and binary copies
`build/ngn.pre-repair-20260904` / `build/ngn.exe.pre-repair-20260904`.
Each replaced Windows executable has a sibling `<filename>.pre-repair-20260904`
when a prior file existed. Mac binaries have the same backup suffix.
The deployment refused a dirty WSL checkout and required a fast-forward.

## Verification

Before activation, WSL Go1.25.5 ran `go test -short ./... -count=1` and
`go test -short -race ./... -count=1`: both passed. The race suite's engine
package took 24.077s; the shared UCI package took 1.016s.

UCI tests against the installed Linux, Windows and Mac defaults passed:

- Sole black en-passant evasion returns `bestmove e4d3`.
- Mate on the fiftieth move returns `bestmove a7a8`, score mate 1.
- Engines answer the handshake and exit cleanly on `quit`.

The WSL install reported `DEPLOYMENT_VERIFIED 71399c063ee9688f499e71e1253ebaf81b211813`;
the Windows default reported `WINDOWS_DEFAULT_ENGINE_VERIFIED`.
Native match workers had exited before deployment. All build/test/smoke work
finished before launching the subsequent absolute rating games.

Logs and deployment scripts: `output/recovery-2026-09-04/`:
`wsl-deploy.txt`, `windows-deploy.txt`, `windows-smoke.txt`, `deploy_wsl.sh`,
`activate_windows.ps1`, and the UCI verification scripts.
