# 2026-07-03 T1b — soft-budget stability scaling (time-management, real-clock gate)

Pre-registered BEFORE launch (CLAUDE.md work-loop step 5). **PREP ONLY at time of writing** — the patch
is in the working tree UNCOMMITTED and NOT launched; diff backed up at `output/t1b.patch`. One isolated
behavior change, candidate vs its immediate base (current HEAD `2225202`), **real-clock ONLY** (time/TC
changes gate on real-clock games, never a proxy — CLAUDE.md instrument-match rule). The launching agent
fills the candidate binary SHA-256 at cross-compile and records the verdict.

```yaml
id: 2026-07-03-t1b-stability-scaling
date: 2026-07-03
change_class: search/eval heuristic (time-management) — real-clock-ONLY gate
hypothesis: >
  scaling the tournament soft target by the (already-tracked but never-read) decision-stability
  signal — EXTEND it while the root best move is still churning, SHRINK it once the choice has
  settled — redistributes the clock toward the moves that decide games and gains Elo. Keeps the
  next-iteration projection T1a proved load-bearing; symmetric, so it is neither the one-sided
  shrink (the 25%-clock bug, removed 2026-06-03) nor the uniform full-soft spend (T1a REJECT -7.8).
base_commit: 2225202 (current HEAD; engine byte-identical to 13560dc == ngn_base4 — git diff 13560dc..HEAD -- engine/ is empty)
candidate_commit: 2225202 + output/t1b.patch (engine/time.go + engine/time_test.go only)
base_binary: ngn_base4.exe sha256 e7f69ea727233a9c80704cba718fec97ce3aa65fc6426660252a1717e4ddc881 (on box, remote-verified; == HEAD engine — a clean-HEAD rebuild in this env reproduced this exact hash, so base and candidate differ only by output/t1b.patch)
candidate_binary: ngn_t1b.exe sha256 0fede586b4ae312f13d7752fe137a542b41cbdf6ee94eeac5988ebd1e63eadea (cross-compiled 2225202 + output/t1b.patch, GOOS=windows GOARCH=amd64 GOAMD64=v3, go1.26.2, no ldflags/trimpath; pushed to box + remote-verified 0FEDE586…)
harness_commit: sprt.exe sha256 30c33e0512725b7f552d8a1cf72ba6f1e0deb4dcb122c6b6ba8f6433c686a762 (M5-aware validated mill harness, HEAD 13560dc build)
mechanism: >
  engine/time.go shouldStopTournamentSearch soft-limit: soft := softTime * stabilitySoftFactor()
  (was the flat softTime). stabilitySoftFactor() = max(stabilitySoftMin, stabilitySoftMax -
  stabilitySoftSlope*stableIters) = max(0.85, 1.30 - 0.10*stableIters): 1.30 volatile (stableIters==0),
  1.00 crossover (stableIters==3), 0.85 floor (stableIters>=5). stableIters is maintained by the
  already-live ReportCompletedIteration (search.go:899, called with bestMoveChanged, attempts==1). The
  ×1 next-iteration projection, the hard ceiling (<=30% bank), and the emergency floor are UNTOUCHED.
node_identity: NOT node-identical (behavioral time-allocation change by design) — hence the real-clock gate.
regression_test: >
  engine/time_test.go TestTournamentSoftStop rewritten to the T1b semantics. RED->GREEN proven by
  reverting only the read line to `soft := tm.softTime`: with softTime=4000ms, lastIter=500ms, the
  extend assertion (stableIters=0 must NOT stop at elapsed 4200ms) and the shrink assertion
  (stableIters=6 must stop at elapsed 3000ms) both FAIL pre-patch (flat soft 4000 stops all at 4200,
  keeps all at 3000) and PASS post-patch. Full go test -short [-race] ./engine + -short ./... GREEN.
openings: canonical sprt_openings (5000 lines) sha256 974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222
```

## Hypothesis and donor prior

NGN spends ~70-80% of its clock vs Counter's ~98% (audit), and it is search-bound (depth pays). The
time-management package is the single biggest TM item on record: **Stash v26 "real time management
system (bestmove type / eval direction / bestmove stability)" = +31/+29 LTC** [S], with +24 [M]; the
best-move-**stability** sub-signal alone is +2.3 [E] and score-drop scaling +2.5 [E]. The **CounterGo
2994 peer** runs a three-regime `difficulty` manager (score drop >50cp → 2.0; best move changed →
max(1.5, d); stable → max(0.95, 0.9·d)) that recomputes the soft limit each iteration — the differential
the revamp doc identifies as a load-bearing part of the NGN→2994 gap.

