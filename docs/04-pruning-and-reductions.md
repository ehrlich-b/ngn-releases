# 04 — Pruning & Reductions

*Anchored to `engine/search.go` @ `87714a1`.*

The selective-search techniques: ways to *not* search (prune) or *search less* (reduce). Every one
trades soundness for speed; the **gates** are what keep the trade safe. A pruning bug is almost always
a missing or wrong gate.

---

## The universal gates

Three conditions recur as the safety rail on nearly every technique here. Treat them as the default;
a technique that omits one needs a reason.

- **`!isPV`** — never prune/reduce away exact-value resolution on the principal variation.
- **`!inCheck`** — when in check, the static eval is meaningless and you cannot "pass"; pruning on a
  meaningless eval drops forced replies.
- **depth bound** — most forward pruning is only sound at shallow remaining depth (the margin/horizon
  arguments break down deep).

NGN additionally uses **`move != ttMove`** on every quiet-move prune (never prune the hash move) and
**`legalTried`** (post-legality count), never the raw `moveCount`, for late-move gates.

---

## Null-Move Pruning (NMP)

**Purpose / Canonical.** If passing (making a null move) and searching shallower still fails high, the
position is so good a real move surely also fails high — prune. Reduce by `R`; verify with a
zero-width window at `beta`. **Must not fire** in check, on the PV, or in zugzwang (where passing is
*better* than any move — so require non-pawn material), and never two nulls in a row.

**NGN** (:1163): `!inCheck && depth>=3 && canNull && !isPV && staticEval>=beta && hasNonPawnMaterial`.
Reduction `R=3`; `nullDepth = depth-R-1` **floored at 0** so the base case routes through qsearch, not
a tactically-blind static eval (:1175). Null child called `canNull=false`, window `(-beta,-beta+1)`.
`return beta` on success.

- **`[INV-NMP1]` Fires at every cut node `[HOLDS]`** *(audited 2026-05-31, 87714a1)* — the "C3" fix: the
  old gate required a full window (`beta-alpha>1`), but PVS scouts every non-PV node with a null window,
  so NMP fired only on the PV spine (almost never). Now gated on `!isPV`. This was a ~+138-ELO latent bug;
  verified fixed and firing.
- **`[INV-NMP2]` Zugzwang-guarded `[HOLDS]`** — `hasNonPawnMaterial` blocks the null move in pawn-only
  endgames. (doc 09 J for NMP × eval.)
- **`[INV-NMP3]` No two nulls in a row `[HOLDS]`** — `canNull=false` threaded into the null child and reset
  to `true` on every real-move recursion (verified by the pruning/reductions audit).

---

## Reverse Futility Pruning (RFP / static null move)

**Purpose / Canonical.** If the static eval already exceeds beta by a depth-scaled margin, return early —
the side to move is so far ahead that a quiet search won't drop below beta. Non-PV, not in check, shallow.

**NGN** (:1139): `depth<=3 && !inCheck && beta-alpha==1`, margin `120*depth`; `return staticEval-margin`.

- **`[DIV-RFP1]` Gated on `beta-alpha==1` instead of `!isPV` `[VIOLATED]` (low impact)** — `beta-alpha==1`
  is a proxy for non-PV, but **mate-distance pruning (:1086) can collapse a genuine PV node's window to
  width 1**, letting RFP fire on the PV and truncate it. Reachability is low (the mate-distance collapse
  usually triggers its own `return alpha` at :1096 first). **Fix:** gate on `!isPV` to match the other
  techniques. *(finding: pruning/reductions agent af2f023f, 2026-05-31; verified mechanism, low trigger.)*
  See doc 09 J9.

---

## Futility Pruning (forward)

**Purpose / Canonical.** At shallow depth, a quiet move that even with a generous margin can't raise
alpha is hopeless — skip it. Non-PV, not in check. **Must exclude moves that change material/structure
unpredictably**: captures, promotions, and ideally checking moves.

**NGN** (flag at :1147, prune at :1301): flag set when `depth<=3 && !inCheck && !isPV && staticEval+margin
<= alpha` (margin `200 + (depth-1)*50`); the loop then skips `!move.IsCapture() && move != ttMove`.

- **`[DIV-FUT1]` Futility prunes quiet promotions `[VIOLATED]` (MEDIUM)** — the prune condition (:1301)
  checks `!IsCapture()` but **not `PromoType()==NoType`**. A quiet queen promotion has `IsCapture()==false`,
  so at a node judged hopeless (`staticEval+200<=alpha`) the ~+800 cp promotion is pruned on a static eval
  that still sees a pawn. The adjacent SEE-quiet block *does* guard promotions (:1338); futility doesn't.
  **Failing case:** a pawn-race/endgame where the side is behind on static eval but has a 7th-rank pawn that
  promotes to a winning queen — futility-pruned, the winning resource hidden. **Fix:** add
  `&& move.PromoType()==NoType` to :1301. *(finding: pruning/reductions agent af2f023f, 2026-05-31; verified.)*
  See doc 09 J10.
- **`[DIV-FUT2]` Futility does not exclude quiet checking moves `[ACCEPTED]`** — `givesCheck` isn't known
  until after `MakeMove` (:1406), but the prune happens before make (:1301). A cheap pre-make check test was
  tried and dropped (it missed discovered/EP/promotion checks). Standard-ish limitation; accept and document.

---

## Late Move Pruning (LMP) & History Pruning

**Purpose / Canonical.** Once enough moves have been tried at a shallow non-PV node without raising alpha,
the remaining late quiets are very unlikely to matter — skip them (LMP), or skip a quiet with very bad
history (history pruning). Move-count and history thresholds scale with depth and the `improving` flag.

