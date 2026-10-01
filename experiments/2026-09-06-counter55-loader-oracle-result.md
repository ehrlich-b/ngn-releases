# Counter 5.5 strict loader and portable full-refresh result

Status: implementation and first compatibility gates complete in isolated worktree; root review pending. No search wiring, score-adapter registration, incremental state, AVX path, performance claim, or engine game has been added.

The implementation is the standalone `countereval` package. `LoadCounter55Legacy` accepts the exact Counter 5.5 24-byte header and 768x512x1 tensor layout, requires exact EOF and finite float32 tensors, computes conservative bounds in float64, and requires every accumulator/product/output bound to remain at most one quarter of MaxFloat32. Model identity stays in the experiment manifest rather than the format parser. `EvaluateFullRefresh` accepts Counter-ordered piece planes, rejects overlap, scans squares A1 through H8, and preserves the pinned portable float32 addition/reduction order. It returns a raw White-perspective float32 only.

The oracle is actual CounterGo source at commit `63c487ca724c620f71c129d62129c6fb9109c872`, built with Go 1.25.5, `GOAMD64=v1`, and no `avx` tag. Its test-only adapter exposes Counter's active indices, all 512 accumulator bit patterns, raw float bits, and historical adapted integer. It is GPL-3.0 test evidence and is not linked into NGN.

Final supervised results on CPUs 12 and 14:

- strict loader, malformed-input, tensor-boundary/order, board-overlap, and synthetic arithmetic tests: PASS;
- 12 shared FENs accepted by both parsers with piece/STM/castling/EP/rule50 identity preserved: PASS;
- ordered feature lists, 6,144 accumulator float32 bit patterns, 12 raw float32 bit patterns, and 12 historical adapted scores: exact PASS;
- five feature/mapping faults, three output arithmetic faults, and nine true adapter faults: each rejected with a named witness;
- early STM sign application: correctly classified equivalent and matched all six scalar adapter cases;
- empty-success stub plus missing, duplicate, unknown, and changed-FEN oracle records: rejected;
- bounded supervisors: exit 0, no timeout, no memory breach, no orphan or surviving process.

Retained expected failures:

1. `ngn-oracle-attempt1` stopped before execution because the evidence command named nonexistent `go.sum`.
2. `ngn-oracle-attempt2` rejected an unwitnessed combined-division mutant; the dedicated scalar case was corrected from `12345.75` to `-14995.75`, where actual sequential division is -8802 and the combined mutant is -8803.
3. `ngn-oracle-attempt4` rejected exact equality between independently summed float64 output bounds that differed by one final-bit step. The safety gate is unchanged; the diagnostic cross-check now uses a predeclared 1e-9 absolute tolerance. Feature, accumulator, and raw output comparisons remain exact-bit gates.

The immutable review packet is `output/counter-pretrained-control-review-v1`. Passing this slice establishes only strict-format and portable full-refresh compatibility with the pinned model and actual Counter 5.5 implementation. The next possible slice is separately reviewed incremental transition parity; current deployment remains unchanged.

Two implementation boundaries are deliberate. The public full-refresh call currently delegates to the trace helper, which allocates an ordered feature slice; it is a correctness reference and is not ready to serve as a search hot path. Also, the build-tagged same-package `counteroracle` test imports `engine` to prove NGN parsing/mapping. Before `engine` imports `countereval`, that integration-test boundary must move to avoid an import cycle. Neither issue affects this isolated compatibility result, and neither is being optimized or rewired in this slice.
