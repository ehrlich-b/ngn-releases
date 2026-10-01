# NNUE N0 foundation — 2026-09-05

Sol implemented the standalone nnue package in /home/ehrli/repos/ngn-nnue-v1; root reviewed its source and independently integrated/tested the exact files in ngn-next.

Delivered: the fixed NGN v1 container and SHA256 identities, immutable loaded model, engine-independent Chess768 piece/square inputs, full-refresh signed64-bit scalar evaluation, and float32 tensor quantization matching the pinned Bullet writer's float64 scaling/rounding. The loader rejects unsupported fields, invalid sizes, corrupt payloads and trailing data. This package has no engine/search import or UCI wiring.

Review strengthened the float input-domain contract, interior tensor-order fixtures, multi-piece/perspective arithmetic, zero-model rejection, immutable ownership and valid intermittent-reader behavior. Tests build expected containers at literal offsets without using the production serializer or feature mapper.

Integrated checks all exit0: package short tests, package race tests, full short ./..., package vet and main build. Commands/logs/exits, source hashes and the root receipt are in output/nnue-n0-integration-20260905.

The resulting default HCE binary is byte-identical to B0: SHA256 5d834a0b3246ac73dacb892ef36e136cf5376dc1e44f659d6b60546e9b7f6a70. Source-model identities and accepted deployment remain unchanged.

Scope remains limited to N0: no trained-network, incremental/optimized inference, trainer execution, Stockfish import, search-adapter or playing-strength claim. Next is a worker-private preallocated incremental context, checked against this scalar reference, then engine integration after ownership gates.
