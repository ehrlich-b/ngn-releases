# NGN-owned NGNN1 inference (2026-10-05)

Base: `74d63eec8f1ee3d18f4c7a3014e486ae0e1f964f` (`indep/main`).
Source edits are confined to the Mac `own/netinfer` worktree; all builds and
execution use `/home/ehrli/wt/own-netinfer` on WSL, Go 1.25.5, `GOAMD64=v3`.
Implementation and random probe weights are written from the shared NGNN1
specification; no third-party evaluator code or data was consulted.

## Implementation

The loader validates magic/version, width (16..2048 in multiples of 16),
QA/QB/SCALE, exact length, and IEEE CRC32 before publishing immutable weights.
Each worker owns a growable per-ply int32 accumulator stack (White, Black),
initialized by a mailbox refresh. Packed moves supply piece removal/addition
features for captures, all promotions, castling and en passant. Push copies the
parent; pop restores by stack index. Null moves retain features and exchange
the output perspective. SCReLU uses int32, the dot sum uses int64, both specified
divisions truncate toward zero, and scores clamp to +/-24,999 cp. No HCE
tempo or halfmove scaling is added to NGNN1 scores.

`EvalFile` defaults empty and `UseNNUE` defaults false. Failed loads discard
prior weights, emit an info-string error, and select HCE. Model identity includes
the network pointer to invalidate TT and correction/history state on reload;
search admission refreshes roots for each private SMP worker.

## Validation

- `go test -p 1 -short -timeout 10m ./...` and
  `go test -p 1 -short -race -timeout 10m ./engine .` pass on WSL.
- 6,313 random legal plies, inverse transitions back to root, and null moves
  match full refresh exactly. Explicit captures, both castling directions,
  en passant, and all quiet/capture promotions cover H=16/256/2048.
- Independent int64 mailbox inference, color reflection, full-range weights,
  negative truncation, saturation, invalid headers/lengths/CRC all pass.
- Exact integer parity passes for all 64 trainer FENs. Fixtures were copied
  unchanged from `origin/own/trainer` at `dd132c6`; network SHA256:
  `9bb171cd06035f7b77089fbd31b215812057a4b44950190fd741d133dd6f52c6`.
- UCI `EvalFile` + `UseNNUE=true` reaches depth 10 at Threads=1 and 4;
  the short race variant reaches depth 3. Workers share weights and keep
  distinct stacks, all restored to root. Fail-closed reload, stop, position,
  new game, Threads, model identity, and active width reloads all pass.

Fresh-process HCE `go depth 8`, Hash=16 MB, OwnBook=false, Move Overhead=0
retains the exact pre-change nodes and best moves:

| Position | Before/after nodes | Best move |
| --- | ---: | --- |
| Start | 6,500 | b1c3 |
| Kiwipete | 52,782 | e2a6 |
| Rook endgame | 24,833 | b4f4 |

## Performance

Go 1.25.5, `GOAMD64=v3`, Ryzen 7 9800X3D, one search worker,
same FENs/options as above, `go movetime 10000`. The probe counts final debug
stats (including the unfinished iteration) divided by measured search wall
time. The NGN random network is H=256, seed 19. Final runs pin to CPU 14.

| Position | Original HCE | Reference HCE | Reference NNUE | Final HCE | Final NNUE |
| --- | ---: | ---: | ---: | ---: | ---: |
| Start | 1,185,657 | 1,163,223 | 676,334 | 646,789 | 494,633 |
| Kiwipete | 1,031,565 | 1,016,274 | 585,874 | 568,157 | 482,968 |
| Rook endgame | 1,741,150 | 1,635,427 | 736,389 | 958,009 | 594,434 |

All entries are wall-clock NPS. Reference binary: `fab5521`; final inference
binary: `523d9ec` (subsequent changes add test coverage only). Final elapsed
times are 10.0004..10.0031 s. Shared NGN data generation started between phases:
load average reached 15.59 with all 16 logical CPUs 89..100% busy. Final child
CPU times were 9.82..9.97 s, so affinity did not reserve a core; SMT contention
and shared resources still depress speed. The table is a loaded-host throughput
measurement, not an uncontended before/after optimization estimate. Random
weights also change the search tree and provide no playing-strength evidence.

Pure-Go optimization fuses removed/added rows into one child traversal after
the parent copy. Fixed array views eliminate per-neuron bounds checks, with
full-row H=256 paths, 16-neuron blocks for other widths, and min/max activation
clamps. No assembly. Kernels remain isolated for later architecture-specific
implementations. Paired CPU-14 benchmarks under shared load, median of three
2-second samples, both with zero bytes and zero allocations per operation:

| H=256 kernel | Reference ns/op | Final ns/op |
| --- | ---: | ---: |
| Quiet push | 642.9 | 438.0 |
| Output | 649.8 | 556.6 |

Receipts and binaries stay under `/home/ehrli/ngn-data/netinfer/`; no large
outputs are committed. Reproduce the final NPS on WSL from this branch:

```sh
export PATH=/usr/local/go/bin:$PATH GOAMD64=v3 GOMAXPROCS=4
go build -p 1 -o /home/ehrli/ngn-data/netinfer/ngn-final .
NGN_NETINFER_NET_PATH=/home/ehrli/ngn-data/netinfer/random_h256.nnue \
  go test ./engine -run '^TestNGNN1WriteProbeNetwork$' -count=1
taskset -c 14 python3 scripts/netinfer-probe.py /home/ehrli/ngn-data/netinfer/ngn-final --seconds 10
taskset -c 14 python3 scripts/netinfer-probe.py /home/ehrli/ngn-data/netinfer/ngn-final --seconds 10 --network /home/ehrli/ngn-data/netinfer/random_h256.nnue
```
