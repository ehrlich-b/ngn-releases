# Codex Engine Review

Date: 2026-06-22

Scope: the current working tree, with emphasis on chess correctness, search accuracy, UCI behavior, match-harness integrity, tests, feature claims, and the engineering gap between NGN and leading Go chess engines.

This is a code review, not an Elo estimate. Several findings need targeted regression tests before they are called proven game-strength bugs, but the highest-priority issues are concrete enough to fix before more tuning.

## Executive verdict

NGN's core move representation, ordinary make/unmake path, legal move generation, evaluation symmetry, and much of the alpha-beta machinery are credible. The focused perft, hash symmetry, evaluation symmetry, pseudo-legality, and current qsearch invariant tests pass. The engine is not in "rewrite it" territory.

The main risk is that search and measurement are being tuned on top of correctness seams:

1. Interrupted searches can continue updating parent scores, histories, correction history, and the transposition table using an abort-time static evaluation.
2. The qsearch depth cap can return a static evaluation while the side to move is in check.
3. Root repetition accounting mixes game-history mutation with search-history interpretation and can classify a second occurrence as a draw.
4. The UCI engine has confirmed data races and lifecycle races around `go`, `stop`, output, and mutable engine state.
5. Quiet promotions are eligible for several forward-pruning rules.
6. En-passant state is hashed even when no legal en-passant capture exists, which can prevent valid repetition recognition.
7. Multiple advertised UCI options and documented features are stubs or deliberately disabled.

The right near-term strategy is:

- freeze speculative search additions;
- fix P0/P1 correctness and protocol issues;
- add adversarial regression tests;
- rerun the candidate/base experiment from clean rebuilt binaries;
- then optimize and tune.

For the "best Go chess engine" target, correctness is necessary but not sufficient. CounterGo advertises real multithreading and is rated far above NGN's current claimed range. Zahak's final architecture included NNUE, Lazy SMP, Syzygy, Polyglot, MultiPV, and OpenBench integration. NGN should first become a smaller, trustworthy single-threaded engine; the likely next major strength projects are NNUE and SMP, not more hand-added pruning rules.

## What is solid

The following checks passed on this tree:

- `go test -short ./...`
- start-position and current standard-suite perft tests;
- make/unmake hash symmetry;
- evaluation symmetry and term symmetry;
- pseudo-legal move validation tests;
- current qsearch invariants;
- correction-history reset tests in the working tree.

The search also has several good foundations:

- mate scores are normalized on TT storage/retrieval;
- root iterative deepening preserves the last completed iteration;
- main search distinguishes PV and cut nodes;
- legality is checked after pseudo-legal generation;
- qsearch correctly forbids stand-pat while in check, except at the hard cap discussed below;
- singular verification avoids directly storing the move-excluded result;
- the local SPRT implementation uses paired pentanomial results as its decision statistic.

The current uncommitted correction-history patch now clears all auxiliary tables in `ClearHistoryTable()` and adds regression tests. That fixes the cross-game state leak in the working tree. Any match binary built before that fix remains invalid and should not be used for a verdict.

## P0: fix before trusting another overnight result

### 1. Abort-time values can contaminate TT and heuristic state

Evidence:

- stop/time/node exits return `EvaluateForPlayer(...)` from `alphaBetaPV` at `engine/search.go:1091-1115`;
- after a child call returns, the parent does not immediately check `info.Stopped`;
- the parent can update `bestScore`, take a beta cutoff, update history/killer/counter/capture/correction tables, and store a TT bound at `engine/search.go:1882-2011`;
- qsearch has the same shape around recursive calls and its lower-bound store at `engine/search.go:2257-2277`.

The root correctly discards an interrupted iteration, but the tree below it is not discarded. The arbitrary static score returned at the interruption point can be negated and propagated as if it were a searched value. That can create false cutoffs and persistent TT entries used on later moves in the same game. It can also train the history and correction tables on an aborted line.

Fix:

