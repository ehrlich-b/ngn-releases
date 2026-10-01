# 2026-07-09 T13 (re-scoped) — qsearch stand-pat fail-high TT store (real-clock, batch-cert gate)

Pre-registered BEFORE launch. ONE change: on the qsearch **stand-pat fail-high** (`standPat >= beta`,
the corrected stand-pat), store a depth-0 `LowerBound` TT entry (value = `standPat`, `EmptyMove`) before
the fail-hard `return beta`. Candidate vs immediate base `c9dd9d2` (T4c keep; == the box's staged
`ngn_t4c.exe`). This is a TT-store behavior add (non-node-identical) => real-clock ONLY gate (no proxy).
NOT committed; diff backed up at `output/t13.patch`.

## Re-scope note (READ FIRST — the original T13 premise was wrong)

The 2026-07-08 audit that promoted T13 stated "qsearch writes NOTHING to the TT today — probes only — so a
stand-pat≥beta LowerBound store is the FIRST qsearch TT write." **That is false.** qsearch has stored the
CAPTURE fail-high to the TT since `a0fdda41` (2026-05-30, the "S3" store) — see `engine/search.go:2401`
(`TranspositionTable.Set(pos.Hash(), bestMove, scoreToTT(beta, ply), 0, LowerBound)`), the counter
`info.QSearchTTStores` (`search.go:536`, printed at `uci.go:834`), and the existing proving test
`TestQSearchTTStoresWired` (`see_test.go:205`). So T13 is the **SECOND qsearch store site**, not the first.
The genuinely-missing store is the STAND-PAT fail-high path (`search.go:2319-2323`), which returned `beta`
without storing. This manifest implements that store only. Coordinator confirmed the re-scope and the
prior downgrade (option A).

