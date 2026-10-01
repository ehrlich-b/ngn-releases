# 2026-08-03 T23 — RFP restricted to true cut nodes — MEASURED, NOT STAGED

**Zero box cost. Tree reverted, nodecheck re-verified at 295507 / 112109 / 667703.** ebfprobe was still
running when this was written; the decision does not depend on it (see below).

## Premise verified against HEAD

Reverse futility pruning gates on `depth <= 7 && !inCheck && beta-alpha == 1` (`search.go:1448`). The
null-window condition is the non-PV test, so — like NMP before T12 and probcut before T22 — **there is no
cut-node condition.** `cutNode` is in scope. One line.

## Why I thought this was the STRONG version of the argument, and why that was wrong again

I picked RFP because its premise looked like the cleanest fit for T12's reasoning: *"if position is very
good, return early"* is an explicit claim that the node will fail high, and **an all node is expected to fail
low** — a direct contradiction, where probcut's speculation had turned out to be about depth rather than node
type.

**On the same scrutiny I applied to T22, this fails the same way.**

**RFP's speculation is about the MARGIN, not the node type.** `staticEval - margin >= beta` is *direct,
position-specific evidence* about this position. The all-node expectation is a *structural prior* inherited
from how the parent ordered its moves. When the two conflict, the position-specific evidence is the more
reliable of the pair — so overriding the eval because of the node's structural label is the weaker call, not
the stronger one.

**That is now twice in one session that "T12's argument applies here too" did not survive inspection.** The
distinguishing feature of T12 is narrower than it first appears: **NMP is unsound in principle** (zugzwang —
passing can be *better* than any move), so at an all node it is a speculative device firing against both the
node's expectation *and* its own soundness caveat. Probcut and RFP are not unsound; they are *approximate*,
and approximation error is not fixed by node type. **The generalisation "restrict speculative prunes to cut
nodes" is not supported by T12 — only the NMP-specific instance is.**

## Cost — worse than anything that has ever kept

| position | base (T12+T19) | T23 | delta |
|---|---|---|---|
| kiwipete d12 | 295507 | 347629 | **+17.6%** |
| mid d12 | 112109 | 209839 | **+87.2%** |
| end d16 | 667703 | 753174 | **+12.8%** |

**RFP does a great deal of work at all nodes, and removing it nearly doubles the middlegame tree.** For
comparison against candidates with known verdicts: T12 **+30.1%** (kept +2.2), T19b **+36.1%** (null), T9a
**+13-32%** (lost -5.2), T22 **-11%** (shrinks). **T23's cost profile is the worst of the set**, and the two
nearest neighbours by cost both failed.

## Depth at fixed nodes — +2 net plies, and it changes NOTHING

| position | base d | T23 d | delta |
|---|---|---|---|
| quiet-mg-QGD | 13 | 13 | 0 |
| quiet-mg-closed | 11 | 13 | **+2** |
| lateMg-ph6-11 | 12 | 12 | 0 |
| najdorf-mg | 13 | 14 | **+1** |
| rook-eg-R4P | 16 | 16 | 0 |
| pawn-eg | 31 | 30 | -1 |

**+2 net — a GAIN, and it is being explicitly disregarded.** Per the T20 lesson, **a depth gain does not
predict a keep and may never promote a candidate**: T20 posted the campaign's best depth result and measured
-0.7. Recording this deliberately, because the probe came back *favourable* and the decision below goes
against it — that is the rule working as intended rather than being quietly dropped when inconvenient.

**Worth noting as a small pattern, not a conclusion:** this is the second candidate this session with the
signature *"grows the fixed-depth tree, yet gains depth at fixed nodes"* — the first was T19b (+36.1% tree,
+4 net plies), **which nulled.** Two instances is not evidence, but the combination has now failed to predict
success once and should not be read as encouraging.

## Decision — NOT STAGED. This is a ranking call, stated as such.

**It is not a formal rejection.** Nodecheck magnitude is not a rejection criterion, and ebfprobe cannot reject
an accuracy-buying candidate (T23 prunes *less*) under the 2026-08-01 narrowing. So the honest label is:
**measured, argued, and ranked below T22 — not queued.**

The reasoning: **a weak mechanism argument plus the worst cost profile on the board is not worth 8 hours of
box time** when T22 sits staged with a stronger argument (measured work saved, tree *shrinks*) and a
better-supported expectation.

**If T22 keeps**, that would be evidence for the general "restrict prunes at all nodes" idea and T23 becomes
worth revisiting — the +87% would still have to be repaid, but the prior would be different. **If T22 nulls
or loses, T23 should be dropped**, since it is the same family with a weaker argument and a much larger bill.
**T22 is the cheaper discriminator; run it first.** (Same discipline as waiting on T19b before its siblings —
which paid off, since T19b nulled and killed three untested siblings for free.)

```yaml
id: 2026-08-03-t23-rfp-cutnode
date: 2026-08-03
change_class: search heuristic (one-line pruning-gate restriction) -- MEASURED, NOT STAGED
result: >
  Nodecheck +17.6/+87.2/+12.8% -- nearly doubles the middlegame tree, the worst cost profile of any candidate
  measured this session. ebfprobe +2 net plies (a GAIN) -- disregarded per T20, which bars promotion on depth. Mechanism argument does not survive scrutiny: RFP's speculation is about the MARGIN
  (position-specific evidence) not the node type (structural prior), so the T12 parallel fails the same way
  it did for T22.
box_cost: zero
decision: not queued; ranked below T22, which is the cheaper discriminator for the whole idea
revisit_condition: only if T22 keeps; drop if T22 nulls or loses
```

## The generalisable finding

**T12's result does NOT license a family of "add `cutNode` to prune X" candidates.** Two attempts to
generalise it (T22 probcut, T23 RFP) both found that the mechanism being restricted speculates about
something *other than* node-type expectation — depth in one case, margin size in the other. **NMP is special
because it is unsound in principle, not merely approximate.** Recorded so the next agent does not work
through the remaining prunes (futility, LMP, SEE, history) expecting T12's result to transfer.
