# 2026-06-28 Non-pawn correction history — real-clock self-play SPRT (STAGED)

Structural ADDITIVE lever (not a single-knob retune, so not a local-optimum wall the
way the LMR divisor was). NGN has only `pawnCorrectionHistory`; strong engines run
several. This adds a second corrector keyed on each side's NON-pawn piece placement,
summed with the pawn correction at the static-eval site. Targets the proven
eval-optimism / horizon-error loss pattern (`project_loss_autopsy_eval`,
`project_search_bound_proven`): it nudges the optimistic leaf eval toward what search
actually finds, which is the cure for horizon errors in a search-bound engine.

```yaml
id:            2026-06-28-nonpawn-corrhist
date:          2026-06-28
change_class:  search/eval heuristic (eval correction) — gate = real-clock self-play SPRT
hypothesis:    a second (non-pawn) corrector reduces optimism at the leaves ⇒ NEW > BASE
base_commit:   2dadef8 (build/win/ngn_base.exe on box)
candidate:     working tree (moveorder.go nonPawnCorrectionHistory + search.go wiring) -> build/win/ngn_corr.exe (staged on box ~\ngn\sprt\)
command:       sprt.exe -new .\ngn_corr.exe -base .\ngn_base.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 0 -elo1 6 -maxgames 3000 -mingames 400
local_checks:  go build OK; go test -short ./engine GREEN (eval symmetry holds — correction is in search, not Evaluate); depth-14 search sane.
decision_rule: KEEP if real-clock self-play shows NEW >= BASE non-regressing. If it REGRESSES, first suspect over-correction (pawn+nonpawn each ±49cp, summed ±98) — halve the nonpawn weight (6245/131072 -> ~3000/131072) and retry before shelving. Then commit-or-revert.
status:        DONE (real-clock self-play on the box, 2026-06-28→29).
result:        1569 games, W-D-L 291-945-333, Penta Elo -9.5 95% CI [-20, +1] (pLLR -2.86), trinomial -9.3 [-26,+8], 0 flag-outs. SPRT stopped H0-ACCEPTED (not better).
verdict:       REGRESS (mild but consistent: point estimate -9.5, more L than W over 1569g). Cause = over-correction: pawn(±49) + nonpawn(±49) summed ±98cp, and the non-pawn key changes almost every move so it learns noisier signal. Per the decision rule, halve-retry launched (below) before shelving.
```

## Follow-on: halve the non-pawn weight (predeclared retry)

```yaml
id:            2026-06-28-nonpawn-corrhist-half
hypothesis:    if the -9.5 is summed-magnitude over-correction, weight 6245->3000 (nonpawn ±49->±23, summed ±72) moves it toward 0/positive
candidate:     ngn_corr_half.exe (HEAD + nonpawn-only @ 3000, NO minor) SHA256 d5dc268a25bc309ac52e98f556f748455bccc37a4d1348d70174cc3ccb67a339, on box
base:          ngn_base.exe (HEAD engine == 2dadef8) — same base as the full-weight run, so the only delta vs that -9.5 result is the weight
command:       sprt.exe -new .\ngn_corr_half.exe -base .\ngn_base.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 0 -elo1 6 -maxgames 3000 -mingames 400
decision_rule: clear positive (CI excludes 0 +) -> KEEP nonpawn@3000, then re-stage minor on it. ~0/no-op or still-negative -> SHELVE the non-pawn corrector (no net benefit) and stop the corrhist stack; box goes idle for an attended depth lever (do not launch one unattended).
status:        DONE (ran to maxgames; the held ssh dropped at exit 255 but the SPRT survived in Session 0 and completed).
result:        3000 games, W-D-L 587-1793-620, Penta Elo -3.8 95% CI [-11, +4] (pLLR -2.88, near the H0 bound). The early +16.8 @ G62 was noise.
verdict:       SHELVE. Halving moved it up from -9.5 to -3.8 (over-correction was partly real) but it is still NOT positive — no net benefit at either weight. Per the predeclared rule (~0/still-negative -> shelve), the non-pawn corrector is rejected. Code reverted to clean HEAD (preserved: scratchpad/nonpawn.patch, nonpawn_plus_minor.patch; binaries ngn_corr/ngn_corr_half/ngn_corr2 on box).
```

## Lane closure: additive eval-correction is DOWNSTREAM of the search fix

Two clean game verdicts (nonpawn @6245 = -9.5, @3000 = -3.8) say the correction-history ADD does
not move NGN's number. Principled reason, not bad luck: corrhist learns from `(search result -
static eval)`, so a SEARCH-BOUND engine is a poor teacher — the corrector mostly learns noise, worst
on the non-pawn key (changes almost every move). This matches `project_3000_mechanism` ("conthist
pays >=3200 not 3000"): advanced history/correction techniques pay off only once the search is
deep enough to teach them. So the minor/major/continuation corrhist follow-ons are NOT worth testing
now (same poor-teacher problem); the corrhist lane is CLOSED until the search-bound is fixed. The
real lever remains DEPTH (the +185/2ply proof), which needs attended work (singular double-ext touches
the search.go:1821 anti-explosion guardrail — see the night runbook).

