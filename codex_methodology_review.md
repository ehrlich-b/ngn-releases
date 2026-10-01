# Codex Methodology Review

Date: 2026-06-22  
Scope: the proposed autonomous night, the current live candidate, the test and tournament harnesses, the repository's experimental history, and the longer-term plan to build the strongest chess engine written in Go.

## Executive verdict

The engine project is technically serious, but the current development methodology is not yet safe to run as an autonomous feature-production loop overnight.

The strongest parts are real:

- paired color-reversed games;
- pentanomial statistics in the local SPRT;
- deterministic node checks;
- external ACPL filtering;
- real-clock support;
- crash, timeout, and cloud shutdown guardrails;
- a frozen external anchor binary;
- substantial regression and invariant testing;
- a willingness to audit prior conclusions and reverse them.

The central failure is not lack of effort or lack of instrumentation. It is that the project repeatedly changes which instrument is authoritative and then makes decisions that the chosen instrument does not support. `TODO.md` currently contains incompatible policies:

- per-change external gauntlet as truth;
- fast self-play SPRT as the per-change gate;
- proxy-only keeps for “correctness-flavored” changes;
- a rule that only completed SPRTs can produce verdicts;
- a rule that positive sign at a cap is enough to keep;
- a rule that no change is committed before its verdict;
- live commits made before their game verdict.

That ambiguity makes an autonomous agent dangerous. It can always find a sentence authorizing the next action.

The right overnight model is:

> Run many clean, paired, candidate-vs-immediate-base games for one pre-registered change. Use external opponents periodically to test transfer and calibrate absolute strength. Do not let the overnight agent invent new hypotheses, modify the harness, reinterpret thresholds, stack changes, or mutate `main`.

“More games” is the correct instinct. The qualification is that more games only reduce variance around the quantity actually being measured. Ten thousand self-play games can measure a self-play effect precisely while remaining confidently wrong about external transfer.

## Immediate stop/go decision

### Do not use the current corrhist run for a keep/revert verdict

The uncommitted auxiliary correction-history implementation adds:

- `materialCorrectionHistory`;
- `nonPawnCorrectionHistoryW`;
- `nonPawnCorrectionHistoryB`.

`engine.ClearHistoryTable()` clears the existing pawn correction table but does not clear these three new tables. `ucinewgame` calls `ClearHistoryTable()` before every game.

Consequences:

1. The candidate learns across games while the base does not.
2. Results depend on game order.
3. Results depend on which worker receives each game.
4. Reversed-color games in a nominal pair can start with different learned state.
5. Re-running the same match can produce a different result even with the same opening order.

This is a direct experimental-integrity bug. The current run may reveal crashes or gross weakness, but it cannot support a statistical verdict.

Before any new corrhist match:

- clear all auxiliary tables in `ClearHistoryTable()`;
- add a test proving every correction table is zero after clear;
- add a test proving two searches separated by `ucinewgame` start from identical correction state;
- rebuild both binaries from recorded source states;
- rerun an A/A null match in the exact intended TC and concurrency.

### The current stack comparison cannot attribute corrhist

The live state says:

```text
candidate = 50-move damping + corrhist extension
base      = pre-damping anchor
```

That is `C vs A`, where:

- A = pre-damping base;
- B = damping only;
- C = damping + corrhist.

The questions require:

- B vs A to measure damping;
- C vs B to measure corrhist.

C vs A cannot identify either component. A neutral stack could mean:

- both are neutral;
- damping helps and corrhist hurts;
- damping hurts and corrhist helps;
- a real interaction cancels two main effects.

The live decision rule nevertheless proposes removing corrhist while retaining damping after a neutral stack result. That conclusion is not identified by the experiment.

### 50-move damping is a heuristic, not a correctness fix

`FiftyMoveDampBudget=256` scales static evaluation from halfmove clock 1 onward and leaves approximately 61% of the eval at the actual 100-ply draw boundary. Search already returns draw at `HalfMoveClock >= 100`.

This may be a good heuristic. It is not a correctness theorem, and “correctness-flavored” is not a valid experimental category. It changes evaluations and search decisions before the rule is reached. It therefore needs the same game evidence as other evaluation heuristics.

The change was committed before its game verdict, contrary to the documented commit rule. That should be treated as an experimental-process exception, not precedent.

## Methodology scorecard

