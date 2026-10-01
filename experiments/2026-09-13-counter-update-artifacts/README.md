# Counter accumulator-fusion evidence

Verdict: **SHELVE_PERFORMANCE**, no production integration. The experiment and
predeclared rule are in [the report](../2026-09-13-counter-update-fusion.md).

- `profile/`: source-line CPU/allocation reports, valid and invalid diagnostic
  receipts, deterministic prefix extractor/corpus, inert probe sources and
  exact SHA-256 inventory. The attribution amendment preserves the distinction
  between source-line affected cost and the interim caller-level proxy.
- `candidate/`: complete recoverable production/test patch, source and artifact
  hashes, exact verification scripts, logs and exit codes, actual disassembly,
  and the consolidated candidate receipt. Large snapshots and binaries stay WSL.
- `gate/`: held/released/canonical manifests, driver and parser tests, baseline
  build receipt, full paired values in `decision.json`, inventory and terminal.
  `review-v1.json` and its independent verifier confirm the complete shelf verdict.
  Canonical manifest hashing uses the driver's sorted serialization, so its
  bytes differ from the equivalent released manifest.

Frozen WSL roots:

- Profile: `/home/ehrli/repos/ngn-counter-profile-20260913/output/`;
  canonical compact archive `profile-evidence-v3.tar.gz`, SHA-256
  `188c1db81d892819598c99ad2e30608eb020d6444d827ee049dc6cf9a4683b7f`.
- Candidate: `/home/ehrli/repos/ngn-counter-fused-20260913/output/counter-fused-20260913/`.
- Complete gate: `/home/ehrli/repos/ngn-counter-fusion-gate-20260913/run-001/`.
  This includes all 42 stage receipts, stdout/stderr, resource samples,
  correctness snapshots and frozen input copies; `gate/inventory.json` binds them.

Decision SHA-256:
`dc89039bb6b7d28742bfb6238a84dff8b0730eb2be09bf49301f4ea95b3b7e74`.
Terminal SHA-256:
`e6737f9f1d15039b37cd66c25dbb5abc2f15f57c648d9cdff8851a60258665a3`.

No generated model, executable, raw profile or large snapshot is committed.
Archive probe `.go.txt` names are intentionally inert: do not rename them to
`.go` under this repository, where `go test ./...` would discover partial packages.
