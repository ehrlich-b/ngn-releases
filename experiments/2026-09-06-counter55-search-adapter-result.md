# Counter 5.5 search-adapter result

The exact Counter 5.5 evaluator is now connected to NGN's existing evaluator-selection, worker, search, and UCI boundaries. This is a compatibility and operational result. It does not establish playing strength, release readiness, or deployment.

## Accepted implementation

The isolated accepted source is commit `49eaf624e772d1a310e9a88c4d1aced574eeb74a`, tree `2c441cc79fd525f820c245d5daa46d77584dfc63`. It was cherry-picked into this branch as `0ffbc52cfffe7b913ab075c23fa74e17822f1e34`. Comparison against the accepted tree found only the two documentation changes already present on this branch; Go sources, module inputs, and Counter fixtures were byte-identical.

The adapter provides:

- strict Counter 5.5 model ownership and identity;
- exact product-rounding and historical score adaptation;
- transactional `EvalFile` and `EvalBackend` changes;
- private primary/helper incremental contexts;
- dynamic context growth with pre-publication numerical validation;
- ordinary, special-move, null, pop, singular-research, emergency, and root-reset handling through the existing worker boundary;
- explicit HCE, NGN-v1, and Counter tagged dispatch without search-policy changes.

The public 128-frame compatibility context remains unchanged. Search uses the separate dynamically growing context. Full refresh remains a reference path and is not the hot incremental implementation.

## Preserved findings and corrections

Three failed integration attempts were retained rather than overwritten:

1. The Counter full-refresh oracle branch used `err` without declaring it in that switch scope. The correction evaluates into local `fresh, err`, preserves the error path, then publishes `raw = fresh` only after success.
2. The singular-verification test seeded a TT evaluation of 100 while its deterministic nonzero Counter root evaluated to 1719. That let the verification return on static evaluation before a transition. The fixture now seeds `rootScore + 256`, keeping the intended verification transition reachable while retaining verification, repush, full-refresh parity, and root-restoration assertions.
3. A transition test copied `Position`, including its `sync.RWMutex`, and failed `go vet`. It now uses the existing lock-safe snapshot helper, which checks board, tag, en-passant state, hash, halfmove clock, and repetition counts.

These changes fixed local compilation or test evidence. They did not alter Counter arithmetic or search policy.

## Final execution evidence

The immutable final run is:

`/home/ehrli/repos/ngn-counter-search-integration/output/counter55-search-integration-gates-v4-attempt4`

All seven supervised stages exited 0 with empty stderr and no surviving processes:

- portable non-fused rounding witness;
- tagged Counter prerequisite and dynamic-context tests;
- full short `engine` and `nnue` regression suites;
- pinned transition, score-adapter, and UCI legal-search oracles;
- targeted race tests;
- `go vet`;
- AMD64-v3 binary build.

The final artifact manifest SHA-256 is `2a55537d759e6b9261200c469dd94faffbbdea5f12b3b4648e5829356620629b`. The built candidate SHA-256 is `c90c12ba3e2ff419978c27aff1d48769f74b48740d91d39f76dda1f073f9f0b6`.

Root acceptance:

- `output/counter55-search-integration-root-review-20260906/terminal-review-v1.json`, SHA-256 `c0138febe7ea87acb535fabfcaa9f99f60759ffadb5c165a14089d2111c89ee0`;
- `output/counter55-search-integration-root-review-20260906/build-provenance-review-v1.json`, SHA-256 `624250fb87c9bed52260cc403cbd615d3592fd3e2017a8716ed8e5d921ce0533`.

The provenance review binds the stage-seven binary to the immutable source snapshot, Go 1.25.5, `GOAMD64=v3`, `CGO_ENABLED=0`, `GOMAXPROCS=2`, `-p=2`, `-trimpath`, and `-buildvcs=false`.

## Remaining decision

The next gate is the already predeclared four-game same-binary HCE-versus-Counter operational screen at one thread. It can justify a larger strength test or stop promotion work; four games cannot estimate Elo. The deployed engine remains `53e4d1b` until the complete release criteria and rollback-controlled deployment are accepted.
