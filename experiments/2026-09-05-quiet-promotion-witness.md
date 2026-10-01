# Quiet promotions: reproduced qsearch horizon omission

**Diagnostic only; no playing change or game candidate is installed or queued.**
The accepted T8 runtime remains `5cf301a870f6cd9218d8085166993f7ad5364aee`.
This follows the [earlier promotion audit](2026-09-05-classical-review-update.md)
with concrete tactical witnesses, rather than rediscovering the omitted move
class or repeating the capture-underpromotion delta probe.

## What is now reproduced

On `7k/5P2/6K1/8/8/8/8/8 w - - 0 1`, `f7f8q` is legal mate in one.
Its color mirror is `8/8/8/8/8/6k1/5p2/7K b - - 0 1`, with `f2f1q` mate.
Full-window quiescence on the actual accepted search source returns +464 cp
after one node on both. A diagnostic that adds quiet promotions returns the
mate score +29999 after two nodes. Delta, SEE, TT, legality and other policies
are unchanged.

| Position | Accepted T8 qscore / nodes | Diagnostic qscore / nodes |
| --- | --- | --- |
| White promotion mate | +464 / 1 | +29999 / 2 |
| Black promotion mate | +464 / 1 | +29999 / 2 |
| Existing knight-fork fixture | -193 / 1 | +553 / 2 |

The fork fixture is `8/2k1P3/5q2/8/8/8/8/4K2R w - - 0 1`.
Its score change demonstrates entry into the promotion class; it does not
establish that the knight promotion was searched by qsearch or that the
remaining delta/SEE policy fully resolves underpromotions.

This is a reproduced missing forcing move at the selected horizon. It is **not
a demonstrated root blunder or Elo gain**. Ordinary depth-one root search in
both versions already finds the two mating promotions. On the bounded checked
predecessor `6k1/5P2/6K1/8/8/8/8/8 b - - 0 1`, both NGN variants choose
`g8f8`; the corrected Stockfish depth-two line also starts with that move.

## Source and independent checks

Artifacts are under `output/qpromo-witness-20260905/`. A fresh archive of
parent `f8e1d2b` supplies `base/`; `diagnostic/` changes only `engine/search.go`.
Root compared all158 tracked Go/module files: baseline equals the current
playing source, and only that one production file differs in the diagnostic.
Both archives have the same additional output-only witness test.

Root reran both probes in fresh processes and asserted the actual scores and
node counts above, rather than relying on the probe's classification-only PASS.
Evidence: `root-source-and-score-validation.json` and the two
`root-*-qsearch.log` files.

The first Stockfish transcript was incomplete: `quit` was queued alongside the
asynchronous search, leaving bestmoves without completed depth/score evidence.
Root rejected those apparent searched preferences. The preserved original
`stockfish-proof.log` and README are superseded for that claim by
`FOLLOWUP-STOCKFISH-PROOF-CORRECTION.md` and
`stockfish-proof-synchronized.log`. The replacement waits for UCI readiness,
perft totals and completed search. Both mirrors reach depth2 with mate1; after
the promotions, Stockfish reports a checker and zero legal replies. Root read
the driver and actual transcript. Stockfish is an independent rules oracle;
no NNUE is added to NGN.

Key SHA-256 identities:

- Accepted search source:
  `f82cd4e1e245628188025e26945a0f7029fa5340dadf694246d880e1f184a6ed`.
- Diagnostic search source:
  `4c59e6915b68f04817744e0529a4300bcab469dbdc6a265adf01ae79a6a21d10`.
- `quiet-promotions-only.patch`:
  `8d695c3ab4f3b1b50302015685ac0e16c9de91af57dd867c9b4428850be9413d`.
- Synchronized Stockfish transcript:
  `7c315d76e967ecdd4bba71687a9a758f6d293c48de262b449527425eaaf022df`.

## Reference mechanism and implementation constraint

The actual hot capture generator is `GenerateCapturesIntoBuffer` at
`engine/movegen.go:875`; it is separate from the allocating capture filter.
Qsearch calls it at `engine/search.go:2456`. Both this specialized generator
and qsearch's selection/score arrays are capped at64. Simply appending noisy
moves risks silently truncating them.

The diagnostic preserves the original64-entry capture generation and ordering,
copies those captures into a256-entry local array, and appends quiet promotions
from the full generator. It also widens the parallel score array. This isolates
the move-class issue on the witness positions, but generates all moves at every
non-check qnode and is **not a performance-ready transplant**.

Pinned Counter 3.8 includes quiet queen promotions in its capture generator
(`common/movegen.go:232`); pinned Chess-3 v4 includes all four quiet promotions
in its noisy generator (`movegen/movegen.go:101`). Their exact revisions remain
in the [review prompt](2026-09-04-classical-review-prompt.md). A future isolated
implementation should use a promotion-rank/empty-square check, preserve the
existing capture path and provide sufficient separate promotion capacity.
The references differ on underpromotions and pruning; copying a feature name
does not settle those interactions.

## Recorded-game prevalence and next decision

Root counted actual quiet promotion moves in the previously independently
replayed `r0904pin.pgn`, verifying its frozen SHA before the scan. There are
**206 quiet promotions in135 of200 games**:159 by NGN and47 by opponents.
Of the opponent promotions,37 occur in games NGN lost. The raw events and
grouped counts are in `root-historical-incidence.json`.

These are played moves, not search-frontier incidence or causal attribution.
Many promotions occur after an earlier decisive mistake; this count cannot
assign Elo to the omission. It establishes that promotion play is common in
the existing measurement corpus and supports a bounded implementation test.
Any efficient candidate still needs correctness/performance checks and its
own frozen game gate. It must not be bundled into the running singular-T8
experiment or the predeclared accepted-engine external pin.

All337 frozen inputs for the current game/rating handoff were reverified
unchanged after these Mac-only diagnostics. No Windows/WSL engine work ran.
