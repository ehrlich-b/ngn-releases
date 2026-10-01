# 2026-08-01 T4e — material correction history (fifth corrector) — BUILT, GATED, STAGED

**STATUS: built, gated, staged on the box with the on-box hash verified. NOT LAUNCHED — M6-retest owns the
box until ~19:41 EDT.** Launch is one command once M6 resolves. Engine tree is REVERTED and clean; nodecheck
re-verified at exactly 346662/149587/765656 from a fresh build. The change lives only in
`output/t4e-material-corrhist.patch` (sha256 `64782a9b809ceeee147ac74692ac2a77cb27f71273e0aba10ae400b03c53706b`).

Key ranking that produced this candidate: `experiments/2026-07-26-corrector-key-ranking.md`.
Its predecessor's verdict, which constrains how this one may be justified:
`experiments/2026-07-26-t4d-continuation-corrhist.md`.

## What it is

The **fifth** correction-history table (fourth still live — T4d was shelved), keyed on **material COUNTS**.
The three shipped correctors all ask *where are the pieces* (pawn skeleton, non-pawn occupancy, minor
occupancy); this asks *what pieces are left*. Both sides' piece counts (kings excluded — always one each)
pack into a base-17 signature which is then hashed into the shared 16384-slot table. The mixing form is
copied byte-for-byte from the residual-ranking pre-flight so the shipped key is provably the key that was
scored.

It rides the identical T4 plumbing as the other three: applied at the qsearch stand-pat (via
`correctedStandPat`) and at the interior static-eval site, learned from the pre-S7 `corrStaticEval`, same
gravity EMA, same `6245/131072` weight, same `corrHistLimit`.

## HOW THIS CANDIDATE MAY AND MAY NOT BE JUSTIFIED — binding, inherited from T4d

T4d's rejection **demoted the residual-R² pre-flight to rejection-only**. So:

- **T4e's R² of 0.290 (55x chance, the strongest untried key) is NOT admissible as promotion evidence.**
  It is recorded because it is why the candidate was built, and it is *not* being counted toward its rank.
- T4e's rank rests on exactly two things: it is a **position-feature key**, the class that is **3-for-3**
  (pawn +9.1, non-pawn +6.4, minor +3.7) with the family's only miss isolated to the move-sequence class;
  and it is the **last unmined member** of that class.
- The phase confound was tested and rejected before T4d (joint material x phase adds only +0.002 over
  material alone), so the key is not a phase proxy. That remains true and is a *mechanism* fact, not an R²
  promotion argument.

## Gates

### Nodecheck — PASSES the >=1% behavioral-delta gate decisively

| position | base | T4e | delta |
|---|---|---|---|
| kiwipete d12 | 346662 | 386920 | **+11.6%** |
| mid d12 | 149587 | 222673 | **+48.9%** |
| end d16 | 765656 | 687889 | **-10.2%** |

Measured from a **fresh** build both sides (the `nodecheck.sh`-does-not-rebuild trap). The base binary
reproduces the locked baseline exactly, so the deltas isolate the change.

### The T18b soundness question — PASSES

*What kind of information moved the tree?* Corrhist persists a **learned correction derived from completed
search results**, under the same gate and clamp as three correctors whose value was established **by games**.
It does not cache a speculative prune the way T18b's probcut store did. Same answer the family has always
given.

### Depth at fixed nodes — **-6 net plies, and this is NOT a rejection. The rule was calibrated first.**

| position | base d | T4e d | delta |
|---|---|---|---|
| quiet-mg-QGD | 14 | 15 | **+1** |
| quiet-mg-closed | 12 | 12 | 0 |
| lateMg-ph6-11 | 13 | 12 | -1 |
| najdorf-mg | 14 | 13 | -1 |
| rook-eg-R4P | 17 | 15 | -2 |
| pawn-eg | 32 | 29 | -3 |

The 2026-07-27 T20 lesson makes ebfprobe **valid for rejection** ("a depth LOSS predicts trouble"), which on
its face kills this candidate — the -6 is the largest depth loss on record, worse than T19's -4.

**So the rule was calibrated against an in-class reference with a known games verdict before being applied.**
A binary was built with the **minor corrector disabled** and probed against HEAD, i.e. measuring what the
**KEPT +3.7 T4c change** did to depth:

