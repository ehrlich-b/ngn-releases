# 2026-07-26 T18b — probcut TT store — SHELVED (clean statistical reject, -22.5)

**VERDICT: SHELVE.** `H0 ACCEPTED: new is NOT better (<= -3 ELO)`, `DONE_EXIT_0`.

This is the **largest single negative of the campaign** — roughly 3x T3a (-7.9), T17 (-6.4) and T9a (-5.2) —
and it landed on the candidate the plateau diagnosis had ranked #1. That makes the post-mortem worth more
than the verdict.

## Result

| Field | Value |
|---|---|
| Penta Elo | **-22.5** 95% CI **[-36, -10]** |
| pLLR min | **-3.09 @G1347** (the final sample IS the min) |
| First crossing of -2.94 | **G1327**; 16 samples at/below the bound |
| Post-mingames pLLR max | **-0.73 @G348** |
| Post-mingames elo max | **-15.9 @G1156** |
| W-D-L / score | 311-637-399 / 46.7% |
| Games | 1347 real-clock 10+0.1 c8 adjudicated |
| Flag-outs | **0 new / 0 base** |
| Wall / throughput | 08:29:11 -> 09:54:28 EDT = 1h25m17s, **948 g/hr** (baseline 934-961) |
| Crashes / illegal / no-move | 0 |

**No c8-drain ambiguity.** The three prior drains (T9a, Batch-2 cert, T17) pulled the printed final pLLR back
*inside* the bound after a genuine crossing; here the drain pushed the same direction (-2.94 @G1327 ->
-3.09 @G1347), so the printed number and the envelope agree.

**The trajectory is exceptionally well-determined** — elo sat in [-27, -18] continuously from G300 to the end:

```
G300  -26.7 [-66,+13]  pLLR -0.84
G500  -22.3 [-53, +8]  pLLR -1.20
G800  -20.0 [-44, +4]  pLLR -1.60
G1000 -19.5 [-41, +2]  pLLR -2.01
G1200 -18.5 [-38, +1]  pLLR -2.27
G1327 -21.8 [-40, -3]  pLLR -2.94   <- first crossing
G1347 -22.7 [-41, -4]  pLLR -3.09
```

Like T17, **never once positive after mingames** — there is no positive phase to argue about, and unlike a
capped null this crossed a bound.

## The implementation was NOT defective — checked before blaming the idea

The obvious suspicion for a store this costly is that it over-claims depth. It does not:

- verification search: `-alphaBetaPV(pos, depth-4, ply+1, -probcutBeta, -probcutBeta+1, false, true, !cutNode, info)`
  — that is the **child** at `depth-4`, i.e. a proof at **`depth-3` from this node's frame**;
- store: `TranspositionTable.Set(hash, move, scoreToTT(score, ply), int8(depth-3), LowerBound)`.

**Stored depth == proven depth.** This matches Stockfish exactly (SF verifies at `depth - 4` and saves at
`depth - 3`). The store site is after `UnMakeMove`, so `hash` is the node's own — verified pre-launch. The
bound is also *sound*: `score >= probcutBeta > beta`, stored as a `LowerBound`, so any later cutoff off this
entry requires `score >= beta_new`, which the entry genuinely licenses.

So this is a **faithful transplant of a standard technique that lost 22.5 Elo in NGN's calibration.** That is
a directly relevant data point for the method-revamp thesis (standard-technique transplants with measured
priors), and it is not explained away by an implementation error.

## Leading explanation — persisting a speculative bound

Probcut is a **self-described speculative prune**. NGN's own comment at search.go:1526:

> if a reduced-depth search at a higher beta fails high, we can prune because the full search would **likely**
> also fail high

Returning `beta` on that basis is a **one-shot, bounded** error: it costs exactly one node's decision.
Writing it into the TT converts that one-shot guess into a **durable, reusable assertion** that fires again on
every later revisit of the position — so the heuristic's error rate is multiplied across the entry's whole
lifetime instead of being paid once.

**This reframes the pre-launch node numbers.** The gate read `-25.5% / -28.5% / +25.9%` as "a quarter fewer
nodes to the same fixed depth = a large efficiency gain". The store fires at only **1622 of 346662 nodes
(0.47%)** at kiwipete d12, yet removes a quarter of the tree — an amplification of roughly **50x per stored
entry**. That amplification factor is *also* the error-multiplication factor. The node delta was not measuring
savings; it was measuring how much the speculative-prune surface grew.

## THE RULE THIS EARNS — a real refinement to the plateau diagnosis