```yaml
id: 2026-07-09-t13-qsearch-ttstore
date: 2026-07-09
change_class: search heuristic (TT-store behavior add: qsearch stand-pat fail-high LowerBound store) — real-clock-ONLY gate (non-node-identical)
hypothesis: caching the qsearch stand-pat fail-high as a depth-0 LowerBound lets a later probe of the SAME position cut immediately in qsearch (case LowerBound: if ttEval>=beta return ttEval, probe path is depth-blind so depth-0 entries ARE usable) instead of recomputing the corrected stand-pat. NGN already stores the capture fail-high; this fills the stand-pat gap (Stash/SF store both). Counterweight measured up front (see node_identity): the stand-pat cut is the MOST FREQUENT qsearch exit, so depth-0 store volume rises sharply — the flood can evict deeper main-search entries (the direct-mapped, depth-preferred replacement at cache.go:144-152 is the guard) AND, via the un-depth-gated S7 refinement at search.go:1368-1374, the new depth-0 LowerBound entries can raise main-search's pruning static-eval, reshaping RFP/futility/NMP. Whether the cutoff/time savings outweigh the flood + margin reshaping is exactly what this SPRT decides.
base_commit: c9dd9d2 (engine-identical to a195717 = T4c keep; c9dd9d2 only re-locked nodecheck baselines, no engine change)
candidate_commit: c9dd9d2 + output/t13.patch (engine/search.go + engine/qsearch_test.go)
patch_sha256: dbd8664139b3c4a5c852306207c39518496e7dd625349185938b11eaa7ade7bf (output/t13.patch, byte-identical to the live working-tree `git diff` at patch time)
base_binary: ngn_t4c.exe sha256 40783F0444ED47CF50BF9CA0DFA5D7635536E1A1519FE47C902498665141A1F8 (box-staged, byte-verified == HEAD engine at the T4c keep — NO re-push; it IS the current HEAD engine)
candidate_binary: ngn_t13.exe sha256 5f9b383e1d39893bc105d06601d2842627a9bf0eea514adcf1566b5926c9fd1b (cross-compiled c9dd9d2 + output/t13.patch via `make build`, GOOS=windows GOARCH=amd64 GOAMD64=v3, go1.26.2 darwin/arm64 host, no ldflags/trimpath — same recipe/toolchain that reproduced the T4c base byte-for-byte)
mechanism: engine/search.go:2320-2327 — on `standPat >= beta`, before `return beta`, add `TranspositionTable.Set(pos.Hash(), EmptyMove, scoreToTT(standPat, ply), 0, LowerBound); info.QSearchTTStores++`. Stores `standPat` (a tighter, sound lower bound than `beta`, since standPat >= beta; SF/Stash convention) not `beta`. bestMove = EmptyMove (no move caused the cut). Mirrors the existing capture fail-high store's conventions (depth 0, scoreToTT mate-adjust, LowerBound). Fail-hard `return beta` unchanged. The in-check qsearch path does NOT stand pat (guarded at search.go:2226) so it never reaches this store, and the IsStopRequested guard (search.go:2163) returns earlier, so a stopped search never stores here (the stand-pat value is a local static eval, not derived from any aborted child — no reset-invariant concern).
node_identity: NOT node-identical (behavioral TT-store add by design) — hence the real-clock gate. Fixed-depth nodecheck MOVED (recorded, NOT re-locked pending verdict): kiwipete d12 230036 -> 320210 (+39%), mid d12 181982 -> 206463 (+13%), end d16 587557 -> 798102 (+36%). All three UP — consistent with the flood + S7 margin-reshaping counterweight above (the new depth-0 entries add TT traffic and feed main-search pruning margins). Reproduced deterministically. This is a real-clock HEADWIND to note (more nodes/depth = shallower search per unit time) that the qsearch stand-pat cutoff/time savings must overcome — the SPRT is the authority.
regression_test: TestStandPatFailHighStoresLowerBound (engine/qsearch_test.go) — a quiet position forced to stand-pat fail-high (beta = standPat-1) must leave a probeable depth-0 LowerBound entry with eval == scoreToTT(standPat), move == EmptyMove. RED pre-patch ("stand-pat fail-high left no TT entry (T13 store missing)" — verified by running the test against the un-patched engine), GREEN post-patch. Existing TestQSearchTTStoresWired (the capture-store guard) still passes. Full go test -short [-race] ./engine + -short ./... GREEN.
harness_commit: scripts/boxsprt.sh (box mill wrapper; unchanged during the run) + the validated mill sprt.exe (13560dc, sha 30C33E0512725B7F552D8A1CF72BA6F1E0DEB4DCB122C6B6BA8F6433C686A762, adjudication-capable)
command: sprt.exe -new .\ngn_t13.exe -base .\ngn_t4c.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 8000 -mingames 300 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
machine: AMD Ryzen 7 9800X3D 8c/16t, native Windows (LAN box 192.168.4.108)
go_version: go1.26.2
goarch_goamd64: windows/amd64 GOAMD64=v3
tc: 10+0.1 (seconds; bullet)
concurrency: 8
openings: canonical sprt_openings (5000 lines)
openings_sha256: 974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222
adjudication: STANDARD-ON (-resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80)
aa_preflight: cite experiments/2026-07-03-m5-adjudication-aa.md — the M5-enable A/A at this exact TC/concurrency/machine/adjudication config (2026-07-03, adjudication ON 10+0.1 c8, penta -1.1 [-13,+11] at cap, 0/0 flag-outs, 77.6% adjudicated, DONE_EXIT_0). Config unchanged since; no fresh preflight (coordinator-confirmed, same basis T1b/T4c used).
decision_rule: see PREDECLARED RULE below (standard batch-cert shape, batch #2 slot)
games_or_pairs: 8000 games (4000 pairs) — ran to the cap
result: penta -0.4 [-6,+5], pLLR -0.32 (LLR -0.28), W-D-L 2220-3551-2229 (49.9%), 8h17m36s, DONE_EXIT_0
flags_errors: 0/0 flag-outs (no HALT); 0 crash / 0 illegal / 0 no-move / 0 time-forfeit; reasons all legitimate (adj-win 4425, adj-draw 1927, max-moves 868, draw-rule 755, checkmate 24, stalemate 1 = 8000); pLLR never reached either SPRT bound
verdict: SHELVED — capped-nonpositive (penta -0.4 < +1, pLLR -0.32 < 0). Engine diff reverted (targeted checkout of engine/search.go + engine/qsearch_test.go); output/t13.patch (dbd86641…) is the shelf copy
next_action: box next item = T1c; reopen path recorded below as T16 (S7 depth-gate)
```

## Decision rule (predeclared — batch-certification shape, real-clock ONLY)

