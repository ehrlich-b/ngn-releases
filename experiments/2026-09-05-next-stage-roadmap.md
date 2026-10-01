# NGN next milestone: NNUE and real multicore search

User direction, September 5: pursue the strongest Go-based chess engine in the world; NNUE is now allowed in principle. All development and computation must happen on the authorized WSL box. The Mac is only a remote terminal. Native Windows runs are historical evidence and are not the execution target for this sprint.

Detailed researched design: [NNUE and multicore implementation plan](2026-09-05-nnue-smp/report-source.md). Its concrete architecture, compatibility tiers, worker ownership, synchronization and staged gates supersede the initial defaults below. Major implementation is assigned to Sol; root coordinates and reviews.

## Starting point

Accepted playing source: 53e4d1b9bf4006a39c0327a7016faba7f2b8f82e. The completed prior native-Windows 400-game calibration is 2884.1429896655936, primary 95% interval [2834.197941381757,2934.08803794943]. Full WSL legal replay covered 400 games/55,349 played plies; all 400 terminal outcomes passed. That completed 2800 goal remains separate from future WSL measurements.

Authoritative raw evidence is in /home/ehrli/repos/ngn/output/corrected-pin-20260905/results-r0905matepin and completion-audit. The old source runbook predates the final result. Do not restart its historical matches or waiters.

WSL hardware inspected: Ryzen 7 9800X3D, 8 cores/16 logical CPUs; AVX2 and AVX-512 flags visible; about 15 GiB WSL RAM and 760 GiB free filesystem space; RTX 5080 with 16,303 MiB reported VRAM. GPU visibility is established, not a working training installation: torch, nvcc and cargo were absent from the inspected WSL environment.

## Recommended next goal

Deliver an accepted Go NNUE evaluator and actual multicore search, each with a separately measured contribution, then deploy the combined accepted release on WSL. Leadership among Go engines remains the longer-term objective; completing these features alone is not a world-best claim.

The runtime remains a Go engine. Portable scalar evaluation is the correctness reference. Architecture-specific Go SIMD or small Go assembly kernels are the recommended acceleration path; the user was asked about strict portability and had not answered when this plan was written. Training/export tools may be Python or Rust on WSL. Do not put another engine's search behind a Go UCI wrapper.

## Priority and dependencies

1. Establish the native-WSL baseline and use a mature match runner. Pin and validate a named fastchess release after source review (latest inspected: v1.8.2-alpha), validate terminal/repetition/clock handling with the known boundary fixtures, and run a fixed A/A control. Preserve complete game histories and binary/net/book identities. This is a new test environment; do not pool its games with prior native-Windows evidence. Keep the existing independent legal and terminal audits.
2. Make mutable search state belong to an engine instance and worker, in small behavior-preserving steps. Inventory global TT, stop/node limits, evaluation caches, histories, per-ply buffers and repetition state. Keep one-thread behavior identical while introducing ownership needed for both NNUE accumulators and later SMP. Do not attempt a broad search rewrite.
3. Deliver one compact NNUE end to end: fixed feature mapping, exporter and checked network format; scalar reference; incrementally updated accumulators; optimized CPU path; search score integration; one trained candidate; matched real-clock comparison against HCE. Begin with a standard compact piece-square network, with exact width/architecture pinned before training, rather than modern Stockfish's complete multi-feature architecture.
4. Deliver Lazy SMP using private positions, histories and accumulators, a deliberately concurrent TT design, shared stop/time control, and deterministic root-result publication rules. Preserve Threads=1 as a first-class configuration. Validate stability and strength at 1/2/4/8 threads. Threads=8 must demonstrate useful playing improvement over Threads=1 at equal wall time; eight threads need not provide eightfold NPS.
5. Profile the resulting engine and optimize the measured hot paths. NNUE SIMD is part of delivering viable inference, so it cannot wait until all features are finished. Broader work follows the new profile: allocations/escape, cache layout, move picking, TT probes, stack/bounds checks, profile-guided optimization and target-specific instruction use.
6. Add real Syzygy WDL/DTZ, with rule-50 and root move ordering coverage. The existing implementation is deliberately disabled placeholder indexing, not working tablebases. Reuse a verified implementation consistent with the runtime-language choice; do not handwave complex indexing. This is a follow-on capability, not a prerequisite to the first NNUE gate.

