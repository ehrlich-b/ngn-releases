# 2026-07-01 M1 — make the two dead SPSA knobs live (node-identical)

Revamp mill-item M1 (`2026-07-01-method-revamp.md`). `TunableSearchParams` (the
cmd/spsa dimension list) advertises 11 knobs, but 2 were VESTIGIAL: `NullMoveR`
(`NULL_MOVE_R`) and `SingularMargin` (`SINGULAR_MARGIN`). SPSA still emitted
`setoption` for them each iteration, so it was wasting 2 of 11 dimensions tuning
phantoms with zero effect on the engine — a measurement defect that also weakens
any joint-SPSA (the revamp's T8).

## Not "lazily hardcoded" — deliberately superseded

The audit's "hardcoded elsewhere" framing undersells it. Both params became dead
because their formulas were intentionally upgraded from flat to depth-scaled (code
comments cite "CG/Ethereal/Weiss converge"):

- NMP (`search.go`): `nullDepth := depth - (4 + depth/6)`  — "was flat NULL_MOVE_R+1=4"
- Singular (`search.go`): `singularBeta := ttEval - 2*depth` — "was flat SINGULAR_MARGIN=64"

So the correct fix is not to revert to flat, but to RE-HOME each param as the
tunable BASE of its (kept) depth-scaled formula.

## Change (node-identical at default)

- `nullDepth := depth - (NULL_MOVE_R + 1 + depth/6)` — at `NULL_MOVE_R=3` this is
  `depth - (4 + depth/6)`, exactly the prior formula. `NullMoveR` now tunes the NMP
  base reduction over its advertised [2,5].
- `singularBeta := ttEval - (SINGULAR_MARGIN*depth)/32` — at `SINGULAR_MARGIN=64`
  this is `ttEval - 2*depth` exactly (64/32=2, integer for every depth).
  `SingularMargin` now scales the per-depth singular margin over [16,256] → 0.5–8
  cp/ply.

No other change. Both read package globals already wired to `setoption` via
`TunableSearchParams`, so the knobs are now live end-to-end.

## Evidence

- Node-identity gate (`scripts/nodecheck.sh`): HEAD and HEAD+M1 both produce
  kiwipete d12 **270446**, mid d12 **81006**, end d16 **840033** — EXACT match, so
  the default search tree is byte-identical (the change is [NEUTRAL], not
  [BEHAVIOUR]).
- `go test -short ./engine -count=1` green (3.13s).
- Verified against clean HEAD with the iir3 working-tree patch stashed, then
  restored; M1 committed alone.

## Next

T8 (SPSA round 1, post-M4) now tunes 11 REAL dimensions. `NullMoveR` and
`SingularMargin` are exactly the kind of base-reduction / extension-margin knobs
strong engines SPSA-tune, so the 2 recovered dims are useful, not filler. Verdict
on the converged vector is GAMES, never the SPSA plus_pct.
