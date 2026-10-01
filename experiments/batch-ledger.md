# Batch-Certification Ledger

The sanctioned route (CLAUDE.md / TODO item 10, 2026-07-01) for capped-but-positive results.
A result with **point estimate ≥ +1, LLR > 0, and no regression signal** may enter main as a
PROVISIONAL keep instead of being shelved-at-cap. After **≤4 provisionals OR 2 weeks**, ONE
powered real-clock SPRT **[0, +6]** of the post-batch HEAD vs the pre-batch base certifies the
whole batch at once (or, on failure, a bisect over the batch finds the regressor). This recovers
the +2–8 self-play class the classical climb is made of (Stash ≈ 100 keeps at median +4.5) without
keeping speculative changes on sign-at-cap noise.

Rules:
- A provisional's own verdict must be a COMPLETED real-clock self-play run (never a killed/mid-run SPRT).
- Certification run needs its own A/A preflight at the exact TC/concurrency/machine.
- Fail → bisect the batch (each provisional isolable via its commit) and drop the regressor(s).
- Certified provisionals graduate to permanent keeps; the pre-batch base advances to the certified HEAD.

---

## Batch 1 — CERTIFIED 2026-07-18 (pre-batch base: `ea69ad6`, opened 2026-07-01, certified HEAD: `c514a2b`)

**CERTIFIED 2026-07-18** — composite `c514a2b` vs `ea69ad6` SPRT [0,+6] = **+43.2 penta [+26,+61], pLLR +3.09
>= +2.94, H1 ACCEPTED, 745g / 372 pairs, 0/0 flag-outs, 46m21s, DONE_EXIT_0** (`2026-07-18-batch1-cert.md`).
The predeclared PASS rule fired: **iir3 graduated from provisional to permanent keep**, and the **pre-batch
base pointer advanced from `ea69ad6` to `c514a2b`**. Measured composite +43.2 exceeds the +34.3 claimed
self-play sum. No bisect (PASS, not FAIL).

| # | Change | Commit | Verdict (real-clock self-play, 10+0.1) | Run record |
|---|---|---|---|---|
| 1 | iir3 — phase-guarded cutNode-IIR (missing standard technique) | `7394668` | **CERTIFIED (graduated from provisional 2026-07-18)** — own run penta +2.0 [-3,+7], pLLR +1.71, 0 flag-outs, 6000g; confirmed inside the certified +43.2 composite | `2026-06-29-iir.md` |
| 2 | T4 — pawn correction-history re-plumb | `ffb24af` | FULL H1 accept — penta +9.1 [+0,+18], pLLR +2.80, 0/0 flag-outs, 2325g | `2026-07-02-t4-corrhist-replumb.md` |
| 3 | T4np — non-pawn correction-history add | `13560dc` | FULL H1 accept — penta +6.4 [-1,+14], pLLR crossed +2.94 at G3796, 0/0 flag-outs, 3804g | `2026-07-02-t4-nonpawn.md` |
| 4 | T1b — symmetric stability-scaling of soft time budget | `35c0c0a` | FULL H1 accept — penta +13.1 [+3,+23], pLLR crossed +2.94 at G2225, 0/0 flag-outs, 2233g | `2026-07-03-t1b-stability-scaling.md` |
| 5 | T4c — minor-piece correction-history add | `a195717` | FULL H1 accept — penta +3.7 [-2,+9], pLLR +2.98, 0/0 flag-outs, 7842g | `2026-07-03-t4c-minor-corrhist.md` |

