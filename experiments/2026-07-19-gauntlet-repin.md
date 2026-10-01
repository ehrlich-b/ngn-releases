# 2026-07-19 gauntlet re-pin — absolute CCRL measure of post-T5-keep HEAD (+ transfer readout)

**Instrument-replication record for the milestone gauntlet** triggered by the CADENCE rule (~+30.5 claimed
self-play since the 2026-07-08 re-pin: T4c +3.7, T1e +6.6, T5 +20.2 — over the +15-20 / ~2-week milestone
threshold). **This is a MEASUREMENT, not a keep/reject gate — no change is kept or reverted off this number
alone** (the Batch-2 certification self-play SPRT `[0,+6]` vs `c514a2b`, due ≤2026-08-01, is the transfer
*certification* instrument; this pin is absolute-sanity + point-signal only). **Sizing: S1** (400g single
HEAD pin, exact 2689/2666-pin replica, on the box) — same sizing decision as the Jul-4 pin; the delta-CI
floor math (below) is unchanged. Result/verdict fields fill at completion.

```yaml
id:                        2026-07-19-gauntlet-repin
date:                      2026-07-19
change_class:              measurement (absolute re-pin + transfer readout; NOT a keep/reject test)
hypothesis:                where does post-T5-keep HEAD sit on the CCRL scale vs the 2026-07-08 pin (2689 [2649,2729]), and how much of the +30.5 self-play from the three keeps since the 2689-pin engine 35c0c0a (T4c +3.7, T1e +6.6, T5 +20.2) transferred?
base_commit:               35c0c0a (conceptual transfer "before" = the engine at the 2689 pin; S1 does not play it — the 2689 pin at 35c0c0a is the absolute reference. A fresh A/B was option S3, not chosen.)
candidate_commit:          5b93966edca30c0a9f3b4cd60baea06fd1787f6a (T5 KEPT; functional HEAD). Repo HEAD = 9eeb2ac29402f365406d73c1a64acf80eac27362 (nodecheck-baseline re-lock only, no functional change — same engine).
base_binary_sha256:        n/a (S1 single-pin: HEAD vs fixed anchors; no fresh base binary — transfer is point-estimate vs the 2689 pin)
candidate_binary_sha256:   0a8f65b75e59c376052f83b14787b0af2eba65552cedfee6f70ce576891ca721  # ngn_t5.exe, GOOS=windows GOARCH=amd64 GOAMD64=v3, standard convention (NO ldflags/trimpath — matches the Jun-28/Jul-04 pins + every box binary). Hash-back verified: a clean build of f337819+output/t5.patch (== commit 5b93966) reproduced this exact hash and the 346662/149587/765656 nodecheck. Staged into the repin dir from the box sprt dir (where it played the T5 SPRT); repin-dir copy re-hashed 0a8f65b7… intact.
harness_commit:            cmd/gauntlet @ 35c0c0a — REUSED BYTE-IDENTICAL from the Jul-04 pin. gauntlet.exe sha256 123e6cee6d4b728eb4373d3149a8382b4f2b53f4de48d3a95da4fbc81d7580d3. `git log 35c0c0a..HEAD -- cmd/gauntlet` and `-- internal/uci` are BOTH empty, so the measurement tool is literally the same binary that produced the 2689 pin — the strongest instrument-replication guarantee (no rebuild).
command:                   gauntlet.exe -ngn .\ngn_t5.exe -anchors ratings.json -tc 120+1 -games 80 -concurrency 8 -lowpower=false -openings sprt_openings.txt -only v6.1.0,v7.2.0,v7.4.0,v8.0.0,counter-3.8 -tally-out repin_0719_tally.json -pgn repin_0719.pgn -no-record
machine:                   LAN 9800X3D (AMD Zen5 V-Cache, 8c/16t), native Windows — the SAME machine class as the 2666 + 2689 pins (comparability requirement)
go_version:                go1.26.2
goarch_goamd64:            windows/amd64 v3
tc:                        120+1  (real clock, SECONDS — CCRL Blitz proxy; same as the 2666 + 2689 pins)
concurrency:               8      (16 procs on 16 threads, oversubscribed; both sides equally slowed -> relative rating preserved — same as the prior pins)
openings:                  sprt_openings (corpus_manifest.md)
openings_sha256:           974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222
aa_preflight:              n/a for a gauntlet (NGN-vs-anchors, not self-play); pre-launch `-probe` recorded below. As on Jul-04, the probe is a movetime/stop probe; this real-clock `-tc` run does not skip older Blunder anchors on probe non-compliance, and clock/forfeit behavior is measured in the run.
decision_rule:             MEASUREMENT — report pooled CCRL rating + 95% CI + per-anchor crosstable; delta vs the 2689 [2649,2729] pin = transfer readout (a SIGNAL: the delta-CI is FLOORED at ±39.5 by the fixed 2689 pin's variance, see Sizing). NOT a keep/reject gate. ANOMALY = a pin REGRESSION (HEAD pooled point below the 2689 point, or the CI excluding 2689 on the low side) — that would contradict the +30.5 self-play and triggers INVESTIGATION, never an auto-revert. Forfeit accounting: report anchor-side vs NGN-side flag-outs separately (Jul-04 had 7/400, all anchor-side).
games_or_pairs:            400 (5 anchors x 80; S1)
result:                    DONE_EXIT_0 — POOLED NGN 2704, 95% CI [2665, 2743] (5 anchors × 80g = 400g). Crosstable + per-anchor perf below.
flags_errors:              4/400 time-forfeits (3 anchor-side: games 39/46/63 vs v6.1.0, NGN won; 1 NGN-SIDE: game 193 vs v7.4.0, NGN lost on time — FLAGGED, see Results). 0 panics / 0 illegal-move / 0 disconnect / 0 no-move / 0 crash. tally.json matches crosstable exactly and PGN result-code distribution matches both.
verdict:                   MEASUREMENT COMPLETE — HEAD pins at 2704 [2665,2743], +15 vs the 2689 [2649,2729] Jul-08 pin = WITHIN the ±39.5 delta-CI floor, so formally FLAT-TO-UP (not a significant gain). No pin regression ⇒ the decision-rule anomaly (HEAD point below 2689) did NOT fire. Consistent with the +30.5 claimed self-play (T4c/T1e/T5) at the demonstrated ~60% transfer, but not separable from noise at this instrument's power. One NGN-side flag-out is a new observation vs Jul-04's 7/7 anchor-side — flagged for watch, not a revert trigger.
next_action:               box returns to the queue — T10b launches immediately (separate agent). Powered transfer certification remains the Batch-2 self-play SPRT [0,+6] vs c514a2b, due ≤2026-08-01. Watch the NGN-side long-TC forfeit if it recurs (T1e time-mgmt at 120+1 is outside the 10+0.1 SPRT test envelope).
```

