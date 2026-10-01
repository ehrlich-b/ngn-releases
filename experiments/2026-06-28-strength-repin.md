# 2026-06-28 strength re-pin (post-correctness-reset)

First absolute-strength measurement of the engine after the 2026-06-28 correctness
reset + queue (items 2-7). Multi-family gauntlet on the LAN 9800X3D. Predeclared
before launch.

```yaml
id:                 2026-06-28-strength-repin
date:               2026-06-28
change_class:       measurement (absolute re-pin, not a keep/reject candidate test)
hypothesis:         after the reset + correctness queue (root-rep, EP-hash, promotion-safety, UCI-lifecycle, SearchFixed), where does NGN sit on the CCRL scale, and did the fixes move it vs the Jun-14 single-family Blunder pin (2655)?
base_commit:        n/a (re-pin of HEAD; prior reference is the Jun-14 pin at 2655)
candidate_commit:   2dadef8336cac93b3fae522929b7a571af9762ab
candidate_binary_sha256: 4e4a04321c8421cce1d5983225565b3aa4466628662ec4a51b9c38b4450d9089  # build/win/ngn.exe (GOOS=windows GOARCH=amd64 GOAMD64=v3)
harness_commit:     2dadef8 (cmd/gauntlet, cross-compiled gauntlet.exe)
command:            gauntlet.exe -ngn .\ngn.exe -anchors ratings.json -tc 120+1 -games 80 -concurrency 8 -lowpower=false -openings sprt_openings.txt -tally-out repin_tally.json -pgn repin.pgn -no-record
machine:            LAN 9800X3D (AMD Zen5 V-Cache, 8c/16t), native Windows
go_version:         go1.26.2
goarch_goamd64:     windows/amd64 v3
tc:                 120+1 (real clock, SECONDS — CCRL Blitz proxy; drops the movetime optimism bias)
concurrency:        8 (16 engine procs on 16 threads ~= 1 SMT thread/engine; both sides equally slowed, so the relative result/rating is preserved — the standard oversubscribed-gauntlet assumption)
openings:           sprt_openings (corpus_manifest.md)
openings_sha256:    974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222
aa_preflight:       n/a for a gauntlet (NGN-vs-anchors, not self-play); the box pipeline was validated by two 2-game smokes (NGN 1W-1D vs Blunder 7.4.0, 0-2 vs Counter 3.8) confirming handshake/clock/scoring on the box before launch
anchors:            Blunder v6.1.0 (2155), v7.2.0 (2425), v7.4.0 (2532), v8.0.0 (2674) [continuity ladder]; CounterGo 3.8 b172b99 (2994) [cross-family top anchor]. 5 anchors x 80 games = 400 games. Fruit (C++) omitted on Windows.
decision_rule:      report the pooled CCRL rating + 95% CI and per-anchor crosstable; this is a MEASUREMENT (no keep/reject). Multi-family (Blunder + Counter) so it is NOT single-family-confounded like the Jun-14 pin. Compare central estimate vs 2655.
games_or_pairs:     400 (5 anchors x 80)
result:             POOLED NGN 2666, 95% CI [2627, 2706]. Per-anchor (W-D-L, score, perf): Blunder 610/2155 61-18-1 87.5% perf2493; 720/2425 44-34-2 76.2% perf2628; 740/2532 34-37-9 65.6% perf2644; 800/2674 24-45-11 58.1% perf2731; Counter-3.8/2994 3-28-49 21.2% perf2766.
flags_errors:       6 time-forfeits / 400 (1.5%) — 4 in Blunder-610 games (anchor-side; NGN <=1L there) + 2 in Blunder-740 games (<=2 possible NGN time losses); 0 illegal/no-move/crash. Clean DONE_EXIT_0. Run survived the held-ssh monitor being killed (gauntlet runs in box Session 0).
verdict:            NGN ~2666 CCRL [2627,2706], MULTI-FAMILY + real-clock (the -tc 120+1 header confirms no movetime bias; the "+30-80 optimistic" footer is the stale unconditional text that does NOT apply to -tc runs). FLAT vs the Jun-14 single-family pin (2655 [2606,2704]) => the correctness reset + items 2-6 PRESERVED strength (they were correctness fixes, mostly latent/rare, not Elo plays) while the instrument is now trustworthy (multi-family, +-40 CI, real clock). Note the monotone perf trend (weak anchors saturate low: 2493->2766 as the anchor strengthens) => the true number likely sits in the upper half (~2690-2730); 2666 is the defensible weighted pooled point estimate.
next_action:        Item 8 DONE. Proceed to item 9 (resume Elo levers) on this trustworthy instrument. PGN captured at repin.pgn on the box for loss classification (lossxray) when picking the first lever.
```

## Crosstable (final)

```
Anchor       CCRL    W   D   L   Score   Perf   95% CI         weight
v6.1.0       2155   61  18   1   87.5%   2493  [2380,2606]    12.1%
v7.2.0       2425   44  34   2   76.2%   2628  [2539,2716]    19.6%
v7.4.0       2532   34  37   9   65.6%   2644  [2565,2724]    24.2%
v8.0.0       2674   24  45  11   58.1%   2731  [2654,2808]    26.1%
counter-3.8  2994    3  28  49   21.2%   2766  [2674,2858]    18.1%
POOLED NGN rating: 2666   95% CI [2627, 2706]   (5 anchors, 80 games each = 400)
```

Cross-family non-transitivity is now visible (the prior single-family pin could not see it): the Blunder ladder rates NGN ~2493-2731 (2155 rung saturates low) and Counter rates NGN 2766; the weighted pool lands at 2666. This is the first re-pin where a non-Blunder family (Counter 3.8, CCRL 2994) anchored the top of the ladder.
