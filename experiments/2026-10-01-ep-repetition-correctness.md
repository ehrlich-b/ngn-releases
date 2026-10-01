# En-passant repetition correctness — exact baseline proof

Date: 2026-10-01 UTC
Branch: `task/ep-repetition-correctness-20261001`
Source baseline: `701328646490f1bc8ed214002f578a58c9b0723e`
Scope: analysis and retained diagnostics only; no production engine behavior,
match, benchmark, tuning, training, release, deployment, or owner resource was
changed.

## Answer

**No.** The current adjacency-only en-passant normalization is not a
FIDE-correct repetition identity. When an adjacent pawn's only apparent EP
capture is illegal because it would expose its king, or because it does not
answer a separate check, NGN retains the EP square and XORs it into the one
ordinary Zobrist key. The position has the same legal possibilities as its
no-EP counterpart, but the keys differ. A legal played double push followed by
two reversible four-ply cycles therefore produces three FIDE-identical
occurrences while NGN records the first under the EP key and the later two under
the no-EP key. `IsFIDEDrawRule` consequently remains false at the true third
occurrence.

This conclusion does **not** come from an artificial FEN pair. Four minimal
legal played histories (pin and unrelated-check mechanisms, mirrored for both
capturing colors) reproduce it. FEN-only behavior is recorded separately
because Stockfish 18 intentionally treats FEN ingestion differently from a
real double push.

## Rule and code mechanism

FIDE Laws of Chess 9.2.3 defines the same position by the same player to move,
same piece placement and same possible moves; 9.2.3.1 distinguishes an EP
capture only when it was actually available. Article 3.9 makes a move that
leaves the mover's king attacked illegal. Primary-source location and retrieval
receipt are in
`2026-10-01-ep-repetition-correctness/primary-sources.md`.

At the exact NGN baseline:

- `engine/polyglot.go:131-175` implements `canCaptureEnPassant` as adjacency
  only. It does not apply king safety or require an EP evasion to answer check.
- `engine/position.go:216-277` creates the target after a double push and drops
  it only when that adjacency predicate is false. `engine/fen.go:42-56` uses
  the same predicate on FEN load.
- `engine/hash.go:57-92` includes every retained EP target in the ordinary
  Zobrist key. `GameMakeMove` counts that same key in `Positions`
  (`engine/position.go:166-174`), and `IsFIDEDrawRule` reads it
  (`engine/position.go:451-461`). Main search computes this one key once for
  both repetition and the TT (`engine/search.go:1398-1404,1463-1473`); qsearch
  does likewise. Other ordinary-key consumers (eval cache, Texel dedup and the
  placeholder Syzygy index) were inspected too.
- `PolyglotHash` is a separate book lookup contract
  (`engine/polyglot.go:58-128,218-229`). Its adjacency EP treatment is retained
  and is not evidence that game repetition should use the same identity.

The June 28 test's pinned paragraph proves only incremental/FEN agreement under
the adjacency policy. Its comment that this is “consistent with
Stockfish/Polyglot” conflates Stockfish's real-move repetition normalization
with FEN ingestion and Polyglot book hashing.

## Independent Stockfish 18 oracle

Artifact:
`/home/ehrli/nnue-owned-morning-20260930/inputs/stockfish18`
SHA-256:
`6b087694916228c905a5e14db74cca8c7e5643602226af1fa5d42353c455b9f9`
Banner: `Stockfish 18 by the Stockfish developers (see AUTHORS file)`

Exact release source `position.cpp` was also inspected at official SF18 commit
`cb3d4ee9b47d0c5aae855b12379378ea1439675c`; its SHA-256 is
`a1e4895235c14cb231e71135812ea5a5c7dcba100d1a459d290925a0a84c7a0d`.
The FEN loader at lines 268-284 checks an adjacent pawn/victim/empty squares but
not king safety. In contrast, real `do_move()` at lines 904-959 says accurate
EP is required for Zobrist/threefold, rejects an EP target when another checker
exists, and tests rook/bishop exposure before adding the EP key.

