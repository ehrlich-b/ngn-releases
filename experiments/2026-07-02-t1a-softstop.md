# 2026-07-02 T1a — soft-stop drops the next-iteration projection (time-management)

Pre-registered BEFORE launch (CLAUDE.md work-loop step 5). One isolated change, candidate vs its
immediate base (post-iir3 HEAD), **real-clock ONLY** (time/TC changes gate on real-clock games, never
a proxy). Speculative until its verdict: NOT committed; the diff is backed up at `output/t1a.patch`.

```yaml
id: 2026-07-02-t1a-softstop
date: 2026-07-02
change_class: search/eval heuristic (time-management) — real-clock-ONLY gate
hypothesis: firing the soft stop at elapsed>=soft (vs the prior ×1 next-iter projection) spends more of the soft bank -> more depth, with no added flag risk
base_commit: 6959a14 (post-iir3 HEAD; includes iir3 provisional #1)
candidate_commit: 6959a14 + output/t1a.patch (engine/time.go + engine/time_test.go only)
base_binary_sha256: 85703c188be79b728909433cb91a2b673efe3b03a6bc6fabc8fce59e1cd58aa5 (ngn_base2.exe, built from 464c33e = post-iir3 engine)
candidate_binary_sha256: 7193250d355093bd1f75567bd7f3e19e4ddf0b8debdf4899415ad2b71922ae33 (ngn_t1a.exe, 464c33e + T1a)
harness_commit: sprt.exe sha256 e9d904b878f4cb373bcc55abc325763d24f81f75248e9cb9d1540c6b93d3a668 (box, unchanged)
command: sprt.exe -new ngn_t1a.exe -base ngn_base2.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 8000 -mingames 300
machine: AMD Ryzen 7 9800X3D 8c/16t, native Windows (LAN box 192.168.4.108)
go_version: go1.26.2
goarch_goamd64: windows/amd64 GOAMD64=v3
tc: 10+0.1 (seconds; bullet)
concurrency: 8
openings: canonical sprt_openings (5000 lines)
openings_sha256: 974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222
aa_preflight: M2 c8 A/A PASSED (aa_c8_out.txt, ngn_base2 vs ngn_base2, 1600g, adjudication OFF): penta Elo -0.7 [-11,+9] centered, W-D-L 311-975-314, 0/0 flag-outs, DONE_EXIT_0, ~930 games/hr -> c8 config validated
decision_rule: H0 elo<=-3 / H1 elo>=+3 (alpha=beta=0.05 -> pLLR bounds +-2.94); 8000 max / 300 min; adjudication OFF. pLLR>=+2.94 KEEP; <=-2.94 REJECT; capped-positive (point>=+1, pLLR>0, no regression) PROVISIONAL keep #2 (batch cert); capped-nonpositive SHELVE; candidate flag-outs above the A/A baseline (0) -> HALT + investigate
games_or_pairs: 2864 games / 1432 pairs (stopped at the H0 bound, before the 8000 cap)
result: penta [LL 44 LD 352 {LW,DD} 701 WD 294 WW 41]; penta Elo -7.8 [-15,-0] pLLR -3.22; trinomial -7.8 [-20,+5] LLR -2.92; W-D-L 510-1780-574 (48.9%); duration 3h9m
flags_errors: flag-outs new 0 / base 0 of 2864 (goal 0); no crashes/illegal; DONE_EXIT_0
verdict: REJECT (H0 ACCEPTED: new <= -3 Elo; pLLR -3.22 crossed the -2.94 bound). 0 flag-outs -> a genuine strength regression, NOT a timing HALT
next_action: SHELVED 2026-07-02 — working tree reverted (git restore; patch kept at output/t1a.patch); T1 re-scoped to T1b (keep the projection, stability-scale the soft target)
```

