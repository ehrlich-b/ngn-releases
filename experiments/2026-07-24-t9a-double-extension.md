# 2026-07-24 T9a — singular double extension (+2 plies on an overwhelmingly forced move)

Pre-registered BEFORE launch (CLAUDE.md work-loop step 5). Isolated single-change per-change gate: candidate =
current main HEAD engine (`5acd68b`, engine byte-identical to the T5-keep `5b93966` — T7 reverted, verified
`git diff 5b93966 HEAD -- engine/` empty) + `output/t9a.patch`; base = that same HEAD engine unchanged, reusing
the on-box `ngn_t5.exe` (the exact T5-keep binary). Real-clock 10+0.1 c8 on the LAN 9800X3D box. Highest-ranked
unblocked queue item after the T7 shelve: the biggest remaining measured prior in the whole T-queue.

Queue rationale (Elo-per-confirmation-hour): T9a **+11.2 / +10.1 LTC** [S #115], +11.9 [Sim] beats T12 NMP
cutnode-only +9.7 [S #230], T11a TT replacement +7.5 [S #16], T9b multicut +5.8, and T10a killer-reset +2.5 [E].
T9 is flagged ATTENDED in the revamp doc because the `:1821`/`:1832` guardrails are load-bearing — that flag is
discharged here by (a) a pre-launch nodecheck explosion gate, (b) a per-path double-extension cap, and (c) a
live early-health flag-out watch at launch, with a HALT rule on any candidate flag-out above the A/A baseline.

## Mechanism (one paragraph)

NGN already runs a full singular framework: at `depth >= SINGULAR_DEPTH` with a TT move it re-searches the
parent with that move excluded at `singularBeta = ttEval - (SINGULAR_MARGIN*depth)/32`, extends by one ply when
the verification fails low (`singularScore < singularBeta`), and — since W-SE2 — shaves a ply on non-PV cut
nodes when the verification fails high with `ttEval >= beta` (search.go ~1868-1885). What is missing is the
standard second arm: when the verification does not merely fail low but fails low **by a wide margin**, every
alternative reply is at least `DOUBLE_EXT_MARGIN` worse, so the move is not just singular but overwhelmingly
forced and is worth a second ply. T9a adds exactly that: `if !isPV && singularScore < singularBeta -
DOUBLE_EXT_MARGIN` then `nextDepth++` a second time. Two guards bound it. (1) Non-PV only — a PV node is
already searched to full depth and re-extending it is how the historical seldepth-111 explosion vectors start.
(2) A per-root-to-leaf-path cap `DOUBLE_EXT_PATH_CAP = 5`, matching the measured prior's shape ([S #115] is
"capped at 5 per path"), carried in a new `info.DoubleExtCount[ply]` stack written for the child before each
recursion (same live-ancestor-only discipline as `MoveStack`, so no unwind restore) — a forcing sequence
therefore cannot compound double extensions without bound. The pre-existing per-node ceiling
`nextDepth > depth+1 -> depth+1` had to be raised to `depth+2` **for a double extension only**: leaving it
would silently erase the second ply on any singular move that also gives check (there `nextDepth` is already
`depth+1`), which would make the change a partial no-op on precisely the forcing nodes it targets — the T7
failure mode. The per-path `EXTENSION_BUDGET = 24` clamp below it is untouched and remains the outer
anti-explosion bound. `DOUBLE_EXT_MARGIN = 24` is registered as SPSA dim `DoubleExtMargin` [4,128] for T8.

```yaml
id: 2026-07-24-t9a-double-extension
date: 2026-07-24
change_class: search/eval heuristic (search behavior — extension depth; real-clock games gate)
hypothesis: >
  extending an overwhelmingly-singular move by 2 plies instead of 1 (non-PV only, capped at 5 per
  root-to-leaf path) nets positive Elo at real clock by spending depth where the tree is genuinely forced.
  Measured prior: Stash #115 double extension +11.2 / +10.1 LTC (capped at 5 per path), +11.9 [Sim] — the
  biggest remaining measured prior in the T-queue. H1: candidate >= +3 Elo over base; H0: candidate <= -3.
base_commit: 5acd68b          # main HEAD; engine byte-identical to T5-keep 5b93966 (T7 reverted)
candidate_commit: 5acd68b + output/t9a.patch   # patch uncommitted per policy (no commit before verdict)
base_binary_sha256: 0a8f65b75e59c376052f83b14787b0af2eba65552cedfee6f70ce576891ca721      # on-box ngn_t5.exe (reused T5-keep binary, hash-verified on box)
candidate_binary_sha256: 79a940b7bf28215794c2adabfde1dd3c19d26bf976a6de9bb0aede1a6d4a36dc  # output/ngn_t9a.exe (reproducible build, hash-verified on box)
harness_commit: sprt.exe sha256 30c33e0512725b7f552d8a1cf72ba6f1e0deb4dcb122c6b6ba8f6433c686a762 (M5-aware validated mill, box-staged, untouched); launcher scripts/boxsprt.sh untouched
command: sprt.exe -new .\ngn_t9a.exe -base .\ngn_t5.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 8000 -mingames 300 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
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
aa_preflight: standing 2026-07-03 M5-enable adjudication-ON A/A (experiments/2026-07-03-m5-adjudication-aa.md) — same TC (10+0.1) / concurrency (8) / adjudication-flag set / machine (LAN 9800X3D) / openings config, penta -1.1 [-13,+11], pLLR -0.18, 0/0 flag-outs, 77.6% adjudicated, 961 g/hr, DONE_EXIT_0; the SAME convention adopted by the KEPT T1b/T4c/T1e/T5 and SHELVED T13/T1c/T16/T10b/T7 runs, so no fresh A/A required.
decision_rule: >
  SPRT [-3,+3] on pentanomial pLLR, bounds +/-2.94, maxgames cap 8000; pLLR >= +2.94 -> KEEP (Batch-2 keep per
  CLAUDE.md batch-certification path); pLLR <= -2.94 -> SHELVE and revert; capped at 8000g with point est >= +1,
  pLLR > 0, and 0 excess flag-outs -> Batch-2 PROVISIONAL keep; capped-nonpositive -> shelve. HALT-AND-
  INVESTIGATE (do not verdict) on any candidate flag-out above the A/A baseline of 0 — this is an extension
  change, so a time-loss cluster is the explosion signature, not noise.
games_or_pairs: (pending)
result: (pending)
flags_errors: (pending)
verdict: (pending)
next_action: (pending)
```

## Pre-launch behavioral-delta gate (the T7 lesson, applied)

T7 burned 8h19m measuring a candidate whose fixed-depth nodecheck moved 0.001%. The gate adopted from that
shelve — **a pruning/extension candidate must move nodecheck by >= ~1% on at least one canonical position
before it earns box time** — is applied here BEFORE launch, and passes on all three positions by 10-30x the
threshold. Counts verified deterministic (two identical runs).

| position | depth | base (5b93966) | T9a       | delta    |
|----------|-------|----------------|-----------|----------|
| kiwipete | d12   | 346662         | 393979    | **+13.6%** |
| mid      | d12   | 149587         | 168792    | **+12.8%** |
| end      | d16   | 765656         | 1009455   | **+31.8%** |

Direction and magnitude are both as expected for a working double extension: the tree grows (extra plies cost
nodes) by ~13-32%, NOT by the 5-20x that a runaway extension would show. This is the explosion gate discharging
the ATTENDED flag: a genuine guardrail failure would not sit at +13%. Baselines are NOT re-locked — that happens
only on a keep verdict. The counterpart risk this trades into is real-clock depth loss (more nodes per ply at
fixed time), which is exactly what the SPRT is measuring.

## Mechanism-fires check (new regression test)

`TestSingularDoubleExtensionsFire` (engine/reentrancy_test.go, alongside the existing
`TestSingularExtensionsFire`) asserts the double-extension arm actually fires, is bounded by the singular
counter, and is gated by the singular toggle. Kiwipete d10: **44 double extensions of 135 singular extensions,
0 cap clamps** — the mechanism is live and `DOUBLE_EXT_PATH_CAP` is a safety net rather than a binding
constraint at this depth. This test is the durable lock against the T7 no-op failure mode: a refactor that
re-clamps `nextDepth` to `depth+1`, or a margin tuned out of reach, turns the feature into a silent no-op that
would otherwise only show up as an expensive null 8 hours later.

New counters `SingularDoubleExts` / `DoubleExtCapClamps` are plain diagnostics, never read by search decisions.

## Tests (patched tree, all GREEN)

- `go test -short ./engine -count=1` -> ok (4.455s)
- `go test -short -race ./engine -count=1` -> ok (25.886s)
- `go test -short ./... -count=1` -> ok (engine, internal/uci)
- `go test -short ./engine -run TestSingular -count=1 -v` -> both singular tests PASS (counters logged above)

## Binaries

Cross-compiled with the record-convention command (Makefile `build` Windows line):
`GOOS=windows GOARCH=amd64 GOAMD64=v3 go build -o output/ngn_t9a.exe main.go`, go1.26.2, no ldflags/trimpath,
from clean HEAD `5acd68b` + `output/t9a.patch`. Tree restored after build; patch stays uncommitted.

- **Candidate** `output/ngn_t9a.exe` sha256 `79a940b7bf28215794c2adabfde1dd3c19d26bf976a6de9bb0aede1a6d4a36dc`
  — reproducible (a second identical build reproduced the sha exactly); on-box `Get-FileHash` returns
  `79A940B7BF28215794C2ADABFDE1DD3C19D26BF976A6DE9BB0AEDE1A6D4A36DC` (full match).
- **Base** reuses the on-box `ngn_t5.exe` sha256 `0a8f65b75e59c376052f83b14787b0af2eba65552cedfee6f70ce576891ca721`
  — the exact T5-keep engine binary already staged and hash-verified on the box (on-box re-verified at stage
  time this session); no rebuild.

Patch: `output/t9a.patch` (engine/search.go + engine/reentrancy_test.go, 153 lines).

## Bounds — [-3,+3] per-change gate (NOT the [0,+6] cert shape)

Isolated per-change SPRT: `-elo0 -3 -elo1 3` (H0: elo<=-3, H1: elo>=3), same shape as the KEPT
T1b/T4c/T1e/T5 and SHELVED T13/T1c/T16/T10b/T7 runs. `alpha=beta=0.05` yields pLLR bounds +/-2.94.

## Launch

(pending — recorded on confirmation)

CONFIRMED LIVE 2026-07-24 19:37:47 EDT via `scripts/boxsprt.sh launch t9a` (command exactly as the `command:`
field above). Box was idle immediately before launch (`ps` empty). Topology verified: 1 `sprt.exe` (PID 14736)
+ 8 `ngn_t9a` (PIDs 5872, 11504, 11572, 28276, 29416, 30152, 30288, 31792) + 8 `ngn_t5` (PIDs 11052, 13152,
13848, 14248, 21456, 24824, 26000, 31412), all started 19:37:47 EDT — the sole live workers on the box.
Out-file: `C:\Users\ehrli\ngn\sprt\t9a_out.txt` (fetch via `scripts/boxsprt.sh fetch t9a`).

On-box hashes verified at stage time: candidate `ngn_t9a.exe`
`79A940B7BF28215794C2ADABFDE1DD3C19D26BF976A6DE9BB0AEDE1A6D4A36DC`, base `ngn_t5.exe`
`0A8F65B75E59C376052F83B14787B0AF2EBA65552CEDFEE6F70CE576891CA721` — both full-hash matches. Header echoes the
predeclared binaries/args/bounds exactly (verbatim from the out-file):

```text
new=.\ngn_t9a.exe  base=.\ngn_t5.exe
mode: real clock 10s+0.1s (concurrency 8) | openings: 5000 (x2 colors) | concurrency: 8
H0: elo<=-3.0   H1: elo>=3.0   (alpha=0.05 beta=0.05 -> LLR bounds [-2.94, 2.94])
adjudication: resign>=900cp/5p  draw<=10cp/10p>=80p  (opt-in; A/A-gate before trusting a verdict)
```

Early health at G1->G9: games flowing, all four adjudication paths firing (draw-rule / adj-draw / adj-win /
max-moves), **0 flag-outs, 0 forfeits, 0 panics, 0 illegal moves, 0 crashes** — the flag-out watch that
discharges the ATTENDED flag. (The G5 `pLLR +2.16` is single-pair early noise at a ±hundreds CI, the same
artifact T7's G4 +4.32 produced; the `-mingames 300` floor means the stop rule is not armed there.)

```text
G1     0W  1D  0L   50.0%  elo   +0.0 [-495,+495]  LLR  +0.00  pLLR  +0.00  draw-rule
G5     1W  4D  0L   60.0%  elo  +70.4 [-209,+350]  LLR  +0.09  pLLR  +2.16  adj-win
G9     1W  7D  1L   50.0%  elo   +0.0 [-213,+213]  LLR  +0.00  pLLR  +0.00  max-moves
```

Poll with `scripts/boxsprt.sh tail t9a` / `scripts/boxsprt.sh ps`. The run drives to the 8000g cap or a
+/-2.94 pLLR crossing; apply the predeclared `decision_rule` on completion, and **HALT rather than verdict if
candidate flag-outs exceed the A/A baseline of 0** (extension changes fail as time losses, not as noise).

## Pre-registered interpretation of a negative result (written at G770, BEFORE the verdict)

Recording this now so the reading of a failure is not reverse-engineered from the number. At G770 the run sits
at penta-adjacent trinomial elo -11.7 [-36,+13], pLLR -0.98 — drifting negative after the usual early noise.
Throughput is normal (770 games in 48.3 min = **957 g/hr** vs the 961 g/hr A/A baseline), so nothing is
pathological about the run itself; the candidate is simply not winning so far.

If this ends negative or capped-nonpositive, the **first** hypothesis is the double-extension RATE, not the
mechanism. The mechanism-fires check recorded **44 double extensions out of 135 singular extensions at
kiwipete d10 — a 33% rate**, which is high for this technique: reference engines double-extend on a small
minority of singular moves. With `singularBeta = ttEval - 2*depth`, a `DOUBLE_EXT_MARGIN` of 24 asks a
singular move to beat its siblings by only ~40cp at depth 8 — a bar that a third of singular moves clear, so
T9a as configured is closer to "extend singular moves by 2 plies" than to "extend *overwhelmingly forced*
moves by 2 plies". Over-extending a third of singular nodes costs breadth everywhere else, which is a
coherent story for a small negative at fixed real clock, and it matches the +13-32% nodecheck growth.

So the predeclared reopen, if the verdict is negative:

1. **Raise the margin** (T9a-2: `DOUBLE_EXT_MARGIN` 64-96) to make double extensions selective, and re-run the
   mechanism-fires check expecting a single-digit percentage rather than 33%. This is a constant change to a
   live mechanism, not a new mechanism — cheap, and the nodecheck gate will confirm it still moves the tree.
2. Only if a selective margin also fails, treat the double-extension lane as genuinely non-transferring for
   NGN and record it as such — noting that unlike T7 this run DID exercise its mechanism (+13.6/+12.8/+31.8%
   nodecheck, 33% fire rate), so a null here is real evidence about the lane, not evidence about a no-op.

`DoubleExtMargin` is already registered as an SPSA dim [4,128], so option 1 is also reachable through T8
rather than by another hand-picked constant — preferred if T8 runs first.

Conversely, if the verdict is positive, the same 33% rate is worth an SPSA pass anyway: the current constant
was chosen from the reference literature, not from NGN's own data, and the fire rate suggests it is not near
NGN's optimum in either direction.

## RESULT — H0 ACCEPTED (SHELVED), 2026-07-25 01:05 EDT

```text
=== RESULT (5h27m58s) ===
Games: 5268   W-D-L: 1359-2471-1438   score: 49.3%
Elo(new - base): -5.2   95% CI [-15, +4]
LLR: -2.57   bounds [-2.94, 2.94]
Pentanomial [LL 153  LD 687  {LW,DD} 1028  WD 618  WW 148] over 2634 pairs
Penta Elo: -5.2   95% CI [-12, +1]   pLLR -2.87  (THE decision stat)
Flag-outs (lost on time): new 0, base 0  of 5268 games  (goal: 0)
Adjudicated early: 2792 decisive, 1229 draw  of 5268 games
Verdict: H0 ACCEPTED: new is NOT better (<= -3 ELO)
DONE_EXIT_0
```

Launched 19:37:47 EDT 2026-07-24, ran 5h27m58s to a self-declared stop at G5268 = **964 g/hr**, versus the
961 g/hr A/A baseline. Throughput held within 0.3% for the entire run; no throughput pathology, no crash, no
illegal move, no no-move result.

**Flag-outs 0/0 — the predeclared HALT condition did NOT fire.** This matters for the reading: T9a was the
first ATTENDED-flagged item, flagged because extension changes classically fail as *time losses* rather than
as noise. They did not. The candidate lost strength while keeping perfect clock discipline, so the per-path
cap (`DOUBLE_EXT_PATH_CAP = 5`) did its job and the failure is a search-quality failure, not a blow-up. The
operational discharge of the ATTENDED flag (nodecheck explosion gate + per-path cap + HALT rule + a
mechanism-fires regression test) is retrospectively validated — that flag never needed a human vigil.

### Bound-crossing detail (recorded because the summary line reads inside the bound)

The printed summary shows pLLR **-2.87**, which is inside `[-2.94, +2.94]`. The full-log scan resolves this:

| Quantity | Value |
|---|---|
| pLLR samples scanned (regex `[-+]?[0-9]+\.[0-9]+`) | 5268 |
| First sample at/below -2.94 | **G5258, pLLR -2.94** |
| Samples at/below the bound before the stop drained | 10 |
| Run minimum | **-2.98 @ G5260** |
| Whole-run maximum | +2.16 @ **G5** (below `-mingames 300`; correctly ignored) |
| **Post-mingames maximum** | **+0.25 @ G303** |

So the crossing is real and the harness stopped on it; the final -2.87 is a **drain artifact** — the games
already in flight across 8 concurrent workers completed after the stop signal and were folded into the
recomputed summary. `DONE_EXIT_0` plus an explicit `H0 ACCEPTED` is the harness's own verdict, not an
inference of mine. Note also that this distinction changes nothing: crossing the reject bound and capping
nonpositive both route to SHELVE under the predeclared `decision_rule`, and the point estimate is negative
either way.

The **post-mingames maximum of +0.25** is the most informative number in the table. Unlike T7 (which touched
+1.92 post-mingames and spent time on the positive side), T9a never had a plausible positive phase at all
once the run was eligible to stop. There is no "it was winning before it wasn't" story to tell here.

### Elo trajectory

| Checkpoint | Elo | pLLR |
|---|---|---|
| G500 | -3.5 | -0.18 |
| G1000 | -7.3 | -0.76 |
| G2000 | -7.3 | -1.49 |
| G3000 | -6.3 | -1.91 |
| G4000 | -3.8 | -1.54 |
| G5000 | -4.9 | -2.52 |
| G5268 (final) | **-5.2** | -2.87 |

Monotone-ish negative from G1000 on, with one softening excursion around G4000 that reversed. The estimate
never crossed zero after G300.

### Verdict and interpretation

**SHELVED.** Engine diff reverted; HEAD's engine remains byte-identical to the T5 keep `5b93966`. The patch is
retained as `output/t9a.patch` (2 files: `engine/search.go`, `engine/reentrancy_test.go`).

The pre-registered interpretation above stands **as written, unamended** — and it is worth stating plainly
that it was written at G770 before the verdict was known, predicting a small negative and naming the
double-extension RATE as the first suspect. The final -5.2 [-12,+1] is exactly the "small negative" that
prediction described. The 33% fire rate (44 doubles / 135 singular extensions at kiwipete d10) remains the
leading explanation: `DOUBLE_EXT_MARGIN = 24` against `singularBeta = ttEval - 2*depth` asks a singular move
to beat its siblings by only ~40cp at depth 8, so T9a as built approximates "extend singular moves by 2"
rather than "extend overwhelmingly forced moves by 2". Buying that extra ply on a third of singular nodes
costs breadth everywhere else, which at a fixed real clock is a coherent mechanism for a -5.

**This null is real evidence about the lane, not evidence about a no-op.** That is the key contrast with T7,
and the reason the two shelves should not be read the same way. T9a moved the tree hard and provably:
nodecheck +13.6% / +12.8% / +31.8% across the three positions, and a regression test asserted the mechanism
fired 44 times with 0 cap clamps. T7 moved it by <1%. A null on a change that demonstrably alters the search
constrains the hypothesis; a null on a change that alters nothing constrains only the patch. The >=1%
nodecheck gate derived from T7 did its job here — it admitted a change that was genuinely testable, and the
test returned a genuine answer.

### Reopen condition

Per the pre-registered ladder, option 1 is live and unconsumed: **T9a-2, `DOUBLE_EXT_MARGIN` 64-96**, to make
double extensions selective, with a re-run mechanism-fires check expected to land in the single-digit
percentages rather than 33%. That is a constant change to a mechanism now proven live in the code, and it is
the cheapest form of a follow-up.

**But it does not jump the queue.** `DoubleExtMargin` is already a registered SPSA dimension `[4, 128]`, so
T8 reaches the same question without another hand-picked constant and without another 5.5-hour box job spent
on one guess. Ranking: prefer T8 to pick the margin from NGN's own data; hand-run T9a-2 only if T8 slips or
excludes the dim. Recorded as a lane that is **narrowed, not closed** — what T9a falsified is
`DOUBLE_EXT_MARGIN = 24`, not double extensions.

Cost of the answer: 5h28m of free box time, 0 dollars.
