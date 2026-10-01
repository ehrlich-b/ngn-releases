# 02 — Transposition Table

*Anchored to `engine/cache.go` and the probe/store sites in `engine/search.go` @ `87714a1`.*

---

## Purpose

Cache the result of searching a position so that when the same position is reached again (by
a different move order — a *transposition*) the work is reused: a best-move hint for ordering,
and, when the cached search was at least as deep and its bound is usable, an immediate cutoff.

---

## Canonical

A TT entry stores, keyed by Zobrist hash: **(best move, score, depth, bound-type, age)**.

**Bound types** encode what the stored score means relative to the window it was searched under:

- **Exact** (PV node): the true value. Usable as-is.
- **LowerBound** (Cut node, fail-high): the true value is `>=` stored. Usable only to cause a
  *fail-high* (`stored >= beta`).
- **UpperBound** (All node, fail-low): the true value is `<=` stored. Usable only to cause a
  *fail-low* (`stored <= alpha`).

**Cutoff rule.** Use a stored entry for a cutoff only when `ttDepth >= depth` (it was searched at
least as deep as we need) and the bound is usable against the current window. **Never cut on the
PV** (you'd truncate exact-value resolution and expose the graph-history-interaction hazard).

**Mate scores are stored node-relative.** A mate score encodes "mate in N plies *from this node*."
The same position reached at a different ply must read back the correct distance, so the score is
adjusted by `ply` on store and un-adjusted on retrieve. (doc 07.)

**Replacement.** The table is a fixed array; collisions are resolved by a replacement policy that
protects deep/exact entries from being clobbered by shallow ones, while letting stale (old-age)
entries be overwritten.

---

## Invariants

- **`[INV-TT1]` A non-exact bound is never returned as exact `[HOLDS]`** *(audited 2026-05-31, 87714a1)* —
  the probe (search.go:1045-1060) returns `Exact` unconditionally, `LowerBound` only if `>=beta`,
  `UpperBound` only if `<=alpha`. Verified by the TT/ordering/state audit.
- **`[INV-TT2]` TT cutoffs only off the PV, only at sufficient depth `[HOLDS]`** — gate is
  `ttHit && !isPV && ttDepth>=int8(depth) && !inSingular` (:1045).
- **`[INV-TT3]` Mate scores round-trip through ply adjustment `[HOLDS]`** *(audited 2026-05-31)* — all
  three stores (:1633, :1673, qsearch :1869) use `scoreToTT(_, ply)`; both reads (:1036, qsearch :1708)
  use `scoreFromTT(_, ply)`. No raw mate score is ever stored or compared. Confirmed by two audits.
- **`[INV-TT4]` A TT move is never trusted blindly `[HOLDS]`** *(audited 2026-05-31)* — a retrieved
  `ttMove` is only used as an *ordering hint*; it is played only if it matches a freshly generated
  pseudo-legal move and then passes the `isInCheck` legality filter. A colliding/illegal TT move is
  silently ignored, never played. So a hash collision cannot corrupt the board. (doc 03, doc 09 J6.)
- **`[INV-TT5]` The verification root neither cuts on nor writes the real entry `[HOLDS]`** — when
  `inSingular` (`ExcludedMove!=EmptyMove && ply==ExcludedPly`), the cutoff (:1045) and both stores
  (:1632, :1672) are skipped, so the move-excluded search cannot corrupt the position's real entry.
  (doc 05.)
- **`[INV-TT6]` Stored depth and move fit their fields `[HOLDS]`** — depth `<= MaximumDepth(100)`
  (the `depth>MaximumDepth` guard at :973 precedes any store), fits 7 bits; `Move` packs into 28 bits
  and `MOVE_MASK` is 28 bits — lossless. Verified by the TT/ordering/state audit.
- **`[INV-TT7]` Lockless reads are self-validating `[HOLDS]`** — the Hyatt XOR scheme (cache.go:140-188):
  write Key=`data^hash` then Data; read Data then Key; accept iff `hash == key^data`. A torn read fails
  the check and reads as a miss. No locks, no false hit. **Load-bearing only while single-threaded-writer
  with atomic 64-bit slots** (INV-A7).

---

## NGN

**Layout** (`cache.go`): each `CachedEval` is two `atomic.Uint64` (Key, Data) padded to a 64-byte cache
line. `Data` packs (`Pack`/`Unpack`, :88-103):

| Field | Bits | Offset |
|---|---|---|
| hashmove | 28 | 0 |
| eval (int16) | 16 | 28 |
| depth (int8) | 7 | 44 |
| nodeType | 3 | 51 |
| age | 10 | 54 |

`NodeType` values (`cache.go:42`): `Exact=1, UpperBound=2, LowerBound=4`.

**Replacement** (`Set`, :140-171): replace if the slot is empty; if the slot holds the *same* hash,
replace when `depth >= oldDepth-3 || nodeType == Exact` (a slightly-shallower re-search or any exact
result wins); if a *different* hash, replace when `oldAge != age || depth >= oldDepth` (stale entries
yield; otherwise depth-preferred). `AdvanceAge` bumps the age each search (:116); the 10-bit age wraps.

**Sizing**: `NewCache(megabytes)` rounds the entry count down to a power of two so `index = hash & mask`
(:136). Default 128 MB.

**Probe/store sites in search**: probe :1035; the four conditional cutoffs :1048-1058; LowerBound store
on beta cutoff :1633; final Exact/UpperBound store :1660-1673; qsearch fail-high LowerBound store at
depth 0 :1869 (doc 06).

---

## Divergences & status

- **Qsearch stores only fail-highs at depth 0 `[ACCEPTED]`** — storing every qsearch node would flood
  the table with depth-0 entries and evict deeper main-search ones; the depth-preferred replacement
  (`0 >= oldDepth-3` fails for any `oldDepth>3`) protects deep entries, and a depth-0 LowerBound can
  never trigger a main-search cutoff (`ttDepth>=depth` with `depth>=1`). (doc 06.)
- **No explicit insufficient-material probe in-tree `[ACCEPTED]`** — eval handles it (`IsDraw` in
  position.go / eval). Standard; not a TT concern.

---

## Joints this opens (doc 09)

- **J4** — TT scores vs path-dependent draws (the graph-history-interaction problem): a score
  influenced by a repetition draw on one path can be reused on another. NGN mitigates by *never
  storing the draw at the repetition node*, but a parent's stored score can still be path-tainted. ACCEPTED.
- **J5** — TT vs mate-distance encoding (INV-TT3) vs aspiration windows.
- **J6** — TT move vs move-generation legality (INV-TT4): the collision-safety joint.
- **J2** — TT-refined static eval ("S7") feeding the NMP/RFP/futility gates (doc 04): the cutoff
  decisions read a value the TT mutates.
