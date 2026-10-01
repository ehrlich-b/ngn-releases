# 2026-07-19 T5 — aspiration-window modernization (progressive widening, never-abandon)

Pre-registered BEFORE launch (CLAUDE.md work-loop step 5). Isolated single-change per-change gate: candidate
= HEAD engine (`801d384`, the T1e keep) + `output/t5.patch`, base = HEAD engine (`801d384`) unchanged, reusing
the on-box `ngn_t1e.exe` (the exact new-HEAD binary), real-clock 10+0.1 c8 on the LAN 9800X3D box. Next ranked
queue item after the T1e keep. Engine diff stays UNCOMMITTED per policy (shelved as `output/t5.patch`, tree
restored); commit only on a keep/provisional verdict.

## Mechanism (one paragraph)

Replaces the fixed 50cp (100cp when |prevScore|>800) aspiration window plus jump-straight-to-±INFINITY-on-fail
plus discard-after-3-attempts scheme with a modern progressive-widening loop (search.go, `searchIterative-
DeepeningUnsafe`). Depths ≤3 still search full width (no stable previousScore yet). From depth 4 up the window
opens as a small delta scaled by the previous score's magnitude — Stash shape `delta = ASP_INIT +
|prevScore|/ASP_SCORE_DIV` with `ASP_INIT=12`, `ASP_SCORE_DIV=81` — giving `alpha = prevScore - delta`,
`beta = prevScore + delta`. On a fail-low the delta grows by `ASP_MULT` (percent; `ASP_MULT=150` ⇒ ×1.5, with a
+1 floor so it strictly grows even when the percentage term truncates) and alpha widens downward
(`alpha = prevScore - delta`, clamped at −INFINITY) while the upper bound stays tight; a fail-high is symmetric
(widen beta up, clamp at +INFINITY, keep alpha). The widening is monotonic and caps naturally at ±INFINITY (a
full-width bound always lands in-window), so the loop is guaranteed to terminate and NEVER discards an iteration
— the pre-T5 `maxAttempts=3` cap and the `AspAbandoned++` discard path are removed; the previous-depth
keep-last-completed semantics remain, but now only a clock stop (not widening exhaustion) can leave an iteration
incomplete. `attempts` is retained solely to feed the window-held-on-first-attempt stability signal to time
management (`ReportCompletedIteration`, unchanged). `ASP_INIT` and `ASP_MULT` are exposed as UCI/SPSA tunables
(TunableSearchParams). Near-mate is handled by the natural widening (no special case): a mate-magnitude
prevScore opens a proportionally larger delta and fail-highs widen beta toward +INFINITY until the mate score is
inside the window. This is a search-behavior change (NOT time-management-only): fixed-depth node counts move
(nodecheck below), so it is gated by real-clock games.

```yaml
id: 2026-07-19-t5-aspiration
date: 2026-07-19
change_class: search/eval heuristic (search behavior — real-clock games gate)
hypothesis: >
  a modern aspiration window (small score-scaled initial delta, progressive geometric widening on fail instead
  of a jump to an infinite bound, and never discarding an iteration) prunes the root re-search cost tighter than
  the fixed-50cp/jump-to-INFINITY/discard-after-3 scheme and nets positive Elo at real clock. H1: candidate
  >= +3 Elo over base; H0: candidate <= -3.
