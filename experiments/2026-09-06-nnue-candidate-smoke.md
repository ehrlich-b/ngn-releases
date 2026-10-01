# First NNUE candidate game diagnostic

Date: 2026-09-06. Operational acceptance only; no playing-strength estimate or release promotion.

The first four paired games at 10+0.1 completed with all independent operational and chess checks passing. HCE scored 4/4 against the 100k-position pilot NNUE. All four games ended by natural mate (204 legal plies total). This pilot network remains a pipeline diagnostic; these results provide no basis to replace the accepted evaluator.

Both roles used the identical clean 4bd54add executable SHA ee59a26d6b06900e5466d42a5b2f0e3d26df44059799d590a523a9819e7a1211. The NNUE role loaded candidate-1 SHA abd956983274bae1d91b522cd7ca3a1b2fa91ef3e69051ccb414b9adf0a5c67e, selected by minimum eligible raw validation loss. Settings were Threads=1, GOMAXPROCS=1, Hash=64 MiB, Move Overhead=50 ms, OwnBook=false, CPU12, concurrency 1. The first two pinned six-ply openings were played in both colors. No score/draw/resignation/maxmoves adjudication, recovery, ponder or tablebase was enabled.

Root independently verified all 86 final artifact hashes, reviewed the trace and chess audits, checked 2354 child-process observations for exact engine identity/GOMAXPROCS/affinity, and confirmed all five supervised stages returned 0 with no timeouts, monitor errors, or surviving processes. Both UCI preflights proved selected evaluator/options and real non-book search. No warning, canonical engine error, external book or probable embedded-book signature was found. Match peak sampled process-tree RSS was 597,324 KiB.

Evidence:
- Run: /home/ehrli/repos/ngn-candidate-runner/output/candidate-smoke-20260906-attempt2
- Final files manifest SHA: b577437428695213865295b34d8b4315e8183c37db349dce81075f707b9b0a7c
- Terminal SHA: a87f653849ff35d1bd864686cc5dc83cbeaad86ae779ee48aac7f9e65aa2861b
- Root receipt: output/candidate-smoke-root-review-20260906/receipt.json
- PGN SHA: 0b1d651675be2e5307773a553d9318d8624985eee380f14aa6a511e58d0a9f6a

Attempt1 is preserved separately. It failed before engine launch because the invocation used the live script path instead of the exact frozen path required by the manifest. Attempt2 changed only the invocation/approval token and used a new output directory.

The next data experiment increases the verified corpus target from 100,000 to 1,000,000 positions under the existing decoder, split, quarantine and independent verification rules. Root rehashed all 1,881 frozen inputs before authorizing data conversion; training requires the exact observed split counts and its own freeze. This changes data diversity; it does not assume that another million presentations of the same small pilot will improve generalization. The 100k pilot already had worsening validation after candidate 1 despite falling training loss.

The deployed WSL engine remains 53e4d1b.
