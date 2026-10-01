# Accepted T8: completed 400-game pin, outcome audit held

`r0905acceptedpin` completed400 games with DONE_EXIT_0, using the unchanged
[prospective protocol](2026-09-05-2800-pin-plan.md): accepted5cf301a,
120+1/c8 on native Windows,80 games per anchor,40 paired openings. No pooling,
sample extension, label change or rerating occurred.

**2800 is not verified.** The original report is2795.7554,95%CI
**[2753.2207,2838.2901]**; its lower bound is below2800. Independently, the
required final-board audit failed on two NGN wins recorded as cap draws.
The measurement verdict is HELD under the predeclared integrity rule.

| Anchor | Recorded W/D/L | Recorded score | Individual performance |
|---|---:|---:|---:|
| Blunder6.1, label2155 |69/11/0|93.125%|2607.72|
| Blunder7.2, label2425 |54/26/0|83.75%|2709.85|
| Blunder7.4, label2532 |48/31/1|79.375%|2766.12|
| Blunder8, label2674 |41/35/4|73.125%|2847.89|
| Counter3.8, label2994 |13/31/36|35.625%|2891.22|

Total225/134/41. All five anchors were retained; both families and50% bracketing
pass. The three real clock forfeits were all Blunder6.1 losses and remain under
the original rule. No NGN flags or operational errors occurred. Whole-opening
bootstrap, seed2800/10000 resamples: [2761.9977,2826.7491], supplementary only.
These are historical local calibration labels, with unmodeled label uncertainty.

## Completed audit and exact defects

- Original cumulative log, PGN/tally/opening/color reconciliation: PASS.
- Frozen Stockfish legal replay: PASS, all400 games and56310 plies.
- Old200-game compatibility control: detected exactly the known game70 defect.
- New all-final-board audit: FAIL, exactly games211 and216, both versus7.4.
  NGN was Black; White had no legal move and was checked. Both rows recorded
  D/max-moves and should have ended at checkmate/W. Raw rows remain unchanged.
  Final FENs are respectively
  `3q4/2K5/8/3q4/8/4n1k1/8/8 w - - 6 101` and
  `8/qr6/8/1K6/8/5k2/2r5/8 w - - 8 101`.
- All subprocess stderr logs were empty. Validation stopped at the expected
  nonzero terminal-audit exit; no final validation-PASS manifest was written.
- After the hold, a separate integrity check reverified all pinned assets and
  original inputs against their snapshots. All337 frozen local inputs also
  still matched. The original-run freeze closes with this completed audit;
  raw evidence remains immutable as subsequent development proceeds.

Evidence: `output/remote-pin-20260905/results-r0905acceptedpin/`, with rating,
legal/terminal reports and raw snapshots. Archive SHA-256
`cb1fbe1c17fa08c84ce3630f5ce0bd2a465780a9e66d4d8c631a477d1edb3f61`;
PGN SHA-256
`7a49e4c83d8d417fc96da988e1271fc89c52c7cc7be6a89cea72a582448798f0`.
Post-hold integrity is also retained in the mate-history regression-followup
directory. Original gauntlet29692 and completion waiter22316 ended; neither
may be resumed or replaced as the same run.

## Next decision

The already isolated harness fix preserves terminal outcomes at the cap. Its
three native terminal-regression tests now pass. The independent direct
mate-stop regression also passes on the guarded WSL source and fails on the
old source specifically at one completed iteration/depth1; setup and stderr
checks pass. The [Windows mate-stop protocol](2026-09-05-mate-stop-windows-recovery.md)
now owns the fresh A/A and subsequent fixed candidate test.

A lightweight inventory of the completed final boards found32 recorded draws
with NGN holding a rook or queen against a bare king, including the two
mislabels. The134 recorded draws comprise88 repetitions, one fifty-move draw
and45 cap draws. This makes conversion a relevant target, but the inventory
does not prove that the mate-stop guard fixes every case or predict its Elo.
No new strength claim follows from self-play, corrected labels or summed gains.
