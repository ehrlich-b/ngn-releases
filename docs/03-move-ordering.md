# 03 — Move Ordering

*Anchored to `engine/moveorder.go` and the ordering call sites in `engine/search.go` @ `87714a1`.*

---

## Purpose

PVS and alpha-beta are only as good as the move order: search the best move first and the
null-window scouts of the rest prove they're worse cheaply; search a bad move first and you
re-search constantly. Ordering is the single highest-leverage efficiency lever in the search.

---

## Canonical

The standard priority order, best-first:

1. **TT move** — the previously-found best move for this position.
2. **Winning/equal captures** — ordered by MVV-LVA or SEE.
3. **Promotions** — queen promotions are nearly always best.
4. **Killer moves** — quiet moves that caused a beta cutoff at *the same distance from root* in a
   sibling. **Canonically indexed by ply** (distance from root), because "what refuted a sibling at
   this ply" is the useful signal.
5. **Counter moves** — the best quiet reply to the opponent's last move.
6. **History heuristic** — quiet moves ranked by how often `[piece][to]` (or `[from][to]`) caused
   cutoffs, plus **continuation history** ([prev-move][this-move]) for context.
7. **Losing captures** — captures with negative SEE, below the quiet killers/counters.

History scores are **gravity-damped** (bonus shrinks as the entry saturates) and periodically aged
to prevent overflow and stay responsive.

---

## Invariants

- **`[INV-MO1]` The TT move outranks everything `[HOLDS]`** *(audited 2026-05-31, 87714a1)* — TT move
  scores `100000`, strictly above every other band (moveorder.go:328). Ensures the most likely cutoff
  move is tried first.
- **`[INV-MO2]` Bands don't overlap; tie-breakers stay inside their band `[HOLDS]`** *(audited 2026-05-31)* —
  the capture-history tie-breaker is `/8` (≤ ±1024), well inside the 60000-wide capture band; killers/
  counters (25000/30000) sit strictly above losing captures (~20000) and below winning captures (~80000);
  quiet history is clamped to ±8192. No band can leak into another. Verified by the TT/ordering/state audit.
- **`[INV-MO3]` Killers/counters are quiet-only `[HOLDS]`** — `UpdateKillerMoves`/`UpdateCounterMove`
  store only `!IsCapture() && PromoType==NoType` moves (:223, :254), so they never mis-tag a capture.
- **`[INV-MO4]` History indexing is bounds-safe `[HOLDS]`** *(audited 2026-05-31)* — `[piece][to]`
  (13×64), continuation `[prevP][prevTo][p][to]`, capture `[p][to][captured]` (13×64×13); piece index
  1..12 (0 is the unused NoPiece sentinel); en-passant carries the real captured pawn. No OOB.
- **`[INV-MO5]` Ordering is pure (no board mutation) `[HOLDS]`** — `orderMovesSinglePassIntoBuffer`
  scores into a fixed `[256]int`, selection-sorts in place, no `MakeMove`. SEE is read-only.

---

## NGN

**Single-pass scoring + selection sort** (`orderMovesSinglePassIntoBuffer`, :314). The score bands:

| Band | Score | Note |
|---|---|---|
| TT move | `100000` | only when `ttHit && move==ttMove` |
| Queen promotion (incl. capture-promo) | `90000 + promoWeight` | above all captures |
| Winning/equal capture (SEE ≥ 0) | `80000 + see + 1000 + chHist/8` | SEE-ordered, capture-history tie-break |
| Under-promotion | `70000 + promoWeight` | rare, kept low |
| Castle | `50000` | |
| Counter move | `30000 (+ history)` | quiet only |
| Killer move | `25000 (+ history)` | quiet only |
| Losing capture (SEE < 0) | `20000 + see + 1000 + chHist/8` | below killers/counters, above quiets |
| Quiet | `history + continuation` | clamped ±8192 each |

**Heuristic tables** (moveorder.go):
- `historyTable[13][64]` — `[piece][to]`. Bonus `depth*depth` on cutoff; penalty on tried-not-cut;
  halved (`ageHistoryTable`) when any entry exceeds 8192.
- `continuationHistory[13][64][13][64]` — `[prevP][prevTo][p][to]`, full weight, same update points.
- `captureHistory[13][64][13]` — `[p][to][captured]`, the capture tie-breaker.
- `pawnCorrectionHistory[2][16384]` — a learned static-eval adjustment per pawn structure (read at
  search.go:1116, doc 07/eval boundary). Capped to ±49 cp at the eval.
- `killerMoves[MaximumDepth][2]` — **indexed by remaining `depth`, NOT ply** (see divergence below).
- `counterMoves[64][64]` — `[from][to]` of the previous move.

**Update points** (search.go on beta cutoff, :1599-1624): `UpdateHistoryTable`, `UpdateKillerMoves`,
`UpdateCounterMove`; penalize all other tried quiets; `UpdateCaptureHistory` for the cutting capture and
`PenalizeCaptureHistory` for tried-not-cut captures.

---

## Divergences & status

- **`[DIV-MO1]` Killers are indexed by remaining depth, not ply `[ACCEPTED]` / open lever** — NGN stores
  `killerMoves[depth]` and looks up `IsKillerMove(move, depth)` with the node's remaining depth (:1538,
  :370, :223). The literature (Stockfish/CPW) indexes killers by **ply**. Depth-indexing conflates nodes
  at very different plies that happen to share remaining depth. A prior attempt to switch to ply-indexed
  killers ("S2") was shelved on an underpowered/invalid node-count proxy, not a real null — the literature
  unambiguously favors ply-indexed. **This is a tuning lever, not a correctness bug** (killers are only an
  ordering hint; a stale slot just costs efficiency). Flagged for a properly-powered A/B.
- **`[DIV-MO2]` Root ordering looks up killers at the wrong slot `[ACCEPTED]` (efficiency only)** — root
  calls `orderMovesIntoBufferWithDepth(moves, currentDepth, …)` (:643), but root children store killers at
  `depth = currentDepth-1`, so `IsKillerMove(_, currentDepth)` mostly hits empty/stale slots. Root moves
  are still ordered by previous-best-first swap (:647) + captures + history, so the only cost is a missed
  hint at the root. *(finding: TT/ordering/state agent a186f283, 2026-05-31.)*
- **`[DIV-MO3]` No upfront MVV/LVA sort in qsearch `[ACCEPTED]`** — qsearch orders captures only by a
  ttMove front-swap (doc 06); a prior full upfront sort regressed −129 ELO (commit 2f61d3a). Deliberate.

---

## Joints this opens (doc 09)

- **J7** — `lastMovePlayed` (the counter-move key) is a global threaded imperatively through recursion
  and zeroed across null moves; ordering reads it via `GetLastMovePlayed()`.
- **J1** — killer/history adjustments feed the **LMR reduction** (doc 04/05): a killer reduces less, bad
  history reduces more. Ordering and reduction share these tables.
