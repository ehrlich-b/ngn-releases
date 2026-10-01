# 2026-07-26 T20 — SEE-prune depth range 4 -> 8 — COMPLETED, SHELVED (capped null)

**STATUS: COMPLETED + SHELVED. Verdict at the bottom.** Launched 2026-07-26 18:33:44 EDT, confirmed live (8 `ngn_t20` + 8 `ngn_t5` + 1 `sprt`, header
echoes `new=.\ngn_t20.exe  base=.\ngn_t5.exe`, both on-box hashes verified equal to the manifest before
launch). T3b SHELVED as a capped null at -3.1, so the conditional base below (`ngn_t5`) is the correct one.
Box idle between runs: ~3 minutes.

> **LAUNCH GOTCHA — cost one failed launch, record it.** `boxsprt.sh launch <name> <args...>` **prepends
> `$BIN` (`sprt.exe`) itself**. Passing the full command line from this manifest's `command:` field verbatim
> produces `sprt.exe sprt.exe -new ...`; Go's `flag` parser stops at the first non-flag token, so **every flag
> is silently dropped** and sprt falls back to its default engine path, dying with
> `engine binary not found: ./build/ngn (build it first)` + `DONE_EXIT_1`. No workers spawn, so nothing leaks,
> but the box sits idle until someone notices. **The `command:` field below is the true full command line for
> the run record; the launch invocation must omit the leading `sprt.exe`.** Not fixing `boxsprt.sh` for this —
> a run is live and the harness must not be edited mid-verdict (CLAUDE.md); re-evaluate when the box is free.

Origin: the June technique census flagged that NGN's SEE-prune dies at `depth <= 4` while the class runs to
~8. Futility was extended to 8 in W1; **SEE-prune never was.** Change size: **two tokens** (`4` -> `8` on the
capture and quiet SEE-prune conditions, search.go:1715 and search.go:1729).

## A SPLIT VARIANT WAS TESTED AND FALSIFIED — run the FULL patch

Reading T20 against the lesson T18b had just delivered (a faithful-looking transplant that imports a
technique without its surrounding calibration) surfaced a real structural asymmetry:

- **capture path prunes at a FLAT `SEE < -100`** — identical threshold at depth 1 and depth 8;
- **quiet path prunes at a SCALED `SEE < -80*depth`**, and the code's own comment states the principle:
  *"Threshold scales with depth so deeper nodes prune only larger losses."*

So T20 extends a **flat** margin into deep nodes on the capture path, violating the convention the engine
states two lines below for quiets. The hypothesis was that the sound half is the quiet path, and a split
variant **T20q** (`output/t20q-quietonly.patch`, quiet path only) would capture the gain at lower risk.

**Measured, and the hypothesis is dead.** Depth at 400K fixed nodes — the metric this lane is steered by:

| position | base | T20 full | T20q | full Δ | T20q Δ |
|---|---|---|---|---|---|
| quiet-mg-QGD | 14 | **15** | **15** | +1 | +1 |
| quiet-mg-closed | 12 | **13** | **13** | +1 | +1 |
| lateMg-ph6-11 | 13 | 13 | 13 | 0 | 0 |
| najdorf-mg | 14 | **15** | **12** | **+1** | **-2** |
| rook-eg-R4P | 17 | 17 | 17 | 0 | 0 |
| pawn-eg | 32 | 32 | 32 | 0 | 0 |

**The capture-path extension is what carries najdorf; removing it costs 2 plies there** — a 3-ply swing
against the split. T20q is strictly worse and is NOT the candidate.

