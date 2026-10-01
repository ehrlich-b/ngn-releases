# Counter 5.5 portable oracle source

`upstream_oracle_test.go` is a test-only adapter for CounterGo commit `63c487ca724c620f71c129d62129c6fb9109c872`. It is copied into a detached checkout of that exact tree and built only with tag `ngn_counter_oracle`. The upstream project is GPL-3.0; the pinned license text and source hashes are retained in the experiment evidence.

The oracle is not linked into NGN and is not a production evaluator. `fixtures.tsv` is shared with NGN's independent parser/evaluator test. Both sides use `GOAMD64=v1` and no Counter `avx` tag for the canonical portable float32 operation order.

The incremental compatibility slice adds `upstream_transition_oracle_test.go`
and `transition_fixtures.tsv`. Copy the test beside Counter's pinned
`pkg/eval/nnue` sources and run only with build tag
`ngn_counter_transition_oracle`, `COUNTER_MODEL`, `COUNTER_TRANSITIONS`, and
`COUNTER_TRANSITION_ORACLE_OUTPUT`. The consumer runs with NGN build tag
`counteroracle` and the same frozen model, fixture, and generated JSON paths.
This oracle uses Counter's legal move generator and records ordered updates,
all accumulator bits, raw bits, null frames, and restored pop frames.
