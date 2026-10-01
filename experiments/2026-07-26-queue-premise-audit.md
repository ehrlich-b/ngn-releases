# 2026-07-26 Queue-premise audit — where NGN's Elo has actually come from

**Written while T20 is live, deliberately before its verdict**, so the analysis cannot be reverse-engineered
from that result. Zero box cost. This is an audit of *candidate selection*, not of the instrument.

The instrument question is **already closed and must not be re-ground**: the Batch-2 composite measured
**+35.1 [+19,+51]** against a claimed +26.8 on 2026-07-25 (`2026-07-24-batch2-cert.md`), Batch 1 measured
+43.2 against a claimed +34.3, and both exceeded claim. Self-play keeps transfer; the ruler is sound. So the
six-candidate dry streak is a candidate-quality fact and the question is *which candidates*.

## Every certified keep, by family

From `batch-ledger.md` (Batch 1 + Batch 2, all real-clock 10+0.1 c8):

| Change | Penta Elo | Family |
|---|---|---|
| T5 — aspiration modernization | **+20.2** | root search control |
| T1b — stability-scaling of soft time budget | **+13.1** | time management |
| T4 — pawn correction-history re-plumb | **+9.1** | eval correction (corrhist) |
| T1e — node-fraction best-move effort scaling | **+6.6** | time management |
| T4np — non-pawn correction-history | **+6.4** | eval correction (corrhist) |
| T4c — minor-piece correction-history | **+3.7** | eval correction (corrhist) |
| iir3 — phase-guarded cutNode-IIR | **+2.0** | **node-level search heuristic** |

## Every shelve/reject, by family

| Change | Result | Family |
|---|---|---|
| T18b — probcut TT store | **-22.5** | node-level |
| T1a — drop soft-stop projection | -7.8 | time management |
| T9a — singular double extension | -5.2 | node-level |
| T3b — capture LMR | -3.1 | node-level |
| T10b — 4-ply continuation history | -2.4 | node-level |
| T17 — `improving` reference fix | H0 reject | node-level |
| T16 — s7 depth gate | -0.6 | node-level |
| T13 — qsearch stand-pat TT store | -0.4 | node-level |
| T7 — history pruning depth<=6 | 0 | node-level |
| T3a — LMR cutNode | reject | node-level |
| T1c — score regimes | H0 reject | time management |

## The finding

| Family | Record | Total Elo |
|---|---|---|
| **node-level pruning/reduction/ordering** | **1 keep / 10 attempts** | **+2.0** |
| time management | 2 / 4 | +19.7 |
| correction history | 3 / 3 | +19.2 |
| root search control | 1 / 1 | +20.2 |
| **everything except node-level** | **6 / 8** | **+59.1** |

**The campaign's entire node-level transplant program has produced +2.0 Elo across ten attempts.** Roughly
97% of certified Elo came from three families the queue has largely stopped drawing from. And the current
queue — T15, T12, T11a, T19, plus the live T20 — is **almost entirely node-level**.

## Why — the mechanism, not just the correlation

This is not "node-level techniques are bad." They are what strong engines are made of. It is that **NGN's
node-level stack is already dense, and density is the thing that saturates.**

The corroborating measurement is independent and already in `TODO.md`: **Counter 3.8 at CCRL 2994 — ~290 Elo
above NGN — runs a LEANER node-level stack than NGN does** (no corrhist, no capture-history, no multicut, no
double-extension, no verification search; IID rather than IIR). Its measured differential against NGN is the
**three-regime stability/score-drop time manager**, LMR *shape*, and draw scaling.

So the engine 290 Elo ahead of us got there with *fewer* node-level rules and a *better* time manager. Every
node-level prune/reduce rule competes for the same nodes: probcut, history pruning, continuation history,
capture-LMR, double extensions and SEE pruning all decide what to cut in overlapping regions of the same
tree. Adding the ninth such rule to an already-dense stack has little left to cut — which is exactly the
observed pattern of near-zero capped nulls (T7 = 0, T13 = -0.4, T16 = -0.6, T10b = -2.4, T3b = -3.1), all
with envelopes that never approached a bound. Those are not near-misses. That is the signature of a
saturated mechanism.

T18b's **-22.5** is the same story at the other extreme: when a dense stack does have room, what fits is
usually something that *persists speculative information*, and it back-fires hard.

