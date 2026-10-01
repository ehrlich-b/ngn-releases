# Mate-stop correction: Windows continuation after user-stopped Mac run

Prospective protocol, before any new Windows A/A or candidate game. The user
explicitly prohibited all NGN execution on the Mac because it is on battery.
All Mac workers, collector and automatic resumption processes were killed.
The incomplete Mac candidate run has no verdict and is not pooled or used to
select this sample. This hardware restriction supersedes the earlier unlaunched
Windows-plan prohibition; the move is required by the user, not selected from a
favorable partial score. No partial candidate strength estimate was calculated
for this decision. All raw Mac evidence is retained.

The correctness candidate and its proved witnesses are unchanged from
[the original mate-stop record](2026-09-05-mate-stop-confirmation.md). The
current playing base remains5cf301a. There is no NNUE or evaluation change.

## Prerequisites and exact assets

Finish the already-running Windows absolute pin `r0905acceptedpin` first,
preserve all400 games, and complete its original report plus legal/terminal
validation on WSL. A failed pin-integrity audit holds the goal rating claim;
root must inspect it before moving on. No competing Windows/WSL engine work
is allowed while that timed pin is active. The Mac is for editing and lightweight
inspection only, including after it is plugged in unless the user changes the
restriction explicitly.

Then run the already-staged `verify_mateguard_startup.ps1 -Phase aa` in the
native sprt directory. It checks unused run name, Windows/WSL idle state,
asset hashes, both NGNs' Hash128 and14 T8 defaults, three canonical UCI
regressions and clean quits, plus native corrected-harness terminal tests.
No earlier A/A with different platform or score adjudication substitutes.

| Asset in C:\Users\ehrli\ngn\sprt | SHA-256 |
|---|---|
|ngn_20260905_t8qcap.exe|43b2a2c93d9a773b0a08d8017d611edb7b39d63efe071cb7366c1ac4c11f92fc|
|ngn_20260905_mateguard.exe|2c29f19652ce0bfa2d068af093e83c4eeca4f5f1907bc5dba7475162439a3c33|
|sprt_20260905_terminal.exe|62e6ffd96d57558938d45eaf0a123ee57dff1a7dec596a768738a0680b262e2a|
|terminal_rules_20260905.test.exe|99cbbb9a258f41963689fa4460212ed09886b5745e4f94ebee72a6b9a36085b7|
|verify_mateguard_startup.ps1|f7aaf3126a92a253c366485a6543b8003d0ba13b9ffcb23be342d7a151ebf9df|
|sprt_openings.txt|974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222|

## Fixed control and candidate

Native9800X3D,10+0.1 real clocks, concurrency8, default scheduling,
lowpower=false,200-ply cap. Score adjudication stays disabled to expose
false-mate conversion failures. Both runs are fixed samples with no SPRT
stopping, no extension and no pooling.

Run `r0905terminalaa`,200 games/100 pairs of identical accepted T8:

```sh
BOXSPRT_BIN=sprt_20260905_terminal.exe scripts/boxsprt.sh launch r0905terminalaa -new '.\ngn_20260905_t8qcap.exe' -base '.\ngn_20260905_t8qcap.exe' -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -maxmoves 200 -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 200 -mingames 201 -resignscore 0 -drawscore 0
```

Require DONE_EXIT_0, exactly200 games, correct cumulative WDL/pentamomial,
zero flags/operational errors, and paired95%CI containing0. A failed control
holds the candidate; no favorable-repeat sampling.

After a passed control and `verify_mateguard_startup.ps1 -Phase candidate`,
run `r0905mateguard`,400 games/200 pairs against immediate accepted T8:

```sh
BOXSPRT_BIN=sprt_20260905_terminal.exe scripts/boxsprt.sh launch r0905mateguard -new '.\ngn_20260905_mateguard.exe' -base '.\ngn_20260905_t8qcap.exe' -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -maxmoves 200 -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 400 -mingames 401 -resignscore 0 -drawscore 0
```