- after every recursive search call, restore the position and predecessor state, then immediately return if `info.Stopped`;
- do not update history, correction history, node classification, or TT bounds while unwinding an aborted search;
- apply the same rule to IID, null move, probcut, singular verification, PVS re-searches, and qsearch;
- consider a dedicated abort result rather than overloading an ordinary centipawn value.

Required tests:

- stop a search inside a child, then assert no parent TT entry was stored;
- compare a clean fixed-node search against the same search preceded by an interrupted search;
- run the test repeatedly with stop injected at different node counts.

### 2. Qsearch may return stand-pat/static evaluation while in check

`quiescenceWithDepth` checks `qDepth >= 6` before it checks `pos.IsInCheck()` at `engine/search.go:2033-2041`. A checking capture or forced evasion chain reaching that cap receives a raw static evaluation instead of legal evasions or checkmate detection.

That is a direct search-soundness violation. It can turn mate, forced material loss, or a forced evasion into an ordinary leaf score.

Fix:

- preferably remove the fixed six-ply qsearch cap and rely on finite captures, SEE/delta pruning, maximum ply, node limits, and stop handling;
- if a cap remains, never apply it in check;
- do not cut a capture sequence at a node where the side to move has no legal stand-pat option.

Also fix qsearch seldepth: `ply` already advances on every qsearch recursion, so `ply + qDepth` double-counts the qsearch portion at `engine/search.go:2034`.

Required test:

- construct a forced checking/evasion sequence of at least six q plies and verify the result against an uncapped reference qsearch.

### 3. UCI search lifecycle is racy and `stop` can be lost

This is confirmed by `go test -short -race ./...`.

Observed races include:

- `UCIEngine.searching` read/write races (`engine/uci.go:438`, `563-565`, `875`);
- tests and the search goroutine concurrently accessing output buffers;
- search state and command state being accessed from separate goroutines without ownership or synchronization.

There is also a deterministic lifecycle race:

- `handleGo` launches the goroutine before that goroutine sets `searching = true`;
- a rapid `stop` can see `searching == false` and do nothing;
- search entry calls `ClearStop()` at `engine/search.go:614`, so an early global stop can also be erased;
- two rapid `go` commands can both pass the `searching` check and launch concurrent searches over global TT/history/stop/max-node state.

`quit` does not exit the scanner loop. `handleQuit` only requests stop and returns at `engine/uci.go:892-898`.

Fix the UCI layer as an ownership problem:

- set search state synchronously before launching;
- give each search a local cancellation token and local node budget;
- use a search `done` channel or wait group;
- serialize output or route it through the command owner;
- make `stop` cancel unconditionally;
- make `position`, `ucinewgame`, and option mutation wait for the active search to finish;
- make `quit` cancel, wait, and return from `Run`;
- eliminate the 10 ms sleep as synchronization.

This should also remove the need for package-global `globalStopRequested`, `globalMaxNodes`, and `lastMovePlayed`.

## P1: chess accuracy and protocol correctness

### 4. Root repetition accounting has mixed semantics

The iterative-deepening root uses `GameMakeMove`, which increments the game-history count for the child (`engine/search.go:760-775`, `engine/position.go:166-174`). The child then enters `alphaBetaPV`, where a game-history count of two is interpreted as "two historical occurrences plus the current search occurrence" and returned as a draw (`engine/search.go:1157-1170`).

At the root, however, the current occurrence has already been inserted. A child position seen once earlier becomes count two and is treated as a draw on its second occurrence, not its third.

Search-path twofold repetition can be a deliberate anti-cycle heuristic. The game-map comment and `IsFIDEDrawRule`, however, clearly intend exact threefold semantics. `SearchFixed` uses ordinary `MakeMove`, so the two search entry points also disagree.

Fix:

- game-history maps should contain actual played positions only;
- root search should use `MakeMove`, not `GameMakeMove`;
- treat the current search node as implicit when comparing against historical counts;
- add tests for first, second, and third occurrence at the root and below the root.

### 5. En-passant hashing can miss repetitions