The 2026-07-26 diagnosis sorted every post-reset result into *search-efficiency/TT wins* (T5 +20.2, T1b +13.1,
T4 +9.1) versus *pruning-constant losses* (T7 0, T10b 0, T9a -5.2, T1c -4.8, T13 -0.4, T16 -0.6, T3a -7.9),
and re-ranked the whole queue on that split. **T18b is the first miss for the promoted class, and it is the
biggest miss on the board** — so the split needs correcting, not just annotating.

The correction: every winner in the "efficiency" class made the search cheaper using **proven or
games-verified** information — T5 aspiration invents no information (pure window management), T1b/T1e
reallocate time, T4 corrhist applies a *learned* correction that was itself validated by games. T18b made the
search cheaper by **caching a guess**.

> **Operative distinction is not "efficiency vs constants". It is: does the change PERSIST speculative
> information?** Node-count reduction is evidence of efficiency only when the information driving it is
> proven. When the reduction comes from caching a heuristic, the node delta measures **error surface, not
> savings** — and a large delta from a low fire rate is a warning sign, not a green light.

**Added to the >=1% nodecheck gate as a second question**: not just *did the tree move*, but *what kind of
information moved it*. This gate would have flagged T18b before spending box time.

**Applied to the live queue — the rule discriminates, it does not just retro-explain:**

- **T11a (4-way TT clustering) SURVIVES and keeps its rank.** It was promoted on TT-retention reasoning, the
  same family that just failed. But it retains *real search results* — information already proven by full
  searches — and invents nothing. The distinction protects it.
- **T19 (ttPv) SURVIVES.** It reduces less at previously-PV nodes; it persists a *structural* mark, not a
  speculative value.
- **T3b (LIVE) is unaffected** — it reduces, it does not cache.

## Lane status: T18 is now FULLY CLOSED, both halves

- **T18a** — TT short-circuit: already killed at the gate (-0.15%, result-identical) and **re-classed as a
  pure speed change that never warranted a games gate**.
- **T18b** — TT store: this record, -22.5.

**REOPEN (weak, deliberately).** Do **not** re-run the store alone. NGN's `probcutBeta = beta + 200` is a flat
margin where SF scales it (improving-aware), and NGN lacks the surrounding calibration SF's store sits inside;
the store is only worth revisiting as part of a whole probcut rework that fixes the margin shape first. Given
a -22.5 measurement, that reopen ranks **below everything currently queued**. A narrower variant (store at a
much shallower depth so the entry expires locally and the amplification drops) is *possible* but is
constant-fiddling on a mechanism that just lost 22 Elo — not worth box time.

## Run record

```yaml
id: 2026-07-26-t18b-probcut-ttstore
date: 2026-07-26
change_class: search heuristic (TT store on probcut success). Predeclared paired game test.
verdict: SHELVE (H0 ACCEPTED, clean statistical reject)
candidate: ngn_t18b.exe sha256 ccbb28ea0a902be787b65edb8c0c966bcb6582f23ce729617f36cad9ddb2d196
base: ngn_t5.exe sha256 0a8f65b75e59c376052f83b14787b0af2eba65552cedfee6f70ce576891ca721
patch: output/t18b.patch (16 lines, engine/search.go only) — SHELF COPY, engine tree never carried it
harness: sprt.exe sha256 30c33e0512725b7f552d8a1cf72ba6f1e0deb4dcb122c6b6ba8f6433c686a762 (M5-validated, untouched)
command: sprt.exe -new .\ngn_t18b.exe -base .\ngn_t5.exe -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 8000 -mingames 300 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
machine: AMD Ryzen 7 9800X3D 8c/16t, native Windows (LAN box 192.168.4.108)
tc: 10+0.1 (seconds)
concurrency: 8
openings: canonical sprt_openings (5000 lines) sha256 974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222
adjudication: STANDARD-ON (M5-validated flags)
games: 1347
result: penta -22.5 [-36,-10], pLLR -3.09, H0 ACCEPTED, DONE_EXIT_0
flag_outs: 0 new / 0 base
log: output/t18b_out.txt
nodecheck_prelaunch: -25.5% / -28.5% / +25.9% vs 346662 / 149587 / 765656
nodecheck_after: baseline unchanged (346662 / 149587 / 765656) — no re-lock, nothing kept
```

Engine tree never carried this change (patch applied only for the build), so there is nothing to revert;
HEAD nodecheck re-verified at the exact 346662 / 149587 / 765656 baseline.