| Area | Assessment | Main issue |
|---|---|---|
| Hypothesis quality | Mixed | Plausible mechanisms are promoted to governing theories too quickly |
| Change isolation | Poor in the live run | Two behavior changes are stacked against an older base |
| Game volume | Good direction | High-N paired games are the correct per-change instrument |
| Statistical decisions | Weak | Sign-at-cap, post-hoc “neutral” rules, and incompatible SPRT policies |
| External validity | Partial | Self-play transfer is known to fail; external testing is single-family and noisy |
| Reproducibility | Poor | Critical openings, binaries, corpora, and raw results are unversioned or local-only |
| Harness operations | Strong but complex | Good failure handling, but cloud aggregation does not preserve the local decision statistic |
| Documentation | Actively harmful | Multiple “authoritative” and “superseded” sections coexist |
| Autonomous safety | Not ready | The agent is authorized to change hypothesis, code, gate, verdict, docs, and git state |
| Long-term technical direction | Plausible but underspecified | “Best Go engine” lacks a frozen rating list, TC, hardware, thread count, and feature class |

## What the project is doing well

### 1. It has moved beyond five-game anecdotes

The paired SPRT harness is a meaningful instrument. Reversed colors, shared opening pair IDs, penta buckets, watchdog recovery, and explicit flag-out reporting are all correct instincts.

### 2. It distinguishes several kinds of evidence

The repository recognizes that:

- ACPL is useful for external move-quality filtering;
- node counts detect behavior changes but do not measure strength;
- NPS matters at real clock;
- external anchors are needed for transfer and absolute calibration;
- game autopsies can identify classes of loss.

That conceptual separation should be retained.

### 3. Operational safety received real engineering attention

The cloud harness has:

- worker TTLs;
- completion shutdown;
- failure sweeps;
- multi-region status;
- S3 heartbeats;
- spot-reclaim handling;
- zero-live-worker fetch gating.

Those are stronger controls than most hobby engine projects have.

### 4. The repository contains valuable invariant knowledge

`docs/09-invariants-and-joints.md` has the right insight: large engine bugs often occur at interactions between otherwise-correct mechanisms. That is a better bug-finding model than feature-count comparison.

## The primary methodological defects

### 1. The project has no single decision policy

The decision policy must be one short table keyed by change class. It cannot be reconstructed from several historical narratives.

The current “KEEP rule” is especially damaging:

> improvement or correctness fix + no measured regression => keep, including effects too small to resolve

For speculative behavior changes, this accumulates false positives and complexity. A positive point estimate at an inconclusive cap is not evidence of improvement. Repeating this across many candidates selects noise.

The batch certificate helps, but it does not undo all selection bias. It also creates expensive bisection work after several weakly supported changes have already been stacked.

Recommended policy:

| Change class | Required evidence | Inconclusive result |
|---|---|---|
| Provable correctness fix | Mechanism proof, regression tests, non-inferiority game test when behavior changes materially | May keep only if correctness is actually proved |
| Node-identical refactor | Exact node/score identity, tests, measured speed or maintainability benefit | Keep if identity is proved |
| Speed optimization | Exact behavior identity plus repeatable NPS/wall-time gain | Shelve if gain is not repeatable |
| Search/eval heuristic | Completed paired game test reaching a predeclared acceptance condition | Shelve; do not keep on sign |
| Large tune | Holdout objective plus games against an immediate base | Shelve or redesign |
| Lane closure | Multiple well-powered falsification attempts using the right instrument | Mark uncertain if evidence is proxy-only |

There should be no “correctness-flavored” category.

### 2. The experiment unit is not stable

The repository alternates between:

- one change;
- a stack;
- a batch of four to six keeps;
- an absolute gauntlet;
- a self-play match against an anchor several commits old.

The default experimental unit should be:

> one behavior change, candidate vs its immediate production base, same harness, same openings, same options, same machine class.

Stacks are allowed only for an explicitly named interaction experiment. A stack result must never be decomposed narratively.

### 3. The SPRT is being asked to answer incompatible questions

The default `[-3,+3]` SPRT is an indifference-zone sign test:

- H0: effect is at most -3 Elo;
- H1: effect is at least +3 Elo.

If the true effect is near zero, it sits between the hypotheses. The expected LLR drift is near zero, so even many games can end inconclusively. That is not harness failure; it is the test design.

The current plan then adds a post-hoc rule:

- if the CI spans zero at 5,000 games, call the change neutral and decide based on narrative or speed.

That is a different fixed-N decision procedure and should be specified before the run.

