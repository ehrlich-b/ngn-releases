# Stockfish 18 SMALL incremental oracle

This directory is diagnostic-only source for Stockfish commit
`cb3d4ee9b47d0c5aae855b12379378ea1439675c` and official SMALL network
`nn-37f18f62d772.nnue` (SHA-256
`37f18f62d772f3107e1d6aaca3898c130c3c86f2ab63e6555fbbca20635a899d`,
3,519,630 bytes). It is not linked into NGN.

The incremental instrumentation is layered on the accepted standalone
full-refresh instrumentation. Start with a clean pinned checkout, copy the
accepted `sf18_small_oracle.h` to `src/nnue/`, apply the corrected full-refresh
instrumentation patch, then apply this directory's `instrumentation.patch`.
Build the portable `ARCH=general-64` oracle with `NNUE_EMBEDDING_OFF`,
`SF18_SMALL_ORACLE`, and `SF18_SMALL_INCREMENTAL_ORACLE` defined.

The added UCI diagnostic owns a private Position, StateInfo chain, SMALL
AccumulatorStack, and refresh cache. Real actions use Stockfish's actual
`AccumulatorStack::push`, `Position::do_move`, `Position::undo_move`, and
`AccumulatorStack::pop`. Null actions use the actual no-stack-push
`do_null_move`/`undo_null_move` path. At every root, push, null, and unwind row,
the live incremental stack is serialized through the already accepted
full-refresh all-lane serializer and compared byte-for-byte with a separate
fresh stack/cache result. The diagnostic also records and checks the actual
per-perspective branch selected inside `AccumulatorStack::evaluate_side`:
refresh, incremental update, or computed-frame reuse.

`sequences.json` contains 143 cases and 703 total rows. The `core` suite has
quiet, capture, en-passant, promotion, promotion-capture, both castles, legal king captures of an opponent rook, mixed real/null, and one legal own-king transition from every square
for each color. Those 128 king cases cover every HalfKAv2_hm bucket, both
mirror orientations, and the d/e mirror boundary. The separate `growth` suite
has a 132-ply reversible sequence and complete unwind.

Run `generate_incremental_oracle.py` once per suite. It verifies the actual
network bytes, exact corpus identity/counts, ordered UCI readiness, exact row
sequence/depth/action metadata, update-branch evidence, and equality of every
unwound state with its saved forward state. Output, transcript, and stderr are
created exclusively and never overwritten. Execution provenance must also
bind the clean checkout, both instrumentation layers, compiler/flags, binary,
network, corpus, raw transcript, stderr, and JSONL.

This oracle covers SMALL/PSQ incremental correctness only. It does not wire an
NGN search evaluator, exercise BIG/threat accumulators, claim Stockfish playing
strength, or optimize caches/SIMD.
