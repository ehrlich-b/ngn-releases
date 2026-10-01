# 2026-07-03 T4c — minor-piece correction-history add on the fixed plumbing (real-clock, batch-cert gate)

Pre-registered BEFORE launch. ONE change: add the standard MINOR-piece correction-history table
(third corrector) on top of the KEPT pawn (T4, +9.1) and non-pawn (T4np, +6.4) tables. Candidate vs
immediate base `35c0c0a` (T1b keep; HEAD). This is an eval-VALUE change => real-clock ONLY gate (no
proxy). NOT committed; diff backed up at `output/t4c.patch`. Awaiting coordinator GO before launch
(box runs M6 first).

```yaml
id: 2026-07-03-t4c-minor-corrhist
date: 2026-07-03
change_class: search/eval heuristic (eval-value: minor-piece corrhist add on fixed T4 plumbing) — real-clock-ONLY gate
hypothesis: a third correction-history table keyed on each side's MINOR-piece (knight+bishop) placement — a finer key than the non-pawn table (which lumps every non-pawn piece incl. rooks/queens/king into one slot) — captures recurring minor-configuration eval errors (outposts, bad bishops, knight-vs-bishop imbalances) the coarse non-pawn key aliases away, riding the SAME fixed T4 plumbing (apply at qsearch stand-pat + interior static, learn from the pre-S7 corrected static via the gravity update). No minor-specific measured prior exists (see Honest prior); the family is 2-for-2 on fixed plumbing but stacks on one eval, so diminishing returns are expected.
base_commit: 35c0c0a (T1b keep; == the box's staged ngn_t1b.exe engine)
candidate_commit: 35c0c0a + output/t4c.patch (engine/moveorder.go + engine/search.go + engine/qsearch_test.go + engine/reentrancy_test.go)
patch_sha256: 2e94c3b6b786881d8eb787f4236121cfec8da06a8a62b126a7504b2551545d05 (output/t4c.patch, verified byte-identical to the live working-tree `git diff`)
base_binary: ngn_t1b.exe sha256 0fede586b4ae312f13d7752fe137a542b41cbdf6ee94eeac5988ebd1e63eadea (box-staged; REPRODUCED locally byte-identical by cross-compiling clean 35c0c0a — T1b was engine-code-only vs this binary's source, so ngn_t1b.exe serves as the base at launch, no re-staging)
candidate_binary: ngn_t4c.exe sha256 40783f0444ed47cf50bf9ca0dfa5d7635536e1a1519fe47c902498665141a1f8 (cross-compiled 35c0c0a + output/t4c.patch, GOOS=windows GOARCH=amd64 GOAMD64=v3, go1.26.2, no ldflags/trimpath — same toolchain/flags that reproduced the base byte-for-byte)
mechanism: a third correction-history table `minorCorrectionHistory[2][corrHistSize]` keyed on `minorCorrectionIndex(whiteMinors, blackMinors)` where minors = knight|bishop per side (NOT king, unlike non-pawn). Summed into the corrected static everywhere the pawn and non-pawn corrections already apply — the qsearch stand-pat (via correctedStandPat, covering both qsearch call sites) and the interior static-eval site — and learned from the pre-S7 `corrStaticEval` (raw + pawn + non-pawn + minor) via the byte-identical gravity update. Distinct splitmix64 finalizer constants (0xBF58476D1CE4E5B9 / 0x94D049BB133111EB), a third pair distinct from pawn and non-pawn, so all three tables alias independently.
node_identity: NOT node-identical (behavioral eval-value add by design) — hence the real-clock gate.
regression_test: TestMinorCorrectionAppliedAtQsearchStandPat — poisons a minor slot (pawn and non-pawn slots stay zero), asserts qsearch stand-pat == raw + minor correction. RED pre-patch (returned raw -28, wanted -4 = -28 + 24, proven by removing the minor term from correctedStandPat: `qsearch_test.go:173: qsearch stand-pat = -28, want raw -28 + minor correction 24 = -4`); GREEN post-patch. Position holds knights so the minor key is a real hashed slot, not the degenerate empty-minor slot 0. Full go test -short [-race] ./engine + -short ./... GREEN.
openings: canonical sprt_openings (5000 lines) sha256 974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222
```