**Calibration (expectation rule):** T1b wires ONLY the best-move-**stability** sub-signal (NGN's existing
`stableIters`), NOT the full Stash package — it has no eval-direction/score-drop regime yet (those need a
second signal = a second behavior change, deferred to a T1 follow-on). So the applicable prior is the
stability *component* (~+2-3 [E] class), not the +31 package headline. Expect a small gain if positive.

## Mechanism (the single change)

`engine/time.go`:

- **New `stabilitySoftFactor()` + constants** (after `ReportCompletedIteration`, its write-side
  counterpart): `f = max(stabilitySoftMin, stabilitySoftMax - stabilitySoftSlope*stableIters)` with
  `stabilitySoftMax=1.30`, `stabilitySoftMin=0.85`, `stabilitySoftSlope=0.10`. `stableIters >= 0`, so `f`
  starts at the max and only decreases; only the lower clamp is needed.
- **Soft-limit application** in `shouldStopTournamentSearch`: `soft := time.Duration(float64(tm.softTime)
  * tm.stabilitySoftFactor())` replaces `soft := tm.softTime`. The two following lines (the
  `lastIterationTime==0` first-iteration case and the `elapsed+lastIterationTime > soft` projection) are
  UNCHANGED — the projection is preserved, only its target scales.
- **Field docstring** for `stableIters` updated from shrink-only to the symmetric description (points at
  `stabilitySoftFactor`).

The write path was already complete and correct (found in code, not added): `stableIters` is declared,
reset per move in `SetTimeControl`, and incremented/reset by `ReportCompletedIteration` — which
search.go:899 calls once per completed depth with `(bestMoveChanged, attempts==1)` (best move unchanged
AND aspiration window held on the first try ⇒ increment; any change or re-search ⇒ reset to 0). **The
only thing missing was the READ** in the soft stop. So T1b is genuinely a one-line behavior change plus
the factor helper; no new signal, no new call site, no change to how `stableIters` is computed.

Effective spend (with the ×1 projection, `lastIter ≈ 0.5·elapsed_stop` in PVS ⇒ stop at
`elapsed ≈ 0.67·soft·factor`): volatile move ≈ 0.87·soft, neutral ≈ 0.67·soft (== today's baseline),
settled ≈ 0.57·soft. **Even the max-extend move stays below the full soft budget** — strictly short of
T1a's uniform ~1.0·soft, and the settled floor (0.85) is far milder than the ×0.5/×0.7 that caused the
25%-clock bug.

## Design decision — SYMMETRIC scaling (deviation from the literal task wording, flagged)

The prep task's design-intent paragraph paraphrased the change as "scale DOWN on stability; when unstable,
allow **up to** the current/full soft budget" — i.e. a factor capped at 1.0 (shrink-only). **I did NOT
implement that**, and the deviation is deliberate and evidence-backed:

1. **Shrink-only is a documented, measured NGN regression.** The one-sided "easy move" shrink (×0.5 at
   stableIters≥6, ×0.7 at ≥3) was REMOVED 2026-06-03 because it fired on nearly every quiet position and
   NGN played whole games at ~25% of its clock, never redeploying the bank (comment preserved in the old
   time.go; the pre-T1b `TestTournamentSoftStop` even had an assertion *guarding* that the soft budget no
   longer shrinks on stability). Re-adding a cap-at-1.0 shrink reintroduces exactly that mechanism.
2. **The cited authority mandates symmetry.** The revamp doc Part 3 T1b (which the task says to adapt
   from) states verbatim: "soft × stability factor, **symmetric (extend on instability, shrink on
   stability)**, hard cap unchanged. … The old one-sided shrink caused the 25%-clock bug; **symmetry is
   what makes it sound.**"
3. **The peer is extend-dominant.** CounterGo 2994's regime floors the shrink at 0.95 and pushes the
   extend to 1.5-2.0 — the redistribution comes from the *extend*, not the shrink.
