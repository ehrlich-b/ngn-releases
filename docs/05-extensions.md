# 05 — Extensions & the Extension Budget

*Anchored to `engine/search.go` @ `87714a1`.*

Extensions are the opposite of reductions: search *deeper* on forcing/critical lines so a tactic
isn't missed at the horizon. They are dangerous because a non-decaying extension (one that keeps
restoring depth) can recurse without bound — hence the budget. **This doc also contains the single
most consequential bug the audits found (J1): extensions and LMR do not compose correctly.**

---

## Canonical

Common extensions (each adds ~1 ply to the child):

- **Check extension** — the side to move is in check (or the move gives check): forced, extend.
- **Recapture extension** — recapturing on the square the opponent just captured on: forced-ish.
- **Passed-pawn / promotion extension** — a pawn pushed near promotion.
- **Singular extension** — the TT move is *singular* (all alternatives fail low by a margin below the
  TT score), so it carries the line — extend it. Verified by a reduced-depth search of the node with
  the TT move **excluded**.

**Soundness rules:**
1. **Bound the per-node extension** (typically ≤ +1 net) so a node can't explode locally.
2. **Bound the cumulative path extension**, because a check extension only *restores* the decrement
   (net 0 along a checking line) — a perpetual check or king hunt never bottoms out. Without a path
   cap the search recurses toward the ply ceiling (seldepth-explosion).
3. **The extended depth must actually be the depth searched.** An extension that is computed but not
   used is a no-op (worse: a misleading counter). This is exactly where NGN breaks — see J1.

---

## NGN

Extensions accumulate into `nextDepth` (base `depth-1`) inside the move loop:

| Extension | Site | Condition | Effect |
|---|---|---|---|
| Check | :1408 | `givesCheck && depth>1` | `nextDepth++`, `CheckExtensions++` |
| Recapture | :1418 | `move.IsCapture() && savedLast.IsCapture() && same destination && nextDepth+1<=depth` | `nextDepth++` |
| Passed pawn | :1426 | pawn push to rank ≥6 (W) / ≤2 (B), `&& nextDepth+1<=depth` | `nextDepth++` |
| Singular | :1438 | `move==singularMove` and the verification search fails low | `nextDepth++` |

Then two caps:
- **Per-node guardrail** (:1473): `nextDepth > depth+1 ⇒ nextDepth = depth+1`.
- **Per-path budget** (:1488): `maxNextDepth = RootDepth - (ply+1) + EXTENSION_BUDGET(24)`; clamp
  `nextDepth` to it. Rationale: a node's net extensions along the path `== depth + ply - RootDepth`;
  capping `nextDepth` keeps the child's net `<= 24`. Once spent, checks decrement normally and the line
  terminates.

**Singular detection/verification** (:1243 detect, :1438 verify): only when `ttHit && ttMove!=EmptyMove
&& depth>=SINGULAR_DEPTH(6) && ExcludedMove==EmptyMove && (Exact||LowerBound) && ttDepth>=depth-3`. The
verification unmakes the move, sets `ExcludedMove/ExcludedPly`, re-searches the node at `depth-4` with the
move excluded and the RepStack entry hidden, then re-makes. Extends if `singularScore < ttEval - 64`.

---

## Invariants

- **`[INV-EXT1]` Per-node net extension ≤ +1 `[HOLDS]`** *(audited 2026-05-31, 87714a1)* — the guardrail
  at :1473 caps `nextDepth` at `depth+1`; check & recapture are mutually exclusive (`nextDepth+1<=depth`
  blocks recapture once check fired), recapture & passed-pawn are mutually exclusive (capture vs non-capture),
  and only singular stacks on top — capped at +1. Verified by the extensions/qsearch audit.