The small quiet-promotion candidate remains useful and must rebase onto the accepted mate guard before its own verdict. A bounded repair of the reproduced TT age-width mismatch is also available. Search-policy work should target child-depth/extension coherence, null-move and history boundaries, and demonstrated pruning misses. Those are separate tests; do not stack every familiar heuristic. The old 50M internal node ceiling is absent from the current time-controlled path: parseSearchParams starts Nodes at zero, UCI passes that value to SearchControl, and search checks a node budget only when nonzero. Explicit go nodes remains a one-worker path; the prospective multicore match uses ordinary clock commands.

## NNUE proof requirements

The previous Texel defects make executable equivalence essential:

- Specify piece/square orientation, king perspectives, side-to-move convention, activation, rounding, clipping, accumulation ranges, score scale and output normalization.
- Compare export/reference/Go scalar inference. Integer scalar and optimized paths must agree exactly; floating export comparisons need a documented quantization tolerance.
- Compare incremental accumulators with full refresh after every move and undo in diverse legal histories, covering capture, en passant, castling, promotion/underpromotion, king moves, null moves and stop/unwind.
- Keep terminal/mate/repetition logic outside learned evaluation. Clear or separate caches when changing model identity. Audit HCE-only lazy cutoffs, phase scaling, rule-50 damping and correction history before reusing them with a new score distribution.
- Use a small licensed dataset pilot and a proven trainer first. Retain data provenance and group-aware holdouts where game/source identity exists; do not claim game-disjointness for flat data lacking those IDs. Scaling requires measured GPU throughput, export parity and a viable playing candidate.
- Prefer comparing HCE and NNUE under identical search first. If search-margin calibration is needed, declare and test that interaction separately.
- No MSE/NPS-only acceptance. Use predeclared paired real-clock games; require a demonstrated positive effect under the chosen test, then confirm the combined release at a longer time control.

A compatible existing small network can isolate inference/search integration from training quality. Its format, architecture, score convention and permitted use must be pinned; arbitrary .nnue files are not interchangeable. The default route is an independently trained compact network using an established trainer, with a borrowed-net diagnostic optional.

## Measuring Go leadership

[The frozen release inventory](2026-09-05-go-opponent-inventory.md) identifies Counter5.5, Zahak10.0, Chess-3 4.0 and Blunder8.5.5. Initial research's older Zahak9.x rating reference below is historical orientation, not the selected release. Official Linux artifacts/source pins are prepared; runtime validation and matched games remain pending.

Counter and Zahak are essential opponents, alongside chess-3 and other Go engines identified in an updated public inventory. Published readmes currently report Counter 5.5 around 3333 in 40/15 and Zahak 9.x around 3278 single-thread /3406 eight-thread blitz; chess-3 reports 4.0 around3033. These are different lists/settings and are only orientation. Check releases/tags, not just default branches: Zahak's default source dates to2022, while its repository was archived in2026.

Build a stronger, diverse local opponent pool around NGN's new strength rather than continuing to rely on saturated weak anchors. Freeze opponents and held-out openings before each milestone. Report matched single-thread and equal-hardware multicore head-to-head results at short and longer clocks. A world-best claim requires completed superiority against the strongest identified public Go releases under those conditions and preferably independent external testing, not arithmetic added to our historical2884 calibration.

## Open-source study and Go performance

Pinned repository metadata and candidate source paths are saved in:
 /home/ehrli/repos/ngn/output/next-stage-20260905/reference-manifest.json

