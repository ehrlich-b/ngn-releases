# 11 — Road-to-3000 research census (2026-06-10)

Three parallel research passes run while the CH-fmh SPRT was live, answering "do we
still have a path to 3000 in the backlog?" Provenance:

- **Survey agent:** mined the FULL elo-annotated commit histories of Weiss (342
  elo-reporting commits, pure HCE 3000+) and Ethereal ≤12.x classical (212 commits)
  via the GitHub API, plus Demolito/Laser/Xiphos/classical-SF. Gains below are
  self-play STC/LTC as reported in those commits.
- **CounterGo agent:** full read of CounterGo 3.8 at tag v1.38.0 (commit `b172b99`,
  May 2021, 5,365 engine lines) — the 2994-CCRL pure-Go-HCE existence proof —
  diffed feature-by-feature against NGN at HEAD.
- **Diagnostic agent:** designed the EBF-attribution instrument (§3) from CPW/
  talkchess reference values + NGN's existing counter plumbing.

**Verdict: the path to 3000 exists but needs ALL of:** (1) the EBF block
(conthist system + the width/geometry items below) ≈ +150-250 IF the depth probe
confirms it, (2) the convergent shopping list ≈ +90-150 real after our measured
~60% self-play→real transfer (covers the 2669→2800 leg by itself), (3) the
orthogonal lanes (time-mgmt ≈ +25 LTC reported, eval-eg realism, speed ~1.5×).
The survey list alone canNOT reach 3000; the diagnostic (§3) decides where the
5-7 missing plies actually live before we spend months on the wrong lane.

**Meta-finding:** CounterGo 3.8 is SIMPLER than NGN (631-line search; no probcut,
no conthist, no capture/correction history, no eval caches). NGN already carries
more search machinery than the existence proof. The 2994 gap is in five places:
pruning GATE DEPTHS (futility/SEE to d8 vs NGN d3-4), ROOT search (CG scouts+
reduces at root; NGN full-windows every root move), RETENTION one-liners (TT age
refresh on read, killer update on TT cutoff), SINGULAR geometry (margin ∝ depth,
verify at depth/2−1), and eval ENDGAME REALISM (drawishness divisors, rule-of-
square). Corroboration for the live lane: the FIRST thing Chizhov added after 3.8
was exactly the conthist system (3.9's history.go = butterfly + counter + followup,
EMA `h += (±16384−h)·min(d²,400)/512`, [side|piece|to] keys, equal-weight sum).

---

## §1 Convergent shopping list (multi-engine evidence, ranked)

Gains are the source engines' self-play STC/LTC; apply ~60% transfer + the A3
lesson (co-tuned-tree transplants fail individually ~40-50% even when 3 engines
converged; ordering/history items transfer better than pruning-geometry items).

