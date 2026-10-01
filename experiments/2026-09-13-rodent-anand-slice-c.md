# Rodent V1.1 Anand engine integration

Status: accepted as an opt-in existing-network backend. No default change,
network bundling, deployment, measured speed gain or strength claim.

Sol implemented and verified Slice C on WSL against base `203830a`. The eleven
integrated production/test files match the exact hashes in the
[receipt](2026-09-13-rodent-anand-artifacts/engine-integration/receipt.json).
The final isolated patch is SHA-256
`0101bc5c6cee25dcf80529a8dd917f8d863adb645f13124d993c092b569038e3`.
Subsequent main commit `b694341` changes only the future-run Python auditor and
documentation, not the Go tree tested here.

## Behavior

- `rodent-v1.1-anand` is available through startup flags and the existing
  `EvalBackend` / `EvalFile` staging machinery. The strict loader accepts only
  the pinned external Anand artifact; no generic Bullet inference or fallback.
- Immutable model identity includes the backend, adapter revision, HCE generation
  and exact model metadata. Each search worker owns its mutable context.
- Direct bitboard/move conversion supplies checked ordinary, capture, promotion,
  en-passant, castle and null transitions. Search restores parent frames on unwind.
- Both ordinary and emergency score routes preserve the release's static formula
  without rule-50 damping, then apply the approved engine-only `[-25000,+25000]`
  clamp to keep static values out of NGN's mate namespace. The package API remains
  exact and unclamped.
- Transactional selection, TT/history invalidation, SMP ownership and existing
  search/correction policies retain their contracts. HCE remains the default.

## Verification

WSL Go 1.25.5, GOMAXPROCS 2, package concurrency 2:

- Full short suite: PASS.
- Full short race suite: PASS.
- Configured `rodentoracle` engine tests under race: PASS, including actual
  stopped-child traversal/unwind, incremental versus full-refresh search identity,
  explicit move/null/pop, clock identity, three-worker isolation/reset, full-history
  and TT switching/warm-state assertions, and failed file-replacement atomicity.
- Retained Slice B oracle: PASS, all 22 sequences / 75 checkpoints.
- Formatting and integrated file hashes: PASS.

The adjacent [checks summary](2026-09-13-rodent-anand-artifacts/engine-integration/checks.log)
records commands and observed results; it is an authored execution summary, not
raw Go stdout. An initial mistyped command exited before Go ran and was corrected;
it is not concealed as a passing test. Independent Sol production review and
the focused review of strengthened test assertions both accepted the final scope.
Root reviewed the adapter and exact integrated hashes. All computation ran on WSL.

## Next

Measure realistic portable cost using the existing six fixed-node search fixtures
and a bounded profile. Optimize a demonstrated hotspot with exact behavior
identity before a separately predeclared real-clock strength gate. The backend's
availability does not establish that NGN benefits from it, or that NGN is 3300.
