# Speed log — per-node-speed work (node-identical only)

Plan + gate in TODO.md "NIGHT 2026-06-23/24 — AUTONOMOUS SPEED WORK". Worktree `/tmp/ngn-wt-speed`
(detached @ f62bfeb). Instrument: `ngn bench [depth]`. Hard gate: nodecheck EXACT (347138/79362/445102)
+ box-nps win. Every change is recorded here so it's reproducible if the worktree is lost.

## Ground truth (2026-06-23, same Mac, kiwipete 4s, UCI movetime)
- **NGN ~2.0M nps / nominal-depth 46** vs **Counter 3.8 (3051 HCE) ~3.5M nps / depth 17** ⇒ NGN ~1.75× slower per node (approximate; cross-engine node-counting differs). Blunder 7.4.0 nps TBD (UCI output handling finicky).
- Implication: real recoverable per-node gap, DISTRIBUTED (heavier eval + 29% move ordering), not one bloat.

## Baseline (speed bundle = f62bfeb + int16 + bench instrument)
- `ngn bench 13`: 2739090 nodes, Mac ~2.5M nps (best 1134ms).
- box `ngn bench 14`: 3947061 nodes, ~2.13M nps (best 1838ms).
- nodecheck (deterministic, the node-identity invariant): kiwipete 347138, mid 79362, end 445102.

## Changes
| # | change | node-identical? | Mac nps Δ | box nps Δ | verdict |
|---|---|---|---|---|---|
| bench | add `ngn bench` subcommand (main.go + engine/bench.go) | n/a (tooling) | n/a | n/a | KEEP (instrument) |
| int16 | continuation+followup history `int`→`int16` (5.5MB→1.4MB each; gravity self-bounds ±8192 < int16) | YES (857236/2739090/3947061 exact, base==cand) | +0.9% (noise) | +0.9% (noise) | KEEP (node-identical, never slower, helps small-cache CCRL boxes; trivial cost) |
| BCE-ordering | hot move-ordering BCE hints: `to & 63` mask (Destination is a 6-bit field → drops [64] check on 3 history reads) + reslice out/scores `[:n]` in scoreMovesIntoBuffer (drops per-write checks) + selectNextMove (drops inner-loop scores[j] check) | YES (347138/79362/445102 exact, both builds) | +0.5% (noise, M4 OoO hides checks) | **+1.0% (10v10 interleaved both orders: BCE 1837ms vs NOBCE 1856ms; clean separation)** | **KEEP** (node-identical, consistent amd64 win, trivial) |
| int32-scores | ordering `scores [256]int → [256]int32` (all score values fit int32 w/ 2× headroom; halves the selectNextMove scan footprint) | YES (347138/79362/445102 exact) | ~neutral/-0.6% | **-0.75% REGRESSION (12v12: I32 1853ms vs BCE 1839ms, every I32 run slower)** | **DISCARD** — scores array is L1-resident at n~35 (280B), so halving doesn't help bandwidth; the int↔int32 casts on the hot write path cost. Reverted. |
| qsearch-capScore-BCE | reslice captures/capScore `[:captureCount]` in the qsearch capture selection-sort (search.go:2214) — same idiom as the kept ordering BCE | YES (347138/79362/445102 exact) | — | **neutral (8v8: 1844 vs 1842ms, fully overlapping)** | **DISCARD** — qsearch captureCount is small (few captures at the horizon), so its O(n²)+checks are tiny vs selectNextMove's ~35-move sort. Fork over-estimated. Reverted. |
| Move/Clear-masks | `dest&63`/`square&63` masked locals in Board.Move/Clear (bitboard.go) to drop [64] checks on SquareMask/PST/mailbox | YES (347138/79362/445102 exact) | — | **-1.2% REGRESSION (8v8: 1864 vs 1841ms, every run slower)** | **DISCARD** — straight-line code (not a tight loop); the mask/int8-index-conversion ops cost more than the OoO-hidden checks. Reverted. |