| # | Item | Mechanism | Reported gains | NGN status |
|---|---|---|---|---|
| 1 | **Bad captures AFTER quiets** | losing-SEE captures ordered at the very end of the list, below history quiets. CG also does this (bad-cap band at mvvlva+0, below quiets ±16384) | Weiss `18cf2aeb` **+18.1/+22.4**; Ethereal `3cba6d0a` +10.0/+3.0; SF staged-gen always | **LACK** — moveorder.go:352-357 bad-cap band 20000 sits ABOVE quiets (≤~8K) |
| 2 | **Fractional statScore LMR** | `r -= statScore/χ` (Weiss χ=8870 continuous) or `clamp(hist/5000,±2)` (Ethereal); statScore = main+cont+fu hist | Weiss `4bab6db2` **+14.1/+13.9** +2 more; Ethereal `995a74ac` +3.6/+4.0 | **= stage-3 CH-statlmr** (current: ±1 at −500/+1000, search.go:1693-1701). NGN 3-table ±8192 scale → χ ≈ 4000-7000, bracket |
| 3 | **Time-mgmt trio** | (a) node-effort: scale soft time by fraction of nodes on best root move; (b) falling-eval extension; (c) best-move-stability multiplier ~1.3-2.5→0.8. CG variant: difficulty 2.0 on −50cp, 1.5 on bestmove flip, ×0.9 decay, hard = 2.7× calm optimum | Weiss effort `14012110` **+11.4/+11.8**, trend `5f8a7dc7` **+11.0**, +3 more; Ethereal stability/score-jump ~+10 across 3 | **LACK** — time.go:256-262 `stableIters` plumbed but DEAD (= D3); hard=4×soft headroom exists, nothing ever spends past soft. Real-clock `-tc` gate ONLY |
| 4 | **LMR for captures** | reduce noisy moves too, gated/scaled on capture history (Ethereal: more reduction when capthist poor) | Weiss `7f9c3e03` +5.8/**+15.0** + series; Ethereal `dcb8560c` +7.2/+2.4, `1b2726a8` +4.2/+4.5 | **LACK** — search.go:1670 `!move.IsCapture()` excludes all captures; captureHistory table already exists as the gate input |
| 5 | **CMH/FMH pruning upgrade** | prune quiets on per-table thresholds (SF: contHist(0)<0 && contHist(1)<0 → skip); use `lmrDepth = depth − LMR[d][played]` for the gate (extends reach to ~d8); improving-dependent threshold; exempt killers | Ethereal `23b841e8` **+23.7/+34.5** (incl. sorting half, banked), `f5059e1d` +3.1/+3.2, +4 more; Weiss `02bfa43d` +9.1/+10.5 | **PARTIAL** — search.go:1496-1501 d≤3, summed score vs flat −1000, no per-table test, no lmrDepth |
| 6 | **Singular package** | (a) double extension: `score < singularBeta − ~20` on nonPV → extend 2, budget-capped; (b) negative extension: ttEval ≥ beta on failed verification → nextDepth−1; (c) margin 2·depth (SF/Demolito) or 1·depth (CG); (d) verification at depth/2−1; (e) limit disproof to ~6 quiets, skip bad caps | Weiss double-ext `a15b4153` **+11.5/+11.2** (largest single post-2021 HCE commit), neg-ext +2.7/+3.1, SE-lower-depth +11.5/+3.2; Ethereal limit-quiets **+10.8/+6.5** | **PARTIAL** — search.go:1352-56,1610-1636: flat margin 64, verify depth−4, unlimited disproof. Reshape FIRST, multicut re-probe only after (multicut ×2 probe-rejects were under the old geometry) |
| 7 | **IIR** | `!ttMove && depth≥4 && pvNode → depth−−` AND cutnode form `cutnode && depth≥8 && !ttMove → depth−−` | Weiss `1d51b23b` +8.0/+5.6, cutnode-IIR +4.8/+6.2, +2 more; Rebel +17; SF since 2020 | **LACK** (PV-IID only, search.go:1149; CG also lacks IIR at 2994 — probe-first stands) |
| 8 | **Aspiration schedule** | initial ±10-25cp; on fail widen the failing side only (double margin / delta += delta/2); never discard an iteration. CG: ±25, double, 2 tries, then full | Weiss `4e236c77` **+14.1/+8.1** +3 more; Ethereal 10cp-base +3.5/+5.1 | **PARTIAL** — search.go:653 ±50 flat (±100 if \|s\|>800); first fail → ±INFINITY; 3-fail exhaustion can DISCARD the iteration (aspab, TODO quirk line) |
| 9 | **Corrhist siblings** | material-key, non-pawn-per-side, minor/major, continuation-keyed corrections — same EMA machinery as the banked pawn-corrhist | Weiss material `5d9ce65e` +3.0/**+7.9**, non-pawn +2.8/**+6.8**, minor/major +1.75/+2.25, cont-corr ×4 +1.4..+4.6 | **PARTIAL** — pawn only (moveorder.go:37-85); each sibling ~20 lines on existing plumbing |
| 10 | **TT package** | store staticEval in entry (needs repack); ttEval refines qsearch stand-pat; store qsearch results at ALL bound types | Weiss store-eval +5.9/+4.3, QS-standpat +2.4/+2.8, save-QS-all +5.4/+6.9 | **PARTIAL** — no eval16 field; qsearch stand-pat ignores ttEval (search.go:2010); qsearch stores LowerBound only (search.go:2085) |
| 11 | **Per-move pruning at PV nodes** | LMP/SEE/futility/hist-prune run at PV nodes too (only whole-node pruning stays !isPV) | Ethereal `9ce411b7` +6.1/+5.4; Weiss +2.8/+6.3 | **LACK** — all four gated !isPV (search.go:1244,1456,1472,1485,1496) |
| 12 | **History-update discipline** | no bonus when FIRST quiet fails high at d≤2-3 (anti-pollution); butterfly [from][to] ADDED to piece-to main history; tuned bonus formula (~16·d² capped) | Ethereal d1/d2/d3 disables +6.2/+3.1/+5.2 LTC; Weiss `5d79de61` butterfly +4.7/**+12.8**, bonus formula `2f7cb2e7` **+23.2/+14.7** | **PARTIAL** — gravity+maluses banked; bonuses fire at ALL depths (search.go:1772); main hist is [piece][to] only |
| 13 | **Upcoming-repetition (cuckoo)** | non-root: if a move could repeat a prior position and alpha<0, raise alpha toward draw — stop burning depth proving reps | Weiss `073eda51` +3.6/+5.4, `fe8e513b` +6.1; SF classical 2018 | **LACK** |
| 14 | **NMP skip-gates** | skip null when TT says `ttValue < beta` with UpperBound (wasted null); skip when prev move has very good history. NOT the rejected A3 R-formula — these are skip conditions | Ethereal `94e78e35` +5.8/+5.4; Weiss +2.7/**+9.3**, +4.0/+7.6; CG has the TT-upper guard too (3-way convergent) | **LACK** — search.go:1260 has neither |
| 15 | **Killers by PLY + child reset** | `killers[ply][2]`, reset ply+2 before move loop; counter-move keyed [side\|piece\|to] | Ethereal reset `4f09be8c` +2.5/+4.4; CG canonical | **BUG-ish** — keyed by remaining DEPTH (moveorder.go:89), never reset; counters [from][to] color-shared (moveorder.go:93). Was dropped on "FMC 89% = ordering fine" — that read is in the §3 AMBIGUOUS band; re-decide on the diagnostic |
| 16 | **SEE-prune range/scaling** | quiets+captures to d8-9; capture threshold scales with depth; CG couples capture-pruning to `eval−100·d ≤ alpha` | Ethereal d8→9 +2.6/+2.1; Weiss tune +16.3 STC; CG shape | **PARTIAL** — d≤4 both; capture flat −100 (search.go:1472-93). ⚠ CG SEE is pawn=1 units — translate to NGN cp |
| 17 | **Futility range + history skip** | to d≤7-8, margin ~100·d, exempt killers/counters, skip-after-null, skip when mover's history high | Ethereal +1.9/+3.7, +4.0/+5.6; CG d≤8 @100·d | **PARTIAL** — d≤3, margin 200+50·(d−1) (search.go:1244-49). ⚠ ungated-RFP→8 probe-REJECTED; A2a improving-gated KEPT — gate everything |
| 18 | **LMR do-deeper** | reduced search returns ≥ alpha+~50 → re-search at nextDepth+1 | Weiss `67722500` +2.4/+3.7; SF 2021+ | **LACK** (search.go:1742) |
| 19 | **Qsearch discipline** | prune in check after 1st evasion; evasion-only movegen in check (+12 STC speed); fix qDepth≥6 → raw-eval truncation (returns mid-sequence eval with hanging pieces; CG runs UNBOUNDED captures+SEE≥0, proves termination) | Weiss `17911653` +5.0/+2.4, `cd7efc26` +12.2 STC | **PARTIAL** — evasions unpruned (search.go:1921-71); cap-6 bug = TODO A4 (soundness) |
| 20 | **Probcut refinements** | gain-gate SEE ≥ probcutBeta−staticEval; return score not beta; allow in check | Weiss +5.4/+2.1 +2 more; Ethereal in-check +3.0/+7.8 | **PARTIAL** — SEE≥0 only, returns beta (search.go:1321,1344). Note CG has NO probcut at 2994 |
| 21 | **Root PVS scout + root LMR** | root moves after the first: scout at null window with LMR reduction (quiets, i>0, clamp [0,d−2]); full window only on scout success; prev-iteration best rotated first | CG search.go:137-171 (2994-proven); universal practice | **LACK** — NGN full-windows EVERY root move at full depth (search.go:730); pure node waste, also makes every aspiration fail expensive |
| 22 | **Retention one-liners** | (a) TT age-refresh-on-read (hot entries never look stale to replacement); (b) killer/history update on TT cutoff | CG transtable.go:83, search.go:217-221 | **LACK** — Cache.Get never touches age (cache.go:165-176); TT-cutoff path returns without killer update (search.go:1130-45). Textbook KEEP-rule candidates |
| 23 | **LMP re-check** | CG `5+d²` improving / halved otherwise = 30/15 d5, 54/27 d7 — NGN prunes ~40% SOONER (30/15 d5… NGN {17,23,30,38}/{9,12,15,19}) | — | possible over-prune at d5-7; cheap re-bracket |

**Lower-confidence (single-engine):** root fail-high progressive re-search
(Ethereal +10.6/+4.8); history extensions (+9.1/+6.2, later constrained — unstable);
LMP counting semantics (+4.8/+6.3); reduce-less-for-checks; don't-add-LMR-to-
extended (+6.5/+3.2); qsearch checks at qdepth 0 (SF yes, Ethereal/Weiss no — split).
**Do NOT reopen:** razoring (Ethereal/SF both deleted it — NGN's removal is the
convergent end-state); TT buckets (ledger-closed).

### Eval items (CounterGo, feeds Lanes B/C)

| Item | Mechanism | NGN status |
|---|---|---|
| **Drawishness divisors** | applied to final score of the side ahead: pawnless force≤minor → /16; two knights /16; pawnless advantage≤minor → /4; one-pawn w/ enemy minor → /8; equal-force one-pawn w/ minor → /2; OCB two-minor /2 (CG evaluation.go:413-438) | only OCB ×32/64 (eval.go:2632-58). Directly attacks the L1 +97cp over-optimism |
| **Rule-of-square passer** | defender force==0 && king can't catch the runner → {0,33}×(rank−1) ≈ up to +165 EG (CG evaluation.go:332-362) | LACK — won K+P races read as small edges |
| **Passer king-proximity** | OppKing-dist × rank weight + OwnKing escort (−OppKing/2.5) + free-passer (stop square empty) | PARTIAL — enemy-king discount exists, no own-king escort |
| **Tempo off in endgames** | tempo {8,8} DISABLED when phase ≤ Q+R (18/64) (CG evaluation.go:386-93) | NGN flat +10 even in pawn endings (eval.go:2670-82) |
| **Per-count mobility curves** | geometric per-count tables scaled by ONE tuned weight per piece (CG weights.go initGeomProgr) — simpler than free per-count tables | NGN linear (count−base)×k (eval.go:2429-54) = Lane C1, with a cheaper CG-shaped option |
| CG material (Texel, err 0.055018) | P 88/100 N 387/332 B 417/351 R 568/616 **Q 1413/1188** pair 51/53 | reference for Lane C |

### What CounterGo does NOT have (what 2994 didn't need)
Probcut; conthist/capture/correction history (3.8 — added in 3.9); history pruning;
history LMR input; recapture/passer extensions; extension budget; delta pruning;
qsearch TT stores; eval cache; pawn hash; razoring. Lesson: the complexity budget
went to deep-cheap gates, retention, and a fully-tuned lean eval.

---

## §2 CounterGo 3.8 vs NGN — shared-parameter table

| Feature | CounterGo 3.8 | NGN |
|---|---|---|
| LMR formula | `trunc(0.9594 + 0.41019·ln(d)·ln(m))`, d,m∈[3..63] | `trunc(ln(d)·ln(m)/2)` (search.go:138-144) |
| LMR start / min depth | move 2 / d3 | move 3 (live var 2 — ⚠ registry Def still 3, search.go:27 vs 52) / d3 |
| LMR adjustments | −1 killer/counter band; clamp [0,d−2]; PV nodes too; reduces checks post-ext | +1 !PV; +1 !improving; ±1 hist thresholds; −1 killer; excl. checks/incheck/captures |
| LMR re-search | reduced>alpha → full-depth FULL window | reduced>alpha → full-depth null window → PV re-search |
| Null move | R=4+d/6 cap d (eval≥β+50) else d−1; need R≥2; min d2; TT-upper skip; lateEndgame skip | flat R=3, min d3 (A3 formula REJECTED −11.3; the skip-guards untested) |
| RFP | d≤8, 100·d, returns beta | d≤7, 120·(d−improving), returns eval−margin (A2a, settled) |
| Futility | d≤8, eval+100·d≤alpha, exempt killer/counter band, skip after null | d≤3, 200+50·(d−1), exempt TT/Q-promo |
| SEE-prune | d≤8, threshold −depth in P1/N4/B4/R6/Q12 units (≈−100cp·d); quiets always; captures only if eval−100·d≤alpha | capture <−100 flat d≤4; quiet <−80·d d≤4 |
| LMP | d≤8: 5+d² improving, halved else (54/27 @d7) | d<8: {…38}/{…19} — prunes ~40% sooner |
| Aspiration | d≥5 ±25, double failing side, 2 tries → full | d>3 ±50/±100, fail → ±INF, 3 tries can discard iteration |
| Singular | d≥8, Lower, ttDepth≥d−3, margin=depth(cp), verify d/2−1 by sibling loop ≤6 quiets; skip if check-ext already | d≥6, Exact\|Lower, margin 64 flat, verify d−4 excluded-move |
| Check ext | +1 any check, ungated, no budget | +1 d>1, per-node cap, path budget 24 |
| IID | d≥8 ANY node, iidDepth = d−d/4−5 (d12→4) | d≥4 PV only, d−2 (d12→10, costly) |
| TT | 16B, direct-mapped; same-key d≥old−3; diff-key older-date OR d≥old; **age refreshed on read**; 16MB default; no qsearch stores | 16B (d1fb499, 8.4M slots @128MB); same-key d≥old−3 OR Exact; diff-key age≠ OR d≥old; NO read refresh; qsearch stores LB-only |
| Qsearch | TT probe any-bound cutoff; captures+Q-promo, SEE≥0; NO delta, NO depth cap, no stores | delta 100+gain; SEE≥0; **qDepth cap 6 → raw eval**; fail-high stores |
| History | EMA `h += (±16384−h)·min(d²,400)/512`, butterfly [side][from][to] | gravity `h += b − h·\|b\|/8192`, b=d² cap 2048, piece-to+CMH+fmh+capture |
| Time | difficulty-scaled budget; hard ≈ 2.7× calm optimum; difficulty 2.0/−50cp, 1.5/flip, ×0.9 decay | soft=bank/moves+0.8·inc; hard=min(4·soft, 0.3·bank); stability signal DEAD |
| Ordering bands | TT 30000 > goodcap 29000+8·v−a > K1 28000 > K2 > counter > quiets ±16384 > **badcap at bottom** | TT 100000 > Qpromo > goodcap 80000 > castle 50000 > counter 30000 > killers 25000 > **badcap 20000 ABOVE quiets** |
| Root | PVS scout + LMR on quiets, prev-best rotated first | full window, full depth, every move (S6 rotation only) |

---

## §3 EBF-attribution diagnostic (designed, ready to implement)

**Purpose:** split NGN's mg EBF ≈ 2.9 (vs class ~2.0) into lanes: (a) ordering,
(b) pruning width at ALL nodes, (c) LMR timidity, (d) qsearch share, (e) re-search/
instability, (f) extension leakage, (g) TT semantics. Decides the post-conthist
queue order with measurements instead of priors.

**Frame:** `EBF ≈ g_main × (1/(1−qshare))^(1/d)`; at the known 42% mg qshare and
d12 the qsearch factor is only 1.046 → g_main ≈ 2.87 vs class ~1.95, and
`g_main² ≈ b_cut × b_all`. NGN product ≈ 8.2, modern ≈ 4. The probe splits that
product. (Confirms night-close: qsearch is a +0.5-ply LEVEL lever, not the ratio.)

**Mechanism:** ~31 new uint64 fields on SearchInfo (after PassedPawnExtensions,
search.go:533) + ~25 one-line increments + 6 `info string sd_*` lines in the
existing `debug on` dump (uci.go:747-753). Existing 20 counters reused. Counters
never branch search ⇒ node-identical by construction; nodecheck before/after must
match exactly (any diff = a typo'd increment). `info string stats` line stays
byte-identical (nodecheck.sh greps it); new tokens never named `nodes`.
**Frozen semantics:** TTHits = usable hits only, TTCutoffs = LowerBound only
(cmd/pruning-analysis consumes them) — add new counters, don't reinterpret.

**New counters (sites verified in working tree; anchors quoted in the design):**
- Ordering: MoveLoopNodes, TTMoveListed (@1390 post-scoreMovesIntoBuffer);
  CutIdxHist[5] (1,2,3,4-7,8+), CutTriedSum, BcutBand[4]/FmcBand[4] (depth bands
  1-2/3-5/6-9/10+), CutByTT/Capture/Promo/Killer/Counter/QuietHist — all inside
  the β-cutoff block @1766-70 BEFORE the history updates @1772 (killer/counter
  tables mutate there).
- Pruning/width: RFPPrunes @1238, NullMoveTries @1277, SEECapPrunes @1474,
  HistPrunes @1498; AllNodes/PVNodesExact/AllTriedSum at the final TT-store
  nodeType branch @1833-45 → **b_all = AllTriedSum/AllNodes**.
- LMR: LMRPliesSum @1720 (mean reduction = sum/LMRReductions).
- Qsearch: QDepthCapHits @1859 (the cap-6 truncation, live-rate check);
  QTTProbes/Hits/Cutoffs @1878-93; QStandPatCuts @2011; QDeltaPrunes @2059;
  QSEEPrunes @2064; QBetaCutoffs @2081+1954.
- Root/instability: AspFailLows/Highs @767-75, AspAbandoned (triple-fail discard
  — any >0 is a red flag), RootBestMoveChanges @781, IIDSearches @1149,
  SingularTries @1602, ExtBudgetClamps @1654.
- Skipped P2: cache.go replacement counters (8.4M slots never fill at 400K —
  capacity exonerated at probe scale by construction; Hash setoption is a stub).

**Interpretation table (pre-registered):**

| Ratio | Reference | Verdict |
|---|---|---|
| FMC = fmc/bcut | >90% strong (CPW); deep 90-95% | **<85% overall or <88% @d≥6 → ordering lane REAL** (conthist aimed right; stage-3 must move THIS, not just depth). **≥92% AND mean cut idx ≤1.3 → ordering is NOT the lane** → b_all. 88-92% ambiguous → cut-class split decides. Prior 89% mg read = ambiguous band, NOT "ruled out" |
| Cut-class split | derived | cuthist >10-15% of cuts landing idx ≥4 → history ordering weak → conthist confirmed surgically. cutcap dominates late → capture ordering. high cuttt + low ttlist → TT starves ordering |
| ttlist/mln | mg raw TT hit 20-35% typical; pawn-eg >75% | <25-30% quiet-mg @400K → store policy starving the move loop (qsearch LB-only stores, IID/singular skip stores) |
| b_cut = ctried/bcut | ~1.0-1.2 | >1.5 → ordering waste compounds per ply |
| b_all = alltried/alln | class needs ~3.5-4 | **≥6 with clean cut nodes → missing plies live at ALL nodes → ladder+LMR lanes** (futility d≤3, SEE d≤4, LMP, no capture-LMR, no cutnode are the named knobs) |
| LMR re-search rate | — | >20% → reductions untrusted → fix ordering FIRST. <5% AND mean reduction <1.5 → LMR too timid → statlmr is the payoff lever |
| qshare | NGN known ~42% mg | arithmetic says LEVEL lever (+0.5 ply); >55% on QUIET probes → discipline pass. QDepthCapHits >0 → A4 soundness live |
| Asp fails/iter | top engines 8-16cp windows | ≥0.4-0.5 with full-width re-search → ~1-ply waste → schedule fix (item 8). aspab >0 → red flag |
| Extensions/MainMoves | — | >6-8% extended in QUIET probes → leakage (check-ext ungated); extclamp >0 → explosion lines |

**Decision rule for the conthist bet:** FMC <88% @d≥6 + material cuthist share →
bet is on the right lane. Stage-3 moves depth +2 but FMC doesn't rise → gain is
coming from somewhere else, re-measure. FMC ≥92% with b_all ≥6 → even successful
conthist won't yield the block → the width/LMR lane inherits it.

**Run plan:** extend scripts/ebfprobe.py with `-diag` (send `debug on`, collect
`sd_*` lines; default invocation stays byte-identical). 6 standing positions ×
two sweeps: 400K (baseline operating point) + 1.6M (~+2 plies — does FMC/b_all
DEGRADE with depth? decaying → history saturation; flat-bad → structure). ×2 runs
byte-identical. ~3 min total on E-cores. Gate sequence: nodecheck pin → implement
→ nodecheck EXACT match → smoke-tactical → sweeps → verdict to TODO exec log.

**Pitfalls (carry into the readout):** FMC is necessary-not-sufficient (silent on
ALL-node ordering — always read with b_all); FMC inflated by in-check forced nodes
and TT-move cuts (split via cuttt); killer/counter cut-labels approximate (tables
mutate during child searches); cut-index mean has a heavy tail (read histogram);
b_all reflects legal-move width (closed positions legitimately wide — compare
within-position across builds); NGN counts qnodes AFTER the TT-probe return
(search.go:1878-96) so qshare is NGN-convention (within-NGN trends only) and the
cap-6 SUPPRESSES it vs uncapped engines; TT rates at 400K cannot indict capacity
(8.4M slots, hashfull ~5%); asp counters only fire d>3 and real eval swings
legitimately fail once; extension counters are events not net plies (budget
clamps cancel some); per-position, never pooled; classification reads only
already-computed values (NO SEE/eval/history calls at increment sites).

---

## §4 Corrections & drive-by findings

1. **TT de-padding is DONE** (`d1fb499`, 16B entry, 8.4M slots @128MB) — any
   roadmap copy still listing "de-pad TT 2M→8M" as a pending lever is stale.
2. **`LMR_MOVE_THRESHOLD` registry default stale:** live var = 2 (search.go:27,
   the kept A1 +33) but TunableSearchParams Def = 3 (search.go:52) — a registry
   "reset to defaults" would silently undo A1. Fix after the fmh verdict (file
   has uncommitted fmh edits; don't touch mid-SPRT).
3. **"Ordering ruled out" (Jun-9, FMC 89% mg) was overread** — 89% is the
   ambiguous band (88-92); killers-by-ply (A6) and the cutnode dimension were
   dropped on it. Re-decide AFTER the §3 diagnostic (banded FMC + cut-class
   split). Counter-evidence: CG plays 2994 with no cutnode param at all.
4. **CG SEE units:** P1/N4/B4/R6/Q12 — every CG SEE threshold must be translated
   to NGN's centipawn SEE before porting (a raw copy is 100× off).
5. **Survey self-check:** gains within one engine's series (e.g. Weiss's four
   capture-LMR commits) are sequential refinements, NOT additive; cross-engine
   convergence is the reliability signal, ordering/history items transfer better
   than pruning-geometry items (the A3/multicut lesson, reconfirmed).