4. **NGN's actual failure is UNDER-spending.** A cap-at-1.0 shrink can only *reduce* total clock on an
   engine that already under-spends and is depth-starved; the redeployment it relies on is precisely what
   2026-06-03 measured NOT to materialize in NGN's allocator (soft is capped by the 10-move floor).

Magnitude is held **conservative** because T1a proved uniform over-spend costs -7.8: ceiling 1.30 (vs
Ethereal's 2.5 / CounterGo's 2.0) keeps even the most volatile move below full soft; floor 0.85 (vs the
old 0.5/0.7) is a mild bank, not the 25%-bug shrink. The constants are **first-candidate, peer-anchored
judgment calls** — untunable without real-clock games (this is why the gate is real-clock, and why they
are named consts, trivially exposable as SPSA dims later, T8). If the SPRT regresses, the failure
signature picks the next candidate: a flat/small loss with clean clocks ⇒ trim the extend (1.30→1.15-1.20);
a clock-usage collapse ⇒ raise the floor toward 0.95 / push the crossover later.

## Decision rule (predeclared, real-clock ONLY, standard batch-certification)

Real-clock 10+0.1 c8 SPRT vs `ngn_base4` (= clean HEAD engine), elo0=-3 elo1=+3, alpha=beta=0.05 (pLLR
bounds ±2.94), **adjudication ON** (validated non-distorting by the 2026-07-03 M5-enable A/A), on the
A/A-validated c8 config, maxgames 8000 mingames 300. Same shape as T4 (standard batch-cert, NOT the strict
nonpawn rule — T1b is a fresh lever, and it does not overturn T1a: T1a dropped the projection, T1b keeps
it and scales the target, a different mechanism):

- **pLLR ≥ +2.94** (H1 accept) → KEEP outright.
- **pLLR ≤ −2.94** (H0 accept) → REJECT; the naive stability-scaling lever is closed and the failure
  signature (above) informs whether a re-scoped T1b candidate is worth one retry.
- **Capped-positive** (8000g cap, point est ≥ +1, pLLR > 0, no regression signal) → PROVISIONAL keep,
  next slot in `experiments/batch-ledger.md` (batch-certification path, CLAUDE.md).
- **Capped-nonpositive** (point < +1 or pLLR ≤ 0 at cap) → SHELVE.
- **Flag-outs above the A/A baseline (0), i.e. any candidate flag-out** → HALT + investigate before
  verdicting (a stability-driven over-extend would show here first).

## Harness / command / machine

Adjudication flags are the M5-validated standard set (`-resignscore 900 -resignplies 5 -drawscore 10
-drawplies 10 -drawminplies 80`; M5-enable A/A PASS 2026-07-03: base-vs-base penta -1.1 [-13,+11], 0/0
flag-outs, 77.6% adjudicated, 961 vs 932 g/hr, `experiments/2026-07-03-m5-adjudication-aa.md`).

```yaml
command: sprt.exe -new .\ngn_t1b.exe -base .\ngn_base4.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 8000 -mingames 300 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
machine: AMD Ryzen 7 9800X3D 8c/16t, native Windows (LAN box 192.168.4.108)
go_version: go1.26.2 (local cross-compile)
goarch_goamd64: windows/amd64 GOAMD64=v3
tc: 10+0.1 (seconds; bullet)
concurrency: 8
adjudication: ON (-resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80)
maxgames: 8000
mingames: 300
aa_preflight: >
  the 2026-07-03 M5-enable A/A (m5aa_out.txt, ngn_base4 vs ngn_base4 at THIS exact TC/concurrency/machine
  with adjudication ON) — penta -1.1 [-13,+11], 0/0 flag-outs, DONE_EXIT_0. Same engine binary as this
  run's base and the identical adjudication config, so it is a valid A/A preflight; a fresh A/A is not
  required (config unchanged). If the launcher prefers a candidate-vs-candidate A/A it may run one, but
  policy is satisfied by the M5 A/A.
openings_sha256: 974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222
```

