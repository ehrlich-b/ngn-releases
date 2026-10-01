# 2026-07-04 gauntlet re-pin — absolute CCRL measure of HEAD 35c0c0a (+ transfer readout)

**Instrument-replication record for the milestone gauntlet** triggered by the CADENCE rule (4 keeps since
`ea69ad6` ≈ +28-31 claimed self-play). **This is a MEASUREMENT, not a keep/reject gate — no change is kept
or reverted off this number alone** (the ~Jul-15 batch-cert self-play SPRT `[0,+6]` vs `ea69ad6` is the
transfer *certification* instrument; this pin is absolute-sanity + point-signal only). **Sizing decided:
S1** (400g single HEAD pin, exact 2666 replica, on the box). Launch is on the team-lead's conditional GO
(see Status). Result/verdict fields fill at completion.

```yaml
id:                        2026-07-04-gauntlet-repin
date:                      2026-07-04
change_class:              measurement (absolute re-pin + transfer readout; NOT a keep/reject test)
hypothesis:                where does HEAD 35c0c0a sit on the CCRL scale vs the 2026-06-28 pin (2666 [2627,2706]), and how much of the +28-31 self-play from the four keeps since ea69ad6 transferred?
base_commit:               ea69ad6 (conceptual transfer "before"; S1 does not play it — the historical 2666 pin at 2dadef8 is the absolute reference. A fresh A/B was option S3, not chosen.)
candidate_commit:          35c0c0a191186fa7070c20d8b0646557a7fb3884 (HEAD; T1b KEPT)
base_binary_sha256:        n/a (S1 single-pin: HEAD vs fixed anchors; no fresh base binary — transfer is point-estimate vs the 2666 pin)
candidate_binary_sha256:   0fede586b4ae312f13d7752fe137a542b41cbdf6ee94eeac5988ebd1e63eadea  # ngn_t1b.exe, already staged on the box. GOOS=windows GOARCH=amd64 GOAMD64=v3, standard convention (NO ldflags/trimpath — matches the Jun-28 pin + every box binary). This IS the T1b-keep engine at HEAD 35c0c0a: t4c-runner independently rebuilt clean 35c0c0a with the standard convention and got this exact hash.
harness_commit:            cmd/gauntlet @ 35c0c0a; cmd/gauntlet source unchanged since Jun-28, rebuilt anyway because imported internal/uci changed after M5. gauntlet.exe sha256 123e6cee6d4b728eb4373d3149a8382b4f2b53f4de48d3a95da4fbc81d7580d3.
command:                   gauntlet.exe -ngn .\ngn_t1b.exe -anchors ratings.json -tc 120+1 -games 80 -concurrency 8 -lowpower=false -openings sprt_openings.txt -only v6.1.0,v7.2.0,v7.4.0,v8.0.0,counter-3.8 -tally-out repin_0704_tally.json -pgn repin_0704.pgn -no-record
machine:                   LAN 9800X3D (AMD Zen5 V-Cache, 8c/16t), native Windows — the SAME machine class as the 2666 pin (comparability requirement)
go_version:                go1.26.2
goarch_goamd64:            windows/amd64 v3
tc:                        120+1  (real clock, SECONDS — CCRL Blitz proxy; same as the 2666 pin)
concurrency:               8      (16 procs on 16 threads, oversubscribed; both sides equally slowed -> relative rating preserved — same as the 2666 pin)
openings:                  sprt_openings (corpus_manifest.md)
openings_sha256:           974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222
aa_preflight:              n/a for a gauntlet (NGN-vs-anchors, not self-play); pre-launch `-probe` recorded below. Note: gauntlet's probe is a movetime/stop probe; this real-clock `-tc` run does not skip older Blunder anchors on probe non-compliance, and clock/forfeit behavior is measured in the run.
decision_rule:             MEASUREMENT — report pooled CCRL rating + 95% CI + per-anchor crosstable; delta vs the 2666 pin = transfer readout (a SIGNAL: the delta-CI is wide, see Sizing). NOT a keep/reject gate. ANOMALY = a pin REGRESSION (HEAD pooled point below the 2666 point, or the CI excluding 2666 on the low side) — that would contradict the +28 self-play and triggers INVESTIGATION, never an auto-revert.
games_or_pairs:            400 (5 anchors x 80; S1)
result:                    POOLED NGN 2689, 95% CI [2649,2729] (5 anchors x 80g). Crosstable: v6.1.0 87.5%/perf2493, v7.2.0 75.0%/perf2616, v7.4.0 74.4%/perf2717, v8.0.0 61.9%/perf2758, counter-3.8 22.5%/perf2779. DONE_EXIT_0, run 2026-07-06 19:31 -> collected 2026-07-08.
flags_errors:              7/400 time-forfeits, ALL anchor-side (6x v6.1.0, 1x v8.0.0 — each an NGN win, W incremented), 0 NGN-side flag-outs, 0 illegal, 0 crash, 0 no-move. Cleaner than the Jun-28 pin (which had 6/400 anchor-side). tally.json W/D/L matches the crosstable exactly.
verdict:                   2689 [2649,2729]. Point-signal +23 vs the 2666 [2627,2706] pin (delta floored at +/-40 by the old pin's CI; NOT anomalous — HEAD is above 2666, no keep/reject). Consistent with the predicted ~+18 transfer (~60% of the +30 self-play from the four keeps).
next_action:               T4c SPRT launched 2026-07-08 21:39:32 box time on the box after this gauntlet reached DONE and boxsprt.sh ps showed idle (experiments/2026-07-03-t4c-minor-corrhist.md). Powered transfer certification remains the ~Jul-15 batch-cert self-play SPRT [0,+6] vs ea69ad6.
```