- **Batch net (claimed self-play):** **+34.3** across 5 keeps = iir3 +2.0 (provisional) + T4 +9.1 + T4np +6.4 + T1b +13.1 + T4c +3.7 (4 full H1 accepts). Post-batch HEAD `c514a2b` engine == the T4c kept tree `a195717` (T13/T1c/T16 all reverted, `git diff a195717..c514a2b -- engine/` empty).
- **Trigger:** certify when this batch reaches 4 provisionals **OR** by **2026-07-15**, whichever first. **Date trigger reached; certification SPRT launching 2026-07-18** (3 days past the 2026-07-15 date trigger) — manifest `2026-07-18-batch1-cert.md`, post-batch HEAD `c514a2b` vs pre-batch base `ea69ad6`, SPRT [0,+6] on the LAN 9800X3D box.
- **Certification run:** post-batch HEAD vs `ea69ad6`, real-clock `-tc 10+0.1`, SPRT **[0, +6]**, A/A preflight required, on the LAN 9800X3D box.
- **What the [0,+6] gate certifies (coordinator, 2026-07-08):** the HEAD-vs-`ea69ad6` composite SPRT is a
  WHOLE-BATCH gate on the NET post-`ea69ad6` transfer (iir3 + T4 + T4-nonpawn + T1b together), consistent
  with the 2689 [2649,2729] gauntlet point-signal (+23 vs 2666, inside the ±40 delta floor). It is NOT an
  iir3-isolation test: iir3's per-change evidence is its OWN completed real-clock c4 run (+2.0 [-3,+7],
  6000g, 0 flag-outs; `2026-06-29-iir.md`) and **no further iir3-specific SPRT is planned.** On PASS the
  whole batch stands; on FAIL the existing bisect-on-fail path isolates the regressor by commit.
- **On fail:** bisect batch-1 commits (`7394668`, …) to find the regressor; drop it, re-certify.
- **Note (2026-07-02) — main also carries T4 (FULL H1 accept, NOT a provisional):** the pawn corrhist
  re-plumb landed on main as an outright SPRT accept (+9.1 penta [+0,+18], pLLR +2.80, 0/0 flag-outs /
  2325g, real-clock 10+0.1 c8; `2026-07-02-t4-corrhist-replumb.md`). Because the certification run measures
  post-batch HEAD vs `ea69ad6`, the certified composite will INCLUDE T4's gain alongside the iir3
  provisional's; the bisect-on-fail rule already covers attribution (T4 is isolable by its own commit).
- **Note (2026-07-02) — main also carries T4-nonpawn (FULL H1 accept, NOT a provisional):** the non-pawn
  corrhist add on the fixed T4 plumbing landed on main as an outright SPRT accept (+6.4 penta [-1,+14],
  pLLR crossed +2.94 at G3796, 0/0 flag-outs / 3804g, real-clock 10+0.1 c8; `2026-07-02-t4-nonpawn.md`) —
  under a STRICT gate (H1-only, NO provisional-on-cap). Same attribution note as T4: the post-batch
  certification composite (vs `ea69ad6`) will include this gain too, and it is isolable by its own commit
  for the bisect-on-fail path.
- **Note (2026-07-03) — main also carries T1b (FULL H1 accept, NOT a provisional):** symmetric
  stability-scaling of the soft time budget (wire the dead `stableIters` read) landed on main as an
  outright SPRT accept (+13.1 penta [+3,+23], pLLR crossed +2.94 at G2225, 0/0 flag-outs / 2233g,
  real-clock 10+0.1 c8 adjudicated; `2026-07-03-t1b-stability-scaling.md`). Same attribution note as
  T4/T4-nonpawn: the post-batch certification composite (vs `ea69ad6`) will include this gain too, and it
  is isolable by its own commit for the bisect-on-fail path. Batch 1 still holds ONE provisional (iir3);
  T4 / T4-nonpawn / T1b are all full H1 accepts. Cumulative claimed self-play since `ea69ad6` is now
  ~+28-30 (T4 +9.1, T4-nonpawn +6.4, T1b +13.1) — past the +15-20 milestone-gauntlet cadence trigger.