The retained driver is
`experiments/2026-10-01-ep-repetition-correctness/stockfish_probe.py`; its
machine-readable receipt is `stockfish-probe.json`. It hash-pins the executable,
uses only `d`, `go perft 1` and `go perft 2`, and verifies every move before
extending a played history.

### Illegal-EP cases

| Fixture | Real-push / no-EP key | FEN-with-EP key | Exact legal moves after push | perft 1 / 2 |
|---|---:|---:|---|---:|
| pinned, White captures | `B7B0D64149BD86A0` | `DB5FED5254C85AE2` | `a5a4 a5a6 a5b4 a5b5 a5b6 e5e6` | `6 / 95` |
| pinned, Black captures | `4A43D48DA3210858` | `944EA550AB65A55A` | `d4d3 h4g3 h4g4 h4g5 h4h3 h4h5` | `6 / 95` |
| unrelated check, White captures | `0E7BF4EA5C4F3EEC` | `6294CFF9413AE2AE` | `a4a3 a4a5 a4b3 a4b4` | `4 / 52` |
| unrelated check, Black captures | `B982BB711E3B21AE` | `678FCAAC167F8CAC` | `h5g5 h5g6 h5h4 h5h6` | `4 / 52` |

For each row, real-push, EP-FEN and no-EP FEN legal move sets and depth-1/2
perft are exactly equal; the apparent EP move is absent. Stockfish's real-move
path emits `-` for EP and its key equals the no-EP key. Its FEN path retains the
pseudo target and produces the other key, which is why the FEN key is not used
as repetition ground truth.

### Minimal legal played repetitions

Each sequence begins at its listed legal start FEN. Stockfish validates all
nine plies and reports the same normalized incremental key after plies 1, 5 and
9.

1. Pinned, White apparent capturer:
   `6k1/3p4/8/K3P2r/8/8/8/8 b - - 0 1`
   `d7d5 a5a4 h5h6 a4a5 h6h5 a5a4 h5h6 a4a5 h6h5`
2. Pinned, Black apparent capturer:
   `8/8/8/8/R2p3k/8/4P3/1K6 w - - 0 1`
   `e2e4 h4h5 a4a3 h5h4 a3a4 h4h5 a4a3 h5h4 a3a4`
3. Separate check, White apparent capturer:
   `4b1k1/3p4/8/4P3/K7/8/8/8 b - - 0 1`
   `d7d5 a4a3 e8f7 a3a4 f7e8 a4a3 e8f7 a3a4 f7e8`
4. Separate check, Black apparent capturer:
   `8/8/8/7k/3p4/8/4P3/1K1B4 w - - 0 1`
   `e2e4 h5h6 d1c2 h6h5 c2d1 h5h6 d1c2 h6h5 c2d1`

The first two remove both pawns from a rook line during the apparent EP capture,
exposing the king. The latter two use a double push to uncover a bishop check;
the adjacent pawn's apparent EP capture would not answer that checker.

### Ordinary legal-EP controls

Both colors distinguish correctly. With a legal target, the EP move is present,
the real-push key equals the EP-FEN key, and the no-EP state loses exactly that
root move:

- White `e5d6`: EP/no-EP perft `5/91` versus `4/75`, keys
  `4F069168A1E75693` versus `23E9AA7BBC928AD1`.
- Black `d4e3`: EP/no-EP perft `5/91` versus `4/75`, keys
  `781E08BB5A021557` versus `A61379665246B855`.

## Exact-baseline NGN diagnostic

`experiments/2026-10-01-ep-repetition-correctness/ngn_baseline_probe.go` is a
standalone success-on-defect diagnostic, not an active failing `_test.go`. It
uses real `GameMakeMove` histories and independently constructs a test-only
FIDE identity by retaining the generated FEN EP field only when
`GenerateLegalMoves` contains an EP move. It covers:

