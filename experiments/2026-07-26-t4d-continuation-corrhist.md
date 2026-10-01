# 2026-07-26 T4d — continuation correction history (fourth corrector) — PRE-STAGED, NOT LAUNCHED

**STATUS: built, gated, staged on the box with the on-box hash verified. NOT LAUNCHED — T20 owns the box
and its verdict comes first.** Launch is one command once T20 resolves. Engine tree is REVERTED and clean;
the change lives only in `output/t4d-continuation-corrhist.patch`
(sha256 `55209c2d4280291ce91855115d041b2e67e1bd7a1e098ea6eaf32245c8731b0f`, 344 lines).

Pre-flight and full motivation: `experiments/2026-07-26-t4d-continuation-corrhist-preflight.md`.

## What it is

The **fourth** correction-history table, and the first keyed on the **move sequence** rather than on
position features. The three shipped correctors (pawn +9.1, non-pawn +6.4, minor +3.7 — the family is
**3-for-3, the only family in the campaign that has never missed**) all ask *what does this position look
like*. This one asks *what move got us here*, keyed `[stm][prevPiece][prevTo]` = 2x13x64 = **1664 slots**,
roughly 20x fewer than the 16384-slot position-feature tables.

It rides the identical T4 plumbing that made the family work — applied at the qsearch stand-pat (via
`correctedStandPat`) and at the interior static-eval site, learned from the pre-S7 `corrStaticEval`, same
gravity EMA update, same `6245/131072` weight, same `corrHistLimit`.

## Pre-flight evidence (measured, zero box cost)

The post-correction residual `bestScore - corrStaticEval` was accumulated under the continuation key, a
same-cardinality random control, and the three existing corrector keys — 1145273 observations.

| key | R² | buckets | chance floor | ratio |
|---|---|---|---|---|
| pawn (KEPT +9.1) | 0.05767 | 12787 | 0.01116 | 5.2x |
| non-pawn (KEPT +6.4) | 0.07200 | 16382 | 0.01430 | 5.0x |
| minor (KEPT +3.7) | 0.06689 | 12832 | 0.01120 | 6.0x |
| **continuation (this candidate)** | **0.07122** | **707** | 0.00062 | **115.5x** |

Comparable explained variance to the three correctors that worked, from **20x fewer parameters**. The
signal **strengthens** under clamping (33x raw → 100x at `corrHistLimit` → 121x at ±256), so it is not an
outlier artifact. The control's measured floor (0.00071) matches the analytic prediction (B-1)/N = 0.00073,
validating the comparison.

## A REAL DEFECT WAS FOUND AND FIXED DURING THE BUILD — record it

The obvious implementation reads the previous move from `info.MoveStack[ply-1]`. **That is wrong inside
qsearch.** `MoveStack` is written only by the main search (search.go:803/1132/1496/1790/1880); **qsearch
never updates it**, so at any qsearch node below the entry point `MoveStack[ply-1]` holds stale data from an
unrelated earlier path. Since qsearch is ~42% of middlegame nodes and the stand-pat is evaluated at nearly
every one of them, a large fraction of corrections would have been keyed on deterministic garbage.

**Fix:** `prevMove` is threaded explicitly as a parameter through `quiescence` / `quiescenceWithDepth`, and
both recursion sites pass the capture just made. The main search passes its own `prevCorrMove`; the probcut
qsearch call passes the move it just made; texel and the qsearch tests pass `EmptyMove`.

## Gates — ALL PASS

- **Behavioral delta (>= ~1%)**: fixed-depth nodecheck vs the HEAD lock (346662 / 149587 / 765656) =
  **-16.7% / +6.1% / -33.4%** (288774 / 158777 / 510107). Two of three positions **shrink** substantially,
  which is the expected direction for a better-corrected eval. Verified against a **fresh rebuild** — the
  reverted tree reproduces the baselines exactly (`nodecheck.sh` does NOT rebuild; a stale binary silently
  reports the wrong tree, and that trap fired once during this build and was caught).
- `go test -short ./engine` green; `go test -short -race ./engine` green; `go test -short ./...` green.
- **Regression test `TestContinuationCorrectionWired`** (appended to `engine/correction_history_test.go`):
  covers learn/apply, EmptyMove inertness, in-check and capture-best-move gates, fail-high/fail-low signs,
  cross-key isolation, and **the ClearHistoryTable leak guard**. **RED->GREEN proven**: removing only the
  clear loop makes it fail with "ClearHistoryTable did not zero continuationCorrectionHistory (cross-game
  leak)". That assertion is not decoration — the 2026-06-23 aux-corrhist run was INVALIDATED mid-flight by
  exactly this omission.

