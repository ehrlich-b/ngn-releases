# NGN K4 archive sampler

This Rust package is the bounded-memory source sampler for the owned K4 NNUE
run. It decodes the pinned Stockfish T80 binpack, assigns complete encoded
chains to a deterministic 97/1/1/1 split, and identifies positions by the exact
`ngn-k4-input-v1` architecture input consumed by the Go labeler and evaluator.

The pipeline is deliberately staged:

1. `ngnk4scan` performs one complete archive scan. It writes sorted fixed-width
   key runs and chain metadata only at complete-chain boundaries. Every chunk
   has an immutable JSON receipt, so an interrupted scan can validate its
   finished chunks, replay the source to the last boundary, and continue. Each
   attempt stops at the first complete-chain boundary after the frozen four-hour
   cap.
2. `ngnk4select` externally merges key runs, quarantines whole chains for
   cross-split or historical-holdout conflicts, chooses unique K4 inputs by
   deterministic chain priority, and emits disjoint pilot and main-expansion
   candidate shards. The frozen pools contain at least 1.25M pilot candidates,
   25M total train candidates, and 125k candidates in each holdout. That 20%
   rejection headroom is consumed by the post-label finalizer, which freezes
   exactly 1M accepted pilot positions, 20M accepted total train positions, and
   100k accepted positions per holdout; the exact pilot is prepended to and is
   therefore an exact subset of the main corpus.
3. `ngnk4sampleverify` independently replays the source and verifies the final
   selection, split isolation, historical exclusions, identities and receipts.

All three stages are implemented. The verifier independently reimplements the
split, priorities, eligibility predicate, K4 key and position ID; reproduces
conflicts, quarantine, owner choice and chain-prefix selection; checks both
owner orderings as the same cryptographic multiset; and replays the source while
matching every selected record to the emitted label shards.

Selection schema v3 keeps one `u32` unique-owner count per complete chain while
performing the first key merge. After chain-priority selection, it repeats the
immutable key merge and materializes owner records only for selected chains.
The by-key and by-chain owner evidence therefore covers the exact selected
25.375M-position corpus rather than duplicating every unique archive position;
the manifest still records the full unique-owner count. The independent
verifier repeats the full owner merge, reconstructs every per-chain count, and
compares both selected-owner orderings. This trades one sequential merge pass
for bounded disk use without changing selection or emitted-shard semantics.

The production run is additionally gated outside this package on at least
60 GiB free on the Windows host volume and must stop before 25 GiB. The pinned
offline Rust dependencies live in the existing WSL toolchain; no new download
or duplicate source archive is required.

`run_wsl_stage.sh` enforces those physical-volume gates, verifies and confines
the command to distinct low-priority physical cores 12 and 14, and supervises
the process group with a one-minute disk check. Use it for every production
scan, selection and verification command.