- **`[INV-EXT2]` Cumulative path extension ≤ EXTENSION_BUDGET `[HOLDS]` (on the ID path)** *(audited
  2026-05-31)* — the budget clamp is a provable upper bound: by induction over every recursion site
  (first-move, reduced scout, both re-searches, null, probcut, IID, singular), each child satisfies
  `depth+ply-RootDepth <= 24`, so `maxNextDepth >= depth-1` always — the clamp **only ever denies an
  extension, never truncates a normal move**. Drives `nextDepth` to qsearch/static-eval only at the very
  bottom of a 24-deep forcing line that was terminating anyway. Brute-forced by the fix-review audit;
  consistent with node-identical Kiwipete d10-12 (the clamp can't bind until seldepth ≥ 25).
- **`[INV-EXT3]` Every budget-using entry point sets `RootDepth` `[VIOLATED]` (MEDIUM-LOW)** — see INV-A5
  (doc 01). `searchFixedUnsafe` (:849) and `alphaBeta` (:937) never set `RootDepth`, leaving it 0, so the
  clamp becomes `23 - ply` and **wrongly truncates any fixed-depth search past ~depth 24** (e.g. depth-30
  search clamps its first child 28→22; at ply≥24, `maxNextDepth` goes negative → child gets a raw static
  eval). Shipping UCI/ID play is unaffected; `cmd/trace_move` and deep fixed-depth tests are. **Fix:**
  `info.RootDepth = depth` in `searchFixedUnsafe`. *(confirmed by fix-review af64cc38 AND extensions/qsearch
  a4f0b130, 2026-05-31.)*
- **`[INV-EXT4]` Singular verification doesn't corrupt state `[HOLDS]`** *(audited 2026-05-31)* — unmake →
  verify (RepStack entry hidden, `inSingular` suppresses the cutoff and both TT stores) → remake; the
  re-made `ep/tag/hc` feed the loop-tail unmake; `RepStackLen--/++` balanced. The historical singular
  re-entrancy / TT-self-cutoff bug is fixed. Verified by two audits. (Latent: the `RepStackLen` straddle is
  panic-unsafe — doc 09 J3; and singular doesn't exclude mate-scored TT entries — efficiency only.)

---

## J1 — the headline joint: extensions × LMR reduced-scout base `[VIOLATED]` (HIGH)

**This is the bug your "it's not the null pruner, it's not the qsearch, it's both together" instinct was
pointing at.** It is not in the extension code (which correctly computes `nextDepth`) and not in the LMR
code (which correctly computes `reduction`). It is in how they compose.

**Mechanism.** For a non-first move (`legalTried>=2`), the loop does (:1559-1582):

```go
reducedDepth := depth - 1 - reduction          // (:1560) base is depth-1, NOT nextDepth
score = -alphaBetaPV(pos, reducedDepth, ...)    // (:1566) null-window scout
if reduction > 0 && score > alpha {             // (:1575)
    score = -alphaBetaPV(pos, nextDepth, ...)   //         re-search at the EXTENDED depth
}
if isPV && score > alpha && score < beta {      // (:1580)
    score = -alphaBetaPV(pos, nextDepth, ...)   //         PV full-window re-search at nextDepth
}
```

LMR (:1503) excludes captures and checking moves (`!IsCapture() && !givesCheck`), so for those moves
`reduction == 0`. With `reduction == 0`:
- the scout (:1566) runs at `reducedDepth = depth - 1` — the **un-extended** depth;
- the `reduction > 0` re-search (:1575) is **skipped**;
- the `isPV` re-search (:1580) is **skipped in non-PV nodes**.

So a **checking move** (`nextDepth == depth`) or a **recapture** (`nextDepth == depth`), when it's not the
first move and we're off the PV, is searched at `depth-1`. The extension that was computed into `nextDepth`
— and counted in `CheckExtensions`/`RecaptureExtensions` — **is never applied.** The in-code comment at
:1572 ("the check extension in nextDepth never actually applied off the PV") describes the gap but the C4
fix only closes it for `reduction>0` moves, which are exactly the moves that are *not* checks/recaptures.

Only **passed-pawn pushes** (LMR-eligible quiets) and the **singular move** (TT move, ordered first → uses
`nextDepth` via the first-move path :1497) actually get extended off the PV today.

**Why it's insidious:** the score returned is a *valid* alpha-beta value — just at a shallower depth than
the code intends. No crash, no wrong-score assertion, no tactical-suite failure. It only shows up as the
engine searching one ply shallower than its own extension logic claims, across the dominant (non-PV) part
of the tree — and as the extension counters lying. It directly undercuts the C4 commit's headline effect,
so **the C4 SPRT is measuring a fix that is a no-op for its primary case.**

**Status / confidence:** `[VIOLATED]`, HIGH confidence — verified independently by the pruning/reductions
audit (af2f023f) and the extensions/qsearch audit (a4f0b130), and re-verified by the lead against the code.

**Fix (to apply one at a time, gated):** base the reduced depth on the extended depth —
`reducedDepth := nextDepth - reduction` (:1560). Then a check (`reduction==0`) gets `reducedDepth == nextDepth
== depth` (extended); a reduced quiet keeps `nextDepth-reduction`. Keep the dead `if reducedDepth>=depth`
branch. The per-path budget (INV-EXT2) already bounds the extra check recursion this opens. **Verify:** depth-10
Kiwipete node/seldepth vs clean HEAD (expect a node increase concentrated on checking lines, seldepth still
≤ RootDepth+24), `make smoke-tactical`, `go test ./engine/`, then re-run the C4-style SPRT — this is the one
correctness fix here likely to move strength, so it earns a real (fixed-time) measurement.

---

## Joints this opens (doc 09)

- **J1** (above) — extensions × LMR. HIGH, VIOLATED.
- **J3** — singular/IID re-entrancy × RepStack.
- **J8** — extension budget × `RootDepth` entry-point setup (INV-EXT3 / INV-A5).
- **J5** — singular's `singularBeta = ttEval-64` reads a TT score that should exclude mate values (efficiency).
