# 2026-07-01 Method Revamp — the non-NNUE path off the 2666 plateau

Trigger: Bryan — "opus and codex have been looping at 2600-2700 for weeks not making any
progress; revamp this to give them a non-NNUE path forward."

Basis: a 5-agent + first-hand audit on 2026-07-01 of search.go / moveorder.go / time.go /
eval.go / cache.go / cmd/sprt / cmd/spsa / all experiments since the reset, plus a
closure-quality re-grade of every "CLOSED/exhausted" lane. This file is the detailed home;
`TODO.md` carries the compact live queue.

---

## Part 1 — Why the plateau persisted (five mechanisms, all evidence-backed)

### 1.1 Evidence decay: load-bearing closures rest on invalid or pre-reset runs

The doctrine "every lever is exhausted, only delicate attended work remains" fails a
closure-quality audit:

- **Joint-SPSA "no-op" — INVALID closure.** Killed at iter 50 of 3000 (~100 games; real SPSA
  tunes run 40k-256k games). Two independent recomputations of the schedule show the two kill
  signals were artifacts: 8-9 of 11 params had per-iter steps (~0.064) that mathematically
  could not cross the ±0.5 integer-rounding threshold in 50 iters regardless of gradient, and
  `plus_pct` at n=100 has SE ~5% (even a +50 Elo joint gain reads ~57%). Verdict: joint
  retune was never actually run. (`experiments/2026-06-28-joint-spsa.md`, cmd/spsa schedule
  math.)
- **"LMR/NMP retunes regress in both directions" — MOSTLY PRE-RESET.** The 3-SPRTs/0-gains
  evidence predates the 2026-06-28 reset, i.e. it ran on the instrument the reset itself
  declared corrupted (stopped-search TT/history pollution). The history-LMR thresholds
  (−500/+1000 at search.go:1888) and the ±1 bumps have never been game-tested post-reset.
- **Butterfly history rejection — proxy-only, admitted under-credit.** Rejected on a
  cold-table FMC proxy whose own manifest notes the 64× larger table cannot fill in one
  search. Zero games. Violates the project's own "proxy = filter, never verdict" rule.
- **Corrhist "class closed / downstream of search" — over-generalized AND the base is
  mis-plumbed** (see 1.3). Two nonpawn weight points (−9.5 stopped early; −3.8 with CI
  spanning +4) closed four untested tables — while the kept pawn corrhist can barely
  express itself (1.3).
- **Stale in the other direction:** `TRIED-LEDGER.md` lists endgame draw-scaling as
  "identified, STAGED, never closed" — it is in fact LIVE in HEAD (eval.go:2684 `drawFactor`
  family ported from Counter, OCB square-color check, pawnless /16, one-pawn /8, plus
  50-move damping eval.go:2817, with tests). The anti-re-grind ledger is wrong in both
  directions, which corrupts lever selection.
- The single most strategy-defining result — depth-12-vs-10 = +185 ("search-bound proof") —
  has **no experiment manifest** (memory + commit message only). The qualitative conclusion
  survives scrutiny (eval is not saturated); the magnitude at operating depth (~+30-50/ply)
  is asserted, not measured.

**Rule going forward:** a lane is CLOSED only by post-reset evidence meeting the decision
table. Pre-reset verdicts are historical color, not closures. Every closure entry in the
ledger must cite its run record.

### 1.2 "NGN has all the machinery" is true at feature-name level, false at formula level

The full-detail audit against 2900-3100 classical practice (Stash/Weiss/Ethereal/CounterGo
class) found the machinery present but repeatedly in weaker-than-standard form:

| Area | NGN form | Standard form | Est. value* |
|---|---|---|---|
| LMR inputs (search.go:1862-1918) | no cutNode term at all; discrete ±1 history; no ttPv; captures never reduced | `cutNode → r+=1..2`; continuous history scaling; ttPv term | +10-20 |
| Singular (search.go:1783-1830) | single +1 ext, neg-ext lite; no double-ext, no multicut | double/triple ext when verification fails far below singularBeta; multicut | +10-20 |
| Aspiration (search.go:696-911) | fixed 50cp; fail → jump to ±INFINITY; 3 fails → iteration DISCARDED | ~12-20cp init, progressive ×~1.5-2 widening, never discard | +5-15 |
| TT (cache.go) | direct-mapped, 1 entry/index, 16B | 3-4-way cluster/bucket, depth-preferred within cluster | +5-15 |
| Time mgmt (time.go:297-330) | soft-stop projects next iter at ×1·lastIter; `stableIters` tracked but NEVER READ | spend-while-under-soft (Counter uses ~98% of clock vs NGN ~70-80%) + best-move-stability soft scaling | +10-30 (real-clock only) |
| History pruning (search.go:1682) | depth≤3, flat −1000, after move 3 | depth≤6-8, depth-scaled threshold | +5-10 |
| Corrhist (moveorder.go:37-85) | pawn-only, and mis-plumbed (1.3) | pawn + nonpawn/minor/major/cont, applied everywhere static eval is used as value | +8-15 (family) |
| qsearch (search.go:2131-2396) | no quiet checks, no quiet promos, TT store fail-high-only, 6-ply cap | quiet checks at first ply, queen promos, store all qnodes | +3-8 |
| Ordering completeness (moveorder.go) | IID PV-only (~75% of mg cut nodes get no first-move finder); no 4-ply conthist; weights 1:1:1; killers never reset between searches | IIR at cut+PV; 4-ply table; ~2× main + 2× 1-ply weights; killers reset per search | +5-15 across items |
| NMP (search.go:1420-1451) | good depth-scaled R; no eval-margin term, no verification search | `R += min((eval-beta)/200, 3)`; verify at high depth | +2-6 |
| SEE capture pruning (search.go:1657) | flat −100 all depths ≤4 | depth-scaled | +2-6 |

*Estimates from standard-engine practice, to be replaced by measured verdicts; treat as
ranking weights, not promises (expectation-calibration rule).

The audits also found **no correctness bug** in draw detection, cutNode propagation,
mate-distance, SEE, TT XOR-verify, stop/unwind — the reset held. And the history UPDATE
machinery (gravity form, malus on tried quiets AND captures, capture history, killers,
counter) is confirmed complete and standard — which is exactly why re-tuning its scalars
kept regressing. The gap is missing inputs/components, not mistuned existing ones.

### 1.3 Mis-plumbed machinery (found by first-hand code read)

1. **Pawn corrhist never touches backed-up values.** Correction is applied ONLY at the
   interior-node static eval (search.go:1353). Qsearch stand-pat — the value that actually
   backs up through the tree — calls `EvaluateForPlayerCached(pos)` raw (search.go:2305).
   So the kept corrhist only shifts pruning margins/improving; it cannot correct what the
   engine actually scores. Every corrhist ADD was tested on top of this.
2. **Corrhist learning target is corrupted by S7.** `maybeUpdatePawnCorrection` (search.go:
   2063, 2094) receives `staticEval` AFTER the S7 TT-refinement (search.go:1358-1364) may
   have replaced it with a previous search's TT score — so at TT-hit nodes it learns
   `search − ttEval` ≈ 0 instead of `search − rawStatic`. Stockfish keeps
   `unadjustedStaticEval` separate precisely for this.
3. **Two of the 11 advertised SPSA knobs are dead.** `SingularMargin` and `NullMoveR` are in
   `TunableSearchParams` (search.go:52-64) but the code hardcodes `ttEval − 2*depth`
   (search.go:1792) and `4 + depth/6` (search.go:1435). Any SPSA over the advertised list is
   partly tuning air (and was, in the killed run).

### 1.4 The economics never closed: remaining gains are smaller than the instrument's daily resolution

- Remaining single-item gains at 2666 are mostly +2-8 self-play. At 10+0.1 with bounds
  [−3,+3], resolving +3-5 takes ~5-15k games; the box does ~570 games/hr at c4 → ~1
  verdict/day, and "shelve on inconclusive at cap" then discards precisely the +1-3 class
  the climb is made of. The climb needs ~40-60 keeps (Ethereal/Weiss keep median ≈ +4);
  at ≤1 net keep/day it stalls — as observed.