## Hypothesis
NGN under-spends its soft clock. The tournament soft-stop fired on the ×1 next-iteration projection
(`elapsed + lastIterationTime > soft`), which trips once `elapsed` reaches `soft − lastIterationTime`
— roughly 2/3 of the soft budget — leaving the bank underused on a depth-starved engine. Firing only
when the soft budget is actually spent (`elapsed >= soft`) buys ~1 more iteration's worth of clock per
move (more depth) WITHOUT touching the hard ceiling or emergency floor, so it cannot raise flag risk.
Class prior: Stash v26 time-management package +31/+29 Elo. This is the 2-week-overdue staged 1-liner
(TRIED-LEDGER "Time management" residual: Counter spends ~98% of clock vs NGN ~70-80%).

## Mechanism (the single change)
`engine/time.go` `shouldStopTournamentSearch`: the soft-limit return changes from
`elapsed + tm.lastIterationTime > soft` (with the `lastIterationTime==0` special case) to
`return elapsed >= tm.softTime`. The hard ceiling (`elapsed >= hardTime`, capped at ≤30% of the usable
bank), the emergency floor (`baseTime − elapsed <= emergencyTime`), and the "finish ≥1 iteration" guard
are UNTOUCHED — they remain the flag protection. `lastIterationTime` is still maintained (its write path
is unchanged) but is no longer read by the soft stop; a KEEP would remove the now-unused field as
follow-up cleanup, a REJECT reverts this single hunk.

`engine/time_test.go` `TestTournamentSoftStop` updated to the new semantics — red→green: the old code
stopped at `elapsed=3700ms` (3700+500 > soft 4000) via the projection; T1a must NOT stop before
`soft=4000ms` and DOES stop at/after it. Full `go test -short [-race] ./engine` + `go test -short ./...`
GREEN with the change.

## Change isolation
- Base = post-iir3 HEAD `6959a14`. Binary `ngn_base2.exe` (cross-compiled from clean 6959a14).
- Candidate = base + T1a (`output/t1a.patch`; engine/time.go + engine/time_test.go only — verified via
  `git status`/`git diff --stat`, 20 insertions / 11 deletions across the two files).
- Binary SHA-256s recorded in Phase C after cross-compile; the diff is the soft-stop condition only.

## Verdict (2026-07-02) — REJECT; the projection is load-bearing

The T1a real-clock c8 SPRT crossed the H0 bound and was REJECTED. Verbatim RESULT (`t1a_out.txt` tail):

```
=== RESULT (3h9m11s) ===
Games: 2864   W-D-L: 510-1780-574   score: 48.9%
Elo(new - base): -7.8   95% CI [-20, +5]
LLR: -2.92   bounds [-2.94, 2.94]
Pentanomial [LL 44  LD 352  {LW,DD} 701  WD 294  WW 41] over 1432 pairs
Penta Elo: -7.8   95% CI [-15, -0]   pLLR -3.22  (THE decision stat; trinomial above is secondary)
Flag-outs (lost on time): new 0, base 0  of 2864 games  (goal: 0)
Verdict: H0 ACCEPTED: new is NOT better (<= -3 ELO)
DONE_EXIT_0
```

**Lesson: the next-iteration projection is LOAD-BEARING, not just flag protection.** T1a made NGN spend
the full soft budget every move (vs stopping at ~2/3 via the projection) and it played **-7.8 Elo WORSE
with ZERO flag-outs** — so this is not a timing/flag failure, it is a genuine strength regression. The
early-40-games health check was clean (G63, 0 non-normal terminations), so the regression is broad, not a
startup artifact. Burning the bank uniformly on every move (including easy ones) starves the moves that
matter — the same failure mode the removed easy-move shrink used to cause. The naive "raise clock usage"
one-liner is CLOSED; the real lever is **T1b**: KEEP the projection and instead STABILITY-SCALE the soft
target (spend more only while the decision is still volatile, less once settled — the CounterGo
three-regime / Stash-v26 stability manager), which is the change the +31/+29 prior actually measured. The
A/A c8 preflight PASSED (penta -0.7 [-11,+9], 0/0 flag-outs), so the config is validated and the verdict
is trustworthy.

**Disposition:** SHELVED. Working tree reverted to HEAD (`git restore engine/time.go engine/time_test.go`);
the diff is retained at `output/t1a.patch`. Box clean (self-terminated DONE_EXIT_0, `ps` empty). TRIED-LEDGER
+ TODO T1 updated to re-scope toward T1b.