Real-clock 10+0.1 c8 SPRT vs `ngn_t4c.exe` (= HEAD c9dd9d2 engine), elo0=-3 elo1=+3, alpha=beta=0.05
(pLLR bounds ±2.94), adjudication STANDARD-ON, on the A/A-validated c8 adjudication-ON config, maxgames
8000 mingames 300.

**PREDECLARED RULE (verbatim):** "pLLR >= +2.94 -> KEEP outright (H1 accept). pLLR <= -2.94 -> REJECT.
Capped-positive at the 8000g cap (penta point est >= +1, pLLR > 0, no regression signal) -> PROVISIONAL
keep under batch certification (batch #2 slot). Capped-nonpositive -> SHELVE with reopen condition. Any
candidate flag-out spike above the A/A baseline (>2 either side) -> HALT and investigate before verdicting."

## Honest prior (DOWNGRADED — the +7.0 belonged to the false "first write" premise)

The +7.0 [S #126] prior in the revamp doc is for introducing qsearch TT stores **at all** — a gap NGN
does NOT have (it already stores the capture fail-high since a0fdda41/S3). So +7.0 does not transfer to
the stand-pat-only store. **Honest expectation: LOW SINGLE DIGITS, plausibly neutral or negative.** The
mechanism is marginal (the stand-pat cut is already cheap — an eval-cache lookup before capture
generation; the stored entry carries no move so gives no ordering help; depth-0 won't satisfy main
search's depth-gated cutoff at line 1262), and the measured nodecheck headwind (+13-39% nodes/depth,
above) is a real drag the cutoff/time savings must overcome. Consistent with the expectation-calibration
rule (no 3-digit promises; demonstrated curve +2-8/item at ~60% transfer). The batch-cert rule
(provisional-on-cap) fits: NGN family has no adverse prior for this store, and the change is small/safe.

## Verification: the qsearch probe consumes depth-0 LowerBound entries (else the store would be dead)

Confirmed BEFORE building (one grep + read): the qsearch TT probe at search.go:2192-2196 is
**depth-blind** — `case LowerBound: if ttEval >= beta { info.QTTCutoffs++; return ttEval }`, no `ttDepth`
gate. So a depth-0 stand-pat LowerBound entry IS usable for a later qsearch cutoff. (By contrast the
main-search probe at search.go:1262 gates on `ttDepth >= int8(depth)`, so these depth-0 entries never cut
in main search at depth>=1 — they only cut in qsearch, and only feed main search indirectly via the S7
static-eval refinement at 1368.) If the probe had been depth-gated the store would be a pure cost and I
would have stopped; it is not.

## Baseline tests (patched tree)

- `go test -short ./engine -count=1` ok (3.75s)
- `go test -short -race ./engine -count=1` ok (21.18s)
- `go test -short ./... -count=1` ok (engine + cmd/sprt + internal/uci all green)
- Targeted: TestStandPatFailHighStoresLowerBound RED pre-patch, GREEN post-patch; TestQSearchTTStoresWired
  (capture-store guard), TestQuiescenceInvariants, and all three corrhist qsearch guards still PASS.

## Status

DEV COMPLETE. Diff backed up at `output/t13.patch` (sha256 dbd86641…). Candidate `ngn_t13.exe` (sha256
5f9b383e…) cross-compiled. Base `ngn_t4c.exe` (40783f04…) already box-staged == HEAD engine. Full suite
green. Awaiting the staging + launch steps below.

## Launch

**LAUNCHED 2026-07-09 06:15:50 EDT (Mac) / 6:15:51 AM box time** (`ps` StartTime for all 17 processes)
via `scripts/boxsprt.sh launch t13`, after full preflight passed and `scripts/boxsprt.sh ps` showed the
box idle. Command verbatim from the run batch:

```text
sprt.exe -new .\ngn_t13.exe -base .\ngn_t4c.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 8000 -mingames 300 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
```

On-box SHA-256s verified at launch:
- `ngn_t13.exe` = `5F9B383E1D39893BC105D06601D2842627A9BF0EEA514ADCF1566B5926C9FD1B` (candidate)
- `ngn_t4c.exe` = `40783F0444ED47CF50BF9CA0DFA5D7635536E1A1519FE47C902498665141A1F8` (base = HEAD c9dd9d2 engine)
- `sprt.exe`    = `30C33E0512725B7F552D8A1CF72BA6F1E0DEB4DCB122C6B6BA8F6433C686A762` (13560dc validated mill)
- `sprt_openings.txt` = `974E4B5AB871A9E106D0C766BFA39FC83676222337FB7702F61782E2AC5B3222` (5000 lines)

Header parse confirmed: `mode: real clock 10s+0.1s (concurrency 8) | openings: 5000 (x2 colors)`;
`H0: elo<=-3.0 H1: elo>=3.0 (alpha=0.05 beta=0.05 -> LLR bounds [-2.94, 2.94])`; adjudication line
`resign>=900cp/5p draw<=10cp/10p>=80p` echoed. `ps` showed exactly one `sprt` + 8 `ngn_t13` + 8 `ngn_t4c`
(the T13 run as the only live worker). Games flowing at first poll (G1-G6, adjudication firing both
kinds). **Verdict TBD** — coordinator owns the watch.

## Verdict

**SHELVED — capped-nonpositive** (coordinator-classified 2026-07-09 per the predeclared rule: ran to the
8000g cap with penta -0.4 < +1 and pLLR -0.32 < 0). Completed 2026-07-09, `DONE_EXIT_0`, duration
**8h17m36s**. The engine diff has been reverted (targeted `git checkout -- engine/search.go
engine/qsearch_test.go`); `output/t13.patch` (sha256 `dbd8664139b3c4a5c852306207c39518496e7dd625349185938b11eaa7ade7bf`)
is the shelf copy, and the reverted engine rebuilds byte-identical to the T4c-keep `ngn_t4c.exe`
(`40783F04…`) — clean HEAD is unchanged, `go test -short ./engine` green.

RESULT block (verbatim from `output/t13_out.txt`):

```text
=== RESULT (8h17m36s) ===
Games: 8000   W-D-L: 2220-3551-2229   score: 49.9%
Elo(new - base): -0.4   95% CI [-8, +7]
LLR: -0.28   bounds [-2.94, 2.94]
Pentanomial [LL 247  LD 981  {LW,DD} 1547  WD 984  WW 241] over 4000 pairs
Penta Elo: -0.4   95% CI [-6, +5]   pLLR -0.32  (THE decision stat; trinomial above is secondary)
Flag-outs (lost on time): new 0, base 0  of 8000 games  (goal: 0)
Adjudicated early: 4425 decisive, 1927 draw  of 8000 games
Verdict: INCONCLUSIVE (ran out of games — add openings or raise -maxgames)
DONE_EXIT_0
```

**Integrity sweep (full 8000-game `t13_out.txt`):**
- **Never crossed either bound:** ZERO lines with |pLLR| ≥ 2.94 across the whole run — it drifted around 0
  (final pLLR -0.32) and ran to the cap, exactly the capped-nonpositive shape.
- **Clean terminations:** the only result reasons in 8000 games are adj-win 4425, adj-draw 1927, max-moves
  868, draw-rule 755, checkmate 24, stalemate 1 (= 8000). **0 crash, 0 illegal-move, 0 no-move, 0
  time-forfeit / disconnect.**
- **Flag-outs new 0 / base 0** — no spike, HALT trigger never approached.
- **Adjudication:** 4425 decisive + 1927 draw fired cleanly (both kinds), matching the validated c8
  adjudication-ON profile.

This confirms the honest prior (the +7.0 belonged to the false "first qsearch write" premise; the
stand-pat-only store is marginal) and the measured nodecheck headwind (+13-39% fixed-depth nodes): the
qsearch cutoff/time savings did not overcome the depth-0 store flood + S7 margin reshaping. Net -0.4, dead
neutral.

**REOPEN CONDITION:** Reopen the stand-pat store only after the S7 static-eval refinement
(`search.go:1368-1374`, `LowerBound && ttEval > staticEval` with NO depth gate) is depth-gated or excludes
depth-0 qsearch entries — the traced +13-39% fixed-depth tree inflation ran through that coupling and is the
plausible confound; also reopenable by a TT-clustering change (T11) reducing depth-0 eviction pressure.
(This reopen path is queued as **T16 — S7 refinement depth-gate**.)