## Which historical form was ported, and why

Same provenance approach as T4np: the ORIGINAL minor corrector was never committed — it lived in the
reverted working tree alongside the non-pawn corrector. RECOVERED VERBATIM from the dangling git blobs
`f0427c11` (moveorder.go) and `8b0bcc6c` (search.go), the exact blobs T4np mined for the non-pawn form.
That tree carried non-pawn AND minor; T4np shipped non-pawn only (deferring minor as a separate item to
respect minimal-variable). T4c now ports the minor corrector on the fixed T4 plumbing.

Ported = the original @6245 full-weight minor form:
- Table: `var minorCorrectionHistory [2][corrHistSize]int` (same size/limit as pawn and non-pawn).
- Key: `minorCorrectionIndex(whiteMinors, blackMinors)` = hash of each side's knight|bishop occupancy
  (`pieces[WhiteKnight]|pieces[WhiteBishop]`, `pieces[BlackKnight]|pieces[BlackBishop]`; king EXCLUDED,
  distinct from the non-pawn key which includes the king), with a third distinct splitmix64 constant pair
  so it aliases independently of both other tables. **Key kept identical to the original blob.**
- Weight: `* 6245 / 131072` (== the pawn and non-pawn weight; saturated entry <= ±49cp).
- Update: byte-identical gravity form to `updatePawnCorrection`/`updateNonPawnCorrection`;
  `maybeUpdateMinorCorrection` has the same in-check / capture-bestmove / directional-signal guards.

**The one plumbing adaptation vs the blob.** The recovered search.go blob passed `staticEval` (the OLD
mis-plumbed, S7-corruptible learn target) to `maybeUpdateMinorCorrection`. On the fixed plumbing the minor
update passes `corrStaticEval` (the pre-S7 CORRECTED static), exactly matching the KEPT pawn and non-pawn
update sites — this is the whole point of the fixed plumbing, and learning from the corrected value is what
the gravity form requires (learning from raw saturates entries to the clamp limit). The interior
application already flows into `corrStaticEval` because the minor term is summed into `staticEval` one line
before it is captured.

## Mechanism (diff summary — 4 files; output/t4c.patch)

- **engine/moveorder.go:** add `minorCorrectionHistory` + `minorCorrectionIndex` (`:163`) /
  `minorCorrectionValue` (`:169`) / `updateMinorCorrection` (`:173`) / `maybeUpdateMinorCorrection`
  (`:188`) — verbatim from blob `f0427c11` with the third distinct constant pair; extend
  `correctedStandPat` (`:65`) to add the minor term (this is the qsearch application — covers both qsearch
  call sites, the qDepth cap-return search.go:2172 and the stand-pat search.go:2315); add the minor clear
  loop to `ClearHistoryTable` (`:355`).
- **engine/search.go:** declare `minorCorrIdx` (`:1346`); compute it in the `!inCheck` branch (`:1361`);
  sum `minorCorrectionValue` into `staticEval` at the interior site (`:1362`) — so `corrStaticEval`
  (captured the next line) becomes raw + pawn + non-pawn + minor, the learn target all three tables share;
  add `maybeUpdateMinorCorrection(... corrStaticEval ...)` at both update sites (fail-high `:2075` and
  node-complete `:2108`).
