# NGN — Everything We've Tried (consolidated ledger)

Last updated 2026-07-24. This is the single inventory of every lever attempted, with verdicts,
so we stop re-grinding dead ends. Detailed records are in the per-experiment files and the memory
topic files; this is the index + verdict.

> **2026-09-05 search-tuning result:** T17 plus the completed24,000-game T8
> vector passed its repaired-base confirmation:734 games, paired+38.02
> [+20.77,+55.45], H1, zero flags/errors. The search-tuning lane was not exhausted.
> This accepts the tested interaction, not individual parameter contributions.
> [Completed result](2026-09-05-t8-qcap-confirmation.md). The separate classical
> evaluation fit was shelved at its inconclusive cap; that does not close all tuning.

> **2026-09-04 correctness correction:** the historical claim below that the
> "correctness era really is over" was false. Independent probes subsequently
> found EP parsing, terminal/draw/search-clock, tuner/model/filter/checkpoint,
> illegal internal castling, and qsearch capture-budget/TT defects. Existing
> game verdicts retain their recorded scope; an old clean-audit assertion cannot
> close correctness investigation. See the [current review evidence](2026-09-04-review-second-pass.md)
> and `TODO.md` for live priorities.

> **2026-07-01 CLOSURE-QUALITY AUDIT (`2026-07-01-method-revamp.md`) — corrections:**
> a closure below is only binding if it is POST-reset games evidence meeting the decision
> table. Re-graded: joint-SPSA = INVALID closure (iter-50 kill, param movement mathematically
> impossible, ~100 games); "LMR/NMP both directions regress" = PRE-RESET evidence (corrupted
> instrument), thresholds never game-tested post-reset; butterfly history = cold-proxy
> false-reject candidate (manifest admits under-credit); corrhist = closed against a
> MIS-PLUMBED base (correction never applied at qsearch stand-pat; learning target corrupted
> by S7 TT-refinement) — reopened as T4. Draw scaling below is STALE: it is LIVE in HEAD
> (eval.go:2684 drawFactor + 50-move damp, tests in eval_ocb_test.go).

## The pattern, in one line

**Every Elo gain in this engine's history has been a CORRECTNESS fix. Every speculative
eval / search-heuristic / tuning play has been null or negative.** That is the plateau.