## Instrument replication (matches `experiments/2026-07-04-gauntlet-repin.md` and `experiments/2026-06-28-strength-repin.md`)

Same instrument; only NGN changes (35c0c0a -> 5b93966) — that is the point of a re-pin:

- **Tool:** `cmd/gauntlet` (pools per-anchor performance ratings by inverse variance). The `cmd/gauntlet`
  source AND its imported `internal/uci` are unchanged since the 2689 pin (`git log 35c0c0a..HEAD --
  cmd/gauntlet` and `-- internal/uci` both empty), so the Jul-04 `gauntlet.exe` is REUSED byte-identical —
  sha256 `123e6cee6d4b728eb4373d3149a8382b4f2b53f4de48d3a95da4fbc81d7580d3` (verified on box). This is a
  stronger replication guarantee than a rebuild: the literal same measurement binary.
- **Anchor set (the exact 5 of the 2666/2689 pins, for pool comparability — do NOT add/remove anchors):**
  | version | binary (box) | CCRL | provenance |
  |---|---|---|---|
  | v6.1.0 | anchors\blunder_610.exe | 2155 | Blunder README rating table; `scripts/build-anchors.sh` |
  | v7.2.0 | anchors\blunder_720.exe | 2425 | Blunder README |
  | v7.4.0 | anchors\blunder_740.exe | 2532 | Blunder README |
  | v8.0.0 | anchors\blunder_800.exe | 2674 | Blunder README |
  | counter-3.8 | anchors\counter_38.exe | 2994 | CounterGo 3.8 b172b99, pre-NNUE pure-Go HCE, cross-family top anchor |
  Box `ratings.json` (sha-listed 707 bytes) contains EXACTLY these 5 with these CCRL labels (counter-3.8 =
  2994, per the anchor-label comparability caveat); the repo `opponents/ratings.json` has 7 (adds
  blunder-500/2080 + fruit-2.1/2685) and is frozen to these 5 via `-only v6.1.0,v7.2.0,v7.4.0,v8.0.0,counter-3.8`.