- **engine/qsearch_test.go:** add `TestMinorCorrectionAppliedAtQsearchStandPat` (red→green guard).
- **engine/reentrancy_test.go:** TestIIDIsWired fragile-FEN fix (test-only, ZERO engine-binary impact — the
  base binary reproduced byte-identical WITH this change stashed, confirming test files do not affect the
  build). The prior single FEN stopped firing IID under the T4c eval shift (IIDSearches=0 → on==off node
  counts spuriously), so the test was pointed at a FEN that still exercises IID and given an explicit
  `on.IIDSearches>0` precondition guard so a future eval shift fails loudly with the real cause instead of a
  misleading "re-entrancy regressed." IID PROVEN still wired: it fires 2× on the new FEN (on 19155 ≠ off
  19387) and on 3 other probed positions. Same class as the nodecheck-baseline refreshes every prior
  corrhist keep required — an eval-value change legitimately moves node-count-sensitive positions.

Convention alignment (the family invariant): the minor term is applied at the qsearch stand-pat (via
`correctedStandPat`) and the interior static-eval site, and learned from the pre-S7 `corrStaticEval`,
exactly matching the KEPT pawn and non-pawn terms. The `ply>=MaximumDepth` emergency cap stays RAW
(untouched).

## Bundled test-only change — TestIIDIsWired FEN refresh (EXPLICIT callout)

One bundled change beyond the corrhist wiring, in `engine/reentrancy_test.go` — TEST-ONLY, ZERO
engine-binary impact (proven: the clean-35c0c0a base binary reproduced byte-identical
`0fede586…` == the box's `ngn_t1b.exe` WITH this test edit stashed, so test files do not enter the
build). Coordinator-approved as bundled; call out in the commit sentence if T4c keeps
("+ IID test FEN refresh" or similar).

- **What broke:** `TestIIDIsWired` asserts IID-on ≠ IID-off node counts on ONE hardcoded FEN. The T4c
  eval-value shift moved that FEN out of the IID-firing regime (IIDSearches → 0, so on==off SPURIOUSLY).
  Same class as the nodecheck-baseline refreshes every prior corrhist keep required — an eval change
  legitimately moves node-count-sensitive positions. `reentrancy_test.go` was last touched pre-corrhist;
  T4/T4np's eval shifts happened to keep the FEN firing, the minor term tipped it over.
- **Proof IID is still wired (NOT a real regression):** IID fires on 3 other probed positions
  (Kiwipete 3×, a Sicilian 2×, a Najdorf-ish middlegame 2×), all on≠off. Only the one hardcoded FEN
  went quiet.
- **The fix:** point the test at a FEN that still fires IID under the corrected eval (fires 2×, on 19155
  ≠ off 19387) AND add an explicit `on.IIDSearches>0` precondition guard. The node-count-differ check
  stays the PRIMARY guard (the re-entrancy bug's signature is "IID runs but changes nothing" — identical
  counts despite IIDSearches>0, which `IIDSearches>0` alone cannot catch). The added precondition makes a
  FUTURE eval shift fail LOUDLY with the true cause ("FEN no longer exercises IID") instead of the
  misleading "re-entrancy regressed" — a real robustness improvement, not a weakening.

## Honest prior (explicit: NO minor-specific measured number)

