# Legal-EP and standard-Polyglot integration review — 2026-10-01

## Question and result

**Question.** Do the reviewed legal-en-passant repetition repair and canonical
standard-Polyglot table/castling decoder coexist while preserving legal move,
repetition, restoration and released owned-NNUE behavior?

**Result: yes, for the frozen integration scope.** The exact reviewed
production components coexist without a production edit in this task. The only
baseline failures were twelve assertion events caused by six EP fixtures still
pinning the old generated-table numbers in two test paths. Replacing only the
twelve stored EP/no-EP expected values with independently frozen standard
Polyglot values makes the combined focused proof and all four required
actual-network short/race suites pass.

This is a correctness/integration result, not owner/default integration,
playing-strength evidence or a compatibility conversion for historical
wrong-key private books.

## Exact source composition and scope

- Review branch: `task/ep-polyglot-integration-review-20261001`
- EP/NNUE base at native admission:
  `28d66322debef23d01645a1bc73d7e942ada9813`
- Reviewed standard-book component:
  `780c89dfb3177c115e682a8c1524ee8101ccaaec`
- First native probe: `2026-10-01T06:08:20Z`
- Materialization receipt:
  `evidence/integration-materialization.json`, SHA-256
  `ff40a812e8fa34f557d22d7258e54b3f8921efc24dc49bb88cd00396d7d1bf92`

The materialized book files were compared byte-for-byte with their Git objects
at `780c89d`; all eight match the pre-admission receipt:

| File | SHA-256 |
|---|---|
| `engine/polyglot.go` | `0d4f8b258ad0b214921d8fde1f555ebd429aaeffe2a19ac7bc13d1774c762037` |
| `engine/polyglot_random.go` | `9faa80dc15ff730cddf3580b87054703d7ce587d92d15a4277362826b1366477` |
| `LICENSES/chess-library-MIT.txt` | `5860e7607afbf4c7e91bad5549b71d16fc4eceb90f0c671cd77a343ae7461a2a` |
| `engine/book_test.go` | `421cb328309f232d1ade68ff407c51b506df783bd6a7216ad90311230eb70290` |
| `engine/polyglot_external_contract_test.go` | `b3d9a088e1bd81af8986271094b0e923d5ca6817aa6ecdfd7e3f6726ba6c8cd0` |
| `engine/polyglot_book_move_contract_test.go` | `0147e6db9c4a7cbde7d0872bc08468ad09164049b72ccda0244e9247715dfb98` |
| book-move oracle JSON | `8289efed4fd50a58a6bff4a816c9b0ecdc7cea7d7d7acc7dcf0a17d26de993ba` |
| book-move provenance | `fc89c17321586d95e0dd4c050895091502bd8768c3f33e56013e66ad78d24b6a` |

The EP production mechanism remains byte-identical to `28d66322`:

- `engine/hash.go`:
  `a0f41a097f46295708b703fbcd8deba69389aba97dbbd9d3429db92a82576419`
- `engine/position.go`:
  `123da9954e7192103883155ad4943e99742fe115156be31f13639014b77b4d65`

The complete tracked production difference from the EP base consists only of
the already materialized reviewed book component. All other production bytes
remain unchanged. This task itself changes only
`engine/enpassant_repetition_correctness_test.go`: twelve numeric expectations
plus its two obsolete legacy-key descriptions.

## Independently frozen twelve-value transition

The immutable transition oracle is retained byte-for-byte at
`evidence/integration-independent-key-transition.json`, SHA-256
`172796acc6b60cc450fe5614734d03c5068d3b5fa4be208e51dc4a44d284623d`.
It identifies python-chess 1.11.2 core SHA-256
`1fde6a8e932508d14b31f6a584497b49c2969fc3c7cdc675347f819f0d667a5b`
and Polyglot module SHA-256
`8dc20733bdc1297a9e8993a76737ba4f328a99185ae94b311ddbb5342fce32fc`.
No replacement value came from NGN.

The original EP fixture source is retained unchanged at
`evidence/enpassant_repetition_correctness_test.legacy.go.txt`, SHA-256
`3e9b1cb951a6640e36c86e2ebb1e7ba42914a2e1f9254227c4ff152922347c98`.
The active source after the transition has SHA-256
`a5a0a6716c04a6d4a3a207f05a91d2ac28f401b40c3c8ad6e331fec99848bf76`.