- **TC 120+1, concurrency 8, openings sprt_openings (974e4b5a…)** — identical to the 2666 + 2689 pins
  (box `sprt_openings.txt` re-hashed `974e4b5a…` before launch).
- **Machine class:** LAN 9800X3D native Windows — the same class as both prior box pins. The project rule
  "never mix machine classes within a pooled run" keeps this on the box (cloud Graviton linux/arm64 would
  not be directly comparable to 2666/2689).
- **NGN binary:** `ngn_t5.exe` (sha256 `0a8f65b7…`), the hash-back-verified post-T5-keep HEAD engine. It
  played the T5 SPRT in the box sprt dir; staged into the repin dir with `boxrepin.sh stage-ngn` and
  re-hashed `0a8f65b7…` intact in the repin dir.
- **Deviation from the prior pins:** ONLY the explicit `-only` freeze of the 5-anchor pool (repo ratings.json
  grew to 7; box ratings.json already holds only the 5). Binary build convention, TC, concurrency, openings,
  anchors, and the gauntlet tool are identical (the tool is byte-identical to Jul-04).

## Transfer question + the three keeps since the 2689 pin

Keeps since `35c0c0a` (the 2689-pin engine, T1b): **T4c +3.7 (a195717, corrhist family 3-for-3), T1e +6.6
(801d384, node-fraction best-move effort scaling — Batch-2 row 1), T5 +20.2 (5b93966, aspiration
modernization — Batch-2 row 2, biggest single keep of the campaign)** ≈ **+30.5 self-play**. The gauntlet
reads the CCRL-scale move for ALL THREE together (no per-change attribution — each change's own SPRT did
that). T4c was certified into the Batch-1 composite (194d894, +43.2, certified base advanced to `c514a2b`);
T1e + T5 are the pending Batch-2 (+26.8, cert due ≤2026-08-01 vs `c514a2b`). The **powered transfer
certification remains the Batch-2 self-play SPRT [0,+6]**; this pin is the absolute-scale readout only.

## Sizing — the delta-CI reality (prior pin: 2689 [2649,2729] = ±40 half-width, 400 games)

