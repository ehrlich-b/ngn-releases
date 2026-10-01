# Existing Rodent Anand network: full-refresh compatibility

Active objective remains **raise the rating to 3300**, using existing networks.
This inactive evaluator compatibility slice is necessary implementation work,
not an accepted playing-strength improvement or completion of that objective.

## Admission

Immediate source base: `270c73563139996d03f3c34b2840015b8e130df9`.
Root read the [exact-network contract](2026-09-13-rodent-anand-integration-contract.md)
and an independent Sol reviewer confirmed the released binary's practical
full-refresh raw oracle. Sol is authorized to implement Slice A only:
strict immutable loader, portable full refresh, exact raw output and release
material scaling, with deterministic exact-release corpus verification.

The network remains an external file, SHA-256
`5f7480b56538e9e64ee02fb5b4842bf0394ee44b707dae3a63ec7f902abd0afb`.
The raw oracle is the exact testers executable, SHA-256
`9c68d7b39dc933eff5fd2da88bd4be1b0d009f6d2add7485c121cfc0d1308531`.
Tagged source is supporting mechanism evidence, not a reproducible-release claim.
In particular, the released Anand settings use scale 192 rather than the tag's
default 400. Preserve the release's asymmetric material formula and absence of
pre-100-ply rule-50 attenuation as compatibility; do not quietly fix either.

## Boundary

- Standalone `rodenteval` package; no engine/UCI selection or default change.
- No incremental context or AVX2 in this first slice.
- No model embedding, redistribution, training or deployment.
- Synthetic malformed-input/arithmetic/mapping tests plus a frozen 50–100 FEN
  corpus generated from the exact released executable, without score selection.
- Distinguish release-observed raw output from final static values derived using
  its independently inspected machine-code material formula.
- Stop on unexplained mismatch; never fit a scale to make the corpus pass.
- Incremental parity, realistic speed and paired-clock strength remain required
  before a future playing-backend adoption. Rodent's rating does not transfer
  automatically with its weights.

## Status

**Slice A accepted as inactive compatibility code.** Root and the independent
Sol reviewer accepted the final seven-file `rodenteval` package. All local source
hashes match the tested WSL files. No playing backend is selected, no model bytes
are bundled, and 3300 remains unverified. The separately admitted Counter minor-
correction experiment does not include this package in its frozen binaries.

The strict loader bounds reads to expected size plus one before rejecting excess
data. Tests distinguish int16 accumulator wrap, int32 output-sum wrap, negative
division truncation, perspective/mirror mapping, invalid boards/models and the
released material asymmetry. This is full refresh only, not incremental parity.

The exact released executable returned two identical passes over 72 unique full
FENs: 24 board layouts, both sides to move, with clocks 0/50/99 on six layouts.
These are coverage cases, not 72 independent chess positions or strength data.
Both NGN raw scores and the separately derived final static match every record.
The release itself exposes raw scores only; final-static authority is its
preserved machine-code adapter, not a runtime final-static oracle.

Completed WSL checks, all exit zero:

- Package race test: PASS, 1.022 seconds.
- Repository `go test -short ./... -count=1`: PASS.
- Opt-in exact-release oracle test: all 72 subtests PASS, no skip.

The [verification receipt](2026-09-13-rodent-anand-artifacts/verification/receipt.json)
binds exact commands, source hashes and retained logs. Receipt SHA-256:
`77a894f45de61f5e8b3cc53dd45f1bb6b22ff48a0c75665485bdabda97e5da50`.
Final source patch SHA-256:
`2c2811139e7deeda9b296c504ca802d837a953e1afbb4ca1ea1eec28b8cff61a`.
The [oracle corpus](2026-09-13-rodent-anand-artifacts/oracle/oracle.json) is
`2685ecfae535fedd55fdce00d8c816b60955cf4f93873bda836366e409f97cb8`.
The adjacent archive retains the verbatim engine input/stdout/empty stderr,
frozen driver/wrapper and clean driver/supervisor receipts. No engines, weights,
Go cache or telemetry are archived.

Exact oracle run: `/home/ehrli/rodent-v1.1-anand-oracle-20260913/run-003`.
Its supervisor completed in 8.105 seconds with empty error state and no survivors.
Run-001 is preserved and excluded: engine output completed, but auxiliary Go
disassembly failed because its isolated environment lacked HOME/GOCACHE.
Source-v2 repaired that setup but was never run; static review found the duplicate
game1/game2 root. Source-v3 consolidated that root with combined provenance before
the successful fixed corpus run. No score-based selection occurred.

Independent review remains WSL at
`/home/ehrli/rodent-v1.1-anand-oracle-20260913/review-v2/review.json`, SHA-256
`f5f5ff3ddd41d77e8d5a5b5aa8c82f801e1d43fa2517e9dbd54eb5fc5cab7333`.
It checked all transcript/corpus bindings, deterministic passes, mirror pairs,
clock invariance, material-formula mutation witnesses, exact release settings,
termination and inventory. Its verbose cache inventory is deliberately not copied.

Next: portable incremental context with lane-for-lane refresh restoration,
king-half transition, special-move, null/pop and worker-ownership tests. Engine
integration and speed/strength adoption remain later, separately gated slices.
