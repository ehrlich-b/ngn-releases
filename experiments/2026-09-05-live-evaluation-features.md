# Live evaluation features: bounded call-path comparison

Read-only comparison at main c24393c / accepted playing source5cf301a.
This corrects an audit assumption and identifies a classical evaluation
hypothesis. It introduces no playing patch or new weight fit.

## Dormant mating helpers

The earlier draw-conversion audit said NGN used queen/rook mating bonuses.
That was incorrect. References in all tracked Go source are:

- `evaluateBasicEndgames`: its definition, and a call in
  `engine/eval_component_test.go:151`.
- `evaluateBasicMatePattern`: its definition, and two calls inside that
  dormant `evaluateBasicEndgames` function.
- The production path is `Evaluate` → `evaluateUnsafe` → `evalCoreWhite` plus
  `evalExtrasWhite`, then draw scaling. It never calls those two helpers.
- `evalExtrasWhite` does call `evaluateKingActivity` below phase6.

Commit `c6675ff1ec1c0ae0dd1cf3fdf4ddf42950c5002c`, April23,2026,
replaced the old evaluation with PeSTO and removed the endgame-helper call.
The helper bodies remained. The old component diagnostic reconstructs obsolete
terms and logs differences from `Evaluate`; its passing status does not certify
live feature activation. Current Texel trace/live parity is a separate check.

This does not establish that restoring the old large mating bonuses would
improve play. The mate-stop guard already repairs the reproduced conversion
failures without changing evaluation. Do not enable legacy helpers as a
correctness fix merely because they exist.

## Explicit bishop-pair feature

NGN's production `evalExtrasWhite` omits `evaluateBishopPair`. The current
Texel model has no bishop-pair coordinate. Source history identifies removal
in `037f853`, whose commit message and current code comment cite an800-game
fixed-node removal result of +20.4 [−4,+45]. That interval was inconclusive;
the run predates the June28 reset. It does not close a current bishop-pair
experiment under the present operating policy.

All three pinned classical references explicitly score the pair in their live
evaluators:

| Reference | Active path | Representation |
|---|---|---|
|Counter3.8, b172b99|`eval/evaluation.go:372`|Separate `BishopPairMaterial` added/subtracted when a side has at least two bishops|
|Blunder8.5.5,89230a7|`engine/evaluation.go:311`|Separate MG22/EG30 bonuses for at least two bishops|
|Chess-3v4,a335316|`eval/eval.go:40` → `addBishopPair` at256|A bonus indexed by friendly pawn count, applied to both MG and EG|

These local source revisions remain under `output/review-2026-09-04/`.
Their values are not interchangeable defaults or predicted NGN Elo gains.
The old claim that a pair term necessarily double-counts PeSTO bishop values
is not a mechanism proof: a linear per-bishop material/PST value cannot
independently price acquiring the second bishop. Other live evaluation terms
may partially compensate, so absence of this explicit feature is a hypothesis,
not a proved scoring bug. The current tuner cannot directly adjust a coordinate
that was removed from its model.

After the existing correctness candidates, a reopened bishop-pair experiment
would require an explicitly scoped live feature, trace/import parity if made
trainable, and its own prospective paired game gate. No candidate is built here.

## Reproduction

```sh
rg -n 'evaluateBasicEndgames|evaluateBasicMatePattern' --glob '*.go' --glob '!output/**' .
git show c6675ff -- engine/eval.go
git show 037f853 -- engine/eval.go engine/texel.go
rg -n -i 'bishop.?pair' output/review-2026-09-04/CounterGo/eval output/review-2026-09-04/chess-3/eval output/review-2026-09-04/blunder/engine/evaluation.go
```

A feature inventory must follow the live call chain. Finding a helper, a
coefficient declaration or a passing diagnostic test is insufficient.
