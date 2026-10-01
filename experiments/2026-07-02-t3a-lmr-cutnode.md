# 2026-07-02 T3a — LMR cutNode reduction term (pure EBF mechanism)

Pre-registered BEFORE launch. One isolated change, candidate vs immediate base (post-iir3 HEAD 7394668).
TWO-TIER gate: Stage-1 fixed-nodes proxy filter FIRST (a pure ordering/EBF mechanism, so the proxy MAY
reject per CLAUDE.md — with a recorded reopen condition); Stage-2 real-clock only if the filter
passes/neutral. NOT committed; diff backed up at `output/t3a.patch`. Awaiting a diff review before any
launch.

```yaml
id: 2026-07-02-t3a-lmr-cutnode
date: 2026-07-02
change_class: search heuristic (pure EBF / move-ordering reduction) — proxy-then-games gate
hypothesis: reducing late quiet moves one extra ply at cut nodes (LMRCutNode) trims the EBF at the fail-high nodes, reaching greater depth at fixed nodes with no strength cost
base_commit: 7394668 (post-iir3 HEAD; == ngn_base2 engine)
candidate_commit: 7394668 + output/t3a.patch (engine/search.go only, 9 insertions)
mechanism: new tunable LMRCutNode (registered in TunableSearchParams; ALL_CAPS global LMR_CUTNODE, def 0, range [0,3]); in the internal-node LMR reduction block, `if cutNode { reduction += LMR_CUTNODE }` right after the base/isPV reduction. Default 0 => node-identical.
base_binary: ngn_base2.exe sha256 85703c188be79b728909433cb91a2b673efe3b03a6bc6fabc8fce59e1cd58aa5 (already on box; behaves as LMRCutNode=0)
candidate_binary: ngn_t3a1.exe (LMR_CUTNODE baked = 1) sha256 CC82093E86B60232C4BA9D471BF473EF455FD21D525F94F3368ADABC8BAEBDF7. cmd/sprt cannot setoption, so the value is baked into the binary; the committed default stays 0 (tunable kept UNCOMMITTED — commit only on a keep, per the dead-SPSA-dim guard).
node_identity: PROVEN. build/ngn_t3a (default 0) == build/ngn_head EXACTLY on all three nodecheck positions (kiwipete d12 246188, mid d12 168053, end d16 806334). LMRCutNode=1 differs (kiwipete d12 304079 != 246188) => a real behavior change, not a /2048-style no-op.
nodecheck_note: the nodecheck.sh hardcoded baselines (270446/81006/840033) are STALE (pre-iir3); the real post-iir3 HEAD counts are 246188/168053/806334. Re-lock recommended in a SEPARATE hygiene commit (not part of T3a).
suite: go test -short ./engine + -race ./engine + -short ./... all GREEN with T3a in tree.
openings: canonical sprt_openings (5000 lines) sha256 974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222
```

