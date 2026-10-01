# Owned K4 exact accepted-corpus gate

The sampler originally froze exactly 1M/20M positions before Stockfish
labeling. The measured teacher acceptance rate is 91.478%, so that contract
could not produce the promised 1M/20M **accepted** training sets. This was a
training-blocking correctness gap, not a tuning issue.

## Corrected contract

- The v2 sampler freezes at least 1.25M pilot candidates, 25M total train
  candidates, and 125k candidates in each fixed holdout. These sizes tolerate
  a 20% teacher rejection rate.
- Pilot and main-expansion candidate shards are disjoint. The first consists of
  whole priority-selected chains; the second contains the remaining selected
  train chains. No position is labeled twice.
- `ngnk4finalize pilot` follows the sampler order and takes exactly 1M accepted
  pilot records plus exactly 100k accepted validation, calibration and
  reserved-test records.
- `ngnk4finalize main` copies the hash-verified exact pilot BF first, then takes
  exactly 19M accepted main-expansion records. The 1M pilot is consequently a
  byte-for-byte prefix and exact subset of the 20M main corpus.
- A shortfall is fatal. The finalizer cannot lower a target or skip a missing
  shard.

The finalizer independently verifies the sampler manifest and every consumed
sampler shard; matches every label ID and K4 key to the corresponding sampler
record in order; binds the frozen SF18 executable, source and network hashes;
recomputes each packed BF hash and the packer's ID/key/ordinal/score stream
digest; and records every used prefix in a no-clobber manifest. The sampler
contract and selection/output schemas were bumped to v2, invalidating the old
pre-label-count interpretation.

## Validation

```text
go test ./training/nnue/k4finalize ./cmd/ngnk4finalize \
  ./training/nnue/k4label ./training/nnue/k4pack \
  ./cmd/ngnk4label ./cmd/ngnk4pack
all passed

cargo test --locked --offline -j2 -- --test-threads=1
5 library tests passed
2 selector tests passed
2 independent-verifier tests passed
cargo fmt -- --check passed
```

The finalizer test constructs ordered sampler, label and pack artifacts, freezes
small exact pilot/main sets, and proves that the pilot bytes are the exact main
prefix. Rust validation used the pinned 1.88 offline toolchain on WSL in
`sampler-src-v4`. No production archive scan, labeling run, or training run was
started: physical C: had only 9 GiB free, below the mandatory 60 GiB launch
gate. The Lean hopper remained untouched.
