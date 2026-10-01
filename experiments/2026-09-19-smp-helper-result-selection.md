# Lazy SMP completed-helper result selection

Status: **completed / SHELVE** against base `4c69226`. The exact candidate
scored 52.25% in the fixed 400-game gate, but its paired lower bound remained
below zero. The experiment made no accepted source change and was not extended.

## Basis and boundary

Pinned Counter 5.5 publishes a completed search result whenever it is strictly
deeper than the current main line. NGN already runs private helper searches and
joins them cleanly, but historically discards every helper decision and returns
only worker zero's move. The source comparison is recorded in
[the Counter SMP note](2026-09-06-counter-smp-source-comparison.md); it motivates
this experiment but is not evidence that the mechanism helps NGN.

Test only completed-helper result selection:

- every worker retains ordinary depth-one sequential iterative deepening; the
  shelved helper-depth staggering candidate remains absent;
- each helper snapshots only `Depth`, `SelDepth`, `BestMove` and `BestScore`
  immediately after a full iteration commits;
- after worker zero finishes, request helper stops and join every launched
  helper before considering a snapshot;
- a helper may replace the final decision only when its completed depth is
  strictly greater than the primary's completed depth. Equal depth always keeps
  the primary; among helpers tied at the deepest eligible depth, the smallest
  worker index wins deterministically;
- require the selected move to be legal at the unchanged root, reconstruct its
  PV only after all shared-TT writers have joined, and fall back to the primary
  if the reconstructed PV is empty or does not begin with the selected move;
- copy only the decision/report fields. Preserve the primary time manager,
  stopped state, histories and control, and preserve exact aggregate counters;
  and
- if selection succeeds, the coordinator may issue one final callback after
  joining helpers so the last reported PV matches the returned best move. A
  helper never invokes the external callback directly.

Do not add Counter's channel task scheduler, initial-depth staggering, helper
score tie-breaking, evaluator changes, time-management changes or one-thread
behavior changes.

## Required evidence

All engine execution remains restricted to the authorized WSL host while the
user-directed Lean hopper continues running.

1. Unit tests must cover immutable completed-iteration capture, strict-depth
   selection, deterministic helper ties, empty/incomplete and illegal moves,
   legal PV reconstruction, unchanged root state, exact counters and the single
   final coordinator callback. Existing cancellation/panic/join tests must stay
   green.
2. Run focused Lazy-SMP tests, the full short engine suite, full short repository
   suite and full short race suite on WSL.
3. Build exact base and candidate Linux/amd64-v3 binaries from clean isolated
   trees with identical flags and bind the source diff, Rodent model, openings,
   tools and harness sources.
4. Before candidate games, freeze the exact paired real-clock manifest and run a
   fresh same-binary A/A at identical width, clock, CPU mask, concurrency and
   shared-host policy. Its paired 95% interval must include zero and every legal,
   process, effective-width and operational check must pass.
5. Only a completed prospective candidate-vs-immediate-base paired game gate may
   accept the change. Adopt only on a positive paired-bootstrap lower 95% Elo
   bound; otherwise shelve. Do not extend after observing the score or combine
   the result with the rejected depth-stagger experiment.

The fixed gate is expected to retain the previously validated width-eight
profile: both roles on physical CPUs `0,2,4,6,8,10,12,14`, `GOMAXPROCS=8`, no
fastchess affinity split, concurrency one, Rodent V1.1 Anand, paired six-ply
openings and `10+0.1`. Exact game counts, seeds and hashes must be frozen only
after correctness and build gates pass. Shared-host CPU is measured and
disclosed, not a standalone rejection condition. Any positive result would be
local relative playing evidence, not an absolute 3300 claim.

## Frozen implementation and correctness

The candidate was based on exact commit
`4c6922684656209e9f302c455b833cc17c77f6ce` (tree
`da05bd039e3d840af54317c097c851a89ceec004`). Its binary diff, including this
prospective note, was frozen at SHA-256
`983c8bfbc764d06c70880ea27fc16047764838d71739ffaf281a5a510edc664e`.
It implemented only the selection contract above; the shelved depth-stagger
source was absent.

The focused selection/Lazy-SMP tests, full short engine suite, full repository
short suite and full `CGO_ENABLED=1` short race suite all passed on WSL with Go
1.25.5. The tests covered strict-depth and deterministic tie policy,
empty/incomplete and illegal candidates, post-join PV validation, unchanged root
state, exact aggregate counters and one final coordinator callback.