Every double pawn push sets `Position.EnPassant` at `engine/position.go:216-223`. The regular Zobrist key includes that square whenever its rank matches the side to move, without checking whether an en-passant capture exists (`engine/hash.go:78-92`, `200-227`).

Two positions with exactly the same legal moves should have the same repetition identity. A non-capturable en-passant target should not split the key. NGN already applies the correct capturability concept in its Polyglot hash helper.

Cross-reference:

- local Blunder source clears `EPSq` after a double push if no opposing pawn attacks it;
- current Stockfish validates and normalizes en-passant state, including whether a legal capture exists, before including it in the position key.

Fix:

- normalize internal en-passant state on FEN load and double pushes;
- hash it only when an en-passant capture is actually legal, or at minimum pseudo-available consistently with the chosen repetition policy;
- make incremental and full hash generation use one shared helper.

Add repetition tests around uncapturable and pinned en-passant cases.

### 6. Forward pruning can remove underpromotions

At `engine/search.go:1568`, futility pruning exempts only queen promotions. LMP at `1577-1585` and history pruning at `1617-1623` do not exempt promotions at all.

Underpromotions are rare but not optional. They can avoid stalemate, deliver a knight check, or avoid a tactical drawback of queening. The queen promotion is normally ordered first, which makes the late underpromotions especially vulnerable to these gates.

Fix:

- all promotion moves should be exempt from futility, LMP, history pruning, quiet SEE pruning, and LMR unless a promotion-specific rule has been proven safe;
- use `move.PromoType() == NoType` as the quiet-pruning prerequisite.

### 7. `SearchFixed` is a divergent and partially inconsistent search

`SearchFixed` does not initialize `SearchInfo.RootDepth` (`engine/search.go:992-1005`), although extension budgeting depends on it. At depths above roughly the extension budget, this can clamp search depth relative to zero rather than the requested root depth.

It also:

- uses a separate root driver from iterative deepening;
- has a special shallow mate re-search that asks for `depth+2` after the root move;
- contains an `INFINITY -> 2000` workaround rather than enforcing score invariants (`engine/search.go:1050-1077`);
- differs from iterative search in root repetition behavior and PVS/LMR.

Fix:

- share one root search implementation;
- make fixed-depth mode run exactly one normal root iteration;
- set `RootDepth`;
- remove the infinity workaround after identifying its original cause.

Until then, fixed-depth diagnostic results are not guaranteed to describe the production search.

### 8. UCI options and feature claims are inaccurate

Advertised but ineffective or incomplete:

- `Hash`: advertised, handler is TODO;
- `Threads`: advertised, handler is TODO;
- `Move Overhead`: advertised, no handler;
- `NullMoveR`: advertised tunable, but search uses hardcoded `4 + depth/6`;
- `SingularMargin`: advertised tunable, but search uses hardcoded `2*depth`;
- `go ponder`: parsed but ignored;
- `ponderhit`: TODO;
- `quit`: does not terminate the loop.

Feature claims:

- README claims Polyglot support, but the known-hash test is skipped because the 781 constants are not the Polyglot constants. Real `.bin` books will not match.
- README claims Syzygy support, but probing is deliberately hard-disabled because the decoder is a hash-modulo placeholder.
- `setoption name SyzygyPath` can report a successful load even though probes are guaranteed misses.

Fix:

- implement an option completely or do not advertise it;
- remove or label Polyglot and Syzygy as placeholders until conformance tests pass;
- add a UCI option conformance test that changes every advertised option and verifies observable engine state;
- add official Polyglot start-position hash vectors;
- integrate a real Syzygy library/decoder rather than retaining a dormant fake decoder.

This is not cosmetic. Match runners and GUIs assume advertised controls work.

## P2: robustness, harness accuracy, and cleanup

### 9. Harness timeouts do not interrupt a blocked scanner

