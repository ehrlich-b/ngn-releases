# 2026-06-28 Overnight autonomous runbook — correction-history lever sequence

Goal for the night: get REAL game verdicts on the correction-history structural lever (the
escape from the local optimum that retunes can't crack), one isolated change at a time on the
LAN box, recording each verdict. "Make it not suck" = no idle box, no speculative stacking, no
mid-run verdicts, every change isolated vs its immediate base.

## Box / loop mechanics

- Box: `ehrli@192.168.4.108`, `ssh -i ~/.ssh/id_ed25519 -o IdentitiesOnly=yes`. SPRT dir `%USERPROFILE%\ngn\sprt\`.
- ONE SPRT at a time (8c/16t, concurrency 8 saturates it). Never two real-clock runs on one box.
- Launch: a `run_X.bat` that runs `sprt.exe ... > X.out 2>&1` then `echo SPRT_..._DONE >> X.out`,
  invoked as `caffeinate -i ssh ... 'cmd /c "%USERPROFILE%\ngn\sprt\run_X.bat"'` with run_in_background=true.
  The held ssh re-invokes me when the run finishes.
- On re-invocation: FIRST `tasklist | findstr /i sprt` on the box.
  - sprt.exe GONE → run finished; read `X.out` tail for the final LLR/elo/CI and the DONE marker.
  - sprt.exe STILL RUNNING → the held ssh dropped early (it is flaky); box kept working in Session 0.
    Re-attach with a poll-monitor (below), do NOT relaunch the SPRT.
- Poll-monitor (use if the held launch drops but sprt is alive):
  `caffeinate -i bash -c 'until ! ssh -i ~/.ssh/id_ed25519 -o IdentitiesOnly=yes ehrli@192.168.4.108 "tasklist | findstr /i sprt.exe" >/dev/null 2>&1; do sleep 180; done; ssh ... "powershell -NoProfile -Command \"Get-Content $env:USERPROFILE\ngn\sprt\X.out -Tail 8\""'`
  run_in_background — exits (re-invoking me) when sprt.exe disappears.
- End of session: `tasklist | findstr /i "sprt ngn spsa"` must be empty of stragglers; kill any with
  single-`/IM` taskkills (multiple `/IM` per call fails on this box).

## Reading a verdict (SPRT is elo0=0 elo1=6, real-clock 10+0.1)

- KEEP   : NEW >= BASE and non-regressing (LLR to H1 bound, OR final point estimate >= 0 with CI not clearly negative). KEEP rule: a small non-regressing gain is a keep.
- REGRESS: final point estimate < 0 with CI excluding 0 on the negative side (or H0 bound hit while clearly negative).
- NO-OP  : flat, CI straddles 0 → KEEP rule keeps it (non-regressing) and we proceed.

## Decision tree

```
RUN 1 (LIVE): ngn_corr  vs ngn_base       [non-pawn corrector]   (early: -30 @ G116, trending REGRESS — not yet confirmed)
  KEEP/NO-OP → commit the non-pawn code. Next base = ngn_corr.
              RUN 2: ngn_corr2 vs ngn_corr [minor corrector, STAGED, SHA aa3ba2ec]
                 KEEP/NO-OP → commit minor. Build MAJOR corrhist on top (rook+queen key), RUN 3.
                 REGRESS    → revert minor code, shelve; stop the corrhist stack (summed over-correction). Pivot.
  REGRESS    → halve non-pawn weight 6245→3000 (predeclared), build ngn_corr_half, RUN 2': ngn_corr_half vs ngn_base
                 KEEP/NO-OP → commit non-pawn@3000; re-stage minor on that base, RUN 3'.
                 REGRESS    → corrhist-summing is harmful for NGN here: revert non-pawn code, SHELVE the whole class.
                              Box goes IDLE. Do NOT launch a speculative depth lever unattended — record state, leave for Bryan.
```

## Build recipes (cross-compile: GOOS=windows GOARCH=amd64 GOAMD64=v3 go build -o build/win/NAME.exe .)

Working tree right now = HEAD-engine + non-pawn + minor (the RUN-2 candidate source). Patches:
`scratchpad/nonpawn.patch` (non-pawn only), `scratchpad/nonpawn_plus_minor.patch` (both). Binaries on box.

- ngn_corr_half (REGRESS branch): re-save both patches first, then
  `git checkout engine/` → `git apply scratchpad/nonpawn.patch` → edit `nonPawnCorrectionValue` 6245→3000
  → build ngn_corr_half.exe → restore tree: `git checkout engine/ && git apply scratchpad/nonpawn_plus_minor.patch`.
- major corrhist (RUN 3, KEEP branch): mirror the minor block in moveorder.go keyed on
  `pieces[WhiteRook]|pieces[WhiteQueen]` / black; new constant pair; wire 3 sites in search.go; build ngn_corr3.exe.

## If the corrhist class shelves — NEXT LEVER is ATTENDED (do not run unattended)

Per the search-bound proof the binding constraint is DEPTH/EBF, so the next lever is a depth add.
Best candidate: **singular DOUBLE-extension** (search.go ~1808). Today's full-weight nonpawn already
shows the additive eval-correction lane is small/negative for NGN; depth is the proven lever instead.
Implementation notes from reading the code (why it is NOT unattended-safe):
- Current: `if singularScore < singularBeta { nextDepth++ }`, then a load-bearing guardrail
  `if nextDepth > depth+1 { nextDepth = depth+1 }` (search.go:1821) caps a node at +1.
- A real double-ext = when singular by a LARGE margin (`singularScore < singularBeta - M`), `nextDepth += 2`
  AND raise that guardrail to +2 for this case — i.e. it MODIFIES the anti-explosion invariant.
  EXTENSION_BUDGET (search.go:1836) stays the cumulative-path safety net.
- Risk: tree explosion / search instability; the suite catches only crashes, not over-deep waste. Gate on
  SPRT flag-outs==0 AND non-regression. NGN already removed the recapture extension as its #1 explosion source,
  so this area is delicate — needs eyes on the node-count + flag-out behavior, not an overnight fire-and-forget.

If RUN 2 (halve) shelves: record it, leave the box IDLE, hand this depth-lever spec to Bryan. Do NOT
manufacture low-EV corrhist key/magnitude variants just to keep the box warm — that is the "suck" to avoid.

## Guardrails

- One behavior change per run, candidate vs IMMEDIATE base. No stacking two untested changes.
- Do not commit a candidate's code until its own verdict KEEPS. Revert on REGRESS.
- Never verdict a killed/mid-run SPRT. Update the change's experiments/ manifest result+verdict at each stop.
- If a decision needs judgment I can't make mechanically (corrhist class exhausted), STOP and surface to Bryan; don't invent unattended risk.
