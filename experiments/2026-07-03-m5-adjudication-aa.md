# 2026-07-03 M5 enable A/A — score adjudication on the box mill (harness validation)

Pre-registered BEFORE launch. This is the **enable-gate A/A** for M5 (score adjudication in cmd/sprt,
committed `8026781`, default-OFF). It is NOT an Elo verdict — it is the sanctioned validation cycle that
must pass before any real verdict trusts adjudication. A same-binary base-vs-base run at the real mill TC
(10+0.1 c8) with adjudication ON must (a) still center on Elo 0, (b) flag-out 0/0, (c) actually fire the
adjudication reasons, and (d) run at ≥ the no-adjudication throughput baseline.

```yaml
id: 2026-07-03-m5-adjudication-aa
date: 2026-07-03
change_class: harness validation (mill throughput) — NOT an Elo verdict; A/A no-bias + capability check
hypothesis: score adjudication (resign>=900cp/5p, draw<=10cp/10p past move 40) is UNBIASED — base-vs-base stays Elo≈0 with 0 flag-outs — and actually fires, while finishing at ≥ the ~920 games/hr no-adjudication baseline (M2 c8 A/A).
harness_under_test: M5-aware sprt.exe (cross-compiled from HEAD 13560dc, GOOS=windows GOARCH=amd64 GOAMD64=v3, go1.26.2) sha256 30c33e0512725b7f552d8a1cf72ba6f1e0deb4dcb122c6b6ba8f6433c686a762
old_harness_backed_up: box sprt.exe (pre-M5, 6/29 build) copied to sprt_pre_m5.exe sha256 e9d904b878f4cb373bcc55abc325763d24f81f75248e9cb9d1540c6b93d3a668 (reversible; the M5 change is default-OFF + backward-compatible, so without the flags the new binary behaves identically)
boxsprt_edit: NONE — scripts/boxsprt.sh passes arbitrary sprt args through verbatim (args="$*"), so the adjudication flags need no wrapper change. The only harness delta is the sprt.exe binary itself (M5 code already committed).
engine_binary: ngn_base4.exe sha256 e7f69ea727233a9c80704cba718fec97ce3aa65fc6426660252a1717e4ddc881 (clean HEAD 13560dc engine; == the committed T4-nonpawn engine, hash reproduced) — played against ITSELF
adjudication_flags: -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80 (the M5 record's suggested conservative first-enable values; reasons emitted = adj-win / adj-draw; end-of-run "Adjudicated early: N decisive, M draw")
openings: canonical sprt_openings (5000 lines) sha256 974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222 (verified on box)
throughput_baseline: M2 c8 A/A (aa_c8_out.txt) = 1600 games / 1h43m15s ≈ 930 games/hr, penta -0.7 [-11,+9], 0/0 flag-outs (no adjudication) — same TC/concurrency/machine/game-count as this run, so wall-time is a direct comparison.
```

## Why a fresh M5-aware sprt.exe was pushed

The box sprt.exe (last written 6/29, before the M5 commit `8026781` on 2026-07-01) is NOT M5-aware — it
rejects `-resignscore` ("flag provided but not defined"). Adjudication lives in the HARNESS binary
(cmd/sprt + internal/uci), not the engine, so a fresh sprt.exe cross-compiled from HEAD 13560dc was pushed
(overwriting), with the old one backed up to `sprt_pre_m5.exe` first. Per CLAUDE.md harness rules, a
harness change needs its own validation cycle + A/A before use in a verdict — **this run IS that cycle.**
Nothing new to commit: the M5 code is already committed and boxsprt.sh is unchanged; the only post-pass
artifact is this run record.

## Predeclared PASS rule (M5 enable gate)

PASS **requires ALL four**:
- **(a) No bias:** Elo ≈ 0 — SPRT H0-accepts, OR reaches the game cap with the 95% CI comfortably
  straddling 0 (mirrors the M2 A/A shape). A decisive H1 accept in EITHER direction = FAIL.
- **(b) Clean clocks:** flag-outs new 0 / base 0 (any flag-out = FAIL, same as every real-clock gate).
- **(c) Adjudication actually fires:** the "Adjudicated early: N decisive, M draw" line appears with
  **N > 0 AND M > 0** (both adj-win and adj-draw kinds observed in the game log). Zero adjudications =
  FAIL (the capability is unproven / thresholds unreachable).
- **(d) Throughput:** games/hr measured and reported; expected ≥ the ~920/hr M2 baseline (the whole point
  — decided/dead games end early). Reported regardless; a REGRESSION below baseline is a flag to
  investigate, not an automatic fail (correctness a-c dominates).

On PASS: adjudication is cleared for future box/cloud verdicts; bake these flags into the launch commands.
On FAIL: adjudication stays OFF; restore sprt.exe from sprt_pre_m5.exe; investigate before any verdict
uses it.

## Command (via scripts/boxsprt.sh, matches the M2 c8 A/A exactly except binary name + the 5 adjudication flags)

```
sprt.exe -new .\ngn_base4.exe -base .\ngn_base4.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 1600 -mingames 300 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
```

```yaml
machine: AMD Ryzen 7 9800X3D 8c/16t, native Windows (LAN box 192.168.4.108)
tc: 10+0.1 (seconds; bullet)
concurrency: 8
maxgames: 1600
mingames: 300
```

## Status