## Honest caveats — where this argument is weaker than it looks

1. **Ordering confound.** The node-level items ran *later*, against a base already carrying the time-mgmt and
   corrhist gains. A tougher base does not by itself shrink a true +5 (SPRT measures the delta), but
   **redundancy does** — and redundancy is precisely the proposed mechanism, so this confound and the
   hypothesis are not independent. It cannot be fully separated with existing data.
2. **Small samples per family.** Root search control is 1-for-1. That is not a family, it is one result.
3. **Time management is not automatic** — 2-for-4, with T1a at -7.8 and T1c a clean reject. "Go back to the
   time manager" is not a guaranteed lane, it is a better-odds one.
4. **Selection effects in what got queued.** Cheap/tractable node-level items may have been preferred
   precisely because they were easy to specify, independent of their prior.

## What this implies for the queue — NOT yet applied

No queue change is made in this document. T20 owns the box and its verdict comes first; if T20 keeps, the
node-level lane has a live counterexample and this audit weakens considerably.

If T20 caps null, the proposed reorder is:

1. ~~**Counter's three-regime stability/score-drop time manager**~~ — **WITHDRAWN, see the correction below.
   This recommendation was wrong: the three-regime manager has already been fully tested.** The replacement
   candidate in this slot is **T1f (composed soft-ceiling)**, derived in the correction section.