**The endgame-cost argument for splitting also failed on its own evidence.** Fixed-depth nodecheck showed
T20 full at **-27.8% / -18.2% / +78.4%**, and that +78.4% endgame figure was the original reason to suspect
the capture path. But it comes from **one** position, and both ebfprobe endgames (rook-eg-R4P, pawn-eg) show
**flat depth and flat nodes**. This is precisely the artifact class `project_endgame_tree_diagnostic` records
("the 2.5-5x eg tree was a mislabeled-position artifact; single-position node counts unreliable for pruning
knobs"). One position is not evidence of an endgame regression.

**What survives from the analysis:** the flat-vs-scaled capture-margin asymmetry is real and is exactly the
premise of **T15** (capture margin `-100` -> `-100*depth`, mirroring the quiet convention), already queued.
T20 and T15 are independent code paths and stay separate changes — but if T20 keeps and T15 later keeps, the
capture margin will have been both extended and re-shaped, and **that composition is worth an explicit
interaction check** rather than assuming additivity.

## Behavioral-delta gate (>= ~1%, earned from T7) — PASSES DECISIVELY

Fixed-depth nodecheck vs the HEAD lock (346662 / 149587 / 765656): **-27.8% / -18.2% / +78.4%**
(250165 / 122356 / 1365868). Baseline re-verified this session from a **fresh HEAD build** — note
`scripts/nodecheck.sh` does **not** rebuild, it measures whatever sits at `build/ngn`, so a stale binary will
silently report "no change" for any candidate.

**Second gate question, earned from T18b: what KIND of information moved the tree?** SEE pruning skips moves
on `staticExchangeEvaluation`, an **exact material-swap computation** — proven information, not a cached
guess, and nothing is persisted to the TT. T20 passes the soundness question that T18b failed. The residual
speculation is the *threshold*, not the computation: SEE cannot see promotion, king activity, or attacking
compensation, so a deep prune can discard a real sacrifice.

Mechanism fires (kiwipete d12 `seeq`): base 10647 -> **T20q 19569 (+84%)**.

> **Counter caveat — `seeq` is CONFOUNDED for the full patch, same trap as T12's `nmtry`.** T20 full reads
> `seeq 10067`, *below* base, despite strictly widening the quiet prune's depth range — because the
> capture-path extension shrinks the tree ~28%, leaving fewer quiet nodes to prune at all. An absolute counter
> cannot witness a change that alters tree composition. The +84% on the isolated quiet variant is the honest
> witness that the mechanism fires.

```yaml
id: 2026-07-26-t20-see-prune-range
date: 2026-07-26
change_class: search heuristic (pruning depth range). Requires a completed predeclared paired game test.
hypothesis: >
  NGN's SEE-prune dies at depth<=4 while the class runs to ~8, and W1 already extended futility to 8 without
  extending SEE. Widening both SEE-prune paths to depth 8 buys depth at equal nodes (+1 ply on 3 of 4
  middlegames) on exact material information, with no persisted speculation.
patch: output/t20-seerange.patch (22 lines, engine/search.go only; two tokens of real change)
rejected_variant: output/t20q-quietonly.patch (quiet path only) — FALSIFIED, -2 plies najdorf
candidate: ngn_t20.exe sha256 90dd0ed2932cae1e2a72f5c7691460838cc84fed9b39e8aae4817ef83e020087
  PRE-STAGED on the box 2026-07-26 with the on-box hash verified equal to the local build; `-short` and
  `-race` engine suites green with the patch applied; tree reverted after the build. Launch is one command
  the moment T3b resolves.
base: ngn_t5.exe sha256 0a8f65b75e59c376052f83b14787b0af2eba65552cedfee6f70ce576891ca721
  CONDITIONAL — correct only if T3b SHELVES. If T3b is KEPT, T20 must be rebuilt and re-gated against the
  T3b tree; do NOT run against a superseded base and pool.
harness: sprt.exe sha256 30c33e0512725b7f552d8a1cf72ba6f1e0deb4dcb122c6b6ba8f6433c686a762 (M5-validated, untouched)
command: sprt.exe -new .\ngn_t20.exe -base .\ngn_t5.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 8000 -mingames 300 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
machine: AMD Ryzen 7 9800X3D 8c/16t, native Windows (LAN box 192.168.4.108)
tc: 10+0.1 (seconds)
concurrency: 8
openings: canonical sprt_openings (5000 lines) sha256 974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222
adjudication: STANDARD-ON (M5-validated flags)
maxgames: 8000
expected_duration: ~8h30m at ~948 g/hr if it drains to cap; decisive runs on this rig have stopped at 1300-5300
```

## Predeclared decision rule (standard per-change T-item gate)

- pLLR **>= +2.94** -> **KEEP** as a Batch-3 keep; re-lock nodecheck to the T20 numbers; re-run ebfprobe +
  the fmc/ttlist table per the re-measure cadence.
- pLLR **<= -2.94** -> **SHELVE** and revert.
- Capped with point est **>= +1 and pLLR > 0** -> **PROVISIONAL** keep into Batch 3.
- Capped **nonpositive** -> SHELVE.

**REOPEN CONDITION (predeclared).** A negative T20 does not close the SEE-prune-range lane by itself, because
depth 8 is a single guess at the cap. The re-derived variant is **depth <= 6** (half the extension), and it
should be tried only if the loss is small; a large negative implicates the flat capture margin and should be
routed to **T15 first** (fix the margin shape at the current depth cap), after which the range extension can
be re-tried on top of the corrected shape.

Verdict is taken from the harness's own `H0/H1 ACCEPTED` line plus a full-envelope scan of the printed
samples — **never the printed final pLLR alone** (c8-drain artifact, observed 3x, symmetric).

## Honest expectation

**+2 to +6 class.** In its favour: +1 ply on 3 of 4 middlegames is the strongest depth result of any candidate
gated this campaign, it runs on exact material information, it persists nothing, and it completes a transplant
the engine already half-adopted (futility to 8 in W1). Discounts: **history pruning is still capped at
`depth <= 3`** — the same unfinished extension, deliberately kept as a follow-up rather than bundled; SEE's
blindness to promotion and attacking compensation is a real cost at depth 8; and the flat capture margin means
this extension lands in a regime the engine's own stated convention says should scale.

## RULE GAP CLOSED — pre-registered 2026-07-27 00:55 EDT, BEFORE the verdict (G6043, pLLR +0.47)

The predeclared rule above covers `point est >= +1 and pLLR > 0 -> PROVISIONAL` and
`capped nonpositive -> SHELVE`. It does **not** name the case T20 is currently sitting in: **capped with a
point estimate strictly between 0 and +1**. Recording the resolution now, while the outcome is still
unknown, so it cannot be chosen to suit the number.

**RESOLUTION: point estimate < +1 at the cap -> SHELVE, regardless of pLLR sign.**

Grounds, both from CLAUDE.md and neither invented for this run:

1. The batch-certification path is defined as *"a capped-but-positive result (**point est >= +1**, LLR > 0,
   no regression signal) may enter main as a PROVISIONAL keep."* The +1 floor is explicit; +0.7 does not
   clear it.
