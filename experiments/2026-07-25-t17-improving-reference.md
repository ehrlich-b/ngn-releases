# 2026-07-25 T17 — `improving` reads a never-written reference (defect fix)

Pre-registered BEFORE launch (CLAUDE.md work-loop step 5). Candidate vs its immediate base, which is also the
newly-certified Batch-3 base. **Batch-3 keep candidate #1.**

## The defect (proven from code + measured, 2026-07-24 independent review)

`improving := !inCheck && ply >= 2 && staticEval > info.StaticEvalStack[ply-2]` (search.go:1419) had two bad
references:

**(a) Slot 0 was never written.** `StaticEvalStack[ply]` is assigned only inside `alphaBetaPV`, which is only
ever entered at ply >= 1 — both root loops call it with ply 1, and the ply-0 `alphaBeta` wrapper is dead code.
So **every ply-2 node compared its eval against a zeroed slot**, computing *"is my eval positive"* rather than
*"is my eval trending up"*. Measured directly: slot 0 held **0** after a full depth-8 search while slots 1/2/3
held 653/419/271.

**(b) `-INFINITY` was used as a sentinel and then compared as a value.** An in-check node at ply < 2 stores
`staticEval = -INFINITY`, so a node reading that slot computed `realEval > -2147483647` = **always true**, and
the sentinel propagated down the odd-ply chain through consecutive checks. Measured: **1068 sentinel
references in the endgame position, 100% of them spuriously `improving = true`.**

Measured frequency of *outcome-changing* improving decisions (temporary counters, since reverted):
kiwipete d11 **1798/85390 = 2.1%**, mid d11 **879/36799 = 2.4%**, end d15 **471/310886 = 0.15%**.

**Impact is leveraged, not proportional.** A wrong `improving` mis-sets FOUR pruning knobs at once — RFP
margin `RFM*(depth-imp)` (improving ⇒ prunes MORE), futility margin `FM*(depth+imp)` (prunes LESS), the LMP
threshold row (prunes LESS), and LMR `if !improving { reduction++ }` (reduces LESS) — and it does so at ply-2
and ply-3 nodes that each root an enormous subtree. A 2% decision-level error is therefore not a 2% tree
error, which the nodecheck below confirms.

## The fix (3 hunks, ~40 lines with comments)