**[DEAD — DO NOT REASON FROM THIS HEADLINE. It describes the PRE-revamp era only.]** Falsified by
seven post-revamp keeps of ordinary search/eval heuristics, none of them correctness fixes: T4 +9.1,
T4np +6.4, T1b +13.1, T4c +3.7, iir3 +2.0, T1e +6.6, **T5 +20.2** (biggest single keep of the
campaign). And the falsification is not self-play-only: the **Batch-1 certification measured
+43.2 [+26,+61] real-clock** for the first five of them against their pre-batch base (2026-07-18,
745g, pLLR +3.09) — *more* than the +34.3 they claimed. The old headline was a true description of a
broken measurement regime, not a law about this engine. Its zombie form ("everything is exhausted /
local optimum") already failed one closure audit (`2026-07-01-method-revamp.md`); do not resurrect it.

Strength trajectory (CCRL, real-clock): 2380 → 2449 → 2632 → 2669 → **2666** [2627,2706] (multi-family, 2026-06-28)
→ **2689** [2649,2729] (2026-07-08 re-pin) → **2704** [2665,2743] (2026-07-19 milestone re-pin,
exact-replica instrument, `2026-07-19-gauntlet-repin.md`). Each step is a point-signal inside the
±40 delta-CI floor, so formally flat-to-up, never a significant single jump — the powered transfer
instrument is the batch certification, not the pin. The climbs were correctness fixes + the time-mgmt
fix; flat Jun-10→Jul-1 despite weeks of eval/search work; **climbing steadily since the 2026-07-01
revamp** (+34.3 certified at +43.2, then +26.8 more claimed in Batch 2 awaiting certification).

---

## ✅ WHAT ACTUALLY MOVED THE NUMBER (kept)

| Change | Effect | Class |
|---|---|---|
| SEE evaluation was garbage → fixed | **+161 Elo** (ddcf82f) | correctness |
| NMP was OFF at every cut node (C3) → fixed | **+138 Elo** (f04eb0d, batched S6/S7/RT1) | correctness |
| Clock-interrupt committed half-searched moves → fixed | blunder 9.7%→3.2%, tactical 29→30/30 (362fd24) | correctness |
| IID & singular extensions silently broken (re-entrancy + self-cutoff) → fixed | restored both | correctness |
| Time management: only ~25% of soft clock used → soft-stop projection fix + easy-move-shrink removal | 25%→57% usage, +1 depth, **+~37 Elo** measured | correctness-adjacent |
| King safety E2 (enemy attacker material) | **+18–20 Elo** | eval-correctness |
| 2026-06-28 HARD RESET: P0 measurement bugs (stopped-search state pollution, qsearch-in-check, UCI stop race, Hash resize, SearchFixed authority, root-rep, EP-hash, promotion safety) | strength FLAT but instrument now trustworthy | correctness/measurement |
| Mobility per-count TABLES (C1) | kept (universal 10/10 in research) | eval shape |
| TT de-padded to 16B (was 64B, 48B wasted) | node-identical, 4× slot count | speed |
| TT make-move prefetch | **+0.8% NPS**, node-identical | speed |
| BCE-ordering pass on hot loop | **+1.0% NPS**, node-identical | speed |
| passersOf + pawn-shield micro-opts | ~+6.5% node-identical (early) | speed |
| **T4 / T4np / T4c corrhist family** (on the re-plumbed base) | **+9.1 / +6.4 / +3.7** penta, all H1 | search/eval heuristic |
| **T1b** soft-budget stability scaling (wire the dead `stableIters`) | **+13.1** penta [+3,+23], H1 | search heuristic |
| **T1e** node-fraction best-move effort time scaling | **+6.6** penta [-1,+14], H1 | search heuristic |
| **T5 aspiration modernization** (scaled init window, progressive widening, never-discard) | **+20.2** penta [+7,+33], H1 — **biggest single keep of the campaign** | search heuristic |
| iir3 phase-guarded cutNode-IIR | +2.0 penta, provisional → **certified** 2026-07-18 | search heuristic |
| **Batch-1 certification** (the 5-keep composite vs its pre-batch base) | **+43.2 [+26,+61]** real-clock, pLLR +3.09, 745g | whole-batch gate |

Takeaway **[REWRITTEN 2026-07-24 — the old one was "only correctness fixes and tiny speedups repeat",
which the bottom seven rows falsify]**: two things produce Elo here. (a) Correctness fixes — still the
only source of +100-class jumps, and still worth hunting first. (b) **Transplanting standard techniques
NGN was missing or had in a degenerate form**, measured one at a time against an immediate base on a
trustworthy instrument — median keep ~+4-7, occasional +13-20, and they *compound and certify*
(+34.3 claimed → +43.2 measured). What does NOT produce Elo is re-tuning knobs that are already near
their local optimum (every single-axis retune, joint-SPSA at its invalid kill, the linear texel basis).
The distinction is "is this mechanism missing/degenerate?" vs "is this constant slightly off?" — and
the T7 shelve (2026-07-24) shows the failure mode of getting that wrong: a change that only *looks*
like a mechanism add but moves the tree 0.001% measures nothing in 8 hours of box time.

---

## ❌ SEARCH heuristics / pruning / reductions — tried, did NOT move it

| Lever | Verdict |
|---|---|
| LMR continuous history scaling (replace crude ±1) | REJECT: /2048 flat (-8), /256 regressed (-45). `2026-06-28-lmr-conthist.md` |
| cutNode-based LMR reductions (extra reduction at cut nodes) | **REJECT -7.9** [-16,0] pLLR -2.87, fixed-nodes proxy 2984g (T3a value-1, `experiments/2026-07-02-t3a-lmr-cutnode.md`), post-reset **BINDING**. Blocked on move-ordering: NGN's FMC ~84% means MORE cutNode reduction over-reduces mis-ordered moves (silent fail-lows). REOPEN only after an ordering/FMC fix (T10); value 2/3 NOT retried (over-reduction isn't cured by more reduction). The [S] +6.2/+10 prior assumed elite ordering. |
| LMR / NMP single-axis retunes (both directions) | 3 SPRTs / 0 gains — both directions regress (well-tuned local optimum). **[pre-reset — NON-BINDING, reopened]** |
| Joint-SPSA over all 11 margin params (coordinated) | NO-OP: zero gradient at iter 50, params pinned. `2026-06-28-joint-spsa.md`. **[INVALID closure (iter-50 kill) — reopened as T8]** |
| Search-param SPSA (earlier) | CLOSED (flat). **[reopened / superseded by T8]** |
| J1 extension×LMR reduced-scout base fix | REVERTED after **65 interrupted games**, −59.4 [−144,+25], pLLR−0.28; **no formal verdict**, pre-reset. Old endgame tree+83% is a cost warning. Current-base interaction review reopened with that limitation (`2026-09-05-depth-policy.md`). |
| Dynamic NMP | REJECT. **(pre-reset evidence non-binding; adjacent to the queued T12 NMP polish)** |
| Multicut (return-beta) | REJECT (the A3 lesson). **[pre-reset REJECT; reopened as T9, prior +5.8 [S]]** |
| Deeper futility (margin+depth) | blew up endgame tree +141% |
| RFP / futility / LMP as the "endgame tree" fix | every standard pruning knob empirically refuted; eg-tree gap was a mislabeled-position artifact anyway |
| TT capacity as endgame fix | bites the MIDDLEGAME (-46%), not endgame |
| qsearch stand-pat fail-high TT store (T13) | **SHELVED -0.4 [-6,+5] at the 8000g cap** (pLLR -0.32, 0/0 flag-outs; `2026-07-09-t13-qsearch-ttstore.md`). NGN already stores the CAPTURE fail-high (a0fdda41/S3); this stand-pat-only add is marginal AND carries a +13-39% fixed-depth nodecheck headwind (depth-0 store flood + the un-gated S7 margin reshaping at search.go:1368-1374). **REOPEN** only after the S7 refinement is depth-gated / excludes depth-0 qsearch entries (queued **T16**), or a TT-clustering change (T11) cuts depth-0 eviction pressure. The audit's "+7.0 [S #126] / first qsearch write" premise was FALSE (qsearch has written capture fail-highs since a0fdda41), so the prior was downgraded pre-launch. |
| History pruning modernization: depth≤3→≤6 cap, flat −1000 → depth-scaled −512·depth, killers/counter exempt (T7) | **SHELVED −0.2 [−5,+5] at the 8000g cap** (pLLR −0.15, post-mingames envelope [−0.92,+1.92] — never within a full unit of either bound, 0/0 flag-outs, 8h19m28s, DONE_EXIT_0; `2026-07-19-t7-history-pruning.md`). **The +23.7 [E] CMH-pruning prior is UNTESTED by this run.** The candidate was near-inert: fixed-depth nodecheck moved **0.001%** (+3/0/−2), because −512·depth is a MIXED edit — *more* pruning at depth 1 (−512 vs −1000), neutral at 2, *less* at 3 (−1536), depths 4-6 newly enabled but demanding −2048…−3072 against gravity-bounded (`historyMax` 8192) entries clustered near 0 — and the killer/counter exemption claws back prunes at every depth. The three effects cancel. **The lane stays OPEN; only these constants are closed.** REOPEN as a re-derived variant (smaller coefficient e.g. −256·depth; or the depth-cap extension ALONE with flat −1000; or the exemption isolated), preferably as T8 SPSA dims. **Process gate earned here: a pruning/extension candidate must move nodecheck ≥~1% on a canonical position BEFORE it earns box time** — this manifest predicted the 0.001% delta and launched anyway. |
| S7 static-eval-refinement depth-gate (T16, T13's reopen) | **SHELVED -0.6 [-6,+5] at the 8000g cap** (pLLR -0.50, 0/0 flag-outs, DONE_EXIT_0, 8000g; `2026-07-10-t16-s7-depth-gate.md`). Added `&& ttDepth > 0` to the S7 refinement guard (search.go:1368) so depth-0 qsearch entries no longer overwrite the pruning staticEval feeding RFP/futility/NMP. Fixed-depth nodecheck MOVED (kiwipete +30%, mid -31%, end -17% — the change is live) but strength is neutral-to-slightly-negative standalone. **REOPEN** only bundled WITH the T13 qsearch stand-pat store as a single paired change — T16 was T13's reopen and neither half helps in isolation. The S7-refinement / qsearch-store neighborhood is now **tapped** (T13 + T16 both shelved). |

Mechanism (2026-06-29 diagnostic): NGN's LMR re-search rate is ~1.1% (reductions ultra-conservative,
huge headroom) but every "reduce/prune more" regressed — because the re-search only catches moves that
BEAT alpha, so over-reduced good moves fail low SILENTLY. The gate is move-ordering precision (FMC ~84%,
not the ~90% of strong engines). See `2026-06-29` tree-shape measurement below.

