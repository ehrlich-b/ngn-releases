# Rodent V1.1 Anand named-backend integration contract

Status: source-only design at NGN `270c73563139996d03f3c34b2840015b8e130df9`.
No production implementation, build, search, game, download, or deployment is
authorized by this record.

## Pinned evidence boundary

- Exact network:
  `/home/ehrli/rodent-v1.1-public-preflight-20260906/extracted/release/release-1-1/nets/rodent_anand_512hl.bin`,
  789,568 bytes, SHA-256
  `5f7480b56538e9e64ee02fb5b4842bf0394ee44b707dae3a63ec7f902abd0afb`.
- Exact release oracle executable:
  `rodent_v_1.1_testers_linux_amd6`, 4,013,485 bytes, SHA-256
  `9c68d7b39dc933eff5fd2da88bd4be1b0d009f6d2add7485c121cfc0d1308531`.
  It contains exactly one byte-identical Anand network at offset 2,131,712.
- Supporting source is tag `Rodent_v_1_1`, commit
  `5689d0babebe95d87592eaaaee73ea555ef9345c`, tree
  `be0effaf52855fcbdf4a288fc1c203e1febcb5ce`, source-archive SHA-256
  `929f560996c3b0a8609e594923e587ab0eef61c9c013afbbd4497045113523fd`.
  It is mechanism evidence, not a reproducible build claim for the testers
  executable.
- Anand personality SHA-256
  `b0e336bc8892043063e6b1fcc42193bbbc6cacf347e9d561d089ddcb602bf3aa`
  locks `hceWeight=0`, `nnueWeight=100`, `nnueScale=192`, and
  `horizontalMirroring=1`.
- Principal tagged files used here: `nnue.go`
  `bb6689c3465996fe982bcc77272c1033744fa53030958a4afb737523d55995bc`,
  `nnue_scalar.go`
  `8623a4c49afa034140e369bbbdf3a2077606ba4a7653b74424128d0555173c2e`,
  `eval.go`
  `2a8b8b955ad2f2d692f721eb5b39b03ed27b576970d80de1d393fee8fb7a9891`,
  and `tables.go`
  `72c5805e540a29030d0bdb84acb7253eec35313b9b45c352610755e6ce4145a9`.

## Serialization

The file is a raw little-endian signed-int16 Bullet parameter stream:

1. input weights `[768][512]int16`, input-major then hidden lane;
2. input biases `[512]int16`;
3. output weights `[2][512]int16`, perspective-output row then hidden lane;
4. one output bias `int16`;
5. a 62-byte trailer, exactly `"bullet"` repeated ten times plus `"bu"` in
   the pinned artifact.

The tensor payload is 789,506 bytes; payload plus trailer is 789,568. Tagged
`nnueLoad` (`nnue.go:574`) decodes the first four items and silently ignores
trailing bytes, while the embedded path reinterprets the prefix as the same
struct. The NGN named loader should be stricter: require the exact byte count,
trailer, and pinned SHA-256, reject truncation/additions, decode explicitly
rather than with `unsafe`, and publish an immutable validated model only after
the entire parse succeeds. This backend is for the exact Anand artifact, not a
generic Bullet-network loader.

## Features and accumulator

Rodent squares are A1=0 through H8=63; White=0, Black=1; piece types are
P,N,B,R,Q,K = 0..5 (`tables.go:39-78`). For a piece `(color, pt, sq)`, own king
square `kingSq`, and perspective `p` in `{0,1}` (`nnue.go:190-201`):

```text
orientedSq = sq
if p == Black: orientedSq ^= 56
if file(kingSq) > 3: orientedSq ^= 7
feature = (color ^ p)*384 + pt*64 + orientedSq
```

Thus each perspective sees friendly planes first, Black perspective flips
ranks, and each perspective horizontally normalizes around its own king half.
This is not a king-bucket network: the king square only chooses whether to
mirror all squares.

An accumulator is `[2][512]int16`. Full refresh copies the same 512 input
biases into both perspectives, then adds the appropriate feature row for every
occupied square (`nnue.go:481-522`). Addition/subtraction is two's-complement
int16 modular arithmetic, matching scalar operations and AVX2 `VPADDW/VPSUBW`.

Incremental child state is copy-make. Exact semantic deltas are:

- quiet/double-pawn: add destination, subtract source;
- capture/EP capture: add mover destination, subtract mover source, subtract
  captured feature at the actual capture square;
- castle: add king destination, subtract king source, add rook destination,
  subtract rook source;
- promotion: add promoted destination, subtract pawn source;
- promotion capture: promotion delta plus captured destination subtraction.

Tagged dispatch is in `nnue.go:430-477`; special descriptors originate in
`moves.go:45-205`. When a king crosses the D/E half-board boundary, only that
king's perspective is rebuilt from the complete post-move board; its row
updates are replaced by zero rows before refresh. The other perspective remains
incremental. A null move changes side-to-move but leaves accumulator bits
unchanged (`moves.go:322-359`). Pop restores the parent frame; it must not use
inverse arithmetic.

## Activation and score adapter

For each hidden lane, clip accumulator to `[0,255]`, square it, multiply by the
signed int16 output weight, and accumulate in int32 (`nnue.go:528-538`). The
side-to-move accumulator uses output row 0 and the other perspective row 1.
Then (`nnue.go:540-555`):

```text
sum = sum / 255 + outputBias
raw = sum * 192 / (255 * 64)
```

All divisions truncate toward zero. Preserve int16/int32 modular arithmetic;
do not widen and thereby change overflow semantics unless exhaustive bounds
prove that widening is bit-identical on this exact model.

Pure-Anand final static evaluation is `raw` followed by Rodent's material
factor (`eval.go:186-199`):

