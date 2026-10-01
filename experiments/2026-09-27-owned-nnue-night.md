# Owned NNUE night session — 2026-09-27

Live record for the overnight WSL session. Artifacts live under
`/home/ehrli/nnue-owned-k4-20260920/night-20260927/` on WSL; small receipts are
copied here with the `2026-09-27-` prefix. All matches use
[`scripts/quickmatch.py`](../scripts/quickmatch.py): hash-checked inputs, the
frozen fastchess (`e61220f6…`) argv shape (sequential frozen openings,
`-repeat` pairs, per-game affinity, `-strict`), the frozen independent chess
auditor (`4c4d7374…`) with Stockfish 18 (`6b087694…`), and a report paired by
round (percentile bootstrap over pair scores, 20,000 resamples). Engine
options are sent in the audited order Threads=1, OwnBook=false, EvalFile,
EvalBackend, Hash=128, Move Overhead=100; `GOMAXPROCS=1`. Host: 9800X3D WSL
(Go 1.25.5, GOAMD64=v3, CGO_ENABLED=0), shared with GPU training and the idle
hopper. Relative results only; no absolute rating.

## Recovered state

The Sep 24 A1 result (archive-labeled K4, **promoted**, +57.9 vs Rodent V1.1)
existed only as uncommitted files in a `/private/tmp` worktree that macOS was
already purging. Files were rescued, verified byte-identical to the sources
that built/trained A1 on WSL, and committed (`1835960`).

## M1 — A1-e10 vs Rodent V1.2 (old binary)

Same binary as the A1 gate (`1f250284…`, int32 K4 kernels), openings
4241–4340, seed 2026092301, 10+0.1, c4 on CPUs 8/10/12/14, 200 games.
Predeclared rule: owned net replaces V1.2 as working reference only if the 95%
lower bound is above zero; V1.2 stays if the upper bound is below zero;
otherwise parity. [Spec](2026-09-27-m1-a1-vs-v12-spec.json),
[report](2026-09-27-m1-a1-vs-v12-report.json).

- 59/103/38 (white/draw/black wins), penta `[2, 29, 50, 18, 1]`, 32,853 legal
  plies, audit PASS, zero book-signature plies.
- A1-e10 score 0.4675 [0.430, 0.505], **−22.6 Elo [−49.0, +3.5]**: parity.
- In-game NPS: K4 1.017M mean vs V1.2 1.193M (K4 at 85%) — the old int32 K4
  kernels, which S1 then replaced.

## S1 — exact int16 kernels (accepted, pure speed)

See [the S1 record](2026-09-27-s1-k4-int16-kernels.md): identical search on 88
fixtures, 1.162x faster. Research tags for K8/W1024 layouts:
[record](2026-09-27-k4-research-layout-tags.md).

## M1b — A1-e10 vs Rodent V1.2 on the S1 binary

Identical to M1 except both roles run the S1-era binary (`5d989a7f…`, built
from the tree committed as `d7501b6`, default tags); the V1.2 code path is
unchanged, so pairing with M1 by round isolates the K4 speedup.
[Spec](2026-09-27-m1b-a1-vs-v12-spec.json),
[paired report](2026-09-27-m1b-a1-vs-v12-paired-report.json).

- Audit PASS, 200 games; penta `[2, 21, 47, 26, 4]`.
- A1-e10 score 0.5225 [0.4825, 0.5625], **+15.6 Elo [−12.2, +43.7]**:
  parity under the M1 rule, point estimate now favoring the owned net.
- Paired with M1: **+0.055 score [+0.0025, +0.1075]** from the S1 speedup
  (supporting evidence only; S1 was accepted on identity plus timing).
- In-game NPS: K4 1.146M mean vs V1.2 1.126M (K4 now at 102%).

## K8 — eight king buckets, same corpus and schedule

