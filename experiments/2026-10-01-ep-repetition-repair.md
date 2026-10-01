# Legal en-passant repetition repair candidate

Date: 2026-10-01 UTC
Branch: `task/ep-repetition-correctness-20261001`
Proof parent: `8b41845aae29757c031e889f88de55a7064bc8b6`
Underlying engine baseline: `701328646490f1bc8ed214002f578a58c9b0723e`

## Result

The bounded local candidate repairs the proved repetition defect. An adjacent
en-passant target now distinguishes NGN's ordinary Zobrist/repetition/TT key
only when at least one EP capture is legal. Captures that expose the king or do
not answer another checker no longer split otherwise identical positions.

The raw `Position.EnPassant` state and `canCaptureEnPassant` remain
adjacency-based. `PolyglotHash` is unchanged and produces every exact pinned
proof-baseline value after both a real double push and FEN setup. This candidate
therefore does not conflate the Polyglot book-key contract with FIDE repetition
identity.

The four legal nine-ply histories from the proof now count the repeated
position at plies 1, 5 and 9 under one ordinary key. At ply 9 the game map count
is three and `IsFIDEDrawRule()` is true. Both ordinary legal-EP controls still
distinguish EP from no-EP positions for White and Black.

## Mechanism

`PositionTag` was a `uint8` with seven occupied bits. The candidate uses its
last bit as private reversible metadata, `enPassantHash`, recording whether the
current raw target has at least one legal capture:

- `hasLegalEnPassant` first validates the target rank, empty target and actual
  displaced enemy pawn. For each of at most two adjacent friendly pawns, it
  copies the fixed-size `Bitboard`, applies the EP displacement to that copy and
  directly checks the mover's king. It does not mutate the production position,
  recurse into hashing, generate a move list, or allocate a heap-backed
  collection.
- FEN initialization and the real-move path refresh that bit after raw
  adjacency normalization. Cold/hand-built EP positions initialize it before
  hashing or transition.
- Full ordinary hashing includes the EP file only when the bit is set.
  Incremental make uses the new bit for the post-move target and the saved undo
  `PositionTag` for the pre-move target. This is necessary because `updateHash`
  sees only the post-move board and cannot reconstruct old EP legality.
- Null make temporarily clears raw EP but preserves the bit, allowing null
  unmake to restore the old key without changing its public API. With raw EP
  absent the retained bit contributes nothing to a null-position key. Warm and
  forced-cold null paths are tested for both colors and both legal/illegal EP.
- Ordinary quiet moves, legal EP captures, game moves and their inverse paths
  all use the same explicit old/new eligibility metadata.

No search, evaluation, UCI, harness, match, benchmark or owner code changed.

## FEN and Stockfish boundary

The immutable independent oracle remains the proof artifact:

- Stockfish 18 executable SHA-256:
  `6b087694916228c905a5e14db74cca8c7e5643602226af1fa5d42353c455b9f9`
- receipt:
  `experiments/2026-10-01-ep-repetition-correctness/stockfish-probe.json`
- receipt SHA-256:
  `897f7ebd0c3adc602839d7782673ddbecdd7c1bc72c58dc73fb424f8884b283e`

Stockfish 18 deliberately differs by entry path: its FEN loader keeps a
pseudo-capturable adjacent EP target in its displayed/internal key, whereas its
real `do_move` path performs the king-safety/check test before adding EP to the
repetition key. The legal played histories, not an artificial FEN-key
comparison, proved the defect. NGN continues to retain the raw adjacent target
on FEN load for move generation and Polyglot, while its **ordinary** full FEN
key and incremental real-move key now both use the FIDE legal-EP identity.

## Active regression coverage

`engine/enpassant_repetition_correctness_test.go` pins all six independently
checked fixtures:

1. pinned horizontal exposure, White apparent capturer;
2. mirrored pinned exposure, Black apparent capturer;
3. EP that does not answer a separate discovered check, White apparent
   capturer;
4. its Black mirror;
5. ordinary legal White EP; and
6. ordinary legal Black EP.

For real-push, EP-FEN and no-EP stages it asserts the exact Stockfish legal root
sets and depth-2 perft. It also pins NGN's pre-repair Polyglot EP/no-EP keys:

| fixture | raw-EP Polyglot | no-EP Polyglot |
|---|---:|---:|
| pinned White | `7FF8481A6D4B0727` | `3CB319FC6443FA09` |
| pinned Black | `3A7000D3208A707B` | `C8A28F718D35A016` |
| separate check White | `B38563CD78EE1818` | `F0CE322B71E6E536` |
| separate check Black | `1619109BBAA6507A` | `E4CB9F3917198017` |
| legal White EP | `A13469DD2E6AA978` | `E27F383B27625456` |
| legal Black EP | `203626AE32C695BE` | `D2E4A90C9F7945D3` |

The regression additionally checks:

- full versus incremental ordinary keys at every ply of all four played
  histories;
