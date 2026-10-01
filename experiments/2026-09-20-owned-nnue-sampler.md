# Owned K4 full-archive sampler source gate

The bounded-memory sampler, selector and independent verifier are implemented
under `training/nnue/k4sample`. This closes the source-side blocker identified
after the SF18 50k accepted-label throughput gate. It does **not** authorize or
claim a production archive scan: the Windows physical volume remains below the
frozen 60 GiB launch threshold.

## Frozen contract

- Source identity remains the 10,809,713,086-byte T80 archive with SHA-256
  `0d22957b8d4f0f8e6f2913be7b744b2dab5178c3563e0f916312d0d94c28b92b`.
- Split seed is `26092001`; complete encoded chains receive deterministic
  97/1/1/1 train/validation/calibration/reserved-test assignments.
- Selection uses deterministic SHA-256 chain priority. It freezes whole-chain
  candidate pools of at least 1.25M pilot positions, 25M total train positions,
  and 125k positions in each holdout. Pilot and main-expansion candidates are
  emitted disjointly. The post-label finalizer takes exact accepted prefixes:
  1M pilot, 19M expansion, and 100k in each holdout, making the exact pilot a
  byte-for-byte prefix and subset of the exact 20M main corpus.
- Input identity is the exact `ngn-k4-input-v1` digest over both sorted
  king-bucketed/mirrored perspective row lists and the material output head.
- All historical eligible validation and sealed-test FENs are independently
  re-keyed under K4. Any new training chain touching one is quarantined.
- Any K4 key observed in more than one new split quarantines every member
  chain. Within a surviving split, the lowest-priority chain owns the unique
  key, with position priority as the deterministic tie-breaker.

## Bounded pipeline

`ngnk4scan` writes one-million-key sorted binary runs and compact 64-byte chain
records. Chunks end only at complete-chain boundaries and carry immutable file
receipts. A restart validates finished chunks and replays the fixed source to
the last boundary. Each attempt stops at the first complete-chain boundary
after 14,400 seconds.

`ngnk4select` performs repeated k-way merges instead of loading archive keys
into RAM. It publishes conflicts, sorted quarantine IDs, owners in both key and
chain order, priority-sorted candidate files, selected chains, and labeler-v2
JSONL shards. Pilot and main-expansion train shards are disjoint; fixed holdout
shards are shared byte-for-byte. Sampler/selection schemas are v2 so the old
pre-label 1M/20M targets cannot be mistaken for accepted-position counts.

`ngnk4sampleverify` does not call the sampler's identity or selection helpers.
It independently reimplements eligibility, split, priority, K4 identity and
position IDs; reconstructs historical keys, conflicts, quarantine, owners and
selected prefixes; checks the two owner orderings with count plus independent
SHA-256 XOR/sum multiset accumulators; then replays every source entry one scan
chunk at a time and matches every selected record to its emitted shard.

`run_wsl_stage.sh` is the mandatory production supervisor. It refuses launch
below 60 GiB physical free space on `/mnt/c`, binds the stage to low-priority
CPUs 10-11, and terminates the process group if the one-minute safety monitor
observes less than 25 GiB or cannot read the disk state.

## Validation performed

Using the pinned Rust 1.88 toolchain and existing offline vendor set on WSL:

```text
cargo test --locked --offline -j2 -- --test-threads=1
5 library tests passed
2 selector tests passed
2 independent-verifier tests passed
cargo fmt -- --check passed
```

The tests cover the two Go cross-implementation K4 goldens, state invariance,
quiet-move legality including pins and promotion rejection, binary padding,
external sort/dedup, retroactive cross-split quarantine, historical quarantine,
same-split owner transfer, whole-chain pilot overshoot, and order-independent
owner-multiset comparison.

The existing Go labeler/packer boundary and the exact accepted-set finalizer
remained green:

```text
go test ./training/nnue/k4label ./training/nnue/k4pack \
  ./training/nnue/k4finalize ./cmd/ngnk4label ./cmd/ngnk4pack \
  ./cmd/ngnk4finalize
```

The disk supervisor negative control refused to launch `/usr/bin/true` with
8.48 GiB physical free, exit 1, proving the production command cannot silently
bypass the current disk hold.

## Remaining before training

Free and verify at least 60 GiB on the Windows volume, then run the complete
scan, selection and independent verifier through the supervisor. Only their
complete immutable receipts may admit candidate labeling and exact accepted-set
finalization. The 5k
versus 20k calibration quality comparison, learning gates, 20M main training,
candidate matches and frozen-scale rating proof remain outstanding.
