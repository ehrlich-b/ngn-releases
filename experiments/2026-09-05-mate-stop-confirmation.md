# Cached-mate termination correction on accepted T8

**USER-STOPPED: Mac candidate interrupted; no candidate verdict.**
The user reported the Mac spiraling and explicitly instructed never to run NGN
on this battery-powered computer. Root stopped and verified absent all eight
engine workers, harness33210, controller14811, deferred pin collector52989 and
resumption helper62206. No automatic local collection or validation remains.
The remote Windows rating games were not stopped. Do not restart any Mac job;
all future engine compute and heavy validation belongs on Windows/WSL.
The completed A/A below remains historical evidence. Candidate partial games
cannot support a verdict or be pooled into a later run. Earlier running/deferral
entries below describe the interrupted history and are superseded by this notice.

`r0905terminalaa_mac` completed200 games in26m49s,44/116/40 W/D/L,
penta2/23/47/25/3, paired+6.9496 [−21.1215,+35.1120], zero flags/errors,
empty stderr and terminal DONE_EXIT_0. Root reconciled the raw log and totals,
verified frozen inputs and the predeclared zero-containing confidence interval.
Raw SHA-256: `d3c64d22a3eb0821763a08c479f0adf0ac15392c668d9aef1f737e9b78b7c357`.

The one-shot controller then launched `r0905mateguard_mac` at11:46:15UTC
September5. Root checked its exact argv against the prospectively recorded
plan. Candidate result is pending; nothing is integrated or deployed.
Controller PID14811/session19640 remains live, with artifacts under
`output/mate-history-20260905/mac-gate/`. The protocol was committed before
launch at62c4fc1. All six initial background-UCI cases passed, including
Hash128 and14 T8 defaults. No additional local chess compute is permitted
until the candidate finishes.
Before any A/A or candidate game, the still-unlaunched Windows plan is
prospectively superseded by the Mac protocol below. Windows staged assets are
retained for provenance; they do not authorize a second favorable-result attempt.
The ongoing Windows absolute rating pin retains its frozen inputs and CPU.
The [draw-conversion audit](2026-09-05-draw-conversion-audit.md) reproduces two
current-engine choices that allow immediate threefold while claiming mate13/15
at depth1. This candidate replaces unconditional mate stopping with Counter3.8's
completed-depth requirement: mate distance plus five plies. It changes no
evaluation weights, TT cutoff, history bookkeeping, qsearch move classes or
other search parameters. It is not a general solution to history-dependent TT
bounds. Quiet promotions remain a separate candidate.

## Source and validated behavior

Root compared all158 tracked Go/module files against main71163ab: only
`engine/search.go` differs. The playing base is accepted5cf301a. One added
`engine/mate_history_test.go` verifies the historical fixture and its draw;
the old/new carried-state UCI probes provide actual failing/passing behavior.
The isolated directory is `output/mate-history-20260905/`.

Both reproduced bad moves change under the guard:

- Game46: `e5e8` at depth1/mate15 becomes `e5f4` at depth10/mate3.
- Game118: `e2f2` at depth1/mate13 becomes `d1f1` at depth10/mate3.

Root independently warmed the final candidate through each game's NGN turns,
then continued against Stockfish with400k NGN /100k SF node limits. Both end in
actual checkmate after five plies. Full histories persist and every continuation
move/final board is independently checked. This verifies the concrete mechanism;
two selected continuations are not an Elo estimate.

The original reproduction used Hash64. A follow-up at the live default
**Hash128** reproduces game118's `e2f2` false mate (depth1/mate14), but game46's
baseline choice changes to `e5e3` and does not reproduce its particular immediate
draw. The guard chooses `e5d4` and `d1f1` at depths10/12. Root independently
continued both guarded games with Hash128 and observed actual checkmates after
five plies. Evidence: `default-hash128/results.json`,
`default-hash128/root-conversion-proof.json/log`, and its hash manifest.
Thus one identified immediate-draw witness persists at the actual native default;
do not claim both Hash64 failure choices reproduce at every hash size.