Identical to the Jul-04 sizing decision. Single-pin CI ≈ 790/√N. **The delta vs the FIXED 2689 pin is
floored at ±39.5** (the prior pin's variance cannot be re-tightened): even an infinite new pin gives delta
±39.5, so a "delta ±25 vs 2689" is **unachievable** by re-pinning HEAD alone; a tight transfer delta would
need a fresh A/B (both `35c0c0a` and HEAD), ~2x games. S1 (400g single pin) gives absolute CI ±39.5 and a
point-signal delta vs 2689. **δ ±25 costs ~4000 games (~$5 cloud / ~42h box)** — past the sweet spot; the
transfer confirmation is the self-play SPRT, so S1 stands (same reasoning as Jul-04, section "Sizing").

Predicted: +30.5 self-play at ~60% transfer ≈ **+18 CCRL**, i.e. HEAD ≈ **~2707** point estimate — inside
the ±40 floor above 2689, so the informative read is "flat-or-up, not down." A point at/above ~2689 confirms
no regression; a point below 2689 (or CI excluding it low) is the anomaly trigger.

## Decision (2026-07-19)

**S1 — 400g exact replica on the box.** Absolute-sanity + point-signal only; the delta-floor math is
accepted (±25 vs the fixed 2689 pin is unachievable); the powered Batch-2 SPRT is the transfer instrument.
Anomaly rule: HEAD below 2689 -> investigate, never auto-revert. This job takes the box now (idle,
interactive session present, no live sprt/gauntlet); expected ~4h box wall @120+1 c8 (the Jul-04 S1 400g run
of the same instrument; wall-time only — Jul-04's launch-to-collection gap was fetch lag, not run duration).

## Status

PENDING LAUNCH — pre-launch preflight complete (2026-07-19):
- Box interactive session present: `explorer.exe` PID 2060, Console Session 1.
- No live run: `boxrepin.sh ps` empty; `tasklist` shows no `sprt.exe`, no `gauntlet.exe` (the T5 SPRT
  completed and drained).
- Staged + hash-verified in the repin dir: `gauntlet.exe` `123e6cee…` (byte-identical Jul-04), all 5 anchors
  under `anchors\`, `sprt_openings.txt` `974e4b5a…`, `ratings.json` (5-anchor, counter-3.8=2994),
  `ngn_t5.exe` `0a8f65b7…` (candidate, freshly staged from the sprt dir).

Launch section (probe + confirmed-live evidence) appends below on GO.

## Launch

CONFIRMED LIVE 2026-07-19 06:59 EDT (`run_repin_0719.bat` + `repin_0719_out.txt` both created 06:59;
launching agent died before recording, coordinator verified directly at ~07:15). Topology: `gauntlet.exe`
PID 31256 + 8 `ngn_t5.exe` workers, Console session 1, sole live run on the box. Out-file header confirms
the instrument: 5 anchors, 80 games/anchor, real clock 120s+1.0s, concurrency 8, PGN capture to
`repin_0719.pgn`. Early health at game 17/400: 16W-1D-0L vs v6.1.0 (checkmates, one draw-rule) — expected
saturation of the bottom rung. `ngn_crashes.log` activity checked and BENIGN: init-banner lines only (one
per NGN process spawn, no crash dumps; file accumulates since Jun-28). Expected completion ~5-7h wall
(~12:00-14:00 EDT); completion marker `DONE_EXIT_` appended by the bat.

## Results / Verdict (2026-07-19)

**DONE_EXIT_0.** Launched 06:59 EDT, finished ~10:05 EDT ⇒ **~3h06m wall** (faster than the
5-7h estimate; the c8 oversubscription + adjudication-free real gauntlet drained quickly).
`repin_0719_tally.json` + `repin_0719.pgn` fetched to `output/` (gitignored — numbers recorded here).
Tally, out-file crosstable, and PGN result-code distribution all agree (400 games).

### Crosstable (verbatim from `repin_0719_out.txt`)

```
Anchor    CCRL    W   D   L   Score   Perf  95% CI         weight
v6.1.0    2155   57  22   1   85.0%   2456  [2351,2561]    13.7%
v7.2.0    2425   49  29   2   79.4%   2659  [2566,2752]    17.5%
v7.4.0    2532   34  42   4   68.8%   2669  [2588,2750]    22.7%
v8.0.0    2674   29  47   4   65.6%   2786  [2707,2866]    23.8%
counter-3.8  2994    8  32  40   30.0%   2847  [2765,2929]    22.2%
------------------------------------------------------------------------
POOLED NGN rating: 2704   95% CI [2665, 2743]   (5 anchors, 80 games each)
```

Harness caveat line (same instrument + caveat as the 2666/2689 pins, so pin-to-pin is valid):
CCRL-scale via Blunder anchors at movetime; ~+30-80 optimistic vs a true 2'+1" clock run.

### Pooled reading vs the prior pin

- **HEAD = 2704 CCRL, 95% CI [2665, 2743].**
- Prior pin = **2689 [2649, 2729]** (2026-07-08, exact-replica instrument).
- **Delta = +15**, which is **WITHIN the ±39.5 delta-CI floor** set by the fixed 2689 pin's variance
  (see Sizing). So the honest read is **FLAT-TO-UP, not a significant gain** — do NOT claim a real +15.
- Predicted was +18 (+30.5 self-play × ~60% transfer ⇒ ~2707); measured +15 lands right on that
  prediction but is **not separable from noise** at 400g single-pin. The powered transfer instrument
  remains the Batch-2 self-play SPRT [0,+6] vs `c514a2b` (due ≤2026-08-01).
- Claimed self-play since the Jul-08 pin = T4c +3.7 + T1e +6.6 + T5 +20.2 = **+30.5**; measured
  transfer is **consistent** with the demonstrated ~60%-transfer curve, but the CI floor means this
  pin can only confirm "no regression," not quantify the gain.
- **Pin trend:** 2380 → 2449 → 2632 → 2669 → 2666 → 2689 → **2704**.
- Decision-rule anomaly (HEAD point below 2689, or CI excluding 2689 low) **did NOT fire** — HEAD point
  is above 2689 and the CI comfortably contains it. No pin regression; no investigation triggered on
  the regression front.

### Forfeit accounting (from the PGN, per-game result codes)

4 time-forfeits in 400 games. PGN line N == game N; result code is NGN-perspective (W/D/L), confirmed
by the per-version tally match.

| game | anchor | NGN color | result | who flagged | side |
|---|---|---|---|---|---|
| 39  | v6.1.0 | white | NGN win  | anchor lost on time | anchor-side |
| 46  | v6.1.0 | white | NGN win  | anchor lost on time | anchor-side |
| 63  | v6.1.0 | white | NGN win  | anchor lost on time | anchor-side |
| **193** | **v7.4.0** | **white** | **NGN loss** | **NGN lost on time** | **NGN-SIDE** |

- **3 anchor-side, 1 NGN-side (game 193).** The out-file running tally confirms game 193 bumped NGN's
  loss count vs v7.4.0 from 1 → 2 with reason `time-forfeit`; the game ran 116 plies (a long endgame
  where NGN as White flagged).
- **⚠ FLAG:** Jul-04's precedent was **7/400 forfeits, all anchor-side (0 NGN-side)**. This run has the
  **first NGN-side flag-out** since the reset. It matters because NGN's time management changed the night
  before (T1e keep — node-fraction effort scaling) and this gauntlet ran at **120+1** (long TC), which
  the 10+0.1 self-play SPRTs never exercise. One in 400 (0.25%) is a low rate and not a revert trigger
  (this is a measurement, not a keep/reject gate), but it is a genuine new behavior — **watch for
  recurrence at long TC**; if NGN-side long-TC flag-outs climb, the T1e/T1b soft-budget composition under
  the [0.72,1.40] clamp is the first suspect.
- No other flags: **0 panics, 0 illegal-move, 0 disconnect, 0 no-move, 0 crash** across all 400 games
  (`ngn_crashes.log` accumulation is benign init-banner lines only, as noted at launch).

### Verdict

Milestone gauntlet complete: **NGN pins at 2704 [2665, 2743]**, flat-to-up vs the 2689 prior within the
CI floor, no pin regression, transfer consistent with ~60% but noise-limited. One NGN-side time-forfeit
flagged for watch. Box returns to the queue; **T10b launches immediately** (separate agent). This pin
supersedes 2689 as the current strength estimate.