Correctness acceptance requires the original mechanism/witness checks, all400
completed games, one terminal DONE_EXIT_0, reconciled WDL/pentamomial, zero
flags, crashes, illegal/missing moves, disconnects or watchdogs. A paired95%
interval wholly below0 rejects this implementation pending investigation.
Otherwise an inconclusive result may support the proved correction, without
an H1 or Elo-gain claim. Preserve all output and errors. This log has no moves
or pair identities; do not claim independent PGN replay from its pair totals.

The native control completed and passed root review; its evidence is below.
Record candidate startup/launch evidence and final interpretation. Any further
absolute-rating claim requires the original goal's valid external evidence;
self-play estimates cannot be added to an old rating to claim2800.

## Completion auditor prepared before launch

`output/remote-pin-20260905/audit_windows_mategate.py`, SHA-256
`48c0e63b9b0d5d8f5aed31afadc4a57d5f6175fda456eef410f195282752f1a1`,
was reviewed against the corrected native harness and launcher. Root reproduced
17 lightweight log-fixture checks: two valid synthetic formats and15 malformed
cases, including native LF/CRLF framing, unexpected stderr, missing/corrupt
game rows, footer, flags, secondary summaries and wrong sample/verdict.
These fixtures transform only the completed Mac A/A as source text; they are
explicitly synthetic Windows format and provide no native playing evidence.
No engine or build runs during these parser checks.

The parser requires exact protocol identities, every cumulative WDL/score row,
fixed completion, reconciled penta/paired CI and zero operational failures.
Secondary trinomial summaries are required as numeric lines but do not decide
the verdict. It refuses Python optimization that would disable assertions.
Reproducible fixtures and review are retained beside the parser.

The file is copied to native repin as `root_audit_windows_mategate_20260905.py`;
reverify its hash before use. On WSL, completed native logs can be audited with:

```text
python3 /mnt/c/Users/ehrli/ngn/repin/root_audit_windows_mategate_20260905.py /mnt/c/Users/ehrli/ngn/sprt/r0905terminalaa_out.txt ngn_20260905_t8qcap.exe -3 3 200 200
python3 /mnt/c/Users/ehrli/ngn/repin/root_audit_windows_mategate_20260905.py /mnt/c/Users/ehrli/ngn/sprt/r0905mateguard_out.txt ngn_20260905_mateguard.exe -3 3 400 400
```

Parser completion alone does not accept either gate: root applies the A/A
CI-containing-zero rule or the candidate's predeclared correctness rule above.

## Direct regression coverage follow-up

The original Go fixture proves the history-free mating score and the actual
draw continuation, but passes with either mate-stop implementation. The archived
carried-state UCI probes supply the existing runtime before/after evidence.
To add direct automatic coverage, Sol prepared a test calling real iterative
deepening with the recorded game46 history and a synthetic exact child TT entry
encoding the archived root mate15 score. Root required the test to verify the
legal `e5e8 g4g5` threefold first and explicitly assert that its first completed
iteration reproduces the intended cached move/score. It then requires a later
completed iteration and a different final move. The injected cache is a targeted
test setup; it does not reproduce the earlier warm process's full TT contents.

The test and rationale are in `output/mate-history-20260905/regression-followup/`.
Test SHA-256: `e0a61d86108420d95c1f2f6b6edc58ab498f1a09bd4d7a323fb54a03a0e28b34`.
Source-only archive SHA-256:
`a2b844e3a35acb83db9413dfc5dd4612d372b5e7242f1079c7adfc79db27220c`.
It is staged under `/home/ehrli/repos/ngn/output/mate-stop-direct-regression-20260905/`,
with separate `base/` and `guard/` source copies and identical tests. The
playing-source difference remains only the exact frozen mate-stop hunk.

Runtime red/green is **proved on WSL**, after the original pin and validation
ended and both Windows and WSL were idle. All isolated source hashes passed.
With Go1.25.5, `GOTOOLCHAIN=local GOMAXPROCS=2`, root ran
`go test -short -p 2 ./engine -run '^TestIterativeDeepeningDoesNotStopOnShallowCachedMateWithHistory$' -count=1 -timeout=90s -v`
sequentially in guard and base. The guard passed (exit0); the base failed
(exit1) specifically with `callbacks=1 finalDepth=1`. Both stderr logs were
empty. `regression-followup/direct-regression-results.json` records commands
and raw-stream hashes. No local compile, engine or chess test was performed.
The frozen candidate archive, binaries and predeclared native game samples
are unchanged by this additional test preparation.

