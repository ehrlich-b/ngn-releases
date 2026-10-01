# 2026-08-01 T15 — SEE capture-prune depth scaling — BUILT, GATED, STAGED

**STATUS: built, gated, staged on the box with the on-box hash verified. NOT LAUNCHED.** Runs after T19.
Engine tree REVERTED and clean; nodecheck re-verified at exactly 346662/149587/765656. Change lives only in
`output/t15.patch`.

## What it is

A one-line threshold reshape at `search.go:1714`. Captures are pruned at a **flat `SEE < -100` for all
depth <= 4**, while the **quiet** SEE path eight lines below scales at **`-80*depth`** with an explicit code
comment stating that deeper nodes should prune only larger losses. T15 mirrors that existing convention onto
the capture path: **`SEE < -100*depth`**.

**This candidate invents no shape — it applies the engine's own stated convention to the one path that
lacks it.** At depth 1 behavior is unchanged (`-100*1 == -100`); at depth 4 a capture must lose more than 400
cp to be discarded instead of 100 cp. So it **prunes strictly LESS**, and only at the deeper end of the
depth<=4 window, which is the most tactically dangerous place to be throwing captures away.

## Provenance — this premise SURVIVED T20's falsification

The flat-vs-scaled asymmetry was discovered while gating T20 (2026-07-26). T20's proposed split variant
(T20q, quiet-only) was **tested and falsified** on depth-at-fixed-nodes, and T20 itself later shelved as a
capped null (-0.7). **T15 was explicitly recorded as UNAFFECTED by that outcome: different code path, and its
flat-margin premise stands.** That is re-affirmed here — T20 changed the SEE-prune *range* (depth<=4 -> 8) on
both paths; T15 changes the capture *threshold shape* within the existing range. Disjoint edits.

## Gates

### Nodecheck — PASSES, reproduces the 2026-07-25 measurement

| position | base | T15 | delta |
|---|---|---|---|
| kiwipete d12 | 346662 | **359952** | **+3.8%** |
| mid d12 | 149587 | **167308** | **+11.8%** |
| end d16 | 765656 | **666817** | **-12.9%** |

Matches the recorded +3.8/+11.9/-12.9%, from a fresh build both sides. **This is the smallest tree
disturbance of the four candidates staged this session** (T4e +48.9% mid, T19 +58.2% mid, T12 +30.1%
kiwipete), which at fixed real clock is the cheapest headwind of the set.

### The T18b soundness question — PASSES

*Does the change persist speculative information?* **No.** SEE is an **exact material-swap computation** —
proven information, nothing written to the TT, nothing cached. This is the structural opposite of T18b's
stored guess. The residual speculation is the **threshold** itself (SEE cannot see promotion, king activity,
or attacking compensation), which is precisely what the change makes *more* conservative rather than less.

### Depth at fixed nodes — -3 net plies, NOT a rejection

| position | base d | T15 d | delta |
|---|---|---|---|
| quiet-mg-QGD | 14 | 12 | **-2** |
| quiet-mg-closed | 12 | 12 | 0 |
| lateMg-ph6-11 | 13 | 13 | 0 |
| najdorf-mg | 14 | 13 | **-1** |
| rook-eg-R4P | 17 | 17 | 0 |
| pawn-eg | 32 | 32 | 0 |

Under the 2026-08-01 narrowing, the depth-loss rule rejects only **depth-BUYING** candidates. **T15 prunes
strictly LESS**, so it is accuracy-buying and a depth cost is its expected signature (the kept T4c corrector
measures -1). **Both endgames are exactly flat** — including identical node counts at the same depth
(354408/354419 and 325143/325143), i.e. the change is nearly inert in the endgame, which is consistent with a
capture-pruning threshold that mostly binds in tactical middlegames.

Standing comparison across this session's accuracy-buying candidates: kept-T4c **-1**, T12 **-2**, T15 **-3**,
T19 **-4**, T4e **-6**.

### Suite

`go test -short ./engine`, `-race ./engine`, `./...` — all green. No new test: a one-line threshold reshape
with no new state, covered by the existing SEE tests and nodecheck.

## PRE-REGISTERED EXPECTATION AND FAILURE HYPOTHESIS — written before launch, outcome unknown

**Honest expectation: +2 to +6 class, and a null is very plausible.** It is a small, principled edit with a
small tree delta — the profile of a change that is either a quiet few Elo or nothing.

**The base-rate warning is the heaviest fact here: this is a node-level pruning-constant change, the family
that is 1 keep / 11 attempts for +2.0 Elo total, and the plateau diagnosis found that "nearly every null came
from tuning constants."** T15 is a better-argued member than most (it applies an existing in-engine
convention rather than a hand-picked number, and the number it introduces is derived from the neighbouring
path), but it does not escape the family prior. **Ranked last of the four staged candidates for exactly this
reason.**

