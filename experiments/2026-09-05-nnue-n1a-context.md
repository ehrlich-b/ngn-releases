# NNUE N1a incremental context — 2026-09-05

Sol implemented the standalone context in ngn-nnue-v1. Root reviewed its transition/undo code and numerical tests, integrated the exact six source hashes into the M0 search-control tree, and reran the required checks on WSL.

Each context binds one immutable loaded model and preallocates 256 real/null move frames. Frames hold both int64 accumulators, side-to-move, compact facts and an inverse transition; only one full piece board is retained. Reset, real-move push, null push and pop validate their preconditions. A rejected operation preserves the live context. The semantic delta supports captures, en passant, castling and promotions and reconstructs the before board from the supplied after position.

Review replaced score-only parity as the main oracle: clipping can hide an incorrect accumulator. Tests now compare all 2x128 accumulator entries with an independently calculated Chess768 refresh using dense signed weights across all input/hidden coordinates. Legal histories compare after every move and undo; fixtures cover both colors, both castling sides, all promotion types, null moves, repetition cycles and stack bounds. A test-only value-copy snapshot exposes no mutable production state. Hot reset/push/evaluate/pop cycles allocate zero times, and independent contexts sharing one model pass the race check.

Integrated checks pass: verbose context oracle, full short tests, full short race tests, vet and main build. Exact commands/logs/exits, Sol evidence copies, source hashes, binary hash and root receipt are retained in output/nnue-n1a-integration-20260905 with DONE_EXIT_0. Sol also reported an earlier unrestricted engine-suite attempt that failed concurrency/strength gates; only that session output survived. It is not a successful check or a retained artifact, and this milestone makes no unrestricted-suite claim.

This context is not yet connected to production search. There is no trained-network or NNUE playing-strength verdict. Next is the pinned Bullet export/parity bridge and worker-owned evaluator integration after the remaining ownership interfaces are settled.
