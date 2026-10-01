# Stuck at ~2600: principled, or floundering?

**Briefing for an adversarial AI reviewer.** NGN is a pure-Go classical-eval (HCE,
**no NNUE**) chess engine parked at ~2600 CCRL for weeks despite continuous work. An
AI assistant (me) has been driving that work; the human (Bryan) wants an independent,
**hostile** check: *am I doing principled engineering, or pattern-matching
plausible-sounding plans that keep failing?* Verify everything yourself — do not trust
this writeup or my reasoning. Challenge every premise, including "we're stuck," "eval
is the lever," and "no NNUE."

## Engine + goal
- NGN: pure-Go HCE. Repo: `/Users/ehrlich/repos/ngn` (`engine/eval.go` ~3000 lines,
  `engine/search.go`, `engine/moveorder.go`). Full modern search: NMP, LMR, LMP,
  futility, RFP, SEE-pruning, probcut, singular ext, IID, continuation/capture/
  follow-up history, TT.
- **Current strength ~2655 CCRL** (real-clock cloud gauntlet vs the Blunder ladder,
  256 games, 2026-06-14). Trend: 2449 (Jun 2) → 2632 (Jun 8) → 2669 (Jun 10) → 2655
  (Jun 14) = **FLAT since ~Jun 8.**
- Goal: **2800 CCRL floor**, 2900 preferred. NO NNUE until ≥2800 (hard human constraint).
- **Existence proof: CounterGo 3.8 (commit b172b99) ≈ 2994 CCRL, pure-Go HCE, with
  LESS search machinery than NGN** — no probcut, no continuation/capture/follow-up
  history, no IID, no singular extension. So 2800-2900 is provably reachable in pure-Go
  HCE. NGN is **340 elo below a leaner engine.** Source at `/tmp/countergo` (checkout
  `b172b99` for the 3.8 HCE era; later tags add NNUE).

## The stuck pattern
- Per-change **self-play** SPRTs show small gains (+1..+25 elo) but the absolute
  (gauntlet) rating is FLAT. A batch of 4 "keeps" summing **+55.7 self-play elo
  certified at +0.6** over 3877 games. Self-play gains are largely transfer-mirages.
- The ONLY changes that ever produced real, transferable elo: **correctness fixes**
  (king-safety over-optimism +44, a time-management bug +33, an SEE bug +161 long ago)
  and **one** eval tune (the PeSTO PST core, +46, on virgin data). Everything else is noise.

## What's been tried (many sessions; full log in `TODO.md` + `memory/`)
- **Eval tuning:** texel tuner (`cmd/texel`: gendata/tune/gradient). PeSTO core won +46
  once. Re-tuning the basis = repeatedly NULL (basis near its weight-optimum). "Expose
  frozen shapes" was the new hope (see today).
- **EBF/search:** NGN mid-game effective branching factor measured ~3.0 vs modern HCE
  ~2.0 — BUT measured against Blunder (same class, possibly blind). One-line pruning
  transplants (deeper futility, aspiration widening, non-PV IIR, multicut, dynamic-NMP)
  mostly REJECTED on sharp-position degradation.
- **Speed:** NGN ~1.5-2x slower per node than Blunder (engineering waste, partly recovered).
- **Search-param SPSA:** measured flat (well-tuned).
- **Correctness:** the proven +elo source, but the obvious bugs are fixed.

