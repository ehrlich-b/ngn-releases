# External Polyglot special-move contract — 2026-10-01

## Question and result

Does the reviewed legal-en-passant repetition repair preserve the **actual
external Polyglot numeric book-key contract** through every frozen forward and
inverse special-move transition?

**Two distinct results are proved:**

1. **The repair preserves NGN's existing `PolyglotHash` behavior exactly.** The
   candidate at `7ce0b410a6e7919ff85eff7c38b84bd9f9d82d6e` and the detached,
   unrepaired proof baseline at
   `8b41845aae29757c031e889f88de55a7064bc8b6` produced the same key at all
   302 forward/inverse observations. Their labeled actual-key sequence has the
   identical SHA-256
   `3f89792dd707913f365e2f0554b862c816469466d20cf6df47452097c52b14f7`.
   Thus the legal-EP repetition metadata change did not alter the book-key
   lifecycle, including illegal-but-adjacent raw EP targets.
2. **Neither revision implements the external Polyglot numeric contract.** All
   302 comparisons against independent standard keys fail identically. The
   smallest control is decisive: NGN returns `4b9aa9e5e768fe35` for startpos,
   while the standard key is `463b96181691fc9c`. The implementation fills its
   781-key table using a xorshift PRNG rather than the canonical fixed Polyglot
   array. This pre-existing limitation was already described by the skipped
   `TestPolyglotHash`; this task extends the measurement across the frozen
   special-move lifecycle rather than discovering a repair regression.

Accordingly, the reviewed repair preserves the **legacy NGN key scheme**, but
an external `.bin` interoperability claim remains false. No production behavior
was changed in this task.

## Immutable oracle

The test fixture
[`engine/testdata/polyglot_special_move_oracle.json`](../engine/testdata/polyglot_special_move_oracle.json)
is byte-for-byte identical to the root-frozen
`../polyglot-independent-oracle.json`:

- SHA-256: `3e7acd53328562ab5fb6943441d586f8bb3723e269c1904c4f128ecee63d05fc`
- python-chess 1.11.2 core SHA-256:
  `1fde6a8e932508d14b31f6a584497b49c2969fc3c7cdc675347f819f0d667a5b`
- `polyglot.py` SHA-256:
  `8dc20733bdc1297a9e8993a76737ba4f328a99185ae94b311ddbb5342fce32fc`
- reused Stockfish legal/perft oracle SHA-256:
  `897f7ebd0c3adc602839d7782673ddbecdd7c1bc72c58dc73fb424f8884b283e`
- 29 targeted histories, 99 forward states, and 10,836 aggregate frozen
  depth-2 nodes; no random campaign.

The histories include all four proved nine-ply illegal-EP repetitions, both
legal-EP controls, four valid null-plus-quiet paths, both-side castling, rook
capture/rights loss, and quiet/capture promotions to queen, rook, bishop, and
knight for both colors. Provenance and immutability rules are retained in
[`polyglot_special_move_oracle.provenance.md`](../engine/testdata/polyglot_special_move_oracle.provenance.md).
The rejected invalid bare-rook castling setup predates the frozen JSON, remains
outside it, and is not treated as an engine result.

## Focused regression and lifecycle coverage

[`TestExternalPolyglotSpecialMoveContract`](../engine/polyglot_external_contract_test.go)
is table-driven directly from the frozen JSON. It is an explicit diagnostic,
enabled with `NGN_POLYGLOT_EXTERNAL_CONTRACT=1`, because the current production
contract is known to fail. Without the environment gate it still reads and
verifies the fixture SHA, then skips so the normal suite stays green.

For every history it verifies:

- exact external numeric book key, independently sorted legal root moves, and
  depth-2 node count at each of the 99 frozen forward states;
- search `MakeMove`/`UnMakeMove` at every real transition;
- `GameMakeMove`/`GameUnMakeMove`, occurrence-map updates, and complete inverse
  traversal for every played history;
- `MakeNullMove`, a real quiet search child, and reverse restoration for all
  four null paths;
