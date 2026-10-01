# Counter 5.5 loader and full-refresh oracle plan

Status: **SOURCE REVIEW REQUESTED — no implementation, build, or oracle run yet.**

This is the first compatibility slice for the pinned Counter 5.5 pretrained network. It stays separate from NGN-v1 and from search/backend selection. It proves file parsing, piece-square mapping, portable float32 full refresh, and the historical score adapter against Counter's own pinned code. It does not add incremental state, wire search, claim AVX/FMA parity, or claim playing strength.

## Frozen provenance

- NGN worktree: `/home/ehrli/repos/ngn-counter-pretrained-control`, detached base `f33178643b04f52fcdb881034bab0bd1ffcd74c2`.
- Counter repository commit: `63c487ca724c620f71c129d62129c6fb9109c872`.
- Release: `v1.55.0` / “Counter 5.5”, published 2024-01-12. The release says the training bug was fixed, the network changed to 768x512x1, and a new net was trained.
- License: pinned repository `LICENSE` is GNU GPL v3 text, Git blob `94a9ed024d3859793618152ea559a168bbcbb5e2`. Any copied Counter oracle source remains test-only and retains the upstream license and attribution. The production loader/evaluator will be a small independent implementation in NGN's tree with provenance comments.
- Network: `pkg/eval/nnue/n-30-5268.nn`, 1,576,988 bytes, SHA256 `3488baed71f4d432d028e05b65e2ff13f8c9afe7f91974d1a3c34a0691d6670c`.
- The seven Counter NNUE files, model, release metadata, and Git tree are cached under `/home/ehrli/repos/ngn-next/output/counter-pretrained-control-research-20260906`. The common parser sources needed by the oracle are content-addressed in `output/counter-pretrained-control-plan-v1/upstream-common` in this isolated worktree. `source-contract-receipt.json` binds every source used for these conclusions.

## Verified legacy file contract

The file starts with this exact 24-byte prefix:

`425a02000100000000030000010000000100000000020000`

Its six little-endian uint32 words are `(154178, 1, 768, 1, 1, 512)`. The old loader merely consumes the bytes; it does not validate their meanings. The compatibility loader will compare the literal bytes and will not attach invented semantics to the words.

The remaining 1,576,964 bytes are exactly 394,241 little-endian float32 values in this order:

1. 393,216 hidden weights, indexed input-major as `weights[input*512+hidden]` for 768x512.
2. 512 hidden biases.
3. 512 output weights.
4. One output bias.
5. Exact EOF; trailing bytes are an error.

Every value in the pinned model is finite. The observed range is `[-40.71996307373047, 43.440914154052734]`.

The loader must reject nonfinite tensors and models whose conservative float32 arithmetic bounds overflow. For each hidden lane, calculate in float64:

`laneBound = abs(bias) + sum(square=0..63, max(piecePlane=0..11, abs(weight[plane*64+square][lane])))`.

This is conservative for any board with no square in two planes, including non-chess boards with all 64 squares occupied. Require every lane bound to fit finite float32. Also require each `laneBound*abs(outputWeight)` and `abs(outputBias)+sum(laneBound*abs(outputWeight))` to fit finite float32. The pinned model's observed bounds are:

- maximum accumulator lane bound: `806.5222992897034`
- maximum output product bound: `27614.78091822753`
- total absolute output bound: `949174.8251401994`

These checks prevent accepted inputs from producing infinity through the prescribed full-refresh arithmetic. They do not promise useful chess values for arbitrary compatible weights.

## Verified feature and arithmetic contract

Counter's piece enum is `Empty=0, Pawn=1, Knight=2, Bishop=3, Rook=4, Queen=5, King=6`. `GetPieceTypeAndSide` returns `side=true` for White and false for Black. Squares are `A1=0, B1=1, …, H8=63`.

For an occupied square:

`plane = pieceType - Pawn + (black ? 6 : 0)`

`feature = square ^ (plane << 6)`

Because the square occupies bits 0..5 and the plane begins at bit 6, this equals `plane*64 + square`. Planes 0..5 are White pawn through king; planes 6..11 are Black pawn through king.

NGN independently uses `WhitePawn..WhiteKing=1..6`, `BlackPawn..BlackKing=7..12`, and `A1=0..H8=63`. Its adapter formula is therefore `(int(piece)-int(WhitePawn))*64 + int(square)`. The compatibility package must not reuse NGN-v1's perspective-flipped feature logic.

Portable full refresh must preserve Counter's exact operation order:

1. Scan squares 0 through 63; derive exactly one feature for each occupied square.
2. Copy hidden biases into a `[512]float32` accumulator.
3. For each feature in square order, add its 512 input-major weights in hidden-index order.
4. Initialize `output` as float32 zero; for hidden lanes 0 through 511, add `accumulator[i]*outputWeight[i]` only when the accumulator is greater than zero.
5. Add the float32 output bias and return a White-perspective raw score. Side to move is not an input and does not alter this raw result.

The first oracle uses Counter's `!avx` source, `GOAMD64=v1`, and no `avx` build tag. Under that frozen target, active features, all 512 accumulator float32 bit patterns, and the raw output float32 bit pattern are exact gates. AVX/FMA and incremental update equivalence are outside this slice.

Counter's historical adapter remains a test-only compatibility oracle in this slice:

1. `int(raw)` truncates toward zero.
2. Clip to `[-15000, 15000]`.
3. Let `np = 4*popcount(knights|bishops) + 6*popcount(rooks) + 12*popcount(queens)`, counting both colors.
4. Integer `score = score*(160+np)/160`, division toward zero.
5. Integer `score = score*(200-rule50)/200`, division toward zero.
6. Negate for Black to move.

The clip occurs before material and rule-50 scaling, so the returned magnitude is not generally bounded to 15000. The production compatibility package will return only the raw White-perspective float32 in this slice.

## Proposed minimal production API

Create a top-level `countereval` package with no dependency on `engine`:

```go
const (
    LegacyHeaderSize = 24
    LegacyFileSize   = 1_576_988
    InputSize        = 768
    HiddenSize       = 512
    FeaturePlaneCount = 12
)

type Board [FeaturePlaneCount]uint64

type LoadMetadata struct {
    SHA256 string
    Bytes int
    Values int
    MaxAccumulatorBound float64
    MaxProductBound float64
    OutputBound float64
}

func LoadCounter55Legacy(r io.Reader) (*Model, LoadMetadata, error)
func (m *Model) EvaluateFullRefresh(board Board) (float32, error)
```

`Board` is in Counter plane order. Evaluation rejects overlapping plane bits, guaranteeing at most one feature per square. It deliberately does not enforce king counts or legal-position rules; legality belongs to the engine adapter, while the numeric contract is well-defined for every overlap-free board.

The core API does not expose weights, accumulators, or a score adapter. Package-internal test helpers may inspect feature indices and accumulator bits. A later engine adapter can convert an NGN board by scanning squares and using `Bitboard.PieceAt`, without creating a package cycle.

Recommended identity rule: the named loader validates the exact Counter 5.5 layout/header/EOF and safe finite arithmetic, while the experiment manifest requires the pinned network SHA256. This allows negative tensor tests and keeps format validation distinct from artifact identity. If root instead wants the library API itself locked to this one model, add a separate `LoadPinnedCounter55` wrapper rather than hiding an identity check in the format loader.

## Actual Counter oracle

After plan approval, create a separate copied checkout of the complete Counter tree at the pinned commit. Do not import it into NGN or edit its production files. Add one test-only file inside upstream `pkg/eval/nnue` so it can call the actual unexported `calculateNetInputIndex` and inspect `hiddenOutputs` after `Init`.

Build and run with the pinned WSL Go tool, `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOAMD64=v1`, no `avx` tag, and at most two build/test workers. The oracle reads only the pinned model and a frozen FEN fixture and emits canonical JSON records containing:

- literal input FEN and Counter's parsed/canonical position facts;
- WhiteMove, Rule50, and the computed non-pawn material term;
- actual ordered active feature indices;
- all 512 accumulator float32 bit patterns;
- raw White-perspective float32 value and bits;
- actual `EvaluateQuick` adapted integer.

NGN tests parse each same FEN independently, map it through `PieceAt` into `countereval.Board`, and compare ordered features, every accumulator bit, raw bits, and a test-only independent adapter result. The fixture will include paired White/Black-to-move copies of asymmetric material, start position, castling-right positions, legal en-passant for each color, promoted-extra-piece positions, A1/H8 boundary occupancy, and halfmove clocks 0, 99, and 100. Fixture finalization must first prove every literal is accepted by both pinned parsers; rejected or normalized-away cases cannot silently disappear.

The oracle harness must require exactly one record for every frozen fixture ID, reject duplicate/unknown/missing records, bind model/source/fixture/toolchain hashes, preserve stdout/stderr/exit status, and reject a success-returning oracle stub.

## Required negative and mutation tests

Loader rejection cases:

- each header byte/word family changed;
- truncation in the header and immediately before/inside/after each tensor section;
- one trailing byte;
- NaN, positive infinity, and negative infinity in each tensor section;
- finite synthetic weights exceeding the conservative accumulator, product, or output bound;
- empty reader and read errors.

Board and mapping cases:

- overlapping piece planes;
- White/Black plane swap;
- square vertical flip/XOR 56;
- piece-plane permutation;
- feature iteration in plane-major rather than square-major order;
- applying STM sign to raw output.

Arithmetic cases:

- omit output bias;
- disable ReLU;
- reverse the output reduction order;
- move clipping after material scaling;
- change adapter multiplication/division order;
- use rounded instead of truncating float-to-int;
- apply STM sign before the other adapter operations.

Each deliberately faulty mapping/arithmetic implementation must be rejected by at least one named frozen fixture. The mutation receipt records the witness fixture and first mismatching field. The harness has its own missing/duplicate record and success-stub rejection controls.

## Acceptance and next boundary

This slice passes only if the strict loader rejects every malformed model, the pinned model metadata and bounds match, all portable oracle feature/accumulator/raw bits match, the independent test adapter matches actual Counter integers, every mutation is killed, all commands exit zero, and no child process survives.

A pass establishes loader/full-refresh compatibility with the pinned portable Counter 5.5 implementation. It does not establish incremental state, AVX equivalence, search integration, speed, playing strength, or fitness to replace HCE. The next separately reviewed slice is incremental transition parity against both upstream incremental state and fresh full refresh, with an explicitly justified floating-error rule.