Trainer built with `NGN_K8` from harness source `478f512f…` (the K8-switch
revision; `d7501b6` only adds the unset-by-default `NGN_H1024` switch), binary
`33ba487c…`, mode `archive-a1-e10` (1,638,400 updates, cosine 0.001→0.00001,
seed 26092001) on the unchanged A1 corpus (`270f65bd…`, 2,374,290,033 records);
~6.2M positions/s (A1: 8.8M), about 58 minutes. Selector (`ngnk4bridge`
built with `-tags ngnk8`) picked update **1,540,096**; integer validation MSE
on the frozen SF18 holdout ≈0.00478 (final 0.004783) versus A1's selected
0.004850. Converted model `765251377cb5…` (9,463,568 bytes).
[Convert](2026-09-27-k8-e10-convert.json),
[parity](2026-09-27-k8-e10-parity.json) (first 3 of 12 records kept): the
Bullet float probe and the Go reference agree to 6.7e-7 z on all 12 parity
FENs (so the Go K8 table matches Bullet's mirrored layout), quantization
delta mean 5.2 cp / max 11.7 cp, **PASS**.

## M2 — K8 vs K4 A1-e10: **K8 SHELVED**

Binaries from the `d7501b6` tree: K8 `ef4e1a53…` (`-tags ngnk8`), K4
`5d989a7f…` (default). Openings 4341–4740 frozen tonight from the pinned
`sprt_openings.txt` (`974e4b5a…`; prefixes `c2531d2a…`, PGN `e9c2c2d0…` via the
existing converter; disjoint from all earlier 4001–4340 slices). 800 games,
10+0.1, c6 on CPUs 4/6/8/10/12/14, seed 2026092702. Predeclared rule in the
[spec](2026-09-27-m2-k8-vs-k4-spec.json): promote K8 only if the paired 95%
lower bound on score exceeds 0.5. [Report](2026-09-27-m2-k8-vs-k4-report.json).

- Audit PASS, 800 games, 121,925 legal plies; penta `[14, 89, 181, 104, 12]`.
- K8 score 0.5069 [0.4856, 0.5281], **+4.8 Elo [−10.0, +19.6]** → rule not
  met; K4 remains the layout. No extension.
- NPS: K8 0.997M vs K4 1.028M (−3%, extra king-bucket refreshes).
- The 1.7% validation-MSE gain converted to at most a small Elo effect.

## W1024 — 1024 hidden, same corpus and schedule

Trainer built with `NGN_H1024` from harness `ba4f9c44…` (= `d7501b6`), binary
`fa3caff2…`; ~7.3M positions/s. Selected update 1,458,176, integer validation
MSE 0.004794 (A1 0.004850, K8 0.004767). Model `8c5de170…`.
[Convert](2026-09-27-w1024-e10-convert.json),
[parity](2026-09-27-w1024-e10-parity.json): probe Δz ≤ 7.2e-7, quantization
mean 7.9 cp / max 16.5 cp, **PASS**. Its gate (M3, seed 2026092703, same rule
and openings, binary `c92e78e0…`) is running.

## Eval-scale diagnostic

Consecutive-ply score pairs (both |score| ≤ 400) in PGNs give each engine's
typical magnitude on shared positions: A1 K4 mean |score| 108.9 vs Rodent
V1.2 64.7 (M1b, n≈10.9k; through-origin slope 1.50); the older 20M K4 135.4 vs
HCE 64.7 (Sep 22 confirmation PGN, n≈15.7k). HCE and V1.2 agree, so the owned
K4 speaks a centipawn scale about 1.7x larger than the one every search margin
(RFP, futility, razoring, aspiration, singular, qsearch delta) was tuned on.

## M3 — W1024 vs K4 A1-e10: **W1024 SHELVED**

Same openings/rule as M2, seed 2026092703; W1024 binary `c92e78e0…`
(`-tags ngnw1024`). [Spec](2026-09-27-m3-w1024-vs-k4-spec.json),
[report](2026-09-27-m3-w1024-vs-k4-report.json).

- Audit PASS, 800 games; penta `[16, 83, 196, 89, 16]`.
- W1024 score 0.5038 [0.4831, 0.5250], **+2.6 Elo [−11.7, +17.4]** → rule not
  met; 768 hidden stays. No extension.
- NPS: W1024 0.936M vs K4 1.021M (−8%).
- With K8 (+4.8) and W1024 (+2.6) both inside noise, extra first-layer
  capacity is not the binding constraint on 2.37B positions at this schedule.

## K8+W1024 stacked net (queued as M4)

Trainer `03b01f80…` (`NGN_K8` + `NGN_H1024`), selected update 1,458,176,
integer validation MSE 0.004661 (best of the night; A1 0.004850). Model
`88d377f4…`, parity PASS. Gated last in the night queue (M4) because its two
components were each individually null.

## Night queue (automatic, one gate at a time on CPUs 4–14 even)

E1 (K4EvalScale 60 vs 100) → M5 (owned K4 vs exact Counter 5.5, run-003
settings, scale per E1) → M6 (result-weight ramp 0→0.25, final checkpoint) →
M7 (e20 schedule) → M4 (K8+W1024) → M8 (best owned configuration vs Rodent
V1.2, 800 games). Every spec embeds its predeclared rule; later gates read
earlier reports only through those rules (E1 scale; M8 candidate = best
promoted variant, else A1). The GPU trains WDL25 then e20 in sequence.

## E1 — K4EvalScale 60 vs 100: **ADOPTED (scale 60)**

Same binary (`75b0afdb…`, tree committed as `ccfb56a`) and net (A1-e10) on
both sides; only `K4EvalScale` differs (60 vs 100, sent after Move Overhead).
Default-scale identity against the S1 binary held on 88 fixtures
([receipt](2026-09-27-k4-evalscale-identity.json)); the engine short suite and
race suite passed. Openings 4341–4740, seed 2026092704, 800 games.
[Spec](2026-09-27-e1-k4-evalscale60-spec.json),
[report](2026-09-27-e1-k4-evalscale60-report.json).

- Audit PASS, 800 games; penta `[8, 83, 189, 106, 14]`.
- Scale 60 score 0.5219 [0.5019, 0.5425], **+15.2 Elo [+1.3, +29.6]** → rule
  met: adopt K4EvalScale=60 for A1-family nets. One value was tested and no
  second value is tried this session, so 60 is a sanctioned setting, not a
  tuned optimum.
- The in-game magnitude now matches the HCE/V1.2 scale: scale-60 mean |score|
  64.8 vs 104.7 at 100 (slope 0.600, as constructed).
- The code default stays 100 (identity with every earlier K4 run); K4 is an
  opt-in backend configured explicitly, so the working configuration sets
  `K4EvalScale 60`.

## M5 — owned K4 (scale 60) vs exact Counter 5.5: external anchor

Owned K4 A1-e10 at K4EvalScale=60 (binary `75b0afdb…`) against the exact
Counter 5.5 release (`6c48fb52…`, embedded NNUE, Threads=1, Hash=128,
ExperimentSettings=false, no overhead option), reusing the accepted
2026-09-18 run-003 opening set (`first200`: PGN `1ae6a65a…`, prefixes
`2257ecfb…`), seed 20260912, 10+0.1, 400 games, c4 on CPUs 8/10/12/14,
fastchess without `-strict` (as run-003 did for the external role).
[Spec](2026-09-27-m5-owned-vs-counter55-spec.json),
[report](2026-09-27-m5-owned-vs-counter55-report.json).

- Audit PASS, 400 games, 66,680 legal plies, zero book-signature plies;
  141 white mates / 117 black mates / 142 draws; penta `[0, 12, 43, 90, 55]`.
- Owned K4 score **0.735 [0.705, 0.765], +177 Elo [+151, +205]** vs Counter 5.5.
- Paired by round with run-003 (NGN at `52ee629` with borrowed Rodent V1.1
  Anand vs the same Counter, 0.541): **+0.194 score [+0.148, +0.240]**. That
  delta also contains search changes accepted since Sep 13, not only the net.
- NPS: NGN 1.164M, Counter 2.629M; NGN wins while searching less than half
  as many nodes per second.
- Boundary: one external opponent, one clock, shared host; relative Elo, not a
  list rating. The previous one-thread orientation put the Sep 12 NGN (HCE)
  at −323 vs Counter; the owned-net configuration now scores +177 against it.

## Incident — WSL instance shutdowns (01:08–01:39)

At 01:08:15 the WSL distro stopped (Windows host up since Sep 7, 18.7 GB
free, no `.wslconfig`); every transient `systemd-run --user` unit died with
it. The boot list then showed each new instance ending seconds to minutes
after the SSH session that started it disconnected (01:09–01:12,
01:13:03–01:13:23, 01:38:31). WSL terminates the distro when no `wsl.exe`
client remains attached. Something that had held one attached all evening
(apparently another long-lived session on the host) ended at 01:08; systemd
user services do not keep the distro alive. **Fix:** Windows scheduled task
`ngn-wsl-keepalive` running `wsl.exe -e sleep infinity` (created with a
far-future one-shot trigger and started manually); verified that the instance
survives with no SSH attached. Remove it with
`schtasks /delete /tn ngn-wsl-keepalive /f` when no longer wanted.

Losses and handling: M6's first attempt stopped at 84/800 games, and two M8
starts stopped at 5 games or fewer. All are preserved as `*-aborted-*`
directories, unscored and unpooled, and each gate reruns from game 1 with an
unchanged spec. e20 training resumes from `candidate-2375680` (145/200) via the
harness resume path. The farseer download resumes by byte range, and the partial
F1 scatter outputs (≤72 KB) were deleted and restarted.

## Incident — host C: drive full (≈02:05)

The WSL distro's `ext4.vhdx` lives on the Windows C: drive
(`C:\Users\ehrli\AppData\Local\wsl\{b04ba7d4-…}\ext4.vhdx`, 430 GB). Inside
WSL `df` still showed ~540 GB free (the virtual disk's maximum), but tonight's
~90 GB of downloads and checkpoints took **C: to 0 GB free**. The VHDX could
not grow, WSL returned EIO even for `/etc/passwd`, and it could not start
processes. This probably also caused the 01:08 shutdown; the keepalive fix
above is still needed for SSH-launched units.

Recovery: stopped the keepalive, `wsl --terminate Ubuntu`, restarted as root
(no ext4 errors in dmesg), deleted only my own re-downloadable farseer
binpacks (51 GB) and trimmed (`fstrim -v /`: 583.9 GiB). WSL `df` now shows
367 GB used, but the VHDX file is still 430 GB and **C: has ~0.5 GB free**.
Returning the trimmed space to Windows needs an elevated compaction, which
this non-elevated SSH session cannot run. `wsl --manage Ubuntu --set-sparse
true` is refused without `--allow-unsafe` (possible data corruption), so I
did not force it.

**Owner action (elevated):** `wsl --shutdown`, then either
`Optimize-VHD -Path "<vhdx>" -Mode Full` (Hyper-V module) or diskpart
`select vdisk file="<vhdx>"`, `attach vdisk readonly`, `compact vdisk`,
`detach vdisk`.

Consequences: the F1 data-scale pipeline is **cancelled**, because it needs
~350 GB of scatter/gather space that this host does not have. The partial F1
outputs were deleted, and the downloads must be redone once there is room.
e20 stays unfinished, resumable from `candidate-2375680`. The attempts cut off
by the EIO (M8 third start) are preserved unscored. Only low-write match gates
continue: M8 → M6 → M4.

## M8 — owned A1 (scale 60) vs Rodent V1.2, 800 games: **parity**

Same binary for both roles (`75b0afdb…`); owned A1-e10 with `K4EvalScale 60`,
borrowed V1.2 default (`c35a1abc…`) unchanged. No night variant had been
promoted, so the launcher picked A1. Openings 4341–4740, seed 2026092708, c6.
This was the fourth start; three earlier starts were cut off by the WSL
shutdowns and the disk-full I/O failure and are preserved unscored.
[Spec](2026-09-27-m8-owned-vs-v12-spec.json),
[report](2026-09-27-m8-owned-vs-v12-report.json).

- Audit PASS, 800 games, 128,605 legal plies; 226/409/165 (white win, draw,
  black win); penta `[6, 108, 165, 103, 18]`.
- Owned score 0.5119 [0.4906, 0.5331], **+8.3 Elo [−6.5, +23.1]** → parity
  under the predeclared rule (neither bound excludes 0.5). V1.2 formally stays
  the working reference; the owned net is statistically level with the
  strongest borrowed evaluator.
- NPS: owned 1.273M vs V1.2 1.246M. At scale 60, score magnitudes on shared
  positions are close (slope 1.03 owned~V1.2; mean |score| 83 vs 76).

## WDL25 — game-result weight ramping 0 → 0.25

Trainer built with `NGN_WDL25` from harness `7bb8ddb4…` (= `dc26c18`), binary
`273ea4b5…`: per superbatch, `ConstantWDL { value: 0.25 · (sb−1)/(total−1) }`,
so the weight on the STM game result ramps linearly from 0 to 0.25 across the
A1 e10 schedule (same corpus, seed, LR and graph). The selector ranks by
pure-score validation MSE, which penalizes a result-blended net, so the
**final** checkpoint (1,638,400) was converted directly, as declared before
training: model `1ec8fc17…`. Parity PASS (probe Δz 3.9e-7, quantization mean
4.6 cp / max 8.1 cp). [Convert](2026-09-27-wdl25-e10-convert.json),
[parity](2026-09-27-wdl25-e10-parity.json).

## M6 — WDL25 vs A1-e10 (both K4EvalScale 60): **WDL25 PROMOTED**

Fourth start (earlier ones cut off by the shutdowns, preserved unscored).
Binary `75b0afdb…` for both roles; openings 4341–4740, seed 2026092706, 800
games. [Spec](2026-09-27-m6-wdl25-vs-a1-spec.json),
[report](2026-09-27-m6-wdl25-vs-a1-report.json).

- Audit PASS, 800 games; penta `[4, 72, 208, 104, 12]`.
- WDL25 score 0.530 [0.5113, 0.5488], **+20.9 Elo [+7.8, +34.0]** → rule met:
  WDL25-e10 replaces A1-e10 as the owned working net.
- NPS equal (1.236M vs 1.239M). At the same scale, WDL25's scores are larger
  than A1's (mean |score| 89 vs 73, slope 0.79 A1~WDL25). Blending in results
  sharpens the evaluation, so K4EvalScale 60 was fitted to A1, not WDL25. A
  lower scale for WDL25 is a natural next single-value gate.
- The CPU slot after M4 now runs M11 (WDL25 at scale 60 vs Rodent V1.2, 800
  games, M8 rule) in place of the planned M10 longer-clock diagnostic, which
  had not started.

## M4 — stacked K8+W1024 vs A1-e10 (both scale 60): **SHELVED**

Binaries from one source tree: `21ba3794…` (`-tags ngnk8,ngnw1024`) vs
`75b0afdb…`; net `88d377f4…` (selected update 1,458,176, validation MSE
0.004661). Openings 4341–4740, seed 2026092705, 800 games.
[Spec](2026-09-27-m4-k8w1024-vs-a1-spec.json),
[report](2026-09-27-m4-k8w1024-vs-a1-report.json),
[convert](2026-09-27-k8w1024-e10-convert.json).

- Audit PASS, 800 games; penta `[6, 91, 188, 102, 13]`.
- Score 0.5156 [0.4956, 0.5356], **+10.9 Elo [−3.0, +24.8]** → rule not met;
  no extension. Capacity gains are consistent and small: K8 +4.8, W1024 +2.6,
  stacked +10.9, all inside noise on this 2.37B-position corpus.

## M11 — owned WDL25 (scale 60) vs Rodent V1.2: **OWNED NET REPLACES V1.2**

Identical to M8 except the owned role's net (WDL25-e10 `1ec8fc17…`, promoted
by M6) and seed 2026092711. The launcher asserted M6's promotion before
freezing the spec. [Spec](2026-09-27-m11-wdl25-vs-v12-spec.json),
[report](2026-09-27-m11-wdl25-vs-v12-report.json).

- Audit PASS, 800 games, 127,730 legal plies; 263/386/151 (white win, draw,
  black win); penta `[11, 60, 185, 126, 18]`.
- Owned score **0.550 [0.5288, 0.5706], +34.9 Elo [+20.0, +49.4]** → rule
  met: the owned WDL25-e10 net at K4EvalScale 60 replaces Rodent V1.2 as the
  working reference evaluator. This is relative to V1.2 in the same engine on
  the shared host, not an absolute rating. No installation or default change
  is made.
- NPS: owned 1.205M vs V1.2 1.184M.

## Night summary

| Gate | Change | Games | Score [95%] | Elo [95%] | Verdict |
| --- | --- | ---: | --- | --- | --- |
| M1 | A1 vs V1.2 (old int32 binary) | 200 | 0.468 [0.430, 0.505] | −22.6 [−49.0, +3.5] | parity |
| S1 | int16 kernels | 88 pos | identical search | 1.162x speed | accepted |
| M1b | A1 vs V1.2 (S1 binary) | 200 | 0.523 [0.483, 0.563] | +15.6 [−12.2, +43.7] | parity |
| M2 | K8 vs A1 | 800 | 0.507 [0.486, 0.528] | +4.8 [−10.0, +19.6] | shelved |
| M3 | W1024 vs A1 | 800 | 0.504 [0.483, 0.525] | +2.6 [−11.7, +17.4] | shelved |
| E1 | K4EvalScale 60 vs 100 | 800 | 0.522 [0.502, 0.543] | +15.2 [+1.3, +29.6] | **adopted** |
| M5 | A1@60 vs Counter 5.5 | 400 | 0.735 [0.705, 0.765] | +177 [+151, +205] | anchor |
| M8 | A1@60 vs V1.2 | 800 | 0.512 [0.491, 0.533] | +8.3 [−6.5, +23.1] | parity |
| M6 | WDL25 vs A1 (@60) | 800 | 0.530 [0.511, 0.549] | +20.9 [+7.8, +34.0] | **promoted** |
| M4 | K8+W1024 vs A1 (@60) | 800 | 0.516 [0.496, 0.536] | +10.9 [−3.0, +24.8] | shelved |
| M11 | WDL25@60 vs V1.2 | 800 | 0.550 [0.529, 0.571] | +34.9 [+20.0, +49.4] | **owned replaces V1.2** |

Not completed: e20 (resumable), F1 data scale (blocked on host disk).

## S12 — screen only: WDL25 at K4EvalScale 50 vs 60

200 games (first 100 pairs of 4341–4740), seed 2026092712, same binary and
net; no decision rule. Audit PASS; penta `[3, 21, 54, 21, 1]`; scale 50 scored
0.490 [0.453, 0.528], −6.9 Elo [−33.1, +19.1]. There is no sign that 50 beats
60, so a predeclared scale-50 gate is low priority.
[Report](2026-09-27-s12-wdl25-scale50-screen-report.json).
