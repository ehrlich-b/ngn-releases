# H0 passive history-shadow diagnostic

Status: **PASS**; H1 factorial pilot admitted without changing the frozen
package or thresholds.

H0 is the mechanism screen for the bounded Counter-inspired quiet-history
package in the September 19 Sol plan. It will retain the accepted V1.2
evaluator and the current search tree while maintaining a write-only shadow of
the proposed completed-node labels and response rate. No shadow value may feed
move ordering, pruning, reduction, evaluation, TT storage, or time control.

The workload is selected before observer results: source games 4–11 inclusive
from the immutable accepted Counter match PGN, excluding the three games used
by earlier profiling. Selection is sequential with no result or position
filtering; the extracted artifact intentionally omits results. Each game must
have at least 64 plies. Searches will occur after plies 16, 32, 48, and 64,
retaining the same searcher, TT, actual history, shadow history, evaluator, and
repetition-bearing position inside each game, with `NewGame` between games.

The source PGN SHA-256 is
`35bdd00b28f585224e6c04392bdbf79cd4472c9ea7549b84acfc4379b07847d2`.
`extract_prefixes.py` is the frozen result-blind extractor.

## Frozen observer and execution

The disposable observer is `history_shadow_diagnostic.go.txt`; its unit tests
are `history_shadow_diagnostic_test.go.txt`, its exact search integration is
preserved by observer commit `de5d77c`, and its tagged probe is
`history_shadow_probe_test.go.txt`. The observer keeps separate shadow main,
continuation, and follow-up tables. It updates them once at the end of each
completed root aspiration attempt or interior move-loop node whose final best
move is quiet and whose final score raises that attempt/node's original alpha.
Only searched quiet moves before the final winner are negative examples. A
noisy winner creates no shadow update. Interrupted nodes and terminal or
early-pruned nodes create no label.

The held run uses the exact accepted Rodent V1.2 default model, one search
thread, `GOMAXPROCS=1`, 128 MiB hash, and 200,000 nodes per search. Baseline and
observer passes each replay all eight games in order; the same searcher, TT,
live history, shadow history, evaluator, and game-bearing position survive the
four targets within a game, and `NewGame` separates games. The observer never
feeds a live search decision. Every target's nodes, completed depth, seldepth,
root depth, best move, score, stopped bit, and PV must match the baseline
exactly.

## Predeclared H0 decision gate

Counters are summed over all 32 observer searches. H0 is valid only if all
trajectories match, the recomputed current LMR reduction has zero mismatches,
and no quiet winner is missing from its searched prefix. Subject to those
integrity requirements, H1 is admitted only when all three mechanism checks
pass:

1. New-only completed-node learning is not negligible:
   `new_only_positive_rate_units / positive_rate_units >= 0.01` and at least
   1,000 new-only labels.
2. Among later quiet beta cutoffs (`legalTried >= 2`), the response-trained
   shadow improves history-only rank: rate-weighted
   `(current_rank - shadow_rank) >= 0.10` positions and improved samples
   outnumber worsened samples.
3. The fixed policy-11 normalized LMR term changes the final reduction on at
   least 5% of measured LMR decisions.

There is also a one-way safety veto for the unchanged shallow history-prune
consumer. When at least 200 shadow-only prune candidates complete a baseline
search, H1 is rejected if more than 10% raise the move-entry alpha or more than
2% reach beta. Fewer than 200 such candidates are reported as underpowered,
not silently interpreted as safe. Main-table multi-origin and sign-conflict
rates, suppressed noisy-cutoff negatives, normalized distributions, and LMR
verification outcomes are diagnostic rather than adjustable gates.

Passing H0 admits only the already-specified four-policy H1 pilot; it is not
playing-strength evidence. Failing any mechanism check shelves this package
without tuning a threshold or constant from the observed data.

## Result

The valid run at observer commit `de5d77c` completed 32 baseline and 32
observer searches in 46.53 seconds. The deterministic archive rerun produced
the identical raw `probe.json` SHA-256
`33a3a9a605f760700991c9b5840583c62ab6b2e7f9be5b0f38cbdeaf69f61a63`.
All search trajectories matched, the current-LMR recomputation had zero
mismatches, and no winning quiet was missing from its searched prefix.

All predeclared gates passed:

- new-only weighted learning mass was 5.374% with 6,331 labels;
- the 82,803 later quiet cutoffs improved by 0.1048 rate-weighted rank
  positions, with 34,239 improvements versus 20,943 regressions;
- policy 11 changed 19.142% of 1,332,618 LMR decisions;
- 209,417 completed shadow-only prune candidates raised alpha/beta 1.402% of
  the time, below both safety vetoes.

The shadow policy is aggressive at the unchanged pruning threshold (221,420
shadow prunes versus 8,247 baseline prunes), and the retained coarse main key
has substantial aliasing (60.120% cross-origin sign-conflict updates). Those
were predeclared diagnostics, not post-hoc gates; they are material risks for
H1 to resolve in games. `result.json` contains the compact adjudication and
exact totals; `probe.json` is the complete per-target receipt.