**Leading failure hypothesis if T15 rejects:** the flat `-100` was already near-optimal because the
`depth <= 4` window is shallow enough that depth-scaling has almost nothing to act on — only depths 2-4
change at all, and the +3.8%/+11.8% tree growth buys back captures that were mostly correctly pruned.
**That would be a clean, informative null: it would close the flat-vs-scaled asymmetry question that T20's
gating opened**, and it should NOT be followed by a re-run at a different coefficient (`-60*depth`,
`-120*depth`) — that is the hand-picked-constant class that keeps failing.

**Interaction note carried forward from T20's gating:** if a future SEE-range extension (T20-b, depth<=6)
ever keeps *and* T15 keeps, the composition of extended-range + reshaped-margin needs an **explicit
interaction check** rather than assumed additivity. T20-b is currently ranked LOW.

```yaml
id: 2026-08-01-t15-see-capture-depth-scaling
date: 2026-08-01
change_class: search heuristic (capture SEE-prune threshold reshaped to depth-scaled, mirroring the quiet path)
hypothesis: >
  Depth-scaling the capture SEE-prune threshold to -100*depth, matching the quiet path's existing -80*depth
  convention, stops discarding marginally-losing captures at the deepest and most tactical end of the
  depth<=4 window. Expect +2 to +6, null very plausible.
base_commit_or_patch: HEAD (clean tree, nodecheck 346662/149587/765656 re-verified from a fresh build)
candidate_commit_or_patch: output/t15.patch
base_binary_sha256: 0a8f65b75e59c376052f83b14787b0af2eba65552cedfee6f70ce576891ca721   # on-box ngn_t5.exe
candidate_binary_sha256: 2a013a0af12438a5648bf35c6e34a8b7de0ac47ca264471e5c12ff81e7c7295d  # on-box ngn_t15.exe, hash verified after push
harness: sprt.exe sha256 30c33e0512725b7f552d8a1cf72ba6f1e0deb4dcb122c6b6ba8f6433c686a762 (M5-validated mill, untouched)
command: sprt.exe -new .\ngn_t15.exe -base .\ngn_t5.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 8000 -mingames 300 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
machine: AMD Ryzen 7 9800X3D 8c/16t, native Windows (LAN box 192.168.4.108)
tc: 10+0.1 (seconds)
concurrency: 8
openings_path: sprt_openings.txt
openings_sha256: 974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222
adjudication: STANDARD-ON (M5-validated flags)
decision_rule: >
  H1 accept (pLLR >= +2.94) = KEEP. H0 accept (pLLR <= -2.94) = SHELVE. At the 8000g cap a PROVISIONAL keep
  requires point estimate >= +1; below +1 shelves regardless of pLLR sign. Verdict from the harness H0/H1
  line plus a full post-mingames envelope scan, NEVER the printed final pLLR alone. HALT if flag-outs are
  non-zero on either side.
next_action: launch after T19 resolves; rebuild on whichever tree is current if any earlier candidate kept -- do NOT pool
```

---

## POST-VERDICT PROCEDURE — written 2026-08-02 13:20 at G2708, OUTCOME UNKNOWN

T15 is **oscillating around zero**: elo -7.0 (G768) -> +4.2 (G1758) -> **-2.3 (G2708)**, pLLR -0.56 -> +0.78
-> -0.65. That is what a null looks like at this sample size, but it is **not** a verdict and nothing below
is conditioned on the sign.

**Decision rule, unchanged:** pLLR >= +2.94 = full KEEP; <= -2.94 = SHELVE; at the 8000g cap a PROVISIONAL
keep needs **penta point estimate >= +1.0 inclusive** (below +1 shelves regardless of pLLR sign). **Decision
statistic is the PENTANOMIAL**; verdict from the harness's own H0/H1 line plus a full envelope scan, never
the printed final pLLR (c8-drain seen 6x). **Note T15 is measured on the T12+T19 composed base, so its number
is a MARGINAL contribution given both.**

### If KEEP (full H1, or penta >= +1 at the cap)

1. **Commit** `output/t15.patch` onto current HEAD (= T12+T19). One-line threshold change, no new state.
2. **Verify BEHAVIOURAL identity, not byte identity** (Go VCS stamping — 2026-08-02 process finding): the
   rebuilt tree must reproduce **316808 / 355965 / 667710**, the pre-measured T12+T19+T15 composition.
3. **RE-LOCK nodecheck baselines** in `scripts/nodecheck.sh` from **295507 / 112109 / 667703** to
   **316808 / 355965 / 667710**, reason in the file header.
   **Expect the middlegame to jump 3.2x (112109 -> 355965).** That is the measured three-way non-additivity,
   **already known before the run** — it is not a surprise and not a reason to re-open the verdict. Per the
   2026-08-02 correction, that cost is **inside** the number T15's own SPRT just measured: if T15 kept, the
   tree growth was paid for and worth it.
4. **Batch 3 row 3**; batch net becomes +9.5 + T15's figure.
5. **Re-measure cadence**: ebfprobe + `output/fmc-table.sh` on the new HEAD. **FMC movement is NOT a strength
   signal here** — T15 is a pruning-threshold change, not an ordering change, and T19 established that FMC is
   valid only for ordering candidates.
