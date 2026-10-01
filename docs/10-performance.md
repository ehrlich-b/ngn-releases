# 10 — Performance / NPS (the speed lane)

*Re-profiled **2026-05-31 @ `7c9c58f`** (post T1/T2a/N1) on E-cores (`taskpolicy -b`) with a deep,
phase-diverse basket (`BenchmarkSearchProfile`, engine/profile_test.go — middlegame + Kiwipete +
endgame, depth 11, cold TT per op, 27M nodes / 67s of samples). Supersedes the earlier same-day
profile and the stale `docs/eliminate_alloc.md` (Feb 2026).*

---

## Why this matters — the gap, decomposed (2026-05-31, verified)

The old "~2.5x NPS" headline was imprecise. "NPS" is **not a scalar** — NGN's own NPS swings ~307K
(middlegame) → ~540–860K (endgame) on E-cores, ~1M on P-cores — and a single ratio hid two
*independent* problems. Measured properly (fixed depth, identical positions, both engines on E-cores
via `taskpolicy -b`, both counting qsearch nodes — NGN search.go:1779-80, Blunder search.go:307) the
gap factors cleanly:

    time-to-depth  =  per-node-speed (nps ratio)  ×  tree-size (nodes ratio)

| position | per-node BLU/NGN | **tree NGN/BLU** | to-depth NGN/BLU |
|---|---|---|---|
| middlegame (`r1bq1rk1/…` d12) | ~1.9x | **1.13x** | ~2.0x |
| middlegame 2 (`r1bqk2r/…` d12) | ~1.9x | **0.78x** | ~1.4x |
| kiwipete (d12) | ~2.0x | **1.00x** | ~2.0x |
| passed-pawn endgame (`8/2p5/…` d16) | ~2.0x | **2.57x** | ~4–6x |
| quiet rook endgame (`r7/5pk1/…` d18) | ~2.4x | **5.12x** | ~12x |

Reproduce with **`scripts/h2h.sh`** (the canonical instrument — built precisely because a scalar NPS
kept us oscillating). **Tree-size ratios are deterministic and exact** (node counts reproduce to the
unit; Blunder resets its counters per-iteration so the tool sums them). **Per-node speed is thermally
noisy on the M4** — back-to-back E-core runs throttle, so the ratio reads 1.74–2.63x depending on heat.
Quote tree-size precisely; quote per-node speed as "~2x".

**This refutes the comfortable "NGN searches fewer, higher-quality nodes" story:**

- **Per-node speed — a flat ~2x tax everywhere.** Engineering, not chess-architecture: another *Go* HCE
  engine (Zahak) benches mid-pack with the C/C++ field, so ~2x is an NGN implementation gap, not a Go
  tax. Recovering it is worth ~+90–130 ELO at blitz (Laskos speed-doubling curve), risk-free.