Use separate tests for separate decisions:

- improvement: H0 = 0, H1 = a practically meaningful gain;
- non-inferiority: H0 = unacceptable regression, H1 = no material regression;
- equivalence/neutrality: fixed-N interval contained inside a predeclared equivalence band;
- cost-bearing feature: require enough Elo to pay for measured NPS, memory, and complexity cost.

A 5,000-game match is valuable, but it is not automatically capable of resolving a 3 Elo effect. Historical penta intervals in this repository suggest several thousand games commonly leave uncertainty of roughly several Elo. The exact resolution depends on the pair distribution.

### 4. Endless candidate testing creates a multiple-testing problem

The project has made 132 local commits since `origin/main` over roughly two weeks:

- 69 documentation-only;
- 47 code-only;
- 14 mixed;
- 81 touch `TODO.md`.

Even a correctly calibrated 5% test will admit false positives when dozens of speculative candidates are screened and positive-looking caps are kept. Informal retries, parameter variants, and lane pivots increase this further.

Controls:

- pre-register one primary candidate per run;
- declare related parameter variants one family;
- do not retry the same mechanism with changed thresholds unless the prior result identified a concrete flaw;
- require a holdout batch certificate before promotion;
- track all attempts, including abandoned and negative ones, in a structured ledger;
- never use “best observed variant” without a fresh holdout test.

### 5. The opening corpus is a hidden training set

`output/sprt_openings.txt` contains 5,000 openings and is ignored by git. It was generated by the engine's own random-opening and balance process. The exact file currently has SHA-256:

```text
974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222
```

Problems:

- it is not versioned;
- repeated experimentation over the same first subset encourages overfitting;
- balance is judged by NGN, which can preserve NGN-specific blind spots;
- the lines are visibly broad but often low-quality random openings;
- the current 5,000-game cap consumes only the first 2,500 color-reversed opening pairs;
- cloud slicing and per-shard caps consume a different subset.

Recommended:

- version the exact opening set or a reproducible generator manifest;
- freeze a development book and a never-touched holdout book;
- use an independent engine or external corpus to balance the holdout;
- deterministically shuffle by recorded seed;
- report opening hash, count, subset, and order in every run;
- add phase-specific books only for explicitly phase-specific hypotheses;
- periodically validate survivors on a materially different book.

More games on repeated openings are useful when real-clock timing creates fresh variation, but they are not equivalent to more independent positions.

### 6. The external gauntlet is useful but overinterpreted

The external gauntlet should remain periodic, not per-change. The user's intuition here is correct.

However, the reported confidence interval is conditional on:

- published anchor ratings being exact;
- the logistic model being correct;
- anchor versions being independent;
- no family-specific non-transitivity;
- the selected TC/hardware mapping cleanly to the cited list.

Two versions from the same engine family are not two independent rating systems. If NGN scores above 50% against every anchor, the pooled result is an extrapolation; the code correctly prints that warning. The strategy docs still promote the extrapolated point estimate too aggressively.

Use the gauntlet for:

- periodic transfer checks;
- broad absolute calibration;
- loss collection;
- detecting family-specific weaknesses.

Do not use it for:

- resolving +3 to +10 Elo per-change effects;
- attributing a batch to individual changes;
- claiming a narrow absolute CI from one opponent family.

For the “best Go engine” claim, direct multi-family matches under a frozen public-like configuration matter more than repeatedly extrapolating a Blunder ladder.

### 7. Cloud aggregation does not preserve the local decision statistic

`cmd/sprt` now treats pentanomial LLR as the decision statistic. `scripts/cloudsprt.sh fetch`, however, parses only final W-D-L totals and recomputes a trinomial LLR.

That loses:

- pair buckets;
- the variance reduction from reversed-color pairs;
- the exact statistic that stopped each shard;
- complete-vs-incomplete pair information.

There is an additional design issue: shards can stop independently, then their sequentially stopped W-D-L totals are naively pooled. That is not the same as one global SPRT.

Before cloud results are treated as canonical:

- persist penta buckets per shard in a machine-readable result;
- aggregate complete pair buckets, not only W-D-L;
- compute one global statistic;
- either use fixed-N shards or implement a central stop decision;
- calibrate A/A false-positive behavior at the exact shard and early-stop policy;
- distinguish “run ended” from “statistical verdict reached.”

The local single-process box avoids most of this and is currently the cleaner per-change instrument.

