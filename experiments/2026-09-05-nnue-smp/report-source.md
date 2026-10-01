# NGN: pluggable NNUE and multicore implementation plan

Date: 2026-09-05. Accepted playing source: 53e4d1b9bf4006a39c0327a7016faba7f2b8f82e. Design branch: sprint/nnue-smp-20260905.

This is the canonical research report and implementation plan. Root coordinates, reconciles evidence, reviews interfaces and controls acceptance; Sol performs the major implementation work in bounded stages. All project work happens on the authorized WSL box. Research inspected source and model files; no new engine behavior, training installation, build or match was performed during this design phase.

## Decision

Build a small independently verifiable NNUE pipeline and safe worker ownership first. Keep the evaluator extensible enough for named external network architectures. Add multicore incrementally: preserve one-thread behavior, establish coherent shared-table access, then add independent helper searches. Prove each boundary before combining features.

The first network is a pipeline control. It need not beat the accepted handcrafted evaluator. A later trained candidate must win a predeclared real-clock test before replacing HCE. Likewise, launching eight workers proves neither useful search diversity nor playing strength.

The resulting milestone aims at a stronger Go engine with verified NNUE and SMP. Neither a new Elo estimate nor world leadership is established by this research.

## What current engines actually do

| Reference pinned on September 5 | Relevant evidence | Consequence for NGN |
|---|---|---|
| Stockfish 18, cb3d4ee9 | Big/small transformers of 1024/128; HalfKAv2_hm; big adds FullThreats; downstream 15/32 and eight stacks | A supported Stockfish release is a specific feature, arithmetic and container contract |
| Stockfish development, edb0d9db | SFNNv16-era single 1024 transformer, FullThreats + PP_3Wide + HalfKAv2_hm, pairwise transform, 32/32 layers | Development compatibility is a separate backend; a commit titled Stockfish 19 is not proof of a public release |
| PlentyChess, 04e07a98 | King buckets, threats and pawn pairs; 1024/16/32, mixed integer/float tensors | Similar concepts do not imply interchangeable network files |
| Reckless, 91b56c29 | Separate piece/threat state, king buckets, 768/16/32, custom raw layout | The context API needs independent refresh validity for feature families |
| Viridithas, cf975c06 | Threat/pawn-pair features, pairwise reduction, compressed custom tensors | Preserve a route to richer features without adding them to the pilot |
| CounterGo, de02aef5 / Zahak, 72ff8066 | Custom float networks and Go worker implementations | Existing Go engines are useful references, but their formats and concurrency choices require individual review |