---

## ❌ EVAL — tried, did NOT move it

| Lever | Verdict |
|---|---|
| Texel tuning, 10 untuned aux weights | NO-OP (±1-6cp, basis already at optimum) |
| Texel tuning, 787-param SF-d16 PST | NO-OP (proxy-artifact, never beat games) |
| Texel tuning, 1000-param | null |
| Linear texel tune, L2=0 | DEAD BY GAMES (= -440, commit e3e56f6) |
| Material weight tune (E1) | null |
| Eval optimism (reads +80 while SF says -320; ~160cp off SF) | STRUCTURAL/concentrated, outside the linear basis — texel CANNOT fix it |
| King-safety over-optimism fix (-38cp via zzbias) | gate-1 passed; SPRT/gauntlet never closed |
| Passed-pawn under-parameterization (flat-20, no rank knob) | identified as structural-add candidate; not confirmed by games |

Loss autopsy (30 losses vs Counter): 28 positional (slow-bleed + eval-blindspot), 1 tactical — looked
like "eval is the lever," but the 2026-06-28 search-bound proof reframed these as HORIZON errors (shallow
search trusting an optimistic static eval), i.e. downstream of search depth, not texel-fixable.

---

## CORRECTION HISTORY

| Table | Verdict |
|---|---|
| Pawn correction history | KEPT (in HEAD; the one corrhist that holds) |
| Pawn corrhist RE-PLUMB (T4: apply correction at qsearch stand-pat + learn from the pre-S7 CORRECTED static, gravity form) | **KEPT +9.1 [+0,+18] penta, H1 ACCEPTED** — real-clock 10+0.1 c8, pLLR +2.80, 0/0 flag-outs / 2325g (`2026-07-02-t4-corrhist-replumb.md`); FIRST outright SPRT accept of the campaign. The prior "corrhist closed" verdicts were all measured on the MIS-PLUMBED base (correction never applied at qsearch stand-pat; learn target corrupted by S7 TT-refinement). |
| Non-pawn corrhist RE-PLUMB (T4-nonpawn: the ORIGINAL @6245 form on the fixed T4 plumbing — same qsearch stand-pat application + pre-S7 corrected learn target) | **KEPT +6.4 [-1,+14] penta, H1 ACCEPTED** — real-clock 10+0.1 c8, pLLR crossed +2.94 at G3796 (peak +2.97, four lines ≥ bound, drain to +2.80), 0/0 flag-outs / 3804g (`2026-07-02-t4-nonpawn.md`). SAME @6245 form the two rows below rejected → the plumbing was the confound, not the corrector. |
| Aux corrhist (C-vs-B, 2026-06-23) | REJECT — and the run was INVALIDATED mid-flight by a ClearHistoryTable cross-game leak (Codex caught it), since fixed |
| Non-pawn corrhist @ weight 6245 (OLD, mis-plumbed base) | ~~REJECT -9.5 [-20,+1], 1569g~~ **SUPERSEDED** by the T4-nonpawn KEEP above: identical @6245 form on FIXED plumbing = **+6.4 H1**. `2026-06-28-nonpawn-corrhist.md` |
| Non-pawn corrhist @ weight 3000 (halved, mis-plumbed base) | ~~REJECT -3.8 [-11,+4], 3000g~~ **SUPERSEDED** — the halve was an over-correction remedy for the broken base; unnecessary on fixed plumbing (the @6245 form wins there) |
| Minor-piece corrhist (T4c: the THIRD corrector — knight\|bishop king-excluded key, learns from the pre-S7 corrStaticEval on the fixed T4 plumbing) | **KEPT +3.7 [-2,+9] penta, H1 ACCEPTED** — real-clock 10+0.1 c8 adjudicated, pLLR +2.98 (peak +3.01 @G7840, c8 drain), 0/0 flag-outs / 7842g (`2026-07-03-t4c-minor-corrhist.md`). Corrhist family now **3-for-3 on fixed plumbing** (pawn +9.1 → non-pawn +6.4 → minor +3.7 — diminishing as predicted, still H1). Bundled a test-only TestIIDIsWired FEN refresh (zero binary impact). |

