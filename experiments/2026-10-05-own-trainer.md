# NGN-owned NGNN1 trainer — 2026-10-05

Implemented from the supplied NGNP1/NGNN1 specification, without reading or
reusing another engine/trainer/toolchain. Dependencies: Python 3.12.3,
NumPy 2.5.3, PyTorch 2.7.0+cu128. All execution occurred in the isolated WSL
worktree `/home/ehrli/wt/own-trainer-20261005`; the Mac was used for editing.
The existing venv supplied only its interpreter and general-purpose libraries;
no installs or venv modifications. GPU: RTX 5080, initially idle. CPU threads
bounded to four, plus one prefetch thread; outputs total 179 MiB.

Design: raw 32-byte shards mapped read-only; validate and filter in bounded
blocks; shuffle blocks and their accepted records; transfer packed bytes and
construct both 768-feature perspectives on GPU. W1/bias/SCReLU/output ordering
matches the shared specification. AdamW, bias decay zero, default score/WDL
blend 0.75 and K=400, quantization clipping after every step. CUDA defaults to
bfloat16 matrix operations with float32 parameters/loss. No compiler needed
for measured throughput. Epoch/superbatch CLI, cosine/step/fixed LR, atomic
checkpoint/export, and epoch-boundary resume. Export rounds ties to even,
uses little-endian NGNN1 and IEEE CRC32. NumPy reference widens to int64 and
performs both Go-style truncating divisions explicitly.

Commands, from the WSL worktree:

```bash
export GOAMD64=v3 OMP_NUM_THREADS=4 MKL_NUM_THREADS=4 OPENBLAS_NUM_THREADS=4
PY=/home/ehrli/ngn-original-nnue-night-20261001/venv/bin/python
RUN=/home/ehrli/ngn-data/own-trainer-20261005
"$PY" -m trainer.make_random_net
"$PY" -m unittest discover -s trainer/tests -v
"$PY" -m trainer.tests.synthetic --records 131072 --out "$RUN/smoke.ngnp"
"$PY" -m trainer.train --data "$RUN/smoke.ngnp" --hidden 64 --batch 4096 \
  --lr 0.003 --epochs 24 --decay none --out "$RUN/smoke"
"$PY" -m trainer.tests.check_material "$RUN/smoke/epoch-0024.pt" "$RUN/smoke/epoch-0024.nnue"
"$PY" -m trainer.tests.synthetic --records 5000000 --out "$RUN/throughput.ngnp"
"$PY" -m trainer.train --data "$RUN/throughput.ngnp" --hidden 256 \
  --batch 16384 --epochs 2 --decay none --out "$RUN/throughput"
```

All **15 tests passed** on WSL (8.658 s). Re-exporting the trained CUDA
checkpoint through the export CLI produced identical bytes; reference CLI
evaluated the bare queen position at +783 cp.

Tests: hand-built decode including bit 63; white/black labels; feature mapping
and color/vertical mirror invariance; exact CPU/GPU dense features; flags/ply
filters, many-shard shuffle and partial tails; invalid records; export layout,
CRC/corruption; random float/integer agreement (maximum **3.7268 cp** across
288 cases at H=16/32/64); independent scalar integer comparison; H=2048 sums
exceeding int32; clipping/rounding; deterministic fixture regeneration; exact
uninterrupted-versus-resumed model/loss; LR endpoints and superbatch cycling.

Synthetic positions have kings on e1/e8 and a subset of each side's initial
non-king inventory, distinct squares, pawns on ranks 2–7, and other pieces
on any free square. Score is material, WDL is sampled with expected value
sigmoid(material/400). They do not enforce check/reachability. An initial
generator restricted every non-king to ranks 2–7: it learned held-out scores
but missed queen features on d1/d8. The corrected generator covers back ranks;
all results below use that version. This tests trainer plumbing, not Elo.

H=64: 24 epochs, 768 updates, 3.452 s summed training time. Loss at epochs
1/2/4/8/16/24: **0.066673 / 0.010322 / 0.009076 / 0.008850 / 0.008106 /
0.006921**. Held-out loss **0.010058**, material correlation **0.995460**.
Quantized evaluations: bare white queen +783 cp (black STM −766); starting
position without black queen +834 cp; without white queen −767 cp; balanced
starting position +41 cp. Sampled WDL labels impose a nonzero loss floor.

H=256 throughput on **5,000,000 distinct generated records** (160 MB):

| Epoch | Updates | Loss | Wall seconds | End-to-end positions/s | Steady positions/s |
| --- | ---: | ---: | ---: | ---: | ---: |
| 1 | 306 | 0.018300 | 1.749664 | 2,857,691 | 4,310,760 |
| 2 | 306 | 0.008964 | 0.971381 | 5,147,313 | 5,164,052 |

Both rates include shuffle/read, packed host transfer, GPU decode, forward,
backward, optimizer, and clipping. CUDA synchronized at timing boundaries.
Steady excludes the first ten updates. Shard validation and checkpoint/export
writes are excluded; epoch 1 includes CUDA startup. The warm epoch reads an
OS-cached shard, not a dataset retained on GPU. Target >=1M positions/s met.
`/home/ehrli/ngn-data/pilot/` was absent at the final data check; no real-data
smoke performed and no other agent's data/process was modified.

Fixture: `testdata/ngnn1/random_h32.nnue`, **49,376 bytes**, seeded 20261005;
64 integer FEN cases in `testdata/ngnn1/random_h32_evals.json` (6,721 bytes).
SHA256 of the network:
`9bb171cd06035f7b77089fbd31b215812057a4b44950190fd741d133dd6f52c6`.
SHA256 of the JSON:
`d92dd1edd84cfcf6c8706653e3654771b0b65a45dabd4920a17d5e6209bc5a6f`.