`internal/uci.WaitFor`, `GetMove`, and `Analyze` compare wall-clock deadlines only between calls to `Scanner.Scan()` (`internal/uci/uci.go:148-216`). `Scan()` blocks while an engine is silent, so the timeout and `stopAfter` logic cannot fire until another line arrives.

Consequences:

- startup can hang beyond the claimed 15 seconds;
- a silent search may never receive the scheduled `stop`;
- tool-specific watchdogs hide the issue in some paths but not all paths.

Fix:

- one goroutine should continuously read stdout into a bounded line channel;
- callers should `select` over lines, process exit, stop timer, and hard timeout;
- consolidate the several duplicate UCI subprocess clients under this implementation.

### 10. Cloud aggregation loses the statistic used locally

Local SPRT stops on paired pentanomial LLR. `cloudsprt.sh fetch` parses only aggregate W/D/L and recomputes a trinomial-style LLR. Independently stopped shards then get pooled as if they were one ordinary sample.

That is not the same experiment:

- reversed-color pair information is lost;
- shard-specific optional stopping is ignored;
- pooled W/D/L cannot reproduce the local decision statistic.

Fix:

- persist pair IDs or the five pentanomial buckets in machine-readable shard output;
- pool the sufficient statistics centrally;
- only the central aggregate should make the final sequential decision, or use a formally defined distributed sequential test.

### 11. Cloud gauntlet tallies discard integrity metadata

`anchorTally` stores only version, rating, W/D/L. Cloud pooling reconstructs results with an empty reason map (`cmd/gauntlet/main.go:47-67`, `531-587`). The local report's 2% forfeit/error exclusion therefore cannot operate on pooled cloud results.

The history record also stores only `MovetimeMs`; a `-tc` run can be recorded and later described through movetime-oriented reporting. The report always prints the movetime-bias footer, including real-clock runs.

Fix:

- include reason counts, time control mode, base time, increment, node/depth mode, concurrency, opening-set hash, binary hashes, and command line in tallies/history;
- use mode-specific reporting and rolling-history keys.

### 12. Opening-book behavior is hidden and always on

The engine probes an external book and then the embedded book on every search, with no `OwnBook`/`UseBook` option. This makes analysis and match reproducibility depend on hidden engine policy. It can also reduce A/B signal if both sides repeatedly leave the supplied opening through the same embedded move.

Local Blunder exposes `UseBook` and defaults it to false. NGN should do the same:

- default books off for engine testing;
- let the GUI or harness supply openings;
- record book use in game metadata.

### 13. FEN and position APIs need stricter contracts

Issues:

- `GenerateFEN` always writes fullmove number `1`;
- `ParseFEN` validates that fullmove is an integer but does not store it or require a meaningful range;
- parser tests permit missing/multiple kings as logged warnings rather than failures;
- en-passant rank/pawn consistency is not validated;
- white en-passant generation does not verify the captured black pawn exists, while one black slice implementation does;
- `MakeMove` returns `legal=true` unconditionally even though legality is checked by callers;
- `IsDraw()` uses `HalfMoveClock > 100`, while the rule boundary is `>= 100`.

Fix:

- store game ply/fullmove number in `Position`;
- strictly validate FEN used by UCI;
- sanitize castling and en-passant rights;
- make move API naming honest (`MakePseudoLegalMove`, or return real legality);
- unify the two draw-rule helpers.

### 14. Test suite cleanup is required

Current outcomes:

- `go test -short ./...`: passes;
- `go test -short -race ./...`: fails on real UCI races;
- `go test ./...`: fails `TestConcurrencyStress` and `TestEnginePlayingStrength`.

`TestConcurrencyStress` concurrently calls mutating legal move generation on the same `Position`. Either document and implement thread safety, or give each goroutine a copy. The current test is testing an unsupported ownership model.

`TestEnginePlayingStrength` compares `Move.ToString()` UCI coordinates such as `c4f7` against SAN strings such as `Bxf7+`, `O-O`, and `a8=Q`. Its 950 Elo result is therefore meaningless. The hand-selected move lists and formula are not a rating test even after notation is fixed.

