# 2026-07-26 Plateau diagnosis — why the mill stalled at 2700, measured

**Trigger.** The pin has moved 2632 → 2704 slowly enough to read as stuck, while Stash sits at **3374 with
zero NNUE** and Counter 4.1 at 3157 as the last pure HCE. The gap is not a classical-engine ceiling. This
measures where it actually goes, because the strategy docs driving the queue turned out to be stale.

## Finding 1 — the EBF lane MOVED, and every recorded number for it was out of date

`scripts/ebfprobe.py`, depth reached at 400K fixed nodes, HEAD vs the script's own standing baseline
(2026-06-11, post-W1):

| Position | Jun-11 baseline | HEAD (2026-07-26) | Δ plies |
|---|---|---|---|
| quiet-mg-QGD | 12 | **14** | +2 |
| quiet-mg-closed | 9 | **12** | +3 |
| lateMg-ph6-11 | 11 | **13** | +2 |
| najdorf-mg | 9 | **14** | +5 |
| rook-eg-R4P | 14 | **17** | +3 |
| pawn-eg | 24 | **32** | +8 |

Implied EBF (N^(1/d)) at HEAD: QGD 2.38, closed 2.73, lateMg 2.70, najdorf 2.51, rook-eg 2.12, pawn-eg 1.49.

**Middlegame EBF is now ~2.4-2.7, NOT the ~3.0 recorded in `project_ebf_lane` and the ebfprobe header.**
Both are stale by ~6 weeks of kept work (T5 aspiration, the corrhist family, T1b/T1e). The lane was never
"exhausted" — it has been quietly delivering, and nobody re-measured.

**The remaining gap is still the whole ballgame.** Elite class is ~2.0. At QGD's 183331 nodes, EBF 2.5 → 2.0
moves depth 14 → **17.5**. With d12-vs-d10 measured at **+185 Elo**, three-plus plies at equal nodes dwarfs
anything the +4-median heuristic mill produces. **2800 is an EBF problem, not an eval problem.**

## Finding 2 — ordering is TT-starved, and that is the specific lever

Measured on HEAD via the `sd_order` / `sd_band` diagnostics:

| Metric | kiwipete d12 | mid d12 |
|---|---|---|
| First-move cutoff rate (fmc/bcut) | 25896/29170 = **88.8%** | 13488/16177 = **83.4%** |
| Move-loop nodes WITH a TT move (ttlist/mln) | 23535/49522 = **47.5%** | 9323/27565 = **33.8%** |
| Cutoffs attributable to the TT move | 12876 = **44%** of cutoffs | 4399 = **27%** |
| Cutoffs from captures | 13656 = 47% | 6802 = 42% |
| Cutoffs from killers / counter / history | 1447 / 292 / 684 = 8.3% | 3210 / 852 / 914 = 30.8% |

**The headline: the TT move is the single strongest ordering signal — 27-44% of all cutoffs — yet it is
present at only 34-47% of move-loop nodes.** Everything that raises TT-move availability raises FMC directly,
and FMC is what gates the entire T3 LMR-reduction family (T3a proved reducing more at 84% FMC costs -7.9).

FMC by remaining-depth band (kiwipete): d1-2 **87.9%**, d3-5 90.3%, d6-9 93.2%, d10+ 96.7%.
mid: d1-2 **83.8%**, d3-5 82.2%, d6-9 82.2%.

**Ordering is WORST at d1-2**, which is where the overwhelming majority of nodes live, so gains there compound
hardest. The quiet/positional `mid` position is ~5 points worse than tactical kiwipete across every band —
consistent with the loss autopsy's "positional slow-bleed" finding.

## Finding 3 — this retro-explains the whole campaign

Sorting every result since the reset by what it touched:

- **Search-efficiency / TT changes**: T5 aspiration **+20.2** (biggest keep of the campaign), T1b **+13.1**,
  T4 corrhist **+9.1**. Plus the strongest gated candidate now queued, T18b, at **-25.5%/-28.5% nodes**
  (**T18b subsequently came back -22.5 — see CORRECTION 2; this column's cut is not where the signal is**).
- **Pruning-constant changes**: T7 **0**, T10b **0**, T9a **-5.2**, T1c **-4.8**, T13 **-0.4**, T16 **-0.6**,
  T3a **-7.9**.

**Every meaningful win came from search efficiency; nearly every null and negative came from tuning pruning
constants.** The mill has been fishing in the wrong pond, and the +4-median model was measured on the wrong
population.

## Strategic consequence — reorder the whole queue around EBF/TT

**Promote (touch TT retention or ordering, i.e. move EBF toward 2.0):**

1. ~~**T18b** — probcut TT store. Adds proved bounds to the TT, which raises ttlist directly. Its
   -25%/-28% node reduction is exactly the predicted signature.~~ **SHELVED -22.5, LANE CLOSED — see
   CORRECTION 2.** The bounds are *speculative*, not "proved", and the -25%/-28% signature was the error
   surface growing, not savings.
2. **T11a — 4-way TT clustering.** Was DOWNGRADED to "+7.5 prior, aging/depth-preferred already exist". That
   downgrade judged the wrong axis: the question is not replacement policy quality but **retention under
   pressure**, and ttlist at 34-47% is the direct measurement of the deficit. **Re-promote to top structural
   candidate.** Note the fixed-node probe UNDERSTATES this (350K nodes into 8.4M slots is 4% occupancy) —
   real games at 10+0.1 accumulate millions of nodes across a game and actually pressure the table, which is
   why a games gate rather than a probe must decide it.
3. **T19 — ttPv.** Already implemented/gated; also TT-borne.
4. **T10 ordering family** — the FMC lever, and the gate on the entire T3 LMR lane. T10b's uniform-weight
   failure did not test the lane, only that one weighting.

**Demote:** further pruning-constant work (T15, the T7/T9a reopens, cutoffCnt) and the eval-shape lane.
Constant-tuning is where six of seven nulls came from.

**T8 SPSA stays last** — it tunes constants, and by this diagnosis constants are the low-yield axis; it also
must run after the structural queue stops moving the base (see its abort record).

## Actions taken from this diagnosis

- Stale numbers corrected on sight: `scripts/ebfprobe.py` header baseline and the `project_ebf_lane` memory
  both said mg EBF ~3.0 / depths 9-12. Both now record the measured HEAD values.
- Queue reordered in `TODO.md` around the EBF/TT axis rather than prior size.
- **Re-measure cadence added**: ebfprobe + the fmc/ttlist table are cheap (~4 min local, zero box time) and
  must be re-read after every structural keep. Letting them go stale for six weeks is what allowed the "lane
  exhausted" story to persist while the lane was in fact delivering.

---

## CORRECTION (same session, from measurement): b_all is STIFF and is the wrong steering metric

Two probes were run specifically to test whether b_all is actionable. **Both say no.**

**Probe 1 — capture-LMR (reduction).** b_all kiwipete 6.38 -> **6.54 (UP)**, QGD 6.42 -> 6.43 (flat).
Obvious once measured: `b_all = alltried/allnodes` counts moves TRIED at an all-node, and LMR still tries every
move, only shallower. **Reduction cannot move b_all by construction.** An earlier version of this file and the
`project_ebf_lane` memory both claimed capture-LMR was "the most direct b_all lever" — that was wrong and is
now corrected in both places.

**Probe 2 — SEE-prune range 4 -> 8 (real pruning, on both the capture and quiet paths).** This is genuine
skipping, not reducing, and it visibly pruned more (kiwipete seeq 13040 -> 18545). b_all kiwipete
6.38 -> **6.33**, QGD 6.42 -> **6.31**. **A doubling of the pruning range bought 1-2% of b_all**, nowhere near
the 6.4 -> 4.0 the EBF arithmetic asks for.

**Conclusion: b_all is structurally pinned**, not a knob. It is held up by the exemption guards every prune
carries (TT move exempt, `legalTried > 0` F1 guard, promotions exempt, killers/counter exempt) and by the fact
that all-nodes cluster at shallow depth where the depth-gated prunes barely apply. Also worth stating plainly:
**"class ~3.5-4" was never measured on a real engine** — it is back-derived from EBF ~= sqrt(b_cut*b_all) at
elite EBF 2.0. Steering hard on a derived target that two independent levers cannot move is how a lane gets
declared "exhausted" while nothing was actually tested.

**Use depth-at-fixed-nodes as the working metric instead.** It responded to both probes where b_all did not:

| Probe | kiwipete depth @400K | QGD depth @400K | b_all |
|---|---|---|---|
| base | 12 | 13 | 6.38 / 6.42 |
| capture-LMR (T3b) | **13** | 13 | 6.54 / 6.43 |
| SEE-range 4->8 (T20) | **13** | 13 | 6.33 / 6.31 |

## Two new gated candidates from this work

- **T3b — capture-LMR** (`output/t3b.patch`, 70 lines). NGN reduced zero captures; now non-winning captures
  (i.e. `!seeNonLosingByMVV`) enter LMR at one ply LESS reduction than quiets, with the quiet-history and
  killer adjustments correctly bypassed for them. +1 ply kiwipete, FMC 88.5 -> 89.9%, LMR re-search churn
  unchanged at 0.2%. Nodecheck -29.6 / +35.4 / -1.4 %. **Explicitly NOT inheriting T3a's gate**: T3a reduced
  QUIETS harder at ~84% FMC and over-reduced mis-ordered moves; captures are a separately- and well-ordered
  population.
- **T20 — SEE-prune range 4 -> 8** (`output/t20-seerange.patch`, 22 lines). The June census flagged that NGN's
  SEE-prune dies at depth<=4 while the class runs to ~8; futility was extended to 8 in W1 but SEE-prune never
  was. +1 ply kiwipete, FMC QGD 81.6 -> 82.3%. History pruning is still capped at depth<=3 — the same
  unfinished extension, kept as a follow-up rather than bundled.

Both are launch-ready behind T18b. Neither is a b_all play; both are depth-at-fixed-nodes plays.

---

## CORRECTION 2 (2026-07-26, from games): Finding 3's split was the WRONG CUT

**T18b came back -22.5 [-36,-10], H0 ACCEPTED** — the largest single negative of the campaign, on the
candidate this document ranked **#1** and used as the flagship of the promoted class
(`experiments/2026-07-26-t18b-probcut-ttstore.md`). Its implementation was checked post-hoc and is a faithful
SF transplant (stored depth == proven depth), so the idea failed, not the code.

**Finding 3 above sorted results into "search-efficiency/TT wins" vs "pruning-constant losses" and re-ranked
the entire queue on that split. The split is not wrong so much as cut in the wrong place.** The real
discriminator:

> **Does the change PERSIST SPECULATIVE INFORMATION?**

Every winner in the "efficiency" column used **proven or games-verified** information — T5 aspiration invents
none (pure window management), T1b/T1e reallocate time, T4 corrhist's learned correction was itself validated
by games. T18b made the search cheaper by **caching a guess**: probcut is a self-described speculative prune
("the full search would *likely* also fail high"), and storing its result turns a one-shot bounded error into
a durable assertion re-fired on every revisit.

**Consequence for how this document's own instrument is read.** The -25.5%/-28.5% nodecheck delta was cited
here as the strongest signal on the board. It should have been read as a red flag: the store fires at **0.47%
of nodes** yet removed a quarter of the tree — **~50x amplification per entry, which is equally the
error-multiplication factor.**

> **Node-count reduction is evidence of efficiency ONLY when the information driving it is proven.** When the
> reduction comes from caching a heuristic, the node delta measures **error surface, not savings** — and a
> large delta from a *low* fire rate is a warning sign, not a green light.

The `>=1% nodecheck` gate now carries a second question: not just *did the tree move*, but **what kind of
information moved it**.

**This does NOT retract the EBF/TT re-ranking** — it sharpens it, and it discriminates rather than merely
retro-explaining. **T11a keeps its top-structural rank** (it retains *real search results* already proven by
full searches, inventing nothing), **T19 ttPv survives** (a structural mark, not a speculative value), and
**T3b is unaffected** (it reduces; it does not cache). Findings 1 and 2 (the stale-EBF re-measurement and the
34-47% ttlist deficit) are untouched by this result.

**T18 is now fully closed, both halves** — T18a was killed at the gate (-0.15%, re-classed as pure speed).