```yaml
id: 2026-07-26-t4d-continuation-corrhist
date: 2026-07-26
change_class: search/eval heuristic (eval correction). Requires a completed predeclared paired game test.
hypothesis: >
  A fourth correction-history table keyed on the previous move (piece x destination) rather than on position
  features captures recurring static-eval errors that the pawn/non-pawn/minor keys are structurally blind to.
  Measured pre-flight: the continuation key explains 7.1% of the post-correction residual variance against a
  0.06% chance floor (115x), comparable to what the three kept correctors leave unabsorbed but from 20x fewer
  buckets — so it warms up far faster, which is what matters at 10+0.1.
patch: output/t4d-continuation-corrhist.patch sha256 55209c2d4280291ce91855115d041b2e67e1bd7a1e098ea6eaf32245c8731b0f
candidate: ngn_t4d.exe sha256 887c8946e5a2c2bbb84f6e697f5c5d78af5b5654d35eda2fa6eb94f47c60ffc2
  PRE-STAGED on the box 2026-07-26; on-box hash verified EQUAL to the local build before launch.
base: ngn_t5.exe sha256 0a8f65b75e59c376052f83b14787b0af2eba65552cedfee6f70ce576891ca721
  CONDITIONAL — correct only if T20 SHELVES. If T20 is KEPT, T4d must be rebuilt and re-gated against the
  T20 tree; do NOT run against a superseded base and pool.
harness: sprt.exe sha256 30c33e0512725b7f552d8a1cf72ba6f1e0deb4dcb122c6b6ba8f6433c686a762 (M5-validated, untouched)
command: -new .\ngn_t4d.exe -base .\ngn_t5.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 8000 -mingames 300 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
  NOTE: boxsprt.sh launch PREPENDS sprt.exe itself — pass the line above WITHOUT a leading `sprt.exe`
  (the T20 launch gotcha; passing it verbatim silently drops every flag).
machine: AMD Ryzen 7 9800X3D 8c/16t, native Windows (LAN box 192.168.4.108)
tc: 10+0.1 (seconds)
concurrency: 8
openings: canonical sprt_openings (5000 lines) sha256 974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222
adjudication: STANDARD-ON (M5-validated flags)
maxgames: 8000
expected_duration: ~8h30m at ~960 g/hr if it drains to cap
```

## Predeclared decision rule (standard per-change T-item gate)

- pLLR **>= +2.94** -> **KEEP** as a Batch-3 keep; re-lock nodecheck to the T4d numbers; re-run ebfprobe +
  the fmc/ttlist table per the re-measure cadence.
- pLLR **<= -2.94** -> **SHELVE** and revert.
- Capped with point est **>= +1 and pLLR > 0** -> **PROVISIONAL** keep into Batch 3.
- Capped **nonpositive** -> SHELVE.

Verdict is taken from the harness's own `H0/H1 ACCEPTED` line plus a full-envelope scan of the printed
samples — **never the printed final pLLR alone** (c8-drain artifact, observed 3x, symmetric).

**REOPEN CONDITION (predeclared).** A negative T4d does not close the continuation-corrector idea, because
the key is one choice among several: `[prevPiece][prevTo]` is the coarsest form. Re-derived variants, in
order: (1) key on `prev2` (our own previous move) instead of the opponent's; (2) combine both plies; (3) a
separate weight from the shared `6245/131072` (the other three were never individually weight-tuned). Run a
re-derived variant only if the loss is small; a large negative implicates applying a move-keyed correction at
the qsearch stand-pat at all, in which case test the interior-only application before abandoning the lane.

## Honest expectation

**+2 to +4.** The family's series is strictly diminishing (+9.1 -> +6.4 -> +3.7) and there is no reason a
fourth corrector breaks the trend; the parameter-efficiency result argues for the top of that range, not
beyond it. **A +2-4 change most likely CAPS rather than crossing +2.94** — T4c at +3.7 only crossed at
G7840 — so the probable outcome is a **capped-positive PROVISIONAL keep**, which is exactly what the
batch-certification path exists for. **Batch 3 has 0 rows and is due 2026-08-08.**