`verify_key_transitions.py` constructs the entire permitted active source from
the preserved source using exactly:

1. the twelve oracle mappings (`wantPolyglotEP` and `wantPolyglotNoEP` for six
   unchanged fixtures); and
2. replacement of the two descriptions which incorrectly called those values
   NGN proof-baseline keys.

It then requires byte equality with the active source and emits all twelve
old/new/FEN mappings in `evidence/key-transition.json`. This proves that no
history, FEN, legal set, perft count, draw expectation, restoration assertion,
ordinary key expectation or fixture structure changed.

## Failing-before isolation

The first real focused run used the exact released owned network and the
unmodified legacy expectations. It exited 1 with exactly twelve obsolete
numeric assertion events:

- six at `TestLegalEnPassantOrdinaryHashAndPinnedOracle`, and
- the same six during the owned-NNUE played-history path.

Each failure's **actual** value was the independently frozen standard EP key;
each wanted value was its legacy generated-table key. No other failure message
occurred. All six fixture subtests reached the Polyglot assertion only after
their pinned legal-move and perft checks passed.

In that same failing run:

- four legal nine-ply histories reached true threefold;
- make/unmake, game undo, null, cold/incremental and occurrence-map restoration
  passed;
- predicate purity, second-capturer, checking-pawn and diagonal controls passed;
- owned-NNUE null/quiet and all four finite-search caller restorations passed;
- the external 99-state contract passed all 302 numeric observations;
- all 23 literal standard book moves and 7,651 nodes passed;
- all four existing king-to-g/c destination aliases passed; and
- embedded-book and external start-entry controls passed.

A separate pre-edit command excluding the numeric fixture consumers also
exited 0, preserving an explicit independent receipt for the repetition,
restoration and predicate controls. Exact receipts are
`failing-before-combined.*` and `before-nonnumeric-ep-controls.*`.

One setup-only attempt used the relative spelling `GOCACHE=../.gocache-ep-repair`;
Go 1.25.5 rejected it before build because Go requires an absolute cache path.
That receipt is preserved as `setup-relative-gocache.*`; all real commands used
the absolute path to that exact existing cache. It is not an engine result.

## Passing combined focused proof

The passing focused command was:

```text
env NGN_EP_OWNED_NNUE=/home/ehrli/nnue-owned-k4-20260920/night-20260927/runs/wdl25-e10.nnue GOTOOLCHAIN=local GOMAXPROCS=1 GOFLAGS=-p=1 GOCACHE=/home/ehrli/ngn-personal-correctness-20261001/.gocache-ep-repair /usr/local/go/bin/go test ./engine -run '^(TestEnPassantHashCapturabilityGate|TestEnPassantSecondCapturerCanBeLegal|TestEnPassantNullDescendantRestoresKey|TestLegalEnPassantOrdinaryHashAndPinnedOracle|TestIllegalEnPassantPlayedThreefold|TestEnPassantKeyTransitionRestoration|TestLegalEnPassantPredicateEdgesAndPurity|TestOwnedK4EnPassantRepetitionReadiness|TestCanonicalPolyglotRandomTable|TestExternalPolyglotSpecialMoveContract|TestExternalPolyglotBookMoveContract|TestPolyglotOrthodoxCastleDestinationCompatibility|TestPolyglotHash|TestExternalPolyglotBookInterop|TestEmbeddedBook.*)$' -count=1 -v
```

Result: exit 0, with no skip.

The proof covers:

- all four illegal-EP nine-ply histories with occurrence count 3 and FIDE draw;
- both-color legal-EP ordinary-key distinction;
- standard Polyglot pseudo-legal adjacency identity for all six EP/no-EP pairs;
- raw, null, incremental, FEN, copy, search/game inverse and map restoration;
- EP king-safety purity and two-capturer edge cases;
- canonical table digest and standard start key;
- 99 states, 302 external key observations, 10,836 aggregate depth-2 nodes and
  zero mismatches, sequence digest
  `010379ec6836a7e23f506cf741aca9b5e0b5252ce125a96b63207db22aeba07e`;
