# NGN Search — Design Spec & Theory

This folder is the **anchor specification** for NGN's tree search. It exists so that
search work is done *against a written contract* instead of from memory. When a change
or a bug report lands, you check it against the invariants here — you do not re-derive
the algorithm in your head each time.

It is deliberately close in spirit to the [Chess Programming Wiki](https://www.chessprogramming.org)
but specialized to NGN: every technique is described first in its canonical form (the
*target* you implement toward), then in NGN's actual form (with `file:line` anchors), and
then the **invariants** that must hold are stated explicitly and given a status.

## Why this exists

Two recurring failure modes motivated this spec:

1. **Eyeballed implementations.** Search techniques were implemented and tuned from
   memory. That works until two correct-looking pieces interact wrongly.
2. **The "joints."** The dangerous bugs in this engine have not been "the null-move pruner
   is wrong" or "quiescence is wrong" in isolation. They have been *interactions*: a
   technique that is locally correct but composes incorrectly with another. Examples found
   in this engine: NMP that silently never fired because PVS collapsed its window; IID/
   singular re-entrancy that returned a bogus draw because the repetition stack wasn't
   hidden; an LMR reduced-scout that searches at the wrong base depth so extensions are
   silently discarded. None of these is a single-technique bug. They live in the joints.

So this spec treats **invariants and joints as first-class** — see
[`09-invariants-and-joints.md`](09-invariants-and-joints.md), which is also the **intake
framework** for bug-hunt findings: every reported bug maps to an invariant and gets a status.

## How to read each technique doc

Every technique doc follows the same five-part template:

| Section | What it is | How to use it |
|---|---|---|
| **Purpose** | One line: the problem the technique solves. | Sanity: is this technique even pulling its weight? |
| **Canonical** | The textbook-correct algorithm + conditions + math. | **This is the spec you implement toward.** |
| **Invariants** | Numbered properties that MUST hold for soundness. | Each is testable. A bug is an invariant violation. |
| **NGN** | What NGN actually does, with `file:line`, params, divergences. | Current state. Compare against Canonical. |
| **Joints** | Which other techniques this one interacts with. | Pointers into doc 09. Read before changing anything. |

## Status tags

Invariants and joints carry one of four tags. (Plain text, no symbols.)

- **`[HOLDS]`** — verified against the code at the referenced lines; the invariant is upheld.
- **`[VIOLATED]`** — verified to be broken right now. This is the fix queue. Carries a finding ref.
- **`[ACCEPTED]`** — a known, deliberate divergence or an inherent limitation (e.g. the
  graph-history-interaction problem). Documented so it is never "rediscovered" as a bug.
- **`[UNVERIFIED]`** — canonical theory says X; NGN's behavior has not been confirmed either
  way. These are the audit targets.

A status without an `(audited <date>, <commit>)` note is an assertion, not a verification.
Treat unaudited `[HOLDS]` as `[UNVERIFIED]` until someone reads the lines.

## Layering (most-fundamental first)

```
01  architecture            the search contract: negamax, ID loop, the recursion frame, global state
02  transposition table     layout, bound semantics, replacement, mate adjust, TT-move validation
03  move ordering            the ordering bands and the heuristic tables that feed them
04  pruning & reductions     NMP, RFP, futility, LMP, SEE-prune, history-prune, probcut, LMR
05  extensions               check / recapture / passed-pawn / singular + the extension budget
06  quiescence               stand-pat, delta, SEE, in-check evasions, qsearch TT
07  mate, draw & scoring     mate-distance encoding, scoreTo/FromTT, repetition, 50-move, stalemate
08  time management          tournament soft/hard budgets, stability scaling, clock-interrupt discard
09  invariants & joints      the consolidated invariant table + the joint catalogue + finding intake
10  performance / NPS        the profiled hot-path breakdown + ranked NPS attack plan (the speed lane)
```

## Source-of-truth anchor

All `file:line` references in this spec are against the engine at the commit noted in each
doc's header. Line numbers drift; the surrounding code and function names are the durable
anchor. When in doubt, `grep` the quoted code, not the line number.

The core files this spec covers:

- `engine/search.go` — `alphaBetaPV` (main recursion), `quiescenceWithDepth`, the two root
  loops (`searchIterativeDeepeningUnsafe`, `searchFixedUnsafe`), SEE, the TT probe/store.
- `engine/cache.go` — the transposition table (`Cache`, `Pack`/`Unpack`, `Set`/`Get`).
- `engine/moveorder.go` — ordering and the history / continuation / capture / pawn-correction tables.
- `engine/time.go` — `TimeManager`, tournament budget math, stability scaling.
- `engine/position.go` — make/unmake, null move, repetition map vs search stack, draw rules.

## Status of this spec

Initial draft written 2026-05-31 against commit `87714a1`, grounded in a full read of the
five core files above plus two completed bug-hunt audits (pruning/reductions, TT/ordering/
state). The remaining audits (extensions/qsearch/mate-draw, fix-review) feed into doc 09 as
they land. This is a living document: when you fix a `[VIOLATED]` invariant, flip it to
`[HOLDS]` with the fixing commit; when you find a new joint, add it to doc 09.
