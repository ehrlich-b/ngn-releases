# C0 clock-mechanism preflight

C0 confirms the suspected mechanism and admits exactly one C1 candidate. In all
12 frozen warm Rodent V1.1 searches, the first decisive stop was a **soft abort
inside an already-admitted iteration**. There were zero soft-admission stops,
hard deadlines, emergency stops or attribution ambiguities. The measured time
after the last completed iteration was 25.74% of aggregate search time, closely
matching the older 25.529% log-derived tail without treating either number as
recoverable Elo.

## Observer contract

The opt-in observer is nil in ordinary play, emits no UCI output and uses fixed
storage. It latches the first decisive reason among soft admission, soft active
abort, hard deadline, emergency reserve, external stop, node cap, terminal and
requested-depth completion. Completed-iteration records contain the whole
iteration duration and aspiration-attempt count.

Deterministic tests prove:

- hard deadline precedes emergency when both conditions coincide;
- a soft active abort remains soft while the search unwinds past the hard time;
- a node-limit request cannot be relabeled as external through the shared stop
  bit, while a genuinely pre-existing external request wins;
- setup expiry is recorded as hard rather than as the stop bit it causes;
- three aspiration segments of 30, 40 and 50 ms produce one 120 ms iteration;
- reset clears the observer, time-check counter and cached stop;
- terminal and depth completion are distinct; and
- observer-off and observer-on 50,000-node searches return identical node count,
  move, score, completed depth and selective depth.

The first focused run found one missing checkmate return site in the observer,
not a search-policy defect. That path was instrumented and covered before the
diagnostic. A probe-only compile error from formatting `Move` as a string was
also corrected before the retained run.

## Frozen diagnostic

- Source base: `7949b5d`.
- Exact Rodent V1.1 Anand model SHA-256:
  `5f7480b56538e9e64ee02fb5b4842bf0394ee44b707dae3a63ec7f902abd0afb`.
- Linux amd64.v3, Threads 1, Hash 128 MiB, `GOMAXPROCS=1`.
- Tournament clock 10+0.1, three fixed-order passes over start position and the
  retained ply-23/ply-48 game-prefix positions; one persistent searcher keeps
  TT and history warm.
- No node cap. Each returned move was legal and every root position was restored.

| Result | Value |
|---|---:|
| Soft active aborts | 12 / 12 |
| Soft admissions | 0 / 12 |
| Hard/emergency stops | 0 / 12 |
| Aggregate observed tail | 2.017880 s / 7.839050 s = 25.74% |
| Median observed tail | 192.049 ms |
| Tail range | 0.076–428.160 ms |
| Median tail/search fraction | 22.17% |
| Completed depth range | 14–27 |
| Median elapsed/raw-soft ratio | 0.7547 |

The raw soft allocation is not the active composed threshold: stability,
node-effort and the previous-iteration projection intentionally move the stop.
That is why elapsed/raw-soft spans 0.4757–1.1703 without any hard-budget breach.
The decisive observation is the call site: every soft stop occurred after an
iteration had already begun.

## Overhead and validation

The hot-check benchmark recorded median 0.9447 ns/op observer-off versus 0.9477
ns/op observer-on (1.0032x), with zero allocations in both. The whole 400,000-node
benchmark retained 8 allocations and 135,409,024 bytes/op in both modes; its
grouped-order medians happened to favor observer-on by 3.03%, which is noise and
is not claimed as a speedup. Together with exact fixed-node identity, this shows
no measurable observer penalty.

WSL gates on the final source:

- focused observer suite: pass;
- complete engine short suite: pass (7.680 s);
- complete engine short race suite: pass (33.234 s);
- repository-wide short suite: pass (engine 6.826 s);
- worktree diff check: pass;
- hopper PID 3099092 remained running and was not reconfigured.

Exact rows, benchmark summaries and source hashes are in `result.json`. The
inert warm-search/overhead probe is `c0_clock_diagnostic_test.go.txt`.

## Decision

Admit one C1 implementation: keep the existing soft formula and 1x previous-
iteration projection, apply them only at the unthrottled new-depth admission
boundary, and make recursive/aspiration/root-move checks hard/emergency-only.
This is a time-policy candidate, not a proved improvement; it still requires the
deterministic boundary tests, frozen budget audit and one no-extension game gate
specified in the Sol plan. If it fails, shelve it and move directly to V1.2 SIMD.
