# NGNN1 AVX2 kernels (2026-10-05)

Written fresh from the NGNN1 specification and current NGN Go kernels in
`own/avx2`; no third-party evaluator sources, kernels, data, or weights were
consulted. Edits use `/Users/ehrlich/repos/ngn-avx2`; all execution uses the
private `/home/ehrli/wt/own-avx2` WSL worktree. Go 1.25.5, `GOAMD64=v3`.
Tested code revision: `6ded70b57bb28c3cc0b6aad54ea625deeb154d01`.

## Arithmetic and dispatch

Accumulators remain int32: a valid network may produce `65 * -32768 =
-2,129,920` from bias plus 64 occupied squares, so int16 is insufficient.
Quiet moves, captures/promotions/en passant, and castling fuse the parent copy
with sign-extended row additions/subtractions. Null moves still copy the parent.

The loader records whether every output weight is in [-128,128], without
restricting the accepted int16 range. This enables a packed SCReLU path:
clamp int32 first, pack x, multiply x*w in int16, then `VPMADDWD(x, x*w)`.
Each perspective has eight int32 sum lanes. Their absolute bound is
`(2048/8) * 255^2 * 128 = 2,130,739,200 < 2^31`; widen to int64 before
combining perspectives or reducing lanes. Larger weights use int32 squared
products (each bounded by `255^2 * 32768 = 2,130,739,200`), sign-extended
to int64 before any product additions. Both original divisions and score
saturation remain in Go. All supported widths are exact, including H=2048.

`amd64 && amd64.v3 && !purego && !race` selects assembly. Other targets,
`-tags purego`, and race builds use the retained Go kernels; race builds
instrument accumulator accesses. Loads/stores accept ordinary unaligned Go
slices, and every assembly return executes `VZEROUPPER`.

## Verification

All 128 widths (16..2048) pass randomized Go/AVX2 differential tests:
32 cases per width for row operations and 32 sparse random networks per width
for output, both perspectives, all row shapes, aliased/disjoint destinations,
unaligned views, guard writes, full int32 accumulator states, clamp boundaries,
full int16 weights, and maximal same-sign/cancelling output sums.
Full-range bias plus 64-row refresh also passes at H=16/256/512/2048.
The unchanged 64-FEN trainer fixture and existing 6,313 random legal-ply,
undo/null, special-move, oracle, and mirror checks pass exactly.

WSL checks: `go test -p 1 -short -timeout 10m ./...`,
`go test -p 1 -short -race -timeout 10m ./engine`, `go vet -p 1 ./...`,
NGNN1 tests with `-tags purego`, and an arm64 test cross-compile all pass.

## Performance

Ryzen 7 9800X3D, CPU 14 affinity, `GOMAXPROCS=1`, paired builds from the
same revision (ordinary AVX2 versus `-tags purego`). Both builds include the
fused parent copy. Kernel medians use three paired 300-ms samples, alternating
build order; all report zero bytes and allocations per operation.

| Kernel | H | Go ns/op | AVX2 ns/op | Speedup |
| --- | ---: | ---: | ---: | ---: |
| Quiet push, both perspectives | 256 | 270.6 | 43.66 | 6.20x |
| Quiet push, both perspectives | 512 | 725.5 | 72.37 | 10.03x |
| Output, weights within +/-128 | 256 | 463.8 | 24.10 | 19.25x |
| Output, weights within +/-128 | 512 | 1049.0 | 44.46 | 23.59x |
| Move rows, one perspective | 256 | 131.9 | 16.42 | 8.03x |
| Move rows, one perspective | 512 | 332.3 | 32.67 | 10.17x |
| Capture rows, one perspective | 256 | 167.7 | 21.73 | 7.72x |
| Capture rows, one perspective | 512 | 404.6 | 38.81 | 10.43x |
| Castle rows, one perspective | 256 | 228.0 | 25.67 | 8.88x |
| Castle rows, one perspective | 512 | 500.6 | 43.95 | 11.39x |
| Output, full int16 weights | 256 | 456.4 | 50.01 | 9.13x |
| Output, full int16 weights | 512 | 1039.0 | 98.58 | 10.54x |

Fresh-process searches use the NGN random H=256 net, seed 19, Threads=1,
Hash=16 MB, OwnBook=false, Move Overhead=0. The three middlegame FENs live in
`scripts/netinfer-probe.py` (`--middlegames`). Fixed-depth identity and one
paired 10-second search per FEN:

| Position | Depth-10 nodes, identical | Best move, identical | Go wall NPS | AVX2 wall NPS | Speedup |
| --- | ---: | --- | ---: | ---: | ---: |
| Kiwipete | 671,918 | c3b5 | 671,437 | 1,146,319 | 1.71x |
| Italian | 50,825 | c1d2 | 721,672 | 1,358,120 | 1.88x |
| Queen's Gambit | 273,734 | c4d5 | 705,928 | 1,332,350 | 1.89x |

Wall NPS uses final debug node totals, including the unfinished iteration,
divided by measured search time (10.0003..10.0005 s). Search build order is
Go/AVX2, AVX2/Go, Go/AVX2. Shared-host 1-minute load was 8.31..8.87 during
kernels and 8.48..8.58 at search boundaries. Child CPU times were 9.88..10.10 s
(including startup/teardown); affinity does not reserve the core. These are
loaded-host throughput results with random weights, not playing-strength data.

## Reproduction

From the WSL worktree, use `/usr/local/go/bin` on PATH, `GOAMD64=v3`,
`GOMAXPROCS=1`, and `taskset -c 14`. Build engines and engine test binaries
normally and with `-tags purego`; the source revision is identical.

```sh
export PATH=/usr/local/go/bin:$PATH GOAMD64=v3 GOMAXPROCS=1
receipt=/home/ehrli/ngn-data/own-avx2
go build -p 1 -o "$receipt/ngn-avx2" .
go build -p 1 -tags purego -o "$receipt/ngn-go" .
go test -p 1 -c -o "$receipt/engine-avx2.test" ./engine
go test -p 1 -tags purego -c -o "$receipt/engine-go.test" ./engine
NGN_NETINFER_NET_PATH="$receipt/random_h256.nnue" \
  go test -p 1 ./engine -run '^TestNGNN1WriteProbeNetwork$' -count=1
taskset -c 14 "$receipt/engine-avx2.test" -test.run '^$' \
  -test.bench '^BenchmarkNGNN1(Push|Output|Rows|OutputWide)$' -test.benchtime=300ms
taskset -c 14 python3 scripts/netinfer-probe.py "$receipt/ngn-avx2" \
  --network "$receipt/random_h256.nnue" --middlegames --depth 10
taskset -c 14 python3 scripts/netinfer-probe.py "$receipt/ngn-avx2" \
  --network "$receipt/random_h256.nnue" --middlegames --seconds 10
```

Repeat paired with `ngn-go`/`engine-go.test`. Receipts, random NGN weights,
and binaries stay under `/home/ehrli/ngn-data/own-avx2/` (63 MB), including
`bench-summary.json`, `depth-identity.json`, `nps-1.json`, and check logs.
