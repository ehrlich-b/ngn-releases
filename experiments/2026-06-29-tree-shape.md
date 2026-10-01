# 2026-06-29 Tree-shape diagnostic — the EBF mechanism (ordering precision gates reduction)

Goal: stop guessing at search tweaks (all regressed). Measure WHERE NGN's tree is wide, via NGN's own
`sd_*` EBF-attribution dump (`debug on`), on 3 phase-diverse positions at mature depth, on the box.

## Data (depth 19-20, ngn_base.exe = HEAD)

| Position | total nodes | FMC | b_all (alltried/alln) | LMR re-search rate | mean reduction |
|---|---|---|---|---|---|
| Middlegame (r1bq1rk1/...10) d19 | 2.36M | 82.1% (200480/244076) | 6.25 (1103551/176463) | 1.2% (6493/528693) | 2.90 ply |
| Endgame (8/2p5/...) d20 | 2.08M | 86.4% (212758/246326) | 6.11 (1093881/178912) | 1.1% (4565/409828) | 2.13 ply |
| Tactical (kiwipete) d20 | 7.40M | 85.0% (593331/698214) | 6.07 (3079942/507125) | 1.0% (13555/1296440) | 2.94 ply |

Supporting: qsearch ~37% of nodes (mid). TT-move available at only ~23-36% of move-loop nodes (ttlist/mln).

## Thesis (unifies every failed search experiment)

Two facts that look contradictory:
1. NGN badly UNDER-reduces — only ~1.1% of LMR reductions ever need a re-search (they're almost always right). Huge SAFE headroom to reduce more and search deeper.
2. Yet every "reduce/prune more" REGRESSED (LMR scaling, cutNode, uniform bumps, LMP, futility — all tested negative).

The connection: **the LMR re-search only fires when a reduced move BEATS alpha** (catches under-reductions).
It is BLIND to the opposite error — a good move OVER-reduced below alpha is silently lost, no re-search, no
signal. So 1.1% measures only half the risk. Reducing more aggressively increases the SILENT over-reduction
of good moves that were ordered late — and at FMC ~84% (vs ~90%+ in strong engines), ~16% of cutoffs already
aren't on the best move, so good moves land in the reduced tail often enough to bleed Elo.

**Move-ordering precision is the upstream bottleneck.** It gates how aggressively NGN can reduce. Every
LMR/pruning tweak regressed because it front-ran the ordering fix.

## Next lever (tractable for the first time)

Raise FMC ~84% → ~90%, then cash in the reduction headroom. The unlock is methodological: **FMC is a fast
deterministic proxy** (one fixed-depth search, no games) — iterate ordering changes against FMC instantly,
SPRT-confirm only what moves it (the role ACPL plays for eval). First FMC-measurable targets: history
bonus/malus formula (`gravityUpdate`, bonus=depth²), history-source weighting (main/cont/followup summed
equal; strong engines lean on continuation history), and the low TT-move availability (~25%).

Reproduce: `(printf 'debug on\nposition fen <FEN>\ngo depth 19\n'; sleep 22) | ssh box ngn_base.exe`, grep `sd_`.
