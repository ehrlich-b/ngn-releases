# Owned NNUE training/resume gate — 2026-09-20

## Outcome

The K4 Bullet harness is now able to start from an exact finalized-corpus
receipt, checkpoint all mutable AdamW state, resume the global schedule and
data cursor, prove that a fresh process reloads the parent checkpoint
byte-for-byte before its first update, and bound the later CUDA continuation
against an uninterrupted run. This closes the training-mechanics gate; it does
not create or promote an owned net.

No bulk labeling, corpus generation, pilot training, or main training was
launched. At the end of the gate Windows C: had only 6.0 GiB physically free,
below the frozen 60 GiB launch requirement. WSL's ext4 filesystem had 611 GiB
logical free space, which does not satisfy that physical-disk gate.

## Frozen execution identity

- Bullet commit:
  `629ee50000b2afb7b3337595401c830d3b1e0f42`
- Local Bullet patch SHA-256:
  `f7f5e0dd02695fe57ec58a630b63006da96cb1ec6b8fd4769a4e6308a8121e54`
- Harness source SHA-256:
  `613dfd93b47b85c5456ae0d0b2e9c8ebe120cac08b4ab7d6f044dd4fae2b828f`
- Final CUDA executable SHA-256:
  `5326c23c5e64d34b017711d9009f854ae71edd10d428f71f873ca78f15eb8203`
- Execution-identity receipt SHA-256:
  `78d91716794d8df84b7d3d8f3e524b1549b8c0577d7713a2302753181490d8a6`
- WSL execution snapshot:
  `/home/ehrli/nnue-owned-k4-20260920/bullet-exec-v7`

The preparation script verifies the pristine upstream commit, clean worktree,
and exact pre-patch hashes for `run.rs`, `value.rs`, `Cargo.toml`, and
`Cargo.lock`. The local patch routes `LocalSettings.batch_queue_size` into the
trainer and replaces upstream's hard-coded queue of 32 with the receipted
value of eight.

## Resume contract

Each attempt refuses an existing output path and records:

- the executable, Bullet commit, and local patch;
- the exact finalized manifest and selected BF;
- architecture, seed, scale, schedule, batch size, threads, and real queue;
- optional parent-checkpoint receipt and starting update;
- a byte-exact immediate reload audit for every resumed attempt;
- weights, AdamW momentum and velocity, deployed float tensors, quantized
  tensors, and the optional loss log;
- cumulative updates, presentations, epochs, data cursor, and the runtime
  reader witness for the completed interval.

The pinned AdamW implementation has no hidden bias-correction clock. Its
mutable state is weights, momentum, and velocity; the attempt receipt supplies
the LR schedule position and the audited reader supplies the data position.
The parent attempt and executable are parsed and matched to the current
contract. Receipted state paths are also rebound to the files beside the parent
receipt, preventing a valid receipt from authorizing unrelated local files.

Strict manifest admission requires exactly one training corpus and the three
fixed holdouts. Pilot training is exactly `pilot/train` with one million
records. Main training is the one-million-record pilot prefix followed by 19
million `main-expansion/train` records. Parent-pilot presence and holdout
inheritance must match pilot/main mode.

## Bounded WSL proof

The final gate used the existing 1,000,000-record public smoke BF and generated
only three 100,000-record synthetic holdout prefixes. The fixture is explicitly
marked `not-owned-production-data` and cannot be mistaken for the future owned
corpus.

It ran under `taskset -c 10-11 nice -n 10` while the Lean hopper and both Lean
workers remained live. The final retained gate directory is 247 MiB:

`/home/ehrli/nnue-owned-k4-20260920/runs/harness-smoke-resume-v6`

The proof sequence was:

1. Start smoke training for 256 updates, retaining candidates 0, 128, and 256.
2. Start a new attempt from the immutable candidate-128 receipt.
3. Immediately re-save the loaded parent and require byte-identical raw,
   quantized, weights, momentum, and velocity files.
4. Train updates 128 through 256 using the same global cosine-LR index and the
   same skip count of 2,097,152 presentations.
5. Require both final reader witnesses to be identical.
6. Apply a conservative continuation-drift guard to deployed floats, weights,
   momentum, velocity, and quantized output.

Final output:

```text
NGN_K4_RESUME_LOAD_AUDIT_PASS parent=/home/ehrli/nnue-owned-k4-20260920/runs/harness-smoke-resume-v6/full/candidates/candidate-128/receipt.json
NGN_K4_CHECKPOINT_COMPARE_PASS quantised_different=0 quantised_max_delta=0 quantised_difference_limit=64 different_float_values=5961601 max_abs=1.5087425708770752e-5 tolerance=1e-3
NGN_K4_SMOKE_RESUME_PASS full=/home/ehrli/nnue-owned-k4-20260920/runs/harness-smoke-resume-v6/full/candidates/candidate-256/receipt.json resumed=/home/ehrli/nnue-owned-k4-20260920/runs/harness-smoke-resume-v6/resumed-from-128/candidates/candidate-256/receipt.json
```

Repeated trials were valuable: they showed that demanding byte-identical state
*after another 128 CUDA updates* is false because sparse-gradient atomic order
is nondeterministic and the trajectory amplifies microscopic differences.
Observed maximum post-run float deltas ranged from `4.47e-8` to `1.80e-5`; one
trial moved one of 2,372,360 quantized coefficients by one integer unit. That is
why state restoration is proven by the exact pre-update reload audit. The later
sanity guard is deliberately separate: every float-state value within `1e-3`,
at most 64 quantized differences, each no larger than one, and identical Bullet
trailers. The successful final repeat had zero quantized differences.

The final negative-admission gate rejected `/dev/null` as non-regular and
created no attempt directory. A receipt from an immediately prior executable
was also rejected before output creation. Superseded smoke runs and execution
snapshots were removed after the final v6/v7 artifacts were retained; they are
reproducible from the committed gate scripts.

## Validation

- Pinned Rust 1.88 `cargo fmt -- --check`: pass.
- Pinned offline CUDA release build with `-j2` on CPUs 10–11: pass.
- `bash -n` and ShellCheck for both shell scripts: pass.
- Focused K4 Go packages and commands: pass.
- Final smoke start, exact load audit, resumed continuation, and comparison:
  pass.
- Final malformed-manifest/no-output check: pass.

The repository-wide `go test ./...` was also attempted. All NNUE and K4
packages passed, but the unrelated `engine` package failed
`TestConcurrencyStress/ConcurrentMoveGeneration` and then hit its ten-minute
timeout in `TestNumericalLimits/Depth_100`. This change does not touch the
engine package; the focused in-scope suite is green.

## Next executable step

After physical C: free space reaches at least 60 GiB, generate and label the
candidate pools, finalize the exact accepted pilot plus fixed holdouts, and run
the 1,024-update pilot through parity, validation selection, and playing gates.
Only a promoted pilot should authorize the 19-million-record expansion and
16,384-update main run.