```text
material = 100*(WP+BP) + 300*(WN+BN) + 300*WB + 300*WR
         + 500*(WR+BR) + 900*(WQ+BQ)
static = raw * (25000 + material) / 32768
```

The missing Black-bishop term and second White-rook term are not transcription
errors here. Static disassembly of the exact testers executable's
`main.evaluateScaledNNUE` confirms those precise count offsets and operations.
The named compatibility backend must reproduce them; any symmetry repair is a
separate evaluation change and game gate.

Rodent V1.1 does not attenuate static evaluation as the halfmove clock rises.
It declares a draw at `clock >= 100` (`search.go:1108`) and otherwise returns
the same static score. NGN must therefore not route this backend through
`nnueSearchScore` or the Counter rule-50 adapter. NGN's existing terminal draw
handling remains search policy, and existing correction history remains NGN
search policy; neither is changed by this backend slice.

## Oracle contract

The exact release binary exposes a usable full-refresh oracle. In its release
directory, send `position fen <FEN>`, then `nnue`, then `isready`. The command
constructs a new accumulator, calls full refresh, and prints `getEval(side)`
without a newline (`uci.go:245-251`), so parse the response as
`^(-?[0-9]+)readyok$`. A no-search feasibility transcript confirmed Anand was
loaded and returned startpos/White `25`, and kings-only
`8/8/8/8/8/8/4K3/7k` as White `-17`, Black `1`.

This oracle returns `raw`, before the material factor. Derive the final expected
static from the FEN counts and the exact release-disassembled formula above.
Freeze binary, network, source, corpus, command, and output hashes.

The release exposes no accumulator-lane or incremental-state oracle. Use this
two-part proof rather than claiming one:

1. A hash-pinned test-only harness against the preserved tagged package loads
   the exact net, emits full and native-incremental accumulator digests and raw
   scores after every ply, and asserts tagged incremental equals tagged refresh.
2. NGN incremental must equal NGN refresh lane-for-lane after every ply, and
   each NGN refresh raw score must equal the exact release `nnue` output. The
   derived final static must also match the disassembled adapter formula.

The corpus must cover both sides, horizontal board pairs, own kings on D/E and
king moves across that boundary, quiet/double-pawn, capture, both EP colors,
all four castles, all promotion pieces with/without capture, null/pop, repeated
push/pop, and capacity growth. Include asymmetric Black-bishop/White-rook
positions to lock the release material quirk, and equal boards with halfmove
clocks 0/50/99 to lock the absence of rule-50 attenuation.

## Smallest NGN implementation

Add one package, tentatively `rodenteval`, and one named backend
`rodent-v1.1-anand`; do not generalize existing NNUE packages.

1. `rodenteval.Model`: strict exact-artifact loader, immutable tensors and
   metadata; portable full refresh and release-static evaluation.
2. `rodenteval.SearchContext`: worker-owned dynamic frames containing
   `[2][512]int16` plus the validated 12-plane board; checked transactional
   push for the six transition classes, bit-copy null, pop, reset, and growth.
   Start portable. AVX2 is a later pure-speed candidate after parity.
3. Engine adapter: add immutable model identity and private worker context in
   `engine/worker_evaluator.go`, transactional selection parallel to
   `SelectCounter55Evaluator`, exact board/move conversion, and backend name
   `rodent-v1.1-anand`. `SearchSTM` returns the release static above;
   `LegacyUndampedSTM` is the same value because this backend has no damping.
   Slice C review clarification: both engine routes apply NGN's existing
   non-mate-band clamp to `[-25000,+25000]` after the exact release static.
   NGN reserves scores at magnitude 29000 and above for mate normalization in
   TT storage; the package's release-static API remains exact and unclamped.
   No halfmove-clock damping is introduced. Boundary and clock-identity tests
   must lock this engine-only adaptation.
4. Startup/UCI: extend the existing `EvalBackend`/`EvalFile` staging machinery,
   requiring this exact file. Do not embed or silently substitute a network.
   Evaluator selection, SMP worker isolation, stop/unwind, TT/history reset, and
   current NGN correction/search policy retain their existing contracts.

## Validation slices and stop rules

- Slice A, about 2-4 focused hours: strict loader, portable full refresh,
  activation, raw/final adapter, and a frozen 50-100 position exact-release
  oracle. Stop on any unexplained score mismatch, serialization ambiguity, or
  inability to reproduce the asymmetric material witnesses.
- Slice B, about 4-8 focused hours: checked incremental context plus all special
  moves, refresh-boundary, null/pop, poisoned-destination, immutability,
  transactional-error, and depth-growth tests. Stop on any lane or raw-score
  drift from full refresh/tagged oracle.
- Slice C, about 4-8 focused hours: named engine/UCI selection, worker/SMP
  ownership, backend switching, full-refresh debug oracle, fixed-search
  determinism, and normal short/race suites. No search-policy changes.
- Only after A-C pass: measure portable cost. Admit one AVX2 optimization if
  needed, then use a separately frozen real-clock game gate for strength. Do not
  infer NGN Elo from Rodent's executable rating.

## Genuine limitations/blockers

- The testers executable has no VCS revision and embeds release-specific Anand
  configuration absent from the tagged build. Raw full-refresh outputs and
  exact disassembly bridge the behavior needed here, but internal release
  accumulator lanes cannot be claimed as directly observed.
- The release zip contains no license file. The tagged repository is
  GPL-3.0-only and contains the network, while NGN currently has no top-level
  license file. A clean implementation that loads the user's pinned external
  file avoids copying Rodent source and embedding bytes, but redistribution of
  the network or copied GPL implementation requires an explicit licensing
  decision before packaging. This does not block local oracle/implementation
  work; it blocks assuming distributable bundled artifacts.
