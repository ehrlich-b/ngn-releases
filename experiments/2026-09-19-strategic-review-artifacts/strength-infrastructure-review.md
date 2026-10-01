# Bounded strength-infrastructure review — 2026-09-19

Source/doc review only, against `f75b578`. No matches, engines, builds, or remote
checks were run for this review. Three material interpretation/admission risks
follow; these are not findings that the latest accepted games were invalid.

## 1. The current screen samples a narrow, repeatedly used opening/clock cell

The accepted Rodent-vs-Counter evaluation comparison uses the **first 200 frozen
openings**, reversed colors, `10+0.1`, and one thread
(`experiments/2026-09-13-rodent-counter-strength.md:9-15`). Its frozen driver
hardcodes `order=sequential`, `start=1`, `plies=6`, `tc=10+0.1` and a separate RNG
seed (`experiments/2026-09-13-rodent-anand-artifacts/strength-preflight/driver-held-v1.py:163-178`).
Changing that seed does not select a new sequential opening subset. The recent
helper-selection gate also used paired sequential six-ply openings and the same
clock (`experiments/2026-09-19-smp-helper-result-selection.md:106-117`).

The underlying default corpus has 5,000 lines, not 200
(`experiments/corpus_manifest.md:13-18`). Its documented generator draws random
legal moves, deduplicates **move strings**, then filters balance with NGN's own
fixed-depth search (`cmd/sprt/main.go:491-528`; default six plies and depth eight
at `:80-83`). These source properties describe the documented generator, not an
independent reconstruction of the historical corpus. The match auditor checks
exact histories and color pairing; it does not certify representative opening
coverage (`scripts/fastchess-match/audit_fastchess_match.py:340-375,448-462`).

**Material failure mode:** a real large evaluation/search gain that appears at
greater search budgets or in a different position distribution can fail this
cell; conversely, a genuine gain in this cell can be overclaimed as a general
rating gain. This review has not demonstrated book overfitting or a TC reversal.
The latest completed external Counter result remains valid for its stated cell.

**Bounded decision:** retain this inexpensive development screen. For a promoted
major architecture/evaluator winner, use one predeclared held-out, longer-clock
confirmation rather than repeatedly recalibrating all candidates. There is
already a validated independent 200-position, 16-ply Stockfish-book selection
and a documented `60+0.6` combined-test plan
(`experiments/2026-09-06-current-status.md:56,74`). That is existing infrastructure,
not a request to launch it now or a claim its games completed.

## 2. Do not use the pure-kernel cold gate to decide architectural/evaluator value

The successful Rodent update-kernel gate measures cold 400,000-node searches on
six fixtures. Acceptance requires a median time ratio <=0.97, **every fixture**
and both order strata improving, plus exact cold/warm search identity
(`experiments/2026-09-13-rodent-update-kernel.md:28-44`). The reported ~135MiB/op
includes lazy TT allocation; warm identity is checked, but the reported speed
result is cold timing. This is a good narrow gate for that exact-result kernel.

**Material failure mode:** blindly carrying that gate to deferred evaluation,
cache reuse, or a different network can miss a game-relevant improvement whose
benefit depends on warm state or whose costs differ across positions. Requiring
all six roots to improve is stronger than requiring the actual workload to
improve. This is a prospective risk, not proof that a current candidate was
incorrectly rejected.

The same-binary Rodent-vs-Counter **real-clock** match is a valid comparison of
the complete backend choices, not an equal-node estimate of network information
quality (`experiments/2026-09-13-rodent-counter-strength.md:3-15`). Source enforces
one shared executable while intentionally selecting different backends/models
(`experiments/2026-09-13-rodent-anand-artifacts/strength-preflight/driver-held-v1.py:385-401`).
In contrast, the exact-result, same-network kernel gate isolates implementation
speed. Neither evidence type substitutes for the other.

**Bounded decision:** for a runtime-only candidate, preserve the same network,
score semantics and exact-search controls, and use representative warm as well
as cold costs. For V1.2-versus-V1.1, use equal-time games after a competitive
implementation; equal-node comparisons can diagnose quality/cost but cannot
adopt the slower package. The current V1.2 integration correctly makes no
strength claim and explicitly leaves profiling and a separate game gate next
(`experiments/2026-09-19-rodent-v12-slice-c.md:44-46,79-93`). The measured V1.1
2.43x kernel improvement is concrete evidence that implementation maturity can
materially change this tradeoff, not evidence of a particular V1.2 gain.

## 3. SHELVE is not a statistical closure of a lane

The driver resamples **whole reversed-color pairs**, converts their average
score to logistic Elo, and uses the paired percentile interval
(`experiments/2026-09-13-rodent-anand-artifacts/strength-preflight/driver-held-v1.py:297-313`).
The auditor verifies exactly one pair per selected opening and both color
orientations (`scripts/fastchess-match/audit_fastchess_match.py:448-462`). This
review found no independent-game-versus-pair estimator mistake.

The fixed 400-game rule adopts only if the paired lower 95% bound is positive;
otherwise it shelves with no result-dependent extension (driver `:420-439`).
The recent actual outcome **+15.65 [-2.61,+33.98]** obeyed that rule
(`experiments/2026-09-19-smp-helper-result-selection.md:134-148`).

**Material failure mode:** treating that binary operational disposition as
evidence the architecture has no worthwhile upside. The observed interval still
permits a gain above 30 Elo. It does not hide a demonstrated 100-Elo win; the
point is that one implementation's inconclusive result cannot justify closing
the whole SMP/search lane. The 20-game A/A interval **[-126.97,+88.74]** is a
protocol smoke test, not evidence of tight timing/role equivalence (`:125-132`).

**Bounded decision:** do not extend the shelved observed match. For the next
genuinely different major candidate, predeclare a meaningful effect target and
appropriate fixed/sequential game rule before seeing scores; preserve
inconclusive versus rejected distinctions in the lane ledger. Do not sum
selected winners' point estimates into an absolute rating.

## Adjudication: current protection, not a new defect

Historical native matches used score/max-ply adjudication, and the final-ply
terminal-precedence bug really did mislabel mates as draws; that defect was
subsequently fixed and disclosed
(`experiments/2026-09-05-draw-conversion-audit.md:9-39`). Do not silently pool
those historical cells with the current protocol.

The latest external checkpoint explicitly disables adjudication
(`experiments/2026-09-13-rodent-external-counter-checkpoint.md:20-24`). The current
general candidate manifest requires all score/resign/draw/max-move adjudication
flags false (`scripts/candidate-match/manifest.py:238-239`), and the independent
auditor rejects adjudication and verifies natural final positions
(`scripts/fastchess-match/audit_fastchess_match.py:20-30,401-428`). This directly
protects current endgame/score-policy experiments from the old failure mode.
The generic HCE-vs-NGN manifest is not the dedicated Rodent driver's schema;
the Rodent report explicitly records that distinction at lines 24-29.