Cleanup:

- delete the synthetic Elo formula;
- convert useful positions to UCI/EPD best-move tests, preferably oracle-validated;
- use deterministic seeds and print/replay failing seeds;
- move benchmarks out of correctness tests;
- make malformed-FEN cases assert errors;
- expand perft to the standard multi-position suite, including castling, en passant, promotions, checks, and discovered-check legality;
- add process-level UCI tests instead of directly racing internal fields and `bytes.Buffer`.

The Makefile's current uncommitted change correctly stops swallowing test failures. Keep it.

### 15. Remove or quarantine dead and misleading code

Candidates:

- `HasLegalMoveQuick` is explicitly approximate and its quick king moves encode captures without the capture tag; it is currently unused and should be deleted or kept outside authoritative game logic.
- dead razoring constants and stale comments should be removed;
- placeholder Syzygy decoding should be replaced with a clear unsupported stub until a real implementation lands;
- duplicate UCI clients across commands should be consolidated;
- package-global mutable search state should move into a per-search object;
- `SearchFixed` should not remain a second search implementation.

## Cross-engine comparison

### CounterGo

Official repository: https://github.com/ChizhovVadim/CounterGo

CounterGo's official README describes a Go UCI engine with multithreading and lists Counter 5.0 at 3373 on CCRL 40/15. Ratings across lists and hardware are not directly comparable, but the architectural message is clear: the top Go-engine target already includes real SMP and a mature neural evaluation path.

### Zahak

Official repository: https://github.com/amanjpro/zahak

Zahak was archived on 2026-04-25. Its final documented feature set included NNUE, Lazy SMP, Syzygy, Polyglot, MultiPV, pondering, and OpenBench compatibility. Its README lists 9.x around 3278 CCRL Blitz. Even though development has stopped, it is a useful completeness baseline.

### Blunder

The repository contains local Blunder source under `opponents/.src/engine`.

Useful contrasts:

- en-passant state is normalized before hashing;
- `Hash` is implemented;
- book use is exposed and disabled by default;
- its search and UCI are smaller and easier to reason about.

Blunder is not the ceiling, but its simpler state contracts are worth copying.

### Stockfish correctness reference

Official source: https://github.com/official-stockfish/Stockfish/blob/master/src/position.cpp

Stockfish's current position loader strictly validates piece counts and kings, sanitizes castling, validates legal en-passant availability, stores game ply, and hashes normalized en-passant state. NGN does not need Stockfish's complexity, but it should adopt the same state-identity discipline.

## Recommended implementation order

1. Make UCI/search ownership race-free and make `quit`/`stop` reliable.
2. Prevent abort unwinding from writing TT/history/correction state.
3. Remove or repair the qsearch cap.
4. Correct root repetition and en-passant hashing.
5. Exempt all promotions from quiet forward pruning.
6. Unify `SearchFixed` with the production root search.
7. Make every advertised UCI option truthful.
8. Fix the harness reader and cloud sufficient-statistic formats.
9. Clean the test suite and expand perft/adversarial regressions.
10. Only then resume search/eval tuning.
11. After the single-threaded engine is trustworthy, benchmark NNUE and SMP as explicit projects.

## Minimum acceptance gate before more tuning

A new candidate should not start an overnight run until all of these hold:

- `go test -short -race ./...` passes;
- full `go test ./...` passes without synthetic Elo tests;
- process-level `go`, immediate `stop`, repeated `go`, `position` after stop, and `quit` tests pass;
- qsearch-in-check-at-cap regression passes;
- interrupted-search TT pollution regression passes;
- second/third repetition tests pass;
- capturable/uncapturable en-passant hash tests pass;
- underpromotion pruning regressions pass;
- all advertised UCI options have conformance tests;
- candidate and base binaries, hashes, options, time control, openings hash, and source SHAs are recorded.

At that point, "more games" becomes useful again. Before it, more games can produce a narrower confidence interval around the behavior of a flawed engine or flawed instrument.