- the true third-occurrence map count and draw flag;
- search make/unmake and game make/unmake including exact map restoration;
- legal EP captures and ordinary quiet moves that clear a legal target;
- warm and cold null make/unmake;
- both capturing colors; and
- direct predicate edge cases: two adjacent capturers with only one legal,
  capturing the checking pawn, diagonal exposure, and a remaining blocker that
  makes the EP capture legal. The predicate is compared with legal move
  generation and checked for zero state mutation.

The historical success-on-defect program under
`experiments/2026-10-01-ep-repetition-correctness/` was not modified; its
baseline diagnosis remains a record of `7013286`, not a candidate test.

## Failing-before proof

The active core regression was first run with production files still at proof
parent `8b41845a` (engine `7013286`). Command and complete output are retained
as `failing-before.command`, `.stdout`, `.stderr` and `.exit`; exit was `1`.
All four illegal cases reported distinct ordinary EP/no-EP keys, then all four
played histories reported different first/third keys. Examples:

```text
pinned White: first 8E29B3E8B604F0EA, third 20B03C7203AC9AB6
pinned Black: first E00ACB0FEEFCFDA4, third 70032EBF22DB1832
separate check White: first 43F64A5636D4CDDC, third ED6FC5CC837CA780
separate check Black: first 998A289EA9585E34, third 0983CD2E657FBBA2
```

`failing-before-regression.patch.gz` retains a proof-parent-applicable copy of the
active core regression. The checked-in test was subsequently strengthened with
the direct predicate edge cases and child full-key checks; production behavior
was not changed until after the failing run.

## Candidate validation

All commands used `GOMAXPROCS=1`, `GOFLAGS=-p=1` and a task-private Go cache.
`environment.txt` records Go `1.25.5`, Linux/amd64, task cgroup, allowed CPUs
`0,2`, nice `10`, `cpu.max=50000 100000` and `memory.max=4294967296`.

| Gate | Result | Retained output |
|---|---|---|
| focused four-test regression, verbose | PASS, exit 0 | `focused-candidate-v2.*` |
| `go test -short ./engine -count=1` | PASS (`12.103s`), exit 0 | `go-test-short-engine.*` |
| `go test -short -race ./engine -count=1` | PASS (`85.531s`), exit 0 | `go-test-short-race-engine.*` |
| `go test -short ./... -count=1` | PASS, exit 0 | `go-test-short-all.*` |

All final stderr receipts are empty. No concurrency, shared service, UCI or
harness code changed, so the conditional full `-race ./...` gate does not
apply. No test was rerun after passing unless its source changed.

`candidate.patch.gz` is the exact pre-commit engine/test delta. `SHA256SUMS`
hashes the concise evidence set and re-pins the immutable proof inputs.

## Scope and disposition

This is a reviewed local correctness candidate only. It changes production
behavior on this isolated task branch, but it is not merged to a default/shared
branch, pushed, published, released, installed or deployed. No strength match
or Elo claim is made or required for this proved correctness repair.

The native execution sandbox exposes `.git` read-only and correctly left the
verified candidate unstaged. After that native turn finished, the authorized
operator independently reviewed the patch and completed the local task-branch
commit through the existing controller. No sandbox, permission, daemon or
security setting was changed. The exact commit is retained in the external
`repair-completion-receipt.json` and verified Git bundle.

## Independent operator review

Review confirmed that the last `PositionTag` bit is copied by value, saved
before board mutation and restored on undo; ordinary hashing ignores it while
raw EP is absent. `Position.Copy` preserves the bit and copies the occurrence
map, and the Polyglot implementation is untouched. No production revision was
needed after native validation.

Two targeted active tests in `engine/enpassant_hash_metadata_test.go` supplement
the initial regressions. They cover an illegal first capturer followed by a
legal second capturer for both colors, and warm/cold-hash quiet descendants
below null moves for all six fixtures. The second-capturer positions also match
the exact pinned Stockfish 18 root legal sets. The predicate allocates zero
objects over 100 measured calls in each of those two-candidate cases.

The retained failing-before patch was applied to a separate detached checkout
at proof parent `8b41845a`; its replay compiled and produced all eight expected
hash/repetition assertion failures. Supplemental focused and focused race
checks passed. Their commands, outputs, exit receipts, oracle data and cgroup
context are retained under `review-*`.

`candidate.patch.gz` preserves the generator's original five-file delta;
`reviewed-candidate.patch.gz` is the complete six-file engine/test delta after
adding review coverage. The final manifest covers both source and every retained
receipt. Original proof/oracle files remain unchanged.

Patch artifacts and the two raw Stockfish review stdout streams are stored
with deterministic gzip encoding to preserve their exact bytes while avoiding
source-whitespace checks on unified-diff context and oracle terminal output.
`artifact-encodings.json` records both raw and compressed hashes. Decompress a
patch with `gzip -dc file.patch.gz` before applying it. Historical command
receipts record the original uncompressed paths that existed during the run.

This repair is ready for separate owner/integration review. It is committed only
on the isolated task branch; the live owner, released engine and default/shared
branches are unchanged. No merge, release, publication or playing-strength
claim is implied.