| position | minor OFF | HEAD (minor ON) | delta |
|---|---|---|---|
| quiet-mg-QGD | 14 | 14 | 0 |
| quiet-mg-closed | 14 | 12 | **-2** |
| lateMg-ph6-11 | 13 | 13 | 0 |
| najdorf-mg | 14 | 14 | 0 |
| rook-eg-R4P | 16 | 17 | +1 |
| pawn-eg | 32 | 32 | 0 |

**T4c measures -1 net ply and it WON +3.7 Elo.** A corrector that keeps loses depth at fixed nodes.

**Conclusion, and it is a reusable one: the ebfprobe depth-loss rejection rule was derived from
depth-BUYING candidates (T6a history-LMR shape, T20 SEE-prune range — pruning/reduction mechanisms) and does
NOT extend to accuracy-buying ones.** Losing a ply is a corrector's *expected signature*: it spends nodes to
evaluate better, so the probe measures its cost side and is structurally blind to its benefit. This is
exactly the argument the T19 record made on 2026-07-26 from first principles; it is now **measured**, against
a change with a real games verdict.

**What the probe DOES establish is the size of the bill, and it is large.** -6 is **6x** the in-class
reference. The rule does not reject T4e, but the magnitude is the leading pre-registered risk below.

### Within-search key cardinality — the mechanism behind the -6, measured

The depth cost has a measured cause. After a real search, counting non-zero slots and mean entry magnitude
per table (temporary probe, since removed; tree clean):

| position | table | slots touched | mean entry | mean correction |
|---|---|---|---|---|
| kiwipete d12 | pawn / nonPawn / minor / **material** | 1210 / 7339 / 1641 / **1896** | 125 / 53 / 124 / **146** | 5 / 2 / 5 / **6 cp** |
| mid d12 | pawn / nonPawn / minor / **material** | 2078 / 6507 / 2109 / **444** | 55 / 25 / 54 / **144** | 2 / 1 / 2 / **6 cp** |
| end d16 | pawn / nonPawn / minor / **material** | 982 / 7364 / 6 / **223** | 120 / 41 / 333 / **305** | 5 / 1 / 15 / **14 cp** |

**Material is a much COARSER key than the three placement keys**, because material changes only on captures
and promotions while placement changes every move. In the middlegame it touches **444 slots vs pawn's 2078
and non-pawn's 6507** — so each slot absorbs far more updates and carries a far larger EMA (mean 144 vs 55
and 25), applying a systematically **larger, more nearly uniform correction: 6 cp vs 2 cp and 1 cp**. A
larger near-constant offset to every static eval moves nodes across the RFP/futility/NMP margin thresholds
wholesale, which is a coherent explanation for both the +48.9% middlegame tree growth and the depth loss.

**This is a warning sign, not a disqualification, and there is a direct in-class precedent: `minor` in the
endgame touches only SIX slots at a mean 15 cp** — an even coarser, larger-offset configuration than
material's — **and minor KEPT at +3.7.** Low cardinality with a big offset is not by itself fatal in this
family.

### Suite

`go test -short ./engine`, `go test -short -race ./engine`, `go test -short ./...` — all green with the patch
applied. Two new tests:

- `TestMaterialCorrectionAppliedAtQsearchStandPat` — poisons one slot and asserts the qsearch stand-pat backs
  up raw + correction, mirroring the pawn/non-pawn/minor tests. **Also asserts `ClearHistoryTable` zeroes the
  new table** — T4d found the hard way that a corrector missing from that function leaks learned state across
  `ucinewgame` and across tests.
- `TestMaterialCorrectionIndexKeysOnCountsNotPlacement` — asserts the key is invariant when only placement
  changes and does change when a knight is removed. Guards the whole point of the candidate: if it aliased
  placement it would be a fourth copy of the existing correctors rather than a new axis.

## PRE-REGISTERED EXPECTATION AND FAILURE HYPOTHESIS — written before launch, outcome unknown

**Honest expectation: +1 to +4, with a materially higher chance of a negative than T4d had.** The family's
own trend is decaying (+9.1 → +6.4 → +3.7), so even a success is expected small, and the batch path's `>= +1`
point-estimate floor applies to any provisional keep.