### 8. The gauntlet loses integrity metadata in cloud pooling

Local gauntlet results retain end reasons and can exclude an anchor when forfeit/error games exceed the configured ceiling. Cloud tally JSON contains only:

- version;
- rating;
- W-D-L.

When tallies are pooled, reason counts are reconstructed as empty. The pooled cloud report therefore cannot enforce the local taint rule or distinguish chess losses from process failures. The Jun-21 run required manual log inspection to establish that “crashes” were only startup banners.

The gauntlet record also does not store real-clock configuration. A `-tc` run is passed to `report()` with the default movetime value, the footer always prints the movetime-bias warning, and rolling history can classify a real-clock run as a movetime run.

Fix the tally and history schemas to include:

- TC mode, base time, and increment;
- concurrency and hardware class;
- opening hash;
- every game-end reason;
- flag-outs by engine;
- crash/illegal/no-move counts;
- actual games rather than requested games;
- binary and harness hashes.

### 9. UCI timeout code uses blocking reads

`internal/uci.WaitFor`, `GetMove`, and `Analyze` check a wall-clock deadline around `bufio.Scanner.Scan()`, but `Scan()` itself blocks. If an engine emits no line, the loop cannot reach its next deadline check. In `GetMove`, the forced `stop` deadline is likewise only checked between output lines.

The outer SPRT watchdog mitigates wedged games by killing engines, but startup handshakes and some gauntlet paths can still hang in a blocking scan. Replace direct scanning with one reader goroutine feeding a channel, then select on:

- output line;
- stop timer;
- hard timeout;
- process exit;
- cancellation.

Until this is fixed, “timeout” comments overstate the guarantee.

### 10. Real-clock concurrency needs a required A/A preflight

The harness itself warns that real-clock concurrency is load-dependent. On an 8-core/16-thread 9800X3D, concurrency 8 fully occupies the physical cores with one active engine per game and leaves limited room for the driver and OS.

That may be a good high-throughput configuration, but it must be validated at the exact:

- OS;
- power profile;
- CPU affinity;
- TC;
- increment;
- concurrency;
- engine build flags;
- background-load policy.

Required preflight:

- identical binary vs itself;
- zero asymmetric flag-outs;
- no color or worker bias;
- stable throughput;
- penta estimate centered near zero;
- repeated run with a different deterministic opening shuffle.

Do not inherit a null result from a different machine or concurrency.

## Reproducibility defects

### 1. A git SHA is insufficient for dirty-worktree binaries

The current candidate includes uncommitted `engine/moveorder.go` and `engine/search.go` changes. `HEAD` therefore does not identify the candidate.

Every run needs:

- base commit;
- candidate commit or patch hash;
- `git diff --binary` checksum;
- binary SHA-256;
- harness binary SHA-256;
- Go version;
- GOOS/GOARCH/GOAMD64;
- CPU and OS;
- exact command;
- environment variables;
- opening checksum;
- start and end timestamps.

### 2. Critical evidence is gitignored

The strategy depends on local-only artifacts:

- `output/sprt_openings.txt`;
- `output/sf_winprob_16k.labels`;
- cloud logs;
- gauntlet logs;
- candidate binaries.

Raw multi-gigabyte data does not need to live in git, but compact evidence must:

- manifest;
- checksum;
- summary;
- penta buckets;
- failure counts;
- verdict;
- link or location of archived raw logs.

At present, future agents must trust prose in `TODO.md`.

### 3. The result ledger is prose, not data

The project should have a tracked append-only experiment ledger, for example:

```json
{
  "id": "2026-06-22-corrhist-aux-v1",
  "status": "invalid",
  "reason": "aux correction tables not reset on ucinewgame",
  "base_commit": "d46c48c...",
  "candidate_patch_sha256": "...",
  "base_binary_sha256": "...",
  "candidate_binary_sha256": "...",
  "harness_commit": "...",
  "mode": "tc 10+0.1",
  "concurrency": 8,
  "openings_sha256": "974e4b...",
  "decision_rule": "predeclared rule text",
  "games": 1186,
  "pairs": 593,
  "penta": [0, 0, 0, 0, 0],
  "verdict": "INVALID"
}
```

Human-readable summaries can be generated from this. They should not be the primary record.

## Documentation and repository cleanup

### Must clean before another autonomous night

1. **Replace the top of `TODO.md` with one current runbook.**  
   Keep current state, one ranked queue, and one decision table. Move superseded narratives to an archive.

