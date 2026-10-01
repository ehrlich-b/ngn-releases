# Stockfish 18 BIG and dual-network compatibility: minimal path

Implementation status: the exact official BIG file is acquired and its full digest/size match the pinned archive's Git LFS object. Standalone loader v2 has passed source review; compilation, tensor parsing, tests and evaluation remain pending. Root source acceptance is `/home/ehrli/repos/ngn-sf18-big-loader/output/sf18-big-loader-20260906/root-source-review-v2.json` (SHA `e445ff693efe886a8a9fbf7329aeb23a87961bd216fcdc410cd9688c18222ef8`). The active Counter/HCE strength match retains exclusive engine/build compute.

Scope: exact public Stockfish 18 release commit `cb3d4ee9b47d0c5aae855b12379378ea1439675c`, extending the accepted standalone `nnue/sf18small` work. This is a source plan only. It does not wire a SMALL-only backend into search and makes no playing-strength claim.

## Exact compatibility target

Treat this as a named backend, `stockfish-18-dual-cb3d4ee`, rather than a generic `.nnue` loader. Stockfish 18 has two independent files and two immutable models:

- BIG default `nn-c288c895ea92.nnue`: HalfKAv2_hm base features plus FullThreats, transformer width 1024, downstream `15 -> 32 -> 1`, eight material stacks and eight PSQT buckets.
- SMALL default `nn-37f18f62d772.nnue`: HalfKAv2_hm only, width 128, the same downstream widths/stacks. Its accepted local artifact is 3,519,630 bytes, SHA-256 `37f18f62d772f3107e1d6aaca3898c130c3c86f2ab63e6555fbbca20635a899d`.

Primary source: `src/evaluate.h`, `src/nnue/nnue_architecture.h`, `src/nnue/network.h` at the pinned commit.

The raw model contract ends at `(PSQT, positional)` components, each divided by `OutputScale=16`. The release selector and score adaptation are a separate contract from `src/evaluate.cpp`:

1. Compute Stockfish material-only STM score using release values pawn 208, knight 781, bishop 825, rook 1276, queen 2538.
2. Use SMALL first iff `abs(simpleEval) > 962`; otherwise use BIG.
3. Blend the selected components as `(125*psqt + 131*positional)/128`, with C++ integer truncation toward zero.
4. If SMALL was used and `abs(blended) < 277`, re-evaluate BIG and replace the components/blend.
5. Apply complexity, optimism, material, rule-50, and the Stockfish tablebase clamp exactly only in an explicitly named Stockfish adapter. `Eval::evaluate` requires a position not in check.

The package API should preserve three distinct observations: raw network components, selector trace (first network, fallback, final components/blend), and full Stockfish-adapted score with caller-supplied optimism. An optimism-zero result must be labelled as such.

NGN cannot silently call the full result “exact Stockfish search evaluation”: NGN reserves mate at 30000 and ordinary static evaluation at +/-25000, while SF18 clamps to +/-31506. Search integration must use a separately named NGN adapter and clamp below NGN's mate band once. It must also avoid applying NGN rule-50 attenuation after the SF adapter already applied Stockfish's rule-50 term. That later policy choice is outside the standalone compatibility slices.

## Loader: exact BIG shape, not dynamic architecture parsing

Add a standalone `nnue/sf18big` package. Keep accepted SMALL code stable in the first slice; do not first refactor both packages into a generic dynamic-shape framework. Narrow common-code extraction can follow exact BIG/SMALL differential tests.

BIG constants derived from the pinned source hash functions are:

- file version `0x7AF32F20`
- HalfKAv2 input dimensions 22,528; FullThreats dimensions 79,856
- transformer hash `0x8F2344B8` (`0x8F234CB8 ^ (1024*2)`)
- architecture hash `0x63336A4A`
- top network hash `0xEC102EF2`
- maximum file length under the same 1 MiB description bound and canonical SLEB maxima: 156,266,767 bytes

The on-disk transformer order is materially different from SMALL:

1. 1,024 int16 biases in canonical signed LEB128; do **not** double them.
2. `79,856 * 1,024` raw int8 threat weights (81,772,544 bytes), exact byte order.
3. `22,528 * 1,024` int16 HalfKAv2 weights in canonical signed LEB128; do **not** double them.
4. One combined canonical signed-LEB int32 PSQT block of `(79,856 + 22,528) * 8 = 819,072` values, threat PSQT first, base PSQT second.
5. Eight stacks, each prefixed by BIG architecture hash. Each stack has FC0 16 int32 biases and `16*1024` int8 weights; FC1 32 int32 biases and `32*32` int8 weights; FC2 one int32 bias and 32 int8 weights.
6. Exact EOF.

Store canonical logical `[feature][lane]` tensors. Runtime SIMD permutations in `FeatureTransformer::permute_weights` and affine layer storage are implementation details and do not belong in the file model. Metadata must contain full file SHA-256/size, all structural hashes, exact feature/architecture/quantization/score-contract IDs and pinned adapter/source revision. Filename prefixes are not identity.