## Instrument replication (matches `experiments/2026-06-28-strength-repin.md`)

Same instrument; only NGN changes (2dadef8 -> 35c0c0a) — that is the point of a re-pin:

- **Tool:** `cmd/gauntlet` (pools per-anchor performance ratings by inverse variance). The `cmd/gauntlet`
  source itself is unchanged since the Jun-28 pin (`git log 2dadef8..35c0c0a -- cmd/gauntlet` empty), but
  the binary was rebuilt from current `35c0c0a` because imported `internal/uci` changed after M5. Staged
  `gauntlet.exe` sha256 `123e6cee6d4b728eb4373d3149a8382b4f2b53f4de48d3a95da4fbc81d7580d3`.
- **Anchor set (the exact 5 of the 2666 pin, for pool comparability — do NOT add/remove anchors):**
  | version | binary | CCRL | provenance |
  |---|---|---|---|
  | v6.1.0 | blunder_610 | 2155 | Blunder README rating table; `scripts/build-anchors.sh` |
  | v7.2.0 | blunder_720 | 2425 | Blunder README |
  | v7.4.0 | blunder_740 | 2532 | Blunder README |
  | v8.0.0 | blunder_800 | 2674 | Blunder README |
  | counter-3.8 | counter_38 | 2994 | CounterGo 3.8 b172b99, pre-NNUE pure-Go HCE, cross-family top anchor |
  `opponents/ratings.json` has since grown to 7 (added blunder-500/2080 + fruit-2.1/2685); both are frozen
  OUT via `-only v6.1.0,v7.2.0,v7.4.0,v8.0.0,counter-3.8` so the inverse-variance pool matches 2666 exactly.
- **TC 120+1, concurrency 8, openings sprt_openings (974e4b5a…)** — identical to the 2666 pin.
- **Machine class:** the 2666 pin ran on the LAN 9800X3D (native Windows). An absolute number *comparable to
  2666* must run on that same class — the project rule "never mix machine classes within a pooled run"
  applies, and cloud Graviton (linux/arm64) is a different class whose number would not be directly
  comparable to the box pin. So: box for the S1 absolute pin.
- **NGN binary:** `ngn_t1b.exe` (sha256 `0fede586…`), already staged on the box — the T1b-keep engine at
  HEAD 35c0c0a, windows/amd64 v3, standard convention (no ldflags/trimpath, matching the Jun-28 pin and
  every box binary). t4c-runner independently reproduced a clean 35c0c0a build byte-identical to it, so it
  is verified-clean 35c0c0a with zero dependence on the shared tree (which carries t4c-runner's uncommitted
  edits).
- **Deviation from the 2666 pin:** ONLY the explicit `-only` freeze of the 5-anchor pool (needed because
  ratings.json grew to 7). Binary, build convention, TC, concurrency, openings, anchors are identical.

## Transfer question + the four keeps in HEAD

