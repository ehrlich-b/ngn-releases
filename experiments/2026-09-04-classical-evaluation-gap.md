# Classical evaluation: two concrete development targets

Follow-up to the [six-angle review](2026-09-04-classical-review-prompt.md).
Root reviewed Terra's source comparison and ran the feature witness below.
NGN evaluator is unchanged from `4986330`; the pending production qsearch patch
does not change these functions. References are CounterGo 3.8 `b172b99b3128d82c44ef1d8d8a278aa09f2d721b`
and Chess-3 v4.0 `a33531629cbe82eef6810f982ec814e79d52a3b3`.

## 1. Make existing evaluation terms jointly trainable

NGN already has per-piece MG/EG mobility curves (`engine/eval.go:2435`) and
threat classes (`engine/eval.go:2585`). Counter uses those mechanisms too;
missing mobility or threats is not the explanation.

The actual limitation is in the optimizer's model. `engine/texel_gradient.go:137`
sets the mobility feature to zero; lines 175–178 fold both mobility and threats
into the fixed contribution. The nominal 798 parameters therefore include two
inactive mobility scalars, while the live per-count mobility curves and six
threat weights cannot move. Better loss on that model cannot demonstrate that
those coefficients improved. King safety exposes aggregate scaling weights,
not its individual attack, shelter or file coefficients.

Chess-3 uses a shared generic evaluator for integer play and floating-point
tuning (`eval/eval.go:15`, `tools/tuner/tuning/vector.go`), with mobility arrays,
threats and individual king-danger inputs in the fitted coefficient set.
Counter's classical tuner invokes its actual evaluator after setting weights.
This is a concrete implementation precedent for reducing model drift and
fitting more than material/PST plus aggregate scalars.

The substantial candidate is a verified expansion of NGN's model: expose the
existing mobility and threat coefficients, preserve current integer arithmetic
at the initial vector, and fit them jointly with material/PST. Removing dead
slots must be versioned so saved vectors cannot silently change meaning.
Use the repaired trace/filter/checkpoint contract; require perturbation parity
for every new parameter and game/opening-disjoint data with untouched validation.
Then compare a frozen fitted binary under real clocks. This is a new model/data
experiment, not another fit of the old parameter vector.

Evidence against assuming a gain: older large-corpus tunes sometimes improved
both training and validation loss without establishing better play. The saved
local corpora lack game IDs. Neither greater parameter count nor a lower MSE is
an Elo verdict; there is no basis for predicting a particular gain.

## 2. Add information about safe checking opportunities

NGN already applies nonlinear king danger (`engine/eval.go:1363`) and separate
shelter/file/material scaling (`engine/eval.go:825`). Its attack-pattern input
counts weighted attacks into a king zone; it does not distinguish potential
checking destinations defended by the opponent from undefended ones.

Chess-3 computes that partition by piece class (`eval/eval.go:132–163`) and
applies separate safe/unsafe coefficients (`eval/king_attacks.go:19–25`). These
are geometric opportunities, not a proof that every counted check is legal or
tactically sound. NGN qsearch covers captures and in-check evasions; it does not
generally search quiet checking moves.

Actual-function witness, same White pieces and Black king; move the black pawn
from g7 to h7:

```text
6k1/6p1/8/3N4/8/8/R7/1QB2K2 w - - 0 1
6k1/7p/8/3N4/8/8/R7/1QB2K2 w - - 0 1
```

| Evaluated component | Pawn g7 | Pawn h7 |
|---|---:|---:|
| NGN king-zone attack penalty | 24 | 24 |
| Chess-3 White-knight safe checking squares | 1 (e7) | 2 (e7, f6) |
| Chess-3 White-knight unsafe checking squares | 1 (f6) | 0 |

The g7 pawn attacks f6; the h7 pawn does not. Overlays instrument the actual
Chess-3 evaluator and invoke the actual NGN attack-pattern function. Both cases
pass NGN's check that the previous mover is not in check. This proves a feature
distinction absent from NGN's attack component. Other NGN terms can distinguish
the positions, so it does **not** prove identical whole-model evaluations, a
wrong move, tactical value or prevalence in lost games.

Reproduce without changing production files:

```sh
python3 experiments/2026-09-04-safe-check-witness.py
```

The script verifies the local Chess-3 revision and writes overlays/results under
`output/recovery-2026-09-04/safe-check-*`. Both actual-function tests pass.
Before promoting this to a playing candidate, find recurring disagreements in
game positions, check legal/tactical relevance, and fit the new inputs jointly
with existing king-danger terms. Do not paste the reference engine's weights.

## Lower-priority difference and limits

Chess-3 also separates safe-pawn threats from its generic threat class
(`eval/eval.go:405–440`), whereas NGN gives every pawn attack on an enemy non-pawn
the same pawn-threat coefficient. Chess-3 still scores unsafe pawn attacks in
the generic class; it does not simply discard them. Static attack masks do not
resolve pins or tactical recaptures. This narrower classifier has no demonstrated
loss incidence and ranks below the two targets above.

These are source-backed targets, not diagnoses of the entire rating gap.
The [tuning repairs](2026-09-04-tuning-contract-repair.md) explain specific
reproducible failures; June 21/22 bugs cannot explain earlier failed tunes.
NNUE would change representation while leaving search, data, clocks and model
integration to validate. The active objective remains 2800 without NNUE.
