# 2026-08-03 Amplification ratio — a candidate gate diagnostic, PROPOSED not adopted

**Zero box cost. No code changed.** Arose from a gap in my own gating of T19b: the T18b rule says *"a big
node delta from a low fire rate is a warning, not a green light"*, but nothing in the gate ever **computed**
that as a number, so T19b passed its gate without the check being applied.

## Definition

```
amplification = |nodecheck delta %| / (fire rate %)
```

i.e. **how much tree each affected node controls.** A change touching many nodes shallowly has low
amplification; one touching a handful of nodes that each gate huge subtrees has high amplification.

## The four candidates where both numbers are known

| candidate | \|delta\|% | fire % | **amplification** | outcome |
|---|---|---|---|---|
| T19 — ttPv -> LMR | 58.2 (mid) | 9.7 | **6x** | **KEPT +7.3** (full H1) |
| T12 — NMP cut-node only | 30.1 (kiwipete) | 3.1 | **10x** | **KEPT +2.2** (provisional, since certified) |
| T19b — ttPv -> LMP | 36.1 (mid) | 1.3 | **28x** | **SHELVED, penta -1.0** (resolved 2026-08-03) |
| T18b — probcut TT store | 25.5 (kiwipete) | 0.47 | **54x** | **-22.5** (largest negative of the campaign) |

**The ordering is monotone: the two keeps sit at 6x and 10x, the worst result of the campaign at 54x.**

**Plausible mechanism, and it generalises T18b's lesson rather than restating it:** high amplification means
the change acts at a few very high-leverage nodes, so an error there propagates across an enormous subtree.
Low amplification means it acts broadly and shallowly, so individual errors average out. That is the same
intuition T18b produced ("0.47% of nodes removed a quarter of the tree = ~50x, which was measuring error
surface, not savings") expressed as a computable quantity instead of a narrative.

## Caveats — this is NOT adopted as a gate, and the reasons matter

1. **Four data points.** Setting a numeric threshold on four points would be exactly the hand-picked-constant
   error this campaign has made repeatedly (T9a's margin, T7's coefficient, T6a's divisor, T15's scaling).
   **No threshold is proposed.**
2. ~~T19b's row is IN FLIGHT~~ **RESOLVED 2026-08-03: T19b capped at penta -1.0 and SHELVED, so the row
   stands at 28x = miss and the monotone ordering HOLDS.** The caveat that a keep would break it did not
   fire. This is now four decided points — still far too few for a threshold.
3. **T19's fire rate is an approximation.** It was measured at LMP sites (9.7% mid) and applied to LMR sites.
   Same node population, different consumer — close, but not the same measurement.
4. **Selection effect.** Both keeps come from this session, both from the same author, and the delta chosen
   per row is the largest-magnitude position rather than a principled aggregate.
5. **Correlation, not causation.** Four candidates that differ in many ways.

## Proposed action — gather, do not gate

**Add to the gating checklist: COMPUTE and RECORD the amplification ratio for every candidate that passes the
>=1% nodecheck gate.** It costs one division once the fire rate is known, and the fire rate is already
measured for most candidates (T12's blocked-attempt probe, T18b's `pcut` count, T19b's ttPv width).

**Do not reject on it.** Revisit once there are ~10 points with real verdicts. If the monotone relationship
survives, it becomes a genuine pre-launch filter; if it does not, this file is the record of a plausible idea
that did not hold.

**One thing it already earns:** the fire rate should be measured **at gate time, not after launch**. For
T19b it was measured only after the run started, which is why the amplification was not available when the
launch decision was made.

```yaml
id: 2026-08-03-amplification-ratio
date: 2026-08-03
change_class: proposed gate diagnostic (not adopted)
result: >
  amplification = |nodecheck delta %| / fire rate %. Across the four candidates where both are known the
  ordering is monotone with outcome: T19 6x KEPT +7.3, T12 10x KEPT +2.2, T19b 28x SHELVED (penta -1.0,
  resolved 2026-08-03), T18b 54x -22.5.
status: GATHER ONLY -- no threshold proposed on 4 decided points.
action: compute and record amplification for every candidate passing the >=1% nodecheck gate; measure fire
  rate AT GATE TIME rather than after launch; revisit at ~10 points.
```


---

## RETRACTION 2026-08-03 10:30 — the diagnostic is ILL-DEFINED and the monotone ordering may be an ARTIFACT

Gating T22 (probcut restricted to cut nodes) required computing its amplification, and doing so exposed that
**the four rows above do not share a denominator.**

| row | what "fire %" actually measured |
|---|---|
| T18b 0.47% | **of NODES** (1622 probcut stores of 346662 nodes) |
| T12 3.1% | **of ELIGIBLE NMP sites** |
| T19 9.7% | **of LMP-eligible sites** |
| T19b 1.3% | **increment of LMP-eligible sites** |

**Only T18b — the row anchoring the high end — used "% of nodes". The other three used "% of eligible
sites", a denominator smaller by two to three orders of magnitude.** Since amplification divides by that
figure, mixing the two systematically inflates T18b relative to everything else, which is exactly the shape
of the "monotone ordering" I reported.

T22 shows how large the effect is. Same candidate, same tree delta, two denominators:

- fire as **% of nodes**: 11.0 / 0.155 = **71x** (T18b territory — looks alarming)
- fire as **% of eligible**: 11.0 / 24.0 = **0.46x** (lowest on the board — looks excellent)

**A diagnostic whose value spans 150x depending on an unstated convention is not a diagnostic.**

### Status: RETRACTED as proposed. Not "gather more points" — the definition itself has to be fixed first.

The underlying intuition may still be sound (T18b's own finding stands on its own terms: 0.47% of nodes
removing a quarter of the tree). But the ratio as I defined it is unusable until the denominator is pinned,
and **the honest position is that the monotone ordering I reported yesterday is not evidence of anything.**

**If revisited, "% of eligible sites" is the defensible choice** — it asks *"of the places this mechanism
could act, how many did the change touch?"*, which is the question that makes the leverage argument. "% of
nodes" conflates a change's leverage with how common its host mechanism is. But **that has to be recomputed
consistently across all rows before any ordering is claimed**, and until then nothing here should be cited.

**Process note: this is the second self-inflicted measurement error in two days** (the other being
observation-time-vs-event-time on the T8 rate). Both share a root cause — **a number was computed and
reported before its definition was pinned down.**