- copied positions with demonstrably independent occurrence maps;
- raw board, tag, EP square, cached ordinary key, halfmove clock, full FEN, and
  occurrence map restoration **before** any `Hash` call or cache refresh;
- incremental, cold-copy, and FEN-reconstructed ordinary-key agreement without
  asserting any external numeric value for the ordinary/repetition key;
- `RunPerftTest(pos, 2)` restoration before subsequent hashing.

The focused summary is emitted only after all lifecycle, legal-move, perft, map,
and restoration assertions finish. Both revisions reached the same summary:

```text
histories=29 frozen_states=99 frozen_perft2_nodes=10836
book_checks=302 mismatches=302
actual_sequence_sha256=3f89792dd707913f365e2f0554b862c816469466d20cf6df47452097c52b14f7
```

The 302 observations include the standard start control plus frozen forward,
search-child/inverse, played forward, and full inverse stages. The focused test
exited 1 on both revisions solely at its final external-contract assertion;
any lifecycle or oracle-control failure would have terminated before the
summary. The machine comparison receipt is
[`compare-focus.stdout`](2026-10-01-ep-polyglot-contract/evidence/compare-focus.stdout).

Only the identical test file and immutable JSON fixture were copied to the
detached baseline. Its production source and `.git` were not changed, and its
known-failing broader EP suites were not run.

## Commands and gates

Focused candidate and baseline command:

```text
env NGN_POLYGLOT_EXTERNAL_CONTRACT=1 GOTOOLCHAIN=local GOMAXPROCS=1 GOFLAGS=-p=1 GOCACHE=/home/ehrli/ngn-personal-correctness-20261001/.gocache-ep-repair /usr/local/go/bin/go test ./engine -run '^TestExternalPolyglotSpecialMoveContract$' -count=1 -v
```

Both expected diagnostic runs exited 1 with 302/302 external mismatches and the
identical actual sequence digest above.

Candidate non-regression gates, with the diagnostic normally skipped after its
fixture-integrity check:

- `go test -short ./engine -count=1` — PASS, exit 0
- `go test -short -race ./engine -count=1` — PASS, exit 0
- `go test -short ./... -count=1` — PASS, exit 0

No shared state, concurrency, UCI, harness, or production path changed, so the
conditional all-package race suite was not required. Exact commands, output,
stderr, exits, timestamps, source inspection, identity hashes, and comparison
receipts are under
[`experiments/2026-10-01-ep-polyglot-contract/evidence/`](2026-10-01-ep-polyglot-contract/evidence/).

## Resources, scope, and next gate

The service retained affinity `{0,2}`, `cpu.max=50000 100000`, nice 10,
`memory.max=4294967296`, `GOMAXPROCS=1`, `GOFLAGS=-p=1`, and Go 1.25.5 with
`GOTOOLCHAIN=local`. No game, search campaign, performance/strength claim,
download, owner checkout/process, adapter, launcher, match, daemon, security,
resource, default-branch, or publication action occurred.

The smallest next correctness task is to replace the generated 781-value table
with the canonical fixed Polyglot random array while leaving ordinary/FIDE
hashing and the pseudo-legal adjacency EP book rule separate. The required gate
is this exact 99-state diagnostic passing on all forward/inverse observations,
the standard start key matching, and the normal short/race/all suites remaining
green. Only after that should the diagnostic environment gate be removed and a
small real external `.bin` lookup fixture be considered. This task does not
authorize or implement that production repair.

## Independent operator review

The controller verified identical test/fixture bytes on the candidate and
detached baseline, then reran the explicit diagnostic with race instrumentation
on both revisions under the coordinated CPU0/2, 50%, nice10, 4GiB and serial Go
caps. Both again reached all lifecycle assertions and the same 302-observation
digest before failing only the final external numeric contract assertion. No
data race was reported. These are expected failing witnesses, not green external
Polyglot results; normal suite success does not certify external book support.

The original native report and manifest are preserved. No test or production
source was changed during operator review. The authorized controller retained
the verified diagnostic on its isolated task branch; the external completion
receipt names its local commit and bundle. Original owner/default branches and
all live matches remain untouched, with no publication or new strength claim.