- **Note (2026-07-09) — main also carries T4c (FULL H1 accept, NOT a provisional):** the minor-piece
  corrhist add (third corrector, on the fixed T4 plumbing) landed on main as an outright SPRT accept
  (+3.7 penta [-2,+9], pLLR +2.98 ≥ +2.94, 0/0 flag-outs / 7842g, real-clock 10+0.1 c8 adjudicated;
  `2026-07-03-t4c-minor-corrhist.md`) — the corrhist family is 3-for-3 on the fixed plumbing (pawn +9.1,
  non-pawn +6.4, minor +3.7, diminishing as predicted). Same attribution note as T4/T4np/T1b: the
  post-batch certification composite (vs `ea69ad6`) includes this gain too, isolable by commit `a195717`
  for the bisect-on-fail path. Cumulative claimed self-play since `ea69ad6` is now **+34.3** (iir3 +2.0
  provisional + T4 +9.1 + T4np +6.4 + T1b +13.1 + T4c +3.7). Post-T4c items T13/T1c/T16 were all shelved
  and reverted, so post-batch HEAD `c514a2b` engine is byte-identical to the T4c keep `a195717`.
- **Certification (2026-07-18) — FIRED, CERTIFIED:** manifest `2026-07-18-batch1-cert.md`; post-batch HEAD
  `c514a2b` (== `a195717` engine) vs pre-batch base `ea69ad6`, SPRT [0,+6] (pLLR bounds ±2.94), real-clock
  10+0.1 c8 adjudicated, on the LAN 9800X3D Windows box. Result **+43.2 penta [+26,+61], pLLR +3.09, H1
  ACCEPTED, 745g, 0/0 flag-outs, 46m21s, DONE_EXIT_0**. The PASS rule fired: whole batch certified, iir3
  graduated from provisional to permanent, pre-batch base pointer advanced to `c514a2b`. No bisect.

---

## Batch 2 — CERTIFIED 2026-07-25 (pre-batch base: `c514a2b`, opened 2026-07-18, certified HEAD: `172e9e1`)

**CERTIFIED 2026-07-25** — composite `172e9e1` (engine == `5b93966`) vs `c514a2b`, SPRT [0,+6] = **+35.1 penta
[+19,+51], pLLR +2.88 with the +2.94 bound crossed at G777 (max +2.98 @G781), H1 ACCEPTED, 785g / 392 pairs,
0/0 flag-outs, 48m36s, DONE_EXIT_0** (`2026-07-24-batch2-cert.md`). Measured **+35.1 exceeds the +26.8 claimed**
self-play sum — the second straight certification to overshoot its claim (Batch 1: +34.3 claimed → +43.2
measured). PASS, so **no bisect**; T1e and T5 both graduate to permanent certified keeps and the **certified
base pointer advances `c514a2b` → `172e9e1`**. Certified base binary staged on the box as `ngn_cert2.exe`
(`0a8f65b7…`, hash-verified after copy); `ngn_cert.exe` (Batch-1 base) deliberately left in place so an older
bisect remains reachable.