**Closure (2026-06-29): [SUPERSEDED — the 2026-07-02 T4 Update below overturns this.]** additive
eval-correction is DOWNSTREAM of the search fix. corrhist learns from `(search - static)`, so a
search-bound engine is a poor teacher → learns noise. Matches "conthist pays ≥3200".

**Update (2026-07-02, T4):** the 2026-06-29 closure was measured on a MIS-PLUMBED base (correction never
applied at qsearch stand-pat; learn target corrupted by S7 TT-refinement). Re-plumbed properly, PAWN
corrhist is **+9.1 H1** and NON-PAWN corrhist (identical @6245 form, STRICT H1-only gate) is **+6.4 H1 /
3804g** — the corrhist family is now **2-for-2 on fixed plumbing**. The "corrhist is negative / downstream
of search / poor teacher" belief was a PLUMBING CONFOUND, not a property of the corrector: the same @6245
non-pawn code that lost -9.5/-3.8 on the broken base WINS +6.4 on the fixed one. This **CLOSES the "non-pawn
corrhist is negative" lane** (it is positive) and de-risks the family — minor-piece corrhist (**T4c**) is
next, same conventions + red→green test template. The remaining caution: the family stacks on ONE eval, so
watch for diminishing returns / interaction when T4c lands (certify via games, not by assuming additivity).
**[2026-07-09: T4c LANDED — KEPT +3.7 H1 (7842g, pLLR +2.98). Family is 3-for-3; the predicted diminishing
returns (+9.1 → +6.4 → +3.7) held, no adverse interaction, no flag-out spike.]**

