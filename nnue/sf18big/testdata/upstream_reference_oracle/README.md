# Stockfish 18 BIG reference oracle

This directory contains diagnostic-only instrumentation for exact Stockfish
commit `cb3d4ee9b47d0c5aae855b12379378ea1439675c` and official BIG network
`nn-c288c895ea92.nnue` (SHA-256
`c288c895ea924429ea9092e3f36b2b3c1f00f2a3a4c759ff7e57e79e3b43e4a7`,
108,919,594 bytes). It is never linked into NGN.

The oracle reuses the already frozen SMALL incremental legal sequence manifest,
SHA-256
`fe843291d9dde5b52bedc6b81debeeb99017d552009d811a854603c661906246`.
It also binds 12 representative 64-ply prefixes in
`representative_sequences.json`, SHA-256
`1f375f586e8da6c7e2528195fdf537af284abba7979165abc9c00c864e5cd23d`.
`freeze_representative_sequences.py` extracts those prefixes deterministically
from the bound legacy game artifact before BIG timing is observed. The oracle
emits the root and every forward action: 423 rows across the core/growth suites
and 780 representative rows.

Apply `instrumentation.patch` to a clean pinned checkout with
`git apply --unidiff-zero instrumentation.patch`, copy
`sf18_big_reference_oracle.h` to `src/nnue/`, and build portable Stockfish with:

```text
make -C src -j2 build ARCH=general-64 \
  EXTRACXXFLAGS='-DNNUE_EMBEDDING_OFF -DSF18_BIG_REFERENCE_ORACLE'
```

For every row the diagnostic serializes upstream active FullThreats indices,
separate base/threat accumulators and PSQT state, transformed input, and the
material-selected raw/output head. The live Stockfish incremental result is
byte-compared with a separate fresh stack and cache before serialization.
Forward transitions also contain independently computed sorted added/removed
sets and Stockfish's `FullThreats::requires_refresh` result for both
perspectives.

Run `generate_reference_oracle.py` once for each suite. The driver verifies the
binary protocol, exact model and sequence identities, row order, provenance,
index bounds, sorted-set invariants, and exclusive output creation. Raw
transcript and stderr files are retained beside each generated JSONL.