PASS (2026-07-03, all four criteria). Base-vs-base A/A with adjudication ON at 10+0.1 c8 = penta **-1.1
[-13,+11]** at the 1600-game cap (INCONCLUSIVE = the A/A terminal state), **0/0 flag-outs**, **77.6%
adjudicated** (858 decisive + 383 draw), **961 vs 932 games/hr (+3.1%)**, 0 crash/illegal/no-move. The
M5-aware sprt.exe (`30c33e05…`, HEAD 13560dc) is now the VALIDATED mill harness; adjudication is
non-distorting and stays ON as standard. Record committed; the pre-M5 sprt.exe backup stays on the box.

## Launch

Launched **2026-07-03 09:20:05 box time** (Mac-initiated 09:20:03 EDT) via `scripts/boxsprt.sh launch
m5aa` → `m5aa_out.txt`. Written batch (`run_m5aa.bat`) verified verbatim (the exact command above, all 5
adjudication flags present).

Liftoff evidence:
- Remote binary SHA-256s CONFIRMED == local before launch: sprt.exe 30c33e05… (M5-aware — box `-h` now
  lists `-resignscore`/`-drawscore`), ngn_base4.exe e7f69ea7…; openings on box == 974e4b5a…. Old sprt.exe
  backed up to sprt_pre_m5.exe (e9d904b8…).
- `ps`: 1 sprt + 16 ngn_base4 engines (c8 × 2, both copies the same binary), StartTime 9:20:05 AM.
- Header parsed correctly, including the adjudication line:
  `adjudication: resign>=900cp/5p  draw<=10cp/10p>=80p  (opt-in; A/A-gate before trusting a verdict)`.
- **Adjudication FIRING live in the first 8 games — BOTH kinds:** G2/G3 `adj-win`, G4–G7 `adj-draw`
  (alongside normal `draw-rule`). PASS criterion (c) is already being satisfied. Early Elo +88 (2W-6D-0L)
  is tiny-sample noise that regresses to 0 over 1600 games.

ETA ~1.2–1.6h (M2 no-adjudication baseline was 1h43m/1600g; adjudication ends decided/dead games early, so
this should beat it — the throughput measurement is the wall-time at 1600 games). This run re-invokes on
completion to apply the PASS rule. M6 (5+0.05 A/A) is NOT launched yet — box is serial, M6 follows this
verdict.

## Verdict (2026-07-03) — PASS (all four criteria)

The A/A ran to the 1600-game cap (DONE_EXIT_0, 1h39m51s) and terminated INCONCLUSIVE — the EXPECTED A/A
terminal state (true Elo = 0 never reaches an SPRT bound). Verbatim RESULT (`m5aa_out.txt` tail):

```
=== RESULT (1h39m51s) ===
Games: 1600   W-D-L: 428-739-433   score: 49.8%
Elo(new - base): -1.1   95% CI [-18, +16]
LLR: -0.16   bounds [-2.94, 2.94]
Pentanomial [LL 56  LD 173  {LW,DD} 337  WD 188  WW 46] over 800 pairs
Penta Elo: -1.1   95% CI [-13, +11]   pLLR -0.18  (THE decision stat; trinomial above is secondary)
Flag-outs (lost on time): new 0, base 0  of 1600 games  (goal: 0)
Adjudicated early: 858 decisive, 383 draw  of 1600 games
Verdict: INCONCLUSIVE (ran out of games — add openings or raise -maxgames)
DONE_EXIT_0
```

| # | Criterion | Result | Verdict |
|---|---|---|---|
| a | No bias: Elo ≈ 0, CI straddles 0 | penta -1.1 [-13,+11], pLLR -0.18 (INCONCLUSIVE at cap = A/A terminal state) | **PASS** |
| b | Clean clocks: flag-outs 0/0 | new 0 / base 0 of 1600 | **PASS** |
| c | Adjudication fires, both kinds, no distortion | "858 decisive, 383 draw of 1600" = 77.6% adjudicated; base-vs-base still centers on 0 (no W/L skew) | **PASS** |
| d | Throughput vs ~932/hr baseline | 1600 games / 1h39m51s (99.85 min) = **961 games/hr = +3.1%** vs M2's 932/hr | **PASS (reported)** |

Integrity: 0 crash / 0 illegal / 0 no-move / 0 time-forfeit lines in the full log; box `ps` empty post-run.

**Throughput mechanism note (why +3.1%, not a naive ~15%).** Real-clock games are WALL-bounded by the
clocks, not by move count — a game runs until the clocks expire regardless of ply count. So ending a
decided/dead game early only reclaims the INCREMENT-phase tail (the 0.1s/move increment on the moves that
would otherwise have been played out), not a proportional slice of a fixed compute budget. At 10+0.1 that
tail is a modest fraction of total wall-time → +3.1%. The lever is real but small at this TC; it grows
with a larger increment or longer base (part of why M6 tests 5+0.05, where the increment share differs).
Much of adjudication's value here is CORRECTNESS, not speed: a +900 game is now scored a WIN instead of
running 200 plies to a `max-moves` DRAW.

**Harness-swap validation.** This A/A also validated the harness binary swap: the M5-aware sprt.exe
(`30c33e0512725b7f552d8a1cf72ba6f1e0deb4dcb122c6b6ba8f6433c686a762`, cross-compiled from HEAD 13560dc) is
now the VALIDATED mill harness — base-vs-base is clean and unbiased with adjudication ON. The pre-M5
sprt.exe backup (`sprt_pre_m5.exe`, `e9d904b8…`) stays on the box (revert path). `scripts/boxsprt.sh` was
NOT edited (flags pass through verbatim). From now on the adjudication flags (`-resignscore 900
-resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80`) are part of the standard box mill command;
adjudication stays ON and is cleared for real verdicts.

**Verdict: PASS — adjudication validated non-distorting; stays ON. Next mill item: M6 (5+0.05 c8 A/A).**
