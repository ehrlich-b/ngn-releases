# Lazy SMP helper initial-depth staggering

Status: **completed / SHELVE** against base `afe3e2d`. The exact candidate
scored 49.75% in the fixed 400-game gate, and its paired interval crossed zero.
The experiment made no accepted source change and was not extended.

## Mechanism

NGN's primary and every helper currently start independent iterative deepening
at depth one. The primary alone owns time decisions, callbacks and the returned
move; helpers contribute only shared-TT work and aggregate counters. Pinned
Counter 5.5 instead initially assigns roughly half its searches at the next
depth and half one depth deeper. Repeating all shallow work is a plausible Lazy
SMP efficiency gap, but the source comparison alone is not evidence of Elo.

Test exactly one conservative scheduling change:

- primary worker zero still starts at depth one and remains the sole time/result
  owner;
- among `N` active workers, indices below `(N+1)/2` start at depth one and the
  rest start at depth two. Thus widths 2/3/4/8 begin at `[1,2]`, `[1,1,2]`,
  `[1,1,2,2]` and `[1,1,1,1,2,2,2,2]`;
- every worker then continues ordinary sequential iterative deepening;
- clamp the start to the requested maximum depth, so depth-one and cancelled
  searches retain valid behavior;
- do not add helper-result selection, a shared task scheduler, new TT policy,
  evaluator changes, time-management changes or any one-thread behavior change.

The Counter source basis remains the pinned release
`63c487ca724c620f71c129d62129c6fb9109c872`, file
`pkg/engine/lazysmp.go`, previously recorded at SHA-256
`01fabc5cd22f09f8bd1269652e09473c27edb6df18a0c28f5b71bec88d1a9231`.

## Predeclared evidence contract

All execution occurs on the authorized WSL host while the user-directed Lean
hopper remains running.

1. Add a direct schedule regression test, including width one and maximum-depth
   clamping. Run the focused SMP tests, full short engine suite, full short suite
   and full short race suite.
2. Build exact base and candidate Linux binaries from clean isolated trees and
   bind their source/binary hashes plus the existing exact Rodent model and
   openings.
3. Before candidate games, run a same-binary/same-binary A/A control at the exact
   clock, width, concurrency and CPU policy. Its paired 95% interval must include
   zero and all legal/operational/process checks must pass.
4. Because this changes search behavior, only a completed predeclared paired
   real-clock game test against immediate base can accept it. Proxies may reject
   a clear regression but cannot keep it. Do not keep an inconclusive positive
   sign, extend after observing the score or combine it with helper-result
   selection.

The exact game manifest, sample cap and decision thresholds must be frozen after
the correctness/build gates and before any candidate game. Shared-host CPU load
is measured and disclosed but, per the September 18 direction, does not alone
invalidate the run. Any accepted result remains local playing evidence, not an
absolute-rating or 3300 claim.

## Frozen implementation and correctness

The candidate was based on exact commit
`afe3e2dc37cd91a9dff3e9ddbd4db584798b3fdd` (tree
`f220e409669e524bd84d62bc54519d276a409956`). Its binary diff, including this
prospective experiment note, was frozen at SHA-256
`eca0419fab096bb1310de40c199e7d7f6fd76f2e0aabd6726ac14a06e60c63d0`.
The implementation added a helper-only initial-depth selector, passed the
schedule `[1,1,1,1,2,2,2,2]` at width eight, and did not change primary result
selection, timing, evaluation, TT policy or one-thread behavior.

All execution was on the authorized WSL host. The following completed before
games:

- focused `TestLazySMP` short tests;
- the full short engine suite;
- the full repository short suite; and
- `CGO_ENABLED=1 go test -short -race ./... -count=1` with Go 1.25.5.

Clean isolated trees produced both exact binaries with
`CGO_ENABLED=0 GOAMD64=v3 go build -trimpath -ldflags='-s -w'`:

