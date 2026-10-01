# NNUE N2 export and parity bridge

Status: reviewed and integrated; required WSL checks pass. This adds an offline converter and parity tool. Production search remains on HCE, and no trained-network strength claim is made.

Sol implemented only cmd/nnuebridge/bridge.go, main.go, and bridge_test.go, based on d346faf. Root reviewed their transport layout, conversion, independent arithmetic, CLI, negative controls, and pinned Bullet writer snapshots, then copied the exact source bytes to the integration worktree.

## Pinned transport and arithmetic

The contract is Bullet commit 629ee50000b2afb7b3337595401c830d3b1e0f42 and its simple dual-perspective Chess768 network with 128 hidden units, SCReLU, QA=255, QB=64, and scale=400. It is a named architecture contract, not a generic Bullet or Stockfish file loader.

- bullet-quantized: exactly 197440 bytes, comprising 197378 tensor bytes and 62 bytes of repeating ASCII bullet padding.
- bullet-raw: exactly 394756 unpadded little-endian float32 bytes, in l0w/l0b/l1w/l1b order.
- Output: the strict NGN v1 format, self-validated by Marshal and Load.

The raw conversion exercises the production quantizer and checks every tensor against an independent float64-multiply, round-away-from-zero, checked-int16 quantizer. Independent integer inference preserves both signed truncating divisions and uses int64. Scalar and Context outputs must equal it exactly. The portable float32 evaluator is diagnostic; exact CUDA/trainer reduction behavior remains a later smoke/pilot gate. Context parity here uses fresh Reset; incremental make/unmake lane parity is covered by N1a.

Primary writer evidence is retained with source blob IDs and SHA-256s in the Sol evidence package, from:
- https://github.com/jw1912/bullet/blob/629ee50000b2afb7b3337595401c830d3b1e0f42/crates/trainer/src/model/weights.rs
- https://github.com/jw1912/bullet/blob/629ee50000b2afb7b3337595401c830d3b1e0f42/crates/bullet_lib/src/value/save.rs
- https://github.com/jw1912/bullet/blob/629ee50000b2afb7b3337595401c830d3b1e0f42/examples/simple.rs

## Usage

Run only on WSL:

    go run ./cmd/nnuebridge convert -format bullet-quantized -in quantised.bin -out model.ngnnue
    go run ./cmd/nnuebridge convert -format bullet-raw -in raw.bin -out model.ngnnue
    go run ./cmd/nnuebridge parity -format bullet-raw -in raw.bin -fens positions.fen

JSON receipts bind the writer commit, arguments, source/model hashes, FEN file hash, per-position scores, and maximum differences. Integer disagreement fails the parity command. Invalid input fails before publishing a model file.

## Verification

Manual tests pin literal tensor offsets, malformed size/padding, nonfinite and out-of-range values, positive/negative rounding witnesses, asymmetric side-to-move ordering, clipping, both negative truncations, and extreme int64-safe tensors. CLI fixtures prove raw and quantized routes produce identical NGN files and exact +400/-400 parity.

Root's integrated checks all exited 0 with GOMAXPROCS=2 and CPU affinity 4,6:
- focused verbose bridge tests;
- full short tests;
- full short race tests;
- vet;
- build.

Evidence: /home/ehrli/repos/ngn-next/output/nnue-n2-integration-20260905, including receipt.json, source hashes, all logs, DONE_EXIT_0, and a frozen copy of Sol's complete source-writer and CLI evidence.

Next gate: pinned trainer/toolchain GPU smoke, a bounded data pilot, and real exported checkpoint parity. No CUDA installation, training, production NNUE wiring, or matches are implied by N2 acceptance.