base_commit: 801d384          # HEAD engine (T1e keep)
candidate_commit: 801d384 + output/t5.patch   # engine == 801d384 + t5; patch uncommitted per policy
base_binary_sha256: f65a3c164987ef79adcf4062ae0f872a4c2c1f899c15ac052c183f2a75b30a1c      # on-box ngn_t1e.exe (reused new-HEAD binary, hash-verified)
candidate_binary_sha256: 0a8f65b75e59c376052f83b14787b0af2eba65552cedfee6f70ce576891ca721  # output/ngn_t5.exe
harness_commit: sprt.exe sha256 30c33e0512725b7f552d8a1cf72ba6f1e0deb4dcb122c6b6ba8f6433c686a762 (M5-aware validated mill, box-staged, untouched)
command: sprt.exe -new .\ngn_t5.exe -base .\ngn_t1e.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 8000 -mingames 300 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
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
aa_preflight: standing 2026-07-03 M5-enable adjudication-ON A/A (experiments/2026-07-03-m5-adjudication-aa.md) — same TC (10+0.1) / concurrency (8) / adjudication-flag set / machine (LAN 9800X3D) / openings config, penta -1.1 [-13,+11], pLLR -0.18, 0/0 flag-outs, 77.6% adjudicated, 961 g/hr, DONE_EXIT_0; the SAME convention adopted by the KEPT T1b/T4c/T1e and SHELVED T13/T1c/T16 runs, so no fresh A/A required.
decision_rule: >
  SPRT [-3,+3] on pentanomial pLLR, bounds +/-2.94, maxgames cap 8000; pLLR >= +2.94 -> KEEP; pLLR <= -2.94
  -> SHELVE; capped at 8000g with point est >= +1, pLLR > 0, and 0 excess flag-outs -> Batch-2 PROVISIONAL
  keep (CLAUDE.md batch-certification path); capped-nonpositive -> shelve; any candidate flag-out above the
  A/A baseline (0) -> HALT and investigate.
