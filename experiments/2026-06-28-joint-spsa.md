# 2026-06-28 Joint-SPSA over the 11 search-margin params — KILLED at iter 50 (no-op signature)

Coordinated escape attempt for the local optimum that single-axis retunes keep hitting
(LMR both directions REJECT, history int16 NPS-flat). SPSA perturbs all 11 `TunableSearchParams`
together so it can move along a diagonal that no one knob can. If the params are genuinely at a
joint optimum, SPSA shows no gradient — which is exactly what it showed.

```yaml
id:            2026-06-28-joint-spsa
date:          2026-06-28
change_class:  tune (search-margin SPSA) — gate would be paired games on the converged params
base_engine:   ngn_base.exe (commit 2dadef8 == HEAD engine; empty non-test engine diff HEAD..2dadef8)
command:       spsa.exe -engine ngn_base.exe -tc 10+0.1 -iters 3000  (out=spsa.jsonl on box)
params:        DeltaMargin ExtensionBudget FutilityMargin FutilityMaxDepth LMRFullDepth LMRMoveThreshold NullMoveMinDepth NullMoveR ReverseFutilityMargin SingularDepth SingularMargin
pace:          ~1 min/iter sequential (iter 25 @ 21:33, iter 50 @ 21:59) ⇒ 3000 iters ≈ 50 h ≈ 2 box-days
```

## Why killed at iter 50

- `plus_pct` 48 → 50%: the perturbed candidate wins ~half the iters — **zero gradient signal**.
- 9 of 11 rounded params pinned at their start values through iter 50.
- The only two that moved are inside the noise band: DeltaMargin 100→97, SingularMargin 63→66
  (±3 wander, no consistent direction).

This is the texel-tune null all over again: the existing knobs are already at a joint optimum,
so SPSA has nothing to climb. Grinding ~2 more box-days for a near-certain no-op is the exact
"suck" to avoid on an overnight run. Killed `spsa.exe` (PID 37232) and its two worker
`ngn_base.exe` instances to reclaim the box for fast structural-lever verdicts.

## Verdict

NO-OP (proxy/early-stop, not a game verdict — SPSA never converged, so nothing to certify).
Reinforces the standing conclusion: **retuning the existing search knobs — single-axis OR
coordinated — is exhausted.** The remaining levers are STRUCTURAL ADDS (new correction
histories that attack the proven eval-optimism/horizon losses) or DEPTH adds (e.g. singular
double-extension), not parameter moves. Do not relaunch this SPSA without a new param axis.