- base: `fc249cb4f6fd30018a61df5671249c37f6ae698aea6e5ea29fbf2b859c0a7d21`;
- candidate: `6a591a1a63b1f97438fa31637ddfecad0abcbaa23fe99551d93d74bf0c3d4084`.

## Frozen gate

The released manifest was frozen before any game at SHA-256
`ca563a063d160467b3f712cb715b55cfb305a40f07fb90fe0ebeb00d4e51d2a7`.
It bound the binaries above, exact Rodent Anand model
`5f7480b56538e9e64ee02fb5b4842bf0394ee44b707dae3a63ec7f902abd0afb`,
the existing 200-opening input and prefix list, fastchess, Stockfish and every
runner/auditor source. The held static audit verified all 13 frozen artifacts
and both engines before release.

The game settings were frozen as:

- Rodent V1.1 Anand, `Threads=8`, `GOMAXPROCS=8`, `Hash=128`, OwnBook off and
  Move Overhead 100;
- both roles exposed to physical CPUs `0,2,4,6,8,10,12,14`, no fastchess
  affinity partitioning, concurrency one;
- paired sequential six-ply openings, `10+0.1`, seed 20260918;
- 20 A/A games followed, only on a valid A/A result, by exactly 400 candidate
  games / 200 pairs;
- 100,000-replicate paired percentile bootstrap, seed 2026091801; and
- adopt only if the candidate lower 95% bound exceeded zero. Otherwise shelve;
  never extend after observing the score.

The machine remained shared exactly as directed. The Lean hopper PID 3099092
was alive before, during and after the gate. External CPU was retained for
disclosure, not used as a standalone rejection condition.

## Result

The A/A control passed all preflight, effective-width, process, protocol and
legal-game checks:

- 3W/13D/4L from engine-A's perspective, 9.5/20, penta
  `[0,3,5,2,0]`;
- 3,346 legal plies;
- paired bootstrap **−17.39 Elo [−88.74,+52.51]**, including zero; and
- clean 678.60-second supervisor, one instance of each role and no survivors.

The candidate phase then completed all 400 games without extension:

- candidate 66W/266D/68L, **199/400 = 49.75%**;
- penta `[4,41,111,41,3]`;
- paired normal **−1.74 Elo [−19.61,+16.13]**;
- paired bootstrap **−1.74 Elo [−20.00,+16.52]**; and
- 59,130 legally replayed plies, no probable embedded-book signature, empty
  match stderr, clean effective-width/protocol trace, one instance per role,
  clean 12,682.47-second supervisor and no survivors.

Shared-host telemetry was modest and non-dispositive: A/A external CPU averaged
0.238 cores and peaked at 0.867; candidate external CPU averaged 0.277 and
peaked at 0.919. The hopper was never paused.

The predeclared lower-bound rule therefore returns **SHELVE**. The point estimate
is slightly negative and the interval is fully consistent with no effect. The
candidate source and its regression test were removed rather than provisionally
kept. Do not rerun, extend or combine this result with later helper-result work.

Immutable WSL evidence is under
`/home/ehrli/repos/ngn-smp-depth-stagger-gate-20260918/run-001`. Binding hashes:

- decision: `0a06042a83aaa03daa1a05bdafc3bfe928d70f5bb6378915493932f288c77168`;
- inventory: `83d2cb44e6bb1f9e2166175180305b1f058ae384939b7fa467775d2d400e0518`;
- terminal: `bae6c7e548ba3b93e9ffba177614e1b390c43f47efd0eacd51430d1f322d1d1b`;
- candidate legal audit:
  `a0686dea6b9361ee08fb390c209bf34dfc30af21ac7fa78de9c485b050dcf0d4`;
- candidate operational audit:
  `f0ac9231651350e5177081f39ee3958c70d9404074b6505712ea467c369451ec`.

This valid null result says only that this shallow deterministic staggering did
not earn adoption at these shared-host eight-thread settings. It neither changes
the accepted external Counter checkpoint nor verifies an absolute rating of
3300.
