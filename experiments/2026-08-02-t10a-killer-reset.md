# 2026-08-02 T10a — killer reset at ply+2 — REJECTED AT THE GATE

**Zero box hours. Tree reverted, nodecheck re-verified at 346662/149587/765656. Shelf copy
`output/t10a-killer-reset.patch`; FMC harness kept at `output/fmc-table.sh`.**

## Why it was built — it was the gate to a blocked family, not just a +2.5 candidate

T10a's value was never its own prior. **The entire T3 LMR-reduction family is gated behind T10 raising FMC**
from ~84% toward ~90% (T3a proved that adding reduction at NGN's ordering quality costs Elo, -7.9). T10b (the
other FMC-raising attempt) already failed. So T10a was the remaining candidate that could **unblock a whole
family**, which is far more leverage than the +2.5 [E] prior suggests.

It is also cheap to falsify: ordering work is proxy-gateable, so this cost no box time.

## The candidate

`killerMoves` is **ply-keyed and global across the whole tree**, so the slots at `ply+2` still hold whatever
some *previous, unrelated* subtree stored there. Those stale moves are then scored as killers in the
grandchild's ordering despite being refutations of a different position. T10a clears `killerMoves[ply+2]` on
node entry (the Ethereal/SF-standard reset), so a grandchild only ever sees killers produced by its own
siblings. Six lines in `alphaBetaPV`.

## Result 1 — FMC is FLAT, so it does NOT unblock T3

Measured at depth 12 on the 5 canonical ordering-diagnostic positions (harness reproduces the recorded
2026-07-26 baseline table to within ~0.3-1.9 pp, so the instrument is sound):

| position | base FMC | T10a FMC | delta |
|---|---|---|---|
| kiwipete | 88.78% | 89.66% | **+0.88** |
| quiet-mg-QGD | 87.54% | 87.22% | -0.32 |
| quiet-mg-closed | 83.76% | 83.10% | -0.66 |
| najdorf-mg | 85.12% | 84.80% | -0.32 |
| rook-eg-R4P | 80.76% | 81.50% | **+0.74** |

**Net +0.32 pp across 5 positions — mean +0.06 pp, 2 up and 3 down. Indistinguishable from flat.**

T10's job is to move FMC ~84% -> ~90%. This moves it by six hundredths of a point. **T3 stays gated**, and
that is the finding that actually matters here.

## Result 2 — the mechanism, and a correction to how this candidate was classified

**Killers are NOT a pure ordering mechanism in NGN.** Checked rather than assumed — `IsKillerMove` has three
consumers:

- **`search.go:1705` — a FUTILITY-PRUNE EXEMPTION.** Killers are exempt from futility pruning.
- **`search.go:1966` — an LMR REDUCTION ADJUSTMENT.** Killers are reduced one ply less.
- `search.go:2081` — diagnostic classification only (`CutByKiller`), behaviourally inert.

So clearing killers does not merely reorder moves: it makes the engine **prune more and reduce more**.
That has two consequences.

1. **The CLAUDE.md sanction "proxies may reject only pure ordering/EBF-mechanism changes" does not strictly
   apply**, because this is not a pure ordering change. Stating that explicitly rather than leaning on a rule
   that does not fit.
2. **The 2026-08-01 ebfprobe narrowing DOES apply**, and cleanly: T10a prunes and reduces *more*, which makes
   it **depth-buying**, the exact class where a depth loss is a valid rejection. Its own tree data agrees —
   fixed-depth nodecheck **-13.6% / -8.0% / -9.7%** (299628 / 137618 / 691743), i.e. it buys a smaller tree.

### Depth at fixed nodes — -2 net plies

