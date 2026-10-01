# NNUE public-source preflight review

This is a research addendum, not a provisioning or training receipt. Public primary sources were checked on September 6, 2026. No installation, download of the training payload, GPU smoke, or training was performed for this review.

## GPU/toolchain acceptance

NVIDIA lists RTX 5080 as compute capability 12.0. CUDA 12.8.1's compiler documentation explicitly supports compute_120 and sm_120. The generic archived Blackwell compatibility guide emphasizes compute capability 10.0, so its examples alone are insufficient to specify this consumer GPU's target.

- https://developer.nvidia.com/cuda/gpus
- https://docs.nvidia.com/cuda/archive/12.8.1/cuda-compiler-driver-nvcc/index.html
- https://docs.nvidia.com/cuda/archive/12.8.1/blackwell-compatibility-guide/index.html

The WSL installation must provide Linux toolkit components while retaining the Windows-supplied GPU driver. NVIDIA's guide excludes the cuda, cuda-12-x, and cuda-drivers meta-packages for this purpose. It also documents limited pinned host memory and incomplete NVML queries. Those constraints favor a small queue and bounded memory in the first trainer smoke; an absent NVML process listing cannot prove that a process has stopped.

- https://docs.nvidia.com/cuda/wsl-user-guide/index.html

CUDA 12.8 Update 1 lists nvcc 12.8.90, NVRTC 12.8.93, and cuBLAS 12.8.4.1. Its updated release notes also list a separate cuBLAS 12.8.5 patch. The listed fixes concern particular cuBLASLt algorithms and ztrmm; this review does not establish that the planned simple FP32 network triggers them. Freeze the actual loaded component versions and library paths; do not treat a toolkit directory name as a complete dependency pin.

- https://docs.nvidia.com/cuda/archive/12.8.1/cuda-toolkit-release-notes/index.html#cublas-patch-release-12-8-5

Acceptance still requires an actual GPU context, compiled/runtime kernel path, forward/backward test, finite optimizer update, checkpoint export, and independent Go parity on the host. Documentation proves support in principle, not a working Bullet installation.

## Teacher data provenance

The publisher's archive identifies its T80 data as converted Leela training data and links the converter and filtering tools. Stockfish PR 5333 separately lists test80-2022-08-aug-16tb7p.v6-dd.min.binpack as one training component. Use by Stockfish does not make these labels current Stockfish UCI centipawns.

- https://robotmoon.com/nnue-training-data/
- https://github.com/official-stockfish/Stockfish/pull/5333
- https://github.com/linrock/lc0-data-converter
- https://github.com/linrock/nnue-data

The public conversion script currently requests best score, best move, tablebase rescoring, and deblundering. This is evidence about the published workflow, not a receipt that binds one exact converter revision and invocation to the frozen August 2022 file.

- https://github.com/linrock/lc0-data-converter/blob/master/rescore_tar_files.sh

Before accepting the real-data pilot: identify the score transform and side-to-move/result convention, retain literal decoded examples, verify feature indices against the independent reader, and record any remaining teacher-unit assumption. Do not apply current Stockfish UCI normalization to this archive merely because its transport is binpack. The transport's signed integer score representation alone does not establish its semantic scale.

The official 2023 training wiki reports better results from staged Stockfish-generated then Leela-derived data than from Leela-only training. This is historical evidence to test if our minimal model plateaus, not a claim about the best 2026 recipe or a reason to enlarge the first pipeline pilot.

- https://github.com/official-stockfish/nnue-pytorch/wiki/Training-datasets

The immediate sequence remains: synthetic GPU/gradient/export smoke, bounded real-data pilot with verified label contract, then a separately declared strength experiment. Exact model-format parity is necessary; it does not prove that labels or playing strength are correct.

## Required correction: external validation and fixed data order

