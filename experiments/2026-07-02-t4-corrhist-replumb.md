# 2026-07-02 T4 — pawn correction-history re-plumb (mechanism fix, real-clock gate)

Pre-registered BEFORE launch. ONE coupled change (two mechanism fixes that must ship together), candidate
vs immediate base 7394668 (post-iir3 HEAD; T1a and T3a reverted). This is an eval-VALUE change => real-clock
ONLY gate (no proxy). NOT committed; diff backed up at `output/t4.patch`. Awaiting a diff review before launch.

```yaml
id: 2026-07-02-t4-corrhist-replumb
date: 2026-07-02
change_class: search/eval heuristic (eval-value: corrhist plumbing) — real-clock-ONLY gate
hypothesis: applying the pawn correction to ALL backed-up static values (qsearch stand-pat + cap-returns, matching interior nodes) AND learning it from the RAW static (not the S7 TT-refined eval) makes corrhist actually function; prior +13/+16 [S #179]
base_commit: 7394668 (post-iir3 HEAD; == ngn_base2 engine)
candidate_commit: 7394668 + output/t4.patch (engine/moveorder.go + engine/search.go + engine/qsearch_test.go)
base_binary: ngn_base2.exe sha256 85703c188be79b728909433cb91a2b673efe3b03a6bc6fabc8fce59e1cd58aa5 (already on box)
candidate_binary: ngn_t4.exe sha256 e6cc015e0173312035e0a2ff3f78eb998dfa98dd05f8fe98c29a24e27d55fe1b (cross-compiled base2 engine + output/t4.patch, GOOS=windows GOARCH=amd64 GOAMD64=v3)
mechanism_a: qsearch backs up the CORRECTED static (new `correctedStandPat` helper = raw + correctionValue) at the !inCheck stand-pat (search.go ~2305) and the qDepth cap-return (~2162 qDepth>=6 && !inCheck) — was raw eval, discarding the correction interior nodes apply at ~1353. The ply>=MaximumDepth emergency cap (~2140) stays RAW (fires before inCheck is known; never correct an in-check static). Eval cache unaffected (correction added outside it).
mechanism_b: capture corrStaticEval = the CORRECTED static (raw + correction) BEFORE the S7 TT-refinement overwrites staticEval (~1353), pass it to maybeUpdatePawnCorrection (~2063/2094) so corrhist learns (search - correctedStatic) instead of (search - ttEval) [the mis-plumbed post-S7 target the audit flagged]. CONVENTION = B (corrected, NOT raw): NGN's corrhist update is the GRAVITY form (updatePawnCorrection `e += bonus - e*|bonus|/LIMIT`, identical shape to gravityUpdate), which converges to the FULL correction only when learning from the corrected value — learning from raw would saturate the entry to the clamp limit. staticEval (raw + correction, then S7-refined) is UNCHANGED for the margin heuristics (RFP/futility/NMP/improving).
node_identity: NOT node-identical (behavioral eval-value change by design) — hence the real-clock gate.
regression_test: TestPawnCorrectionAppliedAtQsearchStandPat — poisons a pawn slot, asserts qsearch stand-pat == raw + correction. RED pre-patch (returned raw -32, wanted -8 = -32 + 24 correction), GREEN post-patch (verified by reverting just the stand-pat line). Full go test -short [-race] ./engine + -short ./... GREEN.
openings: canonical sprt_openings (5000 lines) sha256 974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222
```