The agent's initial prose interpretation of the Hash128 Stockfish responses
reversed UCI perspective. `mate -2` after NGN's move means the opponent to move
is losing, and does not invalidate the guard. Raw transcripts were preserved;
root's complete played continuations provide the outcome proof.

Full short/race suites and vet pass. Root inspected the logs, checked the source
diff and final hashes, and verified all14 T8 defaults plus the three existing
EP/castling/fifty-move UCI regressions and clean shutdown. The seven guarded
carried-state probes avoid all seven particular archived draw moves.

Frozen local candidate artifacts:

| Artifact | SHA-256 |
|---|---|
|Patch including fixture test|`b26fdd92a5aec0b9c37a8e74ab9c7762403076aee666d6cec556100299d23def`|
|Candidate search.go|`1e504c68b18872a40ab88fe44b7a6e1aa42286a6709d0222bb090ff0c7d90d3e`|
|Source Go/module hash manifest|`c5e54b31d28dc2da69ccca3f69f55c8c230b4065bebdc073d7f901d112ed3edc`|
|Mac `build/ngn_20260905_mateguard`|`d23dc283618cae5950c7ea4ed2d7f86c51239f623aadc48daeead6130b741b15`|
|Windows `build/ngn_20260905_mateguard.exe`|`2c29f19652ce0bfa2d068af093e83c4eeca4f5f1907bc5dba7475162439a3c33`|
|Linux `build/ngn_20260905_mateguard_linux`|`0c067627404ab67a5f4a57fef63be024fd84e9248423286ef3dfc103041d266e`|

Go1.26.2, CGO0, Mac arm64 and Windows/Linux amd64/v3. The Go VCS build stamp
reflects the containing checkout; use the explicit source hashes/diff as
candidate provenance. Root evidence is in `root-evidence-sha256.json`,
`root-conversion-proof.json/log` and `root-protocol.json/log`.

## Prospectively revised game instrument: Mac

Revised before either test starts, September5. The Mac is available while the
Windows `r0905acceptedpin` measurement continues. This moves the whole candidate
gate to one machine; it does not mix machine classes or add a second trial.
Windows/WSL engine compute stays exclusive to the rating pin. The Mac result
cannot establish an absolute rating or be pooled with Windows games.

Mac16,1 (M4), Darwin24.6.0 arm64,24GiB,4 performance and6 efficiency cores.
Use10+0.1 real clocks, concurrency4, `lowpower=true` (`/usr/sbin/taskpolicy -b`).
This requests background scheduling, not guaranteed hard CPU affinity. Both
sides use the same setting. No other local chess compute while the gate runs.
GOMAXPROCS/GODEBUG are unset. Engines use their verified Hash128/default T8
options, Go1.26.2, CGO0. The Mac terminal-precedence harness is built from the
same independently validated source as the staged Windows instrument.
Mac boundary tests pass, including both-color mate at cap, stalemate, repetition,
fifty-move draw, ordinary cap and terminal precedence over score adjudication.

Frozen assets:

| Asset | SHA-256 |
|---|---|
|Mac accepted `build/ngn`|`0a42c12d287269251946f4bcbd016a497f200965a445b9806c22d57a982b4345`|
|Mac corrected `output/harness-boundary-20260905/build/sprt_20260905_terminal`|`20cfc9c06d035cf3a7e891428fc7e9ad0b85de8d92da6afa0c9c9d6235f32d06`|
|Harness `internal/uci/uci.go`|`2dee9d00fbc0b207d5fe9233530af850a39a0fcca5bab0434aa60788a8c8ad42`|
|Opening book `output/sprt_openings.txt` (5000)|`974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222`|

Candidate hash is the Mac hash in the table above. Versioned Windows candidate,
harness, native unit-test binary and startup verifier remain staged and unrun;
the earlier exact Windows plan is preserved in commit4963acc.

