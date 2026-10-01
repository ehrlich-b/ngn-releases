# Counter output kernel evidence

This directory preserves the bounded September 13 speed gate. The parent
[experiment report](../2026-09-13-counter-output-kernel.md) owns the verdict and
limitations. No network files, engine binaries or large search snapshots are
committed here.

- `driver.py` and `static-parser-tests.py`: frozen measurement driver and parser
  tests, executed on WSL only.
- `manifest-held.json`: pre-review held manifest. `manifest-released.json`
  changes only the release state. `manifest-frozen.json` is the driver's
  sorted-key serialization of that released manifest; different serialization
  explains its different byte hash.
- `candidate.patch`: exact frozen candidate, including the benchmark-only
  `engine/counter_transition_profile_test.go` probe. The probe is present in
  both measured engine binaries and is not a production engine change.
- `candidate-receipt.json`, `base-build-receipt.json`, `disassembly.log` and
  `verification/`: source, binary, numerical and baseline-test evidence frozen
  before timing. The candidate receipt's `executed: false` fields describe that
  pre-timing state; the terminal record below supersedes them for run status.
- `provenance-mismatch.md`: prelaunch file-mode correction. Original patch bytes
  were not retained; the exact replacement patch and receipt were frozen before
  measurement. The receipt's unversioned patch path has the same SHA-256 as the
  versioned patch explicitly bound by the gate manifest.
- `decision.json`, `terminal.json`, `inventory.json`: completed timing values,
  terminal verdict and hashes of the complete raw run.
- `review-v2.json` and `verify_counter_output_run_v2.py`: independent acceptance
  and its raw-log/hash verifier. The original reviewer script compared an outer
  process-supervisor command with the intentionally inner command in its receipt;
  correcting that reviewer bug required no change to the frozen run.
- `integration-receipt.json`: all ten integrated source hashes and successful
  Linux/Windows amd64-v3 packaging builds; no default binary was replaced.

The complete immutable raw run remains at
`/home/ehrli/repos/ngn-counter-output-gate-20260913/run-001` on the authorized
WSL host. It includes 42 supervised stage receipts, stdout/stderr, CPU samples,
all 15 copied inputs, and both large correctness snapshots. Both snapshots hash
to `2ffbc729295f3794883502e2c74779614623ef132844c795dc72c8d2e1dd1656`.
The original candidate and verification files remain under
`/home/ehrli/repos/ngn-counter-output-20260913/output/counter-output-20260913`.

Reproduction requires adjusting a new manifest to valid artifact paths and a
new run directory. Do not overwrite or rerun `run-001`. Preserve the existing
negative and positive results; any changed protocol is a new experiment.
