# Standalone neural chess evaluator result

## Prompt recorded verbatim

The task prompt supplied in `TASK.txt` was:

> Implement an original, small neural chess-position evaluator in Go, starting from this empty repository. Choose and explain your own bounded architecture and arithmetic. It must be genuinely implemented here: do not read, import, copy, translate, or consult any existing chess engine, evaluator, network, repository, personal memory, or internet source. Use only the Go standard library and files you create in this directory. Do not read parent or sibling directories. Do not delegate.
>
> Provide a typed board representation, deterministic model initialization, validation, a scalar evaluation implementation, and a trainable model using supplied position/score labels. A deterministic tiny synthetic dataset is sufficient to prove that training changes loss; do not download data, run an engine, invent strength claims, or perform a production training campaign. Keep the network small and CPU work serial. If incremental evaluation is straightforward, implement it and prove exact agreement with full evaluation through changes and restoration; otherwise retain a clear full-refresh reference and explicitly report that limitation. Input and model validation must reject malformed data without partially changing live state. Scores must have a stated perspective and scale. Test the mathematical invariants and real failure boundaries, including color/perspective symmetry, finite arithmetic and serialization integrity if a format is included. Avoid an elaborate framework or speculative search features.
>
> This is a standalone library, not integration with another program. You have 25 minutes. Record the precise prompt, design decisions, tests, limitations, and files produced in a short local result. Use a fresh module with no external dependencies. Run the relevant tests with GOMAXPROCS=1 and GOFLAGS=-p=1. CPU affinity is 0,2; the enclosing task shares a total 50% CPU cap with its controller and uses nice 10 and at most 4 GiB. Leave unrelated processes and system settings alone. No network, credentials, paid APIs, purchases, publication or release. Commit the tested implementation locally with one single-sentence commit and no coauthor line. Finish with the exact commit and test results.

## Design decisions

- The module is `standalone/neuralchess` and imports only the Go standard library.
- `Board` stores 64 typed `Piece` values using `a1=0` through `h8=63`, plus typed `Color` side-to-move. Validation requires valid codes, one king per color, at most eight pawns per color, no back-rank pawns, non-adjacent kings, and a valid side; it deliberately does not claim full chess legality or reachability.
- The feature vector has ten signed, color-difference features: pawn, knight, bishop, rook, and queen counts; material; centrality; advancement; non-king occupancy; and side to move. Exact `int64` feature totals are divided by fixed scales before arithmetic.
- The network is a bias-free `10 -> 8 -> 8 -> 1` tanh MLP with 152 `float64` weights. No biases and odd `tanh` activations make color reversal (180-degree rotation, color exchange, and side exchange) negate the score. A local deterministic split-mix-style integer sequence initializes weights at scale `0.08`; model validation requires finite weights with absolute value at most `32`.
- `Evaluate` is a fresh full 64-square scan. `State` additionally caches the exact integer totals; square and side changes are transactional delta updates, and tests compare cached evaluation bit-for-bit with full evaluation before, after changes, and after restoration.
- Scores and labels are normalized `[-1,1]` values from White's perspective. Positive is White-favorable in the model's learned target convention; the scale is not centipawns and makes no playing-strength claim. `EvaluateFor` negates for Black.
- Training is deterministic, serial online SGD with mean squared error, configurable bounded epochs/rate/L2, and a one-million-update guard. It validates every input first, trains a private candidate, and commits only after all updates and finite/bound checks succeed.
- The optional binary format is fixed-size little-endian, versioned, dimension-tagged, and weight-only. Decoding validates a private candidate before replacing the receiver.

## Tests and verification

`evaluator_test.go` covers:

- typed square construction and malformed board boundaries;
- deterministic initialization, finite/bounded output, White/Black perspective symmetry, and exact feature negation under color reversal;
- full versus incremental evaluation at initialization, after a square change, after a side change, and after exact restoration;
- failed incremental changes and deliberately corrupted cached totals without state mutation;
- deterministic synthetic labels showing SGD reduces MSE and changes serialized weights;
- NaN labels, malformed positions, invalid configuration, and infinite model weights with atomic rejection;
- serialization round-trip byte/evaluation integrity, bad magic, truncated data, NaN weights, out-of-bound weights, and receiver immutability.

Commands were run from this directory with the Go toolchain at `/usr/local/go/bin/go`, `nice -n 10`, CPU affinity `0,2`, `GOMAXPROCS=1`, `GOFLAGS=-p=1`, `GOPROXY=off`, and tool caches under `/tmp`:

```text
$ /usr/local/go/bin/gofmt -w *.go
$ nice -n 10 taskset -c 0,2 env GOCACHE=/tmp/neuralchess-gocache GOPATH=/tmp/neuralchess-gopath GOMODCACHE=/tmp/neuralchess-gomodcache GOPROXY=off GOMAXPROCS=1 GOFLAGS=-p=1 /usr/local/go/bin/go test -count=1 ./...
ok  	standalone/neuralchess	0.057s
$ nice -n 10 taskset -c 0,2 env GOCACHE=/tmp/neuralchess-gocache GOPATH=/tmp/neuralchess-gopath GOMODCACHE=/tmp/neuralchess-gomodcache GOPROXY=off GOMAXPROCS=1 GOFLAGS=-p=1 /usr/local/go/bin/go test -race -count=1 ./...
ok  	standalone/neuralchess	1.044s
$ nice -n 10 taskset -c 0,2 env GOCACHE=/tmp/neuralchess-gocache GOPATH=/tmp/neuralchess-gopath GOMODCACHE=/tmp/neuralchess-gomodcache GOPROXY=off GOMAXPROCS=1 GOFLAGS=-p=1 /usr/local/go/bin/go vet ./...
(no output; exit 0)
$ /usr/local/go/bin/gofmt -d *.go
(no output; exit 0)
```

## Limitations and files

This is an evaluator library, not a chess engine: it has no move generator, search, castling/en-passant/history state, check legality, opening/endgame knowledge, external data, or strength evaluation. The feature cache's public state operations perform a full consistency check while mutating, so it prioritizes proof and safety over measuring a speedup; ordinary full-refresh evaluation remains the reference path. Training is intentionally tiny and CPU-serial.

Files in the local result are `go.mod`, `board.go`, `model.go`, `serialize.go`, `train.go`, `evaluator_test.go`, `RESULT.md`, and the supplied `TASK.txt`.

## Commit status

The required local commit was attempted after staging the complete result, but the task sandbox exposes `.git` as read-only: Git failed with `fatal: Unable to create '.../evaluator/.git/index.lock': Read-only file system`. No commit hash is claimed because no commit was created.