Command matches the KEPT T4/T4-nonpawn batches exactly except the run name, the two binary names, and the
5 (now-standard) adjudication flags. `.\` on exe paths is the Go ErrDot dodge; the openings path is bare.
Expected duration ~2.5-4h at c8 depending on where the SPRT crosses.

## Status

**LAUNCHED (2026-07-03 14:40:06 EDT) — real-clock c8 SPRT live on the box, awaiting verdict.** Patch
reviewed + APPROVED by team-lead (symmetric confirmed correct vs the revamp doc); still UNCOMMITTED
(`output/t1b.patch`, engine/time.go + engine/time_test.go, +83/-34; NOT committed until its verdict per
the speculative-change rule). Full suite green (short + race ./engine, short ./...); red→green verified.
This run re-invokes / is watched for completion to apply the decision rule and append the Verdict section
(verbatim RESULT tail + applied rule), same format as T4.

## Launch (2026-07-03)

Launched **14:40:06 EDT** (box time 2:40:08 PM) via `scripts/boxsprt.sh launch t1b` on the 9800X3D LAN box
(192.168.4.108, native Windows) → `t1b_out.txt`.

```yaml
launched: 2026-07-03 14:40:06 EDT (Mac) / 2:40:08 PM box time
output_file: t1b_out.txt (box C:\Users\ehrli\ngn\sprt\)
command: sprt.exe -new .\ngn_t1b.exe -base .\ngn_base4.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 8000 -mingames 300 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
harness_commit: sprt.exe sha256 30c33e0512725b7f552d8a1cf72ba6f1e0deb4dcb122c6b6ba8f6433c686a762 (M5-aware validated mill harness, remote-verified) + scripts/boxsprt.sh (unchanged during the run)
base_binary_sha256: e7f69ea727233a9c80704cba718fec97ce3aa65fc6426660252a1717e4ddc881 (ngn_base4.exe, remote-verified; clean-HEAD rebuild in-env reproduced this exact hash)
candidate_binary_sha256: 0fede586b4ae312f13d7752fe137a542b41cbdf6ee94eeac5988ebd1e63eadea (ngn_t1b.exe, remote-verified 0FEDE586…)
openings_sha256: 974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222 (remote-verified)
adjudication: ON (resign>=900cp/5p, draw<=10cp/10p past move 40 [80 plies])
eta: ~2.5-4h at c8 (T4 crossed early at 2h31m/2325g; a full 8000g cap is ~7h)
```

**Liftoff evidence (clean):**
- Remote SHA-256s CONFIRMED == local before launch: ngn_t1b.exe 0FEDE586…, ngn_base4.exe E7F69EA7…,
  sprt.exe 30C33E05…, sprt_openings.txt 974E4B5A…. Box `ps` was empty pre-launch (idle).
- `ps` post-launch: **1 sprt + 8 ngn_base4 + 8 ngn_t1b** (17 procs, c8×2+harness), all StartTime 2:40:08 PM,
  CPU accumulating (~10 CPU each = actively searching).
- Header parsed correctly: `mode: real clock 10s+0.1s (concurrency 8) | openings: 5000 (x2 colors)`;
  `H0: elo<=-3.0  H1: elo>=3.0  (alpha=0.05 beta=0.05 -> LLR bounds [-2.94, 2.94])`; adjudication line
  `resign>=900cp/5p  draw<=10cp/10p>=80p`.
- First 8 games flowing, **adjudication firing BOTH kinds** (adj-win G1/G4-G7, adj-draw G2/G3, one
  max-moves G8) — **0 flag-outs / 0 time-losses / 0 illegal / 0 no-move**. Early Elo (−43.7 at G8,
  2W-3D-3L) is small-sample noise, not a signal.

Poll: `scripts/boxsprt.sh tail t1b` / `scripts/boxsprt.sh ps`. This run applies the predeclared decision
rule on completion.

## Verdict (2026-07-03) — KEEP (H1 ACCEPTED)

The real-clock c8 SPRT ran to completion (DONE_EXIT_0, wall 2h18m5s) and crossed the H1 pLLR bound.
Verbatim RESULT (`t1b_out.txt` tail):

```
=== RESULT (2h18m5s) ===
Games: 2233   W-D-L: 636-1046-551   score: 51.9%
Elo(new - base): +13.2   95% CI [-1, +28]
LLR: +2.77   bounds [-2.94, 2.94]
Pentanomial [LL 54  LD 260  {LW,DD} 447  WD 258  WW 97] over 1116 pairs
Penta Elo: +13.1   95% CI [+3, +23]   pLLR +2.90  (THE decision stat; trinomial above is secondary)
Flag-outs (lost on time): new 0, base 0  of 2233 games  (goal: 0)
Adjudicated early: 1180 decisive, 570 draw  of 2233 games
Verdict: H1 ACCEPTED: new is stronger (>= 3 ELO)
DONE_EXIT_0
```

```yaml
harness_commit: sprt.exe sha256 30c33e0512725b7f552d8a1cf72ba6f1e0deb4dcb122c6b6ba8f6433c686a762 (M5-aware) + scripts/boxsprt.sh (unchanged during the run)
machine: AMD Ryzen 7 9800X3D 8c/16t, native Windows (LAN box 192.168.4.108)
tc: 10+0.1 (seconds; bullet)
concurrency: 8
base_binary_sha256: e7f69ea727233a9c80704cba718fec97ce3aa65fc6426660252a1717e4ddc881 (ngn_base4.exe = HEAD engine)
candidate_binary_sha256: 0fede586b4ae312f13d7752fe137a542b41cbdf6ee94eeac5988ebd1e63eadea (ngn_t1b.exe)
openings_sha256: 974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222
adjudication: ON (-resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80)
games_or_pairs: 2233 games / 1116 pairs (stopped at the H1 bound, before the 8000 cap)
result: penta [LL 54 LD 260 {LW,DD} 447 WD 258 WW 97]; penta Elo +13.1 [+3,+23] pLLR +2.90; trinomial +13.2 [-1,+28] LLR +2.77; W-D-L 636-1046-551 (51.9%); duration 2h18m5s
flags_errors: flag-outs new 0 / base 0 of 2233 (goal 0); 0 crash / 0 illegal / 0 no-move; DONE_EXIT_0
verdict: KEEP (H1 ACCEPTED: new is stronger, >= 3 Elo; pLLR crossed the +2.94 bound at G2225)
next_action: KEPT into main (single commit, engine + test + records); T1 follow-on = T1c (score-drop / eval-direction regimes); next behavior change = T4c (minor-piece corrhist); next box action = M6 relaunch
```

**Crossing + drain note.** The SPRT stop crossed at **G2225 (pLLR +2.97)** and held ≥ the +2.94 H1 bound
for six consecutive lines (G2225–G2230, pLLR +2.97→+2.98); the ~6 in-flight games then drained
(concurrency 8), so the FINAL printed pLLR is +2.90 rather than the crossing value — the same pattern as
T4 (+2.98→+2.80) and T4-nonpawn (+2.97→+2.80). The penta point estimate is a stable **+13.1 to +13.6**
across the crossing and drain. The negative bound (−2.94) was never touched anywhere in the log.

**Integrity checks (independently verified on the fetched log).** Flag-outs 0/0 across all 2233 games —
the key check for a time-management change: the extend side did NOT cause time trouble (the hard ceiling +
emergency floor were untouched by design). 0 crash / 0 illegal / 0 no-move lines; adjudication fired 1180
decisive + 570 draw (78%), base-vs-candidate balanced (636W-551L). Post-run box `ps` empty (no leaked
workers). Working tree byte-matches `output/t1b.patch` (sha256
`08621f80582fcdedf37663849cbbd5be68a860928ecfa26f820465dcc6e90134`).

**Decision (predeclared rule, verbatim).** pLLR ≥ +2.94 → KEEP outright; ≤ −2.94 → REJECT; capped-positive
→ PROVISIONAL; capped-nonpositive → SHELVE; flag-out spike → HALT. **Outcome: KEEP outright — the +2.94
bound was crossed (G2225). NOT a batch provisional.** T1b is the second design-lever (after the T4 family)
to land as a full H1 accept, and the first time-management Elo of the campaign: the symmetric
stability-scaling redistributes the clock toward volatile (hard) moves — exactly the lever T1a's uniform
overspend failure pointed to.

**Learn / lane note.** T1b confirms the revamp-doc T1b spec (symmetric, not the removed one-sided shrink)
and the CounterGo three-regime direction: wiring only the bestmove-STABILITY sub-signal already banks
+13.1. The remaining regimes (score-drop → max-think, eval-direction) are the queued T1c follow-on — a
second signal (score tracking through ReportCompletedIteration), expected to add on top per the Stash-v26
+31 package.
