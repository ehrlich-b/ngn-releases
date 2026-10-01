# Result

Review finding: `Rank.Name` was returning `string`; the public contract requires `Name() int`. It now returns `int`, `Square.Name` formats that integer, and an external-package compile-time interface assertion covers the declared signature.

Serial tests:

```text
$ GOMAXPROCS=1 GOFLAGS=-p=1 GOPROXY=off GOCACHE=/tmp/ngn-independent-primitives-go-cache /usr/local/go/bin/go test -count=1 ./...
ok  	standalone/primitives	0.001s
```

Exit status: `0`.

Race tests:

```text
$ GOMAXPROCS=1 GOFLAGS=-p=1 GOPROXY=off GOCACHE=/tmp/ngn-independent-primitives-go-cache /usr/local/go/bin/go test -race -count=1 ./...
ok  	standalone/primitives	1.005s
```

Exit status: `0`.

Vet:

```text
$ GOMAXPROCS=1 GOFLAGS=-p=1 GOPROXY=off GOCACHE=/tmp/ngn-independent-primitives-go-cache /usr/local/go/bin/go vet ./...
```

Exit status: `0`.

Limitations:

- The module contains only the requested piece, square, and packed-move primitives and their focused tests.
- The default Go build-cache location was read-only, so the successful commands used the writable cache path shown above.