## Mechanism (the single change)
`engine/search.go`: (1) new global `LMR_CUTNODE = 0` in the tunable var block; (2) `{"LMRCutNode",
&LMR_CUTNODE, 0, 0, 3}` in `TunableSearchParams` (auto-advertised + applied via uci.go setoption, and a
new SPSA dimension); (3) in the internal-node LMR reduction (the quiet-move `reduction` block, after the
Stockfish base reduction and the `!isPV`/improving adjustments), `if cutNode { reduction += LMR_CUTNODE }`.
At the default 0 this is `reduction += 0` — provably and empirically node-identical to HEAD. A cutNode is
expected to fail high, so its late quiet moves are even less likely to be the PV; reducing them one extra
ply is the standard Stockfish/Ethereal cutNode LMR term (prior +6.2/+10 [S #119]).

## Pre-launch integrity — verified on the ACTUAL box filter binaries (2026-07-02, before launch)
- **2a — candidate** `ngn_t3a1.exe` (LMRCutNode baked=1) sha256 `CC82093E86B60232C4BA9D471BF473EF455FD21D525F94F3368ADABC8BAEBDF7` → box kiwipete d12 = **304079** (== the setoption-value-1 count; confirms the baked binary is value-1, NOT an accidental value-0 A/A). Native build agrees exactly.
- **2a — base** `ngn_base2.exe` (post-iir3 HEAD) sha256 `85703c188be79b728909433cb91a2b673efe3b03a6bc6fabc8fce59e1cd58aa5` → box kiwipete d12 = **246188** (== native HEAD; confirms base == post-iir3 HEAD, and cross-platform determinism box==native). 2a PASS: candidate ≠ base (real change), base == HEAD.
- **2b — cap safety:** the cap `if reduction >= depth-1 { reduction = depth-2 }` bounds every increment, so `reducedDepth = depth-1-reduction ≥ 1` even at LMRCutNode=3 (the tunable max); empirically value-3 searches cleanly (kiwipete d12 308872, valid bestmove e2a6). 2b PASS.
- Counts measured by driving the box `.exe` with `debug on` + `go depth 12` (the `info string stats … nodes` line); fixed-nodes-style and CPU-contention-independent, so safe to measure before/alongside the filter.
- **Stage-1 filter LAUNCHED 2026-07-02 ~05:27:59 box time** (`ngn_t3a1.exe` vs `ngn_base2.exe`, `-nodes 128000 -concurrency 8`, → `t3a_filter_out.txt`). Healthy start: 16 engines + sprt, **~3680 games/hr** (fixed-nodes; ~2.7h to the 10000 cap ≈ 08:10 box time barring an earlier bound cross), all-normal terminations (no flag concept at fixed nodes), early elo ~neutral (G144 +4.8 [-52,+61]).

## Two-tier decision rule (predeclared)
- **Stage-1 — fixed-nodes proxy filter (box, deterministic, cool):** candidate `ngn_t3a1` (LMRCutNode=1)
  vs base `ngn_base2`, `-nodes 128000 -concurrency 8`, elo0=-3 elo1=+3, maxgames 10000 mingames 300 (the
  exact iir3/ttage fixed-nodes form). T3a is a pure ordering/EBF mechanism, so the proxy MAY reject:
  **clearly negative (point est < -3, or H0-trending) → REJECT**. REOPEN CONDITION = reopen ONLY after a
  move-ordering-precision / FMC fix lands (raising FMC ~84%→~90%, the documented gate on safe reduction) —
  over-reduction is NOT cured by MORE reduction, so value 2/3 are the WRONG follow-ups for a NEGATIVE
  value-1 result. **Neutral/positive → proceed to Stage-2** (value 2/3 are follow-ups ONLY if value 1 is
  positive/neutral). The filter credits ordering/EBF and is concurrency-free; a pass is necessary, not
  sufficient (it does NOT gate strength).
- **Stage-2 — real-clock verdict (box, the KEEP gate):** `ngn_t3a1` vs `ngn_base2`, `-tc 10+0.1
  -concurrency 8`, SPRT elo0=-3 elo1=+3, adjudication OFF, on the A/A-validated c8 config. pLLR ≥ +2.94
  KEEP; ≤ -2.94 REJECT; capped-positive (point ≥ +1, pLLR > 0, no regression) → PROVISIONAL keep #2 under
  batch certification; capped-nonpositive → shelve. Any candidate flag-outs above the A/A baseline (0) →
  HALT + investigate.

## Value
The first filter tests LMRCutNode=1 (the conservative, most-common +1-ply cutNode reduction). If value 1
is neutral, value 2 is the follow-up (range allows up to 3). SPSA round-1 (T8) can later co-tune it as a
real dimension.

## Stage-1 verdict (2026-07-02) — REJECT; move-ordering is the prerequisite

The fixed-nodes proxy filter (ngn_t3a1 LMRCutNode=1 vs ngn_base2, -nodes 128000 c8) REJECTED. Verbatim RESULT:

```
=== RESULT (47m39s) ===
Games: 2984   W-D-L: 650-1616-718   score: 48.9%
Elo(new - base): -7.9   95% CI [-20, +5]
LLR: -2.56   bounds [-2.94, 2.94]
Pentanomial [LL 59  LD 402  {LW,DD} 645  WD 320  WW 66] over 1492 pairs
Penta Elo: -7.9   95% CI [-16, +0]   pLLR -2.87  (THE decision stat; trinomial above is secondary)
Verdict: H0 ACCEPTED: new is NOT better (<= -3 ELO)
DONE_EXIT_0
```

**Finding: adding cutNode LMR reduction is BLOCKED on move-ordering precision.** Value-1 regressed -7.9 on the
cheap fixed-nodes proxy (2984 games, ~48 min, NO real-clock slot spent — the two-tier gate worked exactly as
designed). This is the tree-shape mechanism in action: NGN's FMC is ~84% (vs ~90% elite), so reducing MORE at
cut nodes over-reduces mis-ordered good moves, which then fail low SILENTLY (the LMR re-search only rescues
alpha-BEATERS). Over-reduction is NOT cured by more reduction, so value 2/3 are NOT retried.

**REOPEN CONDITION (binding):** re-test the cutNode LMR term ONLY after a move-ordering-precision / FMC fix
raises FMC toward ~90% (the T10 ordering-completeness lane — killer-reset, 4-ply conthist, main/1-ply weights).
Until then the whole LMR-reduction sub-lane (T3a/b/c) is gated behind ordering.

**Disposition:** SHELVED. Working tree reverted to HEAD (`git restore engine/search.go`; the diff is kept at
`output/t3a.patch`). Tunable never committed (no dead SPSA dim). Box clean (self-terminated DONE_EXIT_0, ps
empty). TRIED-LEDGER + TODO updated (T3a rejected; T10 promoted to LMR-lane prerequisite).