1. `const unknownStaticEval = -INFINITY` — names the sentinel so it stops looking like a number.
2. `improvingVsPrevTurn(info, ply, staticEval)` — walks back 2 plies, then 4 (Stockfish's fallback) when the
   reference is unknown, and returns **false** when nothing usable is on the stack. Replaces the raw
   comparison at the call site.
3. `seedRootStaticEval(pos, info)` — writes slot 0 with the root's corrected static eval (raw eval + all three
   correction tables, mirroring `alphaBetaPV`'s non-check branch), or the sentinel when the root is in check.
   Called from **both** root entry points (`searchIterativeDeepeningUnsafe`, `searchFixedUnsafe`).

The in-check store at search.go:1393 now writes `unknownStaticEval` instead of a bare `-INFINITY` — a pure
rename, no behavior change on its own.

## Regression test + negative control

`TestImprovingReferenceIsWritten` (engine/reentrancy_test.go) asserts: slot 0 is non-zero and non-sentinel
after a real depth-6 search; a sentinel reference does **not** read as improving at ply 2 or ply 3; a known
reference still compares correctly in both directions; and the 4-ply fallback fires when the 2-ply reference
is unknown.

**Negative control run (the durable lock against T7's silent-no-op failure mode):** with only the two
`seedRootStaticEval` calls commented out, the test FAILS with
`StaticEvalStack[0] is 0 after a depth-6 search`. The test therefore genuinely discriminates the fix from its
absence, verified rather than assumed. Fix restored and re-verified green afterward.

## Behavioral-delta gate (the >= ~1% rule earned from T7)

| Position | Base (T5 lock) | T17 | Delta |
|---|---|---|---|
| kiwipete d12 | 346662 | 247063 | **-28.7%** |
| mid d12 | 149587 | 208825 | **+39.6%** |
| end d16 | 765656 | 709959 | **-7.3%** |

**Passes the gate by a wide margin — this is nothing like T7's 0.001%.** The mixed direction is the expected
signature, not a red flag: a spuriously-true `improving` pushes RFP toward *more* pruning while pushing
futility/LMP/LMR toward *less*, so correcting it moves the tree in opposite directions depending on which
knob dominates in a given position. Nodecheck baselines will need re-locking **only if this is kept**.

```yaml
id: 2026-07-25-t17-improving-reference
date: 2026-07-25
change_class: search heuristic (defect fix that changes search behavior — NOT a "correctness-flavored" free pass; CLAUDE.md is explicit that no such class exists, so this takes the full real-clock games gate)
hypothesis: >
  repairing the two bad references in the improving heuristic — seeding slot 0 with the root static eval, and
  treating the in-check sentinel as unknown rather than as a value — makes RFP/futility/LMP/LMR margins fire
  on a true "is my eval rising" signal instead of a corrupted one, and is worth >= +3 Elo (H1) rather than
  <= -3 (H0). Same defect class as T1b (+13.1), which was a variable WRITTEN but never READ; this is one READ
  but never WRITTEN. Honest expectation is nonetheless SMALLER than T1b: it changes ~2% of improving
  decisions, so this is a cheap SPRT, not a headline swing.
base_commit: 734a8ee            # == certified Batch-3 base 172e9e1 engine == T5 keep 5b93966 engine
candidate_commit: 734a8ee + t17.patch (UNCOMMITTED per policy; shelf copy output/t17.patch)
base_binary_sha256: 0a8f65b75e59c376052f83b14787b0af2eba65552cedfee6f70ce576891ca721   # on-box ngn_t5.exe
candidate_binary_sha256: c928604155baec2c0133e5670edeaa7e475bd62cf5551d3ea8ba7d9f331578c9   # on-box ngn_t17.exe
build: GOOS=windows GOARCH=amd64 GOAMD64=v3 go build -o output/ngn_t17.exe main.go ; go1.26.2, no ldflags/trimpath
harness_commit: sprt.exe sha256 30c33e0512725b7f552d8a1cf72ba6f1e0deb4dcb122c6b6ba8f6433c686a762 (M5-aware validated mill, box-staged, untouched); launcher scripts/boxsprt.sh untouched
command: sprt.exe -new .\ngn_t17.exe -base .\ngn_t5.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 8000 -mingames 300 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
machine: AMD Ryzen 7 9800X3D 8c/16t, native Windows (LAN box 192.168.4.108)
go_version: go1.26.2 (local cross-compile, darwin/arm64 host)
goarch_goamd64: windows/amd64 GOAMD64=v3
tc: 10+0.1 (seconds; bullet)
concurrency: 8
openings: canonical sprt_openings (5000 lines)
openings_sha256: 974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222
adjudication: STANDARD-ON (-resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80; M5-validated non-distorting)
maxgames: 8000
mingames: 300
aa_preflight: standing 2026-07-03 M5-enable adjudication-ON A/A (experiments/2026-07-03-m5-adjudication-aa.md) — same TC / concurrency / adjudication flags / machine / openings config, penta -1.1 [-13,+11], pLLR -0.18, 0/0 flag-outs, 961 g/hr, DONE_EXIT_0. Same basis every T-item and both certifications on this box use.
tests_run: >
  go test -short ./engine -count=1 (ok 5.5s); go test -short -race ./engine -count=1 (ok 24.6s);
  go test -short ./... -count=1 (all ok); go build ./... + go vet ./engine clean.
decision_rule: >
  SPRT [-3,+3] on pentanomial pLLR, bounds +/-2.94, maxgames cap 8000.
  pLLR >= +2.94 -> KEEP, commit to main as Batch-3 keep #1, re-lock nodecheck baselines to the T17 numbers.
  pLLR <= -2.94 -> SHELVE and revert (keep output/t17.patch).
  Capped with point estimate >= +1 and pLLR > 0 -> PROVISIONAL keep into Batch 3 (batch-certification path).
  Capped nonpositive -> SHELVE and revert.
  NOTE the asymmetry this fix creates: a SHELVE verdict does NOT restore the defect's legitimacy. If T17 is
  shelved, the reference bug is still a bug; the finding is then that the search had ADAPTED to it (the
  surrounding margins were hand-tuned on top of the corrupted signal), which makes the fix a prerequisite for
  T8 rather than a standalone gain. Record that explicitly rather than filing it as "improving is fine".
games_or_pairs: 4256 games / 2128 pairs
result: penta -6.4 [-14, +1], pLLR -2.88 printed (true min -2.96 @G4253, crossed -2.94 at G4248); trinomial -6.4 [-17,+4] LLR -2.52
flags_errors: 0 flag-outs new / 0 base of 4256; 0 crashes / illegal / no-move; DONE_EXIT_0; 4h25m46s = 960 g/hr vs the 961 baseline
verdict: H0 ACCEPTED — SHELVE and revert (shelf copy output/t17.patch)
next_action: T8 SPSA round 1 on the UNFIXED base; T17 reopens ONLY bundled with a retune of the four consumer knobs
```

## Why T17 runs before T8 (dependency, not a re-rank)

T17's own queue entry called it "a cheap SPRT, not a queue-jumper", and that judgement stands — its expected
value has **not** been revised upward. It runs first for a dependency reason: **T8 SPSA tunes RFP margin,
futility margin, LMP thresholds and LMR reduction — precisely the four knobs this defect mis-sets.** Tuning
those constants on top of a proven-broken input would converge them toward values that *compensate* for the
bug, and applying the fix afterward would invalidate the tune. Fix the input, then tune it.

## Launch — CONFIRMED LIVE 2026-07-25 10:39:35 EDT

| Check | Result |
|---|---|
| `boxsprt.sh ps` before launch | empty (Batch-2 cert fully drained) |
| candidate on-box hash | `C9286041…78C9` == local build — full match |
| base on-box hash | `0A8F65B7…C721` == certified Batch-3 base engine — full match |
| topology | **1 `sprt` + 8 `ngn_t17` + 8 `ngn_t5` = 17 processes**, all StartTime 10:39:35, CPU accruing evenly (1.77–2.47s) |

Header echo as printed by the box:

```text
new=.\ngn_t17.exe  base=.\ngn_t5.exe
mode: real clock 10s+0.1s (concurrency 8) | openings: 5000 (x2 colors) | concurrency: 8
H0: elo<=-3.0   H1: elo>=3.0   (alpha=0.05 beta=0.05 -> LLR bounds [-2.94, 2.94])
adjudication: resign>=900cp/5p  draw<=10cp/10p>=80p
```

`H0: elo<=-3.0  H1: elo>=3.0` confirms the **per-change T-item bounds**, not the certification `[0,+6]`.

Post-launch hygiene: engine diff extracted to `output/t17.patch` (2 files: `engine/search.go`,
`engine/reentrancy_test.go`, 151 lines) and the working tree reverted to HEAD, per the no-speculative-commit
rule. Verified truly reverted by rebuilding and re-running nodecheck: **346662 / 149587 / 765656 = exact
baseline match**, so the local tree carries none of the candidate's behavior while the run is live.

Poll with `scripts/boxsprt.sh tail t17` / `scripts/boxsprt.sh ps`. Expected duration 1.5-8.5h depending on
where it lands; at the 961 g/hr box rate the 8000g cap is ~8h20m.

## RESULT — SHELVED 2026-07-25 (H0 ACCEPTED, clean statistical reject)

```text
Games: 4256   W-D-L: 1098-1982-1176   score: 49.1%
Elo(new - base): -6.4   95% CI [-17, +4]      LLR: -2.52   bounds [-2.94, 2.94]
Pentanomial [LL 119  LD 560  {LW,DD} 848  WD 482  WW 119] over 2128 pairs
Penta Elo: -6.4   95% CI [-14, +1]   pLLR -2.88   (THE decision stat)
Flag-outs (lost on time): new 0, base 0  of 4256 games
Adjudicated early: 2265 decisive, 1016 draw  of 4256
Verdict: H0 ACCEPTED: new is NOT better (<= -3 ELO)     4h25m46s   DONE_EXIT_0
```

### The printed pLLR is inside the bounds — full-envelope scan (3rd observation of the c8 drain)

The summary line reads -2.88, inside [-2.94, +2.94], yet the harness declared H0 ACCEPTED. Scanned all 4256
samples rather than assuming either number was wrong:

| Quantity | Value |
|---|---|
| First sample <= -2.94 | **G4248 (-2.95)** |
| Minimum pLLR | **-2.96 @G4253** |
| Samples at/below -2.94 | **8** |
| Samples at/above +2.94 | 0 |
| Post-mingames pLLR max | **-0.07 @G305** |
| Whole-run pLLR max | +0.15 @G128 (below the mingames-300 floor) |

The bound was genuinely crossed; the printed -2.88 is the **c8 drain** — in-flight games across 8 workers
complete after the stop signal and are folded into the recomputed summary. **This is now the third
observation and it has appeared in both directions** (T9a -2.87 after crossing -2.94; Batch-2 cert +2.88 after
crossing +2.94; T17 -2.88 after crossing -2.94), which confirms the drain is a symmetric artifact of the stop
protocol and not a directional bias. Verdicts on this box must be taken from the harness's own
`H0/H1 ACCEPTED` line plus an envelope scan, never from the printed final pLLR.

### Trajectory — never positive after mingames

| Games | pLLR | penta elo |
|---|---|---|
| 300 | -0.11 | -5.8 |
| 500 | -0.10 | -2.8 |
| 1000 | -0.27 | -2.4 |
| 2000 | -0.99 | -4.5 |
| 3000 | -1.61 | -5.0 |
| 4000 | -1.73 | -4.1 |
| 4256 | -2.88 | -6.4 |

Post-mingames elo envelope: **max -0.7, min -11.2 — never once positive.** Unlike T7 (which oscillated to
+1.92) there is no positive phase to argue about, and unlike a capped null this crossed a bound. This is the
cleanest negative of the campaign.

## What this means: the defect is LOAD-BEARING (the pre-registered asymmetry, now binding)

The manifest pre-registered, before any games were seen, that a shelve **does not vindicate the defect**.
That clause is now binding and must not be softened in hindsight.

Both defects are still real and still proven from code and from direct measurement:
slot 0 was never written (measured: 0 after a full depth-8 search), and `-INFINITY` was compared as a value
(measured: 1068 sentinel references in the endgame, 100% spuriously `improving = true`). **Nothing about
this result makes those correct.** What the result establishes is that **the four consumer knobs have been
tuned, by hand and by prior SPRTs, on top of the corrupted signal** — RFP margin, futility margin, the LMP
threshold row and the LMR reduction all currently carry values that are good *given* a mis-set `improving`.
Repairing the input alone breaks that adapted composition and costs ~6 Elo.

This is the same shape as **T1a** (`experiments/2026-07-02-t1a-softstop.md`), where removing the time
projection cost -7.8 and the finding was recorded as "the projection is LOAD-BEARING, not just flag
protection." Same lesson, different subsystem: a component that looks like a bug in isolation can be
carrying load placed on it by everything tuned around it.

**Consequence for T8, recorded now so it cannot be rediscovered later:** T8 SPSA tunes
`ReverseFutilityMargin`, `FutilityMargin`, `LMRFullDepth` and `LMRMoveThreshold` — four of the knobs
`improving` feeds — on the UNFIXED base. Its converged vector will therefore be **entangled with the
defect**, and T17 can never afterward be re-tested as an isolated change. T17's only valid reopen is
**bundled with a retune of its consumers** (fix + SPSA over the four margins as one experiment, explicitly an
interaction test per CLAUDE.md), never as a standalone re-run.

## ADDENDUM 2026-07-26 — the reopen is nearly FREE, and the current queue order would foreclose it

The consequence recorded above ("T8 on the unfixed base entangles the vector, and T17 can never afterward be
isolated") was written as an accepted cost. **It does not have to be paid.** The reopen this record demands —
*fix + retune the consumers as one interaction test* — is obtainable by changing **which base T8 runs on**,
and nothing else.

**Feasibility checked against the code, not assumed:**

- The four named consumers are all live SPSA dims — `ReverseFutilityMargin`, `FutilityMargin`, `LMRFullDepth`,
  `LMRMoveThreshold` (search.go:59-73) — and the LMP threshold row became a fifth, `LMPBase`, when T8b
  re-homed it. So **all five consumers of `improving` are already in the registry.**
- `cmd/spsa` tunes **every** entry of `engine.TunableSearchParams` (`cmd/spsa/main.go:83`); there is **no
  dim-subset flag**. A literal "SPSA over the four margins" would therefore need a `-dims` flag or a trimmed
  registry — a harness change.
- **But the subset is not needed.** Running the *existing* 14-dim T8 on the **fixed** base recalibrates all
  five consumers to a corrected `improving` signal simultaneously, which is exactly the interaction test this
  record asks for, at **identical cost to the T8 round already planned**.

**Proposal — T17b:** apply the T17 fix, then run T8 SPSA **on the fixed base** instead of the unfixed one.
Verdict instrument is unchanged (converged vector -> real-clock 10+0.1 c8 SPRT [-3,+3] vs unmodified HEAD;
never `plus_pct` or param movement).

| | T8 on UNFIXED base (current queue) | T8 on FIXED base (T17b) |
|---|---|---|
| Cost | ~25h | ~25h — **the same** |
| `improving` defect | cemented into the tuned vector permanently | repaired, and the tune adapts to the repair |
| T17 reopen | **foreclosed forever** | satisfied |
| Ambiguity on a negative | tune only | fix-or-tune (inherent to an interaction test, sanctioned by CLAUDE.md when explicit) |

This is why it matters *now*: it is cheap to decide before T8 runs and impossible to undo after. It also fits
the pattern the campaign's own results show — the keeps (T5 aspiration, T4 corrhist, T1b/T1e) replaced or
added whole mechanisms, while the misses (T7, T10b, T9a, T13, T16, T3a, T1c, T18b) adjusted single sites or
constants. "Repair a proven defect and recalibrate everything that consumes it" is a mechanism-level change;
"tune 14 constants on a known-corrupted input" is not.

**Not yet scheduled** — T3b is live and T20/T12/T19 are gated ahead of it. Recorded so the T8 launch decision
is made deliberately rather than by queue inertia.

## Ledger correction found while queuing T8 (stale claims, fixed on sight)

Verified against HEAD rather than trusting the queue text:

- **`DoubleExtMargin` is NOT an SPSA dim.** TODO said to prefer T8 for the T9a reopen "since `DoubleExtMargin`
  is already an SPSA dim [4,128]". T9a was reverted, so the constant does not exist in the engine at all
  (`grep DOUBLE_EXT engine/` is empty). The T9a-2 reopen path via T8 **does not exist** without first
  re-adding the code. T9a-2 is therefore a fresh hand-picked-constant SPRT if it is ever run, with all the
  cost that implies — which lowers its rank, it does not raise it.
- **LMP dims are NOT registered.** `lmpThreshold` (search.go:123) is a hardcoded `[2][9]int` table, not a
  tunable. "Include LMP-scale dims" is work-to-do, not a fact. Parameterizing it is a real shape decision
  (a base/scale formula), not an M1-style node-identical re-home, so it is split out as **T8b** rather than
  bolted onto round 1.

Live registry is **13 dims** (`engine/search.go:59-73`), all verified present: NullMoveR, NullMoveMinDepth,
LMRFullDepth, LMRMoveThreshold, FutilityMargin, FutilityMaxDepth, ReverseFutilityMargin, DeltaMargin,
SingularDepth, SingularMargin, ExtensionBudget, AspInit, AspMult.
