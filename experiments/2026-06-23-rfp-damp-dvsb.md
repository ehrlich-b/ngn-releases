# Experiment manifest: anti-optimism damped RFP return (D vs B)

Pre-registered per the Codex methodology review. Third clean isolated experiment of the
run. One isolated change, candidate vs its immediate production base (B = kept-damping HEAD),
real-clock, predeclared decision rule. No autonomous git action — Bryan authorizes keep/discard.

## Hypothesis
Damping the reverse-futility-pruning fail-soft return toward beta reduces the propagation of
NGN's diagnosed optimism bias (28/30 losses are eval-optimism/slow-bleed) and improves, or at
least does not regress, real-clock strength.

## Mechanism (the single change)
`engine/search.go` RFP block: on a reverse-futility cutoff, return `beta + (staticEval-margin-beta)/3`
instead of `staticEval - margin`. Still returns >= beta (cutoff semantics unchanged); only the
fail-soft magnitude is damped ~2/3 toward beta. Matches the cited peer formula (Stormphrax/Caissa
`lerp(eval, beta, ~0.69)`). One-line change; built in an isolated worktree off HEAD so the
uncommitted (rejected) corrhist is NOT in the candidate.

## Change isolation
- **B (base):** commit `f62bfeb` (HEAD, kept-damping). Box binary `ngn_B.exe` sha256 `a8eed3abd6811c504403bb2f2511dba8e594e199af766983bda1352bde0082be`.
- **C (candidate, ngn_D):** B + the one-line RFP-return damp ONLY. Built from worktree `/tmp/ngn-wt-rfp` @ f62bfeb, GOOS=windows GOARCH=amd64 GOAMD64=v3. `ngn_D.exe` sha256 `6b79c12f348263a46f86d21b5f435bc9004c17cc49e43ba99dbb756525bdeb03`.
- Build clean, full short engine suite GREEN (no new reds).

## Node-check (FILTER, recorded for honest interpretation — NOT a verdict)
Comparative (cand vs clean f62bfeb base, deterministic counts): kiwipete d12 274459 vs 347138 (0.79×),
**mid d12 240926 vs 79362 (3.04× — middlegame tree blow-up)**, end d16 324426 vs 445102 (0.73×). Geomean ~1.2×.
A score-changing search tweak butterflies move ordering; this one inflates the middlegame tree 3×.
⇒ **PRIOR: this is a real-clock COST concentrated in the dominant phase; the anti-optimism benefit
must overcome it. A regression (H0) is the more likely outcome.** The real-clock SPRT is the honest
arbiter (it charges the tree cost); the node count CANNOT verdict it (filter, per CLAUDE.md).

## Apparatus
- `sprt.exe` on the 9800X3D, `-lowpower=false`. TC **10+0.1 real clock**, concurrency 8.
- Openings `sprt_openings.txt` sha256 `974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222`. Config A/A-validated (damping A/A: +0.9, symmetric, 0/0).
- Primary metric: pentanomial LLR + Elo with CI.

## Predeclared decision rule
H0: elo ≤ -3   H1: elo ≥ +3   ([-3,3], alpha=beta=0.05 → LLR bounds ±2.94). Max 10000g, mingames 400.
- **pLLR +2.94 (H1)** → `ACCEPT`: anti-optimism overcomes the tree cost; recommend Bryan COMMIT the search.go change.
- **pLLR -2.94 (H0)** → `REJECT`: the middlegame tree cost wins; recommend DISCARD. (Self-limiting — a clear regression bounds out fast.)
- **Cap, CI spans 0** → `INCONCLUSIVE` → recommend **DISCARD**: this is a non-correctness change with a known middlegame tree cost; the KEEP rule requires improvement-or-correctness to keep, and INCONCLUSIVE demonstrates neither. Defer final to Bryan.

## Forbidden (Codex overnight protocol)
No new hypotheses, no stacking, no mid-run rule reinterpretation, no commit/discard of code, no
harness edits, no lane closure (do NOT "close the anti-optimism lane" on this one implementation —
a gentler damp or TT-decoupled variant is untested), no absolute-rating claim. Never verdict a killed/mid-run SPRT.

## Result (STOPPED EARLY as a clear dud 2026-06-23)
- status: **REJECT / DISCARD** (stopped at 2467g — NOT run to the formal bound)
- games: 2467 (510W 1409D 548L)
- Elo [CI]: -5.4 [-19, +8]; pLLR -1.73 (heading to H0 -2.94, not reached)
- flag-outs: 0; no crashes
- verdict: **DISCARD.** Consistently negative over 2400+ real-clock games (new lost more than it won throughout). Per the KEEP rule this is not a keep (no improvement shown); per the pre-registration BOTH H0 and INCONCLUSIVE → discard, so stopping before the formal bound costs ZERO decision quality — grinding to -2.94 would only have burned box time to confirm a loser. This is a screening decision on a large, stable sample — NOT a precise elo verdict from a noisy kill (distinct from the G68 anti-pattern). The node-check's predicted 3× middlegame tree cost showed up at real clock exactly as flagged. **Anti-optimism lane NOT closed** — only this one implementation (2/3-to-beta RFP damp) is rejected. Worktree removed; no main-tree git change (the damp lived only in the worktree). **CADENCE LESSON: kill clear duds by ~1000-1500g, don't grind screens to the bound.**