**Why this result mattered beyond the number:** it landed directly after three consecutive T-item shelves
(T10b, T7, T9a), when the live hypothesis was that self-play keeps were not transferring and that T5's +20.2 —
the campaign's biggest single keep — would be the first suspect in a bisect. The composite came in *above*
claim instead, which clears the instrument and the keep rule and localizes the recent nulls to candidate
quality. **Batch certification is now 2-for-2, both times overshooting**, mild evidence that isolated
per-change SPRTs systematically *understate* keeps (each is measured against a base already containing the
others' gains, and each stops at the first bound crossing rather than estimating precisely).

Certified base `c514a2b` is engine-identical through current HEAD `f5b7c34` (the `e7bfd46`/`f5b7c34` delta is
doc-only). Batch 2 accumulates the next capped-but-positive PROVISIONAL keeps against this base; full H1
accepts also land on main and are folded into this batch's composite for attribution, same as Batch 1.

| # | Change | Commit | Verdict (real-clock self-play, 10+0.1) | Run record |
|---|---|---|---|---|
| 1 | T1e — node-fraction (best-move node-effort) time scaling | `801d384` | FULL H1 accept — penta +6.6 [-1,+14], pLLR peak +2.96 crossed +2.94 at G4218, c8-drain to +2.85, 0/0 flag-outs, 4226g (2026-07-19) | `2026-07-18-t1e-node-effort.md` |
| 2 | T5 — aspiration modernization (scaled init + progressive widening + never-discard) | this commit | FULL H1 accept — penta +20.2 [+7,+33], pLLR peak +2.98 crossed +2.94 at G1202 (sustained G1299–1305), c8-drain to +2.88, 0/0 flag-outs, 1309g (2026-07-19) | `2026-07-19-t5-aspiration.md` |

- **Trigger fired:** the 2-week date trigger (2026-08-01), at 0 provisionals — certified 6 days early, 2026-07-25.
- **Certification run:** DONE — `172e9e1` vs `c514a2b`, real-clock `-tc 10+0.1` c8, SPRT [0,+6], standing M5 A/A basis, LAN 9800X3D box.
- **On fail:** n/a — PASSED.
- **Batch net (claimed self-play):** **+26.8** (T1e +6.6 + T5 +20.2, both FULL H1 accepts — landed on main, not provisionals; folded into the composite for attribution same as Batch 1's full accepts). 0 provisionals so far.
- **Note (2026-07-19) — T1e (FULL H1 accept, NOT a provisional):** node-fraction best-move effort scaling of the
  soft time budget (SF-standard third time signal, composed multiplicatively with T1b's stability factor under
  the T1b-proven [0.72,1.40] clamp) landed on main as an outright SPRT accept: penta **+6.6 [-1,+14]**, pLLR
  peak **+2.96 crossed the +2.94 bound at G4218** (first & only crossing, c8-drain to +2.85 at 4226g), 0/0
  flag-outs / 4226g, real-clock 10+0.1 c8 adjudicated (`2026-07-18-t1e-node-effort.md`) — under the STRICT
  per-change gate (H1-only, NO provisional-on-cap). Isolated SPRT [-3,+3] vs `c514a2b`. Fixed-depth
  node-identical (nodecheck 230036/181982/587557 unchanged), so no nodecheck re-lock; hash-back reproduced the
  exact playing binary `f65a3c16…`. The post-batch certification composite (vs `c514a2b`) will include this
  gain, isolable by this commit for the bisect-on-fail path.
- **Note (2026-07-19) — T5 (FULL H1 accept, NOT a provisional):** aspiration-window modernization (replace the
  fixed 50cp / jump-to-±INFINITY / discard-after-3 scheme with a Stash-shape scaled initial window
  `delta = 12 + |prevScore|/81`, progressive ×1.5 widening on fail with a +1 growth floor, never-discard
  iterations, `AspInit`/`AspMult` exposed as UCI/SPSA tunables) landed on main as an outright SPRT accept:
  penta **+20.2 [+7,+33]**, pLLR **peak +2.98 crossed the +2.94 bound at G1202** (first touch G1202–1203,
  sustained crossing G1299–1305, c8-drain to +2.88 at 1309g), 0/0 flag-outs / 1309g, real-clock 10+0.1 c8
  adjudicated (`2026-07-19-t5-aspiration.md`) — under the STRICT per-change gate (H1-only, NO
  provisional-on-cap). Isolated SPRT [-3,+3] vs the on-box T1e base `ngn_t1e.exe` `f65a3c16…` (== `801d384`).
  This is a search-behavior change, NOT node-identical: fixed-depth nodecheck MOVED (kiwipete d12
  230036→346662 +51%, mid d12 181982→149587 −18%, end d16 587557→765656 +30%), verified deterministic, so the
  baselines were re-locked to 346662/149587/765656 in the follow-up commit. Hash-back reproduced the exact
  playing binary `0a8f65b7…`. **Biggest single keep of the revamp campaign** (prior max T1b +13.1). The
  post-batch certification composite (vs `c514a2b`) will include this gain, isolable by this commit for the
  bisect-on-fail path. Batch-2 net claimed self-play now **+26.8** (T1e +6.6 + T5 +20.2), 0 provisionals.

---

## Batch 3 — **CERTIFIED 2026-08-02** (pre-batch base: `172e9e1`, opened 2026-07-25, certified HEAD: `50adc99`)

**CERTIFIED 2026-08-02** — composite `50adc99` (T12 + T19, on-box `ngn_t19on12.exe` `6a560c98…`) vs
`ngn_cert2.exe` (`172e9e1`), SPRT [0,+6] = **+9.9 penta [+3,+17], pLLR +2.98 (envelope max +3.05, 10 samples
at/above the bound; post-mingames envelope [0.15,+3.05] never negative), H1 ACCEPTED, 4013g / 2006 pairs,
0/0 flag-outs, 4h11m32s, DONE_EXIT_0** (`2026-08-02-batch3-cert.md`). **Penta CI EXCLUDES ZERO** — a
materially stronger statement than either component alone managed (T12 [-3,+8], T19 [-0,+15]).

**Measured +9.9 vs +9.5 claimed** — the third straight overshoot, so the DIRECTION is established (isolated
SPRTs do not over-state keeps here). **But the magnitude collapsed: +8.9, +8.3, then +0.4.** The earlier
surpluses were not a bonus that scales; at this size the claimed sum is close to accurate. **Stop planning as
though batches come in ~+9 above claim.**

PASS, so **no bisect**. **T12 graduates from PROVISIONAL to permanent**; T19 confirmed. **Certified base
pointer advances `172e9e1` → `50adc99`.** Certified binary staged on box as `ngn_cert3.exe` (`6a560c98…`,
hash-verified after copy); `ngn_cert.exe` and `ngn_cert2.exe` deliberately left in place so older bisect
targets stay reachable.

**Duration note:** pre-registered as "hours, possibly the full cap — NOT the ~48m of Batches 1 and 2", because
those crossed on effects 5-7x the H1 bound while this claim was ~1.6x it. Took **4h11m / 4013g**. The "~1h"
figure first used to justify early certification was optimistic and was corrected **before** the run.

---

## Batch 4 — OPEN (certified base: `50adc99`, opened 2026-08-02)

On-box certification binary: `ngn_cert3.exe` sha256
`6a560c98cee9902f0ff5ca469ffade44b7c206bbe7e608b20ff9c764be277fb3`.

| # | Change | Commit | Verdict (real-clock self-play, 10+0.1) | Run record |
|---|---|---|---|---|
| — | (none yet) | — | — | — |

- **Trigger:** certify when Batch 4 reaches 4 provisionals **OR** 2 weeks (**<= 2026-08-16**), whichever first.
- **Certification run:** post-batch HEAD vs `ngn_cert3.exe` (`50adc99`), real-clock `-tc 10+0.1` c8,
  SPRT [0,+6], LAN 9800X3D box. **Record the pollable run name in TODO's BOX RUN STATE when launched.**
- **On fail:** bisect Batch 4 commits, drop the regressor, re-certify.
- **Batch net (claimed self-play):** +0.0 — empty. **First candidate: T19b, live since 2026-08-02 22:18.**

---

## Batch 3 — superseded header (kept for the ledger's chronology)

Original: OPEN (certified base: `172e9e1`, opened 2026-07-25)

Certified base `172e9e1` has engine byte-identical to the T5 keep `5b93966` (T7, T9a and T10b were all
shelved and reverted, so nothing since T5 has changed the engine). On-box certification binary:
`ngn_cert2.exe` sha256 `0a8f65b75e59c376052f83b14787b0af2eba65552cedfee6f70ce576891ca721`.

Batch 3 accumulates the next capped-but-positive PROVISIONAL keeps against this base; full H1 accepts also
land on main and are folded into this batch's composite for attribution, same as Batches 1 and 2.

| # | Change | Commit | Verdict (real-clock self-play, 10+0.1) | Run record |
|---|---|---|---|---|
| 2 | **T19 — ttPv (reduce less at previously-PV nodes)** | `50adc99` | **FULL H1 ACCEPT** — penta **+7.3 [-0,+15]**, pLLR **+3.05** crossed +2.94 at G3908 (16 samples at/above, envelope [-0.30,+3.05], printed == max so no drain), 3929g, 0/0 flag-outs, 4h6m20s, DONE_EXIT_0. Measured ON TOP of T12. | `experiments/2026-08-01-t19-ttpv.md` |
| 1 | **T12 — NMP restricted to true cut nodes** | `c010ca7` | **PROVISIONAL** — penta **+2.2 [-3,+8]**, pLLR +1.83 (envelope [-0.64,+1.83], no bound crossing), 8000g capped, 0/0 flag-outs, 8h19m21s, DONE_EXIT_0 | `experiments/2026-08-01-t12-nmp-cutnode.md` |

- **Trigger:** certify when Batch 3 reaches 4 provisionals **OR** 2 weeks (**≤ 2026-08-08**), whichever first.
- **DECIDED 2026-08-02: CERTIFY EARLY — immediately after T15 resolves, BEFORE launching T8.** The trigger is
  a maximum ("≤ 4 provisionals"), not a minimum, so certifying at 2 rows is within the rule (Batch 2 also
  certified with 2 rows). Reasons, in order of weight:
  1. **T8's tuning base is HEAD + `output/t17.patch`, and HEAD now contains T12 + T19.** If that composite is
     a mirage, a **25-hour** tuner is spent optimising constants against an illusory base. Certification costs
     **~1 hour** (Batch 1 = 46m, Batch 2 = 49m) and de-risks the largest single box commitment left.
  2. **Bisecting 2 rows is trivial; bisecting 4 is not.** If the composite fails, doing it now leaves exactly
     two candidates to separate.
  3. Two keeps just landed after **nine consecutive misses**; confirming them cheaply is worth more than the
     hour it costs.
- **DURATION ESTIMATE CORRECTED 2026-08-02 — my own "~1 hour" figure in the early-cert decision was
  OPTIMISTIC and rested on a bad comparison.** Batch 1 (46m, 745g) and Batch 2 (49m, 785g) crossed fast
  because their effects were **5-7x the H1 bound** (+43.2 and +35.1 against `elo1 = 6`). **Batch 3 claims
  only +9.5 — about 1.6x the bound** — so the SPRT has far less margin to work with and should be expected
  to take **substantially longer, plausibly running to the 8000g cap (~8h)** rather than stopping in under
  an hour. **Does the early-cert decision still hold at 8h? Yes** — the argument was never that certification
  is cheap in absolute terms, it is that **a 25-hour tuner must not run against an unconfirmed base**, and
  8h < 25h regardless. But the "~1h" claim should not be repeated as though it were established.
- **Cert command (one command either way — both candidate binaries already on box, hash-verified):**
  - if **T15 KEEPS**: `-new .\ngn_t15on1219.exe -base .\ngn_cert2.exe`
  - if **T15 SHELVES**: `-new .\ngn_t19on12.exe -base .\ngn_cert2.exe`
  Full flags, matching Batches 1 and 2 exactly:
  `-tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 0 -elo1 6 -alpha 0.05
  -beta 0.05 -maxgames 8000 -mingames 300 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10
  -drawminplies 80`. `ngn_cert2.exe` verified on box at
  `0a8f65b75e59c376052f83b14787b0af2eba65552cedfee6f70ce576891ca721`.
- **PRE-REGISTERED READING (written 2026-08-02 11:20, before the cert and before T15's verdict):**
  - **H1 accept (pLLR ≥ +2.94)** ⇒ composite CERTIFIED. T12 graduates from provisional to permanent, T19
    confirmed, certified base pointer advances to the new HEAD, Batch 4 opens.

- **POST-VERDICT PROCEDURE, written 2026-08-02 19:20 at G1968 with the outcome unknown** (cert was at +8.1,
  pLLR +1.08 and climbing). A certification carries more bookkeeping than a normal verdict, so both branches
  are written out:

  **On PASS (pLLR >= +2.94):**
  1. Record the result here and in `experiments/2026-08-02-batch3-cert.md`, with the **measured vs claimed**
     comparison (+9.5 claimed) — that ratio is the running check on whether isolated SPRTs under- or
     over-state keeps, and it is now the THIRD data point (Batch 1 +43.2 vs +34.3, Batch 2 +35.1 vs +26.8).
  2. **T12 graduates from PROVISIONAL to permanent**; T19 (already a full accept) is confirmed.
  3. **Advance the certified base pointer** to the current HEAD commit, and **stage the certified binary on
     the box as `ngn_cert3.exe`**, hash-verified after copy. **Leave `ngn_cert.exe` and `ngn_cert2.exe` in
     place** — older bisect targets stay reachable, exactly as Batch 2 did.
  4. **Open Batch 4** against the new pointer, 0 rows, trigger `<= 4 provisionals OR 2 weeks`.
  5. **The gauntlet re-pin trigger should be re-evaluated but NOT auto-fired:** claimed self-play since the
     2026-07-19 pin would be ~+9.5, still well short of the +15-20 cadence and only ~+5 CCRL at the measured
     transfer against a ±39.5 delta-CI floor. **Deferral stands unless the certified number is much larger
     than claimed.**
  6. Launch **T19b** (`experiments/2026-08-02-t19b-ttpv-lmp.md`), which is ranked ahead of T8.

  **On FAIL (pLLR <= -2.94):**
  1. **Bisect the two rows** — this is why the cert was moved early, while only T12 and T19 are in play.
     The natural first split is T12 alone vs `ngn_cert2.exe` (T12's own SPRT measured it against the pre-T12
     base, so a direct re-measure against the certified base is the clean separator).
  2. **Do NOT launch T8** — its base contains this composite, which is the whole reason for certifying first.
  3. Drop the regressor, re-certify, and only then resume the queue.

  **On CAPPED INCONCLUSIVE:** **not a pass** (pre-registered above). Report the estimate against +9.5, leave
  the base pointer where it is, leave T12 provisional, and **still launch T19b** — an inconclusive says the
  composite is not demonstrably >= +6, not that it is bad, so the queue continues while Batch 3 stays open.
  - **H0 accept (pLLR ≤ −2.94)** ⇒ FAIL ⇒ bisect the two rows and drop the regressor.
  - **Capped inconclusive** ⇒ **NOT a pass.** Report the point estimate against the +9.5 claim and do **not**
    advance the certified base; the batch stays open. Do not reinterpret an inconclusive as a soft pass.
  - **Calibration note recorded in advance so it cannot be used selectively afterward:** both prior batches
    **OVER**-shot their claims by ~+9 (Batch 1 +43.2 vs +34.3; Batch 2 +35.1 vs +26.8). **Batch 3's claim is
    +9.5 — far smaller — so the overshoot pattern may simply not scale, and an under-shoot here is NOT by
    itself evidence of the non-additivity finding.** If it under-shoots, the middlegame interaction is a
    reasonable first diagnostic hypothesis, nothing more.
- **Certification run:** post-batch HEAD vs `ngn_cert2.exe` (`172e9e1`), real-clock `-tc 10+0.1` c8, SPRT [0,+6], LAN 9800X3D box.
- **On fail:** bisect Batch 3 commits to find the regressor; drop it, re-certify.
- **Batch net (claimed self-play):** **+9.5** — T12 +2.2 (provisional) + T19 +7.3 (full accept). Increments are MARGINAL (each measured vs the immediate base), so the net is a telescoping sum, not an additivity assumption.
- **Nodecheck baselines re-locked by row 1** to 451141 / 136100 / 796906, then **by row 2** to **295507 / 112109 / 667703** (`scripts/nodecheck.sh`).
- **Interaction watch:** T12 x T19 measured strongly NON-additive before T19 launched (-14.8/-25.1/-12.8% composed, where each alone grows kiwipete). If T19 also keeps, this batch's composite is the place a non-additive loss would surface — and note Batches 1 and 2 both OVER-shot their claims (+43.2 vs +34.3, +35.1 vs +26.8), so an under-shoot here should be read against `experiments/2026-08-01-queue-composition-check.md` rather than treated as a surprise.