---

## MOVE ORDERING

| Lever | Verdict |
|---|---|
| Full history machinery (main + continuation + followup + capture history, killers, counters) | present, mature |
| BCE-ordering pass | KEPT (+1.0% NPS) |
| qmvv ordering | +22.6 (early keep) |
| Staged / lazy quiet-move history scoring | REJECT (not node-identical) |
| MVV/LVA "−129" folklore | VOIDED |
| 4-ply continuation history, uniform 1:1:1:1:1 weight (T10b) | **SHELVED -2.4 [-8,+3] at the 8000g cap** (pLLR -2.07, scan max -0.02/min -2.47 — never crossed ±2.94, 0/0 flag-outs, DONE_EXIT_0; `2026-07-19-t10b-conthist4.md`). The **+13.1 [S #139] prior did NOT transfer** — the first straight miss in the T10 lane. Hypotheses, none isolated: uniform weight dilutes the sharp 1/2-ply signal; NGN's existing 2-ply conthist already carries most of the ordering value at ~84% FMC; +10.56 MiB of new tables taxes real-clock cache locality (mixed nodecheck +7.7/-23.7/+15.3). **REOPEN only as a re-derived variant** — non-uniform ply weights (T10c/SPSA) or 3-ply only — **never a uniform-weight re-run.** |

**2026-06-29 measurement: FMC ~84% (mid 82%, eg 86%, tac 85%) — strong engines hit ~90%+.** This is the
identified next lever (the ordering precision gate, FMC as fast proxy). NOT yet attempted.

---

## PER-NODE SPEED (node-identical lane — "be the fastest Go engine")

Wins: BCE +1.0%, TT prefetch +0.8%, TT de-pad, passersOf/pawn-shield. **Lane is THOROUGHLY TAPPED.**

| Dud | Verdict |
|---|---|
| int16 history tables (4× smaller) | node-identical but NPS-FLAT — NGN is NOT memory-bound (96MB V-cache holds the 11MB tables) |
| eval-cache size sweep | bits=22 = -2.7% (locality), bits=18 = neutral → bits=20 already tuned. `speed-log.md` |
| int32 scores, qsearch-capScore-BCE, Move/Clear-masks, staged-scoring, pin-legality | all DUDS (added per-node work the OoO core already hides) |

Rule: a node-identical change pays ONLY in the single hottest tight loop with ~0 added instructions, or
when it hides real memory latency (prefetch). **Untapped:** LARGE PAGES (TLB, +5-10%, needs box admin / SeLockMemoryPrivilege).
"allocation-free hot path" was silently FALSE for ages (probcut) — fixed + locked by TestHotPathAllocationFree.

---

## TIME MANAGEMENT — KEPT (a real win)

Soft-stop projection fix + easy-move-shrink removal: clock usage 25%→57%, +1 depth, ~+37 Elo (transferred).
Counter still spends ~98% of clock vs NGN ~70-80% — a residual time-mgmt gap was identified but the 1-line
soft-stop projection refinement was STAGED, then CLOSED post-reset by T1a (below).

**2026-07-02 — the naive projection-drop 1-liner is CLOSED (post-reset, binding).** T1a fired the soft
stop at `elapsed >= soft` (spending the FULL soft budget vs ~2/3 under the projection): real-clock c8
SPRT, A/A-validated (penta -0.7 [-11,+9], 0/0 flag-outs) = **REJECT -7.8 [-15,-0], pLLR -3.22, 0/0
flag-outs / 2864g** (`experiments/2026-07-02-t1a-softstop.md`). The projection is LOAD-BEARING: spending
more time per move regressed strength with ZERO flag-outs — a uniform overspend starves the moves that
matter (the removed easy-move-shrink failure mode). The +31/+29 [Stash v26] prior belongs to the
STABILITY-SCALED soft target (T1b: keep the projection, spend more only while the decision is volatile),
NOT this one-liner.

**2026-07-03 — T1b STABILITY-SCALED soft target is the win the +31 prior predicted. KEPT (a real Elo).**
Wired the dead `stableIters` READ (search.go already MAINTAINED it via `ReportCompletedIteration`; only
`shouldStopTournamentSearch` never read it) to SYMMETRICALLY scale the soft budget: `stabilitySoftFactor`
= `max(0.85, 1.30 − 0.10·stableIters)` — extend ×1.30 while the root best move is volatile, shrink ×0.85
once it has settled (crossover 1.0 at stableIters 3), applied at the soft-limit check only; the ×1
projection, hard ceiling, and emergency floor are UNTOUCHED. Real-clock 10+0.1 c8 adjudicated SPRT =
**+13.1 penta [+3,+23], pLLR crossed +2.94 at G2225 (peak +2.98, drain to +2.90), H1 ACCEPTED, 0/0
flag-outs / 2233g** (`experiments/2026-07-03-t1b-stability-scaling.md`). This is the
REDISTRIBUTE-not-uniform lever T1a's failure pointed to: keep the load-bearing projection, spend MORE only
on volatile (hard) moves and LESS on settled ones — distinct from the removed one-sided easy-move shrink
(shrink-only → the 25%-clock bug). Confirms the revamp-doc symmetric spec and the CounterGo three-regime
direction. Follow-on = **T1c** (score-drop / eval-direction regimes = the rest of the Stash-v26 +31 /
CounterGo three-regime package; needs a second signal — score tracking — beyond bestmove-stability). The
"every speculative search/eval play is null or negative" headline is now falsified three times over (T4
+9.1, T4-nonpawn +6.4, T1b +13.1) — all standard-technique transplants under the revamp method.

**2026-07-10 — T1c score-drop / eval-direction regime is SHELVED (clean reject).** Added the third
CounterGo/Stash-v26 regime — a second, orthogonal time signal composed multiplicatively with T1b's
stability factor: `scoreDropSoftFactor` (clamped ramp 1.0 at ≤50cp drop → 1.25 at 150cp, extend-only) that
lengthens the soft budget when the root score is falling across completed iterations. Real-clock 10+0.1 c8
adjudicated SPRT vs the T4c base = **REJECT −4.8 penta [−11,+1], pLLR −2.98 (crossed the −2.94 bound),
0/0 flag-outs / 5763g** (`2026-07-09-t1c-score-regimes.md`). Clean statistical reject, not a cap. The
score-drop signal as a pure orthogonal extend-on-drop multiplier is marginally harmful; T1b's
best-move-stability scaling was the whole time-management win. REOPEN only if the score-drop signal is
re-derived in a materially different form (e.g. gated on low best-move stability, not an independent
orthogonal multiplier). Matches the T1a signature (time overspend regresses with zero flag-outs).

---

## TUNING METHODS — all exhausted **[SPSA rows reopened/superseded by T8, 2026-07-08; texel stays closed]**

- **Texel/linear eval tuning: CLOSED.** Multiple objectives × multiple param sets, all null/dead-by-games. Don't reopen without a NEW (non-linear / structural) basis.
- **Joint-SPSA over search margins: NO-OP** (zero gradient). The coordinated retune single SPRTs can't do — and it found nothing to do. **[INVALID closure (iter-50 kill) — reopened as T8]**
- **Search-param SPSA: CLOSED** (flat). **[reopened / superseded by T8]**

---

## ENDGAME

- "2.5–5× bigger endgame tree" — REFUTED as a mislabeled-position artifact (the anchor was tactically won). Quiet endgames are ~0.85× Blunder (smaller), evals agree. No quiet-eg gap exists.
- Mop-up mating gradient (KR-vs-K converts) — flat self-play SPRT, PARKED (git stash on main).
- Endgame draw-scaling — **LIVE in HEAD** (`eval.go:2684` `drawFactor` + 50-move damp, ported from CounterGo; the "absent / STAGED / never closed" note is STALE).

---

## NNUE — not attempted (correctly)

Gated ≥2800 (Bryan, emphatic). The search-bound proof says it's the wrong tool anyway: a search-bound
engine can't teach an eval network (poor-teacher problem). Pure-Go HCE proves classical 2800-2900
is reachable without it, and the **Go-HCE ceiling is far higher (VERIFIED 2026-07-08, CCRL 40/15 1CPU lists
dated 2026-07-04): the last pure-HCE Counter is 4.1 = 3157 (NNUE began at Counter 5.0, Nov 2023 — NOT 4.x);
Stash 37 (Apr 2025) = 3374, still pure HCE and actively developed → the classical ceiling is demonstrated at
≥3157.** Anchor-label caveat (VERIFIED 2026-07-08, CCRL 40/15 1CPU, list dated 2026-07-04): counter-3.8 CCRL = 3050
(`ratings.json`'s 2994 ≈ Counter 3.7 at 2997); `ratings.json` kept at 2994 for pin-to-pin comparability —
the absolute pin therefore reads ~11 Elo conservative (19.4% pool weight × ~56); do NOT change
`ratings.json` mid-campaign.

---

## INSTRUMENTS BUILT (the measurement scaffolding)

cmd/sprt (self-play SPRT, pentanomial), cmd/gauntlet (absolute CCRL, real-clock -tc), cmd/oracle + `make acpl`
(SF-MultiPV cp-loss eval pre-filter), cmd/texel, cmd/spsa, cmd/zzbias / lossxray / sfpv (eval-bias),
cmd/blunder-finder, scripts/h2h.sh (node-identity + NPS), the gaming-PC SPRT mill (9800X3D), the `sd_*`
EBF-attribution dump (debug on). Hard-won lesson: self-play SPRT gains are transfer-MIRAGES vs the gauntlet;
fixed-nodes/ACPL/MSE/param-movement are FILTERS, never verdicts; a verdict = real-clock GAMES of the actual binary.

**[2026-07-24 amendment — "transfer-MIRAGES" is too strong now.]** It was a fair read of the pre-reset
instrument, but the Batch-1 certification measured **+43.2** real-clock against a claimed **+34.3** of
pooled self-play keeps — transfer *exceeded* the claim. The disciplined version of the lesson survives: a
single self-play SPRT is not a transfer guarantee, and the pin has a ±40 delta-CI floor that no single keep
can clear. The batch certification exists precisely because it is the *powered* transfer instrument, and it
has now fired once and passed. Later additions: `scripts/boxsprt.sh` (M3, one-command box mill),
score adjudication (M5, validated non-distorting at +3.1% throughput), parallel SPSA `-pairs K` (M4).

---

## META-LESSONS (why the plateau is real)

**[REVISED 2026-07-24. The section title "why the plateau is real" is itself now wrong — the plateau
broke on 2026-07-01. Lessons 1 and 2 are superseded; 3-5 stand with amendments.]**

1. **SUPERSEDED.** "The only +100s ever were correctness fixes" is still true, but its companion claim —
   "no speculative eval/search play has moved the number" — is **false as of 2026-07-02**. Seven ordinary
   heuristic keeps (T4 +9.1, T4np +6.4, T1b +13.1, T4c +3.7, iir3 +2.0, T1e +6.6, T5 +20.2) and one
   certified composite (+43.2 real-clock) say otherwise. What was actually true: NGN's *measurement* was
   broken, so speculative work could not be told from noise. Fix the instrument and the ordinary climb works.
2. **SUPERSEDED.** "NGN already HAS all the modern machinery" is feature-NAME true and formula-level FALSE —
   the 2026-07-01 audit found aspiration at fixed-50cp/discard-after-3 (T5 = +20.2 to fix), time management
   missing all three standard scalers (`stableIters` written and never read — T1b = +13.1), no cutNode IIR
   (iir3), corrhist never applied at qsearch stand-pat (T4 family = +19.2 combined). "We have feature X" is
   not evidence; read the formula. The local-optimum framing was measuring a mis-plumbed engine.
3. **STANDS.** PROVEN search-bound (+185 Elo from 2 plies). Eval has headroom; the binding constraint is
   search DEPTH/EBF. This is why NNUE stays the wrong tool.
4. **STANDS, with a caution.** Depth is gated by move-ordering precision (FMC ~84% vs ~90% elite), so
   aggressive reduction over-reduces mis-ordered moves (T3a −7.9 proved it). **Amendment 2026-07-19:** the
   first attempt to raise FMC (T10b, 4-ply conthist) FAILED, so the T3 LMR-reduction lane stays gated and
   the ordering spearhead is not yet demonstrated to convert into strength.
5. **STANDS and is being discharged.** The delicate depth work (richer LMR formula, singular double
   extension) touches the anti-explosion guardrails → attended. **T9a (double extension) went live
   2026-07-24** with the ATTENDED flag discharged operationally rather than by vigil: a pre-launch nodecheck
   explosion gate (+13.6/+12.8/+31.8% = mechanism live, nowhere near runaway), a per-path cap of 5, a
   HALT-on-any-flag-out decision rule, and a `TestSingularDoubleExtensionsFire` regression test that pins the
   arm actually firing. That combination is the reusable template for the rest of the delicate lane.
6. **NEW (the T7 lesson, 2026-07-24).** An experiment can be *methodologically perfect and still measure
   nothing.* T7 had a pre-registered manifest, verified hashes, a clean 8000-game run, and 0 flag-outs — and
   tested a change that moved the search tree 0.001%. Before spending box time, ask what the instrument will
   see, not just whether the protocol is correct. The nodecheck ≥~1% gate is the durable form of this.
7. **NEW (the real bottleneck is handoff, not throughput).** Three stalls — after M6, after the gauntlet, and
   T7's result sitting unread 2026-07-20→24 — have cost roughly **14 box-days against ~7 verdicts delivered**.
   No search idea in the queue is worth as much as reading a finished run the day it finishes. Every session
   that sees a run finish must record the result AND launch the next queued item before ending.

---

## INDEPENDENT CODE REVIEW 2026-07-24 (what a fresh read of the engine found, and did NOT find)

Done from the code rather than from this ledger, on the principle that the accumulated doctrine has been
wrong twice (the "everything is exhausted" frame, and the corrhist plumbing confound). Reviewed cache.go
(TT), the whole of alphaBetaPV, quiescence, time.go, the moveorder updaters, the eval cache, and the T5
aspiration rewrite.

**Found (both queued as T17/T18 in TODO.md):**

1. **`improving` reads a never-written reference.** Two distinct bad references at search.go:1419 — slot 0 of
   `StaticEvalStack` is never written by anything (proven empirically: still 0 after a depth-8 search while
   slots 1/2/3 hold 653/419/271), so ply-2 nodes compute "is the eval positive" instead of "is it trending
   up"; and `-INFINITY` is stored as an in-check sentinel at ply 1 then compared as a value at ply 3, making
   `improving` unconditionally true (measured 1068 such refs in the endgame probe, 100% spuriously true).
   Frequency ~2% of improving decisions, but leveraged: one wrong `improving` mis-sets RFP, futility, LMP and
   LMR simultaneously, at nodes 2-3 plies from the root. Same shape as T1b (+13.1) inverted.
2. **Probcut has no TT short-circuit** — it does the generation, SEE and verification search even when the
   cached entry already proves it cannot fail high.

**Did NOT find — the classic bug classes are clean.** Recording this because "we looked and it was fine" is
evidence too, and it should stop the next agent re-grinding the same ground:
- TT mate-score encoding round-trips correctly (`scoreToTT`/`scoreFromTT`), and depth (max 100, guarded)
  sits well inside the 7-bit packed field, so no truncation or sign inversion is reachable.
- Every prune/reduce gate carries its guards — promotion, in-check, PV, and the `legalTried > 0` F1 guard
  that stops an all-futile node from returning a false stalemate.
- History/killer/counter updaters guard captures *internally*, so the unguarded call sites at the beta
  cutoff (whose comments say "for quiet moves") are harmless — the asymmetry looks like a bug and is not.
- Capture generation is bounds-checked stage by stage: it truncates at 64 rather than overflowing.
- The eval cache is keyed on the full 64-bit hash, stores a pure function of the board, applies fifty-move
  damping and tempo *outside* the cached value, and cannot go stale on a param change because eval weights
  are not runtime-settable (only search params are UCI options). The texel path uses the *uncached* eval, so
  the texel closure is not corrupted by it either — a hypothesis worth killing explicitly.
- The T5 aspiration rewrite is correct: it classifies fail-low/fail-high against the *snapshotted* window
  rather than the alpha mutated during the move loop, and the `+1` growth floor guarantees the window
  reaches full width in a bounded number of re-searches.

**The meta-finding: the correctness era really is over.** The +161/+138 wins came from a broken engine; this
one is clean enough that a deep read produced two small warts and no P0. That is direct evidence for the
revamp's thesis — the remaining path is standard-technique transplants and throughput, not bug-hunting — and
against any future "there must be a big bug left" theory.

**One open thread, not a finding:** qsearch never consults the TimeManager (it checks only the external UCI
stop flag and the node budget), `ShouldStopSearch` reads the clock just once per 1024 calls, and in-check
qsearch nodes are exempt from the qDepth-6 cap so a checking sequence can recurse toward ply 100. That is a
concrete candidate mechanism for the one unexplained anomaly in the pin — the NGN-side flag-out at 120+1 in a
116-ply endgame (2026-07-19 gauntlet, game 193). Unproven, cheap to instrument, and worth doing if a second
NGN-side flag-out ever appears.