## TODAY (the immediate "floundering?" context)
1. Ran a 4-agent review → concluded **"NGN is INSTRUMENT-limited"** (600-game SPRTs =
   sign-guessing on noise = random walk) **"and the lever is EVAL TUNING"** (Counter's
   edge is its jointly-tuned eval; NGN exposes ~16 movable eval knobs vs Ethereal's ~1400).
2. Rebuilt the SPRT into a real instrument (pentanomial stop, [-3,3] bounds,
   cloud-parallelized, ~$2-3/verdict). Certified the pentanomial stat for free.
3. Tested the eval-tuning thesis by exposing frozen shapes + texel-tuning on a 1.05M
   SF-labeled corpus (`output/lichess_eval.txt`):
   - **Threats: FAILED** — tactical term; the texel corpus is quiet-filtered, so threats
     barely fire → ~no MSE signal.
   - **Mobility (all 132 cells): FAILED** — overfit (wild sparse high-mobility cells).
   - **Mobility (restricted to well-supported cells): FAILED** — texel-MSE dropped a lot
     (real "leverage") BUT **ACPL got +4.5 cp/move WORSE vs Stockfish** (tactical 30/30,
     so not a tactical break — a positional/move-selection regression). The MSE-optimal
     mobility is *steeper*, which distorts search (mobility becomes a search attractor).
     The hand-seeded mobility is already near the play-optimum; texel pulls away from it.
4. Concluded **eval-texel is 0-for-3** → proposed pivoting to SEARCH (fractional
   statScore-based LMR, a one-liner the strong engines all use continuously vs our crude
   ±1 step; then a `cutNode` threading refactor).

## Contradictions a skeptic should chew on
- **Eval near-optimal-for-play (texel keeps failing) vs Counter 2994 having a jointly-
  tuned rich eval as its claimed edge.** If Counter's edge is eval, why does tuning
  NGN's eval fail? (Is NGN's eval already fine? Is texel-MSE-on-quiet-positions the wrong
  objective? Is Counter's real edge NOT the eval?)
- **EBF gap (3.0 vs 2.0) yet NGN out-machineries the 2994 engine on search.** If NGN has
  MORE search machinery than Counter, why is it weaker / its tree wider? Is the EBF
  number even valid (Blunder-anchored)?
- **MSE≠strength wall:** texel-MSE improvements repeatedly fail to transfer or transfer
  negative. The static-eval objective doesn't capture search behavior.
- **Instrument:** self-play SPRT gains are transfer-mirages; the gauntlet is truth but
  coarse (±49 at 256 games). Can we even measure a real +10?

## Your job (be hostile; verify, don't trust)
1. **Principled or floundering?** Is "eval-texel 0-for-3 → pivot to search" sound
   iteration, or lane-hopping after each plausible plan fails? Name it.
2. **What is the REAL reason NGN is 340 elo below Counter 3.8?** Tear down both
   (`/tmp/countergo` vs `engine/`). Eval? Search? Speed? A bug? A measurement artifact?
3. **Is there a structural blind spot** — a bug, an architectural ceiling, or a
   measurement flaw — that makes the whole incremental-SPRT approach unable to reach 2800?
4. **Was the v2 thesis ("instrument-limited + eval-is-the-lever") right or wrong?** It
   produced an 0-for-3 within hours. Diagnose the thesis itself.
5. **Single highest-EV action** to move NGN toward 2800, with a falsifiable test.
6. Challenge the constraints: is "no NNUE until 2800" blocking the obvious path? Is
   "pure-Go HCE to 2800" realistic on a solo machine, or is the measurement/process the
   actual ceiling?

## Resources
- `/Users/ehrlich/repos/ngn` — engine + `TODO.md` (full history, read the top governing blocks).
- `/Users/ehrlich/.claude/projects/-Users-ehrlich-repos-ngn/memory/` — accumulated
  findings: `project_meta_plan_v2`, `project_3000_mechanism`, `texel_pipeline`,
  `project_ebf_lane`, `project_speed_lane`, `feedback_*`.
- `/tmp/countergo` — the 2994 existence proof (`git checkout b172b99`).
- Instruments: `cmd/sprt`, `cmd/gauntlet`, `scripts/acpl.sh`, `scripts/nodecheck.sh`.

**Be brutal. If the work is floundering, say so and say exactly why. If the framing is
wrong, reframe it. Ground every claim in code or data you checked yourself.**
