# NNUE next decision after the minimal pipeline

The minimal architecture has fulfilled its plumbing purpose. The shared Chess768-to-128 network now has independently checked training, target construction, raw and quantized export, incremental transitions, scalar/optimized equivalence, real engine selection, and complete audited games. Those games also show that the selected pilot networks are not ready to replace HCE. Further work must produce a stronger evaluator rather than repeatedly prove the same pipeline.

The current 3M TOTAL-position expansion is one bounded data question: does increasing distinct training data, while keeping the original validation bytes fixed, improve generalization under the same 512-update schedule? The 1M schedule extension already answered the complementary question: beyond update 128, additional updates lowered the segment training loss while every later validation checkpoint worsened.

## Decision after the 3M run

If conversion and all leakage gates pass, freeze a separate training request with the exact expanded training BF and the unchanged original validation BF. Keep the graph, seed, optimizer, target construction and quantization gates fixed. Report the learning curve on that same validation set, presentation coverage, quantization error, and independently verified integer parity.

A lower validation loss is diagnostic evidence only. A selected candidate must still play against HCE at the same search policy. If a short operational/playing screen again shows no plausible improvement, do not automatically extend the schedule, rerun the same four openings, or iterate tiny corpus-size increases. Use the result to choose a conventional architecture increase or a verified imported-network control. If validation itself does not improve, further count-only extension of this architecture/data combination is especially poorly motivated.

No synthetic material probe, previous sealed loss, or game result may be used to select a checkpoint retrospectively. Repeatedly inspected sealed data are operational diagnostics, not the final unseen release holdout.

## Conventional architecture and imported-network path

The researched default architecture experiment after pipeline validation is a conventional wider transformer or king-dependent features, with one architectural change at a time and a newly registered format contract. The current NGN-v1 contract stays exact; broadening dimensions silently would make existing files ambiguous.

The user also requested reasonable Stockfish network compatibility. The existing exact compatibility ladder remains the implementation plan: first Stockfish 18 small HalfKAv2_hm inference and independent component parity, then the full release's big/threat evaluator, small-to-big selection/fallback and explicit score adapter. Small-only inference is a compatibility milestone; Stockfish's selective use of that model means it is not a general strength substitute for the complete evaluator.

Stage the first imported backend into reviewable pieces:

1. Pin the release source, official network bytes and architecture identifiers; enumerate feature indexing, orientation and integer arithmetic with an instrumented release oracle.
2. Implement the bounded file decoder and full-refresh scalar inference in an isolated package. Require rejection of truncation, malformed lengths/LEB128, unknown structural hashes and trailing bytes, plus component-by-component oracle parity. No search wiring yet.
3. Add worker-private incremental state, king-move refresh and transition tests against full refresh.
4. Register the exact backend and score policy transactionally, then optimize from profiles. Preserve existing NGN-v1 and HCE behavior.
5. Add the full big/small release behavior before treating imported Stockfish evaluation as a serious general pretrained control.

This follows the source-pinned research in 2026-09-05-nnue-smp/report-source.md; it does not introduce a new architecture or claim that arbitrary .nnue files are compatible. Training and external-network implementations remain Go inference inside NGN's own search. Sol owns implementation; root reviews each stage.

## Scheduling

The immediate machine priority is the first HCE8-vs-HCE8 complete-game control, followed by prospective 8v1 strength testing, after the active data job's actual terminal. Data/training/build work cannot overlap an exclusive clock-controlled game window. Source-only design can proceed while games run. The accepted deployed source remains 53e4d1b.