2. Active policy states *"Do not keep speculative search/eval changes on positive sign at an inconclusive
   cap."* A sub-+1 point estimate with an envelope that never neared a bound **is** sign-at-cap, which is
   precisely what that rule forbids.

So the only outcomes that keep T20 are `pLLR >= +2.94` (full H1) or a capped point estimate `>= +1` with
`pLLR > 0`. Anything else shelves and T4d launches against `ngn_t5` as staged.

**Note the asymmetry this creates and accept it deliberately:** a true +2 change will usually shelve under
this rule at this bound width, because +2 rarely reaches either bound in 8000 games at 10+0.1. That is the
known cost of the [-3,+3] per-change gate and is the reason the batch path exists at all — it is not a
reason to lower the bar mid-run.

## VERDICT — SHELVE (capped null), 2026-07-27 03:30:51 EDT

```yaml
verdict: SHELVE (capped-nonpositive) — ran to the 8000g cap without crossing either bound; the predeclared
  "capped nonpositive -> SHELVE" clause fired directly (the pre-registered sub-+1 gap-closure was not even
  needed: -0.7 is nonpositive outright). No revert required — never committed; shelf copy output/t20-seerange.patch.
games: 8000 (4000 pairs), cap reached
wdl: 2144W-3695D-2161L  score 49.9%
penta: -0.7 Elo, 95% CI [-6, +4]
penta_buckets: LL 214 / LD 1006 / {LW,DD} 1599 / WD 945 / WW 236
trinomial: -0.7 Elo [-8, +7]
pllr_final: -0.63
harness_line: "INCONCLUSIVE (ran out of games)" + DONE_EXIT_0 — neither H0 nor H1 accepted
flag_outs: new 0, base 0 of 8000   (goal 0 — met)
adjudicated_early: 4288 decisive, 1832 draw of 8000
wall: 18:33:44 -> 03:30:51 EDT = 8h57m07s (~894 g/hr)
box_idle_after: 30 seconds (T4d launched 03:31:21)
```

**Full-envelope scan (the required check, never the printed final pLLR):**

| statistic | value |
|---|---|
| samples | 8001 |
| max pLLR | +2.16 @G3 (startup artifact, 1 pair) |
| min pLLR | -1.15 @G2462 |
| **post-mingames max** | **+1.50 @G4094** |
| **post-mingames min** | **-1.15 @G2462** |
| samples >= +2.94 | **0** |
| samples <= -2.94 | **0** |

**The tightest capped null of the campaign.** The entire post-G300 trajectory lived inside **[-1.15, +1.50]**
— never within half the distance to either bound across 7700 samples. There is no reading in which the
SEE-prune range extension is worth keeping, and no c8-drain ambiguity to argue about.

## The depth gain was REAL and did not convert — record this, it is the lane's lesson

T20 had **the strongest depth result of any candidate gated this campaign**: +1 ply at 400K fixed nodes on
3 of 4 middlegames, on exact material information, persisting nothing. It measured **-0.7**.

**So depth-at-fixed-nodes gained without a strength gain.** That is the first clean case this campaign of the
steering metric moving and Elo not following, and it materially weakens ebfprobe as a *promotion* instrument
even as it remains useful for *rejection* (T6a's -4 plies, T19's -4). The asymmetry to carry forward:
**a depth LOSS still predicts trouble; a depth GAIN does not predict a keep.**

## REOPEN — recorded but ranked LOW, decided at verdict

The manifest's reopen says a small loss should route to the **depth <= 6** half-extension. The loss is
indeed small (-0.7). But that reopen is **not promoted**, for the same reason T3c was not:

1. **T20 is the FOURTH consecutive capped null in the node-level family** — T7 -0.2, T10b -2.4, T3b -3.1,
   T20 -0.7 — all with envelopes that never neared a bound. The queue-premise audit's tally becomes
   **node-level: 1 keep / 11 attempts, +2.0 Elo total**.
2. The audit's pre-registered trigger was exactly this: *"if T20 also caps null, question the queue's premise
   rather than draw the next card."* Drawing `depth <= 6` IS drawing the next card from the same deck.
3. Depth 6 is a smaller edit than depth 8, and depth 8 moved the tree hard (-27.8/-18.2/+78.4% nodecheck)
   for -0.7. A half-extension has less mechanism, not more.

T20-b (depth <= 6) stays open at the bottom of the queue, to be run only if the node-level lane is
re-motivated by new evidence. **T15 (capture-margin depth-scaling) is unaffected by this verdict** — it is a
different code path and its own premise (flat -100 vs the quiets' -80*depth) still stands.
