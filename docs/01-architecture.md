# 01 — Search Architecture (the contract)

*Anchored to `engine/search.go` @ commit `87714a1`.*

This is the frame every other doc plugs into: the negamax contract, the node algorithm in
canonical order, the iterative-deepening driver, and the global/per-search state the search
relies on. Read this before any technique doc.

---

## Purpose

Turn a `Position` into a best move and a score, by searching the game tree with alpha-beta
+ a transposition table + selective deepening, under a time or node or depth budget, without
ever (a) returning an illegal move, (b) corrupting the board, or (c) committing a
half-searched move when the clock interrupts.

---

## Canonical: negamax + PVS

NGN uses **negamax** (single recursion, scores always from the side-to-move's perspective)
with **principal-variation search** (PVS) layered on alpha-beta.

```
score(node) = max over legal moves m of  -score(child after m)
```

The recursion is `alphaBetaPV` (search.go:941):

```go
func alphaBetaPV(pos *Position, depth, ply, alpha, beta int,
                 isPV, canNull bool, info *SearchInfo) int
```

| Param | Meaning | Invariant |
|---|---|---|
| `depth` | plies left to search to the horizon. `depth==0` → quiescence. | may be raised by extensions, lowered by reductions |
| `ply` | distance from root (0 at root). | `0 <= ply < MaximumDepth` always (guard at :988) |
| `alpha`, `beta` | the search window, side-to-move-relative. | child is always called with `(-beta, -alpha)` |
| `isPV` | true on the principal-variation spine and the first child of every PV node. | a node with `beta-alpha>1` is a PV node; the scout window `beta==alpha+1` is non-PV |
| `canNull` | may this node try a null move? false inside a null child (no two nulls in a row). | reset to `true` on every real-move recursion |

**Node types** (the standard PV/Cut/All trichotomy):

- **PV node** (`isPV`, full window): the value must be *exact*. First child searched full
  window; the rest are scouted with a null window and **re-searched full-window if the scout
  lands inside `(alpha, beta)`**.
- **Cut node** (`!isPV`, expecting a fail-high): scout window only; one good move produces a
  beta cutoff. NMP / most pruning live here.
- **All node** (`!isPV`, expecting a fail-low): no move beats alpha; returns an upper bound.

PVS rests on **good move ordering** (doc 03): if the first move really is best, the null-window
scouts of the rest prove they are worse cheaply.

---

## Canonical: the node algorithm, in order

This is the contract for one `alphaBetaPV` invocation. NGN follows this order; the parenthetical
is the `search.go` line. **Order matters** — several joints (doc 09) are about *what is computed
before what*.

1. **Seldepth + budget guards** (:942–990): bump seldepth; trip stop on node cap / UCI stop /
   time; return static eval on `depth<0`, `depth>MaximumDepth`, or `ply>=MaximumDepth`.
2. **Hash once** (:993): `hash := pos.Hash()` — reused for repetition and the TT.
3. **Draw detection BEFORE anything else** (:996–1024): 50-move (`HalfMoveClock>=100`), game-history
   3-fold (`Positions[hash]>=2` ⇒ this is the 3rd), search-path repetition (`hash` already on
   `RepStack` ⇒ 2-fold-in-search is a draw). Each returns `0` *immediately* — **no TT store of a
   path-dependent draw** (doc 07). Then push `hash` onto `RepStack`; pop via `defer`.
4. **Horizon** (:1028): `depth==0` → `quiescence` (doc 06).
5. **TT probe** (:1035): get move/score/depth/bound. `scoreFromTT` un-adjusts mate scores by ply.
   Take a cutoff only when `!isPV && ttDepth>=depth && !inSingular` (doc 02).
6. **IID** (:1064): if PV, deep, and no TT move, do a reduced re-search to populate the TT move,
   *hiding this node's RepStack entry during the re-entry* (doc 09 joint J3).
7. **Mate-distance pruning** (:1086): tighten `[alpha,beta]` to the best/worst mate still
   reachable at this ply (doc 07).
8. **Static eval** (:1100–1133): `inCheck`; `staticEval` (eval + pawn-correction, optionally
   refined by the TT score — "S7"); `improving` (vs ply-2). Undefined in check → inherit ply-2.
9. **Reverse futility** (:1139) and the **futility flag** (:1147) — both read `staticEval`.
10. **Null-move pruning** (:1163) — cut nodes only, guarded by `staticEval>=beta` + non-pawn material.
11. **Probcut** (:1192).
12. **Singular detection** (:1243): mark the TT move as a singular candidate (verified inside the loop).
13. **Generate moves; handle no-legal-moves** (:1252): no moves ⇒ checkmate (`-MATE_VALUE+ply`) if
    in check, else stalemate (`0`).
14. **Move loop** (:1283): order; per-move prune (futility / LMP / SEE / history) *before* make;
    legality via make/inCheck/unmake; compute extensions → `nextDepth`; clamp by per-node +1 and
    per-path budget; **PVS + LMR** search; on `score>=beta` update history/killer/counter, store a
    LowerBound, return; else raise alpha.
15. **Final store** (:1657): pawn-correction update; classify `UpperBound`/`Exact`; store unless
    `inSingular`; return `bestScore`.

---

## Iterative deepening (the driver)

Two entry points build a `SearchInfo` and drive the recursion:

### `searchIterativeDeepeningUnsafe` (:522) — the real one (UCI play)

- Clears the stop flag, ages the TT (`AdvanceAge`), allocates the four reusable buffers once.
- Loops `currentDepth = 1..maxDepth`. Each iteration: `NewIteration()` (time), reset `SelDepth`,
  **set `RootDepth = currentDepth`** (the extension-budget reference — doc 05), set the aspiration
  window from the previous score, run the root move loop, re-search on fail-high/low (≤3 attempts).
- **Aspiration** (:596): full window for `depth<=3`; otherwise ±50 cp (±100 near mate). Classify
  against the *snapshotted* window (`windowAlpha/windowBeta`, :638), not the alpha mutated during
  the loop.
- **The clock-interrupt discard (INV-A4, load-bearing)** (:710): if `info.Stopped` mid-iteration,
  `break` and keep the *previous completed* iteration's best move. A partially-searched iteration's
  `iterationBestMove` is a half-searched, often-blundering move; committing it was the root cause of
  catastrophic time-pressure blunders. Only a fully-completed iteration writes `info.BestMove`.
- **Forced-mate early exit** (:756): once a mate score is proven, stop deepening (deeper iterations
  just re-derive it via the TT).
- Feeds the time manager the stability signal `ReportCompletedIteration(bestMoveChanged, attempts==1)`
  (doc 08).

### `searchFixedUnsafe` (:839) — fixed-depth, no ID

- Single depth, full window, no aspiration. Used by tests and `cmd/trace_move`.
- **Does NOT set `info.RootDepth`** → it stays 0. See INV-A5 below — this is a real latent bug.

---

## Global & per-search state

**Global (package-level), single-threaded** — the UCI `Threads` option is a stub (uci.go), so
these have no data race:

- `globalStopRequested atomic.Bool` (:34) — UCI `stop`; cleared at search entry.
- `globalMaxNodes atomic.Uint64` (:57) — `go nodes N` budget; trips stop when spent.
- `lastMovePlayed Move` (moveorder.go:90) — the **only** global threaded *imperatively* (via
  `SetLastMovePlayed`/`GetLastMovePlayed` + a saved/restored `savedLast` on every recursion). Feeds
  the counter-move heuristic and extension logic. Fragile: see joint J7 (doc 09).
- `TranspositionTable *Cache`, and the heuristic tables `historyTable` / `continuationHistory` /
  `captureHistory` / `killerMoves` / `counterMoves` / `pawnCorrectionHistory` (doc 03).

**Per-search (`SearchInfo`, :461)** — freshly allocated each `Search*` call, so a panic-corrupted
frame dies with that search (the crash handler recovers and the next search starts clean):

- `Nodes`, `Depth`, `SelDepth`, `RootDepth`, `BestMove`, `BestScore`, `Stopped`, `TimeManager`.
- `StaticEvalStack[ply]` — for the `improving` heuristic.
- The four reusable buffers (`MoveBuffer`, `CaptureBuffer`, `OrderedBuffer`, `SEEGains`).
- `ExcludedMove` / `ExcludedPly` — the singular-verification exclusion (doc 05).
- `RepStack[512]` / `RepStackLen` — the search-path repetition stack (doc 07).
- A pile of pruning/extension counters (diagnostics only; *not* read by search — but note F1 in
  doc 09: a counter incrementing does **not** prove the technique took effect).

---

## Invariants

- **`[INV-A1]` Negamax sign discipline `[HOLDS]`** *(audited 2026-05-31, 87714a1)* — every child is
  called as `-alphaBetaPV(..., -beta, -alpha, ...)`. Scores are side-to-move-relative everywhere;
  the root negates to get White-relative for UCI.
- **`[INV-A2]` Make/unmake balance `[HOLDS]`** *(audited 2026-05-31)* — every `MakeMove`/`MakeNullMove`
  is matched by an unmake on *every* exit path, and `lastMovePlayed` is saved before and restored
  after. All pruning `continue`s occur *before* the make; the legality `continue` (:1389) and the
  time `break` (:1297) unmake correctly; the singular unmake→remake (:1444/:1464) is balanced before
  the loop-tail unmake (:1585). Verified by the TT/ordering/state audit + `hash_makeunmake_test.go`.
- **`[INV-A3]` RepStack balance per node `[HOLDS]` (push guarded, pop unconditional — latent asymmetry)**
  — push is guarded by `RepStackLen < 512` (:1020) but the `defer` pop is unconditional (:1026). The
  `ply<MaximumDepth(=100)` guard at :988 means `RepStackLen` never approaches 512, so the asymmetry is
  currently unreachable. Defensive fix (guard the pop or push unconditionally) is cheap. See doc 09 J3.
- **`[INV-A4]` Clock-interrupt discard `[HOLDS]`** *(audited 2026-05-31)* — a `info.Stopped`
  mid-iteration is discarded entirely; only a fully-completed ID iteration commits `info.BestMove`
  (:710, :746). This is the single most important anti-blunder invariant in the engine.
- **`[INV-A5]` Every budget-using entry point sets `RootDepth` `[VIOLATED]`** — the per-path extension
  budget clamp (:1488) reads `info.RootDepth`. `searchIterativeDeepeningUnsafe` sets it (:586);
  **`searchFixedUnsafe` does not** (:849), leaving it 0, so the clamp `maxNextDepth = -(ply+1)+24`
  wrongly truncates any fixed-depth search past ~depth 24. Shipping UCI play is unaffected (routes
  through ID); `cmd/trace_move` and deep fixed-depth tests are. **Fix:** `info.RootDepth = depth` in
  `searchFixedUnsafe`. *(finding: fix-review agent af64cc38, 2026-05-31; verified.)* See doc 05.
- **`[INV-A6]` Ply never overflows ply-indexed arrays `[HOLDS]`** — `ply>=MaximumDepth` returns a static
  eval (:988), guarding `PV[MaximumDepth]`, `StaticEvalStack`, and `killerMoves` against a non-decaying
  checking line. The extension budget (doc 05) is the *finite-ness* guarantee that keeps real searches
  far from this cap; this guard is the hard backstop.
- **`[INV-A7]` Single-threaded heuristic state `[HOLDS]`** — search is single-threaded (Threads is a
  stub), so the global tables are race-free. **If multi-threading is ever added, every global table and
  `lastMovePlayed` becomes a data race** — this invariant is load-bearing for that future work.

---

## Joints this frame opens (see doc 09)

- **J3** — re-entrancy (IID/singular re-search the same node) vs the RepStack.
- **J5** — mate scores vs the TT (ply adjustment) vs aspiration windows.
- **J7** — `lastMovePlayed` global vs null move vs recursion.
- **J8** — the extension budget vs `RootDepth` set-up (INV-A5).