This section supersedes earlier planning that proposed LocalSettings.test_set as the validation mechanism for Bullet 629ee500. Root independently fetched that exact public source on WSL and verified that ValueTrainer::run only emits a not-implemented warning for the option. Its training call receives the training loader; it does not evaluate or select checkpoints using the supplied validation data.

- https://github.com/jw1912/bullet/blob/629ee50000b2afb7b3337595401c830d3b1e0f42/crates/bullet_lib/src/value.rs
- Verified source SHA-256: f3e36f52c91c6b604846a83ed1b7241926557ce1ea4539389258c67b3c746fe6.

Use an external evaluator over frozen checkpoints, and explicitly save the initial checkpoint as an eligible candidate. The validation set must demonstrably influence checkpoint selection; altering sealed-test labels must not influence it. The trainer must never receive the sealed-test path. Evaluate fresh checkpoint state, not a potentially cached live trainer evaluator.

The pinned SF-binpack loader also calls seeded_rng for its shuffles; that RNG is initialized from the current time. A model seed alone does not freeze the data order. For the proof pilot, decode and audit first, retain source metadata in a sidecar, freeze a deterministic hash order, and use DirectSequentialDataLoader. Verify actual batch order with a receipt.

- https://github.com/jw1912/bullet/blob/629ee50000b2afb7b3337595401c830d3b1e0f42/crates/bullet_lib/src/value/loader/sfbinpack.rs
- https://github.com/jw1912/bullet/blob/629ee50000b2afb7b3337595401c830d3b1e0f42/crates/bullet_lib/src/value/loader/rng.rs

The public snapshots and hashes are retained under output/nnue-public-preflight-20260906. These are source-based design corrections; no training behavior has yet been tested.

## Bounded pilot contract prepared by Sol

Target 100000 accepted positions, whole encoded-chain splits targeting 80/10/10 percent with actual counts frozen. Quarantine or consistently reassign whole chains for cross-split model-input duplicates; require zero exact model-input overlap between splits. The source format does not prove original-game identity, and this process must not claim game-disjointness.

Use explicit model seed 198273612 and deterministic data order. Budget 16384 positions per batch, 32 batches per superbatch, four superbatches: 2097152 presentations. Save initial plus all four checkpoints. Two loader threads, queue depth eight, peak RSS below 12 GiB, peak VRAM below 14 GiB.

For plumbing only, the proposed target uses result weight 0.75 and raw stored-score weight 0.25 with divisor 400 in raw_binpack_score_unit. This is an explicit provisional label contract, not evidence that T80 is measured in centipawns. Resolving or empirically validating teacher-label calibration remains necessary before strength interpretation.

Separate gates: GPU/kernel and gradient smoke; fixed-checkpoint float/export/integer parity; data provenance/count/dedup and selection isolation; then predeclared real-clock games. No installation or run is authorized by this document alone; the existing user task authorizes eventual WSL work, with root scheduling and review controlling execution.

## Required correction: validate every checkpoint export

The pinned Bullet save_to_checkpoint catches raw and quantized export errors and prints them without propagating them to the training caller. The quantized writer creates its output file before tensor quantization, so file presence alone is also insufficient. Every eligible checkpoint must pass strict raw/quantized size, parse, range, hash and parity checks. A directory, process exit zero, or falling training loss cannot substitute for those checks. A wrapper using the direct save methods must propagate their Result errors.

- https://github.com/jw1912/bullet/blob/629ee50000b2afb7b3337595401c830d3b1e0f42/crates/bullet_lib/src/value/save.rs
- Source SHA-256: 49a6113bec059df11ec001ed5d09631d2defd63b0a7d877b850ba258092f00f6. The public snapshot is retained alongside the other preflight source evidence.

ModelEvaluator.load_device_weights clones shared Arc buffers. One-time evaluator binding alone therefore does not prove that the live evaluator is stale; the earlier recommendation to evaluate frozen checkpoints is an isolation contract, not a confirmed upstream stale-weight defect.
