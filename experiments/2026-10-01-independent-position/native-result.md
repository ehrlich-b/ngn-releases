# Result

Implemented `position.go` and `position_test.go` in package `engine` without
changing the supplied board, primitive, hash, value, or callback files.

Covered behavior includes tags and turns, lazy/incremental/cold hashing,
ordinary/capture/promotion/castling/en-passant make and undo, partial and null
moves, callback-derived check state, raw versus legal en-passant metadata,
castling-right restoration, repetition-map locking and isolation, copying,
and both legacy draw policies, including the corrected bare-kings draw case.

Validation completed with:

```text
GOMAXPROCS=1 GOFLAGS=-p=1 go test ./...
GOMAXPROCS=1 GOFLAGS=-p=1 go test -race ./...
GOMAXPROCS=1 GOFLAGS=-p=1 go vet ./...
```

`gofmt -l position.go position_test.go` reported no files. The legacy
`IsDraw` and `IsFIDEDrawRule` policies remain compatibility helpers rather
than a complete FIDE adjudicator; search behavior was not redesigned.