- **Tree size — ~1x in middlegames (sometimes *smaller*, 0.78x), but 2.5–5x LARGER in endgames.** NGN is
  **not** searching fewer nodes. Effective branching matches Blunder in middlegames (~3.2 vs ~3.17) but
  is ~10% worse per ply in endgames (~2.25 vs ~2.05), which compounds over the deep searches endgames
  reach (1.098¹⁸ ≈ 5x). **This is a pruning-EFFICIENCY collapse, not eval cost — and it is the bigger,
  more surprising lever.** A large share of the +150 CCRL deficit to Blunder may live in endgames. NMP
  is *not* the cause (it fires correctly — `hasNonPawnMaterial` gates it off only in pawn-only
  positions, and a rook is present). **PROBED 2026-05-31** (a throwaway build with check extensions
  disabled): check extensions are a *phase-general* tree cost, **not** the endgame cause — disabling
  cut the middlegame tree −47% (to *below* Blunder's) but the endgame stayed 3.25x Blunder (was 5.12x).
  So there are TWO levers: **(a)** discipline the full-ply check extension at `search.go:1466` (SEE-gate
  it — helps all phases, esp. middlegame; a strength feature ⇒ SPRT, not removal); **(b)** an
  endgame-specific residual — futility prunes fire on only **13% of endgame nodes vs 69% in the
  middlegame**, so the endgame futility/pruning gap is the second factor. Both are fixed-time SPRTs, and
  **neither** is addressed by any [NEUTRAL] speed item below.

Both engines are single-threaded classical Go on the same box, so neither factor is language/threads/
hardware. The per-node budget below maps the *speed* factor; the endgame *tree* factor is a separate
search-correctness investigation.

---

## The per-node micro-budget (verified, not estimated)

The profile is 66.97s of CPU samples over 27.05M nodes ⇒ **~2.48 µs/node E-core (profiled, cold-TT);
the real game-search node is ~3.75 µs E-core (267K NPS) / ~1 µs P-core (~1M NPS).** Absolute ns
shifts with hardware/TC; **the % split below is hardware-independent and is the thing to attack.**

### View A — by subsystem (cumulative; the all-in cost of each *action*)

These overlap (eval is called *inside* qsearch, SEE *inside* ordering), so they sum > 100% — this is
the "what does one X cost, all-in" view.

| Action (per node) | % CPU (cum) | ns @ 3.75µs | What it is |
|---|---|---|---|
| **Quiescence subtree** | **49.8%** | — | half the whole tree is qsearch (container; its children are below) |
| **Evaluation** (`EvaluateForPlayer→evaluateUnsafe`) | **18.6%** | ~700 | full classical eval; runs at leaves AND internal nodes (RFP/futility/NMP/improving) |
| **Move ordering** (`scoreMovesIntoBuffer` 10.4 + `selectNextMove` 7.5) | **~18%** | ~675 | scores every move (SEE per capture) + incremental selection sort |
| **Make + Unmake** (`makeMoveHelper` 10.1 + `unMakeMoveHelper` 6.7) | **16.8%** | ~630 | board mutation + incremental hash + N1 eval accumulator |
| **SEE** (`staticExchangeEvaluation`) | **11.0%** | ~410 | called for *every capture during ordering* + qsearch pruning |
| **Move generation** (`GenerateMovesIntoBuffer`) | **10.8%** | ~405 | full pseudo-legal gen every node; sliders **ray-walked** (see below) |
| **isInCheck** | **8.2%** | ~305 | node-in-check + per-move legality scan + gives-check |

### View B — by primitive leaf (flat self-time; where raw cycles actually burn, sums ≈ sampled total)

| Leaf | Flat % | Note → target |
|---|---|---|
| `selectNextMove` | **7.5%** | O(n²) selection-sort scan; already deferred past cutoffs (T2a). Inherent. |
| `Bitboard.Clear` | **5.4%** | 12-way switch + composite-bb + **N1 accumulator subtract** (N1 moved eval cost here) |
| `generateSlidingMovesIntoBuffer` | **4.9%** | **ray-walk** + per-step wrap guards → **T5 (magic)** |
| `findLeastValuableAttacker` | **4.7%** | SEE inner loop |
| `GetBitboardOf` | **4.6%** | **no mailbox** — 12-case piece→bb switch → **T6** |
| `scoreMovesIntoBuffer` | **4.4%** | move scoring (SEE per capture) |
| `generateSlidingCapturesIntoBuffer` | **3.6%** | ray-walk (captures) → **T5** |
| `isInCheck` | **3.5%** | king-attack recompute |
| `Bitboard.Move` | **2.7%** | 12-way switch + accumulator add |
| `PieceAt` / `GetPiece` | **2.3% / 1.9%** | **no mailbox** → **T6** |
| `updateHash` | **2.3%** | incremental Zobrist (fine) |
| `madvise` / `memclrNoHeapPointers` | **2.2% / 2.1%** | **GC + zeroing — driven by the qsearch allocation (below)** |
| `GetBishopAttacks`/`GetRookAttacks` | **2.1% / 1.8%** | O(1) magic — used by isInCheck/SEE, **NOT by movegen** (the waste) |
| `atomic.Uint64.Load` | **1.5%** | per-node stop/max-node checks |
| eval leaves (`pawnShield` 2.4, `kingAttackPatterns` 1.7, `mobility` 1.5, `bishopPair` 1.5, `kingSafety` 1.2, `outpost` 1.1, `rookOpenFile` 0.9 …) | **~12% total** | the positional terms not covered by the N1 accumulator |

**Three cross-cutting taxes** (distributed across the subsystems above, so they don't show as one row):
- **No mailbox:** `GetBitboardOf` + `PieceAt` + `GetPiece` = **~8.8% flat** of 12-way piece→bitboard
  switches, called from movegen/eval/SEE/make. A `[64]Piece` mailbox makes these O(1) array reads.
- **Allocation/GC:** `madvise` + `memclr` + `mallocgc` + `growslice` ≈ **~5% flat**, driven entirely by
  the qsearch allocation isolated below.
- **Ray-walk sliders:** `generateSlidingMoves`+`Captures` = **~8.4% flat**, fully replaceable by the
  magic getters that already exist in the tree.

---

## Allocation profile — the picture INVERTED after T1

The earlier same-day profile said *"alphaBetaPV = 62% of allocations, quiescence = 33%, ~77–207
allocs/op, allocation is NOT the bottleneck."* **T1 (the frame pool) fixed alphaBetaPV — so the
residual migrated entirely to quiescence, which T1 never touched:**

```
-alloc_objects:  quiescenceWithDepth 96.9%   alphaBetaPV 1.5%
-alloc_space:    quiescenceWithDepth 93.9% (7.08 GB / 2.7M-node search)
                 → search.go:1842  var moveBuffer [256]Move   = 6.0 GB   (stalemate probe @ qDepth==0)
                 → search.go:1789  var moveBuffer [256]Move   = 1.08 GB  (in-check evasion buffer)
   ~684,000 allocs/op  ≈ 0.25 allocs/node  ≈ 260 bytes/node
```

Root cause: quiescence stack-declares `var moveBuffer [256]Move` (1 KB) and hands `moveBuffer[:]` to
non-inlined `GenerateMovesIntoBuffer`; the compiler can't prove the slice doesn't escape ⇒ the whole
1 KB array lands on the heap, **per qsearch node**. (`var captures [64]Move` at :1876 does *not* escape
— it's filled by an inlined `copy`, so it stays on the stack. The escape is specifically the slice
handed across the gen call.) The 6 GB line :1842 is the worst: a full move-gen + legality-test just to
ask *"is there one legal move (stalemate)?"* at **every** qsearch root (= every main-search horizon
node). Blunder is **zero-alloc per node** — this is the single clearest architectural divergence.

> **Measurement caveat (the T1 lesson — still load-bearing):** removing alphaBetaPV's allocations in
> T1 moved *real game-search* NPS < 2%, far below the profile's GC share, because a benchmark loop
> churns the allocator far harder than one game search does. So the ~5% GC tax above is an **upper
> bound** on the qsearch-pool win; expect the real-search gain to be smaller. It is still worth doing:
> it's node-identical (KEEP RULE), it removes 7 GB of churn + GC-pause variance, and it's the
> prerequisite for trusting the rest of the profile. Just don't expect it alone to move the 2.5x. Verify
> on a real `go depth N`, never on the benchmark-loop alone.

---

## What would "2.5x" even mean? (the honest decomposition)

NGN ~3.75 µs/node vs Blunder ~1.43 µs/node (E-core) — a ~2.3 µs/node gap. **It is not one thing; it is
a stack of independent factors that *multiply*.** Decomposing by what Blunder structurally does NOT pay:

| Gap factor | NGN cost | Blunder | Recoverable? | Lever |
|---|---|---|---|---|
| Per-node heap alloc | ~5% (GC tax) + variance | zero-alloc | **FREE** (node-identical) — **SHIPPED T1b** | **T1b** |
| Ray-walk vs magic movegen | ~8.4% flat | O(1) magic | **TRADEOFF** — gen-order → tie-break → node counts (SPRT) | **T5** |
| 12-way `PieceAt` vs mailbox | ~2.3% flat | `Squares[sq]` O(1) | **FREE** (node-identical) | **T6** |
| 12-way `GetBitboardOf` switch | ~4.6% flat | array-of-`[12]uint64` | **FREE-ish** — big struct refactor, gated | deferred |
| `GetPiece(type,color)` construct | ~1.9% flat | — | not addressable (not a lookup) | — |
| Per-node `defer` (RepStack/frame) | ~3% | none | **FREE** (node-identical, bug-prone) | **T3** |
| SEE on *every* capture in ordering | ~8% (ordering-SEE) | MVV-LVA in ordering, SEE only in qsearch | **TRADEOFF** (SPRT) | **T2b** |
| Rich eval (mobility/king-safety/pawn-struct/outposts/coord) | ~18.6% (vs Blunder ~10%) | 6 lean terms | **TRADEOFF** (SPRT; N3 already failed) | — |
| Heavier per-node bookkeeping (correction history, S7, static-eval stack, singular) | a few % | leaner negamax | mostly **buys ELO** — leave | — |

**The honest arithmetic:**
- **Genuinely free, node-identical wins are MODEST:** T1b (shipped — churn gone) + T6 (`PieceAt` mailbox
  ~2.3%) + T3 (defer ~3%) ≈ ~5–6% of compute ⇒ **~1.1x**; add the `GetBitboardOf` array-of-bitboards
  refactor (~4.6%, neutral but a big struct change) ⇒ **~1.15–1.2x**. That is the free ceiling — small,
  and the alloc part of it (T1b) transfers to real-game NPS weakly (T1 caveat). Worth ~+5–20 ELO via depth.
- **The single biggest compute item — magic movegen (T5, ~8.4%) — is NOT free.** Slider gen order sets the
  tie-break order of equal-score moves, and that is load-bearing: an empirical test (flipping the
  `selectNextMove` tie-break `>`→`>=`) moved node counts −30% to +22% and even changed a root bestmove
  (mid a2a3→d4c5). So T5 is a fixed-time SPRT (expected ~neutral — tie-break order is arbitrary — kept iff
  no regression), not a node-identity item.
- **The rest of the ~2.5x is NOT free either.** A large chunk is NGN *deliberately* spending cycles on a
  richer eval (18.6%) and SEE-accurate move ordering that **buy strength per node**. Closing it means
  trading that strength — every such cut clears a fixed-time SPRT under the KEEP RULE, and history is
  against it (N3 lazy-eval regressed; an upfront MVV-LVA capture sort once cost −129 ELO, commit 2f61d3a).

**Conclusion / reframe (corrected 2026-05-31 by the decomposition at the top of this doc):** the prior
"NGN trades NPS for node-quality on purpose" was **WRONG**. The data shows NGN does *not* buy a smaller
tree with its richness (tree ~1x in middlegames, **2.5–5x LARGER in endgames**) while paying ~2x per
node — and the lean peer (Blunder, a 560-line eval vs NGN's ~2750) is +150 CCRL *stronger*. *strength = NPS ×
node-quality* still holds, but NGN is currently behind on **both** factors, not trading one for the
other. Levers, ranked by honest payoff:

1. **Endgame search efficiency (the 2.5–5x tree).** Biggest and most surprising; it is pruning/ordering,
   **not** eval; likely a real bug/mistune; a fixed-time SPRT target. **Investigate this first.**
2. **Per-node speed (~2x, the engineering tax).** Bank the free node-identical wins (T1b done, T6/T3,
   `GetBitboardOf` refactor ≈ ~1.15–1.2x), then the [BEHAVIOUR] items (magic movegen) by SPRT. Risk-free.
3. **Eval/SEE cost audit.** NGN's eval (18.6%) is *richer* than the stronger peer, so richness is not the
   bottleneck — but it is unaudited. Test each expensive term with a **fixed-nodes SPRT (isolates
   node-quality) + a bench-NPS delta**: keep terms that pay, cut neutral-but-expensive ones — the KEEP
   RULE *supports* this (a neutral cut that raises NPS with no ELO regression is a keep).

**Do NOT** chase NPS by gutting eval blindly (N3 lazy-eval regressed; MVV-LVA-only sort once cost −129),
and **do NOT** disable techniques by ply (folklore — nobody does it; the correct model is *cheap leaf
eval × depth-scaled pruning*, and our endgame is **under**-pruned, the opposite of too-much-search).
**Intent:** be a lean-eval, fast, strong-search engine like the Blunder/Weiss/Fruit cohort that clears
2500+ on lean eval — rich eval only wins at the very top (SF-classical) on a world-class tuned search we
don't have yet. NNUE remains a separate project.

---

## Ranked attack plan

Each target is **[NEUTRAL]** (must be behaviour-identical — verify by node-count + seldepth + score +
bestmove identity at fixed depth vs clean HEAD on the 3 baselines, then measure NPS) or **[BEHAVIOUR]**
(changes the tree — needs a fixed-time SPRT, not just a node check). The node-identity gate is the
speed-lane analogue of the keep-correctness rule (doc 09).

**Current node-identity baselines (2026-09-04 correctness release):**

| Position | Depth | Nodes | Seldepth | Best move |
|---|---:|---:|---:|---|
| Kiwipete | 12 | 295507 | 25 | e2a6 |
| Middlegame | 12 | 112109 | 19 | a2a3 |
| Rook ending | 16 | 858052 | 34 | b4f4 |

Exact FENs and the wait-for-bestmove driver are in `scripts/nodecheck.sh`;
run `scripts/nodecheck.sh build/ngn`. The first two counts are unchanged from
August. Correct terminal draw handling changes the ending from 667703 nodes.
The exact pawn-push witness preserves all repaired node counts, scores and PVs.
[Validation and game result](../experiments/2026-09-04-fast-correctness.md).

### SHIPPED (kept, behaviour-neutral, do not re-litigate)

#### T1 — frame-pool hot-path buffer reuse `[NEUTRAL]` — SHIPPED `b9c1859`
Recursion-frame-counter pool (`searchFrame{moveBuffer,ordered,quiets,captures}`, balanced by the
RepStack defer, stack fallback on exhaustion). Cut `alphaBetaPV` allocations ~75% (77→18/op on
`BenchmarkSearch`), node-identical on all 3 baselines. The trap it navigates (doc 09 J3): `ordered`/
`quiets`/`captures` are live across the loop's recursive calls, so they **cannot** be ply-indexed (IID
and singular re-enter at the *same* ply, singular *mid-loop*) — hence a frame counter, not ply.
**Real-search NPS moved < 2%** → see the measurement caveat above; kept on KEEP RULE (free, no
regression). **Did NOT touch quiescence — that's the open residual (T1b).**

#### T2a — incremental (deferred) selection sort `[NEUTRAL]` — SHIPPED `7445851`
Split ordering into a score-only pass (`scoreMovesIntoBuffer`) + a single-step `selectNextMove` driven
from the `alphaBetaPV` loop, so a cutoff at move *k* never sorts *k+1..n*. Full selection sort and
deferred selection produce the identical order (strict `>` keeps ties at the earlier index) ⇒
node-identical (verified 3 baselines). ~4–7% faster. **Saves SORT work, not SEE work** — SEE is still
computed for every capture up front (that's T2b).

#### N1 — incremental material+PST+phase eval accumulator `[NEUTRAL]` — SHIPPED `d0fae30`
Three running ints on `Bitboard` (`accMG/accEG/accPhase`) maintained by the only three board-mutation
primitives — `Clear` subtracts, `UpdateSquare`/`Move` add — so every move type is exact with zero
per-type code. `evaluatePeSTO` reads the accumulator instead of rescanning 32 squares + 8 popcounts
every eval (and eval runs at internal nodes too). Bit-exact ⇒ node-identical (3 baselines);
`TestEvalAccumulatorUnderMakeUnmake` pins it over make/unmake on 7 FENs. ~3.6% faster. *Caveat:* this is
**why `Bitboard.Clear` is now 5.4% flat** — the accumulator add/subtract moved eval cost into the
make/unmake primitives. A packed mg/eg `int` (Stockfish `make_score`) would halve that; not done.
**Does NOT contradict "don't trim eval"** — eval is exact, only the *scan* was removed.

### NEXT — free, node-identical (do these first, in order)

#### T1b — quiescence frame pool `[NEUTRAL]` — SHIPPED `3b6809b`, KEPT
**Result: allocs/op 684K→11K (−98.4%), ~7 GB→~0 churn/search; node-identical to HEAD on all 3 baselines;
`go test -short` green; tactical 28/30. Benchmark-loop NPS +~27% but that overstates the real-game gain
(T1 lesson) — kept on node-identity + churn elimination regardless of the real-NPS magnitude.** Gave
quiescence the same treatment T1 gave alphaBetaPV. The escaping `var moveBuffer [256]Move` at
**search.go:1789** (in-check evasions) and **:1842** (qDepth==0 stalemate probe) now come from a
per-qsearch-frame `qframe.moveBuffer` (FrameDepth counter, stack fallback), not a per-call stack array. **Constraint:** :1789's buffer is live across the recursive
`quiescenceWithDepth` call (:1815), so it needs a *per-qsearch-frame* buffer (index by a qsearch frame
counter, or extend `searchFrame` — qDepth is bounded at 6, so a small fixed pool also works); :1842's is
consumed before any recursion (gen → legality-test → discard) so it can share a single `SearchInfo`
scratch field. *Bonus question while here:* :1842 generates ALL moves + make/unmake-tests each just to
detect a rare stalemate at every horizon node — verify whether that stalemate check earns its cost
(removing it is [BEHAVIOUR], measure separately). Gate: 3-baseline node-identity + `go test -short` +
`make smoke-tactical`. Expect a modest real-NPS gain (T1 caveat), kept regardless on KEEP RULE.

#### T5 — magic-bitboard move generation `[BEHAVIOUR → SPRT]` — biggest compute item, NOT free
`generateSlidingMovesIntoBuffer`/`generateSlidingCapturesIntoBuffer` (movegen.go:530+) ray-walk 4–8
directions × up to 7 steps with per-step wrap guards and a `PieceAt` lookup per capture (~8.4% flat).
The O(1) magic getters **already exist** (`GetRookAttacks`/`GetBishopAttacks`, magic.go:253/260), used for
check/SEE but not move-gen. Rewrite slider gen as `attacks = GetXxxAttacks(sq, occ)` + iterate set bits
(classify capture by `enemy & bit`). **NOT node-identical (corrected — earlier draft wrongly called this
neutral):** magic emits in square order, ray-walk in direction-then-step order, and `selectNextMove`
breaks equal-score ties by *generation order* (the exact property that made T2a neutral) — so the
try-order of tied quiets changes ⇒ node counts move. Proven empirically: flipping the tie-break `>`→`>=`
moved nodes −30%..+22% and changed a root bestmove (mid a2a3→d4c5). ⇒ gate by a fixed-time SPRT (expected
~neutral — tie-break order is arbitrary — kept iff no regression). If a node-identical version is wanted,
emit per-direction in step order using the magic set as a blocker mask (preserves order, still drops the
per-step wrap guards) — more work, smaller win.

#### T6 — mailbox for `PieceAt` `[NEUTRAL]` — ~2.3%, modest (corrected scope)
Add a `[64]Piece` mailbox to `Bitboard`, maintained in `Clear`/`UpdateSquare`/`Move` (the three primitives
N1 already hooks), and make `PieceAt(sq)` read it instead of the 12-way bitboard scan (~2.3% flat).
Node-identical (PieceAt returns identical values). **Corrected scope:** this does NOT help `GetBitboardOf`
(4.6%, piece-type→bitboard, not a square lookup) or `GetPiece(type,color)` (1.9%, a constructor) — neither
is mailbox-addressable. `GetBitboardOf` would need an array-of-`[12]uint64` struct refactor (replace the
named fields + 12-way switch with `bitboards[piece]`) — large, deferred. The node gate catches any mailbox
desync; pairs with N1's accumulator maintenance.

#### T3 — drop the per-node RepStack `defer` `[NEUTRAL]`
`defer func(){ info.RepStackLen--; info.FrameDepth-- }()` (search.go:~1052) runs every node; Go's defer
machinery historically showed ~3% (`nextDefer`). Replace with explicit decrements at each return path.
**Caution:** this is the exact invariant doc 09 INV-A3/J3 guards — every `return` in `alphaBetaPV` must
decrement RepStackLen AND FrameDepth exactly once (the frame pool now rides this defer too, so a missed
path corrupts buffer reuse, not just repetition). Verify node-identity + re-audit every return.

### THEN — strength tradeoffs (fixed-time SPRT each, KEEP RULE)

#### T2b — lazy SEE / staged generation `[BEHAVIOUR]`
Today scoring calls SEE on **every** capture before the loop. Stage it: TT move, then captures by
MVV-LVA with **SEE computed only when a capture is actually reached**, stopping at a beta cutoff. Changes
capture tie-break order ⇒ node counts shift ⇒ fixed-time SPRT. Bigger SEE-cost win than T2a but
behaviour-affecting, and NGN's SEE-accurate ordering may search better nodes — net unknown. (Tangle: SEE
also feeds qsearch pruning at :1907 and the losing-capture band sits below killers/castling — a naive
MVV-LVA swap mis-orders losing captures. Needs real staged deferral. See TODO N5.)

#### T4 — cut redundant check detection `[NEUTRAL/BEHAVIOUR]`
`isInCheck` runs up to 3× per move; N2 already removed the tag-addressable two. The **residual is the
per-move legality scan** (search.go:~1434, mover's-king color) — the dominant remaining cost. Killing it
needs *incremental legality* (only re-check the king when a move could expose it: pins / EP / king
moves) — behaviour-neutral but bug-prone. Lower priority; node-identity-gated if attempted.

### Not now
- **Don't trim eval for speed** — the ~18.6% is where strength lives; N3 (lazy stand-pat eval) already
  regressed (distorts null-window scouts in sharp positions). Revisit only as a cheaper-AND-stronger
  regime (packed score, or NNUE), not as a term cull.
- **Lazy SMP / multithreading** — the other large lever, but a separate project (the `Threads` option is
  a stub; every global heuristic table becomes a race, INV-A7). Out of scope here.

---

## Sequencing & gate

**Free/node-identical lane (commit each on node-identity + NPS, no SPRT): T1b (DONE) → T6 → T3 →
GetBitboardOf array refactor (big).** **SPRT lane (fixed-time, run-to-bound, kept iff no regression):
T5 magic movegen → T2b lazy-SEE → T4 incremental legality.** The free lane buys only ~1.1–1.2x; T5 — the
biggest single compute item — lives in the SPRT lane because gen-order moves node counts. After the free
lane + T5 land, re-run the Kiwipete-vs-Blunder depth comparison to see how much of the 2.5x actually
closed — don't expect much; the rest is the eval/ordering tradeoff, by design.

**Every [NEUTRAL] target must produce identical node counts, seldepth, score, and bestmove at fixed
depth** on the 3 baselines — that is the proof it changed only speed, not strength. If node counts move,
it's secretly [BEHAVIOUR] and needs an SPRT.

---

## Endgame-tree autopsy (2026-06-02) — the eg blowup is COMPOUNDING ORDERING, not a broken knob

New instrument: `cmd/pruning-analysis` rebuilt into a phase-bucketed tree-composition profiler
(`build/autopsy`); two counters added to `SearchInfo` (`TTProbes` = hit/cutoff denominator,
`FirstMoveCutoffs` = beta cutoffs on the 1st legal move = move-ordering quality). It buckets a real
corpus (`output/lichess_eval.txt`) by computed game phase and searches each bucket to a fixed depth.

**Ground truth (h2h, fixed depth, current build, E-cores) — the gap GROWS with eg depth:**

| pos | depth | NGN nodes | BLU nodes | tree ratio |
|---|---|---|---|---|
| mid / mid2 / kiwipete | d12 | — | — | **0.79–0.89x** (NGN tree *smaller* than Blunder in mg) |
| pp-endgame | d16 | 1.27M | 336K | **3.78x** |
| quiet-end | d18 | 3.90M | 426K | **9.16x** |

NGN's eg EBF ≈ 2.4 vs Blunder's ≈ 1.95 in a quiet rook ending — a ~0.45/ply difference that compounds
to 3.78x at d16 and 9.16x at d18. Blunder's sub-2.0 eg EBF is the anomaly (near-perfect ordering +
aggressive pruning in simple endgames); NGN's ~2.4 is ordinary. **NGN has no eg BUG; Blunder has an eg
STRENGTH, and it's depth-compounding.**

**Composition autopsy (depth 8, 30 eg ph≤5 vs 30 mg ph≥14), eg/mg ratio of each per-node rate:**

| metric | eg | mg | read |
|---|---|---|---|
| mean nodes/pos | 33.3k | 96.5k | eg tree is *smaller* than NGN-mg at fixed depth (gap is vs Blunder) |
| first-move-cutoff % | 87.5% | 89.7% | **mediocre in BOTH** (top engines ~95%); ordering headroom is global |
| qsearch node % | 35.7% | 41.4% | qsearch is **not** the eg leak (lower share in eg) |
| TT cutoff %/probe | 20.5% | 8.8% | TT is **more** effective in eg — not the leak |
| futility prunes /knode | 149 | 472 | **collapses in eg** — but emergent (eg static evals sit near alpha), not phase-gated, working as designed |
| LMR reductions /knode | 94 | 145 | fires less in eg (fewer late quiets qualify) |
| check ext /knode | 30 | 11 | **2.7x eg storm** (every check extends; lone-king checks are frequent) |
| passed-pawn ext /knode | 35 | 1.7 | **20x eg storm** — was over-broad (extended *any* 6th/7th push, not actual passers) |

**Synthesis (the mechanism).** In middlegames NGN's aggressive futility (472/knode) + LMP (547/knode)
keep the tree at/below Blunder's *despite* mediocre 88% move ordering — the pruning masks the ordering
weakness. In endgames the static eval sits near alpha so **futility stops firing** (149/knode, by
design), the mask is removed, and the underlying 88% ordering + the extension storms are exposed and
**compound over deep quiet lines** into 3.78–9.16x. So the eg gap is not one broken knob (TT, qsearch,
futility, LMR, NMP, TT-capacity all already refuted in [[project_endgame_tree_diagnostic]] and
re-confirmed here) — the cleanest framing is the **MASK**: in mg, aggressive futility/LMP keep NGN's
tree at/below Blunder's; in eg that crutch is removed and the tree grows at its raw EBF (~2.4 vs ~1.95).

**HONESTY CAVEAT (don't overclaim ordering).** "88% first-move-cutoff is the cause" is UNPROVEN — I did
NOT measure *Blunder's* first-move-cutoff, and [[project_endgame_tree_diagnostic]] previously found
NGN's ordering machinery is *richer* than Blunder's (SEE+counter+continuation-history vs MVV-LVA) with
~equal LMR amount. So the lever is NOT confidently "ordering"; the grounded NEW finding from this
autopsy is the **extension storms** (untested by the 2026-05-31 audit), which directly match the
depth-growth signature.

**Acted on:** the passed-pawn extension was scoped to actually-passed pawns (commit 102b8ce); tactical
28/30, but **node-neutral** (−1.5% eg nodes — in real endgames almost all advanced pushes are already
passers), so it is a *correctness* fix, not the lever (fixed-time non-regression SPRT gating it).

**Next candidates (one-axis, fixed-time SPRT each), best-grounded first:**
1. **tighten the eg extension storms** (the new finding) — check-ext (SEE≥0-gate? / cap), and
   `EXTENSION_BUDGET=24` is very loose (24 net plies/path — effectively uncapped at NGN's eg depths),
   which permits the deep compounding checking lines that match the depth-growth (3.78x→9.16x).
2. **an eg-appropriate replacement for the futility-collapse** — a pruner that fires when evals are flat
   (history-leaf pruning / eg-tuned LMP) to restore the mask futility provides in mg.
3. **eg move ordering** — ONLY after measuring Blunder's first-move-cutoff to confirm a real gap exists
   (NGN's is 88%; if Blunder's is similar, there is no ordering lever here).
