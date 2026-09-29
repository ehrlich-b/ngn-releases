# NGN K4 Bullet harness

This directory owns the auditable training wrapper for NGN's K4 NNUE. It is
built against Bullet commit
`629ee50000b2afb7b3337595401c830d3b1e0f42`, plus the repository-local patch
`ngn-k4-bullet.patch` (SHA-256
`f7f5e0dd02695fe57ec58a630b63006da96cb1ec6b8fd4769a4e6308a8121e54`).
The patch makes Bullet's configured batch queue size real; upstream otherwise
hard-codes a queue of 32. It also adds the direct serde dependencies used by
the harness.

`prepare_snapshot.sh` refuses a dirty or mismatched upstream tree, verifies the
four patched base files by hash, creates a new execution snapshot, applies the
patch, installs `ngn_k4_train.rs`, and records the Bullet, patch, and harness
identities in `NGN_K4_EXECUTION_IDENTITY`.

## Frozen modes

| mode | finalized manifest | BF records | updates | checkpoint interval |
| --- | --- | ---: | ---: | ---: |
| smoke | pilot | 1,000,000 | 256 | 128 |
| pilot | pilot | 1,000,000 | 1,024 | 128 |
| pilot-s1 | pilot | 1,000,000 | 16,384 | 512 |
| pilot-lr1 | pilot | 1,000,000 | 16,384 | 512 |
| probe5m | probe5m | 5,000,000 | 32,768 | 1,024 |
| probe5m-lr1 | probe5m | 5,000,000 | 32,768 | 1,024 |
| main | main | 20,000,000 | 16,384 | 512 |
| main-m1 | main | 20,000,000 | 131,072 | 4,096 |

`pilot-s1` is the schedule-only diagnostic on the exact pilot BF. Its longer
cosine horizon leaves the model, target, seed, batch size, and data order fixed.
`pilot-lr1` repeats that horizon on the exact same BF and seed with only the
cosine learning-rate endpoints reduced fivefold (`0.0002` to `0.00001`).
`probe5m-lr1` repeats `probe5m` on the exact same finalized 5M BF with those
same reduced learning-rate endpoints.
The `main` row records the withdrawn original schedule. `main-m1` is the
predeclared replacement for a 20M corpus, held pending the 5M quality gates in
the failure analysis. Its selection rule uses a 32,768-update minimum and six
non-improving 4,096-update checks.
`probe5m` consumes the exact pilot prefix plus the first four million accepted
expansion inputs, with all three fixed holdouts inherited.

All modes use seed 26092001, batch size 16,384, two loader threads, and an
actual queue depth of eight. Except `pilot-lr1` and `probe5m-lr1`, they use a cosine learning rate
from `0.001` to `0.00005`. Targets are 100% record score
(`result_weight=0`) under a scale-400 sigmoid. The 768-feature factorizer is
zero-initialized, trained jointly, and merged into four transformer buckets on
export.

Archive A1 modes also support a linear game-result target ramp. Build with
`NGN_WDL25=1` for 0 to 0.25, or `NGN_WDL40=1` for 0 to 0.4. Setting both is a
compile error. Without either variable the target remains pure record score.
The weight uses the global superbatch index, so a resumed run continues the
same ramp. `attempt.json` records `result_weight_end`, which is checked on
resume; checkpoint receipts bind that attempt by hash. Old pure-score attempts
decode a missing endpoint as zero. Existing weighted attempts remain bound by
their original executable and are not migrated or rewritten. The completion
line reports the actual result-weight endpoint and complementary score weight.

The harness accepts only a complete `ngn-k4-finalized-corpus-v1` manifest. It
verifies the manifest, sampler, optional parent-pilot, and every corpus file by
size and SHA-256; enforces the exact train/validation/calibration/reserved-test
corpus set; and checks the frozen segment structure. Pilot training must be the
one-million-record `pilot/train` segment. Main training must be that exact
pilot segment followed by 19 million `main-expansion/train` records. Main
holdouts must be inherited, while pilot holdouts must not be.

## Start and resume

```text
ngn_k4_train start smoke|pilot|pilot-s1|pilot-lr1|probe5m|probe5m-lr1|main|main-m1 FINALIZED_MANIFEST NEW_ATTEMPT_DIRECTORY
ngn_k4_train resume smoke|pilot|pilot-s1|pilot-lr1|probe5m|probe5m-lr1|main|main-m1 FINALIZED_MANIFEST PARENT_CHECKPOINT_RECEIPT NEW_ATTEMPT_DIRECTORY
```

The harness refuses an existing attempt directory. `attempt.json` binds the
executable, Bullet commit and patch, finalized input, schedule, architecture,
seed, loader settings, and optional parent checkpoint. Every candidate receipt
binds the deployed float and quantized tensors, weights, AdamW momentum and
velocity, loss log, cumulative presentations, data cursor, and the exact
reader interval witnessed at runtime.

Resume restores weights, momentum, and velocity, resumes the global cosine-LR
superbatch, and asks the audited sequential reader to skip the exact cumulative
record count. Pinned Bullet AdamW has no hidden bias-correction clock, so these
files plus the externally receipted LR and data cursor are the complete mutable
training state. Before the first resumed update, the harness writes a
`resume-load-audit` checkpoint and requires its weights, momentum, velocity,
deployed float export, and quantized export to be byte-identical to the parent.
The attempt receipt binds that audit and later resumes validate the audit chain.

CUDA sparse-gradient reductions are not bit-deterministic across a fresh
process. After exact load verification, `compare` provides a bounded
continuation sanity check rather than making a false byte-identity claim about
two independently scheduled GPU trajectories:

```text
ngn_k4_train compare CHECKPOINT_DIRECTORY CHECKPOINT_DIRECTORY
```

It requires every deployed float, weight, momentum, and velocity value to be
within `1e-3`; quantized tensors may differ in at most 64 of 2,372,360
coefficients and only by one integer unit; and the Bullet trailer must match.
The smoke gate separately requires identical reader witnesses. Repeated RTX
5080 trials saw maximum float deltas of `1.80e-5`, with zero or one quantized
coefficient differing by one.

`smoke_resume_gate.sh` builds a clearly marked synthetic finalized-manifest
fixture around the existing one-million-record public smoke BF, runs 256
updates, resumes the intermediate 128-update receipt in a new attempt, and
applies those equivalence gates. Its fixture is validation data, not owned
production training data.

## Export and parity

`raw.bin` is the deployed float tensor stream, not Bullet's default raw save.
The default writer omits save-format transforms; this harness independently
merges the factorizer and transposes the output layer, then requires the
quantized output to match that transformed stream.

The probe command evaluates FENs through Bullet's feature mapper and graph:

```text
ngn_k4_train probe CHECKPOINT_DIRECTORY FENS_FILE
```

Its tab-delimited output is consumed by `ngnk4bridge parity`, which checks the
raw-to-integer tensors, an independent float evaluation, Bullet-vs-Go output,
scalar-vs-incremental Go evaluation, and the prospective quantization gates.
Checkpoint eligibility and promotion remain the responsibility of the
selector, manifest-bound bridge, parity tests, and playing gates.