CounterGo and Zahak give concrete Go implementations of neural evaluation and SIMD; Counter's inspected current evaluator is a float32 network with512 hidden units, and Zahak documents769->128->1. They are references, not evidence that either architecture is NGN's best choice. Chess-3 supplies classical evaluation and Go search comparisons. Stockfish, Ethereal, Berserk and Viridithas are selected search/accumulator/threading references. Bullet supplies established training/export tooling. Fastchess/OpenBench supply match and experiment machinery.

For each borrowed mechanism, record donor revision/path, prerequisite invariants, our implementation, targeted failure test, isolated game verdict, and applicable source/network/data provenance. Breadth belongs in the source inventory; accepted playing changes need focused evidence.

Go1.27 adds experimental portable simd and revises simd/archsimd under GOEXPERIMENT=simd; the earlier1.26 API is not a fixed target. Investigate these alongside small assembly kernels rather than assuming Go must cross cgo for vectorization. Pin any experimental toolchain and retain a portable scalar fallback. WSL's baseline Go toolchain is1.25.5; a newer compiler needs separate installation/build comparison, not an unrecorded replacement. Go PGO is another measured candidate, not an assumed speedup.

Historical speed results already contain failed int32, staged-scoring, pin-aware-legality and recent block-picker experiments. Those findings constrain repetition but do not close speed work after NNUE/SMP changes the workload. Benchmark actual search and equal-time Elo, not cross-engine NPS as though each node did equal work.

Primary sources:
- https://github.com/ChizhovVadim/CounterGo
- https://github.com/amanjpro/zahak
- https://github.com/paulsonkoly/chess-3
- https://official-stockfish.github.io/docs/nnue-pytorch-wiki/docs/nnue.html
- https://github.com/jw1912/bullet
- https://github.com/Disservin/fastchess
- https://github.com/AndyGrant/OpenBench
- https://go.dev/doc/go1.27
- https://go.dev/doc/pgo

## Sprint completion

Completed correctness/parity and concurrency checks; accepted NNUE gain; measured useful SMP scaling; longer-clock combined-release confirmation; no unexplained crashes/illegal moves/time failures in acceptance runs; reproducible source/net/toolchain/data/artifact identities; updated stronger-opponent progress report; deployed WSL release with rollback. Every behavior-changing candidate gets a declared verdict before promotion. World leadership, real Syzygy and further search/speed work remain subsequent milestones unless explicitly incorporated into a later goal.

## Implementation progress

The maintained sprint state is [current status](2026-09-06-current-status.md). It supersedes this roadmap's historical installation and progress notes.

The development engine includes real Lazy SMP (168385f), strict NGN-v1 scalar/incremental/export equivalence, transactional evaluator selection, portable int32 and bounded AVX2 inference, private worker state, joined cancellation and OwnBook control. Standalone Counter 5.5 compatibility (ed9ee07) and the SF18 SMALL loader (e88d36c) are now integrated. Deployment remains 53e4d1b.

Corrected WSL B0 A/A passed 200 games. Multicore passed its correctness/concurrency checks, scaling matrix, operational control, and prospective same-code 8v1 SPRT: 63 wins, 3 losses and 36 draws over 102 games, with the first upper-bound crossing at pair 51. HCE2v1 and HCE4v1 remain outstanding; longer-clock combined validation is separate.

The minimal trained NNUE pipeline passed numerical checks, but the 100k, 1M and 3M selected networks each lost separate four-game HCE screens 0–4, triggering the stop rule for count-only extensions. The imported Counter 5.5 evaluator is now integrated and accepted against same-code HCE at one thread: 89 wins, 37 losses and 40 draws over 166 games, first upper-bound crossing at pair 83. SF18 SMALL compatibility remains a separate staged path.

The next combined gate is Counter8 versus HCE8 at 60+0.6 on a separately pinned 16-ply opening set, followed by matched external comparisons. Counter 5.5 is the first calibration opponent. Rodent V1.1 Anand/testers is the publicly rated September 5 CCRL anchor; Rodent V1.2 non-Tal testers is the separately pinned latest-stable target. HCE2v1/HCE4v1, combined confirmation, external games and WSL deployment remain required. Deployment remains 53e4d1b.
