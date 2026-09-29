# Deterministic search snapshots

`searchsnapshot` records the one-thread production iterative-deepening path at fixed maximum depths. Each fixture runs in a fresh child process. Its first search therefore starts with cold TT, evaluation caches, pawn cache, and history tables; its second search retains exactly the package state that production leaves warm.

The artifact includes the asserted engine source identity, Go build settings, complete evaluation-model digest, search options, reconstructed played-position map, predecessor move, complete hidden-board digest, final move/score/node/PV data, and the same deterministic data plus all uint64 diagnostics at every completed-depth callback. It does not record elapsed time or NPS. The comparator reports source/toolchain metadata changes separately and fails on every recorded option, root-history, callback, or final-result difference.

Build and run on WSL from the repository root:

```sh
GOMAXPROCS=1 GOAMD64=v3 go build -p=1 -o output/searchsnapshot/searchsnapshot ./cmd/searchsnapshot
GOMAXPROCS=1 output/searchsnapshot/searchsnapshot record \
  -manifest cmd/searchsnapshot/testdata/accepted-v2.json \
  -source 53e4d1b9bf4006a39c0327a7016faba7f2b8f82e \
  -out output/searchsnapshot/accepted-a.json
GOMAXPROCS=1 output/searchsnapshot/searchsnapshot compare \
  -baseline output/searchsnapshot/accepted-a.json \
  -candidate output/searchsnapshot/accepted-b.json
```

The required `-source` value is an asserted production identity and is stored alongside the executable's embedded VCS settings. This lets a recorder added on a later documentation/harness commit identify an unchanged accepted engine tree without hiding the harness revision.
