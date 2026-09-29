# Stockfish 18 SMALL oracle instrumentation

This directory is diagnostic-only source for the exact Stockfish commit
`cb3d4ee9b47d0c5aae855b12379378ea1439675c`. It is not linked into NGN.

For the later oracle gate, acquire a complete clean checkout at that commit,
copy `sf18_small_oracle.h` into `src/nnue/`, apply `instrumentation.patch`, and
build Stockfish with `-DSF18_SMALL_ORACLE` using the frozen build command. Run
the generator with the exact official `nn-37f18f62d772.nnue`. The generator
checks the network's actual size and SHA-256 before launching Stockfish and
creates the transcript, stderr, and JSONL outputs exclusively.

Under the diagnostic macro, `Engine::trace_eval` takes a direct SMALL-only
route before the ordinary in-check guard and BIG-network trace. The helper
calls the upstream SMALL accumulator transform, every upstream layer's
`propagate` function, each architecture's actual `propagate`, and the actual
`NetworkSmall::trace_evaluate`. It aborts unless all wrapped intermediates,
all eight raw transformer PSQT values and transformed buffers, all eight raw
architecture values, and all eight public component pairs agree.

Private tensors are exposed only under the diagnostic macro. Widened shadow
accounting does not replace or modify production arithmetic. The shadow uses
explicitly copied two's-complement bit patterns and canonical scalar operation
order, canonicalizes Stockfish's load-time SIMD lane permutation, and checks
its final wrapped values against actual upstream outputs. The release contract
is exact agreement for the official SMALL network on every recorded lane; it
does not claim portable Stockfish behavior for adversarial tensors that drive
signed C++ arithmetic outside its range.

`oracle_fens.txt` is an exact ordered full-refresh fixture. Its exhaustive
king-square rows cover every HalfKAv2_hm king bucket from both mirror halves
for both perspectives, and an in-check row verifies the direct hook. The
castled, en-passant, and promoted rows are static board placements only. Legal
move-history and incremental-update coverage belongs to the later incremental
slice.

The JSON payload's source/network strings are assertions, not independent
provenance. The release receipt must additionally bind the clean checkout HEAD,
patch/header hashes, compiler and flags, full build command, executable hash,
actual network hash, raw transcript, stderr, and generated JSONL.