**Leading failure hypothesis, registered now so it cannot be chosen to suit the result: the key is too
coarse.** If T4e rejects, the measured cardinality data above (444 mid slots, 6 cp mean vs 1-2 cp for the
placement keys) is the pre-committed explanation — a corrector this coarse behaves less like a per-position
correction and more like a learned global eval bias, and the `6245/131072` weight it inherited was calibrated
for a *fine* key that spreads across thousands of lightly-loaded slots.

**What a rejection would license:** exactly one reopen, **T4e-2 = the same key at a reduced weight**, and it
is **deliberately ranked LOW** — it is a hand-picked constant, which is the class that has failed repeatedly
(T9a's margin, T7's coefficient). Prefer exposing the corrector weights as SPSA dims over a second 4-hour box
job on one guess.

**What a rejection would NOT license:** it would **not** close the corrhist family, and it would **not**
revive the move-sequence keys (`cont2`/`cont12` remain unrun by rule). It **would** mean the position-feature
class is 3-for-4 and the family is mined out, which is a real and reportable conclusion.

**If T4e keeps or caps positive:** the position-feature class goes 4-for-4, the family's decay curve extends,
and the sibling **direct imbalance eval term** (encode it rather than learn it) becomes the next candidate.

```yaml
id: 2026-08-01-t4e-material-corrhist
date: 2026-08-01
change_class: search/eval heuristic (fifth correction-history table, material-count key)
hypothesis: >
  A corrector keyed on material COUNTS absorbs the imbalance error NGN's eval has no term for, which the
  three placement-keyed correctors are structurally blind to. Expect +1 to +4.
base_commit_or_patch: HEAD (clean tree, nodecheck 346662/149587/765656 re-verified from a fresh build)
candidate_commit_or_patch: output/t4e-material-corrhist.patch sha256 64782a9b809ceeee147ac74692ac2a77cb27f71273e0aba10ae400b03c53706b
base_binary_sha256: 0a8f65b75e59c376052f83b14787b0af2eba65552cedfee6f70ce576891ca721   # on-box ngn_t5.exe
candidate_binary_sha256: 9e8dc920bb7dc743f3838f55bb4b551a9646c54fa686880673a6d3ca45728688  # on-box ngn_t4e.exe, hash verified after push
harness: sprt.exe sha256 30c33e0512725b7f552d8a1cf72ba6f1e0deb4dcb122c6b6ba8f6433c686a762 (M5-validated mill, untouched)
command: sprt.exe -new .\ngn_t4e.exe -base .\ngn_t5.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 8000 -mingames 300 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
machine: AMD Ryzen 7 9800X3D 8c/16t, native Windows (LAN box 192.168.4.108)
tc: 10+0.1 (seconds)
concurrency: 8
openings_path: sprt_openings.txt
openings_sha256: 974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222
adjudication: STANDARD-ON (M5-validated flags)
decision_rule: >
  H1 accept (pLLR >= +2.94) = KEEP. H0 accept (pLLR <= -2.94) = SHELVE. At the 8000g cap, a PROVISIONAL keep
  requires point estimate >= +1 per the T20 rule-gap closure; anything below +1 shelves regardless of pLLR
  sign. Verdict is taken from the harness H0/H1 line plus a full post-mingames envelope scan, NEVER the
  printed final pLLR alone (c8-drain, observed 4x in both directions).
  HALT if flag-outs are non-zero on either side.
next_action: launch when M6-retest resolves; base is NOT conditional on M6 (M6 is an A/A that changes no engine behavior)
```

## Base is NOT conditional on M6

M6-retest is an A/A of the *ruler* at 5+0.05 — it plays `ngn_t5` against itself and changes no engine
behavior. Whatever it returns, T4e's base stays on-box `ngn_t5.exe` `0a8f65b7…` and T4e's own verdict stays
at **10+0.1**, because M6 on PASS clears 5+0.05 for Stage-1 filters and SPSA batches ONLY — per-change
verdicts stay at 10+0.1 pending M6b.

---

# RESULT 2026-08-01 — SHELVED. Clean statistical reject, and the SECOND-LARGEST negative of the campaign.

Launched 19:15, finished 20:35 (1h19m33s). Read and the next job launched within **4 minutes**.