2. **T6 — LMR shape with Counter's saturating-EMA continuous history scaling.** The spec explains *why*
   NGN's `/2048` retest no-op'd (gravity-equilibrium history values vs Counter's saturating EMA), so this is
   a shape fix with a diagnosed prior failure, not another prune-rule transplant.
3. **T11a (TT retention) survives the audit on mechanism** even though it is nominally node-level: it raises
   TT-move *availability* (`ttlist`), which the plateau diagnosis identified as the specific deficit
   (TT move drives 27-44% of cutoffs from only 34-47% availability). That is information retention, not
   another rule competing to cut the same nodes.
4. **T15/T12/T19 drop below all of the above** — they are additional node-level rules in a saturated stack.

The one thing this audit should NOT be used for: closing the node-level lane. Ten attempts with one small
keep is evidence of low yield, not of impossibility, and `iir3` proves the lane can produce. It is a
ranking argument, not a closure.

---

# CORRECTION, 2026-07-26 (same day, T20 still live) — recommendation #1 was wrong

**The error.** Recommendation #1 above proposed running "Counter's three-regime stability/score-drop time
manager" as the next candidate. **That package has already been fully tested. There is no untested
three-regime manager to run.** Checked against `TODO.md` and the run records:

Counter 2994's `difficulty` manager has exactly three regimes: `score drop >50cp -> 2.0`;
`best move changed -> max(1.5, d)`; `stable -> max(0.95, 0.9*d)`.

| Counter regime | NGN status | Result |
|---|---|---|
| best move changed -> extend | **T1b**, `stabilitySoftFactor` at `stableIters==0` -> 1.30 | **KEPT +13.1** |
| stable -> shrink | **T1b**, same factor floored at 0.85 | **KEPT** (same run) |
| score drop -> extend | **T1c**, `scoreDropSoftFactor` ramp 50->150cp | **REJECTED -4.8, pLLR -2.98, crossed the bound** |

T1b banked regimes 2 and 3; T1c ran regime 1 and was a *clean reject that crossed -2.94*, not a capped null.
T1e then added a fourth signal Counter does not have (SF node-effort, **KEPT +6.6**), and T1a falsified
dropping the projection (**-7.8**). **All three Counter regimes have been measured.** The audit's own shelve
table listed T1c, so this was an internal inconsistency in the audit, not missing information — the family
record was counted correctly (2/4) and then a member of that record was recommended as if it were new.

**This does not change the audit's central finding.** The family tallies, the 1-keep-in-10 node-level record,
and the saturation mechanism are all unaffected. What it changes is the *actionable* content: "go back to the
time manager" cannot mean "run the thing Counter runs," because NGN already ran it.

## What is actually left in time management — derived from code, not from the donor

`engine/time.go:429` composes the surviving signals as a **clamped product**:

```go
composed := tm.stabilitySoftFactor() * tm.nodeEffortFactor()   // T1b x T1e
if composed < softComposedMin { composed = softComposedMin }   // 0.72
if composed > softComposedMax { composed = softComposedMax }   // 1.40
soft := time.Duration(float64(tm.softTime) * composed)
```

Two provable facts fall out of the constants (`stabilitySoftMax/Min/Slope` = 1.30/0.85/0.10,
`nodeEffortMin/Max` = 0.85/1.25):

1. **The lower clamp is unreachable dead code.** The natural product minimum is `0.85*0.85 = 0.7225`, and
   `softComposedMin` is 0.72. It can never bind. (The T1e comment says this was deliberate — recording it so
   nobody "fixes" it or tunes it expecting an effect.)
2. **The upper clamp binds only when `stableIters <= 1`:**

| stableIters | stability | upper clamp binds when |
|---|---|---|
| 0 | 1.30 | best-move node fraction **< 0.433** |
| 1 | 1.20 | best-move node fraction **< 0.208** |
| >= 2 | <= 1.10 | **never** (would need nodeEffort > 1.25, above its max) |

So `softComposedMax = 1.40` truncates the raw 1.625 maximum by **14%**, and it does so in *exactly one*
regime: **the best move is churning AND the search cannot concentrate on it** — the most volatile, most
contested positions. That is precisely the population the loss autopsy blamed for 28/30 losses
(positional slow-bleeds / horizon errors), and it is where Counter spends **2.0x** against NGN's capped 1.40x.

**The candidate this yields — T1f, composed soft-ceiling.** Raise `softComposedMax` (1.40 -> ~1.60, i.e.
toward the natural 1.625 the two kept signals already want to produce), leaving the projection, the hard
ceiling, and the emergency floor untouched.

Why this is a legitimately new candidate and not a re-run of something shelved:

- **It is not T1c.** No new signal is added; nothing new is measured. This is the *composition shape* of two
  signals that both already passed their own SPRTs.
- **It is not T1a.** T1a dropped the `x1` next-iteration projection and lost -7.8, proving the projection is
  load-bearing. T1f does not touch the projection, the hard ceiling, or the floor — the same safety argument
  T1b and T1e each made, both of which measured 0/0 flag-outs.
- **The 1.40 value was never measured.** It was chosen in T1e on the reasoning that the pair "cannot
  double-count into a budget that overshoots T1b's proven 1.30 max." That is a defensible default, but it is
  an untested constant that silently caps the only two time signals that ever worked.

## The pre-flight this candidate MUST pass first — and why the usual gate does not apply

**The >=1% nodecheck behavioral-delta gate is inapplicable to this change class.** `ShouldStopSearch` returns
false under FixedDepth, so the time manager is inert at fixed depth — T1e was explicitly recorded as
"fixed-depth node-identical." Running nodecheck here would report 0.0% for *any* time-management patch and
would be a meaningless green light.

T7's lesson still binds, though: a candidate that barely moves behavior burns 8.5h to measure nothing. So
T1f needs an equivalent gate in its own instrument — **measure how often the upper clamp actually binds in
real tournament play.** If it binds in a negligible share of soft-stop evaluations, T1f is near-inert and
must NOT be run; if it binds in a meaningful share, the 14% truncation is live and worth 8.5h.

That measurement is local, costs zero box time, and is the next step. **T1f is NOT staged and NOT queued
until it passes.** Recording the gate before measuring it, per the work loop.

## Honest caveat on this correction

Finding that the donor-derived recommendation was already-tested is *mild evidence against* the audit's
optimism about the time-mgmt lane, not for it. The lane's record is 2-for-4, and the two wins are already
banked and now compose into a ceiling that this correction proposes to raise — which is a smaller,
more derivative idea than "port the manager from the 2994 engine." It should be judged as the
low-single-digit change it is, not inflated by the 2994 provenance.

---

# T1f PRE-FLIGHT RESULT, 2026-07-26 — **T1f IS NEAR-INERT. DO NOT RUN IT.**

Measured locally while T20 held the box. **Zero box cost. The candidate is killed before it consumed 8.5h.**

## Method

Temporary counters in `shouldStopTournamentSearch` (shelf copies `output/t1f-clampprobe.patch` and
`output/t1f-clampprobe_test.go.txt`), driving **real tournament time control at the gate's own 10+0.1**
over **400 positions replayed from the canonical SPRT opening corpus** (`sha256 974e4b5a…`), one full
iterative-deepening search each. For every soft-stop decision the probe recomputed what the decision
*would have been* at a range of candidate ceilings — the decision-relevant delta, not the raw bind rate.

Instrumentation is **fully reverted**; `go test -short ./engine` green on the restored tree.

## Result

```text
soft-stop evaluations        : 872398
upper clamp (1.40) binds     : 69290  (7.94%)
lower clamp (0.72) binds     : 0      (0.00%)
mean raw composed factor     : 1.1630
mean truncation when binding : 0.0771  (5.5% of the soft target withheld)
```

**The lower-clamp prediction is confirmed exactly: 0 binds in 872,398 evaluations.** `softComposedMin = 0.72`
is unreachable dead code, as derived. Recorded so nobody tunes it expecting an effect.

Decision-relevant delta — **moves that would search at least one more iteration**:

| `softComposedMax` | moves changed | % of moves |
|---|---|---|
| 1.45 | 15 / 400 | 3.75% |
| 1.50 | 16 / 400 | 4.00% |
| 1.55 | **18 / 400** | **4.50%** |
| 1.60 | 18 / 400 | 4.50% |
| 1.625 | 18 / 400 | 4.50% |
| **2.00 (Counter's value)** | **18 / 400** | **4.50%** |

## Why this kills the candidate

**The curve saturates at 4.50% by a ceiling of 1.55, and going all the way to Counter's 2.00 — a 43% raise —
buys exactly the same 18 moves.** The reason is structural: `stabilitySoftMax * nodeEffortMax = 1.30 * 1.25 =
1.625` is a hard cap on the raw product, so once the ceiling reaches 1.625 the clamp cannot bind at all and
every higher value is identical. **The ceiling is not the binding constraint. The input signal ranges are.**

So the entire achievable effect of *any* ceiling change is: **≤4.5% of moves get ≤16% more time**
(1.625/1.40 = 1.161) on their final iteration. Set against T1b — which reshaped **every** move's budget by
up to ±30% and measured +13.1 — that is roughly one-twentieth the reach at a fraction of the per-move
magnitude. It is squarely in T7 territory: a candidate that would burn 8.5h of box time to measure nothing.

**This is the pre-flight gate working as designed.** The nodecheck ≥1% gate is structurally inapplicable to
time-management changes (the manager is inert at fixed depth), and this probe is its replacement for that
change class — build it from the shelf patch whenever a time-mgmt candidate needs the same question answered.

## The generalization — time management is closer to tapped than the audit implied

To widen T1f's reach you would have to raise `stabilitySoftMax` (1.30) or `nodeEffortMax` (1.25). **Those are
precisely the constants T1b and T1e measured and kept.** Re-tuning them is the "tuning constants" pattern that
produced nearly every null in this campaign. Combined with T1a rejected (-7.8), T1c rejected (-4.8), and both
wins banked with measured constants, the honest read is:

**The time-management lane has no cheap Elo left in it.** What remains is **T1d** (sudden-death 10-move floor
leak), which is a correctness item, not an Elo tune, and should be judged on the correctness track.

## Consequence for the reorder

Audit recommendation #1 is now **empty** — the donor package was already tested, and the one derived
candidate it yielded is measured near-inert. **If T20 caps null, the reorder's new #1 is T6 (LMR shape with
saturating-EMA history scaling)**, followed by T11a, then T15/T12/T19.

This is a net negative update on the audit's optimism, and it should be read that way: the audit correctly
identified *where the Elo came from*, but "go back to the winning families" has now failed to produce a
runnable candidate in the family with the second-best record. The saturation argument may apply more broadly
than to node-level heuristics alone.

## Caveats

1. **Local machine, single-threaded** — not the box at c8. Absolute flip counts could shift; the *saturation*
   is algebraic (`1.30*1.25 = 1.625`) and is machine-independent.
2. **One search per opening position**, at a fixed early-middlegame ply, not a full-game phase distribution.
   Later phases could bind differently. The structural cap holds regardless of phase.
3. The probe scores the stop decision in isolation; it does not model how a longer search would feed back
   into subsequent stability/effort signals. That feedback could only matter on the same ≤4.5% of moves.
