# Fresh 2600 check after the correctness repair

September5 audit correction: the original numeric report below is retained,
but its replay PASS did not establish correct termination for every game.
An [all-final-board follow-up](2026-09-05-draw-conversion-audit.md) found game70
was checkmate delivered by NGN at the200-ply cap, recorded as a draw. The other
199 final boards passed that supplemental terminal check. No raw result was
changed or rerated. Treat this as a measurement defect in the historical pin.

Predeclared before launch. This runs only after the repaired harness A/A and
replacement 400-game correctness non-regression check pass. The original repair
failed its game gate; the exact speed optimization and fresh preflight are
documented in [the replacement manifest](2026-09-04-fast-correctness.md). The user asked to get the engine
past 2600 and deploy it on WSL; the old 2704 estimate used the faulty harness,
so a fresh absolute measurement is warranted. Expected cost ~90 minutes for
200 games, based on July's 400-game run taking 3h06m.

## Instrument

- Candidate and source patch: [optimized repair manifest](2026-09-04-fast-correctness.md).
  `ngn_20260904_fast.exe`, SHA-256
  `7b5af06a4a0ff02a0d9c51aa5eb041dd8ddf9935bbe4691d65dd76146a515828`.
- Gauntlet rebuilt with the corrected shared rules/parser. `gauntlet_20260904.exe`,
  SHA-256 `5d1b6ca586420c04bffed729f5e48c775105c2fb04b2c267a15bd6b6a6769b9a`.
  go1.26.2, GOOS=windows, GOARCH=amd64, GOAMD64=v3.
- Same 9800X3D/native Windows box, concurrency 8, default scheduling, **120+1
  real clocks**. No movetime discount applies despite the old footer text.
- Five historical anchors, **40 games each**, colors reversed per opening.
  Same pinned 5000-line openings SHA-256
  `974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222`.
- On-box `repin/ratings.json` SHA-256
  `f1ac3a64ee26448f51c82b14312a04335b690ea00d3a56b527e2be7238696d32`.
  Historical labels retained for continuity; these are local calibration labels,
  not a claim to a current official CCRL listing. The Counter label is known to
  be conservative; it is deliberately not adjusted in this run.

| Anchor | Label | Binary SHA-256 |
|---|---:|---|
| Blunder 6.1.0 | 2155 | `d1bfa4cbc435e0a6d18e2e1fa301df8556bea05a2c6b1f38bf714049860040b4` |
| Blunder 7.2.0 | 2425 | `ed3340ffa916133bcae66609e624cecece60f674db8b8a4de766750ee813f998` |
| Blunder 7.4.0 | 2532 | `160e0866a6811b9c769505a1e463017241682e1dc4449c9aecbae4f65be927d1` |
| Blunder 8.0.0 | 2674 | `7dbf4b3915b37318c688538b7368f54e4046c957d9ee0fcfdb4cbea45556beec` |
| Counter 3.8 | 2994 | `d260174182ea10c5b902a8d25e217166f5b5b54dc4fa64c220b498acce6dec11` |

## Decision rule

Fixed 200 games; no early rating verdict or stopping on a favorable estimate.
Report per-anchor W/D/L, exclusions/errors, pooled rating and 95% CI. The 2600
threshold is supported if the **lower CI exceeds 2600**, both engine families
remain represented, anchors bracket 50%, and no NGN error or flag-out occurred.
Otherwise report the target as unconfirmed and investigate; do not silently
extend or pool favorable extra games. Pre-existing >2% per-anchor error exclusion
remains in effect and must be disclosed. Any new rating is a local estimate,
not an official CCRL result. The changed harness precludes attributing a rating
delta versus July solely to engine improvements.

The gauntlet is against anchors, not self-play. Its shared game logic has unit
regressions plus the newly completed A/A preflight. It runs its own engine
handshake/warmup preflight at startup.

```text
gauntlet_20260904.exe -ngn .\ngn_20260904_fast.exe -anchors ratings.json -tc 120+1 -games 40 -concurrency 8 -lowpower=false -openings sprt_openings.txt -only v6.1.0,v7.2.0,v7.4.0,v8.0.0,counter-3.8 -tally-out r0904pin_tally.json -pgn r0904pin.pgn -no-record
```

Run name **`r0904pin`**, in `C:\Users\ehrli\ngn\repin`.
Poll with `scripts/boxrepin.sh tail r0904pin`.
Results and raw logs to `output/recovery-2026-09-04/` after completion.

## Completed result

**2726, reported 95% CI [2668,2784]**, all 200 games, `DONE_EXIT_0`.
The original 2600 gate passes: the lower bound exceeds 2600, both families
remain represented, scores bracket 50%, and there are no NGN errors or flags.
This does **not** establish the later target of at least 2800.

| Anchor | NGN W/D/L | Score |
|---|---:|---:|
| Blunder 6.1 | 33 / 6 / 1 | 90.0% |
| Blunder 7.2 | 24 / 16 / 0 | 80.0% |
| Blunder 7.4 | 21 / 18 / 1 | 75.0% |
| Blunder 8.0 | 18 / 19 / 3 | 68.8% |
| Counter 3.8 | 5 / 10 / 25 | 25.0% |

One Blunder 6.1 **time-forfeit**, no other flags or process errors. An earlier
working note incorrectly said the >2% rule automatically excluded that block.
The code explicitly omits real time-forfeits from `forfeitReasons`; an occasional
clock loss is treated as part of play. Thus the original-rule primary report
retains all five anchors. A stricter sensitivity excluding all forty Blunder
6.1 games gives **2753 [2691,2815]**, also above 2600 but not a replacement pin.
The dropped anchor's weaker performance explains why that exclusion raises the
estimate. No games were extended or pooled after seeing the result.

Independent Stockfish replay verified **all 27546 played plies**, final mates,
draw rules, max-move draws and the forfeit result/side. Audit: PASS. End reasons:
130 checkmates, 49 rule draws, 20 max-move draws, one opponent clock loss.

The reported interval is the existing harness's approximate inverse-variance
calculation and excludes anchor-label uncertainty. The twenty six-ply synthetic
opening prefixes are paired by color and reused across anchors. As an additional
sensitivity, a seed-2800 bootstrap of those twenty complete opening clusters
(10000 resamples, four-anchor subset) gives [2719,2784]. This does not cure
synthetic-opening or historical-calibration limitations. No movetime discount
applies to this real-clock run; the printed movetime footer is stale text.

Raw artifact SHA-256s:

- `r0904pin.pgn`: `a6b25b2af418f66c7c7cd10b9addaf48b5be4cea53e950d042caa6310848c077`
- `r0904pin_out.txt`: `d2bf54023054788c0d68461429008f312ca30e62868cb1fd3f1d1347aa61ff2a`
- `r0904pin_tally.json`: `2d52cafee00cbf072bc6f05a9abb54b5ea6baa901982047eec525b5e7a22c8e8`

Audit and independent sensitivity outputs: `r0904pin-audit.json` and
`r0904pin-rating-sensitivity.json`, alongside their Python drivers in the same
raw artifact directory. Tally JSON does not encode error reasons; consult the
full game/output logs before any future pooling.