2. **Remove contradictory live instructions.**  
   In particular, reconcile per-change gauntlet, fast SPRT, proxy keeps, and commit-before-verdict behavior.

3. **Fix `ClearHistoryTable()` for all new correction tables and add reset tests.**

4. **Make `make test` fail when tests fail.**  
   The current recipe catches `go test` failure and exits 0. An unattended agent can therefore see a false green build.

5. **Preserve reason counts and TC metadata in cloud gauntlet results.**

6. **Make UCI read deadlines real rather than wrapping blocking `Scanner.Scan()` calls.**

7. **Version or checksum the opening book and every corpus used for a verdict.**

8. **Record exact binary and patch hashes.**

9. **Disable autonomous `git checkout <files>` cleanup.**  
   Run candidates in isolated worktrees. Destructive file restore in the shared worktree can discard unrelated user edits.

10. **Do not let the overnight agent edit the harness it is using.**  
   Harness changes require a separate validation cycle and A/A run.

### High-priority cleanup after the night

1. **Split planning from history.**

   Suggested files:

   - `ROADMAP.md`: stable strategic direction;
   - `RUNBOOK.md`: current experimental rules;
   - `experiments/results.jsonl`: append-only structured results;
   - `docs/archive/`: historical reviews and superseded plans;
   - `TODO.md`: short actionable queue only.

2. **Delete or clearly label stale authority.**

   Examples:

   - `README.md` reports an older strength range;
   - `CLAUDE.md` reports another range;
   - `TODO.md` reports 2655 and 2716 in different sections;
   - `docs/11-road-to-3000-census.md` still marks root PVS/LMR as missing even though it landed;
   - `docs/09-invariants-and-joints.md` is anchored to an old commit and contains stale line references;
   - `DIAGNOSIS.md`, `review.md`, `performance_brief.md`, and ignored `gpt_review.md` are historical and should not appear current.

3. **Correct feature claims.**

   The README and CLAUDE file advertise:

   - Polyglot support, while the Polyglot hash test is skipped because real `.bin` lookup is broken;
   - Syzygy support, while probing is deliberately hard-disabled because the decoder is a placeholder;
   - configurable `Hash` and `Threads`, while both UCI options are stubs.

   These are not cosmetic discrepancies. They affect reproducibility, rating configuration, and the definition of “best.”

4. **Reduce narrative commits.**

   Since `origin/main`, 69 of 132 commits are documentation-only and 81 touch `TODO.md`. Record experiments in the ledger and make one concise documentation update per completed experimental batch.

5. **Clean local artifacts.**

   Current ignored footprint is approximately:

   - `output/`: 947 MB;
   - `build/`: 379 MB.

   Add a retention policy:

   - keep manifests and summaries indefinitely;
   - compress accepted/important raw logs;
   - expire routine failed-run logs after a fixed period;
   - keep reproducible corpora by checksum and documented storage location;
   - remove stale binaries and duplicate profiles.

6. **Separate formatting from dependency mutation.**

   `make format` also runs `go mod tidy`. Formatting should not silently change module metadata.

## Recommended overnight protocol

The goal of one night should be one valid answer, not the maximum number of edits.

### Allowed autonomous actions

- create isolated worktrees;
- build exact base and candidate binaries;
- calculate hashes;
- run unit, regression, and invariant tests;
- run an A/A preflight;
- launch one pre-registered A/B match;
- monitor health;
- stop on a predeclared statistical bound or hard cap;
- write a result manifest and raw log;
- leave the shared worktree and `main` unchanged.

### Forbidden autonomous actions

- invent a new feature after a result;
- reinterpret the decision rule mid-run;
- commit or revert production code;
- edit tests to make a candidate green;
- edit the tournament harness;
- stack another behavior change;
- close an entire research lane;
- update absolute rating claims;
- launch paid cloud resources without explicit authorization;
- use a killed or invalid run as evidence.

### Phase 0: freeze the experiment

Write a manifest before games start:

- hypothesis;
- mechanism;
- exact one-change diff;
- base;
- candidate;
- primary metric;
- TC and concurrency;
- opening set and order;
- acceptance, rejection, and inconclusive rules;
- maximum games;
- known costs;
- what action each outcome permits.

### Phase 1: validate the apparatus

