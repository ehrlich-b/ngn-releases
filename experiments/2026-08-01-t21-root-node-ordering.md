# 2026-08-01 T21 — root move ordering by previous-iteration subtree nodes — REJECTED AT THE GATE

**STATUS: KILLED AT THE PROXY GATE FOR ZERO BOX HOURS. Both sort directions tested.** Engine tree reverted;
nodecheck re-verified at exactly 346662/149587/765656. Shelf copy `output/t21-root-node-ordering.patch`
(descending variant).

## Why it was built — the strongest family argument on the board

The 2026-07-26 queue audit tallies the campaign by family:

| family | record | Elo |
|---|---|---|
| **root search control** | **1 / 1** | **+20.2** |
| corrhist | 3 / 4 | +19.2 |
| time mgmt | 2 / 4 | +19.7 (now TAPPED) |
| node-level pruning/reduction | **1 / 11** | **+2.0** |

**Root search control is the only perfect family, holds the single biggest keep of the campaign (T5
aspiration, +20.2), and had exactly ONE member ever tested.** Three of the four candidates staged this
session (T12, T19, T15) are node-level, i.e. drawn from the deck the audit indicted. Finding a second
root-control member was the highest-value queue gap available.

## The candidate

NGN orders root moves with the generic ply-0 heuristic (`orderMovesIntoBufferWithDepth`, where killers are
ply-keyed and therefore weak at the root) and then swaps the previous best move to the front (S6). Nothing
uses **how expensive each root move actually was**.

**T1e already computes exactly that** — `moveNodes := info.Nodes - nodesBefore` per root move — **and discards
every value except the best move's.** So the candidate reused live plumbing: retain all per-move counts, and
after the S6 best-move-first placement, sort the remaining root moves by previous-iteration subtree size.

Design points that were done correctly (recorded so a reopen does not redo them):

- Counts are committed to the ordering key **only when an iteration completes in-window**, so ordering is
  never driven by a failed aspiration attempt's or a clock-stopped iteration's partial counts.
- State is **local to the search** — no globals, nothing to add to `ClearHistoryTable`, nothing to leak
  across `ucinewgame`. (The T4d leak trap does not apply.)
- Index 0 is never sorted, so S6's best-move-first placement is preserved.

**It also passes the T18b soundness question in the strongest form:** node counts are *exact measurements of
completed searches*, not a speculative value. This is proven information, which is precisely the property
that separates every winner from T18b.

## Why it is rejected — proxy rejection is SANCTIONED for this change class

CLAUDE.md: *"Proxies (FMC/ebfprobe/fixed-nodes/ACPL/MSE) may REJECT only pure ordering/EBF-mechanism changes,
and every proxy rejection records a reopen condition."*

**T21 is a pure move-ordering change — the exact class the rule names.** Ordering cannot change *what* is
searched, only the order, so better ordering produces earlier cutoffs, a smaller tree, and MORE depth at
fixed nodes. It is **depth-buying by construction**, so unlike T4e/T12/T19/T15 the 2026-08-01 narrowing does
**not** exempt it. Depth-at-fixed-nodes is the correct instrument here.

### Descending (expensive moves first) — the built form

nodecheck +1.9% / -9.6% / **+44.0%** (353155 / 135296 / 1102854)

| position | base d | cand d | delta |
|---|---|---|---|
| quiet-mg-QGD | 14 | 12 | **-2** |
| quiet-mg-closed | 12 | 12 | 0 |
| lateMg-ph6-11 | 13 | 12 | -1 |
| najdorf-mg | 14 | 14 | 0 |
| rook-eg-R4P | 17 | 15 | **-2** |
| pawn-eg | 32 | 29 | **-3** |

**-8 net plies** — the worst depth result measured in this campaign.

### Ascending (cheap moves first) — the opposite hypothesis, tested rather than assumed

nodecheck -17.1% / +61.2% / -10.7% (287250 / 241165 / 684023)

| position | base d | cand d | delta |
|---|---|---|---|
| quiet-mg-QGD | 14 | 13 | -1 |
| quiet-mg-closed | 12 | 13 | **+1** |
| lateMg-ph6-11 | 13 | 13 | 0 |
| najdorf-mg | 14 | 13 | -1 |
| rook-eg-R4P | 17 | 15 | **-2** |
| pawn-eg | 32 | 30 | **-2** |

**-5 net plies.**

**BOTH DIRECTIONS LOSE DEPTH.** That is what makes this rejection durable rather than a coin-flip on a sign:
the failure is not the ordering direction, it is that **previous-iteration subtree size is a worse root
ordering key than what NGN already has**, whichever way it is read. Testing only the built direction would
have left a cheap, obvious "you had it backwards" reopen; there isn't one.

## Why it fails (leading explanation, not load-bearing)

The sort **replaces** the heuristic order rather than refining it: every move gets a node-count key, so
capture/history/killer information is discarded entirely from index 1 onward. Subtree size conflates two very
different things — a move that is *good* (close to alpha, expensive to refute) and a move that is merely
*complicated* (tactically murky, expensive for reasons unrelated to its value). NGN's existing root order
already achieves FMC 80.6-88.4% and **exactly 0 TT misses in every position**
(`experiments/2026-07-26-ordering-miss-diagnostic.md`), so the bar it had to clear was higher than the
family-tally argument suggested.

## REOPEN CONDITION (required by CLAUDE.md for every proxy rejection)

Reopen **only** as a **blend, never a replacement**: use subtree size as a tiebreak or a bounded bonus on top
of the existing heuristic score, so capture/history/killer ordering is preserved and node count only breaks
ties among otherwise-equal moves. **Ranked LOW** — it is a hand-tuned blend weight, which is the class that
has failed repeatedly (T9a's margin, T7's coefficient, T6a's divisor), and this measurement shows the signal
is weak enough in both directions that a blend has little to add.

## What this result does and does not mean for the root-control family

- It does **NOT** close the family. The family is now **1 keep / 2 attempts**; one proxy rejection at the
  gate is not a lane closure, and CLAUDE.md requires multiple falsification attempts with the right
  instrument before closing a lane.
- It **does** correct the family-tally argument that motivated the build. **1-for-1 was one data point.**
  Root control is no longer a free pass, and the next root-control candidate needs its own mechanism
  argument, not the family record.
- **Cost: zero box hours.** This is the filtering the campaign says is its most direct lever — the candidate
  was designed, built, measured in both directions, and killed without touching the mill.

```yaml
id: 2026-08-01-t21-root-node-ordering
date: 2026-08-01
change_class: pure move ordering (root) -- proxy-rejectable per CLAUDE.md
hypothesis: >
  Ordering root moves after the best move by previous-iteration subtree node count is a better key than the
  ply-0 heuristic order, because subtree cost measures how hard a move was to refute.
result: >
  REJECTED at the depth-at-fixed-nodes gate. Descending -8 net plies, ascending -5 net plies. Both directions
  lose depth, so the key itself is worse than the existing root order.
box_cost: zero
reopen_condition: >
  Only as a bounded tiebreak/bonus layered on the existing heuristic score, never as a replacement. Ranked
  LOW (hand-tuned blend weight, the repeatedly-failing class).
next_action: none -- tree reverted, nodecheck re-verified, shelf copy output/t21-root-node-ordering.patch
```