- **No adjudication anywhere:** cmd/sprt/gauntlet parse out and DISCARD engine scores; dead
  draws run the full 200 plies. Score-based draw/win adjudication (standard in
  cutechess/fastchess/OpenBench) is the single cheapest throughput multiplier.
- cmd/spsa is strictly serial (2 games/iter on a 16-thread box).
- Diagnostics kept displacing staged executable items (time-mgmt residual staged since
  Jun-15, never run; meanwhile multiple diagnostic instruments were built).

### 1.5 The strategy over-rotated on a single axis

"Search-bound ⇒ only EBF matters ⇒ eval-side work is worthless" closed corrhist and parked
accuracy items. The proof shows eval is NOT saturated and depth pays — it does not show
depth is the only payer. Time management (pure real-clock Elo), value-accuracy plumbing
(corrhist), and TT quality all pay independently of EBF and are invisible or half-visible
to the fixed-nodes filter that dominated the queue. The reference engines settle this:
Stash and Ethereal banked eval-shape and corrhist items at +2-27 continuously through the
exact band NGN is in (see the secondary lane in Part 3), while also grinding search.

---

## Part 2 — The path: three workstreams

### W1. Fix the mill first (no Elo verdicts needed; ~2-3 sessions)

Per harness guardrails each harness change gets its own validation + A/A before use in a
verdict; do these BETWEEN verdicts, never mid-run.

- **M1 (trivial, do first): fix the dead SPSA knobs.** Wire the singular margin coefficient
  (the `2` in `2*depth`) and the NMP R base/divisor (`4`, `6`) as real `TunableSearchParams`;
  drop or repoint the dead entries. Anything less invalidates every future SPSA.
- **M2: A/A-validate c8 at 10+0.1 on the box.** Real-clock games are WALL-bounded by the
  clocks, so games/hr scales ~linearly with concurrency until flag-outs: c4 ≈ 13.7k
  games/day, c8 ≈ 25-27k/day (1 engine/thread on 16 threads = the safe ceiling; c12 risks
  starvation flag-outs). This is the single biggest throughput lever and needs zero code —
  just an A/A (penta ≈ 0, zero flag-outs) before first use in a verdict.