1. `go test -short ./...` with a writable, explicit `GOCACHE`.
2. Candidate-specific regression tests.
3. Exact reset-state tests.
4. Binary hash capture.
5. A/A null at the exact intended environment.
6. Confirm zero asymmetric flags, crashes, illegal moves, and watchdog voids.

### Phase 2: run one isolated comparison

For the current chain, the valid comparisons are:

```text
A = 31bb779 pre-damping base
B = A + 50-move damping
C = B + fixed auxiliary corrhist
```

Run either:

- B vs A, or
- C vs B.

Do not run C vs A and infer both components.

Given limited overnight time, pick the higher-value unresolved question and devote the whole night to it. If the result is inconclusive, the correct outcome is `INCONCLUSIVE`, not a narrative keep.

### Phase 3: morning handoff

Produce:

- manifest;
- exact W-D-L;
- complete penta buckets;
- Elo estimate and interval;
- LLR and bounds;
- flags/crashes/watchdogs;
- throughput;
- invalidity checks;
- one of `ACCEPT`, `REJECT`, `INCONCLUSIVE`, or `INVALID`;
- no source mutation based on the result.

The human can then authorize the git action.

## Recommended long-term experimental loop

### Per-change loop

1. Mechanism review against live code.
2. One isolated patch.
3. Deterministic correctness and behavior gates.
4. A/B paired self-play on a development opening set.
5. Completed predeclared verdict.
6. Promotion to a candidate branch, not immediately to the production anchor.

### Holdout loop

After a small batch of accepted changes:

1. test the batch against the pre-batch anchor;
2. use a holdout opening set;
3. use a different TC or node regime;
4. if it fails, bisect using fresh holdout subsets;
5. do not reuse the failed holdout indefinitely for tuning.

### External transfer loop

Periodically, not per change:

- play several engine families;
- use exact frozen options and hashes;
- include at least one stronger opponent so results bracket 50%;
- capture losses for diagnosis;
- report conditional game-sampling uncertainty separately from anchor/list uncertainty.

### Absolute-strength loop

Define the target before claiming “best Go engine”:

- rating list;
- time control;
- hardware;
- number of threads;
- hash;
- opening book policy;
- tablebase policy;
- pure Go requirement;
- HCE vs NNUE class.

Without this, “best” can change meaning after every result.

## Whole-project technical assessment

### The engine is not blocked by a lack of feature ideas

The repository has more candidate mechanisms than it can measure reliably. Adding more ideas is lower value than:

- making experiments reproducible;
- fixing harness/statistical mismatches;
- auditing correctness joints;
- reducing per-node overhead;
- implementing real thread support if the target permits multiple cores;
- eventually implementing NNUE if the goal is the unrestricted Go-engine crown.

### The current HCE-first constraint is coherent only as an intermediate class goal

A pure-HCE target can be valuable and technically interesting. It is not the same as “best Go engine in the world” if the leading Go engines use NNUE. The roadmap should state two separate goals:

1. strongest pure-Go HCE engine under a frozen one-thread configuration;
2. strongest unrestricted Go engine, where NNUE and threading are required workstreams.

The threshold “no NNUE until 2800” is a human project constraint, not an evidence-derived engineering optimum. That is acceptable, but it should not be presented as the fastest route to the unrestricted crown.

### Single-threading is a strategic limitation

The engine advertises a `Threads` option but does not implement it. Whether that matters immediately depends on the chosen rating list. It is still a major gap for an unrestricted “best engine” objective.

### Speed deserves a first-class lane

The project correctly notes a substantial per-node speed deficit against another Go engine. Fixed-node self-play hides that cost. Every behavior candidate should report:

- nodes per move;
- depth distribution;
- wall time;
- NPS on a phase-diverse basket;
- memory footprint;
- real-clock game result.

Speed work should use exact behavior identity where possible. That is one of the few lanes where large amounts of deterministic engineering can accumulate without statistical selection noise.

## Final recommendation

Let the machine run all night, but narrow what “the guy” is allowed to do.

The overnight agent should be an experiment operator, not an autonomous research director. It should execute one pre-registered, isolated, high-game-count comparison and preserve evidence. The current corrhist result is invalid because correction state leaks across games, and the current stack design cannot attribute effects anyway.

After the reset bug and run-manifest gaps are fixed, the preferred cadence is:

> many paired self-play games per isolated candidate; strict completed verdicts; holdout batch certification; occasional multi-family external calibration.

That preserves the user's correct “more games is better” instinct while preventing the project from becoming very confident about the wrong target.
