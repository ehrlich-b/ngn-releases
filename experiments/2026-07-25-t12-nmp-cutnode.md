# 2026-07-25 T12 — NMP restricted to cut-nodes (PRE-VERIFIED, not yet launched)

**STATUS: QUEUED, NOT LAUNCHED.** The box is running T8 SPSA round 1 until ~2026-07-26 20:00 EDT. This file
records the premise verification and gate work done in advance so the run can launch the moment the box frees,
per the anti-stall rule (every session that sees a run finish records it AND launches the next queued item).

Prior: **+9.7 [S #230]** (Stash). Change size: **one line.**

## Premise VERIFIED from code (not assumed)

Two of the queue's claims turned out stale today (`DoubleExtMargin`, LMP dims), so this premise was checked
against HEAD before any box time was committed.

**The gap is real.** NMP's condition (search.go:1469) is:

```go
if SearchToggles.NullMove && !inCheck && depth >= NULL_MOVE_MIN_DEPTH && canNull && !isPV && staticEval >= beta &&
    hasNonPawnMaterial(&pos.Board, pos.Turn()) &&
    !(ttHit && ttNodeType == UpperBound && ttEval < beta) {
```

There is **no cut-node condition** — NMP fires at every non-PV node, all-nodes included.

**The plumbing already exists**, so this is not a new-concept change. `cutNode` is a parameter of
`alphaBetaPV` (search.go:1164), correctly propagated (`!cutNode` to the null-move child at 1488 and the scout
child at 1993, `!isPV && !cutNode` to the main child at 1906), and **already consumed in a search decision** —
IIR gates on it at 1349 (`ttMove == EmptyMove && depth >= 4 && cutNode && accPhase >= 8`). So the candidate is
literally `&& cutNode` inserted into the condition above.

## Behavioral-delta gate (the >= ~1% rule earned from T7) — PASSES DECISIVELY

Measured locally, candidate built and reverted; tree confirmed back to exact baseline afterward.

| Position | Base (HEAD lock) | T12 | Delta |
|---|---|---|---|
| kiwipete d12 | 346662 | 451141 | **+30.1%** |
| mid d12 | 149587 | 136100 | **-9.0%** |
| end d16 | 765656 | 796906 | **+4.1%** |

Nothing like T7's 0.001%. The mixed direction matters and is **encouraging**: if the change were a pure
slowdown every position would grow. Mid *shrinking* 9% says removing unsound all-node null cutoffs improves
what gets stored and ordered downstream in at least some position types, rather than only costing breadth.

## Mechanism fires — and the obvious witness is CONFOUNDED (methodology note)

First attempt at a witness compared `nmtry` between the binaries at kiwipete d12:

```text
BASE:  nmtry 10850  nmcut 4107
T12:   nmtry 14796  nmcut 5376
```

**NMP attempts went UP under a strict restriction.** That is not a contradiction and not a bug — it is a
confound. Adding `&& cutNode` makes all-nodes expand their full move list instead of cutting off, those
children are cut-nodes (`!cutNode` propagation), so the tree grows 30% *and* becomes enriched in exactly the
node type where NMP still fires. **Absolute counters cannot witness a restriction when the restriction
changes tree composition** — a rate (per node) does not fix it either, since it also rose (3.13% → 3.28%).

Resolved with a temporary direct probe (inserted, measured, reverted): count nodes where every NMP condition
holds **except** `cutNode`.

```text
kiwipete d12: 473 NMP attempts blocked solely by the new cut-node gate  (vs 14796 that still fire)
```

So the gate **is** binding, and it is **narrow but high-leverage**: only ~3.1% of otherwise-eligible NMP sites
are all-nodes, yet blocking 473 of them grows the tree by ~104k nodes (~220 nodes per blocked cutoff). This is
the T9a-style lock — the mechanism provably fires — so whatever this run returns will be evidence about the
hypothesis, not merely about the patch.

## Honest expectation

**Below the +9.7 prior.** Two reasons to discount it:

1. The mechanism touches only ~3.1% of NMP-eligible nodes in the probed position.
2. At fixed real clock a bigger tree buys soundness with depth, which is **exactly how T9a lost** (-5.2:
   double extensions bought a ply on 33% of singular nodes and paid for it in breadth). kiwipete +30.1% is a
   real headwind that the SPRT must overcome.

The counterweight is that T12 *removes* work at all-nodes where NMP's "already >= beta" premise is weakest,
and mid d12 shrinking 9% suggests a genuine ordering/TT-quality gain rather than pure cost. Treat as a
+2-6 class candidate that could plausibly land null; it is worth the box time because it is one line with a
measured external prior and a proven-firing mechanism.

## Launch plan (when the box frees)

Standard per-change T-item gate, identical to every other run on this rig:

```text
BOXSPRT_BIN unset (sprt.exe); candidate ngn_t12.exe vs base ngn_t5.exe 0a8f65b7…
-tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt
-elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 8000 -mingames 300
-resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
```

Decision rule: pLLR >= +2.94 KEEP (Batch-3 keep, re-lock nodecheck to the T12 numbers); pLLR <= -2.94 SHELVE
and revert; capped with point est >= +1 and pLLR > 0 PROVISIONAL keep into Batch 3; capped nonpositive SHELVE.
Full run record to be filled from `experiments/RUN_RECORD_TEMPLATE.md` at launch.

**Sequencing caveat:** T8's converged vector will be gated by its own SPRT against unmodified HEAD. If T12 is
kept first, T8's confirmation SPRT must be re-based onto the T12 tree, since T8 tuned `NullMoveR` and
`NullMoveMinDepth` — both NMP dims — on a base where NMP fires at all-nodes. **Do not run the two gates
against different bases and pool them.** Simplest ordering: finish T8's gate first, then T12.
