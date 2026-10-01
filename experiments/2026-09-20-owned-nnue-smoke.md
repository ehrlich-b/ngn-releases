# Owned K4 NNUE shared-machine smoke

Status: **PASS for graph/export/runtime feasibility; not strength-eligible.**

This bounded smoke was run on the shared WSL RTX 5080 while the Lean hopper
remained live. Training was restricted to logical CPUs 12-15 at nice 10, with
two loader threads and queue capacity 8. It used 130 MiB of observed host space
(Windows C: free moved from 5.81 to 5.68 GB). Because the host remains below the
plan's 60 GiB launch gate, this exception was limited to the measured smoke;
pilot labeling/training and all bulk work remain held.

## Result

- Pinned Bullet: `629ee50000b2afb7b3337595401c830d3b1e0f42`.
- Model: `ngn-k4-768-v1`, seed 26092001, four mirrored king buckets, 768
  hidden units, eight material heads.
- Bootstrap input: exactly 1,000,000 retained public-label BF records,
  SHA-256 `4a9c68d4105b8747010bfd87fcf17eea48116bb0660313bbfe65b0f656eadb08`.
- Work: 256 updates / 4,194,304 presentations in 1.8 seconds; final reported
  throughput 6,942,164 positions/s. Queue-full and exact-consumption witnesses
  were present.
- Candidate quantized Bullet artifact:
  `2c76a029b7600ba6dd1aaf31bd0cba6062c10719fca4765336480c03175ee3f2`.
- Strict manifest-bound NGN model:
  `1d7d5b16279a4daf3f6ac59b9858315b4570007a18fcebcaa5458c629caf916e`.
- WSL artifact root:
  `/home/ehrli/nnue-owned-k4-20260920/runs/smoke-public-bootstrap-v2`.

Attempt v1 failed closed before training. It exposed that Bullet's ordinary
`save_unquantised` output ignores save-format transforms: it omitted the shared
factorizer merge and output transpose even though the quantized exporter applied
them. The maintained harness now writes an independently transformed deployed
float stream and requires every exported i16 to equal its prospective
quantization. Attempt v2 passed that full-tensor gate.

## Independent parity

The frozen 12-position corpus exercises both sides to move, all four king
buckets/file mirrors and every `MaterialCount<8>` output head. Bullet evaluated
the retained optimizer checkpoint through its own feature mapper and graph; the
Go bridge independently decoded/evaluated the deployed float stream and the Go
runtime evaluated the strict quantized model.

- Tensor raw-to-model quantization: exact for all 2,372,360 deployed values.
- Bullet vs independent float evaluator max delta: `4.817180991167208e-8` z.
- Float vs quantized Go absolute delta: mean 3.00251 cp, p99 12.36654 cp,
  max 12.36654 cp; passes the prospectively frozen 8/32/64 gates.
- Go scalar vs Go incremental context: exact on all positions.
- Receipt SHA-256:
  `fed7ef3897660c3a3acbd72e9749f98173c86e1a03dd17960c19d0584f641285`.

The Linux engine then loaded the manifest-bound model through the public UCI
configuration and completed non-book depth-3 and depth-4 searches, including a
castling-rights position. The focused Go tests, race tests and vet gate passed.
The broad legacy suite still has the separately known concurrency-stress failure
and depth-100 timeout; this smoke does not relabel those as K4 regressions.

## Interpretation

This proves the changed graph, optimizer checkpoint, transformed exporter,
strict format, Bullet/Go feature semantics, quantized evaluator, incremental
stack and UCI selection path can operate together on the shared machine. It does
not estimate Elo and must not be promoted: the smoke reused historical public
labels, performed only 256 warm updates, and had no checkpoint selection or game
gate. The next strength-bearing work is the deterministic fresh-label pipeline,
integer-validation selector and 1M cold pilot after host disk headroom is fixed.