**Score adjudication is disabled.** Resigning on a large evaluation would hide
the failure being tested: the base announces mate and subsequently draws.
Games reach actual terminal/draw rules or the explicit200-ply cap. No playing
engine change is bundled into the corrected harness.

## Fixed A/A followed by fixed candidate gate

Run `r0905terminalaa_mac`:200 games/100 pairs, identical accepted T8 binaries.
Require zero operational errors/flags and paired95%CI containing0. A failed
control holds the candidate for investigation. No favorable repeat or machine
fallback is allowed after seeing results.

Run `r0905mateguard_mac` only after the A/A passes:400 games/200 pairs, guarded
candidate versus immediate accepted T8. Both phases use the same corrected
harness, book, clocks, cap, background policy and four concurrent games.

Exact command construction and one-shot launcher are in
`output/mate-history-20260905/mac-gate/run_gate.py`; each phase records its
complete argv and environment before launch. Shared arguments:

```text
-tc 10+0.1 -concurrency 4 -lowpower=true -openings /Users/ehrlich/repos/ngn/output/sprt_openings.txt -maxmoves 200 -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -resignscore 0 -drawscore 0
```

A/A uses `-maxgames 200 -mingames 201`, candidate uses
`-maxgames 400 -mingames 401`. `-new` is the exact Mac base for A/A and the
exact Mac candidate for the second phase; `-base` is the exact Mac base in both.
The min-game argument disables SPRT stopping. Both must finish the full cap.

Before each phase the launcher verifies frozen asset/script hashes and local
idle state. Initial startup checks both engines under taskpolicy, all14 T8
defaults/Hash128, three canonical UCI regressions and clean quits. Exclusive
startup marker prevents accidental duplicate launches. Failure stops for root
inspection; no retries, integration or remote commands occur in the launcher.
The A/A audit must pass before the launcher starts the candidate. Both runs
preserve stdout and stderr separately; any stderr, game-count/pair-accounting
failure, flags, illegal/missing moves, crash, disconnect or watchdog holds the
result. The auditor accepts only a complete fixed-cap terminal verdict.

Correctness acceptance requires the proven witnesses/checks, all400 completed
games, one terminal DONE_EXIT_0, and independently reconciled cumulative W/D/L,
final pentamomial/paired confidence interval, with zero operational failures.
A paired95% interval wholly below0 rejects this implementation pending
investigation. Otherwise an inconclusive result may support the proved
correction; it does not earn an H1 or Elo-gain claim. No cap extension or pooling.
The existing SPRT log contains no moves/pair IDs, so final pair counts can be
reconciled with WDL but this self-play gate does not claim independent PGN replay.
The corrected instrument's boundary tests supply direct outcome-priority proof.

The337 inputs of the live Windows pin remain unchanged until its collector
finishes. Even if this candidate completes first, root defers main integration
until that freeze ends. Any later external goal confirmation uses a separately
frozen accepted binary and corrected measurement instrument. Do not add a
self-play estimate to2726 or relabel an old pin to manufacture2800.

## Deferred local collection to preserve Mac isolation

At12:29:51UTC root paused only local pin collector PID52989 with SIGSTOP.
The collector would otherwise launch local bootstrap/Stockfish replay when the
Windows games finish, creating a scheduling race with the Mac candidate.
No fetched PGN, rating report or legal replay existed before deferral; the
local validation had not begun. Windows games continue unchanged,285/400
at the post-deferral check. Mac controller PID14811 remains running.

One-shot helper `mac-gate/defer_pin_collection.py`, SHA-256
`086b1382c47e65a81a317564b4e9f49db60b26cd2822b1d4dfa827cef30eacb6`,
PID62206/session49860, verifies both exact process identities, pauses the
collector, waits for the Mac controller to exit, then verifies and resumes the
same collector with SIGCONT. It never launches games or modifies frozen files.
An observation/identity failure holds for root inspection instead of guessing.
Root verified OS state Ts for the collector and Ss for the Mac controller.
Current helper state is in `mac-gate/collection-deferral.json`.