A loaded BIG model occupies 131,329,312 tensor bytes before slice/object overhead: 2,048 bias + 46,137,344 base FT + 81,772,544 threat FT + 720,896 base PSQT + 2,555,392 threat PSQT + 141,088 dense stacks. SMALL is 6,514,720 tensor bytes, so one shared dual model is 137,844,032 bytes (about 131.46 MiB) before descriptions and Go overhead. Allocate once, publish only after full validation, and share read-only across workers.

## Full-refresh arithmetic

The following accepted SMALL pieces transfer directly in behavior, though the first implementation need not refactor their source:

- HalfKAv2_hm square/piece planes, 32 horizontally mirrored king buckets and per-perspective orientation;
- material stack `(pieceCount-1)/4`;
- dense FC0, squared/clipped branches, FC1, FC2, the FC0 forward skip, wrapping/truncation rules, and `OutputScale=16`;
- strict immutable metadata and scalar trace strategy.

BIG-specific work:

- Generate FullThreats attack-edge features exactly from current occupancy. Both friendly and enemy occupied targets participate; exclusions and piece-pair maps must match `features/full_threats.{h,cpp}`. Threat orientation uses the perspective king's file half with a different orientation table from HalfKAv2; sharing one orientation helper is a compatibility bug.
- Base accumulation starts at the 1,024 biases with int16 base weights. Threat accumulation starts at zero with int8 weights. They are independent arrays.
- For each perspective and paired lane `j,j+512`, add base and threat with int16 narrowing/wrap **before** clamping each sum to `[0,255]` (matching scalar `std::clamp<BiasType>` and SIMD `vec_add_16`), multiply, divide by 512, and emit 512 bytes. The scalar slice needs an explicit overflow-boundary witness against upstream. SMALL instead uses doubled base tensors and `[0,254]`; applying SMALL load scaling or clipping to BIG invalidates parity.
- BIG PSQT is `(baseSTM-baseNTM + threatSTM-threatNTM)/2`, then divided by 16. Preserve component and intermediate traces before blending.

A scalar full refresh costs at most roughly 327,680 transformer lane additions for two perspectives (`32*1024` base plus `128*1024` threat per perspective), plus attack enumeration, pairwise transform and about 16,000 FC0 multiplies for the selected stack. An all-eight-stack trace multiplies the dense cost by eight. This scalar path is the reference oracle; optimize only after parity.

## Incremental and dual state

The semantic NGN move delta and post-position validation can serve both networks for HalfKAv2. Numeric accumulators cannot be shared: widths, tensors and SMALL's x2 load scaling differ. Every real/null transition must advance both contexts even when the selector evaluates only one, or a later SMALL-to-BIG fallback will read a stale stack.

For exact reusable state, one frame needs at least:

- SMALL base: two `128*i16` arrays plus two `8*i32` PSQT arrays = 576 bytes;
- BIG base: two `1024*i16` plus PSQT = 4,160 bytes;
- BIG threats: two `1024*i16` plus PSQT = 4,160 bytes.

That is 8,896 numeric bytes per ply before board facts, side, semantic delta and threat-diff metadata. At NGN's current initial 128-frame capacity it is at least 1,138,688 bytes per worker (8.69 MiB for eight workers); at Stockfish's fixed `MAX_PLY+1=247`, at least 2,197,312 bytes per worker (16.76 MiB for eight).

Stockfish Finny caches only HalfKAv2 base accumulators, separately for BIG and SMALL. From the pinned aligned structs, one worker's BIG cache is 278,528 bytes and SMALL cache 49,152 bytes, total 327,680 bytes; eight workers add 2.5 MiB. Threat refresh has no Finny table. These caches are optional after correctness, worker-private, and invalidated by the composite dual model identity.

Do not reproduce Stockfish's complex `DirtyThreats` generator as the first incremental step. A simpler exact reference can retain the before/after board, enumerate old/new active threat index sets, merge-diff them, and apply int8/int32 columns; refresh a perspective when the moving side's king crosses the file-half boundary, matching `FullThreats::requires_refresh`. Null frames copy/alias identical numerical state. This preserves model output, but its performance is unproven and it must be checked against upstream incremental traces before being called compatible. Only then consider Stockfish's fused/double updates.

The main scalar bottleneck is copying at least 8.9 KiB per pushed frame and applying up to 80 changed threat columns times 1,024 lanes; full threat refresh is up to 128 columns per perspective. A later frame layout may lazily materialize BIG state because many nodes select SMALL, but that optimization must preserve transition history and fallback correctness. No interface dispatch should occur inside lane loops.

## Pluggable NGN boundary

After standalone BIG and dual parity, extend the accepted concrete tagged evaluator model with one backend value, not a generic NNUE interface:

- immutable `sf18dual.Model{Big, Small}` shared by all workers;
- comparable identity containing both full file digests/sizes, SF18 adapter revision and the still-required HCE/PST generation;
- worker-private `sf18dual.Context` owning synchronized BIG/SMALL stacks and scratch/cache state;
- cold `SelectSF18DualEvaluator(big,small)` that validates and prebuilds a candidate worker before receiver publication;
- existing `Reset`, `PrepareMove`, `PushMove`, `PrepareNull`, `PushNull`, `Pop`, `SearchSTM` branches, with rollback or prevalidation guaranteeing that a failure cannot advance only one child context.

TT/history invalidation remains driven by the composite identity. The model owns no TT, locks or globals. SMP shares immutable tensors but never mutable accumulators/caches. Keep standalone raw/selector/full-SF score APIs available so engine adapter tests can detect double scaling or damping.

## Implementation gates

**First independently testable slice: strict immutable BIG loader only.** Prerequisites before coding are an official BIG artifact receipt and exact license/provenance binding. Fetch the source-declared filename through the official Stockfish network endpoint, record redirect, byte size, full SHA-256 and exact grammar; independently confirm the same exact file is included by the pinned `official-stockfish/networks` revision before applying its blanket CC0 statement. The existing SMALL receipt and archive claim do not by themselves prove the BIG blob's license. Stockfish source is GPLv3-or-later; preserve source notices for copied/derived routines and keep model-byte provenance separate.

Loader gates:

- independently derive/check BIG structural hashes and block counts against pinned C++;
- stream/bound all canonical SLEB blocks, raw threat bytes, description and total file; reject short/trailing/noncanonical/range/hash/order failures without publishing a partial model;
- official file exact digest/size/header and independent tensor spot checks across bias, raw threat interior, base FT interior, both PSQT halves and interior values in every stack;
- bounded parser-unit fixtures use reduced test-only counts; avoid repeatedly constructing 100+ MiB synthetic production-shaped malformed files.

Second slice is scalar BIG full refresh with an upstream-production oracle that calls actual `NetworkBig::evaluate`/`trace_evaluate`. Its corpus must cover all HalfKAv2 king buckets and mirrors, both sides to move, every piece/attacker-target family, slider blockers, friendly attacks, sparse/dense threats, all eight material stacks, and official no-overflow checks. Compare active base/threat indices, both accumulator families and PSQTs, transformed 1,024 bytes, all stack intermediates and final components. Only after that passes should an incremental BIG context be added and compared after every real/null push/pop and full unwind.

Third slice is the standalone dual coordinator/selector with upstream `Eval::evaluate` fixtures, including threshold boundaries 962/277, SMALL fallback and no-fallback, negative truncation, all score-adapter terms, optimism variants, rule-50 and in-check rejection. Search/UCI wiring remains a later separately reviewed slice after exact raw, incremental and dual parity plus memory/performance measurements.

## Compatibility-invalidating risks

- accepting a structural hash while parsing the wrong BIG block order, especially combined threat/base PSQT;
- doubling BIG tensors or using SMALL's 254 clip instead of BIG's base+threat 255 clip;
- conflating HalfKAv2 and FullThreats king orientation/refresh conditions;
- advancing only the network selected at the current node;
- deriving FullThreats from piece occupancy without exact slider/blocker attack semantics;
- comparing only final scores, allowing wrong feature order, stack selection or PSQT/positional cancellation;
- claiming full `Eval::evaluate` parity with optimism forced to zero, in-check inputs, NGN's +/-25000 clamp, or duplicate rule-50 damping;
- loading a later Stockfish network that shares `.nnue` suffix but has a different structural hash/feature family;
- treating source default filename or digest prefix as full model/provenance identity.

Primary source URLs:

- `https://github.com/official-stockfish/Stockfish/blob/cb3d4ee9b47d0c5aae855b12379378ea1439675c/src/nnue/nnue_feature_transformer.h`
- `https://github.com/official-stockfish/Stockfish/blob/cb3d4ee9b47d0c5aae855b12379378ea1439675c/src/nnue/features/full_threats.cpp`
- `https://github.com/official-stockfish/Stockfish/blob/cb3d4ee9b47d0c5aae855b12379378ea1439675c/src/nnue/nnue_accumulator.cpp`
- `https://github.com/official-stockfish/Stockfish/blob/cb3d4ee9b47d0c5aae855b12379378ea1439675c/src/nnue/nnue_architecture.h`
- `https://github.com/official-stockfish/Stockfish/blob/cb3d4ee9b47d0c5aae855b12379378ea1439675c/src/nnue/network.cpp`
- `https://github.com/official-stockfish/Stockfish/blob/cb3d4ee9b47d0c5aae855b12379378ea1439675c/src/evaluate.cpp`
- model-license archive revision: `https://github.com/official-stockfish/networks/tree/dd5c7f74c073a7d0bcbb52648d26c537a3d04cf4`