## Mechanism (the coupled change)
Two fixes that only make sense together (from the 2026-07-01 audit: "pawn corrhist NEVER applied at qsearch
stand-pat, and learns vs the S7 TT-refined eval instead of raw"):

- **(a) Apply where a static becomes a backed-up value.** New `correctedStandPat(pos)` = raw eval +
  `correctionValue(stm, pawnCorrectionIndex(...))`, used at the qsearch !inCheck stand-pat and the qDepth
  cap-return. Before T4, qsearch backed up the RAW eval — so ~42% of nodes (qsearch) discarded the
  correction the interior nodes apply. Now the whole tree backs up corrected statics. The
  ply>=MaximumDepth emergency cap stays RAW (fires before inCheck is known; an in-check static must never
  be corrected).
- **(b) Learn from the pre-S7 CORRECTED static (convention B).** `corrStaticEval` = `staticEval` captured
  right after the interior application (raw + correction) and BEFORE S7 overwrites it, passed to
  `maybeUpdatePawnCorrection`. Before T4 the learn target was the post-S7 `staticEval` (= ttEval when the TT
  bracketed), so on a TT hit the correction learned `bestScore - ttEval` ≈ 0 and barely updated. NGN's
  corrhist update is the GRAVITY form (same shape as `gravityUpdate`), so it converges to the FULL
  correction ONLY when learning from the corrected value: bonus = search - (raw + entry) shrinks to 0 as
  entry → the correction (an EMA-toward-(search-raw)). Learning from raw (bonus = search - raw, constant)
  would instead saturate the entry to ±LIMIT. The margin path is untouched: `staticEval` still = raw +
  correction then S7-refined, used for RFP/futility/NMP.

## Decision rule (predeclared, real-clock ONLY)
Real-clock 10+0.1 c8 SPRT vs `ngn_base2`, elo0=-3 elo1=+3, alpha=beta=0.05 (pLLR bounds ±2.94), adjudication
OFF, on the A/A-validated c8 config, maxgames 8000 mingames 300. pLLR ≥ +2.94 KEEP; ≤ -2.94 REJECT;
capped-positive (point ≥ +1, pLLR > 0, no regression) → PROVISIONAL keep #2 under batch certification;
capped-nonpositive → shelve; any candidate flag-outs above the A/A baseline (0) → HALT + investigate.
Prior +13/+16 [S #179]. Command (candidate = value-baked cross-compile `ngn_t4.exe`):
```
sprt.exe -new ngn_t4.exe -base ngn_base2.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 8000 -mingames 300
```

**Scope note:** the NON-pawn corrhist follow-on (prior +26/+27 [S], but REAL post-reset rejections
-9.5/1569g and -3.8/3000g on the OLD mis-plumbed base) is a SEPARATE later item, gated behind T4 KEEPING
and held to a strict gate — NOT part of this change.

## Review resolutions (2026-07-02)
- **Item 1 (learn target):** RESOLVED to convention B — NGN's corrhist update is the gravity form, so it
  learns from the pre-S7 CORRECTED static (`corrStaticEval`), not raw. Learning from raw would saturate the
  entry to the clamp limit (see mechanism_b).
- **Item 2 (MaximumDepth cap):** RESOLVED — the `ply >= MaximumDepth` emergency cap-return (~2140) is left
  RAW. It fires before `inCheck` is computed and an in-check static must never be corrected; it is a rare
  emergency cap where raw is acceptable and simplest. 2162 and 2305 are already on !inCheck paths.

## Status
KEPT (2026-07-02). Cross-compiled `ngn_t4.exe` (sha256 e6cc015e…), launched real-clock c8 on the LAN box,
WON the SPRT — H1 ACCEPTED, penta **+9.1 [+0,+18]**, pLLR +2.80 final (crossed +2.94 at G2317), 0/0
flag-outs, 2325g (see the Verdict section below). Committed to main in a single commit (engine +
regression test + run records); patch retained at `output/t4.patch`. Full suite green (short + race) at
commit time. First outright SPRT accept of the campaign.

## Verdict (2026-07-02) — KEEP (H1 ACCEPTED)

Launched 06:51:55 box time via `scripts/boxsprt.sh` on the 9800X3D LAN box (192.168.4.108, native
Windows). The real-clock c8 SPRT crossed the H1 pLLR bound and was ACCEPTED. Run-execution facts (the
pre-registration block above holds hypothesis / mechanism / decision rule):

```yaml
harness_commit: scripts/boxsprt.sh (box mill wrapper; unchanged during the run)
command: sprt.exe -new ngn_t4.exe -base ngn_base2.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 8000 -mingames 300
machine: AMD Ryzen 7 9800X3D 8c/16t, native Windows (LAN box 192.168.4.108)
go_version: local toolchain (cross-compile)
goarch_goamd64: windows/amd64 GOAMD64=v3
tc: 10+0.1 (seconds; bullet)
concurrency: 8
base_binary_sha256: 85703c188be79b728909433cb91a2b673efe3b03a6bc6fabc8fce59e1cd58aa5 (ngn_base2.exe; base2 == HEAD engine, git diff 6959a14..HEAD engine/ empty)
candidate_binary_sha256: e6cc015e0173312035e0a2ff3f78eb998dfa98dd05f8fe98c29a24e27d55fe1b (ngn_t4.exe)
openings_sha256: 974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222
adjudication: OFF
aa_preflight: the M2 c8 A/A at this exact TC/concurrency/machine (aa_c8_out.txt, ngn_base2 vs itself, penta -0.7 [-11,+9], 0/0 flag-outs, DONE_EXIT_0) — completed clean before T1a, covers this config
games_or_pairs: 2325 games / 1162 pairs (stopped at the H1 bound, before the 8000 cap)
result: penta [LL 36 LD 252 {LW,DD} 538 WD 287 WW 49]; penta Elo +9.1 [+0,+18] pLLR +2.80; trinomial +9.1 [-5,+23] LLR +2.62; W-D-L 499-1388-438 (51.3%); duration 2h31m56s
flags_errors: flag-outs new 0 / base 0 of 2325 (goal 0); no crashes/illegal/no-move; DONE_EXIT_0
verdict: KEEP (H1 ACCEPTED: new is stronger, >= 3 Elo; pLLR crossed the +2.94 bound)
next_action: KEPT into main (single commit, engine + test + records); T4-lane follow-up = nonpawn corrhist retest on the fixed plumbing, STRICT gate (H1-only keep, NO provisional-on-cap)
```

Verbatim RESULT (`t4_out.txt` tail):

```
=== RESULT (2h31m56s) ===
Games: 2325   W-D-L: 499-1388-438   score: 51.3%
Elo(new - base): +9.1   95% CI [-5, +23]
LLR: +2.62   bounds [-2.94, 2.94]
Pentanomial [LL 36  LD 252  {LW,DD} 538  WD 287  WW 49] over 1162 pairs
Penta Elo: +9.1   95% CI [+0, +18]   pLLR +2.80  (THE decision stat; trinomial above is secondary)
Flag-outs (lost on time): new 0, base 0  of 2325 games  (goal: 0)
Verdict: H1 ACCEPTED: new is stronger (>= 3 ELO)
DONE_EXIT_0
```

**Crossing + drain note.** The SPRT stop crossed at G2317 (pLLR +2.98), held at G2318 (+2.98) and G2319
(+2.94, still ≥ the +2.94 H1 bound); the ~6 in-flight games then drained (concurrency 8), which is why
the FINAL printed pLLR is +2.80 rather than the crossing value. The penta point estimate is a stable +9.1
across the crossing and drain. Flag-outs 0/0 across all 2325 games — a genuine strength gain, not a timing
artifact. Predeclared decision rule (verbatim): pLLR ≥ +2.94 → KEEP outright; ≤ −2.94 → REJECT (corrhist
genuinely downstream of search); capped-positive (point ≥ +1, pLLR > 0, no regression) → batch provisional
#2; capped-nonpositive → shelve; flag-out spike → HALT. **Outcome: KEEP outright — NOT a batch provisional
(the +2.94 bound was crossed).**

**Monitoring note.** Mid-run the Mac lost LAN connectivity to the box (monitoring outage only); the run
completed unattended on-box and self-terminated DONE_EXIT_0. Post-run box `ps` clean (no leaked workers).

**Learn-target convention (confirmed by the KEEP).** The revamp-doc T4 spec said "learn against RAW
static". That is correct ONLY for EMA-toward-diff update forms. NGN's `updatePawnCorrection`
(moveorder.go ~56-69) is the GRAVITY form (`*e += bonus − (*e)·|bonus|/corrHistLimit`, byte-identical to
`gravityUpdate`), which self-decays toward an equilibrium — so it must learn from the CORRECTED value
(pre-S7 `corrStaticEval` = raw + correction) or entries saturate at ±corrHistLimit and go magnitude-blind.
Shipped = learn-from-corrected (convention B, mechanism_b above). The +9.1 KEEP validates the coupled
change: apply the correction at the qsearch stand-pat AND learn from the corrected pre-S7 eval.
