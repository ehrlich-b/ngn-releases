# 06 — Quiescence Search

*Anchored to `engine/search.go` `quiescenceWithDepth` (:1682) @ `87714a1`.*

---

## Purpose

At `depth==0` you cannot just return the static eval — the position may be mid-capture-sequence (the
*horizon effect*: a hanging queen one ply past the horizon reads as material-up). Quiescence keeps
searching **forcing moves** (captures, and evasions when in check) until the position is "quiet," then
returns the static eval. It gives alpha-beta a stable leaf value.

---

## Canonical

- **Stand-pat.** When *not* in check, the side to move can always decline to capture, so the static eval
  is a *lower bound* on the value: if `standPat >= beta`, fail high immediately; else `alpha = max(alpha,
  standPat)` and search captures that might raise it further.
- **In check ⇒ no stand-pat.** You cannot pass out of check, so you must search *all evasions* (not just
  captures) and you cannot use the static eval as a floor. A node with no legal evasions is checkmate.
- **Delta pruning.** Skip a capture whose best-case material gain (captured piece + promotion bonus) plus a
  margin still can't reach alpha — it can't matter.
- **SEE pruning.** Skip captures that lose material on the target square (`SEE < 0`).
- **Bounded.** A depth/ply cap stops pathological capture chains.

---

## Invariants

- **`[INV-Q1]` No stand-pat while in check `[HOLDS]`** *(audited 2026-05-31, 87714a1)* — the in-check branch
  (:1729) searches all generated evasions (full move gen, not captures-only), never computes/returns a
  stand-pat, inits `bestScore=-INFINITY`, and returns `-MATE_VALUE+ply` when no evasion is legal. Verified by
  the extensions/qsearch audit. This is the eval × check-detection × qsearch joint, handled correctly.
- **`[INV-Q2]` Delta/SEE pruning is sign-correct and promo-aware `[HOLDS]`** *(audited 2026-05-31)* — delta
  (:1843) `standPat + gain + DELTA_MARGIN <= alpha` includes the promotion bonus in `gain` and the captured
  pawn for en-passant (so a real `Pxf8=Q` or an EP capture isn't spuriously pruned); SEE skip (:1848) is
  `< 0`. Verified by the extensions/qsearch audit.
- **`[INV-Q3]` Captures-only when not in check `[HOLDS]`** — `GenerateCapturesIntoBuffer` (:1816). Quiet checks
  are deliberately dropped: `moveGivesCheck` misses discovered/EP/promotion checks, and an incomplete
  quiet-check path is worse than captures-only at the horizon. Deliberate.
- **`[INV-Q4]` Bounded recursion `[HOLDS]`** — `qDepth >= 6` returns the static eval (:1687); SEE/delta and the
  ply ceiling (INV-A6) bound it further.
- **`[INV-Q5]` qsearch TT stores can't poison the main search `[HOLDS]`** *(audited 2026-05-31)* — qsearch stores
  only a fail-high LowerBound at depth 0 (:1869), with `bestMove` already set to the cutting capture (:1863);
  a depth-0 entry can never satisfy a main-search cutoff (`ttDepth>=depth`, `depth>=1`), and depth-preferred
  replacement protects deeper entries. Verified by two audits. (doc 02.)

---

## NGN

`quiescence(pos, α, β, ply, info)` → `quiescenceWithDepth(…, qDepth=0)`. Flow:

1. **Seldepth** (:1683): `d := ply + qDepth` — see DIV-Q2 (over-counts).
2. **qDepth cap** (:1687): `>=6` → static eval.
3. **Node-budget / UCI-stop** guards (:1692-1700).
4. **TT probe** (:1706): cut on Exact / `LowerBound>=beta` / `UpperBound<=alpha`; keep `ttMove` for ordering.
5. **In check?** (:1726). If yes → the evasion branch (:1729): generate all moves, front-swap `ttMove`,
   search every legal evasion with a full window, return `bestScore` (or mate).
6. **Not in check.** At `qDepth==0` only, detect stalemate (:1782) → return 0. Then **stand-pat** (:1801):
   `standPat>=beta` → return beta; else `alpha=max(alpha,standPat)`.
7. **Captures** (:1816): copy into a local buffer, front-swap `ttMove` if it's a capture (no full MVV/LVA
   sort — that regressed −129 ELO, commit 2f61d3a); for each: delta prune, SEE prune, legality, recurse; on
   fail-high store the depth-0 LowerBound and return beta; else raise alpha. Return `bestScore`.

Constants: `DELTA_MARGIN = 100`, qDepth cap `6`.

---

## Divergences & status

- **`[DIV-Q1]` Stalemate is scored as the static eval below `qDepth==0` `[VIOLATED]` (LOW–MED)** — the
  not-in-check stalemate detection runs **only at `qDepth==0`** (:1782). At `qDepth>0`, a side-to-move that is
  stalemated (no legal moves, not in check) falls through stand-pat → no/only-illegal captures → returns
  `bestScore == standPat` (the material eval) at :1878, **not the drawn `0`** — same wrong-score class as the
  old "qsearch stalemate = −500" bug, different sentinel. **Failing case:** a pawn-endgame line where a capture/
  promotion in qsearch leaves the opponent stalemated; qsearch returns "+a piece" for a draw, the parent steers
  into it. Rare (needs a true stalemate reachable purely through a capture chain). **Fix:** accept as a documented
  horizon limitation, or in the captures path, if no pseudo-legal capture exists and a cheap legal-move probe
  finds none while not in check, return 0. *(finding: extensions/qsearch agent a4f0b130, 2026-05-31; verified.)*
- **`[DIV-Q2]` seldepth double-counts in qsearch `[VIOLATED]` (cosmetic)** — `d := ply + qDepth` (:1683) over-
  reports because the recursion bumps **both** `ply` and `qDepth` (:1756, :1858), so `ply` already encodes the
  qsearch distance (reports `P+2k` at qsearch level k). `SelDepth` feeds only the UCI `seldepth` string — never
  pruning/scoring/indexing — so this is a misreported statistic, not a correctness bug. **Fix:** `if ply >
  info.SelDepth { info.SelDepth = ply }` to match the main search (:942). *(finding: a4f0b130, 2026-05-31.)*
- **`[DIV-Q3]` In-check qsearch fail-high doesn't store to TT `[ACCEPTED]`** — only the captures branch stores
  (:1869); the evasion branch (:1763) returns beta without a store. Asymmetric but a missed store, not a bug.

---

## Joints this opens (doc 09)

- **J11** — qsearch stand-pat × in-check (INV-Q1): the must-not-pass-out-of-check joint. HOLDS.
- **J4** — qsearch TT stores feed the same table the main search reads (INV-Q5 / doc 02).
- **J12** — qsearch leaf eval × the `improving`/static-eval reuse in the parent (doc 04/07).
