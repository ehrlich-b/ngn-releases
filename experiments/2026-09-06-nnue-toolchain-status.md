# NNUE public toolchain status

The WSL GPU/toolchain entry gates pass. Root read the actual native/NVRTC/cuBLAS logs and version/source receipts; hashes are retained in output/nnue-public-preflight-20260906/root-gpu-smoke-review.json. Sol performed installation and tests in /home/ehrli/nnue-public-toolchain-20260906. The accepted engine deployment and rollback are unchanged.

Pinned Bullet 629ee50000b2afb7b3337595401c830d3b1e0f42 declares Rust1.87 but uses let-chains in crates/compiler/src/tensor/pattern.rs. The original rustc1.87 build failed with 136 E0658 errors before GPU execution (Cargo101, 0.70s, peak RSS181928KB). Its original source, toolchain and log remain preserved. The user explicitly authorized full WSL access and bidirectional source/log transfers, resolving the earlier automatic-review block on reading that diagnostic.

The minimal corrective toolchain is official-checksummed user-local Rust1.88.0, with unchanged pinned Bullet source, Cargo.lock and offline dependencies. CUDA12.8.1 is toolkit-only, retaining the Windows GPU driver; the installed nvcc identifies itself as V12.8.93. Do not infer component versions from the installer name.

Completed actual checks:
- Native four-element CUDA kernel compiled with -arch=sm_120, correct output, cuobjdump target verified.
- cargo test --locked --offline --release -p bullet-gpu --features cuda runtime::tests::cuda::compile_load_execute_kernel -- --exact --nocapture: one test passed, 28.00s including build, peak RSS365784KB.
- cargo test --locked --offline --release -p bullet-gpu --features cuda tests::cuda::axby -- --exact --nocapture: one test passed, 1.60s, peak RSS567192KB. This exercises one narrow FP32 cuBLAS layout, not every matrix operation or derivatives.

The next smoke remains source-review gated. Sol is preparing a bounded 128-hidden dual-perspective SCReLU forward/backward/AdamW harness, numeric checks of representative derivatives, direct checked exports, and Go bridge parity. The bundled public batch fixture is sufficient; no T80 payload or real training is needed yet.

A further pinned upstream setting bug was verified: LocalSettings.batch_queue_size is not passed by ValueTrainer::run; crates/trainer/src/run.rs hardcodes sync_channel(32). The isolated harness must wire the configured capacity through, prove the actual bound, use two loader threads and queue8, and propagate errors. Setting queue8 in LocalSettings alone is not resource enforcement.

Earlier research corrections remain in 2026-09-06-nnue-public-preflight-review.md: ignored test_set, time-seeded SF loader ordering, swallowed checkpoint export errors, and unverified historical teacher score units. No trained network, production NNUE integration, or playing-strength verdict is established by these toolchain tests.

## End-to-end pipeline acceptance

Root accepted the bounded synthetic pipeline smoke after independently reading attempt 3 and verifying every final file hash. The two-layer Chess768 network (128 hidden units, dual perspective SCReLU) completed 49,152 presentations on the WSL RTX 5080. Six representative CUDA gradients match independent central differences; the largest error is 1.154e-6. Asymmetric legal positions make the same-hidden STM and NTM gradients distinguishable, and swapping those actual gradients is explicitly rejected. Zero, sign-flipped and doubled nonzero gradients are also rejected. This is representative derivative coverage, not an exhaustive check of every weight.

Initial and final GPU outputs match the independent floating evaluator. Raw and quantized exports both convert to the identical NGN file, SHA256 c7581bfae2e43e267aae6b2cbd24d85596c35e89b1728c27f45d9ccd4c9300a6, with zero integer delta for all 16 frozen positions through the independent bridge, Go scalar and reset-context paths. The measured floating-to-integer difference reaches 7.5959167 cp; it remains a calibration diagnostic. Existing N1a tests separately cover incremental transition/undo accumulator equality.

The queue-capacity-eight full witness passed, optimizer state and weights remained finite, and all 98,689 weights changed (weight decay also changes unused weights). GNU time reported maximum RSS 651,724 KiB; this is not an aggregate process-tree memory cap. Eighteen numeric global VRAM samples peaked at 2,016 MiB. Wrapper timeout was 600 seconds; the command completed in 17.85 seconds including compilation.

Accepted evidence: /home/ehrli/nnue-public-toolchain-20260906/runs/ngn-v0-smoke-attempt3. Root review: output/nnue-public-preflight-20260906/root-pipeline-smoke-v3-review.json. Attempt 1's compile failure and attempt 2's pass with symmetric derivative coverage are preserved. No real-data, production-search, optimized-inference or playing-strength verdict follows from this smoke. The next step is the bounded, independently audited T80 data pilot.