- 23 literal standard moves, all exact NGN `Move` values, 7,651 aggregate
  before/after depth-2 nodes, exact caller/copy/inverse restoration and four
  legacy king-destination aliases; and
- all embedded-book controls.

The reviewed literal e2e4 xFEN projection remains unchanged in the exact book
component and continues to pass.

## Released owned-NNUE proof

- Model:
  `/home/ehrli/nnue-owned-k4-20260920/night-20260927/runs/wdl25-e10.nnue`
- SHA-256:
  `1ec8fc1737ddfdd5f8b6ff0b4e26778085fb563f7c5e82c1fc5d3ea454b6ec29`
- Backend: `ngn-k4-768-v1`
- K4 evaluation scale: 60

`TestOwnedK4EnPassantRepetitionReadiness` ran non-skipped and passed all 16
existing leaf cases: six played/inverse histories, six null-plus-quiet inverse
paths, and four depth-2 finite-search caller-restoration cases. The test checks
the file and loaded model identities, scale/backend, incremental-versus-fresh
raw evaluation, full-position parity, evaluator depth/lifecycle, raw
EP/tag/key/FEN/map restoration and caller state. No alternate model was used.

## Required actual-network checks

Every command set the exact `NGN_EP_OWNED_NNUE` path above, `GOTOOLCHAIN=local`,
`GOMAXPROCS=1`, `GOFLAGS=-p=1`, and the absolute form of the existing private
cache path.

| Check | Result |
|---|---|
| `go test -short ./engine -count=1` | PASS, engine 12.902s |
| `go test -short -race ./engine -count=1` | PASS, engine 103.195s |
| `go test -short ./... -count=1` | PASS, engine 13.885s |
| `go test -short -race ./... -count=1` | PASS, engine 101.683s |

Exact commands, environment, stdout/stderr, exit codes and UTC timestamps are
retained in the evidence directory. The machine verifier requires all four
commands to contain the exact network path and all four exits to be zero.

## Resources and disposition

Actual controls were affinity `{0,2}`, nice 10, `cpu.max=50000 100000`,
`memory.max=4294967296`, `GOMAXPROCS=1`, `GOFLAGS=-p=1`, and cgroup
`ngn-personal-integration-review-20261001.service`. All chess and Go execution
was on WSL. No game, benchmark, training, asset write, owner checkout, adapter,
match process, daemon/security setting, external publication or shared/default
branch was touched.

At native completion the verified proposal remained uncommitted because
native `.git` is read-only. Independent controller review and the local task-branch
commit are recorded below. Owner/default integration remains a separate
decision. Historical private books keyed with the former generated table are
unchanged and are not claimed to have become standard-compatible.


## Independent operator review and disposition

The controller verified every copied component against committed book component
780c89df and every other production Go file against EP/NNUE base28d66322. The
EP regression is exactly the original source plus twelve externally frozen
numeric expectations, its obsolete comment and one failure-label update. The
Stockfish legal/perft witness, both literal Polyglot fixtures, original NNUE
integration test and independent input map remain byte-identical.

All four required commands were verified with actual-net environment enabled,
zero exits and empty stderr. The exact owned model hash and source gate ensure
that these runs cannot silently skip the NNUE integration case. An independent
focused short-race replay then passed all16 owned NNUE cases, four true
threefold histories, both legal-EP controls and restoration/purity cases,
302 canonical book-key observations,23 exact book moves,7651 book-move perft2
nodes and all four legacy orthodox castle-destination controls.

The native report and original manifest are retained as deterministic gzip with
raw hashes before this disposition. Native verify_integration.py is deliberately
locked to the pre-commit worktree and live service controls; its captured output
is a stage witness. The final SHA256SUMS and operator-review.json bind the
reviewed proposal after commit. The independent command/environment/output
and cgroup receipts are retained in operator-* evidence, together with the
original independent twelve-key input and materialization receipt. No further
production or test mechanism was changed during review.

The authorized controller commits this verified combined proposal only on the
isolated task branch. Original EP and book component branches stay clean at
their previously reviewed commits. The benchmark owner's checkout, adapters,
default branch, matches and released assets remain untouched. This result is
correctness and interoperability evidence; it contains no games, Elo estimate
or strength claim. Owner/default integration and publication remain separate
authority gates.
