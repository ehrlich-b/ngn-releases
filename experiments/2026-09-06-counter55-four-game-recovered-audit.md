# Counter 5.5 in-process four-game recovered chess audit

The approved four-game diagnostic used one accepted NGN binary for the
Counter 5.5 and HCE backends. Both roles ran with Threads=1, GOMAXPROCS=1,
Hash=128 MiB, Move Overhead=100 ms, OwnBook=false, and time control 30+0.3.

The original runner attempt remains FAILED. Its match, both UCI preflights, and
trace audit completed successfully, but the frozen chess auditor assumed that
concurrent PGN records appeared A-white then B-white. Fastchess wrote completed
games in the opposite order for Round 1, so the original audit failed closed.

Auditor v5 groups exactly two records by contiguous numeric Round, requires one
of each declared color orientation, maps each Round directly to its opening,
and keeps every PGN record attached to the final-EPD line at the same original
file index while ordering pairs.

Focused pairing and count fixtures passed 9/9 with unchanged source hashes and
no survivors. The corrected Stockfish-backed audit then passed over the
immutable original artifacts:

- 4 games / 2 pairs and 430 independently legal plies;
- Counter 5.5: 2 wins, 0 losses, 2 draws, 3.0/4 points;
- terminal reasons: two White mates and two threefold draws;
- pentanomial counts: [0, 0, 0, 2, 0];
- ordered source-game indices: [2, 1, 3, 4];
- zero probable embedded-book signatures;
- source and all original inputs matched before and after;
- supervisor and command exited 0 with no timeout, memory breach, monitor
  error, orphan, or surviving process.

This is recovered diagnostic chess evidence with no strength claim. It does not
change the original run's FAILED status, and a later strength run must complete
cleanly under the corrected runner.

Evidence:

- original failed run:
  /home/ehrli/repos/ngn-counter55-runner/output/counter55-inprocess-vs-hce-four-game-20260906-attempt1
- focused fixtures:
  /home/ehrli/repos/ngn-counter55-audit-v5/output/counter55-audit-v5-validation-20260906/focused-attempt1
- recovered audit:
  /home/ehrli/repos/ngn-counter55-audit-v5/output/counter55-four-game-chess-audit-recovery-20260906-attempt1
- recovered audit SHA-256:
  f68d39730c665828f0643e559d4a379ff2e44a0cb3e91ecd622d8b94d834df09
- recovered result receipt SHA-256:
  120f0d9a5ea0e5e1e7704099dfbfd49d1c1765b5c76d07f88aaf6f690c01cb78
- v4-to-v5 patch SHA-256:
  c1286991f85f628c57adbc13c172a7f127717b880bd82328e5cc97185e5d2760
- v5 source manifest SHA-256:
  d431eacd45eeac08603e5e161c51062a412c8e4f4d471e19c935a5673c992215

Root independently accepted this recovered diagnostic in
`output/counter55-inprocess-runner-root-review-20260906/recovered-four-game-review-v1.json`
(SHA `bf763bf11dbbbe75aeeda32c22272a559b5bd0d979ee9ca4361fe1438c158cb7`).
