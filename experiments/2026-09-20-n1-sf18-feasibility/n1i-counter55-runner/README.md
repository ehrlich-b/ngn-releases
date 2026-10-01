# Candidate match runner v1

The Python validator is authoritative; `manifest.schema.json` documents its external JSON shape. The frozen `ngn-pre-m4c-pinned-one-worker-v1` profile remains the same-binary, one-worker NGN HCE versus NGN-v1 diagnostic. The added `ngn-m4c-time-controlled-smp-v1` profile is HCE-only and admits manifest widths 1, 2, 4, or 8. It requires requested Threads, UCI Threads, effective Threads, GOMAXPROCS, and the role environment to agree, with one total 128 MiB Hash per role.

The M4c profile runs at concurrency one under the exact parent CPU mask `0,2,4,6,8,10,12,14` and does not pass fastchess `-use-affinity`. Both engine roles inherit that whole mask; `/proc` observations verify the mask and each role's declared GOMAXPROCS. Changing a worker count within this profile does not multiply Hash.

The manifest first exists in HELD state with the exact null approval block. `manifest.review_subject_sha256` canonicalizes the complete JSON with sorted keys and compact separators after replacing only status and approval with their HELD values. Root reviews that digest, then changes only those two authorization fields. An approved run recomputes the subject digest before copying or launching anything. The CLI approval token must also equal the approved manifest token.

Both roles send options in this exact order: Threads, OwnBook=false, EvalFile, EvalBackend, Hash, Move Overhead. EvalFile therefore stages a validated NGN-v1 file before selecting that backend. HCE uses `<empty>`. The role preflight uses sequential `isready` barriers, checks the selected evaluator, preserves OwnBook=false through `ucinewgame`, and requires a real start-position depth-2 search with more than one node and no fixed book signature. For M4c, the Threads barrier separately proves the configured width. Its non-node search proves exactly one matching per-go effective-width line when Threads is greater than one and proves omission for Threads one.

The wrapper copies every reviewed source, binary, network, opening, and audit tool into a new run directory. It uses the accepted bounded process supervisor for preflights, the match stage, operational trace audit, and independent chess audit. Match children are checked through `/proc` for exact executable, working directory, CPU mask, and GOMAXPROCS. For M4c, the trace audit binds every clock-controlled `go` to its role and requires exactly one matching configured/effective-width line before `bestmove` for widths greater than one. It rejects missing, duplicate, stale, late, mismatched, and node-limited width evidence. The existing checks still reject engine errors, warnings, illegal move/PV reports, nonzero exits, unexpected stderr, option-order drift, and probable embedded-book signatures. The independent frozen auditor validates complete opening histories, every played move, natural terminal state, paired orientation, final EPD, telemetry, and reports zero probable embedded-book signature plies.

No timed match is authorized by this source. A concrete manifest remains HELD until root supplies its bound review digest, token, and exclusive match-window approval.

## Prospective paired SPRT profile

`classification=prospective-strength` is a separate HCE-only M4c profile. It fixes the candidate as the first role at Threads/GOMAXPROCS 8, the reference as the second role at 1, Hash 128 MiB total per role, Move Overhead 100 ms, `30+0.3`, concurrency 1, the full eight-physical-core parent mask, and no fastchess affinity. Its hypotheses are normalized Elo 0 versus 20 with alpha=beta=0.05; they are not an ordinary-Elo +20 claim.

The manifest's 400 games and 200 pairs are maxima. A successful early stop therefore has an even audited game count below 400. The independent chess audit establishes complete ordered pairs and legal terminal positions. `sprt_audit.py` recomputes the cumulative pentanomial LLR after every pair, requires the first boundary crossing to be the final audited pair, and reconciles fastchess's displayed direction. A 400-game run with no crossing is `INCONCLUSIVE_AT_CAP`. Process timeout, malformed output, partial pairs, or failed audits are operational failures and never statistical verdicts.

Pinned fastchess `f618e34540f94f4719ad3817950618dabe441318` does not print its SPRT completion line on an in-bound hard-cap exit because its cap comparison occurs before incrementing the completed-game counter. The independent cap verdict is authoritative. Boundary exits still require exactly one matching H0/H1 completion line.

The `counter-5.5-release-v1` capability is a narrow mixed-engine diagnostic:
NGN HCE is the first role and the exact Counter 5.5 release is the second. Both
are configured with Threads/GOMAXPROCS 1 and total Hash 128 under the same full
physical-core mask; fastchess affinity is disabled. Counter's Threads maximum
is derived from `runtime.NumCPU()` and must equal the actual admission cpuset,
so the prior max-1 receipt from a one-CPU taskset is provenance evidence rather
than a global engine limit. Counter receives only Threads, Hash, and
ExperimentSettings. Its exact two startup stderr records replace the NGN crash
log contract for that role. The profile forbids SPRT and carries no strength
claim.

The external Counter release provenance binds the official GitHub release metadata snapshot (release `v1.55.0`, asset `counter-5.5-linux-amd64`, browser URL, and 4,316,021-byte asset size) plus the exact 4,316,021-byte local binary SHA-256. No download transaction receipt is available, so the runner does not claim one. The saved one-CPU handshake proves the exact local binary's emitted UCI identity/options and startup version/build/revision/runtime fields under that one-CPU admission; it does not prove how the binary was acquired. The actual diagnostic preflight separately requires the full admitted cpuset, its resulting `NumCPU`/Threads maximum, and the same runtime identity.

## Source-only external release profiles

The additive external_profiles.py and external_uci_admission.py files pin the Counter 5.5, Rodent V1.1 Anand/testers, and Rodent V1.2 non-Tal testers identities. They run a sequential, fail-closed preflight before a later fixed-game external comparison. Rodent admission requires the exact locked Testers option surface under the match mask because the tagged source and manual disagree. The pinned V1.1 artifact is restricted to width one after the durable full-mask GOMAXPROCS=8 UCI-only probe also advertised no Threads; its T8 unavailable receipt binds that transcript and root review. No games may use a failed or revised-in-place admission.

The focused external fixture files cover identity/model/thread/book/stderr/time/stop failures. They are source-only until a supervised validation window is assigned. The complete fixed schedule and interpretation limits are in experiments/2026-09-06-final-external-counter-rodent-plan.md.

## Fixed external comparison source slice

The additive external source uses external_match_manifest.py to bind a completed
profile-specific admission receipt and transcript before a fixed cell can be
approved. prepare_external_match.py emits the exact 100-game/50-pair fastchess
and corrected v5 chess-auditor command contract. external_trace_audit.py reuses
the accepted trace parser, clock validation, error vocabulary, width receipt,
and crash-log helpers while allowing only the exact pinned external startup
profile. run_external_admission.py produces the receipt required before a cell
manifest can be frozen; run_external_fixed_match.py repeats admission against
the run-local binary copy and then owns the accepted preflight, supervised
fastchess, profile-aware trace, v5 chess audit, report, sealing, and survivor
lifecycle. external_report.py keeps pairs intact for deterministic bootstrap
reporting and adds a conservative Hoeffding interval when the observed pair
scores are degenerate.

Width-eight admission requires the exact full physical mask, Threads=8, and an
observed process CPU/wall ratio of at least 1.5 during a two-second
infinite/stop witness. An artifact that only advertises Threads remains
unavailable. No external cell may substitute a different binary after
admission.