```text
=== RESULT (1h19m33s) ===
Games: 1280   W-D-L: 292-619-369   score: 47.0%
Elo(new - base): -20.9   95% CI [-40, -2]
LLR: -2.59   bounds [-2.94, 2.94]
Pentanomial [LL 50  LD 165  {LW,DD} 263  WD 136  WW 26] over 640 pairs
Penta Elo: -20.9   95% CI [-34, -8]   pLLR -2.86  (THE decision stat)
Flag-outs (lost on time): new 0, base 0  of 1280 games  (goal: 0)
Adjudicated early: 658 decisive, 297 draw  of 1280 games
Verdict: H0 ACCEPTED: new is NOT better (<= -3 ELO)
DONE_EXIT_0
```

- Penta **-20.9 [-34, -8]**, pLLR **min -2.97**, first crossing of -2.94 at **G1272**, **5 samples at/below**.
- **Post-mingames envelope [-2.97, 0.00] with elo max -1.1 — never once positive after the mingames floor.**
  Like T17 and T18b there is no positive phase to argue about.
- **c8-drain, 6th observation:** printed -2.86 is shallower than the true envelope minimum -2.97. Verdict
  taken from the harness `H0 ACCEPTED` line plus the envelope scan.
- 0/0 flag-outs, 1280g, ~965 g/hr. Engine tree never carried the change.
- **Second-largest negative of the campaign**, behind only T18b's -22.5.

## Pre-registration applied VERBATIM

**Expectation was "+1 to +4, with a materially higher chance of a negative than T4d had". The outcome is
-20.9 — far outside that range and a much worse miss than predicted.** Recording that plainly: the
expectation was wrong in magnitude, not merely in sign.

**Licensed by the rejection, per the manifest:**

- The **position-feature key class is now 3-for-4** and **the corrhist family is mined out**. The manifest
  pre-committed to this being "a real and reportable conclusion", and it is hereby reported. **Family overall:
  3 keeps / 5 attempts, +19.2 Elo.**

**NOT licensed, per the manifest, and still not licensed:**

- The corrhist family is **not closed** as a lane.
- `cont2` / `cont12` remain **unrun by rule** (move-sequence keys, the class T4d indicted).

## The reopen the manifest allowed is now DEAD — on evidence gathered BEFORE the verdict

The manifest licensed **exactly one** reopen: *"T4e-2 = the same key at a reduced weight, deliberately ranked
LOW."* **That reopen is now withdrawn entirely**, and the reasoning is not special pleading — the evidence was
measured **while the run was live at G1004 with the outcome unknown**
(`experiments/2026-08-01-imbalance-eval-probe.md`), and it strengthens rather than softens the rejection.

That probe falsified the premise the whole candidate rested on. The measured residual signal that made
material the top-ranked key is a **recapture artifact**, not an eval defect: prev-CAPTURE residual **128.2 cp
vs prev-quiet 6.6 cp (19x)**, the entire `imbalanced:N` bucket is mid-exchange (mean cp == mean |cp| ==
+304.0, every residual positive), and **conditioned on a quiet previous move, material class explains nothing**
(balanced 8.2, NB 0.1, P 6.9, PNB 7.1).

**So a lower weight cannot help.** The signal being learned is *"a recapture is pending"* — a horizon state
the search already resolves for itself. Reducing the weight only scales the harm toward zero; there is no
gain underneath it to uncover. **A reopen requires a mechanism, and this one has none.**

## Why it lost 20.9 Elo rather than merely failing to gain

The pre-verdict hypothesis, now **corroborated but not proven**: the corrector learns a large positive
offset for material signatures that are frequently mid-exchange, then applies it as a **standing static-eval
correction at every node sharing that signature** — including the positions where the imbalance is **real and
permanent** (a completed trade, a genuine sacrifice), which get inflated by up to ~a minor piece. That is not
a neutral or merely-uninformative correction; it is a systematic eval error injected at high-leverage nodes,
which is consistent with a double-digit loss rather than a null.

This also explains why T4e is so much worse than T4d (-8.2): T4d's move-sequence key aliased badly but
carried no systematic sign, while T4e's key is **correlated with a large, consistently-signed horizon
residual**.

## Standing corrhist ledger after this run

| corrector | key class | result |
|---|---|---|
| pawn | position-feature | **KEPT +9.1** |
| non-pawn | position-feature | **KEPT +6.4** |
| minor | position-feature | **KEPT +3.7** |
| **material (T4e)** | position-feature | **SHELVED -20.9** |
| continuation (T4d) | move-sequence | SHELVED -8.2 |

**3 keeps / 5 attempts. The family is mined out; both remaining key ideas are dead by rule or by mechanism.**
