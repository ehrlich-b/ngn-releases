# 2026-08-01 Imbalance eval "defect" — FALSIFIED. It is a recapture artifact.

**Zero box cost. No code changed; probe shelved to `output/imbalance-probe_test.go.txt`, tree clean.**
**Measured while T4e was LIVE and BEFORE its verdict — timestamped here so nothing below can be read as
retro-fitting an explanation onto a known outcome.** T4e stood at G1004 / elo -14.9 / pLLR -1.55 when this
was written; its outcome was unknown.

## What was being checked

The 2026-07-26 corrector-key-ranking **addendum** reported that mean residual per phase bucket is
non-monotonic with a parity structure — **odd-phase ~99 cp vs even-phase ~54 cp** — and concluded:

> "an odd phase means an odd number of minor pieces on the board, i.e. a material imbalance. So the eval is
> roughly twice as wrong when material is imbalanced ... it is a **missing imbalance term**."

That inference is load-bearing: it **strengthened T4e**, and it spawned **"a direct imbalance eval term"** as
T4e's queued sibling candidate. The addendum itself flagged the right caveat — *"the residual gate is
asymmetric (fail-highs and fail-lows enter on different conditions), which biases the measurement"* — and
called the finding **"suggestive, not established."** This settles it.

## Instrument — ungated and symmetric by construction

60 self-play games from the canonical opening corpus, 40 plies each, **depth 10**, **2400 observations**.
Residual = `searchScore - rawStaticEval`, both side-to-move relative, recorded **at ROOT positions only**.
Every position contributes **exactly once**, with **no gate at all** — which removes precisely the
fail-high/fail-low asymmetry the addendum warned about.

## Result 1 — the parity finding REPLICATES, and is far larger than reported

| bucket | n | mean cp | mean \|cp\| |
|---|---|---|---|
| even phase | 2278 | 18.9 | 45.7 |
| **odd phase** | **122** | **244.7** | **254.0** |

Not 2x — **13x**. So the addendum's observation is real and reproduces on a cleaner instrument.

## Result 2 — where it lives: one class, and the mean equals the mean-absolute

| material class | n | mean cp | mean \|cp\| |
|---|---|---|---|
| balanced (all 5 counts equal) | 1326 | 9.5 | 35.4 |
| imbalanced: **N only** | 47 | **304.0** | **304.0** |
| imbalanced: NB | 369 | 6.5 | 37.5 |
| imbalanced: P only | 383 | 29.0 | 52.4 |
| imbalanced: PNB | 181 | 15.0 | 50.1 |

**`mean cp == mean |cp| == 304.0` is the tell.** Every single residual in that bucket is positive and
~one minor piece in size. That is not "the eval misjudges imbalance" — a genuine eval defect would scatter
in both directions. It is the signature of **positions in the middle of an exchange, where the side to move
is about to recapture**: the static eval correctly reports being a piece down, and the search correctly sees
the recapture, so the residual is ~+300 every time.

Note also that `imbalanced:NB` — where knights *and* bishops both differ, i.e. a **completed**, genuinely
imbalanced trade — sits at **6.5 cp, below the balanced bucket's 9.5.** If the eval had a real imbalance
blind spot, that is exactly where it should show, and it does not.

## Result 3 — the confound test. Decisive.

Re-ran splitting every bucket by whether the **previous move was a capture**:

| bucket | n | mean cp | mean \|cp\| |
|---|---|---|---|
| **prev-CAPTURE (all)** | 469 | **128.2** | 142.2 |
| **prev-quiet (all)** | 1931 | **6.6** | 35.4 |
| imbalanced:N / prev-CAPTURE | 47 | 304.0 | 304.0 |
| *imbalanced:N / prev-quiet* | **— (n < 30)** | — | — |
| balanced / prev-CAPTURE | 122 | 21.4 | 48.6 |
| balanced / prev-quiet | 1204 | 8.2 | 34.0 |
| imbalanced:NB / prev-quiet | 315 | **0.1** | 33.5 |
| imbalanced:P / prev-quiet | 250 | **6.9** | 35.8 |
| imbalanced:PNB / prev-quiet | 142 | **7.1** | 48.2 |

