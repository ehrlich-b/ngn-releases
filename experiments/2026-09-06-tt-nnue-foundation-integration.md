# TT and NNUE runtime foundations — September 6

Accepted on the isolated WSL integration branch through source commit 6a282211c42e1d97ccd77357c4fda59e932b8b47. Sol implemented the four slices; root reviewed their production code and targeted tests, verified isolated evidence, integrated separate commits, and ran the combined gates. The deployed playing engine remains 53e4d1b.

| Slice | Commit | Result |
| --- | --- | --- |
| M3 | 730855b | Per-engine TT ownership, direct or striped coherent access, receiver generation invalidation, callback snapshots and migrated callers |
| N3a | d13168e | Transactional compact move/null transitions checked against actual engine state |
| N3b | e444844 | Concrete private worker evaluator, immutable model identity and explicit ordinary/emergency score policies |
| N4 | 6a28221 | Separate portable int32 accumulator with exact wide-reference parity and checked model capabilities |

M3 preserves existing TT entry layout, replacement and the known age-width behavior. Separate receivers can search concurrently without sharing mutable histories, HCE caches, stop control or TT. The synchronized mode protects whole entry tuples; Clear, mode and size changes require idle ownership. This is a prerequisite for sharing a deliberately selected table among future helpers, not multicore search itself.

The NNUE worker owns its accumulator and shares only an immutable model. Compact transitions validate post-move state, including promotions with unchanged occupancy, and null transitions preserve all piece bitboards. Scores retain the specified signed integer operation order. The portable narrow accumulator is separate from the existing int64 reference. Its full legal feature bound is [-2129920,2129855]; output weights outside the optional bounded-dot capability retain the int64 output route. Search wiring and SIMD dispatch are subsequent slices.

The combined full short suite, full short race suite, vet, all-package build, recorder build, record, and corrected-reference comparison all exited zero on WSL CPUs8,10. All seven stderr files are empty. The deterministic search recorder agrees exactly for search results and diagnostics; only requested source and compiler build metadata differ. Reference SHA256: 13151fdc0eb1dace72e9619c749fa12a527da517a48752f03b6dd8a869aa6c89.

Evidence: output/tt-nnue-foundation-integration-20260906/evidence.json, SHA256 b033de396e52b64c281bb14383f2322068554f20d4d93eb0081a10d88e48c937. Root independently verified all33 listed artifact hashes. Isolated M3 final evidence includes50 verified artifacts; N3 and N4 gate manifests include31 and7 verified artifacts.

The final M3 test replaced a Go1.22 integer-range construct with a Go1.21-compatible counted loop. The agent's all-path final patch also included unrelated stale document differences; integration used a source-only36-path patch, SHA256618ab6ea8ad336462c7a7694ce0d38aef372d68a7c60560c4adf3391e9efc3c5. Its only difference from the reviewed source-v2 patch is that test loop. Earlier compile/CGO invocation failures remain recorded.

These are correctness and integration verdicts. No new trained-network, speed, multicore or playing-strength verdict is claimed.