games_or_pairs:   1309 games / 654 pairs
result:           +20.2 penta [+7,+33], pLLR +2.88 (peak +2.98 @G1302, crossed +2.94 at G1202); trinomial +20.2 [+1,+39], LLR +2.51
flags_errors:     0/0 flag-outs; 0 crash/illegal/no-move; 689 decisive + 334 draw adjudicated early
verdict:          H1 ACCEPTED (>= 3 Elo) — KEEP; pLLR peak crossed the +2.94 bound, predeclared KEEP rule fired
next_action:      committed to main as Batch-2 keep #2; nodecheck baselines re-locked 346662/149587/765656; box -> milestone multi-family gauntlet re-pin (trigger fired) -> T10 -> T7
```

## Constants chosen

| Param | Value | Meaning | Prior |
|---|---|---|---|
| `ASP_INIT` | 12 | initial half-window base (cp), Stash `~10-16` class | 10cp base +3.5/+5.1 LTC [E 0ad3477] |
| `ASP_SCORE_DIV` | 81 | score-magnitude scaling divisor (Stash `\|score\|/81`); not a tunable | window-vs-eval scaling +3.1 [S #128] |
| `ASP_MULT` | 150 | widening multiplier in percent (×1.5 per fail; +1 growth floor) | Stash ×1.30 / Ethereal +10 linear, spec range ×1.3-2 |

`ASP_INIT` and `ASP_MULT` are exposed as UCI spin / SPSA dims (TunableSearchParams: `AspInit` [4,40] def 12,
`AspMult` [110,250] def 150) for the T8 joint tune. `ASP_SCORE_DIV` is a plain const (spec exposes only the two
window knobs).

## Binaries

Cross-compiled with the record-convention command (Makefile `build` Windows line):
`GOOS=windows GOARCH=amd64 GOAMD64=v3 go build -o output/ngn_t5.exe main.go`, go1.26.2, no ldflags/trimpath.

- **Candidate** `output/ngn_t5.exe` sha256 `0a8f65b75e59c376052f83b14787b0af2eba65552cedfee6f70ce576891ca721`
  — built from clean HEAD `801d384` + `output/t5.patch` (guard `go test -short ./engine -count=1` GREEN before
  build). Tree restored after build (`git checkout -- .`, `git apply --check output/t5.patch` re-passes); patch
  stays uncommitted per policy.
- **Base** reuses the on-box `ngn_t1e.exe` sha256 `f65a3c164987ef79adcf4062ae0f872a4c2c1f899c15ac052c183f2a75b30a1c`
  — the exact `801d384` (T1e keep) engine binary already staged and hash-verified on the box; no rebuild.

## Tests (patched tree, all GREEN)

- New behavior test `TestAspirationProgressiveWidening` (engine/search_test.go): with `ASP_INIT=1`,
  `ASP_MULT=110` (1cp window, slow growth) on kiwipete at depth 8, the search fails against the tiny window many
  times and widens progressively (`fh=25 fl=77`, total 102 re-searches across depths 4-8 — far above the
  one-fail-per-depth ceiling of the old jump-to-INFINITY scheme), yet completes every iteration to depth 8 with
  `AspAbandoned=0`. Would have been discarded under the pre-T5 `maxAttempts=3` cap.
- `go test -short ./engine -count=1` → ok
- `go test -short ./... -count=1` → ok (engine, cmd/sprt, internal/uci)
- `go test -short -race ./engine -count=1` → ok

## Nodecheck (node counts EXPECTED TO MOVE — search-behavior change, NOT re-locked)

T5 reorders the root re-search, so fixed-depth node counts move from the baselines (this is the intended
width/re-search change, not a bug). Deterministic (two runs byte-identical), so the numbers are exact:

| pos | depth | T5 | baseline | Δ |
|---|---|---|---|---|
| kiwipete | 12 | 346662 | 230036 | +50.7% |
| mid | 12 | 149587 | 181982 | −17.8% |
| end | 16 | 765656 | 587557 | +30.3% |

Both-directions-by-position butterfly (the aspiration re-search cost shifts the tree). Baselines are NOT
re-locked — that happens only on a keep verdict.

## Bounds — [-3,+3] per-change gate (NOT the [0,+6] cert shape)

Isolated per-change SPRT: `-elo0 -3 -elo1 3` (H0: elo<=-3, H1: elo>=3), same shape as the KEPT T1b/T4c/T1e and
SHELVED T13/T1c/T16 runs. `alpha=beta=0.05` yields pLLR bounds +/-2.94. Expected header:
`H0: elo<=-3.0 H1: elo>=3.0 (alpha=0.05 beta=0.05 -> LLR bounds [-2.94, 2.94])`.

The revamp spec's "fixed-nodes filter → real-clock" two-stage gate is SUPERSEDED by current mill practice: the
box mill runs the per-change real-clock SPRT directly (T13/T16/T1c/T1e precedent), so T5 goes straight to the
real-clock games gate.

## Planned box run (staging + launch)

Base already on box; stage the candidate and re-verify both on-box hashes before launch:

```bash
scripts/boxsprt.sh push output/ngn_t5.exe ngn_t5.exe    # candidate — new to box
scripts/boxsprt.sh hash ngn_t5.exe        # expect 0A8F65B7…
scripts/boxsprt.sh hash ngn_t1e.exe       # expect F65A3C16…  (reused base)
```

Preflight: confirm an interactive session (`tasklist | findstr /I explorer.exe`) — boxsprt.sh's schtasks
launcher needs a logged-on session (locked is fine); NO session -> do NOT launch, report staged+blocked.

Launch (writes `run_t5.bat`, out-file `t5_out.txt`):

```bash
scripts/boxsprt.sh launch t5 -- -new '.\ngn_t5.exe' -base '.\ngn_t1e.exe' -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 8000 -mingames 300 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
```

At launch confirm: header parse, on-box SHA-256s, openings == 974e4b5a…, `ps` showing the t5 run as the ONLY
live worker (1 sprt + 8 ngn_t5 + 8 ngn_t1e), then early health (~4 min: games completing, 0/0 flag-outs).

## Launch

CONFIRMED LIVE 2026-07-19 05:13:33 EDT via `scripts/boxsprt.sh launch t5` (command exactly as above).
Topology verified: 1 sprt.exe + 8 ngn_t5 + 8 ngn_t1e, all started 05:13:33, sole live workers on the box.
On-box hashes re-verified at stage time: candidate `ngn_t5.exe` `0A8F65B7…`, base `ngn_t1e.exe` `F65A3C16…`.
Header matches the predeclared rule (`H0: elo<=-3.0  H1: elo>=3.0  (alpha=0.05 beta=0.05 -> LLR bounds
[-2.94, 2.94])`), mode real clock 10s+0.1s conc 8, openings 5000. Early health at G41 (~3.5 min): 9W-20D-12L,
elo -25.5 [-130,+80], pLLR -0.10 (early noise — CI spans ±100 at 41 games), adjudication firing
(adj-win/adj-draw/draw-rule/max-moves), 0 forfeit/flag-out/panic/illegal/crash lines, flag-outs 0/0.

## Verdict — H1 ACCEPTED, KEEP (2026-07-19)

FULL H1 accept. Final block from `t5_out.txt` (verbatim):

```text
=== RESULT (1h21m46s) ===
Games: 1309   W-D-L: 383-619-307   score: 52.9%
Elo(new - base): +20.2   95% CI [+1, +39]
LLR: +2.51   bounds [-2.94, 2.94]
Pentanomial [LL 27  LD 138  {LW,DD} 266  WD 178  WW 45] over 654 pairs
Penta Elo: +20.2   95% CI [+7, +33]   pLLR +2.88  (THE decision stat)
Flag-outs (lost on time): new 0, base 0  of 1309 games  (goal: 0)
Adjudicated early: 689 decisive, 334 draw  of 1309 games
Verdict: H1 ACCEPTED: new is stronger (>= 3 ELO)
DONE_EXIT_0
```

**Peak crossing verified (predeclared KEEP rule = pLLR >= +2.94).** Full-file parse of every pLLR line in
`t5_out.txt` (1309 game lines + final penta line): the pLLR reached/exceeded +2.94 on **9 game-lines** —
**first crossing G1202** (pLLR +2.94, elo +22.0 [+2,+42]) and G1203, then it dipped back and **sustained the
crossing G1299–G1305**, hitting the **max pLLR +2.98 at G1302** (and G1303/G1304/G1305). It then drained on the
c8 tail to +2.92 (G1307) and **+2.88 at the final 1309g** — the known c8-drain pattern (T1e/T4c precedent: the
LLR bound is touched mid-run, the last few concurrent pairs land as draws/adj and shave it back under +2.94 at
the reported endpoint). The engine's own verdict line reads H1 ACCEPTED. Predeclared rule satisfied: **KEEP.**

Verified crossing lines (from the on-box file):

```text
G1202 352W 574D 276L   53.2%  elo  +22.0 [+2,+42]  LLR  +2.53  pLLR  +2.94  adj-win     (FIRST crossing)
G1302 382W 616D 304L   53.0%  elo  +20.8 [+2,+40]  LLR  +2.57  pLLR  +2.98  adj-win     (MAX pLLR)
G1309 383W 619D 307L   52.9%  elo  +20.2 [+1,+39]  LLR  +2.51  pLLR  +2.88  adj-draw    (final, c8 drain)
```

**Hash-back (2026-07-19):** the kept tree (clean HEAD `f337819` + `output/t5.patch`) rebuilt
`GOOS=windows GOARCH=amd64 GOAMD64=v3 go build main.go` reproduces sha256
`0a8f65b75e59c376052f83b14787b0af2eba65552cedfee6f70ce576891ca721` — **bit-identical to the binary that
played.** Tests all GREEN on the patched tree (`go test -short ./engine`, `go test -short ./...`,
`go test -short -race ./engine`). Nodecheck on the patched tree reproduces the manifest's expected moved counts
**exactly and deterministically** (kiwipete d12 346662, mid d12 149587, end d16 765656) — baselines re-locked
to these in the follow-up commit (the T5 keep is a search-behavior change, not node-identical).

**Batch-2 accounting:** T5 lands on main as Batch-2 keep **#2** (row 2 in `experiments/batch-ledger.md`), a
FULL H1 accept (not a provisional). Batch-2 net claimed self-play now **T1e +6.6 + T5 +20.2 = +26.8**, 0
provisionals; Batch-2 certification due by 2026-08-01 (2-week trigger) or at 4 provisionals, whichever first.
T5 is the **biggest single keep of the revamp campaign** (prior max was T1b +13.1).