- **M3: `scripts/boxsprt.sh`.** Wrap the manual Windows-box flow (cross-compile
  GOOS=windows GOARCH=amd64 GOAMD64=v3, scp to `~\ngn\sprt\`, schtasks far-future-`/sd`
  launch + delete-after-run, `Stop-Process -Name sprt` kill, fetch+tail of `*_out.txt`, `ps`
  subcommand). The box is the free mill; today it costs a page of remembered gotchas per run.
- **M4: parallelize cmd/spsa.** Batch K opening-pairs per iteration across the existing
  worker-pool pattern from cmd/sprt (K=4 at c8 ⇒ ~8 games/iter wall-parallel); average the
  K pair-results into one `R`. Standard fishtest practice; turns 3000 iters from 50h serial
  into 1-2 box-nights.
- **M5: score-based adjudication in cmd/sprt.** Worth ~10-15% at bullet (drawn tails only
  cost increment time since games are wall-bounded — smaller than first assumed, still
  free). Read the `info score cp` the harness currently discards: draw if both sides report
  |cp| ≤ 20 for 8 consecutive plies after move 40 (threshold must clear the draw-scaled
  evals: a /16-scaled dead ending still reads ~±20cp); win if both report |cp| ≥ 600 for 6
  plies sign-consistently. Validate: A/A + re-run one known verdict, confirm same sign.
- **M6: A/A-validate 5+0.05 c8** (~48-54k games/day) for Stage-1 filters and SPSA only;
  verdicts stay at 10+0.1 until 5+0.05 shows sign-agreement on 2-3 changes.

### W2. The transplant queue (Part 3) — missing-standard-technique adds, one at a time

The one post-reset Elo-positive lead (IIR, pending) is exactly this class: a genuinely
missing standard technique, added in standard form, phase-guarded where NGN's profile
demands. The queue in Part 3 is ~14 more items of the same class, each with file:line spec,
filter, verdict gate, and reject condition. NOT speculative knob-turning — every item is
something 2900+ classical engines converged on and NGN lacks or half-implements
(audit-verified).

### W3. Verification cadence that compounds instead of evaporating

- **Two-tier gate per item:** Stage-1 filter (fixed-nodes on box, or FMC/ebfprobe for pure
  ordering/EBF mechanisms) → Stage-2 real-clock 10+0.1 SPRT [−3,+3] to bound or 8k-game cap.
- **Batch-certification protocol (the KEEP-rule ⇄ noise-selection reconciliation):**
  - Resolved H1 at Stage-2 → KEEP outright.
  - Capped-but-positive (point est ≥ +1, no regression signal, LLR > 0) → PROVISIONAL keep:
    goes into main, recorded in the batch ledger.
  - After ≤4 provisional keeps (or 2 weeks): ONE powered batch-vs-pre-batch-base real-clock
    SPRT [0,+6]. Pass → all four become KEEP. Fail → bisect the batch, drop the noise.
  - This preserves Bryan's KEEP rule (small clear improvements are kept, immediately, in
    main) while guaranteeing nothing noise-borne survives two independent looks.
- **Milestone pin:** multi-family real-clock gauntlet (Blunder ladder + Counter, 120+1, the
  2026-06-28 harness) every ~2 weeks or every +15-20 claimed self-play Elo, whichever first.
- **Ledger hygiene:** TRIED-LEDGER entries must cite a run record and its instrument;
  pre-reset entries get a `[pre-reset]` tag and cannot close a lane. (Corrections applied
  2026-07-01: draw-scaling marked LIVE; joint-SPSA re-graded INVALID-closure; LMR/NMP
  re-graded pre-reset.)
- **Every strategy-grade result gets a manifest** — starting with a backfill manifest for
  the depth-12-vs-10 = +185 run.

### Honest arithmetic (expectation calibration)

The reference engines now give this exact numbers: Stash's whole classical climb was ~100
SPRT-passed changes at median +4.5 STC; classical Ethereal's was the same shape (median
~+4.0, range mostly +2-7, a handful of +10-25 outliers). That is the terrain. If half the
queue lands at +3-8 self-play each, plus one valid SPSA round (+3-26 measured in-class),
that is ~+40-70 self-play per month at ~60% transfer ≈ +25-45 CCRL/month once the mill does
2-3 verdicts/day. 2800 is a ~3-month grind in the realistic case, not weeks; there is no
demonstrated single +100 block left (the only historical +100s were correctness fixes). Say
so in any projection. The corollary: an instrument+policy that cannot BANK +2-8 changes
cannot climb at all — that is what W1/W3 fix.

---

## Part 3 — Ranked queue (each item isolated, vs immediate base)

Format: mechanism → change → gate → reject condition. Priors are now MEASURED where
possible, from the 2026-07-01 reference mining of Stash (pure-classical, zero NNUE ever,
climbed to the 2900s+ on ~100 SPRT-passed changes, median +4.5 STC) and classical-era
Ethereal (245 SPRT commits, median ~+4.0). Cite format: [S]=Stash commit, [E]=Ethereal,
[Sim]=Simbelmyne, [V]=Viridithas, [M]=MadChess. Self-play STC numbers; expect ~60% transfer.

**T1. Time-management package** — real-clock-ONLY gates (fixed-nodes blind here).
   Prior: the single biggest TM item on record is Stash v26's "real time management system
   (bestmove type / eval direction / bestmove stability)" = **+31/+29 LTC** [S]; also
   +24 [M], +5.6 "allow longer thinking on unfinished depths" [S #69], stability +2.3 and
   score-drop scaling +2.5 [E]. NGN spends ~70-80% of clock vs Counter ~98%.
   - T1a (the staged 1-liner, 2 weeks overdue): soft-stop `elapsed+lastIterationTime > soft`
     → `elapsed >= soft` (time.go:329). The ×1 projection stops at ~2/3 of soft.
   - T1b (after T1a verdict): wire the dead `stableIters` signal — soft × stability factor,
     symmetric (extend on instability, shrink on stability), hard cap unchanged.
     (stableIters is written and never read — time.go:63,256-262. The old one-sided shrink
     caused the 25%-clock bug; symmetry is what makes it sound.) EXACT PEER SPEC — CounterGo
     2994's three-regime `difficulty` (timemanager.go, mined 2026-07-01): score dropped
     >50cp vs last iter → difficulty=2.0 (max think); best move changed → max(1.5, d);
     stable → d=max(0.95, 0.9·d) (decays toward fast). Soft limit recomputed each iteration
     from difficulty; hard set once at difficulty=2. Also: Counter uses movestogo=40
     default (not NGN's 10-floor) and MoveOverhead 300ms.
   - T1c: the second leak — `estimateMovesRemaining` floors at 10 phantom moves in sudden
     death (time.go:188-194), perpetually reserving bank NGN never spends. Decay the
     divisor as the game progresses.
   - Gate: 10+0.1 self-play SPRT; sanity spot-check at 60+0.6 (TC-sensitivity, small n
     fine). Reject: regression or flag-outs above A/A baseline.
**T2. iir3** — resolve the pending box verdict (patch `scratchpad/iir3.patch`); commit on
   positive/holds, shelve on ~0/neg. Precedent: Stash removed IIR as neutral at v30 then
   REINTRODUCED it at **+9.5** [S #61] — marginal-then-real is the known trajectory here.
**T3. LMR modernization family** — one small item at a time, in this order:
   - T3a cutNode term: `if cutNode { reduction++ }` (new param `LMRCutNode`). Measured:
     "introduce cutNodes + perform LMR on them" **+6.2/+10.0 LTC** [S #119]. The #1 audit gap.
   - T3b capture/tactical LMR: allow reducing captures with bad capture-history
     (**+7.2** [E dcb8560]; Stash reduces noisy moves with a separate gentler log formula).
     NGN currently NEVER reduces captures.
   - T3c TT-move-is-capture → reduce quiets more (**+2.3/+5.3 LTC** [S #166]).
   - Gate each: fixed-nodes filter → real-clock. Reject: filter clearly negative after a
     phase-split check (the iir2→iir3 lesson: if endgame-driven, try the accPhase guard once).
**T4. Corrhist re-plumb (2 sub-changes, tested as one; mechanism-proven, games-gated).** [pawn re-plumb KEPT +9.1 H1 2026-07-02 — see the learn-target correction below and `experiments/2026-07-02-t4-corrhist-replumb.md`.]
   Prior: pawn corrhist measured **+13.1/+16.0 LTC** [S #179], **+46** [Sim]; NGN's is
   mis-plumbed (Part 1.3) so some of that value is being left on the table today.
   - Apply correction at qsearch stand-pat (search.go:2305): compute `corrIdx` there (two
     bitboard reads + hash, cheap) and add `correctionValue(stm, corrIdx)`; eval cache is
     unaffected (correction applied outside it — verified eval.go:2806-2822).
   - Learn against RAW static: keep `rawStaticEval` before S7 refinement (search.go:1358)
     and pass THAT to `maybeUpdatePawnCorrection` (search.go:2063,2094); S7-refined value
     still feeds margins/improving.
     **[2026-07-02 correction — SHIPPED learn-from-CORRECTED, not raw.** "Learn against RAW
     static" is right only for EMA-toward-diff update forms; NGN's `updatePawnCorrection` is the
     GRAVITY form (`*e += bonus − (*e)·|bonus|/corrHistLimit`, self-decays toward equilibrium), so
     learning from raw saturates entries at ±corrHistLimit and turns them magnitude-blind. Shipped
     target = the pre-S7 CORRECTED static (`corrStaticEval` = raw + correction). Pawn re-plumb KEPT
     **+9.1 [+0,+18] penta, H1**, real-clock 10+0.1 c8, pLLR +2.80, 0/0 flag-outs / 2325g
     (`experiments/2026-07-02-t4-corrhist-replumb.md`).]**
   - THEN (separate item, only if T4 keeps): retest the nonpawn corrhist add — measured
     **+26.3/+27.4 LTC** [S #201], +19.7 [Sim] — the single largest modern classical lever
     on record. The prior NGN rejections tested it on the broken plumbing.
**T5. Aspiration modernization** — init ~10-16cp (Stash: `delta = 8 + |score|/81`), widen
   ×~1.3-2 per fail (Stash ×1.30, Ethereal +10 linear), never discard an iteration (drop
   maxAttempts; on clock-stop keep last completed as today), params `AspInit`, `AspMult`.
   search.go:693-911. Measured: 10cp base **+3.5/+5.1 LTC** [E 0ad3477], window-vs-eval
   scaling +3.1 [S #128], "tweak aspiration implementation" +20.5 [Sim]. Gate: fixed-nodes
   filter (visible there) → real-clock.
**T6. History-LMR: continuous scaling, retested with the SCALE insight** — CounterGo uses
   `r -= clamp(histSum/5000, −2, +2)` where its EMA-form history saturates entries toward
   ±16384 (sum range ±49k). NGN's gravity-form entries cluster near 0 (equilibrium), so the
   tested `/2048` truncated to 0 almost everywhere (the recorded "int-trunc no-op") and
   `/256` over-swung — the 2026-06-28 rejection never tested a divisor matched to NGN's
   actual value distribution WITH a ±2 clamp. Retest: measure the live histSum distribution
   (sd_* counters), pick divisor ≈ P90/2, clamp ±2; expose `LMRHistDiv` + the ±1 thresholds
   as SPSA dims (T8). Pre-reset "both directions regress" evidence is void (Part 1.1).
**T7. History pruning modernization** — depth≤6, depth-scaled threshold, killer/counter
   exempt, params exposed (search.go:1682). Measured class: counter-move-history
   prune+sort **+23.7/+34.5 LTC** [E 23b841e — the biggest single classical-Ethereal item];
   history pruning +4.1 [V].
**T8. SPSA round 1** — AFTER M1/M4 and ≥2 of T3/T5/T6/T7 land: joint over the now-real
   ~15-20 params, ≥3000 iters × K=4 pairs at 5+0.05 (~24-48k games, 1-2 box-nights),
   verdict = games on the converged vector (never mid-run plus_pct/param movement — that
   mistake is documented in 1.1). Measured precedent in-class: "SPSA-tune all search
   constants" **+25.9** [S #151, 2023] and +3.3/+8.5 LTC [S #181, 2024]. Include LMP
   scale as dims: NGN's LMP thresholds are ~2× more conservative than current Stash
   ((13+4d²)/16 vs NGN's (5+d²)/2 non-improving); "make LMP much more aggressive" was
   **+24.8** [S #46] — but bracket it via SPSA, not by hand.
**T9. Singular family (ATTENDED — the :1821/:1832 guardrails are load-bearing):**
   - T9a double extension: `if singularScore < singularBeta - SE2Margin { nextDepth += 2 }`,
     per-path budget cap kept, watch node counts + flag-outs live. Measured: **+11.2/+10.1
     LTC** (capped at 5 per path) [S #115], +11.9 [Sim].
   - T9b multicut: singular search fails high ≥ beta → prune. Measured **+5.8** [S #63],
     +5.7 [E d397a7b]. (NGN's prior return-beta multicut rejection is pre-reset evidence.)
   - T9c trigger-depth experiment: Ethereal gained **+12.7** raising singular depth 8→10;
     Stash gained +7.9 LOWERING to 7. Direction unknown a priori — SPSA dim, not hand test.
**T10. Ordering completeness** — small items, FMC proxy as FILTER only, games verdict each:
   - T10a killer reset per search/child-ply (**+2.5/+4.4 LTC** [E 4f09be8]).
   - T10b 4-ply continuation table (**+13.1/+8.8 LTC** [S #139] — big measured prior).
   - T10c ordering weights: try ~2× main+1-ply vs the current 1:1:1 (SF-shape; untried —
     the tried "2× continuation" was the wrong axis).
   - (Butterfly warm-table retest hangs off T10c only if weights move FMC.)
**T11. TT quality** — two stages: T11a replacement-policy tweak (Berserk-style lower-depth
   conditional replace was **+7.5** [S #16]) — cheap, single-entry, try first; T11b 4-way
   64B clustering (cache.go) if T11a leaves signal on the table. Fixed-nodes filter →
   real-clock.
**T12. NMP polish** — eval-margin R term (CounterGo: `min(2, (eval-beta)/200)`; Stash
   `/111 cap 5`; Ethereal `/200 cap 3`, ~+3.3 [E 89ed7eb]); NMP only at expected cut-nodes
   (**+9.7** [S #230]); verification search (Stash yes, Ethereal/Counter no — low
   confidence, last). Cheap.
**T13. qsearch completeness** — TT store of stand-pat ≥ beta as LowerBound (**+7.0** [S
   #126]); queen promos in qsearch generation; (quiet checks PARKED until movegen gains a
   complete givesCheck — bigger job).
**T14. Speed leftovers (opportunistic)** — large pages when Bryan can grant
   SeLockMemoryPrivilege on the box; the deferred smaller-TT (16-64MB) behavior SPRT.

Secondary lane (not queued, kept open): eval SHAPE adds kept paying for Stash/Ethereal all
the way up — phalanx/connected pawns +25.4 [S #26], passed-pawn king proximity +22.3 [S
#38] / +6.1 [E], threats-all-pieces +15.5 [S #197], mobility-through-friendly-sliders
+10.3 [S #157]. NGN has some of these (threats, passer-king terms live); when the search
queue thins or a loss-autopsy flags a shape, this lane reopens via ACPL filter + games —
"search-bound" means search FIRST, not eval NEVER (1.5).

Dependencies: T8 after M1/M4 + a few structural adds; T4-then-nonpawn ordering; everything
else independent — pick top-down, one live verdict at a time, mill never idle (next item's
filter can run on the Mac while the box holds the verdict).

---

## Part 4 — Policy deltas applied 2026-07-01

1. **Closure integrity:** only post-reset evidence closes a lane; ledger entries cite run
   records; pre-reset verdicts are historical.
2. **Batch-certification protocol** (W3) added as the standing keep path for
   capped-but-positive results — implements the KEEP rule without sign-at-cap noise
   accumulation.
3. **Proxy scope:** FMC/ebfprobe/fixed-nodes may REJECT only pure ordering/EBF-mechanism
   changes, and any proxy rejection must record a reopen condition; anything touching eval
   values, time, or TC behavior gets a games gate.
4. **Instrument-match rule:** time-management and TC-sensitive changes are gated real-clock
   only (fixed-nodes/movetime blind or misleading).
5. **SPSA validity:** a SPSA run is only evidence when (a) params verified live (M1), (b)
   run to a meaningful fraction of schedule with games ≥ ~20k, (c) verdict = games on the
   converged vector.

## CounterGo cross-check (mined 2026-07-01, pre-NNUE tip `3eda962` — the 2994 peer)

Counter 2994 runs a LEANER stack than NGN: no corrhist, no capture history, no multicut,
no double/negative extensions, no NMP verification, IID not IIR, check extension only. So
the NGN→2994 differential is NOT exotic machinery; from source it is (1) the three-regime
stability/score-drop TIME manager (T1b spec above; NGN uses ~70-80% of clock vs ~98%),
(2) LMR shape — log·log product lerped onto reductions ~[3..8] with pvNode −2, history
±2 continuous, improving/check adjustments (vs NGN's /2.0 base with ±1 nudges) — T3/T6,
and (3) draw scaling, which NGN already ported. The Stash ledger (corrhist +13/+26,
double-ext +11, 4-ply conthist +13) is the "beyond Counter" growth lane on top. Both
corroborate the queue order: T1 first, LMR family next, corrhist re-plumb close behind.
Other Counter constants worth stealing when their T-items run: singular margin 1×depth
trigger depth≥8 (NGN: 2×depth, ≥6); RFP 100×depth flat; noisy-SEE threshold alpha-aware
`−max(depth,(staticEval+100−alpha)/100)`; killer update on TT lower-bound cutoff.

## Current known-unknowns

- iir3 real-clock c4 verdict — on the box, box offline at audit time (`iir3c4_out.txt`).
- Weiss mining failed on a network error — non-blocking, not retried.
- The depth-slope at operating depth (the "+30-50/ply" assertion) — unmeasured; a
  depth-18-vs-16 probe with a manifest would firm up the EBF-lever ceiling but does not
  change the queue.
