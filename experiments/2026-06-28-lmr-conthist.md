# 2026-06-28 LMR continuous history scaling — real-clock self-play SPRT

First structural search-efficiency lever after the search-bound proof
(`project_search_bound_proven`). Replace the crude history-LMR (`histScore < -500
→ reduction++`, `> 1000 → reduction--`) with continuous scaling
(`reduction -= clamp(histScore/2048, -2, +2)`) — good history reduces less, bad
reduces more, smoothly. Aims to lower EBF (deeper tree at the same time).

```yaml
id:            2026-06-28-lmr-conthist
date:          2026-06-28
change_class:  search heuristic (efficiency / EBF) — node-spending, so the gate is REAL-CLOCK (fixed-nodes would under-credit the depth gain)
hypothesis:    finer history-scaled reductions search deeper at the same time ⇒ NEW > BASE
base_commit:   2dadef8 (build/win/ngn_base.exe)
candidate:     working tree, search.go LMR block (build/win/ngn_lmr.exe)
command:       sprt.exe -new .\ngn_lmr.exe -base .\ngn_base.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 0 -elo1 6 -maxgames 3000 -mingames 400
machine:       LAN 9800X3D (8c/16t), native Windows
openings:      sprt_openings sha256 974e4b5...
suite:         go test -short ./engine green; depth-at-400K +~0.3 ply (crude)
decision_rule: KEEP if real-clock self-play shows NEW ≥ BASE with a positive/non-regressing trend (search-efficiency change; self-play real-clock is the matching gate — unlike the mop-up, this is general, not endgame-vs-weaker). REJECT on a measured regression. If flat, retune the /2048 divisor (smaller = more aggressive) before shelving. Gauntlet-confirm an absolute move if KEEP.
result:        /2048 divisor = FLAT (-8 Elo [-44,+28] @ 358g; int-trunc made it a near-no-op for typical history magnitudes). Retuned /256 (active) = REGRESSION (-45 Elo [-88,-2] @ 254g; ±2 swing over-reduces).
verdict:       REJECT. Continuous history-LMR scaling is worse both ways — the crude ±1-at-thresholds was already near-optimal. Reverted to HEAD. This is the predicted local-optimum wall: single-axis LMR moves regress in both directions.
next:          Single-axis search retunes are exhausted (this + the memory int16 both failed today). Escape the local optimum via a COORDINATED move (joint-SPSA over the 11 margin params) or a STRUCTURAL ADD (more correction histories — targets the proven eval-optimism/horizon losses, additive so not a single-knob wall).
```
