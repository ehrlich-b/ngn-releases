# Counter 5.5 incremental-context compatibility plan

Status: source and oracle design only. The strict legacy loader and portable
full-refresh reference are complete. This plan does not implement an
incremental context, connect Counter to NGN search, change the deployed
binary, or authorize an engine run.

## Pinned reference

The behavioral reference is Counter commit
`63c487ca724c620f71c129d62129c6fb9109c872`, portable `!avx` arithmetic,
and the already pinned `n-30-5268.nn` model
SHA-256 `3488baed71f4d432d028e05b65e2ff13f8c9afe7f91974d1a3c34a0691d6670c`.

The relevant upstream contract is concrete:

1. A search root calls `EvaluationService.Init(root)` before entering search.
   Init copies all 512 hidden biases, scans occupied squares A1 through H8,
   and adds each active feature row in that square order.
2. For a legal ordinary move, Counter first creates and validates the child
   position. It then calls `evaluator.MakeMove(preMovePosition, move)`.
3. `MakeMove` derives updates from the pre-move position and appends them in
   this exact order:
   - remove the moving piece at the source;
   - when present, remove the captured piece at the destination, or at the
     actual en-passant capture square;
   - add the moving piece, or promotion piece, at the destination;
   - for castling, remove the rook at its original square;
   - for castling, add the rook at its destination.
4. `UpdateHidden` increments the context index, copies the preceding 512
   float32 accumulator lanes, then applies every update in insertion order.
   Each update processes hidden lanes 0 through 511 in order.
5. A null move first constructs the child position, then pushes an empty
   update list. The accumulator is therefore a bit-exact copy even though
   side to move, en-passant state, rule50, and the position key change.
6. `UnmakeMove` only decrements the accumulator index. It does not apply
   inverse arithmetic. The engine position itself is held in a parallel search
   stack, so returning from recursion exposes the prior position and prior
   accumulator frame together.
7. Counter allocates 128 frames indexed 0 through 127. A checked replacement
   must allow at most 127 pushes after root initialization and reject overflow
   and root underflow before mutating state.

Primary locations in the pinned tree are
`pkg/eval/nnue/evaluation.go:69-156`,
`pkg/eval/nnue/nnue_instructions.go:8-34`, and
`pkg/engine/search.go:34-38,520-536`. Counter's position transition is in
`pkg/common/position.go:213-322`.

## Proposed package boundary and API

The next compatibility slice may add a context inside `countereval`; it must
still have no dependency on `engine`.

The context owns:

- the immutable `*Model`;
- 128 accumulator frames of 512 float32 lanes;
- the corresponding 128 `countereval.Board` values;
- a current depth;
- per-frame arithmetic-bound bookkeeping used by tests and receipts.

A move-neutral package type should describe semantic deltas without importing
NGN's move representation:

```go
type MoveDelta struct {
    MovingPlane   uint8
    From, To      uint8
    CapturedPlane int8 // -1 means none
    CaptureSquare uint8
    PromotionPlane int8 // -1 means none
    CastleRookFrom int8  // -1 means no castle
    CastleRookTo   int8
}

func (m *Model) NewContext(root Board) (*Context, error)
func (c *Context) PushMove(delta MoveDelta, expectedPost Board) error
func (c *Context) PushNull() error
func (c *Context) Pop() error
func (c *Context) Board() Board
func (c *Context) EvaluateRaw() float32
```

Names may change during implementation review, but the semantics may not:

- `PushMove` validates ranges, paired optional fields, current occupancy,
  capture identity, promotion consistency, castling rook consistency, and
  capacity before touching a frame.
- It independently applies the semantic delta to the current board and requires
  exact equality with `expectedPost`. It then creates the five possible
  feature-row updates in the pinned Counter order above.
- Validation failure is transactional: depth, board frames, accumulators, and
  bound counters remain bit-identical.
- `PushNull` accepts no board mutation and copies the preceding accumulator
  and board exactly while advancing depth.
- `Pop` at depth zero fails transactionally. A valid pop only decrements the
  frame index; it never adds inverse weights.
- `EvaluateRaw` uses the already proved portable ReLU/output order. Side to
  move and the historical score adapter remain outside the context.

A later engine adapter, in a separately reviewed slice, will extract
`MoveDelta` while the NGN position still represents the parent, make and
legality-check the engine move, translate the resulting board, then call
`PushMove(delta, postBoard)`. A context failure must cause the adapter to
restore the engine position before returning failure. Null make/unmake must
push/pop the context beside NGN's position mutation. No search code belongs in
the first incremental compatibility slice.

## Independent upstream transition oracle

Extend the isolated, exact Counter checkout with a test-only oracle. It must
create real Counter moves from its legal move generator rather than synthesise
packed move words. For every root, push, null, and pop it records:

- root identifier, transition identifier, depth, pre/post canonical FEN, and
  UCI move or explicit `null`/`pop`;
- the ordered feature indices and coefficients generated by Counter;
- all 512 accumulator float32 bit patterns;
- portable raw-output float32 bits;
- current active feature set;
- the matching earlier frame identifier after a pop.

The NGN-side oracle consumer reconstructs legal NGN positions and moves,
derives the neutral delta, and requires exact equality for transition order,
active features, every accumulator bit, and the raw-output bit at every frame.
It also runs `EvaluateFullRefresh` on each post board as the independent
state oracle; full-refresh and incremental float bits are not assumed equal.