Clean isolated worktrees produced both exact binaries with identical
`GOTOOLCHAIN=local GOFLAGS=-mod=readonly CGO_ENABLED=0 GOOS=linux GOARCH=amd64
GOAMD64=v3 go build -p=1 -trimpath -buildvcs=true -ldflags='-s -w'` commands:

- base: `fc249cb4f6fd30018a61df5671249c37f6ae698aea6e5ea29fbf2b859c0a7d21`;
- candidate: `15a3abae0f951559f9214797b064658489926cf090a1c2149e8b27f304e2094f`.

## Frozen gate

The released manifest was frozen before games at SHA-256
`cd9b5df02b5aa758c826ac97af49783213c378d5903ec154cdf9f7010704f4c3`.
Its held audit verified the driver, 13 frozen harness/input artifacts and both
engine binaries. It bound the exact Rodent Anand model
`5f7480b56538e9e64ee02fb5b4842bf0394ee44b707dae3a63ec7f902abd0afb`,
openings, prefixes, fastchess, Stockfish and all auditor sources.

The protocol was fixed as:

- Rodent V1.1 Anand, `Threads=8`, `GOMAXPROCS=8`, `Hash=128`, OwnBook off and
  Move Overhead 100;
- both roles on physical CPUs `0,2,4,6,8,10,12,14`, no fastchess affinity
  split, concurrency one;
- paired sequential six-ply openings, `10+0.1`, seed 20260919;
- 20 A/A games, then only after a valid A/A, exactly 400 candidate games / 200
  pairs; and
- 100,000-replicate paired percentile bootstrap with seed 2026091901. Adopt
  only if the lower 95% bound exceeded zero; otherwise shelve, with no
  post-score extension.

The Lean hopper PID 3099092 remained alive before, during and after the run.
External CPU was measured and disclosed under the user-directed shared-host
policy, not used as a standalone rejection condition.

## Result

The same-binary A/A control passed every preflight, effective-width, process,
protocol and legal-game audit:

- 4W/11D/5L from engine-A's perspective, 9.5/20, penta
  `[1,2,4,3,0]`;
- 2,955 legal plies;
- paired bootstrap **−17.39 Elo [−126.97,+88.74]**, including zero; and
- clean 636.99-second supervisor, one instance per role and no survivors.

The prospective candidate phase then completed all 400 games:

- candidate **83W/252D/65L**, **209/400 = 52.25%**;
- penta `[3,37,102,55,3]`;
- paired normal **+15.65 Elo [−2.57,+33.95]**;
- paired bootstrap **+15.65 Elo [−2.61,+33.98]**; and
- 65,022 legally replayed plies, no probable embedded-book signature, empty
  match stderr, clean effective-width/protocol trace, one instance per role,
  clean 13,330.66-second supervisor and no survivors.

Shared-host telemetry was low and non-dispositive: A/A external CPU averaged
0.128 cores and peaked at 0.454; candidate external CPU averaged 0.113 and
peaked at 0.768. The hopper was never paused.

The positive point estimate is encouraging, but the predeclared lower-bound
rule returns **SHELVE**. A near miss does not authorize extra games, a provisional
keep or combination with the rejected depth-stagger candidate. The production
source and candidate tests were removed.

Immutable WSL evidence is under
`/home/ehrli/repos/ngn-smp-helper-result-gate-20260919/run-001`. Binding hashes:

- decision: `3ab8dbe30bc9e85cbc48e4d0797dc4ed83c716cd4b79b36ba594bd3ca5f7706f`;
- inventory: `9243f939c7bd51b431fca2f93532ded9a91508f0a4dbd3b33df9702eca3b5f0d`;
- terminal: `d5896a281ed39e57147e89bbf263e5ba93f4aaff15bf9b77d7fba8bbede136ec`;
- candidate legal audit:
  `d6c744f78e1ed14141f1869f897d3fc454da8069f7262a7ca88548c0e6dfed4d`;
- candidate operational audit:
  `29ed0d0c985c40da1d8eab90efff2983b699dbd64b9676b92a4071b6bfc78a63`.

This valid, narrowly inconclusive result does not change the accepted engine,
the external Counter checkpoint or the unverified absolute 3300 target.
