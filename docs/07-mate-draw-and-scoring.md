# 07 — Mate, Draw & Score Encoding

*Anchored to `engine/search.go` and `engine/position.go` @ `87714a1`.*

The terminal-value bookkeeping. Small here, but the source of subtle bugs because mate scores are
*ply-relative* and draw scores are *path-relative* — two properties that fight the position-keyed TT.

---

## Mate scores

**Canonical.** A checkmate is scored `±(MATE_VALUE − distance_to_mate_in_plies)` so that, among winning
lines, the engine prefers the *fastest* mate (and the *slowest* loss). "Distance" must be measured from
the node reporting it. The fixed offset band `[MATE_IN_MAX, MATE_VALUE]` marks "this is a mate score, not
a normal eval."

**NGN.** `MATE_VALUE = 30000`, `MATE_IN_MAX = MATE_VALUE − 1000` (search.go:12). Checkmate at a node is
`-MATE_VALUE + ply` (:1261, :1651, qsearch :1775) — `ply` is the distance from root, so deeper mates score
lower (less preferred), correct. `abs(score) >= MATE_IN_MAX` means "mate score."

- **`[INV-M1]` Mate distance uses `ply`, not a stale field `[HOLDS]`** *(audited 2026-05-31, 87714a1)* — an
  earlier version used `info.Depth − depth`, but `info.Depth` is only set *after* an iteration completes, so
  mid-tree it was wrong. Now `ply` directly. Both audits confirmed.

---

## Mate scores in the TT (the ply-adjustment joint, J5)

A mate score stored at one ply and read at another would report the wrong distance. So it is stored
*node-relative* and converted on the way in/out:

```go
scoreToTT(score, ply):   if score>= MATE_IN_MAX { return score+ply }   // (:117)
                         if score<=-MATE_IN_MAX { return score-ply }
scoreFromTT(score, ply): inverse                                       // (:129)
```

- **`[INV-M2]` Every TT store/read mate-adjusts `[HOLDS]`** *(audited 2026-05-31)* — all three stores (:1633,
  :1673, :1869) use `scoreToTT(_, ply)`; both reads (:1036, :1708) use `scoreFromTT(_, ply)`. No raw mate score
  is ever stored or compared. Confirmed by two audits. (doc 02 INV-TT3.)

---

## Mate-distance pruning

**Canonical.** No mate found at this ply can be faster than `mate-in-(ply)`, nor slower-loss than
`mated-in-(ply)`; tighten `[alpha,beta]` to that band and cut if it collapses. Prunes pointless deeper
mate searches.

**NGN** (:1086): `mateAlpha = -MATE_VALUE+ply`, `mateBeta = MATE_VALUE-ply-1`; raise alpha / lower beta to
them; `alpha>=beta ⇒ return alpha`.

- **`[INV-M3]` Uses `ply`, collapse returns a valid bound `[HOLDS]`** *(audited 2026-05-31)* — verified by the
  extensions/qsearch audit.
- **`[CAVEAT]`** the window collapse can make a genuine PV node width-1, which trips RFP's `beta-alpha==1`
  proxy — see doc 04 DIV-RFP1 / doc 09 J9.

---

## Draw detection

Three draw types, checked **before the TT probe and before move generation** (:996-1024), each returning
`0` *immediately* (so a path-dependent draw is never stored in the TT):

1. **50-move** — `HalfMoveClock >= 100` (half-plies). (:998)
2. **Game-history threefold** — `Positions[hash] >= 2` means this occurrence is the 3rd. (:1003) The
   `Positions` map is maintained only by `GameMakeMove`/`GameUnMakeMove` (the real game line), not the
   search's `MakeMove`. (position.go:166)
3. **Search-path repetition** — `hash` already on `RepStack` (ancestors only; the scan precedes the push).
   A 2-fold *within the search* is treated as a draw — standard heuristic (the second occurrence in search
   implies a forced cycle). (:1013)

Then `hash` is pushed onto `RepStack` and popped via `defer` (doc 01 INV-A3).

**At the root**, draw adjudication uses strict FIDE threefold (`IsFIDEDrawRule`, value `>= 3`,
position.go:441) — the game map and the search stack are disjoint, so no double-counting.

**Insufficient material** (`IsDraw`, position.go:375) — K vs K, K+minor vs K, KB vs KB same-color — is
handled by eval/adjudication, not probed in-tree. Standard.

- **`[INV-D1]` Draw scores are never stored in the TT `[HOLDS]`** *(audited 2026-05-31, 87714a1)* — all three
  draw returns are `return 0` *before* any `TranspositionTable.Set`. Both audits confirmed. This is the local
  half of the GHI mitigation (J4).
- **`[INV-D2]` Stalemate scores 0 regardless of material `[HOLDS]` (main search)** — `legalTried==0 && !inCheck
  ⇒ return 0` (:1266, :1654). Scoring it by material would break the zero-sum invariant (the opponent reads the
  same position oppositely and steers into it). *(But see doc 06 DIV-Q1: this fails in qsearch below qDepth==0.)*
- **`[INV-D3]` Game map and search stack are disjoint `[HOLDS]`** *(audited 2026-05-31)* — verified by the
  TT/ordering/state audit; no repetition is double-counted.

---

## J4 — the graph-history-interaction (GHI) limitation `[ACCEPTED]`

The TT is keyed by position hash alone, but whether a position is a draw can depend on the *path* taken to
reach it (a repetition draw exists on one move order but not another). NGN mitigates the worst case by
**never storing the draw score at the repetition node** (INV-D1), so a drawn leaf doesn't directly pollute
the table. But a *parent's* stored score can still be tainted: if a node's best line ran into a repetition
scored 0, the node stores a score reflecting that path-specific draw, and that score may later be reused for
the same position reached by a different path where the repetition isn't available.

This is the classic GHI problem. A full fix needs path-aware hashing (irreversible-move counters in the key,
or excluding draw-influenced scores from storage). NGN accepts the residual error — it is small in practice
and standard across non-trivial engines. **Documented so it is never re-filed as a fresh bug.**

---

## Joints this opens (doc 09)

- **J4** — TT vs path-dependent draws (above). ACCEPTED.
- **J5** — mate scores vs TT ply-adjustment vs aspiration windows (INV-M2/M3).
- **J3** — the RepStack push/pop and its hiding during IID/singular re-entry (doc 01/05).