## Native control completed

`r0905terminalaa` launched September5 at10:11:16 local on the authorized native
Windows box, using the exact command above. Startup verified both engines'
Hash128/defaults and all three canonical UCI cases, plus all three native
terminal-regression tests. The process-only PowerShell execution-policy bypass
did not change host policy. Startup output is retained as `aa-startup.stdout`.

The control completed in13m28s, DONE_EXIT_0, exactly200 games/100 pairs:
36W/126D/38L, penta `[4,24,43,28,1]`, paired−3.4745
[−32.4516,+25.4543], zero flags and operational errors. The predeclared parser
passed on WSL with exit0 and empty stderr; parser, baseline, harness and opening
hashes matched. The harness and all engine workers drained. Root inspected the
completed output and audit and accepts this control: its paired interval
contains0. No candidate strength conclusion follows from this A/A.

Raw log SHA-256:
`4b58a44b9ea77996a1b759335eccfbf84f8375079d2c63c904afd77e74624072`.
All control evidence is in `output/mate-history-20260905/windows-gate/`.

## Native candidate launched

Candidate-phase startup passed with exit0 and empty stderr: both exact engines,
all defaults, all three canonical UCI cases and the three native terminal tests.
Root independently checked Windows and WSL for competing engine/build workers
and verified the startup script's hash before execution. Evidence is retained
as `windows-gate/candidate-startup.*`.

`r0905mateguard` launched at10:37:56 local September5, PID9284, using the exact
fixed400 command above. The scheduled task entry was deleted after launch.
At14:39:12UTC root verified its exact batch, header and16 child processes
(eight candidate, eight base); `candidate-launch-inspection.stdout` retains the
receipt. This run has since completed; root's verdict is below. No other engine
work competed on the box during the comparison.

## Completed candidate: accepted correction

The exact400-game candidate completed in26m0s with DONE_EXIT_0:
91W/242D/67L, penta `[1,32,115,46,6]` over200 pairs. Paired estimate
**+20.8712 [+3.5584,+38.2883]**, pLLR1.60766; zero flags and operational errors.
The fixed-sample harness prints INCONCLUSIVE because there is no sequential
H1 crossing; no H1 is claimed. The predeclared400-game correctness rule applies.

Root inspected the completed log, prescribed WSL parser output (exit0, empty
stderr), all five pinned hashes and native/WSL drain receipts. The direct
old-fail/new-pass regression and carried-state witnesses prove the mechanism;
the complete valid sample has no regression signal. **Root accepts this
correction under the original correctness rule.** Its favorable paired estimate
is separate from the external2800 goal and must not be added to an old rating.

Raw log SHA-256:
`79ead559be0bbc8ade59ed0f9d1967c6d13fa9c40c754a2b5f71510558c26972`.
Evidence is in `output/mate-history-20260905/windows-gate/candidate/`.
All games remain separate from the interrupted Mac run and completed controls.
The log reconciles pair totals but does not contain moves or individual pair
identities; independent PGN replay is not claimed for this self-play gate.

The exact guard and both regression files are now copied into main alongside
the already committed terminal harness correction. All162 staged source/data
files match main. Full short/race/vet and independent oracle validation all
passed on the idle WSL box, Go1.25.5, GOTOOLCHAIN=local, GOMAXPROCS2 and-p2.
The oracle actually ran and passed; every command exited0 with empty stderr.
Root fetched and inspected the raw logs in `integration/validation/`.
Validation archive SHA-256:
`70c2cc112c2e262925c64927b110864c611db1f203bbcdd5e1d482aaf64c674d`.
No Mac engine/build/test execution occurred. Integration is accepted for
commit/deployment; the new external rating is still unlaunched.
