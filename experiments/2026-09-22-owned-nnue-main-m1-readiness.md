# K4-M1 schedule readiness

The frozen 20M candidate schedule from the K4 failure diagnosis is implemented
as a separate `main-m1` training and selection mode. This is code readiness,
not authorization to start the 20M expansion or evidence that K4 has reached
the strength gate. The ongoing 5M probe remains the prerequisite.

`main-m1` consumes the same exact 20M finalized corpus contract as the
withdrawn `main` schedule: 1M pilot prefix, 19M ordered expansion records and
inherited fixed holdouts. It uses batch size 16,384, 131,072 updates,
4,096-update checkpoint intervals and the existing AdamW cosine settings. The
selector cannot trigger early stopping before update 32,768 and records a
retrospective stop after six consecutive non-improving checks. It selects by
the same strict validation integer MSE and quantization gates as earlier K4
probes. The old `main` mode remains distinguishable in its receipts so that no
historical schedule is silently reinterpreted.

The M1 trainer was compiled offline from the pinned Bullet commit and patch in
an isolated WSL workspace, leaving the active 5M trainer untouched:

The initial build omitted Bullet's `cuda` feature and produced a mock-runtime
binary (`17020af410bb67cdaea783291796295bffa8c9d088af3d218de7d283d95cfed8`).
It was replaced in the isolated M1 workspace by the CUDA-enabled build below;
the mock binary is not a training candidate.

- Rust harness source SHA-256:
  `febb926b3926ece196975153bc64d8af3ae0531e4dfceb08a4950f6ed703971a`.
- Bullet patch SHA-256:
  `f7f5e0dd02695fe57ec58a630b63006da96cb1ec6b8fd4769a4e6308a8121e54`.
- CUDA-enabled M1 trainer binary SHA-256:
  `5736805d5418471907f0f88a20b6351679c6799b7aed72e45d96d54ebb8b01bf`.
- Active 5M trainer binary SHA-256, verified unchanged:
  `6a989de4ca6746b2f7c10938206ed3a2e89d1c50beef580ff08399cafa805f0b`.
- M1 Linux bridge binary SHA-256:
  `50cc5904a306f167a3b2ae8d71161192454c8e09fb48ec36ef546dbf6a4686d9`.

The selector previously understood the 5M finalized manifest but treated a
20M main manifest as a 1M pilot. Its main-corpus identity and record-count
check is now explicit. A focused fixture accepts a valid inherited 20M main
manifest and rejects a 5M count; another checks the minimum and early-stop
behavior. `go test ./cmd/ngnk4bridge`, `go vet ./cmd/ngnk4bridge`, and the pinned
offline release build passed. No GPU training or new labeling was started by
this readiness work.