The frozen fixture set must include both colors and at least:

- quiet pawn and non-pawn moves, captures by both colors, and a branch with
  repeated push/pop;
- white and black en-passant captures from positions with a real legal EP move;
- non-capture and capture promotions, with at least two promotion piece types;
- king-side and queen-side castling, including both colors across the set;
- null push/pop while en-passant is present, proving the board/accumulator copy
  and the independent position-state change;
- maximum accepted depth, overflow, root underflow, malformed delta, mismatched
  post board, overlap, wrong capture square/piece, and invalid promotion or
  castling metadata.

True-fault mutation witnesses must reject swapped capture/add order, wrong EP
capture square, promotion-as-pawn, each omitted castling-rook update, null
refresh or mutation, inverse-arithmetic unmake, and any validation failure that
partially advances the context. A named mutation is accepted only when the
frozen fixture changes the expected oracle record; an equivalent mutation is
reported as equivalent rather than called killed.

## Floating-point drift policy and bound

Incremental addition is deliberately path ordered, while full refresh is
square ordered. Their float32 accumulators can differ even when they represent
the same exact active-feature sum. Compatibility therefore has two separate
gates:

1. Exact bit parity against pinned Counter incremental portable arithmetic is
   mandatory at every recorded frame.
2. Difference from an independently recomputed full refresh must be finite and
   lie within a precomputed rounding-error bound. This is a diagnostic safety
   gate, not permission to substitute a looser numerical implementation.

Let `u = 2^-24`, `eta = 2^-150`, and
`gamma(n) = n*u/(1-n*u)`. For a hidden lane, a frame beginning with an exact
float32 bias and applying `n` float32 additions/subtractions has:

```text
S = abs(bias)
    + sum(abs(initial active-row values))
    + sum(abs(all update-row values on the surviving path))

E_acc <= gamma(n)*S + n*eta/(1-n*u)
```

The second term retains an absolute roundoff allowance when a result is
subnormal, where a purely relative bound is insufficient. The initial active
rows and every update row are counted in their actual execution order.
A null adds zero operations. A pop restores the earlier frame's `n`, `S`,
and accumulator; traversed sibling work does not accumulate into it.

For an independently fresh post-board accumulator, compute `E_fresh` from
its square-ordered active rows. The required lane check is:

```text
abs(incremental_lane - fresh_lane) <= E_acc + E_fresh
```

All terms are evaluated conservatively in float64 with outward rounding. A
test-only exact-real oracle additionally converts each float32 model value to
its exact binary rational and sums with `math/big.Rat`; it verifies both
computed accumulator errors directly and falsifies the bookkeeping mutants.
The analytic bound and exact-real check are independent of the production
float32 accumulator.

For raw output, ReLU is 1-Lipschitz. With `H=512`, define for each path:

```text
E_input  = sum_j(abs(outputWeight[j]) * E_acc[j])
T        = abs(outputBias)
           + sum_j(abs(outputWeight[j])
                   * (abs(exactAccumulator[j]) + E_acc[j]))
E_reduce = gamma(2*H + 1)*T
           + (2*H + 1)*eta/(1-(2*H + 1)*u)
E_raw    = E_input + E_reduce
```

This covers one rounded multiply and one rounded add per lane plus the output
bias addition, without assuming fused multiply-add. Both the observed error
against the exact-real network value and
`abs(rawIncremental-rawFresh) <= E_rawIncremental+E_rawFresh` must hold.
The loader's existing finite/range checks remain prerequisites.

The operational drift policy is deterministic:

- every new search root or worker initializes with full refresh;
- no accumulator survives between UCI searches;
- a search frame uses incremental updates only, up to the checked depth limit;
- null copies and unmake restores; neither introduces new rounding;
- no periodic or threshold-based silent refresh is allowed in the
  compatibility slice, because it would cease to match Counter's path;
- any exact-oracle mismatch, non-finite value, or proved-bound violation rejects
  the slice. A refresh policy change would require a separate experiment.

With at most 32 root pieces, at most five update rows per ordinary move, and
127 pushes, a conservative live path has no more than 667 hidden-lane
add/subtract operations; receipts use the actual per-frame count and absolute
sum, not this worst case.

## Import-cycle resolution before engine integration

The current build-tagged `countereval/oracle_parity_test.go` is in package
`countereval` and imports `engine`. Once production `engine` imports
`countereval`, that same-package test would create an import cycle.

Before integration:

- retain package-internal loader, arithmetic, context, transaction, and
  mutation tests in `package countereval`, with no engine import;
- move FEN/move adapter and cross-package parity tests to external
  `package countereval_test`, which may import both `engine` and
  `countereval`;
- compare only exported context/evaluation results at that boundary;
- keep the actual Counter oracle generator in the isolated upstream module;
- if accumulator-bit diagnostics are needed, emit them from package-internal
  tests or a build-tagged test command rather than exposing mutable production
  accumulator storage.

This reorganization must be a byte-for-byte oracle-data-preserving test
refactor before search wiring.

## Staged acceptance

The next implementation request is limited to the context, independent
transition oracle, exact parity tests, analytic/exact-real drift checks, and
true-fault mutation controls. It may not edit search, UCI, deployment, or the
historical score adapter. Search integration remains a later reviewed slice
after this packet passes.
