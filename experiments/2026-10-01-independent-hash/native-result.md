# Result

Implemented `hash.go` in package `engine` with the specified deterministic key stream, ordinary hash generation, incremental move and null-move updates, lazy en-passant metadata, and legal en-passant probing. Each legality probe uses a new temporary `Position` initialized only with the copied `Board` and passes the original side explicitly to `isInCheck`.

Added bounded tests covering the complete key stream, tag filtering, ordinary, capture, promotion, all castling branches, en-passant deltas and undo, null moves, cold and delegated fallbacks, both adjacent candidates, blocked and opened-line probes, mutation, and probe allocations.

Validation, run with `GOMAXPROCS=1`, `GOFLAGS=-p=1`, `GOPROXY=off`, `GOTOOLCHAIN=local`, and `nice -n 10`:

- `gofmt -d hash.go hash_test.go`: clean
- `go test -count=1 ./...`: passed
- `go test -race -count=1 ./...`: passed
- `go test -gcflags=all=-d=checkptr=2 -count=1 ./...`: passed
- `go vet ./...`: passed

The local contract adapter now passes `Position` by value to `contractCheck`; `isInCheck` retains its pointer ABI and performs that value copy. The en-passant tests therefore validate the hashing probe against the adapter boundary, not an engine check implementation or end-to-end search strength.
