# 2026-08-02 Eval-shape triage — "space term (evaluateCenterControl DEAD code — wire or delete)" RESOLVED: do NOT wire

**Zero box cost. No code changed.** Resolves a queue item that has been sitting in the eval-shape lane
unexamined, and corrects the label it was filed under.

## Why this was picked up now

The eval-shape lane is explicitly conditional in TODO: *"only pull these when the search queue is blocked"*.
**That condition is now met.** As of tonight the search queue is genuinely blocked, not merely thin:

- **T3 LMR-reduction is gated with no unblocking path left** — both T10 FMC-raising attempts have failed
  (T10b -2.4 by games, T10a flat by proxy tonight).
- **Corrhist is mined out** (3 keeps / 5, both remaining key ideas dead by rule or mechanism).
- **Time management is fully closed** (T1d disproved tonight).
- **TT/retention closed**, root control 1-for-2 after T21's gate rejection.
- Node-level pruning/reduction is **1 keep / 11 attempts**, and T12/T19/T15 are the last three staged
  members of it.

So the eval-shape lane is now legitimately open, and its cheapest item is the one with code already written.

## Finding 1 — the dead code is MISLABELED. It is not a space term.

The queue entry reads *"space term (evaluateCenterControl is DEAD code — wire or delete)"*, which conflates
two different ideas:

- **What the dead code actually does:** counts own pawns/knights/bishops/queens sitting **on** d4/e4/d5/e5
  (plus an extended ring) and adds flat bonuses. That is **centre OCCUPATION**.
- **What a space term actually is** (SF and peers): the squares **behind one's own pawn chain in enemy
  territory** that are safe from enemy pawn attack, scaled by piece count. That is a measure of **room to
  manoeuvre**, and it has essentially nothing to do with which pieces stand on the four centre squares.

These are different evaluations. Resolving one says nothing about the other.

## Finding 2 — centre occupation is ALREADY IN THE PST. Do not wire it.

`evaluateCenterControlFast` awards **+25 for a pawn** and **+20 for a knight** on d4/e4/d5/e5.

NGN's eval is PeSTO-tapered material+PST. Reading `mgPawnTable` directly, the middlegame PST values for those
exact squares are:

| square | mg PST value |
|---|---|
| d5 | **+21** |
| e5 | **+19** |
| e4 | **+13** |
| d4 | **+8** |

**The PST already rewards central pawns, and the dead code would stack a flat +25 on top of it** — roughly
doubling to tripling a tuned value with a hand-picked constant.

**This is precisely the bishop-pair precedent, which was measured, not argued: removing NGN's explicit
bishop-pair bonus scored +20.4 because PeSTO already encodes it.** An explicit centre-occupation term is the
same category of double-count, and there is no reason to expect a different sign.

**Verdict: do NOT wire it.** Deleting it is harmless cosmetic cleanup with **zero Elo content** — it is dead
code and cannot affect behaviour — so it must not be filed as an Elo candidate or counted toward the queue.

## Finding 3 — its removal history supports this rather than contradicting it

`evaluateCenterControl` was removed in **`4ff7d61` (2025-09-02)**, *"remove expensive positional evaluations
to restore 330K+ NPS"* — i.e. cut for **speed**, which on its face leaves open the possibility that the term
was valuable but unaffordable.

**That reading does not survive the PST check.** The term was redundant regardless of cost, so its removal was
correct for a second, better reason than the one recorded at the time. Also worth noting the date: this is
**legacy code from ~11 months before the current campaign**, predating the 2026-06-28 reset entirely.

## What remains genuinely open

**A real space term is untested — `grep -i space engine/eval.go` returns nothing, so NGN has no such
evaluation at all.** That is a legitimate eval-shape gap and it stays on the list, but it is a **new
implementation**, not a wiring job, and it must be gated the eval way: **ACPL pre-filter (`make acpl`) first,
then real-clock games** — never a self-play SPRT alone, per the standing rule that eval changes get a
real-clock gate.

**Ranked honestly: low.** Space terms are worth a few Elo in engines that lack them, NGN is **search-bound**
(depth-12-vs-10 = +185), and every eval-value change in this campaign's history has been null or negative
while the linear texel basis is closed by multiple independent nulls.

```yaml
id: 2026-08-02-space-term-triage
date: 2026-08-02
change_class: triage / queue correction (no code changed)
result: >
  "wire evaluateCenterControl" is REJECTED without a games gate. The dead code is centre OCCUPATION, not a
  space term, and the PeSTO PST already scores those squares at +8..+21 mg while the dead code would add a
  flat +25 -- the same double-count as the bishop-pair bonus whose REMOVAL measured +20.4. Deleting the dead
  code is cosmetic cleanup with zero Elo content.
box_cost: zero
queue_correction: >
  The queue entry conflated centre occupation (already in the PST, closed here) with a genuine space term
  (squares behind the pawn chain in enemy territory -- absent from NGN entirely, still open, ranked LOW,
  and requiring an ACPL pre-filter plus a real-clock games gate).
```