Discounts, stated plainly: R² on the eval residual is not Elo; the pre-flight comparison flatters
continuation (its signal is wholly unabsorbed while the existing keys' figures are post-correction);
the pre-flight ran 4 games at fixed depth on a local machine; and this is the fourth table summed into the
same static eval, so some of its signal may be redundant with corrections the other three already apply.

## LAUNCH READINESS — verified 2026-07-26 22:35 EDT, while T20 was still live

- **Patch reproduces the staged binary byte-for-byte.** `git apply output/t4d-continuation-corrhist.patch`
  onto clean HEAD + `make build` yields sha256 `887c8946e5a2c2bbb84f6e697f5c5d78af5b5654d35eda2fa6eb94f47c60ffc2`,
  identical to what is staged on the box. The build is deterministic and the run record is reproducible.
- **All three on-box binaries verified present and correct**: `ngn_t4d.exe` 887C8946…, `ngn_t5.exe`
  0A8F65B7…, `sprt.exe` 30C33E05… (M5-validated harness, untouched).
- **Launch argument string pre-checked**: first token is `-new`, NOT `sprt.exe` — `boxsprt.sh launch`
  prepends `$BIN` itself, and passing the leading `sprt.exe` silently drops every flag (the T20 gotcha).
- Tree reverted to clean HEAD after verification; nodecheck baselines restore exactly.

**Launch is blocked on exactly one thing: T20's verdict.** If T20 SHELVES, launch as written. If T20 KEEPS,
`ngn_t5` is a superseded base — rebuild T4d on the T20 tree, re-gate, re-stage, and do NOT pool.

## PER-NODE SPEED RISK — identified from the speed-lane profile, MEASURED, and it comes out FAVOURABLE

**The risk:** the profile in `project_speed_lane` puts **qsearch at 47% of CPU**, and T4d adds a FOURTH
corrector evaluated at every qsearch stand-pat via `correctedStandPat`. A per-node speed cost there could
offset the eval gain — and at 10+0.1 the thing that actually matters is **time-to-depth**, not node count.
This gate was not in the original build checklist; it should be for any change touching the stand-pat path.

**Measured** (fixed depth, 3 reps, rook-eg-R4P d16 — the qsearch-heaviest position in the probe set):

| | nodes | ms (3 reps) | mean ms | NPS |
|---|---|---|---|---|
| base | 266201 | 108 / 93 / 112 | 104.3 | ~2.55 M |
| **T4d** | **188226** | 88 / 92 / 88 | **89.3** | ~2.11 M |

Node counts are **exactly deterministic across every rep** (266201 and 188226, no variation), confirming the
node-count ruler and isolating all variance to wall time.

- **Per-node speed: -17% in the endgame** — the cost is real and is where predicted (qsearch stand-pat).
- **Tree size: -29%** — the better-corrected eval prunes more.
- **NET: 14.4% FASTER to depth 16.** The tree shrink more than pays for the per-node cost.

kiwipete d12 agrees in direction: 346662n/190ms base vs 288774n/154ms T4d (+2.8% NPS, -17% nodes, faster to
depth). **Both positions reach the same depth in less wall time**, which is the correct real-clock read.

This does not predict the SPRT result — it only retires the specific risk that a fourth stand-pat corrector
would pay for its eval gain in lost speed. It does not.

## PRE-REGISTERED READING OF THE OUTCOME — written 2026-07-27 06:25 EDT at G2721, pLLR -2.06, OUTCOME UNKNOWN

T4d is tracking negative (pLLR -2.06, ~70% of the way to the H0 bound). Recording how to read a rejection
**before** it happens, so neither the excuse nor the escalation can be chosen to suit the result.

### If T4d REJECTS, these two things follow and must be accepted

**1. The residual-R² pre-flight is DEMOTED to rejection-only, exactly as ebfprobe was on 2026-07-27.**
T4d was promoted on a measured 115x-over-chance signal — the strongest pre-flight of the session — and if
that converts to a clean H0 reject then **explained-variance-on-the-residual does not predict Elo**. It would
then join depth-at-fixed-nodes as a filter that can kill a candidate but must never promote one. That is a
real cost: it is the instrument the whole corrector-key ranking rests on, and its *ordering* would survive
only as a weak prior, not as evidence. **Do not special-plead** ("the key was fine, the weight was wrong") —
that is the move the revamp doc made for T6 with the int-trunc story, which measurement later falsified.

**2. A cleaner pattern would emerge, and it FAVOURS T4e rather than indicting it.** The three correctors that
worked are **all position-feature keys** (pawn structure, non-pawn occupancy, minor occupancy). T4d is the
campaign's **first move-sequence key**. A T4d rejection therefore reads as *"position-feature correctors work,
move-sequence correctors do not"* — a statement about the **key class**, not about the corrhist family.
**T4e (material signature) is a position-feature key**, in the class that is 3-for-3, and carries 4.4x
T4d's excess-over-chance signal (0.285 vs 0.065). So T4e is NOT withdrawn by a T4d rejection.

### What a rejection would NOT license

- It would **not** promote T4d's reopen variants. The key ranking already measured `cont2` (prev2) at 110x
  *below* the built `cont1` at 124x, and `cont12` at 11.6x. Both are move-sequence keys — the class the
  rejection would indict. **Do not run them.**
- It would **not** make the corrhist family closed. 3-for-4 with the miss in a distinct key class is not a
  closed lane.

### If T4d instead KEEPS or caps positive

Then the residual-R² instrument survives as a promotion signal (one confirmation, not proof), the
move-sequence key class is validated, and T4e proceeds on an even stronger footing. The `>= +1` point-estimate
floor for a PROVISIONAL keep still applies — see the T20 rule-gap closure, which is now campaign-wide policy.

## RESULT — SHELVED 2026-08-01 (run completed 2026-07-27 07:05, read 5 days later)

**Clean statistical reject. H0 ACCEPTED.**

```text
=== RESULT (3h34m7s) ===
Games: 3432   W-D-L: 874-1603-955   score: 48.8%
Elo(new - base): -8.2   95% CI [-20, +3]
LLR: -2.63   bounds [-2.94, 2.94]
Pentanomial [LL 101  LD 451  {LW,DD} 683  WD 390  WW 91] over 1716 pairs
Penta Elo: -8.2   95% CI [-16, -0]   pLLR -2.99  (THE decision stat)
Flag-outs (lost on time): new 0, base 0  of 3432 games
Adjudicated early: 1822 decisive, 803 draw  of 3432 games
Verdict: H0 ACCEPTED: new is NOT better (<= -3 ELO)
DONE_EXIT_0
```

- Penta **-8.2 [-16, -0]**, pLLR **min -2.99**, first crossing of -2.94 at **G3424**, **10 samples at/below
  the bound**. **No c8-drain ambiguity: printed pLLR == envelope min (-2.99)**, so the drain deepened in the
  same direction — printed value and envelope agree, and the harness's own line is `H0 ACCEPTED`.
- Post-mingames pLLR envelope **[-2.99, +0.53]**; elo max +15.7 (early, near the mingames-300 floor). Unlike
  T17/T18b there *was* a brief positive phase, but it never approached the +2.94 bound.
- 0/0 flag-outs / 3432g real-clock 10+0.1 c8 adjudicated, 3h34m7s, ~962 g/hr (on the 961 baseline).
- Engine tree was never dirty; the change lives only in `output/t4d-continuation-corrhist.patch`. On-box
  `ngn_t4d.exe` `887c8946…` retained.

### The pre-registered reading applies verbatim — both consequences are ACCEPTED

**1. The residual-R² pre-flight is DEMOTED to REJECTION-ONLY.** T4d carried the strongest pre-flight signal
of the session (115x over chance, comparable explained variance to all three keeps from 20x fewer
parameters) and it converted to a clean H0 reject. **Explained-variance-on-the-residual does not predict
Elo.** It now sits beside ebfprobe: it may kill a candidate, it may never promote one. Its *ordering* of the
9 keys survives only as a weak prior, not as evidence. No special pleading was entered — the "key was fine,
the weight was wrong" move was pre-emptively barred and is not being made.

**2. The indicted unit is the KEY CLASS, not the corrhist family.** The three keeps (pawn +9.1, non-pawn
+6.4, minor +3.7) are all **position-feature** keys; T4d was the campaign's first **move-sequence** key and
is its only miss. Family is **3-for-4 with the miss isolated to a distinct key class**, which is not a lane
closure. **T4e (material signature) is a position-feature key and is NOT withdrawn.**

### What this result does NOT license (pre-registered, unchanged)

- **`cont2` and `cont12` stay unrun.** Both are move-sequence keys — the indicted class — and both ranked
  *below* the variant that just lost.
- **The corrhist lane is NOT closed.**
- **T4e may not be promoted on its R² (0.290).** That number is now rejection-only evidence. T4e's rank
  rests on the position-feature key class being 3-for-3, not on its residual fit.

### Leading explanation (recorded, not load-bearing)

The 1664-slot move-sequence key aliases far more aggressively than the position-feature tables: a given
`[stm][prevPiece][prevTo]` bucket pools corrections from structurally unrelated positions that happen to
share the last move. The position-feature keys pool positions that are genuinely similar in the feature the
eval is wrong about. High explained variance on the *residual* is consistent with both, which is precisely
why the pre-flight failed to discriminate and why it is now rejection-only.

### Process cost — HANDOFF STALL #4

Run finished **2026-07-27 07:05:28**; read **2026-08-01 15:29**. **~128 box-hours idle** (~5.3 days), the
second-worst stall of the campaign after stall #3's 4.5 days. The M7 analysis stands: no in-session waiting
mechanism can cover an 8-hour run, and the durable fix is an out-of-agent notifier.
