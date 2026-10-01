# Quiet-promotion gate evidence

These are intentionally retained experiment artifacts, not installed engine
changes. See [the run record](../2026-09-12-qpromo-gate.md) for the frozen protocol,
completed inconclusive result and next experiment.
Patch files retain exact unified-diff context whitespace; do not reformat them.

- `quiet-promotion-*.patch`: exact production and regression diffs against
  `b7ffdc8`, shelved at the fixed cap.
- `build-test-receipt.json`: WSL build, source, patch and test-log identities.
- `decision.json`, `terminal.json`, `inventory.sha256.json`: byte-exact completed
  run receipts; the inventory names files in the frozen WSL run, not this folder.
- `resource-amendment-receipt.json`, `run001-supervisor.json`: exact prelaunch
  manifest-bound provenance retained outside run 002 by the original driver.
- `review-failure-v2.json`, `review-failure-v3.json`: preserved failed review
  attempts exposing those two absent run-local copies. No match reran.
- `verify-terminal.py`, `independent-review.json`: exact final v4 independent
  checker and passing result, including explicit bindings to both external
  provenance receipts and reconstructed paired statistics.

Large raw PGNs, traces, process samples, binaries, model, earlier verifier
versions and all unchanged original files remain under
`/home/ehrli/repos/ngn-qpromo-gate-20260912/` on WSL. The main completed run is
`run-002`; independent review outputs are in sibling `review-run-002`.
The original failed control remains `run-001` and is not pooled with run 002.