6. **Then the Batch 3 certification**, per the early-cert decision:
   `-new .\ngn_t15on1219.exe -base .\ngn_cert2.exe -elo0 0 -elo1 6`.

### If SHELVED (penta < +1, or pLLR <= -2.94)

1. Nothing to revert — the tree never carried T15. **Baselines stay 295507 / 112109 / 667703.**
2. Apply the pre-registered reading **verbatim**: the leading hypothesis is that **the flat `-100` was already
   near-optimal because the `depth <= 4` window is too shallow for depth-scaling to act on** — only depths 2-4
   change at all. **Do NOT re-run at a different coefficient** (`-60*depth`, `-120*depth`); that is the
   hand-picked-constant class that has failed repeatedly. A clean null here **closes the flat-vs-scaled
   asymmetry question that T20's gating opened**, which is a real result rather than an absence of one.
3. **Node-level family would go to 2 keeps / 13 attempts.** State it plainly.
4. **Then the Batch 3 certification** on the unchanged composite:
   `-new .\ngn_t19on12.exe -base .\ngn_cert2.exe -elo0 0 -elo1 6`.

**Either branch: the certification is the next box job, NOT T8** — T8's base contains this composite and a
25-hour tuner must not run against an unconfirmed one. Both cert binaries are already on box and
hash-verified. Verify topology (1 sprt + 16 engines) after launching.

---

# RESULT 2026-08-02 — SHELVED. Clean statistical reject, penta -4.7 [-11, +1].

```text
=== RESULT (6h14m52s) ===
Games: 6012   W-D-L: 1529-2873-1610   score: 49.3%
Elo(new - base): -4.7   95% CI [-13, +4]
LLR: -2.68   bounds [-2.94, 2.94]
Pentanomial [LL 176  LD 743  {LW,DD} 1227  WD 706  WW 154] over 3006 pairs
Penta Elo: -4.7   95% CI [-11, +1]   pLLR -3.04  (THE decision stat)
Flag-outs (lost on time): new 0, base 0  of 6012 games  (goal: 0)
Adjudicated early: 3123 decisive, 1434 draw  of 6012 games
Verdict: H0 ACCEPTED: new is NOT better (<= -3 ELO)
DONE_EXIT_0
```

- pLLR **min -3.08**, first crossing of -2.94 at **G6004**, **10 samples at/below**, zero above.
  Envelope **[-3.08, +1.00]**, elo [-15.0, +5.5].
- **c8-drain, 7th observation:** printed -3.04 vs true envelope minimum -3.08. Verdict taken from the
  harness `H0 ACCEPTED` line plus the scan, as always.
- 0/0 flag-outs / 6012g / 6h14m52s, ~963 g/hr. Engine tree never carried the change.
- **Measured on the T12+T19 composed base**, so -4.7 is T15's marginal contribution given both.

## Pre-registration applied verbatim

Expectation was **"+2 to +6 class, and a null is very plausible"**; the outcome is a **clean reject at -4.7**,
i.e. worse than the stated range. **The heaviest pre-registered fact was the base rate, and it held:** this
was flagged before launch as *"a node-level pruning-constant change, the family that is 1 keep / 11 attempts,
and the plateau diagnosis found that nearly every null came from tuning constants"* — and it was **ranked last
of the four staged candidates for exactly that reason.** It finished last.

**The leading failure hypothesis, pre-registered, stands as the reading:** the flat `-100` was already
near-optimal because the `depth <= 4` window is shallow enough that depth-scaling has almost nothing to act
on — only depths 2-4 change at all — so the tree growth bought back captures that were mostly correctly
pruned.

**What this null CLOSES, which is a real result rather than an absence of one:** it settles the
**flat-vs-scaled asymmetry** question that T20's gating opened on 2026-07-26. The capture path prunes at a
flat `SEE < -100` while the quiet path scales at `-80*depth`; that asymmetry looked like an oversight against
the engine's own stated convention. **It is not an oversight — mirroring the convention onto the capture path
measurably loses Elo.** The asymmetry is now explained rather than outstanding.

**NOT licensed, per the pre-registration:** do **NOT** re-run at a different coefficient (`-60*depth`,
`-120*depth`). That is the hand-picked-constant class that has failed repeatedly, and the null is about the
shape of the window, not the size of the number.

## Standing

**Node-level pruning/reduction family: 2 keeps / 13 attempts.** The one keep (T12) was argued from
**soundness** — removing a prune whose premise is contradicted at all-nodes — while T15 was argued from
**convention symmetry**. That distinction now has two data points behind it and is consistent with the
campaign-wide pattern that mechanism-level changes keep and constant-shaped changes do not.

**Session scoreboard (2026-08-01 -> 08-02): T4e -20.9, T12 +2.2 (provisional), T19 +7.3 (full accept),
T15 -4.7.** Batch 3 holds T12 + T19 at a claimed **+9.5**, and its certification is now live.