The revamp doc (`2026-07-01-method-revamp.md` Part 3 T4) has **NO minor-piece-specific corrhist number.**
Its measured corrhist priors are pawn (**+13.1/+16.0 LTC** [S #179], +46 [Sim]) and non-pawn
(**+26.3/+27.4 LTC** [S #201], +19.7 [Sim]) only. There is no direct measured prior for a minor corrector
in the mined ledgers.

- **Nearest measured analog:** the non-pawn corrector, a coarser key over a superset of the same material
  (+26.3/+27.4 [S #201]). But that is a STANDALONE prior.
- **Most relevant LOCAL prior:** T4c is the THIRD corrector, stacking on pawn (+9.1 kept, T4) and non-pawn
  (+6.4 kept, T4np) already in HEAD. The realized NGN family trend is **pawn +9.1 → non-pawn +6.4 —
  diminishing** as each table captures signal the prior ones already absorbed, and the minor key is a
  SUBSET (knight+bishop) of the non-pawn material the second table already keys on. The TRIED-LEDGER
  explicitly warns the family "stacks on ONE eval, so watch for diminishing returns / interaction when T4c
  lands (certify via games, not by assuming additivity)."
- **Honest expectation:** low-single-digit, plausibly neutral — below the +6.4 non-pawn add. Consistent
  with the expectation-calibration rule (demonstrated curve +2-8/item at ~60% transfer; no 3-digit
  promises). This is precisely why the gate is the standard batch-cert rule (provisional-on-cap allowed),
  NOT T4np's strict H1-only rule: T4np needed strict to overturn adverse post-reset priors; T4c has NO
  adverse prior (the minor corrector was built + suite-green but never game-tested) and the family is
  2-for-2, so a capped-positive is legitimate provisional evidence.

## Decision rule (predeclared — batch-certification shape, real-clock ONLY)

Real-clock 10+0.1 c8 SPRT vs `ngn_t1b.exe` (= clean 35c0c0a), elo0=-3 elo1=+3, alpha=beta=0.05 (pLLR
bounds ±2.94), adjudication STANDARD-ON, on the A/A-validated c8 adjudication-ON config, maxgames 8000
mingames 300.

**PREDECLARED RULE (verbatim):** "pLLR ≥ +2.94 → KEEP outright (H1 accept). pLLR ≤ −2.94 → REJECT.
Capped-positive at the 8000g cap (penta point est ≥ +1, pLLR > 0, no regression signal) → PROVISIONAL keep
under batch certification (batch #1 slot). Capped-nonpositive → SHELVE with reopen condition. Any candidate
flag-out spike above the A/A baseline (>2 either side) → HALT and investigate before verdicting."

## Harness / command / machine (to be finalized at launch)

```yaml
harness_commit: scripts/boxsprt.sh (box mill wrapper; unchanged during the run) + the validated mill sprt.exe (13560dc, sha 30c33e05…, adjudication-capable)
command: sprt.exe -new .\ngn_t4c.exe -base .\ngn_t1b.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 8000 -mingames 300 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
machine: AMD Ryzen 7 9800X3D 8c/16t, native Windows (LAN box 192.168.4.108)
go_version: go1.26.2 (local cross-compile)
goarch_goamd64: windows/amd64 GOAMD64=v3
tc: 10+0.1 (seconds; bullet)
concurrency: 8
adjudication: STANDARD-ON (-resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80; enabled 2026-07-03 after the M5-enable A/A validated it non-distorting)
aa_preflight: the M5-enable A/A at this exact TC/concurrency/machine/adjudication-config (2026-07-03, adjudication ON 10+0.1 c8, penta -1.1 [-13,+11] at cap, 0/0 flag-outs, 77.6% adjudicated, DONE_EXIT_0 — validates the c8 adjudication-ON config; coordinator to decide at launch whether a fresh ngn_t1b-vs-ngn_t1b A/A is wanted)
openings_sha256: 974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222
```

## Status

DEV COMPLETE, awaiting coordinator GO (box runs M6 first). NOT committed (speculative diff stays
uncommitted until verdict). Diff backed up at `output/t4c.patch` (sha256 2e94c3b6…, byte-identical to the
live tree diff). Candidate `ngn_t4c.exe` (sha256 40783f04…) cross-compiled; base reproduced byte-identical
to the box's `ngn_t1b.exe` (0fede586…). Full suite green (short + short-race ./engine, short ./...) on the
patched tree.

2026-07-06 wait-time prep while the gauntlet re-pin was running: `go test -short -race ./... -count=1`
passed locally; `scripts/boxsprt.sh` was approved as the persistent box SPRT wrapper; `ngn_t4c.exe` was
pushed to `C:\Users\ehrli\ngn\sprt\ngn_t4c.exe`; on-box hashes verified:
`ngn_t4c.exe` = `40783F0444ED47CF50BF9CA0DFA5D7635536E1A1519FE47C902498665141A1F8`,
`ngn_t1b.exe` = `0FEDE586B4AE312F13D7752FE137A542B41CBDF6EE94EEAC5988EBD1E63EADEA`.

## Launch

CONDITIONAL GO granted 2026-07-03 (coordinator). Box order tonight: M6 (running) → gauntlet re-pin
(t1b-runner, ~4h) → T4c (deep-night slot). Launch autonomously — no further round-trip — once BOTH:
(a) t1b-runner's gauntlet reaches DONE and `scripts/boxsprt.sh ps` is empty (handoff coordinated directly
with t1b-runner, which is instructed to hand off the box); (b) staging verifies — push `ngn_t4c.exe`,
confirm on-box sha256 == 40783f04…; base = the already-staged `ngn_t1b.exe` (0fede586…, byte-identical to
clean 35c0c0a — NO re-push).

**A/A preflight (no fresh preflight; coordinator-confirmed):** cite the M5-enable adjudication-ON A/A —
2026-07-03, adjudication ON, 10+0.1 c8 on the same LAN box / harness config, penta -1.1 [-13,+11] at cap,
0/0 flag-outs, 77.6% adjudicated, DONE_EXIT_0 (`experiments/2026-07-03-m5-adjudication-aa.md`). Same
TC/concurrency/machine/harness as this run; validates the c8 adjudication-ON config (same basis T1b used).

At launch, fill in: timestamp (box + Mac EDT), the command verbatim from the run `.bat`, the confirmed
on-box binary SHA-256s, openings on box == 974e4b5a… (5000 lines), header parse (`mode: real clock
10s+0.1s (concurrency 8)`, `H0: elo<=-3.0 H1: elo>=3.0 (alpha=0.05 beta=0.05 -> LLR bounds [-2.94,
2.94])`, adjudication flags echoed), and confirm `ps` shows the T4c run as the ONLY live worker. Then arm
a DONE_EXIT watcher (~9h cap; re-arm if the environment kills it — the on-box run is durable).

### Launched (2026-07-08)

**LAUNCHED 2026-07-08 21:39:32 box time** (`ps` StartTime 9:39:33 PM box; Mac-observed the same minute over
LAN) via `scripts/boxsprt.sh launch t4c`, after the gauntlet re-pin (`repin_0704`) reached DONE and
`scripts/boxsprt.sh ps` showed the box idle. Command verbatim from the run batch:

```text
sprt.exe -new .\ngn_t4c.exe -base .\ngn_t1b.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 8000 -mingames 300 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
```

On-box SHA-256s re-verified at launch:
- `ngn_t4c.exe` = `40783F0444ED47CF50BF9CA0DFA5D7635536E1A1519FE47C902498665141A1F8` (candidate)
- `ngn_t1b.exe` = `0FEDE586B4AE312F13D7752FE137A542B41CBDF6EE94EEAC5988EBD1E63EADEA` (base = clean 35c0c0a)
- `sprt.exe`    = `30C33E0512725B7F552D8A1CF72BA6F1E0DEB4DCB122C6B6BA8F6433C686A762` (13560dc validated mill)

Header parse confirmed: `mode: real clock 10s+0.1s (concurrency 8)`; `H0: elo<=-3.0 H1: elo>=3.0
(alpha=0.05 beta=0.05 -> LLR bounds [-2.94, 2.94])`; adjudication line `resign>=900cp/5p draw<=10cp/10p>=80p`
echoed. `ps` showed exactly one `sprt` + 8 `ngn_t4c` + 8 `ngn_t1b` processes (the T4c run as the only live
worker). Patch integrity re-verified at launch: the live working-tree `git diff -- engine/` sha256 ==
`output/t4c.patch` == `2e94c3b6b786881d8eb787f4236121cfec8da06a8a62b126a7504b2551545d05`. A DONE_EXIT
watcher is armed Mac-side (re-arm if the environment kills it — the on-box run is durable). **Verdict TBD.**

## Verdict

**KEEP outright — H1 ACCEPTED** (coordinator-classified 2026-07-09 per the predeclared rule: pLLR +2.98 ≥
+2.94). Completed 2026-07-09, `DONE_EXIT_0`, duration **8h07m04s**. The corrhist family is now **3-for-3 on
the fixed plumbing** (pawn +9.1, non-pawn +6.4, minor +3.7 — diminishing as predicted, still H1).

RESULT block (verbatim from `output/t4c_out.txt`):

```text
=== RESULT (8h7m4s) ===
Games: 7842   W-D-L: 2178-3569-2095   score: 50.5%
Elo(new - base): +3.7   95% CI [-4, +11]
LLR: +2.63   bounds [-2.94, 2.94]
Pentanomial [LL 227  LD 926  {LW,DD} 1544  WD 985  WW 239] over 3921 pairs
Penta Elo: +3.7   95% CI [-2, +9]   pLLR +2.98  (THE decision stat; trinomial above is secondary)
Flag-outs (lost on time): new 0, base 0  of 7842 games  (goal: 0)
Adjudicated early: 4256 decisive, 1830 draw  of 7842 games
Verdict: H1 ACCEPTED: new is stronger (>= 3 ELO)
DONE_EXIT_0
```

**Integrity sweep (full 7842-game `t4c_out.txt`):**
- **Bound crossing + c8 drain:** pLLR first reached the +2.94 upper bound at **G7828**, oscillated around it
  (concurrency-8 in-flight pairs draining), **peaked +3.01 at G7840-G7841**, and settled **+2.98 at G7842**
  (the terminal game). LLR +2.63 at stop. This is the expected c8 drain, not an ambiguous crossing.
- **No reject-bound touch anywhere:** ZERO lines with pLLR ≤ −2.94 across the whole run (checked) — no
  double-crossing, no earlier flirt with H0.
- **Clean terminations:** the only result reasons in 7842 games are adj-win 4256, adj-draw 1830, max-moves
  914, draw-rule 824, checkmate 17, stalemate 1 (= 7842). **0 crash, 0 illegal-move, 0 no-move, 0
  time-forfeit / disconnect.**
- **Flag-outs new 0 / base 0** — no spike vs the M5-enable A/A baseline (0/0); the HALT trigger (>2 either
  side) never approached.
- **Adjudication:** 4256 decisive + 1830 draw fired cleanly (both kinds), 77.6% adjudicated, matching the
  validated c8 adjudication-ON profile.

**Binary integrity (hash-back):** a fresh `make build` of the KEPT tree (35c0c0a + this diff) produced
`build/ngn.exe` sha256 `40783F0444ED47CF50BF9CA0DFA5D7635536E1A1519FE47C902498665141A1F8` — **byte-identical
to the box-run `ngn_t4c.exe`**, so the committed HEAD == the exact engine that played the SPRT.

**Baseline tests (kept tree):** `go test -short ./engine` ok, `go test -short -race ./engine` ok, `go test
-short ./...` ok (engine + cmd/sprt + internal/uci all green).

**Nodecheck re-lock (separate follow-up commit, per the 9c15bc7 precedent):** T4c is non-node-identical, so
the fixed-depth `scripts/nodecheck.sh` baselines moved and were re-locked — kiwipete d12 371112→230036 (−38%),
mid d12 124266→181982 (+46%), end d16 294316→587557 (+100%); reproduced identical on two runs (deterministic).
Single-position node shape, NOT a strength signal; the real-clock SPRT above owns the strength verdict.

**Box hygiene:** `scripts/boxsprt.sh ps` empty at fetch time — the run self-cleaned on `DONE_EXIT_0`, no
stray workers.