Keeps since `ea69ad6` (the batch-1 base): **iir3 +2.0 (provisional #1), T4 +9.1, T4-nonpawn +6.4, T1b
+13.1** (the last three are full H1 accepts) ≈ **+28-31 self-play**. The gauntlet reads the CCRL-scale move
for ALL FOUR together (no per-change attribution — each change's own SPRT did that). The ~Jul-15 batch-cert
self-play SPRT `[0,+6]` vs `ea69ad6` is the transfer *certification*; this pin is the absolute-scale
readout. The 2666 pin was `2dadef8` (one step before `ea69ad6`); the 2dadef8->ea69ad6 gap is M-items/reset
(node-identical / harness / latent-correctness), expected ≈ flat, so HEAD-vs-2666 ≈ the four-keep transfer.

## Sizing — the delta-CI reality (prior pin: 2666 [2627,2706] = ±39.5 half-width, 400 games)

Single-pin CI ≈ 790/√N. **The delta vs the FIXED 2666 pin is floored at ±39.5** (the old pin's variance
cannot be re-tightened): even an infinite new pin gives delta ±39.5, so a "delta ±25 vs 2666" is
**unachievable** by re-pinning HEAD alone. A tight transfer delta would need a fresh **A/B** (gauntlet BOTH
`ea69ad6` and `35c0c0a`), delta CI = √2·CI_single — ~2x games.

| Option | What | Games | Absolute CI | Transfer δ-CI | Box wall @120+1 c8 | Cloud est* |
|---|---|---|---|---|---|---|
| **S1 (CHOSEN)** | single HEAD pin, 2666 replica | 400 | ±39.5 | ±56 vs 2666 (point signal) | ~4h (free) | ~$0.5 |
| S2 | single HEAD pin, tighter | 800 | ±28 | ±48 (still floored) | ~8h | ~$1 |
| S3 | fresh A/B ea69ad6-vs-HEAD | 2x1000 | ±25 ea | ±35 (A/B) | ~21h | ~$2.5 |
| S3′ | A/B for δ ±25 | 2x2000 | ±18 ea | ±25 | ~42h | ~$5 |

*The cloud gauntlet EXISTS and is turnkey (`scripts/cloudsprt.sh gauntlet` = `cmd_gauntlet` at
cloudsprt.sh:762 ships ngn+gauntlet+anchors+ratings per shard with `-tally-out`, pooled by `gfetch` at
:829; built by commit `907a78e`, Counter anchor wired by `09f28d6`). It is NOT used here for TWO reasons:
(i) Graviton (linux/arm64) is a different MACHINE CLASS than the Jun-28 9800X3D box pin, so a cloud number
is not directly comparable to 2666; (ii) the multi-family gauntlet has never actually been run on Graviton
(no cloud-gauntlet run record), so a cloud pin would itself need a validation pass first. **The box is
chosen for machine-class comparability, not because cloud is unavailable.** (An earlier draft of this
manifest wrongly claimed cloud orchestration was missing — corrected 2026-07-04.)

**δ ±25 costs ~4000 games** (~$5 cloud / ~42h box) — past the ~$1-3 sweet spot; not worth it for a
measurement whose confirmation is the self-play SPRT. Hence S1.

## Decision (2026-07-04, team-lead)

**S1 — 400g exact replica on the box.** Absolute-sanity + point-signal only; the delta-floor math is
accepted (±25 vs the fixed 2666 pin is unachievable); the powered ~Jul-15 batch-cert SPRT is the transfer
instrument. Anomaly rule approved (HEAD below 2666 -> investigate, never auto-revert). Sequencing: the
gauntlet takes the box IMMEDIATELY after M6 (ahead of T4c — this job is a fixed ~4h; T4c's variable
2-8.5h SPRT takes the deep-night slot after).

## Status

LAUNCHED 2026-07-06 19:31:40 box time / 19:32 EDT Mac-observed via `scripts/boxrepin.sh launch
repin_0704 ngn_t1b.exe 80`. The box was idle before launch; post-launch `ps` showed exactly one
`gauntlet.exe`, eight `ngn_t1b.exe`, and eight current anchor processes. Output header confirms
5 anchors, 80 games/anchor, real clock 120+1, concurrency 8, PGN capture to `repin_0704.pgn`.

Pre-launch staging:
- `gauntlet.exe` sha256 `123e6cee6d4b728eb4373d3149a8382b4f2b53f4de48d3a95da4fbc81d7580d3`
- `ngn_t1b.exe` sha256 `0fede586b4ae312f13d7752fe137a542b41cbdf6ee94eeac5988ebd1e63eadea`
- `sprt_openings.txt` sha256 `974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222`

Pre-launch probe (movetime/stop probe only; not a real-clock skip gate):

```text
Movetime/stop compliance probe (budget 1000ms)
Anchor    CCRL  complies     probe move time
v6.1.0    2155  NO (skip)    13.662s
v7.2.0    2425  NO (skip)    3.263s
v7.4.0    2532  yes          1.001s
v8.0.0    2674  yes          1.001s
counter-3.8  2994  yes       1s
```

Launch batch verified on box:

```bat
@echo off
cd /d "%USERPROFILE%\ngn\repin"
gauntlet.exe -ngn .\ngn_t1b.exe -anchors ratings.json -tc 120+1 -games 80 -concurrency 8 -lowpower=false -openings sprt_openings.txt -only v6.1.0,v7.2.0,v7.4.0,v8.0.0,counter-3.8 -tally-out repin_0704_tally.json -pgn repin_0704.pgn -no-record > repin_0704_out.txt 2>&1
echo DONE_EXIT_%ERRORLEVEL%>> repin_0704_out.txt
```

At completion: report crosstable + pooled number + flag/forfeit accounting to the team-lead, fill the
result/verdict fields here, then hand the box to t4c-runner.

## Result (2026-07-08)

Run started **2026-07-06 19:31** box time; result collected **2026-07-08**. `DONE_EXIT_0` (completion
evidence: final line of `output/repin_0704_out.txt`; fetched artifacts `repin_0704_out.txt`,
`repin_0704_tally.json`, `repin_0704.pgn` to the gitignored `output/`). Fetched `out.txt` tail and
`tally.json` both match this crosstable exactly.

Verbatim crosstable:

```text
== Crosstable ==
Anchor    CCRL    W   D   L   Score   Perf  95% CI         weight
v6.1.0    2155   60  20   0   87.5%   2493  [2380,2606]    12.4%
v7.2.0    2425   42  36   2   75.0%   2616  [2529,2703]    20.9%
v7.4.0    2532   42  35   3   74.4%   2717  [2631,2803]    21.2%
v8.0.0    2674   27  45   8   61.9%   2758  [2680,2836]    26.1%
counter-3.8  2994    6  24  50   22.5%   2779  [2689,2869]    19.4%
------------------------------------------------------------------------

POOLED NGN rating: 2689   95% CI [2649, 2729]   (5 anchors, 80 games each)
DONE_EXIT_0
```

**Flag / forfeit accounting.** 7/400 games ended `time-forfeit`; all 7 are **anchor-side** (the anchor
flagged) — each was an NGN win (the running W column incremented on every forfeit line): 6× against v6.1.0
(games 38, 41, 42, 44, 56, 70) and 1× against v8.0.0 (game 257). **0 NGN-side flag-outs.** No `illegal`, no
`crash`, no `no-move` results anywhere in the file (result-reason vocabulary is only checkmate 233,
draw-rule 117, max-moves 43, time-forfeit 7). This is cleaner than the Jun-28 pin's 6/400 anchor-side
forfeits — NGN's time manager never flagged at 120+1 c8. `repin_0704_tally.json` W/D/L equals the crosstable
for all five anchors.

**Interpretation.** Pooled HEAD (35c0c0a, T1b keep) rates **2689 [2649,2729]** on the exact 2666-pin
instrument (same box class, TC, concurrency, openings, 5-anchor pool). Point-signal **+23 vs the 2666
[2627,2706] pin.** Per the manifest's own Sizing section the delta vs the fixed 2666 pin is floored at
±39.5 (S1 cannot beat the old pin's variance), so +23 is a **point signal inside the noise floor**, not a
powered transfer number — but it is the right sign and magnitude: the four keeps since `ea69ad6` (iir3
+2.0, T4 +9.1, T4np +6.4, T1b +13.1 ≈ +30 self-play) predict ≈+18 on the CCRL scale at ~60% transfer, and
+23 sits right on that. This is **NOT an anomaly** (anomaly = HEAD pooled point *below* 2666, or the CI
excluding 2666 on the low side — neither holds; the CI [2649,2729] sits entirely above the 2666 point), so
no investigation trigger and, by design, no keep/reject off this number. Per-anchor: counter-3.8 (the
cross-family 2994 anchor) rates NGN **2779**, and the Blunder-ladder saturation pattern persists — the
weakest 2155 rung reads NGN at only **2493** (NGN wins ~87.5% and the perf estimate saturates below the
true strength because the rung is too weak to resolve the gap), while the stronger rungs climb 2616 → 2717
→ 2758. The **powered transfer instrument remains the ~Jul-15 batch-certification self-play SPRT [0,+6] vs
`ea69ad6`**; this pin is absolute-scale sanity + point-signal only, and it passes both.