- **prev-CAPTURE 128.2 cp vs prev-quiet 6.6 cp — a 19x split.**
- **The entire `imbalanced:N` bucket is prev-CAPTURE.** Its quiet counterpart did not even reach n=30.
- **Once you condition on the previous move being quiet, material imbalance explains essentially nothing**:
  balanced 8.2, NB 0.1, P 6.9, PNB 7.1 — all indistinguishable.

**Conclusion: the "eval is ~2x wrong when material is imbalanced" finding is an artifact of measuring
residuals at mid-exchange positions. There is no measured imbalance eval defect.**

## Consequences

1. **The "direct imbalance eval term" candidate is KILLED before it was ever built — zero box hours.** Its
   entire motivation was the addendum's inference, which is now falsified. Do not queue it.
2. **The key-ranking addendum's Finding-3 correction is itself CORRECTED.** The addendum rightly said "do NOT
   open an mg/eg taper investigation — the bias is material-structured." The bias is neither taper nor
   material-structured: **it is horizon-structured.**
3. **Methodological rule earned, and it generalises: a residual-vs-static measurement MUST condition on
   quiescence.** Comparing a full-search score against a raw static eval at a position with pending captures
   measures the **horizon**, not the eval. Any key correlated with "mid-exchange" will score highly on such a
   residual for reasons that have nothing to do with what the key names.
4. **This gives a mechanism for the T4d demotion, which was previously only empirical.** The residual-R²
   pre-flight was demoted to rejection-only on 2026-08-01 because its 115x-over-chance winner lost cleanly.
   **Now we know why it could not predict Elo: a large share of the residual variance it was ranking keys on
   is recapture noise**, and material counts (and phase, which is computed from them) **directly encode
   whether an exchange is half-finished**. Material scored 29% R² substantially by being a good detector of
   mid-exchange state. The demotion stands and is now explained rather than merely observed.

## Bearing on T4e — recorded BEFORE its verdict

T4e's manifest pre-registered **key coarseness** as the leading failure hypothesis. This measurement supplies
a **sharper and more specific** one, and it is logged here while the run is still live:

> A material-keyed corrector learns "buckets with an odd minor count carry a large positive residual" — but
> that is **"a recapture is pending"**, a horizon state the search already resolves for itself. Applying it as
> a **standing static-eval offset at every node sharing that material signature** is not merely uninformative,
> it is **actively wrong** for the positions where the imbalance is real and permanent (a genuine piece
> sacrifice, a completed trade), which get inflated by ~a minor piece.

**This does not amend T4e's pre-registration and must not be used to claim the outcome was predicted.** If
T4e keeps, this hypothesis is wrong and the note stands as a recorded miss. If T4e rejects, the campaign has
a mechanism rather than a guess — and the corrhist family's remaining reopens should be judged against it.

```yaml
id: 2026-08-01-imbalance-eval-probe
date: 2026-08-01
change_class: measurement / falsification (no code changed)
result: >
  The "eval is ~2x wrong when material is imbalanced" finding is a recapture artifact. prev-CAPTURE residual
  128.2 cp vs prev-quiet 6.6 cp (19x). The entire imbalanced:N bucket (mean +304, mean|cp| +304, all positive)
  is mid-exchange. Conditioned on a quiet previous move, material class explains nothing (0.1-8.2 cp across
  all classes vs balanced 8.2).
box_cost: zero
code_changed: none (probe shelved to output/imbalance-probe_test.go.txt)
kills: the queued "direct imbalance eval term" candidate
explains: why the residual-R2 pre-flight could not predict Elo (T4d demotion mechanism)
rule_earned: residual-vs-static measurements must condition on quiescence
```
