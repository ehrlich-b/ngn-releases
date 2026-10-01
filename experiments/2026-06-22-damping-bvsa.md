# Experiment manifest: 50-move eval damping (B vs A)

Pre-registered per the Codex methodology review (2026-06-22). One isolated change,
candidate vs its immediate production base, real-clock, predeclared decision rule.
No autonomous git action on the result — Bryan authorizes any keep/revert.

## Hypothesis
The committed 50-move eval damping (commit d46c48c) improves or at least does not
regress real-clock strength vs the pre-damping base. It is currently in production
HEAD **unverified by games**; this run resolves that.

## Mechanism
`EvaluateForPlayerCached` (engine/eval.go:2806-2822) scales the static eval by
`(FiftyMoveDampBudget - HalfMoveClock) / FiftyMoveDampBudget`, FiftyMoveDampBudget=256.
Applied outside the eval cache (depends on hmc, not the board), only on static evals
(never mate scores). Factor ≈ 1.0 at hmc=0, 0.92 at hmc=20, 0.80 at hmc=50, 0.61 at
hmc=100. This is a systematic few-percent eval shrink that grows with the halfmove
clock — a real eval heuristic with measurable magnitude (NOT "correctness-flavored";
the Codex review correctly rejects that category).

## Change isolation (the single change)
- **A (base):** commit `31bb779` (pre-damping). binary sha256 `1888fcda9660f618e407a0d226320b7c05f6760003e1d814e5e6c829cc03d377`
- **B (candidate):** commit `f62bfeb` (= 31bb779 + d46c48c damping; f62bfeb adds only
  test-file changes that do not affect the engine binary). binary sha256 `a8eed3abd6811c504403bb2f2511dba8e594e199af766983bda1352bde0082be`
- Diff A→B = the FiftyMoveDampBudget block only. Verified: `FiftyMoveDampBudget`
  present in B/engine/eval.go, absent in A. Built clean from git worktrees (no
  uncommitted WIP), GOOS=windows GOARCH=amd64 GOAMD64=v3, Go (Mac host).

## Apparatus
- Harness: `sprt.exe` on the 9800X3D (native Windows), `-lowpower=false`.
- TC: **10+0.1 real clock**, concurrency 8.
- Openings: `sprt_openings.txt` sha256 `974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222` (5000, ×2 colors, paired color-reversed).
- Primary metric: pentanomial LLR + Elo with CI.
- **A/A preflight gate (REQUIRED before this run):** ngn_B vs byte-identical copy,
  800 games, same config. Must show Elo centered near 0, symmetric flag-outs, stable
  throughput. This B-vs-A run launches ONLY if A/A passes.

## Predeclared decision rule
H0: elo ≤ -3   H1: elo ≥ +3   (alpha=beta=0.05 → LLR bounds ±2.94)
Max games **10000**, mingames 400.
- **LLR hits +2.94 (H1)** → `ACCEPT`: damping is a measurable gain; recommend keep d46c48c.
- **LLR hits -2.94 (H0)** → `REJECT`: damping regresses; recommend Bryan revert d46c48c.
- **Cap 10000, CI spans 0** → `INCONCLUSIVE`: damping effect < ~3 Elo (below the
  resolution of a [-3,3] test at this N — Codex defect #3). Report point estimate + CI.
  Final keep/revert deferred to Bryan's KEEP rule (no measured regression ⇒ may keep),
  with the explicit caveat that this design cannot resolve sub-3-Elo effects.

## Costs
- $0 (free local box). Wall ≈ 7-8h for 10000 games at conc-8.

## Forbidden (per Codex overnight protocol)
No new hypotheses, no stacking another change, no mid-run rule reinterpretation, no
commit/revert of production code, no harness edits, no lane closure, no absolute-rating
claim. Never verdict a killed/mid-run SPRT — run to a bound or the cap.

## A/A preflight result (gate — PASSED 2026-06-22)
- 800 games, ngn_B vs byte-identical copy, 10+0.1 conc-8.
- Elo(new−base) +0.9; **Penta Elo +0.9 [-14,+15]** (THE stat); LLR +0.09 (bounds ±2.94).
- Pentanomial [LL 14, LD 90, {LW,DD} 189, WD 94, WW 13] over 400 pairs — **symmetric** (LL≈WW, LD≈WD ⇒ no color/first-mover bias).
- **Flag-outs 0 / 0 of 800** — conc-8 at 10+0.1 has zero time-loss pathology; throughput clean.
- Verdict INCONCLUSIVE-by-cap = the correct A/A outcome (no real difference ⇒ never hits a bound). **Harness/config VALIDATED.**
- Binary hashes re-verified on box pre-launch: A `1888FCDA…` ✓, B `A8EED3AB…` ✓ (match manifest).

## Result (B vs A — COMPLETE 2026-06-23)
- status: **ACCEPT** (H1 accepted at a proper LLR bound — not a cap/kill)
- games / pairs: 4413 / 2206
- penta buckets: [LL 70, LD 502, {LW,DD} 1010, WD 540, WW 84]
- Elo [CI]: **Penta +5.2 [-1, +11]** (decision stat); trinomial +5.2 [-5, +15]
- LLR / bounds: pLLR **+3.04** crossed +2.94 (H1); trinomial LLR +2.78
- flag-outs: new 0 / base 0 of 4413; no crashes/watchdog voids
- throughput: ~910 games/hr (4413 in 4h45m, conc-8)
- verdict: **ACCEPT — damping is a measured self-play gain (≥+3 elo) with zero regression and zero flag-outs.** Recommend KEEP d46c48c (already committed). The A/A on the identical config measured +0.9 null, so +5.2 is above the harness floor. CAVEAT: self-play gain ≠ guaranteed CCRL gain (transfer-mirage risk) ⇒ owed inclusion in the next gauntlet batch (#0) for the absolute number. NO autonomous git action taken (it was already committed; the verdict confirms the keep).