- FEN-load and incremental keys;
- exact legal sets and depth-2 perft;
- true third-occurrence tracking versus NGN's hash map;
- both colors and both illegality mechanisms;
- ordinary legal-EP controls for both colors;
- search make/unmake, game make/unmake (including the repetition map), and null
  make/unmake restoration; and
- the separate ordinary and Polyglot hashes.

NGN probe result: **all four exact-baseline defects reproduced; both legal EP
controls passed**. Each illegal-EP history has three legal-identity occurrences
but only two occurrences in NGN's hash map, and `IsFIDEDrawRule` remains false.
Search make/unmake, game make/unmake including the repetition map, and null
make/unmake all restore the exact snapshot in all four cases.

The independently run `compare_probe_receipts.py` verifies all six fixtures
against the unchanged Stockfish receipt. Legal move sets and depth-2 perft match
at each real-push/EP-FEN/no-EP stage. All four nine-ply histories match the
Stockfish legal-position identity; Stockfish's played keys repeat at plies
1/5/9 while NGN's first key differs from the equal later two keys, whose counts
are 1/1/2. Its input SHA-256 hashes and assertions are retained in
`independent-comparison.json`.

## Commands and gates

Stockfish command is retained verbatim in `stockfish-command.txt`; exit was
zero and stderr empty. The gate runner
`experiments/2026-10-01-ep-repetition-correctness/run_go_gates.sh` refuses to
compile until `../cpu-coordinated.json` exists with `approved:true`, then pins
`GOMAXPROCS=1` and `GOFLAGS=-p=1`, records cgroup limits and runs exactly once:

```sh
/usr/local/go/bin/go run experiments/2026-10-01-ep-repetition-correctness/ngn_baseline_probe.go 701328646490f1bc8ed214002f578a58c9b0723e
/usr/local/go/bin/go test -short ./engine -count=1
/usr/local/go/bin/go test -short -race ./engine -count=1
/usr/local/go/bin/go test -short ./... -count=1
```

All four commands exited **0**, with empty stderr. The recorded environment is
Go 1.25.5 on Linux/amd64, `GOMAXPROCS=1`, `GOFLAGS=-p=1`, affinity `0,2`, nice
10, `cpu.max=50000 100000`, and `memory.max=4294967296`. The go-ublk owner
explicitly acknowledged the same shared CPU/resource limits before admission;
that acknowledgement is retained in `go-environment.txt`. The gated run began
at 00:39:47 UTC and finished at **00:43:17 UTC** on 2026-10-01.

- Short engine suite: passed, 12.320 seconds reported by Go.
- Short engine race suite: passed, 93.211 seconds reported by Go.
- Short all-packages suite: passed, including the standalone probe's package.
- Independent receipt comparison: passed for all six fixtures.

Commands, stdout/stderr and exit receipts are retained beside the diagnostic;
`SHA256SUMS` covers these artifacts and this report. The analysis native thread
was idle during Go execution. After its finalization turn reached its cumulative
task token cap, the coordinator completed the receipt comparison, documentation,
syntax checks and experiment-only local commit without further model calls.
This was a task-goal limit, not a personal provider quota or payment denial.

## Disposition and smallest next change

**This task proves an existing defect; it does not repair production behavior.**
The retained standalone diagnostic succeeds
only when the exact current defect and all controls/restoration properties are
observed; the normal suite remains green.

The smallest next bounded correctness change is to introduce an ordinary-key
EP predicate that requires at least one **legal** EP capture after the real
double push (covering both king exposure and unrelated check), while preserving
Polyglot's independent adjacency/book contract. Apply it consistently to full
and incremental ordinary hashing or to both EP state set-sites, then promote
the four played histories and two legal controls into active engine regression
tests. The required gate is the same three baseline commands above plus the
focused regression, make/unmake/null restoration, incremental/full-key equality,
and short race coverage. No match or strength gate is required for the proof;
any behavior patch still needs the repository's correctness acceptance process.