**NGN.** LMP (:1309): `!isPV && !inCheck && 0<depth<8 && !IsCapture() && move!=ttMove && legalTried>0`,
prune when `legalTried > lmpThreshold[improving][depth]` (the `[2][8]` table at :74, ~2× sooner when not
improving). History pruning (:1348): `!isPV && !inCheck && depth<=3 && !IsCapture()`, skip when
`history<-1000 && legalTried>3`.

- **`[INV-LMP1]` Counts post-legal moves `[HOLDS]`** — uses `legalTried`, not `moveCount` (which includes
  illegal moves); table index `depth ∈ 1..7` into `[2][8]`, in-bounds.
- **`[DIV-LMP1]` LMP/history-prune also lack the promotion guard `[VIOLATED]` (LOW)** — same missing
  `PromoType()==NoType` as FUT1, but near-unreachable: queen promos score 90000 in ordering (doc 03), so they
  are tried at small `legalTried`, before any late threshold. Add the guard for consistency. *(finding:
  pruning/reductions agent af2f023f, 2026-05-31.)*

---

## SEE Pruning (in the main search)

**Purpose / Canonical.** Use static-exchange evaluation to skip captures/quiets that lose material on
their target square, at shallow non-PV nodes.

**NGN.** Bad captures (:1324): `!isPV && !inCheck && depth<=4 && IsCapture() && move!=ttMove`, skip if
`SEE < -100`. Quiet "soft hangs" (:1337): same gates plus `!IsCapture() && move!=ttMove &&
PromoType==NoType` and excluding kings, skip if `SEE < -80*depth`.

- **`[INV-SEE1]` Sign + exclusions correct `[HOLDS]`** *(audited 2026-05-31)* — negative SEE = material
  loss; the quiet block correctly excludes king moves (infinite SEE weight poisons the swap) and promotions.
  The quiet block has no move-count floor (prunes from the first move) — aggressive but gated to non-PV and
  documented (commit 5b19b75). Verified by the pruning/reductions audit.

---

## Probcut

**Purpose / Canonical.** If a capture, searched at reduced depth against a raised beta (`beta+margin`),
fails high, the full search almost certainly also fails high — prune. Non-PV, not in check, deep.

**NGN** (:1192): `!isPV && !inCheck && depth>=5 && abs(beta)<MATE_VALUE-100`; `probcutBeta=beta+200`;
captures with `SEE>=0`; qsearch verify at `-probcutBeta`, then a `depth-4` search; `return beta` on success.

- **`[INV-PC1]` Sign + return-bound correct `[HOLDS]`** *(audited 2026-05-31)* — window math negates
  correctly; `return beta` (not probcutBeta) is the conservative valid fail-high. Verified by the
  extensions/qsearch audit.

---

## Late Move Reductions (LMR)

**Purpose / Canonical.** Late, quiet, non-checking moves are unlikely to be best — search them shallower
first (a reduced null-window scout). If a reduced scout beats alpha, it may have been under-counted, so
**re-search at full depth**. The reduction grows with depth and move number (log-log), and is nudged by
PV-ness, the improving trend, history, and killer status.

**NGN** (:1502-1557): applies when `depth>=3 && legalTried>3 && !IsCapture() && !inCheck && !givesCheck`.
Base `reduction = lmrTable[depth][legalTried]` (= `floor(log d · log m / 2)`, :108); `+1` if `!isPV`; `+1`
if `!improving`; `+1` for bad history (`<-500`), `-1` for good (`>1000`), `-1` for a killer; capped so
`reducedDepth >= 1`.

- **`[INV-LMR1]` Reduced depth stays ≥ 1 `[HOLDS]`** *(audited 2026-05-31)* — the cap `reduction >= depth-1
  ⇒ reduction = depth-2` and the `reduction<0 ⇒ 0` floor keep `reducedDepth = depth-1-reduction >= 1`, so a
  reduced move never accidentally drops into qsearch. Verified by the pruning/reductions audit.
- **`[INV-LMR2]` Captures and checking moves are not reduced `[HOLDS]`** — the gate excludes them. This is
  *also the root of the most important joint in the engine* (J1): because these moves get `reduction==0`,
  the reduced-scout path computes their depth wrongly. See next.
- **`[VIOLATED] (HIGH) — see J1 / doc 05`** — the reduced scout is launched at `reducedDepth = depth-1-
  reduction` (:1560), based on `depth-1` **not** the extension-extended `nextDepth`. For LMR-excluded moves
  (captures, checks: `reduction==0`) off the PV, the scout runs at `depth-1`, the `reduction>0` re-search
  (:1575) is skipped, and the `isPV` re-search (:1580) is skipped — so the check/recapture extension is
  **computed and counted but never applied**. Confirmed independently by two audits. The dead
  `if reducedDepth>=depth` branch (:1561) is a genuine no-op and must stay (do not "clean it up"). Full
  treatment and the fix (`reducedDepth := nextDepth - reduction`) in **doc 05** and **doc 09 J1**.

---

## Ablation harness

`SearchToggles` (:84) flips each technique off one at a time for `TestPruningAblation`: a technique whose
removal makes the engine solve *more* tactics is over-pruning real refutations (a correctness bug), not just
trading accuracy for speed. This is the cheap, deterministic guard for the soundness of everything above —
**run it after touching any gate here.**

---

## Joints this opens (doc 09)

- **J1** — LMR reduction × extensions (the reduced-scout base-depth bug). The headline.
- **J2** — NMP/RFP/futility all read `staticEval`, which "S7" overwrites with a TT score (doc 02/07).
- **J9** — RFP/mate-distance window collapse (DIV-RFP1).
- **J10** — futility/LMP/history-prune × promotions (DIV-FUT1, DIV-LMP1).