| position | base d | T10a d | delta | base N | cand N |
|---|---|---|---|---|---|
| quiet-mg-QGD | 14 | 14 | 0 | 183331 | 161846 |
| **quiet-mg-closed** | 12 | 12 | 0 | **169011** | **373653** |
| lateMg-ph6-11 | 13 | 12 | -1 | 396246 | 294998 |
| najdorf-mg | 14 | 13 | -1 | 390110 | 237232 |
| rook-eg-R4P | 17 | 18 | **+1** | 354408 | 311531 |
| pawn-eg | 32 | 31 | -1 | 325143 | 280090 |

**A candidate that buys a 8-14% smaller tree at fixed depth still LOSES 2 net plies at fixed nodes.** That
combination is the tell, and the closed position shows why.

## The smoking gun — and NGN's own code predicted it

**`quiet-mg-closed` needs 373653 nodes to reach depth 12, against the base's 169011 — a 2.2x inflation** in
the one position that got *worse* while every other position's node count fell.

The comment sitting directly above the futility-prune killer exemption at `search.go:1700-1703` says:

> "extending to d8 prunes quiet maneuvering moves that hold alpha on closed positions — pruning a killer/
> counter there fails the node low and triggers a re-search (**the closed-position tree inflation the EBF
> probe flagged**). Keep searching them."

**That exemption exists specifically to prevent this failure mode, and T10a disables it by emptying the
killer slots.** The measurement reproduces the documented pathology exactly, in the position class it names.
This is not an inferred explanation; it is the engine's own recorded reason.

## Verdict and reopen condition (required for every proxy rejection)

**REJECTED at the gate. Zero box hours.** Two independent grounds: FMC flat (so it fails its own strategic
purpose of unblocking T3), and -2 net plies as a depth-buying change (the sanctioned rejection instrument),
with a mechanism proven from NGN's own source.

**REOPEN CONDITION:** only as **killer reset PLUS preservation of the exemptions** — i.e. reset the killer
*ordering* slots while keeping the futility-prune and LMR-reduction exemptions keyed on something that does
not go stale. That is a materially larger change than a 6-line reset and it has no prior behind it, so it is
**ranked LOW**. A bare re-run of the reset at a different ply offset (`ply+1`, `ply+4`) is **NOT** a valid
reopen — the failure is the lost exemptions, not the offset.

## The generalisable lesson

**This is the T18b pattern again: a correctly-transplanted standard technique that fails because NGN's
surrounding calibration differs.** In Ethereal/SF a killer reset is a pure ordering hygiene fix. In NGN
killers are **load-bearing for pruning and reduction**, so the same six lines mean something different and
strictly worse. **Before transplanting any technique, enumerate every consumer of the state it touches** —
`grep` for the accessor, not just the writer. Two minutes of grep would have reclassified this candidate
before it was built.

**T10 lane status: both FMC-raising attempts have now failed (T10b -2.4 by games, T10a flat by proxy). T3
LMR-reduction remains gated with no unblocking path left in the queue** — T10c (weight tuning) is a constant
tune, the class that has produced nearly every null. That is a real and reportable narrowing of the campaign.

```yaml
id: 2026-08-02-t10a-killer-reset
date: 2026-08-02
change_class: ordering + pruning/reduction (NOT pure ordering -- killers gate futility exemption and LMR)
result: >
  REJECTED at the gate. FMC net +0.32 pp over 5 positions (mean +0.06, 2 up 3 down) so it does not unblock
  T3; -2 net plies on ebfprobe as a depth-buying change; quiet-mg-closed inflates 2.2x (169011 -> 373653
  nodes for the same depth 12), reproducing exactly the closed-position pathology that NGN's own comment at
  search.go:1700-1703 says the killer futility exemption exists to prevent.
box_cost: zero
reopen_condition: >
  Only as killer reset PLUS preserved futility/LMR exemptions. Ranked LOW. A bare re-run at a different ply
  offset is NOT a valid reopen -- the failure is the lost exemptions, not the offset.
lane_consequence: >
  Both T10 FMC-raising attempts have failed (T10b by games, T10a by proxy). T3 LMR-reduction stays gated with
  no unblocking path left in the queue.
```