Primary code: [Stockfish 18 architecture](https://github.com/official-stockfish/Stockfish/blob/cb3d4ee9b47d0c5aae855b12379378ea1439675c/src/nnue/nnue_architecture.h), [current Stockfish architecture](https://github.com/official-stockfish/Stockfish/blob/edb0d9db6731067ec50ce619ff372b463bc4dd5d/src/nnue/nnue_architecture.h), [PlentyChess](https://github.com/Yoshie2000/PlentyChess/blob/04e07a98ee6ac104c30e7374450c94b96d94ef4d/src/nnue.h), [Reckless](https://github.com/codedeliveryservice/Reckless/blob/91b56c29861f0a5713204bdeffd6c45e9eb9f649/src/nnue.rs), [Viridithas](https://github.com/cosmobobak/viridithas/blob/cf975c061da3a494d793d6ec2973a514b6191831/src/nnue/network.rs), [Counter weights](https://github.com/ChizhovVadim/CounterGo/blob/de02aef5a6887b82383ae9a7d6ce0614b730155f/pkg/evalnn/weight.go), [Zahak evaluator](https://github.com/amanjpro/zahak/blob/72ff8066e2d8348d8d3847fab62c4a6b8344240d/engine/nnue.go).

The shared direction is incremental feature transformers, stronger nonlinear transforms and richer positional features. Our inference is that NGN should accommodate these families, while validating a much smaller implementation first. There is no evidence that merely increasing the network width or importing weights will fix search or pipeline defects.

## NNUE plan

### N0: fixed small architecture and independent numerical reference

Use Bullet revision 629ee50000b2afb7b3337595401c830d3b1e0f42. The model has one shared Chess768 → 128 transformer evaluated from both perspectives; concatenate side-to-move then opponent; apply squared clipped activation and a linear scalar output. No king buckets, threat inputs, output buckets or extra hidden layers in this version.

The source example uses QA=255, QB=64, score scale=400. Its 98,689 signed-16-bit parameters occupy 197,378 tensor bytes. The feature identity is piece/color/square relative to each perspective, including kings; black perspective flips ranks, not files. Confirm the piece-code order and board normalization independently against the pinned Rust feature mapper before freezing golden fixtures.

[Bullet example](https://raw.githubusercontent.com/jw1912/bullet/629ee50000b2afb7b3337595401c830d3b1e0f42/examples/simple.rs), [feature mapper](https://raw.githubusercontent.com/jw1912/bullet/629ee50000b2afb7b3337595401c830d3b1e0f42/crates/bullet_lib/src/game/inputs/chess768.rs).

Define integer inference in this exact order:

~~~text
a[perspective][j] = bias[j] + sum(active feature weights[:,j])
activation(x) = clamp(x, 0, 255)^2
dot = sum(activation(stm[j])*w[j] + activation(other[j])*w[128+j])
value = truncate_toward_zero(dot / 255)
value += output_bias
raw_cp = truncate_toward_zero((value * 400) / (255 * 64))
~~~

The reference uses signed 64-bit accumulator, activation, dot and scaling arithmetic. Do not collapse the two divisions. Narrow optimized representations require bounds computed from the loaded tensors and accepted position domain, proving equivalent results. The scalar proof can use at most 64 occupied board squares; any tighter legal-position bound must be enforced explicitly. Arbitrary admitted i16 weights can produce a dot around 5.45e11: the example's i32 path is not a sufficient general loader contract. This was caught during root review.

Match the pinned writer exactly: training/raw tensors are float32; promote each stored value to float64, multiply by the quantizer in float64, then round halfway away from zero and range-check before casting. Reject non-finite values and serialize little-endian column-major tensors in order FT weights, FT bias, output weights, output bias. Quantizers are 255, 255, 64, 16,320. Bullet's raw quantized output has 64-byte alignment padding; NGN's canonical payload omits padding. Its own versioned envelope records architecture, feature and quantization IDs, dimensions, payload length and checksum, with exact EOF. Freeze a 128-byte v1 header: eight-byte magic NGNNUE followed by two zero bytes; thirteen LE u32 fields (format version, header length, architecture ID, feature ID, quantization ID, score-contract ID, input width, hidden width, perspective count, output width, QA, QB, scale); LE u64 payload length; 32-byte SHA256 of the canonical payload; 28 zero reserved bytes. IDs are registered constants, dimensions/constants must match this architecture, and there is no variable description. Require exactly 197,378 payload bytes. Model identity includes the full-file SHA256 and contract IDs, so header changes cannot alias a model. A dedicated converter accepts the exact tensor prefix and expected aligned length of the pinned Bullet transport. Require a particular padding-byte pattern only after inspecting the pinned writer; do not infer zero padding from alignment documentation.

[Saved-network semantics](https://github.com/jw1912/bullet/blob/629ee50000b2afb7b3337595401c830d3b1e0f42/docs/4-saved-networks.md), [pinned Bullet quantizer implementation](https://github.com/jw1912/bullet/blob/629ee50000b2afb7b3337595401c830d3b1e0f42/crates/trainer/src/model/weights.rs#L287).

Gate: an independent tensor reader and integer reference agree exactly with Go scalar inference, including asymmetry, both turns, negative divisions, SCReLU clipping, large weights, truncation, invalid sizes, corrupt checksums and trailing data. A successful training process is insufficient: Bullet can finish despite failure to emit a quantized checkpoint.

### N1: one backend boundary, owned by each worker

An immutable Model owns tensors and identity: digest, schema, feature version, quantization contract and raw-score contract. An engine configuration generation selects it. Each Worker owns a preallocated EvalContext with accumulator stack, validity flags and refresh cache. Loaders and optimized tensor packing run outside search.

Conceptual operations are Reset(position), Push(delta, afterPosition) after successful board mutation, Pop(), and Evaluate(position). Delta carries fixed-capacity typed piece/square removals/additions, special-move kind, old/new king squares, old/new occupancy and per-color pawn bitboards. Prove that afterPosition plus delta supports query-equivalent reconstruction of the prior board; otherwise pass read-only before/after views. This avoids retaining an entire board copy per ply while permitting later threat and pawn-pair backends. Rich feature derivation is deferred; a backend may always invalidate and fully refresh. Loader/factory polymorphism stays cold. Each Worker binds one backend/context; use direct concrete calls or one predictable evaluator-kind dispatch in v0. No per-piece callbacks, allocations or registry lookup belong in recursion; further specialization follows profiling.

A null move changes evaluation ordering, not board features. Use explicit no-op/alias-frame semantics and symmetric undo. Real moves push; undo pops. Capture, en passant, promotion and castling must identify all changed pieces, including the EP capture square and castling rook.

Every search evaluation call uses one score adapter: root seeding, main static evaluation, qsearch stand-pat and emergency/raw fallbacks. At 53e4d1b, engine/search.go:1322,1327,1337 bypass the cached route; these cannot remain HCE-only shortcuts. Current engine/eval.go:2806 adds rule-50 damping, perspective and tempo outside the raw cache. NNUE raw output is already side-to-move relative. Preserve the HCE contract exactly. For v0, cache raw STM centipawns; after every raw-cache lookup apply NGN's existing halfmove attenuation exactly once, then clamp below the mate band. Do not add HCE Tempo or phase blending. Keep correction/pruning policy unchanged for the first comparison, with reset on model generation. The halfmove adapter stays outside the raw cache because a board hash can recur at a different clock. Terminal, mate, repetition and rule draws remain search/rules logic.

Successful model replacement is an idle transaction: stop and join, validate/build replacement, install fresh contexts, invalidate TT/caches and all persistent worker histories, corrections, killers and counters, then increment generation. Recursive search/eval parameters are immutable per-session snapshots; package-level tuner mutation cannot coexist with active engines. A failed load leaves the prior model intact and reports failure. Board-only cache keys are insufficient across model or score-contract changes.

Gate: full refresh equals incremental state after every move and undo in independent legal histories; cover both-color EP, both castlings, all promotions, king moves, null moves, transpositions, repetition, maximum stack depth and interrupted unwinding. A→B→A model changes on the same position must not reuse stale scores. HCE one-thread traces remain unchanged when HCE is selected.

### N2: prove the trainer and data pipeline on WSL

Use isolated pinned Bullet tooling first; official nnue-pytorch is reserved for exact Stockfish training later. The RTX 5080 is compute capability 12.0 and needs a Blackwell-capable toolchain. WSL currently sees the GPU and driver 591.86, but has no cargo/rustup/nvcc; Docker resolves to Podman and the NVIDIA container path is unverified. Install Rust ≥1.87 and CUDA ≥12.8 in versioned user-local WSL toolchain directories or an isolated WSL image; record PATH and CUDA_PATH and preserve existing Go/compiler and engine artifacts. First execute an actual sm_120 batch and record GPU use, peak VRAM, host RSS and throughput.

[NVIDIA GPU capabilities](https://developer.nvidia.com/cuda/gpus), [Blackwell compatibility](https://docs.nvidia.com/cuda/blackwell-compatibility-guide/), [CUDA 12.8 driver requirements](https://docs.nvidia.com/cuda/archive/12.8.0/cuda-toolkit-release-notes/), [Bullet setup](https://github.com/jw1912/bullet/blob/629ee50000b2afb7b3337595401c830d3b1e0f42/docs/2-getting-started.md).

Begin with deterministic legal fixtures and 50K–1M positions, about 2M training presentations. This proves plumbing and export. Reproducibility means frozen inputs, seeds, configuration and tool revisions with bounded repeated metrics; require bit-identical GPU checkpoints only after deterministic execution is established. Then use a bounded game-aligned corpus and roughly 50M presentations to assess holdout behavior. Scale toward hundreds of millions only after measured throughput, memory and equivalence justify it. Start with a 512 MiB loader buffer, queue depth 8 and 2–4 loader threads; target host RSS below 12 GiB and VRAM below 14 GiB. These are limits, not predicted performance.

The first serious training route is supervised public binpack scores plus game results, using the official Stockfish master-binpacks collection as the preferred source to evaluate. Pin the exact file, checksum, license and provenance before download/conversion. T80 data is a candidate, not a claim that one file will produce a strong NGN net. Split whole game chains before flattening, retaining deterministic group IDs and deduplicating across splits. If a format cannot preserve game identity, do not claim game-disjointness. Flat Lichess FEN/eval records can serve other purposes but do not supply this split.

Teacher relabeling is a separate later experiment with a pinned teacher binary, node budget and score perspective. NGN selfplay is another separately manifested experiment with whole-game grouping. Do not mix these sources silently or present HCE-generated labels as independent strength evidence.

[Official binpacks collection](https://huggingface.co/datasets/official-stockfish/master-binpacks), [Lichess database](https://database.lichess.org/).

Use named result_weight and score_weight fields. Bullet's example target is 0.75 × game result + 0.25 × sigmoid(STM score/400). Do not copy similarly named trainer parameters without verifying their meaning. Require distinct training, validation and sealed test groups; choose checkpoints on validation only, considering the initial checkpoint too. Bullet exposes one optional test_set slot: use it for validation, keeping sealed test data entirely outside trainer.run. Evaluate the sealed test once with a separate frozen-checkpoint command. Never silently use training data for validation.

Gate: pinned Bullet in-memory float forward on frozen positions → independent raw.bin float evaluator within declared tolerance → quantized integer reference → exact Go parity; documented float-to-quantized error limits chosen on validation before opening test. Feature/weight mutations must affect the intended path. Test-label mutations must not affect training or checkpoint choice. Filters are pure functions of recorded training inputs, independent of production TT, clocks, stop flags and correction history. Explicit fixtures cover checks, captures, quiet promotions and predecessor/fullmove metadata. These address the previous Texel trace, filtering and checkpoint failures directly.

### N3: optimize and earn playing acceptance

After scalar/incremental proof, profile actual Go search. Add Go SIMD or small assembly kernels with exact scalar parity and proven arithmetic bounds; keep a portable fallback. Go 1.27 has experimental portable simd and revised simd/archsimd, so investigate it as a separately pinned toolchain option. Installed Go 1.25.5 is not silently replaced. No cgo inference or foreign search engine is required by this design.

[Go 1.27 SIMD status](https://go.dev/doc/go1.27), [Go PGO](https://go.dev/doc/pgo).

First compare NNUE and HCE with the same search policy. HCE-specific lazy evaluation, correction histories and pruning margins need an explicit audit; do not tune them silently into the first comparison. Subsequent score calibration or search adaptations receive separate verdicts. If the compact network cannot improve play after pipeline correctness is proved, the next architecture experiment is a conventional wider/king-bucket model, one change at a time, or the external-network control below.

### External-network compatibility ladder

1. NGN v1: the exact small architecture above.
2. Stockfish 18 small backend: exact HalfKAv2_hm, 128/15/32, eight PSQT/output stacks, skip/activation behavior, decoded small-FT rescaling, material-bucket selection, structural hashes, LEB128 and load-time tensor permutations. Compare raw components against an instrumented pinned Stockfish oracle. This proves imported inference/incremental arithmetic; Stockfish uses the small net selectively in material imbalances and may fall back to big, so small-alone play is not a general pretrained strength control.
3. Full Stockfish 18 backend: implement the 1024 big network with threats, small-net selection/fallback and the explicitly selected Stockfish score adapter. Compare raw PSQT/positional components, selected-network blend and adapted outputs separately. Full adaptation accepts explicit optimism input or declares optimism=0 parity; that search-provided value cannot be hidden in a model-plus-board cache.
4. Current development and other engines: separately registered versions after their independent golden suites. No moving-master compatibility promise.

Stockfish 18 selects small or big with small-to-big fallback, combines the selected network's PSQT and positional components, and applies additional material/complexity/optimism/rule-50 adjustments outside the network. A bit-exact raw loader alone does not reproduce its full evaluator, and importing that evaluator does not import Stockfish search strength. [Stockfish 18 evaluation](https://github.com/official-stockfish/Stockfish/blob/cb3d4ee9b47d0c5aae855b12379378ea1439675c/src/evaluate.cpp).

Dispatch Stockfish files by supported container version and architecture/component hashes; validate bounded lengths, signed LEB128 consumption and EOF. Structural hashes are 32-bit compatibility identifiers, not payload-integrity hashes; retain independent full SHA256 provenance. A filename extension is not an architecture. Unknown files fail clearly. Current defensive parser checks can inform a release-compatible loader without importing development architecture. [Release loader](https://github.com/official-stockfish/Stockfish/blob/cb3d4ee9b47d0c5aae855b12379378ea1439675c/src/nnue/network.cpp), [development parser](https://github.com/official-stockfish/Stockfish/blob/edb0d9db6731067ec50ce619ff372b463bc4dd5d/src/nnue/nnue_common.h).

The official network archive states its included networks are CC0; engine source has its own GPL terms. Record the exact model's archive provenance and preserve attribution/source obligations for any copied code. No public net was verified to match NGN v1's exact layout, so the pilot trains its own. [Official network provenance](https://github.com/official-stockfish/networks/blob/dd5c7f74c073a7d0bcbb52648d26c537a3d04cf4/README.md).

## Multicore plan

### M0: ownership before goroutines

The current code has a useful worker-local SearchInfo (engine/search.go:514–624) and Position.Copy (engine/position.go:476). It also has global mutable TT entries and age, histories/correction tables/killers, exact-eval and pawn caches, stop/node limits, mutable parameters and book/TB handles. Cloning the board alone cannot make it safe.

Use three lifetimes:

| Owner | State |
|---|---|
| SearchEngine / UCI instance | Total TT allocation, immutable config/model generation, resource handles, persistent Worker objects |
| SearchSession, one go command | Copied root position/full history, immutable limits and root restrictions, cancellation/deadlines, result/completion, aggregate accounting |
| Worker | Position, SearchInfo/stacks, every history/correction/killer/counter, last-move state, private eval/pawn caches and EvalContext |

Immutable attack tables, magic tables and Zobrist values may remain shared after initialization. Legacy package wrappers may target a default engine for compatibility; real implementation uses explicit receiver state. No global or goroutine-local current-worker selector.

Implement in small one-thread slices: control/engine identity; worker histories/path state; caches/config/resources; evaluator context. Preserve operation order, default sizes and cold/warm lifetime behavior. Snapshot old traces before edits.

Gate: exact move, score, nodes, PV and deterministic callback content at fixed depth in cold and warm fixtures. Exclude inherently variable elapsed/NPS fields. Poison engine A's state and prove engine B matches clean C; concurrently stop A while B completes. Run required short suites and full short race suite. Any unexplained drift stops the refactor.

### M1: a coherent transposition table

Keep Hash as one total MB allocation per engine. Admit at most one active session per engine. Start with striped mutexes protecting the complete record for concurrent mode; choose direct versus synchronized mode while idle. In direct mode the session exclusively owns TT access through final PV/hashfull extraction, and external diagnostic readers cannot overlap. Never infer exclusivity from an unsynchronized reader count. All accesses, including hashfull/PV/diagnostics, must obey the same ownership/protocol. Advance age only while idle and join before clearing/resizing.

Two independently atomic key/data words do not form a coherent record. XOR validation detects many mixed pairs but is not a transactional proof. A seqlock with plain concurrently accessed payload fields still races in Go. CounterGo provides a useful alternate CAS-gated entry design that skips contention; Stockfish's deliberately inconsistent TT semantics and Zahak's plain shared fields are not a Go correctness template.

[Go memory model](https://go.dev/ref/mem), [Counter TT](https://github.com/ChizhovVadim/CounterGo/blob/de02aef5a6887b82383ae9a7d6ce0614b730155f/pkg/engine/transtable.go), [Zahak TT](https://github.com/amanjpro/zahak/blob/72ff8066e2d8348d8d3847fab62c4a6b8344240d/engine/cache.go), [Stockfish TT contract](https://github.com/official-stockfish/Stockfish/blob/edb0d9db6731067ec50ce619ff372b463bc4dd5d/src/tt.h).

Gate: overlapping searches on the same engine are rejected/serialized; independent direct-mode engines run concurrently under -race; clear/resize/diagnostic access cannot overlap a direct session. Adversarial multiwriter/readers assert every accepted hit is a complete published tuple, plus race detection, collisions/replacement/mate-score/age-wrap tests and model-generation invalidation. The existing age-width mismatch is a separate proved correction, not hidden in this refactor. Profile contention before replacing locks with a more complex protocol.

### M2: minimal Lazy SMP

Keep Threads=1 on the direct path. For 2/4/8 workers, copy root state into private workers and run independent iterative deepening with shared coherent TT and cancellation. Use per-go goroutines over persistent Worker objects initially.

Worker 0 owns soft time decisions, progress data and the final last-completed iteration. One coordinator/output writer serializes UCI messages; workers never write protocol output directly. Helpers initially contribute TT information and node statistics. When worker 0 finishes or shared stop triggers, cancel all helpers, join every worker, then publish one final result. Add explicit helper diversification only after this version is correct; keep worker 0's starting policy unchanged. Cross-worker move voting is a later playing change.

This primary-result shape is directly grounded in [Counter's Lazy SMP](https://github.com/ChizhovVadim/CounterGo/blob/de02aef5a6887b82383ae9a7d6ce0614b730155f/pkg/engine/lazysmp.go). Stockfish's more involved [worker vote selection](https://github.com/official-stockfish/Stockfish/blob/edb0d9db6731067ec50ce619ff372b463bc4dd5d/src/thread.cpp) is a later reference, not a prerequisite.

Timed searches keep private node counters and publish periodic atomic snapshots for aggregate UCI statistics; worker 0's effort/time heuristics continue using its private counts. Sum final values after join. Avoid a contended atomic increment on every ordinary search node. For the first SMP milestone, explicit go nodes uses worker 0 only and reports the effective worker count, preserving existing one-thread semantics. This prevents helpers consuming the budget before the sole result owner completes an iteration. Concurrent node-limited search is a separate stage: CAS admission capped at N immediately before counted work, explicit primary reservation/fairness, no cancellation on the Nth successful admission, and a budget-exhausted sentinel on the next failed admission. Distinguish admitted from completed work and external stop/time cancellation. Test N=1, N<Threads, N=Threads±1, simultaneous final admission, interrupted work, legal fallback and incomplete-iteration suppression before enabling it.

### M3: clock, lifecycle and failure behavior

Start non-ponder timing at go receipt so parsing, cloning and NNUE refresh consume the move budget. Parse the cheap command, publish the session/cancel/completion handle, and return control to the UCI loop before cloning/context Reset/refresh in the coordinator. Setup must be cancellable; recheck stop/deadline before each worker launch. Compute a legal restricted fallback before expensive setup. Worker 0 owns mutable time policy; helpers only observe shared abort/deadline state. Check expiry after setup and preserve a legal root-restricted fallback. Resolve the existing 50M implicit node ceiling separately before interpreting long-clock/SMP scaling.

Use explicit idle/searching/pondering/stopping state and one completion signal. stop is idempotent cancellation plus join. position, ucinewgame, and state-changing setoption stop/join before mutation. The isready handler never joins or touches mutable worker state, including setup/ponder/search. When it follows a serialized state-changing command, that command's join/install completes first. A duplicate go cannot reset the active session. EOF/quit also join. Replace busy polling with completion synchronization.

Preserve model/Hash/Threads options through ucinewgame while clearing every worker's game state. Reconfiguration builds replacements off to the side and installs only on success. Natural completion and explicit stop publish exactly one bestmove; replacement commands, EOF/quit or rejected/superseded commands join while suppressing stale session output. A helper or leader failure cancels and joins all workers, then the coordinator emits one serialized info error and legal restricted fallback (0000 if no legal root) for an active go. Do not hide failure by continuing with fewer workers. Always run deferred completion cleanup, including evaluator failures during setup.

Existing incomplete ponder/searchmoves behavior must be handled in a separately reviewed protocol stage. Root move restrictions apply to every worker and fallback. Real pondering needs suppressed bestmove and dormant ordinary deadlines until ponderhit; do not claim full support from parsing an option. These fixes must not be bundled into the node-identical ownership stage.

Gate: fake-clock/blocking-Reset tests for stop/isready during setup, expiry entirely during setup and cancellation preventing late worker launch; stop racing natural completion; actual parallel searches under -race; stop immediately and during refresh/search; stop→position→go; reconfigure during search; repeated/duplicate go; quit/EOF; independent-engine cancellation; restricted roots; injected main/helper failure. Assert no leaked workers, deadlock, double bestmove, output after completion, stale generation or root-state corruption. Race tests establish only the exercised paths, so deterministic interleaving tests accompany stress. [Race detector scope](https://go.dev/doc/articles/race_detector).

### M4: demonstrate useful scaling

On the 8-core/16-logical 9800X3D, first test 1/2/4/8 physical cores. Discover actual topology and constrain each engine to distinct physical-core CPU sets; record GOMAXPROCS and affinity. Do not claim individual goroutine pinning from process affinity. Threads=16 is a separate SMT experiment.

Use c1 for directly comparable SMP gates; never c8 with 8-thread engines. With ponder disabled, only the side-to-move searches, but verify opponents and helpers actually park. If later parallelizing matches, concurrency × max(active engine threads) must fit eight physical cores. Keep total Hash fixed across thread counts and report per-worker/cache/model memory for both resident engines.

Repeat fixed-position wall-time measurements with warmed binaries and rotated CPU sets; report aggregate NPS, depth, CPU use, memory and stop latency. Then run A/A controls and paired equal-wall-time games at 2/4/8 versus 1. NPS gains alone do not close the milestone; eight workers must produce a useful playing improvement and pass longer-clock confirmation.

## Measurement and promotion

Before candidate games, establish accepted source on WSL with pinned compiler/binary identities. Pin and validate fastchess; the latest inspected release is v1.8.2-alpha, not an unnamed stable version. Verify which manual flags exist at that tag and retain the built binary's --help output. Exercise known final-ply mate, repetition, fifty-move, stalemate and clock fixtures through the actual runner, then a fixed 200-game A/A control with prospective criteria. This is a runner/termination sanity check, not a strength calibration; investigate unexpected bias without automatically treating random deviation as a harness defect.

Preserve full PGNs and UCI logs, openings/seeds, all source/binary/model checksums, CPU/thread/Hash settings, clocks, and independent legal/terminal replay. Disable optional score adjudication for initial conversion checks; do not enable crash recovery to conceal failures. [Fastchess release](https://github.com/Disservin/fastchess/releases/tag/v1.8.2-alpha), [official manual](https://github.com/Disservin/fastchess/blob/v1.8.2-alpha/man.md).

Candidate manifests are written before launch. Recommended short clock: 30+0.3; longer confirmation: 120+1. Use reversed-color opening pairs and pentanomial analysis. Default planning hypotheses: NNUE H0=0/H1=+10 Elo, SMP8 versus 1 H0=0/H1=+20, alpha=beta=0.05. Calibrate game/time caps from WSL A/A throughput/variance before candidate results, normally targeting the repository's 1–2 hour confirmation budget. Reaching a cap is inconclusive, not acceptance on a positive sign. A different longer test is a new prospective manifest; do not extend a run opportunistically.

These are test-design defaults, not a prediction or a claimed lower bound from an H1 verdict. [Paired-test method](https://official-stockfish.github.io/docs/fishtest-wiki/Fishtest-Mathematics.html).

Keep comparisons separable: accepted HCE1 → ownership HCE1 → NNUE1; HCE1 → HCE8; then NNUE1 → NNUE8 and combined longer-clock confirmation. Freeze the appropriate immediate base and do not pool different candidates. If HCE8 is only a diagnostic rather than a promoted release, state that explicitly. Profile/build/train outside timed-match windows.

Update the opponent pool to include pinned leading Go releases and stronger diverse engines. Existing published ratings are orientation, not comparable estimates that can be added to NGN's historical 2884 result. World leadership requires completed matched-condition comparisons, with independent testing desirable.

## Sequenced Sol work packages

1. WSL baseline/runner controls and immutable fixture/artifact snapshots.
2. M0 ownership slices, reviewed after each exact one-thread gate.
3. N0 format/export/scalar oracle; N1 integration only after shared ownership interfaces are settled.
4. N2 GPU smoke and bounded training pilot; N3 optimized inference and first strength candidate.
5. M1 coherent TT, then M2/M3 helper search and lifecycle in separately reviewable changes.
6. M4 scaling; combined long-clock verdict; deployment with accepted rollback.
7. External Stockfish-small proof can run as an isolated diagnostic after N1. Full Stockfish and richer trained architectures follow evidence, not a one-shot implementation.

Sol owns major code. Root reviews consequential numerical/concurrency contracts and verdicts, schedules exclusive CPU windows and keeps the accepted deployment intact. Independent work may overlap; overlapping mutation of the same files or simultaneous training/builds and timed matches may not.

## Remaining bounded decisions

The first corpus file and group-preserving decoder require a pilot-entry check. WSL GPU execution and actual inference throughput remain unmeasured. TT stripe count, helper diversification, SIMD choice and larger architecture size are measurement decisions, not research blockers. No arbitrary-network or across-version Stockfish compatibility is promised. A network-size discrepancy was reconciled against the archived LFS pointer and a fully hashed WSL download. HTTP length alone is not a model identity; every imported file must match its canonical full digest and declared format.

Research records and pinned source metadata are under /home/ehrli/repos/ngn/output/next-stage-20260905. The separate claim ledger records confidence, source and independent checks. This plan supersedes the initial roadmap where details differ; the wider active goal and acceptance requirements remain unchanged.
