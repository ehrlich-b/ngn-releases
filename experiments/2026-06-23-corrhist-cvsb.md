# Experiment manifest: aux correction-history (C vs B)

Pre-registered per the Codex methodology review. Second clean isolated experiment of
the night, launched after B-vs-A (damping) reached a proper LLR bound = ACCEPT, which
makes B the unambiguous base. One isolated change, candidate vs its immediate base,
real-clock, predeclared decision rule. No autonomous git action — Bryan authorizes keep/discard.

## Hypothesis
Adding the material + non-pawn(W,B) auxiliary correction-history tables on top of the
already-shipped pawn correction table improves, or at least does not regress, real-clock
strength. This is the diagnosed-on-target structural anti-optimism lever (history-based
walk-back of an optimistic staticEval).

## Mechanism
`engine/moveorder.go`: three new `[2][corrHistSize]int` aux tables (material, nonPawnW,
nonPawnB), each contributing `table[stm][idx] * auxCorrWeight / 131072` (auxCorrWeight=2048)
to the correction applied to staticEval, summed with the shipped pawn term
(`* 6245 / 131072`). `ClearHistoryTable()` (every ucinewgame) now zeros ALL FOUR tables —
this is the fix for the cross-game leak that INVALIDATED the prior stacked run (regression
tests: TestClearHistoryResetsAuxCorrection, TestUcinewgameResetsCorrectionState, both pass).
At auxCorrWeight=0 the aux value function early-returns ⇒ node-identical to B (proven).
`engine/search.go` (+6): computes the aux indices once and threads them to the value/update calls.

## Change isolation (the single change)
- **B (base):** commit `f62bfeb` (HEAD = pre-corrhist, includes the kept damping). Box binary `ngn_B.exe` sha256 `a8eed3abd6811c504403bb2f2511dba8e594e199af766983bda1352bde0082be` (re-verified on box).
- **C (candidate):** B + the uncommitted aux-corrhist (`engine/moveorder.go` +127, `engine/search.go` +6 — the ONLY engine-binary-affecting unstaged changes; verified via `git status`/`git diff --stat`). Built from the working tree, GOOS=windows GOARCH=amd64 GOAMD64=v3. `ngn_C.exe` sha256 `6ba7fdc78dcd1b092a81504b388814f796283cf412143815bd7046346cc0eec6`.
- Diff B→C = the aux-corrhist tables + threading only. Full short engine suite GREEN with C's code (no new reds; aux-reset regression tests pass).

## Apparatus
- Harness: `sprt.exe` on the 9800X3D (native Windows), `-lowpower=false`.
- TC: **10+0.1 real clock**, concurrency 8. Openings `sprt_openings.txt` sha256 `974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222`.
- Primary metric: pentanomial LLR + Elo with CI.
- **A/A preflight: NOT re-run, and that is correct here.** The damping A/A already validated this exact config (10+0.1, conc-8) — Penta +0.9 [-14,+15], symmetric, 0/0 flag-outs. A/A validates the *config* (color/throughput bias), which is binary-independent. An A/A on C could NOT catch a corrhist state bug anyway: both C and its copy run identical code, so any leak would be symmetric and pass. The corrhist integrity is covered by the unit tests (aux-reset) + clean build + the depth-10 integration test. Crash/launch failures are caught by the start-confirm poller.

## Predeclared decision rule
H0: elo ≤ -3   H1: elo ≥ +3   (alpha=beta=0.05 → LLR bounds ±2.94). Max games **10000**, mingames 400.
- **pLLR hits +2.94 (H1)** → `ACCEPT`: aux-corrhist is a bullet self-play gain; recommend Bryan COMMIT moveorder.go+search.go.
- **pLLR hits -2.94 (H0)** → `REJECT`: aux-corrhist regresses even at bullet; recommend Bryan DISCARD the uncommitted corrhist.
- **Cap 10000, penta CI spans 0** → `INCONCLUSIVE`. **CRITICAL CAVEAT (pre-registered so it can't be misread as a reject):** corrhist scales UP with TC (peers: +12 Viridithas / +10 Stormphrax at LTC, ~+2 at STC), and this is the INCREMENTAL aux tables on top of an already-present pawn table (diminishing returns). So an INCONCLUSIVE at 10+0.1 bullet is EXPECTED and CONSISTENT WITH A REAL LONG-TC GAIN — it is NOT evidence corrhist is worthless. On the KEEP rule (on-diagnosis structural fix + no measured regression) it is a KEEP CANDIDATE, owed a long-TC gauntlet (#0 batch) to read its true value. Final keep/commit deferred to Bryan.

## Costs
- $0 (free local box). Wall ≈ 5h for 10000 games at conc-8 (~910 g/hr observed on B-vs-A), less if a bound fires early.

## Forbidden (per Codex overnight protocol)
No new hypotheses, no stacking another change, no mid-run rule reinterpretation, no
commit/discard of code, no harness edits, no lane closure, no absolute-rating claim.
Never verdict a killed/mid-run SPRT — run to a bound or the cap.

## Result (C vs B — COMPLETE 2026-06-23)
- status: **REJECT** (H0 accepted at a proper LLR bound — not a cap/kill)
- games / pairs: 7258 / 3629
- penta buckets: [LL 152, LD 891, {LW,DD} 1588, WD 871, WW 127] — LL 152 > WW 127 (C lost more pairs than it won; direction consistent, not an asymmetry artifact)
- Elo [CI]: **Penta -3.4 [-8, +2]** (decision stat); trinomial -3.4 [-11, +5]
- LLR / bounds: pLLR **-3.05** crossed -2.94 (H0); trinomial LLR -2.80
- flag-outs: new 0 / base 0 of 7258; no crashes
- throughput: ~930 games/hr (7258 in 7h48m, conc-8)
- verdict: **REJECT — the aux correction-history tables (material + non-pawn) REGRESS at bullet self-play (-3.4 elo, H0 bound).** Decisive (7258g, 0 flag-outs, clean bound). Per the KEEP rule: non-correctness feature + concrete measured regression ⇒ REJECT. The pre-registered "INCONCLUSIVE = keep candidate" caveat does NOT apply — this was a resolved H0, not a sub-resolution null. **The bullet-under-reads-corrhist caveat cannot rescue an H0:** if the aux tables were merely small-positive-under-read we'd see INCONCLUSIVE/H1, not a -3.4 regression. A TC-sign-flip (net-negative bullet / net-positive LTC) is *possible* but speculative; it would not change the verdict at our validated per-change instrument, only argue for STASHING (not deleting) the code if Bryan wants a future LTC re-test.
- **RECOMMENDATION (no autonomous git action):** discard the uncommitted aux-corrhist — scoped to `engine/moveorder.go` + `engine/search.go` + `engine/correction_history_test.go` (the aux tables + their now-moot reset-fix + their tests; reverts to shipped pawn-corrhist-only). KEEP the independent uncommitted fixes (`Makefile` test-swallow, `CLAUDE.md` anti-flounder). Alternative: `git stash` the 3 files to preserve for an LTC re-test. Pawn corrhist (shipped in B) is untouched either way.