## STAGED/LAZY QUIET-SCORING (the "big lever") — IMPLEMENTED + BOX-TESTED → DUD (2026-06-23 night)
Implemented the behaviour-change version (park quiets at a `quietUnscored` sentinel; `materializeQuietScores` fills them in alongside materializeCaptureScores, so the ~88% TT-cutoff nodes skip quiet history reads). Full short suite + perft GREEN (no correctness bug). **Box bench (depth 14) verdict — REGRESSION on BOTH speed axes:**
- **nps 2052K vs 2143K = −4.3% SLOWER per node** (nps normalizes tree size = the clean per-node-speed measure). The park + materialize + re-hoist overhead EXCEEDS the quiet-scoring saved — the "190ms quiet reads" simply aren't a big enough fraction of per-node cost to win back.
- **nodes 4680874 vs 3947061 = +18.6% MORE** (worse ordering: deferred history is read AFTER the TT subtree mutated the tables, so it's a POLLUTED signal — exactly what lazySEE keeps history EAGER to avoid; moveorder.go:390-394 comment).
- net +24% slower to depth. No SPRT needed: slower per node AND a bigger tree ⇒ strictly slower to any depth at real clock, and the ordering is worse (not better moves) — there is no axis on which it wins. Reverted.
- **LESSON: the premise "skip quiet scoring at cutoff nodes = free speed" is FALSE.** Quiet scoring is cheap relative to node processing; deferring it adds overhead and pollutes ordering. CONFIRMS the lazySEE eager-history design. (Mac d13 bench had shown −9.5% nodes/faster — a depth-13 ordering fluke; the box d14 + nps comparison is the real picture. Node-counts are non-monotonic, unreliable filters — Bryan's rule.)

## DECISIVE PATTERN (2026-06-23 night) — node-identical micro-opt lane is EXHAUSTED
4 BCE-class changes box-tested: **only BCE-ordering paid (+1%)** — because selectNextMove is 16% with a TIGHT inner loop where the per-iteration check matters. int32 (−0.75%), qsearch-capScore (neutral), Move/Clear (−1.2%) all failed. **Rule learned: node-identical micro-opts pay ONLY in the hottest tight inner loops; in straight-line code or small-n loops the added mask/cast/reslice instructions exceed the bounds checks the OoO core already hides.** The engine is already lean (incremental accumulator, maintained mailbox/occupancy, lazySEE, shared sliderAtt — no redundant recompute; fork-confirmed). Further node-identical micro-opts are net-negative. ⇒ The remaining per-node gap vs Counter is distributed NECESSARY work; recovering it needs BEHAVIOUR changes (staged-scoring / hybrid-sort / pin-aware legality) gated by real-clock SPRT — Bryan: "make everything defend itself with specific elo / node speed" (node-identical defends via bench; behaviour-change defends via SPRT-elo).

## Settled findings (do not re-litigate)
- **Eval cache (16MB `evalCache[1<<20]`, eval.go:2790) is NET-POSITIVE (+2%)** — recompute pricier than the probe (tested cache-off, node-identical, 1157ms vs 1134ms). KEEP it.
- **Footprint shrinks DON'T pay on our hardware** — M4 (big cache) and the box (96MB V-Cache) both absorb the 11MB history tables. The history int16 was ~neutral for this reason. ⇒ pursue COMPUTE/access-count reductions, which show on any machine.
- **BCE is amd64-only & mostly small** — on cache-MISSING reads (contHist/fuHist, the 190ms) the bounds check is hidden behind memory latency, so BCE there buys ~nothing; it only helps cache-HITTING accesses (scores/out/historyTable). M4's wide OoO hides it entirely (~0.5%); amd64 shows ~1%. Measure speed candidates on the BOX.
- **★ STAGED/LAZY quiet-history scoring (old SPEED-2 "big lever") is NOT node-identical — it's a BEHAVIOUR change ⇒ OUT of the node-identical lane, defer to an SPRT.** Reason: deferring the quiet contHist/fuHist/historyTable reads to the materialize point (oi=1, after the TT move is searched, mirroring lazySEE) reads the tables AFTER the TT move's subtree has MUTATED them (historyTable[piece][to] is updated on cutoffs at any (piece,to); contHist[prevMove] can be hit by a deep node sharing our previousMove). lazySEE works only because SEE is a PURE function of the position; history is not — the existing lazySEE comment (moveorder.go:470) says exactly this ("a deferred read would not be node-identical"). So the 29%-ordering waste at the ~88% TT-cutoff nodes is real but only recoverable as a strength-neutral-ish SPRT candidate, not tonight. Flagged for the SPRT/search lane.

## CERT — BCE bundle vs base, real-clock self-play (2026-06-24)
`ngn_bce.exe` vs `ngn_B.exe` (f62bfeb pure), 10s+0.1s real clock, conc 4, BelowNormal, on the box.
**942W-3439D-936L = 50.1%, elo +0.4 [−9, +10] over 5317 games, 0 flag-outs.** Node-identical pair ⇒
strength-neutral as predicted; a [−3,3] SPRT on a true-0 pair random-walks around zero (never hits a
bound) so it ran to ~5.3k games. **Locks the refactors' real-clock soundness: no crash, no flag-out, plays
dead-equal — the +1% is per-node speed only.** sprt + bins killed, box `ps` verified clean.

## MEMORY / CACHE LANE (2026-06-24) — "did you fix memory bottlenecking? what does L2-only look like?"
Honest answer to the night's gap: NO. The int16 history shrink (5.5MB→1.4MB) was NEUTRAL because it
**never crossed an L2 boundary** on either test machine (M4 P-cluster L2 ~16MB; box L2 1MB but 1.4MB still
spills to the 96MB V-Cache which hides it) — "neutral" meant "didn't cross a cache line," not "memory is fine."

**Hot random-access working set (measured from the structs):**
- **TT 128MB** (`DEFAULT_CACHE_SIZE`, CachedEval 16B × 8.4M) — DOMINATES; exceeds even the box's 96MB V-Cache.
- eval cache 16MB (`evalCache[1<<20]`, 16B). contHist+fuHist 2.77MB (`[13][64][13][64]int16` ×2). Rest <100KB.
- Total ~147MB of hash-indexed, prefetch-defeating random access. On a normal CCRL box (8-32MB L3) nearly
  every TT/eval probe is a RAM miss — we've been measuring on a V-Cache monster that FLATTERS the 128MB TT.

**TT-size cliff — `ngn bench 14 [ttMB]` sweep (added a ttMB arg to bench; nps = per-node throughput):**
| TT MB | box nps | Mac nps |
|---|---|---|
| 1 | 2.33M | 2.62M |
| 16 | 2.11M (−9%) | 2.37M |
| 64 | 1.99M (−15%) | 2.24M |
| 128 (default) | 1.99M (−15%) | 2.30M |
| 256 | 1.87M (−20%) | 2.16M |
⇒ **"search in L2" ≈ +15% per-node on the box** (1MB-resident TT vs the 128MB default), ~18% on Mac, and that's
JUST the TT (eval+contHist still spill L2). The cold-bench node counts are NON-MONOTONIC/noisy (TT replacement
reshuffles ordering) so the to-depth TIME column is NOT a verdict — the clean signal is the nps curve.

**TT PREFETCH (node-identical — pure timing hint, touches no value):**
- Go has no inlinable prefetch intrinsic; added a tiny PREFETCHT0 asm stub (`engine/prefetch_amd64.s` +
  `_amd64.go` decl + `_other.go` no-op) and `Cache.Prefetch(hash)`. amd64-only; arm64 (Mac) is a no-op.
- **Close placement (right before the probe, search.go:1139): DUD, −2.5% box.** The draw/rep work is mostly
  skipped at low halfmove-clock so there's ~no distance to hide the miss, and the un-inlinable asm CALL per node
  dominates. (C++ engines inline prefetch to one instruction; Go pays a call — structural disadvantage.)
- **Make-move placement (prefetch the CHILD entry right after legality, search.go:~1663): KEEP, +0.8% box at
  128MB**, neutral at 16MB. Hash is maintained incrementally in MakeMove (updateHash, position.go:271) so the
  child hash is cheap; extensions/singular/LMR/recursion-setup all run before the child probes = max distance.
  Node-identical (4911585/3947061 bench totals exact vs base). Helps exactly when the TT misses; the V-Cache
  caps the box payoff — on a normal-L3 CCRL box the exposed latency (and this gain) is larger.

**THE REAL LEVER (not yet pulled — behaviour change, needs a WARM-TT real-clock SPRT, Bryan's call):** the 128MB
default is sized for "fewer nodes" with zero accounting for per-node miss latency, and it was tuned on a 96MB
V-Cache box. A smaller TT (16-64MB) is faster per-node AND fits a normal CCRL L3 — but a smaller TT = fewer hits
= more nodes (esp. at longer TC). Net = empirical: real-clock SPRT, a few sizes vs 128MB base, on the box.
Anti-flounder: the bench (cold TT) UNDER-represents warm-game hit rate, so it's a FILTER here, not the verdict.

## PIN-AWARE LEGALITY (the "one remaining per-node lever") — IMPLEMENTED + BOX-TESTED → DUD (2026-06-24)
Replaced the main-search per-move `isInCheck(movingColor)` legality filter (search.go ~1656) with a
precomputed pin/check skip: compute `pinned` + reuse the `inCheck` tag once per node, then skip the
make/unmake isInCheck for provably-legal moves (not in check, not a king move, not pinned, not ep);
fall back to the exact test otherwise. CORRECTNESS fully established (this part is sound): adversarial
review (no must-fix; caught my missed fen.go:75-77 tag-set), a perft-tree "provably-legal ⇒ actually-legal"
walk over 278,392 pin/ep/check-dense interior positions, perft suite green, **node-identity EXACT**
(d13=2739090, d14@16MB=3947061), full short suite green.
- **Box bench A/B vs pre-pin baseline (both node-identical) — REGRESSION on BOTH impls:**
  - magic-lookup computePinned: **−3.1% @128MB, −3.2% @16MB**.
  - table-based computePinned (precomputed betweenBB[64][64] + pseudo-ray tables, no magic): **−2.9% @128MB, −2.5% @16MB** — barely better, so the magic lookups were NOT the main cost.
- **WHY (the real reason):** `MakeMove` ALREADY computes one `isInCheck` per move (the child's InCheck tag,
  position.go:265); the legality `isInCheck` I removed was the *second* one and is cheap + OoO-hidden (early-exit
  attack scan, independent loads the wide core parallelizes). The added per-move `mustVerify` test (×~35 moves/node)
  + per-node `computePinned` cost MORE than the hidden op they save. Same lesson as BCE/int32/staged: adding per-move
  work to save an already-hidden operation loses on the OoO box.
- **REVERTED.** No SPRT needed (node-identical + slower per node = strictly slower to depth). Confirms the prior
  TODO prediction ("likely neutral too" — turned out slightly worse). The pin/betweenBB infra is gone with it.

## DECISIVE PATTERN v2 (2026-06-24) — the box-local node-identical lane is THOROUGHLY TAPPED
Wins (2): BCE-ordering +1.0% (hottest tight loop), TT make-move prefetch +0.8% (hides real RAM latency, ~0 added
work). Duds (4): int32-scores, qsearch-capScore-BCE, Move/Clear-masks, staged-scoring, pin-legality. **The rule that
predicts the sign:** a node-identical change pays ONLY if it (a) lives in the single hottest tight loop with near-zero
added instructions, or (b) hides real memory-latency the OoO core can't (prefetch). It LOSES whenever it adds per-move
or per-node work to eliminate an operation the wide OoO core already hides. NGN's per-node work is lean; the residual
gap vs Counter is distributed necessary work. The one UNTAPPED structural per-node lever is LARGE PAGES (TLB, not
cache — node-identical, +5-10%, gated on a test rig Bryan must enable). See TODO MEMORY SUB-LANE.

## Queue (see TODO for detail)
1. ~~BCE pass on hot ordering~~ — DONE, KEPT (+1.0% amd64, node-identical). Candidate bundle = ngn_bce.exe on box.
2. ~~Staged/lazy quiet-move history scoring~~ — NOT node-identical (see findings); deferred to SPRT lane.
3. Eval micro-opts (pprof -list the hot terms) — node-identical compute/recompute reductions. NEXT.
4. isInCheck redundant-recompute — cache/pass-down node-identically.
5. make-unmake / movegen / SEE micro-opts.

## 2026-06-29 — eval-cache SIZE swept (node-identical), bits=20 is tuned — DUD both ways
Fresh pprof (BenchmarkSearchProfile, depth 11): top self-time = TT `LoadKey` 10.8% (miss latency — already
prefetched), `selectNextMove` 9.3% (optimal hoisted selection sort), `scoreMovesIntoBuffer` 8.4%, eval subtree
24.7% cum. No clean code win — re-confirms "node-identical lane THOROUGHLY TAPPED."
The one untested knob was the EVAL cache size (`evalCacheBits`, eval.go:2782; full-key check ⇒ transparent ⇒
resize is node-identical, proven: depth-16 PV + node count identical 20↔22↔18). Box (9800X3D) fixed-nodes 8M,
4 interleaved trials each, all 7223018 nodes (node-identity re-proven on box):
- bits=22 (64MB): median 4428 ms vs base 4311 ms = **-2.7%** (bigger = locality/L3-pressure dud, the documented rule).
- bits=18 (4MB):  median 4311 ms vs base 4318 ms = **+0.16% (within ~14-33 ms spread = NEUTRAL)**.
⇒ bits=20 (16MB) is at the optimum; size is not a lever. Binaries ngn_ec22/ngn_ec18 on box. Consistent with the
v2 rule: enlarging a cache adds memory work the OoO core loses on; shrinking recovers locality but loses hit-rate,
net flat. The remaining per-node lever is still LARGE PAGES (TLB) — needs a Bryan-enabled test rig.
