# Result

Implemented `cache.go` and focused tests in `cache_test.go` for packing,
construction, clearing, occupancy sampling, replacement rules, age wrapping,
empty-hash behavior, and synchronized pair coherence.

Commands run serially with `GOMAXPROCS=1`, `GOFLAGS=-p=1`, `GOPROXY=off`,
`GOTOOLCHAIN=local`, and a writable temporary `GOCACHE`:

```text
GOMAXPROCS=1 GOFLAGS=-p=1 GOPROXY=off GOCACHE=/tmp/ngn-cache-gocache GOTOOLCHAIN=local gofmt -w cache.go cache_test.go
GOMAXPROCS=1 GOFLAGS=-p=1 GOPROXY=off GOCACHE=/tmp/ngn-cache-gocache GOTOOLCHAIN=local go test ./...
ok   standalone/cache  0.012s
GOMAXPROCS=1 GOFLAGS=-p=1 GOPROXY=off GOCACHE=/tmp/ngn-cache-gocache GOTOOLCHAIN=local go test -race ./...
ok   standalone/cache  1.399s
GOMAXPROCS=1 GOFLAGS=-p=1 GOPROXY=off GOCACHE=/tmp/ngn-cache-gocache GOTOOLCHAIN=local go vet ./...
```

The direct mode and `Clear` retain their documented exclusive-ownership
requirements. Allocation requests are not clamped; callers control admitted
sizes. `OldAge` is exposed as required but is not used because the explicit
replacement rule compares the decoded age with the sampled full age directly.
