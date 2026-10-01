# Result

## Design

`evaluation_core.go` rebuilds both phase tables analytically from `(file, rank)`
coordinates. The coordinate center term is a doubled Manhattan distance from
the board center; forward distance is rank from each piece's own home side.
Black entries are the negative of the vertically reflected White entries. The
score blends a small middlegame and endgame accumulator according to a phase
of 38.

The hand-chosen material baselines are 100 for a pawn, 315/330 for a knight,
335/350 for a bishop, 510/540 for a rook, and 975/985 for a queen (MG/EG).
These are round experimental centipawn-scale numbers chosen only to make
material dominate the smaller coordinate terms; they are not fitted or copied
from an engine. Phase increments are 2 for each minor, 3 for each rook, and 5
for each queen, giving 38 in the initial material. Pawn terms cover isolated,
doubled, supported, and passed pawns. King terms cover a small forward pawn
shield, while the analytic king square term is defensive in the middlegame and
active in the endgame. Bare kings and the requested single-minor dead
material shapes return zero.

The evaluator rebuilds from the mailbox, does not repair or trust cached
accumulators, uses no allocations, and clamps strictly inside +/-25000.
`isEndgame` is deliberately simple: no queens and at most two rooks. The
sliding functions walk coordinate rays directly, including the first blocker,
without wraparound or lookup/magic tables.

## Verification

All commands were run serially with one Go build worker and a writable
temporary build cache:

```text
gofmt -w evaluation_core.go evaluation_core_test.go slider_attacks.go slider_attacks_test.go
GOMAXPROCS=1 GOFLAGS=-p=1 GOCACHE=/tmp/ngn-hce-gocache go test ./...
ok   standalone/hce  0.003s
GOMAXPROCS=1 GOFLAGS=-p=1 GOCACHE=/tmp/ngn-hce-gocache go test -race ./...
ok   standalone/hce  1.010s
GOMAXPROCS=1 GOFLAGS=-p=1 GOCACHE=/tmp/ngn-hce-gocache go vet ./...
passed (no diagnostics)
gofmt -l evaluation_core.go evaluation_core_test.go slider_attacks.go slider_attacks_test.go
passed (no output)
```

The tests independently oracle every origin and single blocker for rook,
bishop, and queen attacks; exercise deterministic random occupancies; check
allocation freedom and input preservation; and cover evaluation reflection,
bounds, material sign, dead material, deterministic table rebuilds, stale
accumulators, and reversible quiet, capture, promotion, and castling-shaped
board updates.

## Limitations

This is a compact experimental evaluator, not a move legality, checkmate,
draw, or search implementation. It has no claimed playing strength or rating;
the host remains responsible for side-to-move conversion, terminal handling,
and caching.
