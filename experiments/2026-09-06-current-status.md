# Current NNUE / multicore sprint status

> Superseded for live release state by
> [the September 12 evidence audit](2026-09-12-release-state-audit.md). This file
> remains the implementation-history record through September 6.

Updated 2026-09-06 after accepting and integrating the Counter eight-worker witness and AVX2 feature-update optimization. All project work runs on WSL; the Mac is SSH transport only. Sol implements; root coordinates and accepts results.

The deployed engine remains **53e4d1b**. The development branch has integrated real multicore search, the Counter 5.5 evaluator and adapter, and the Counter transition fast path. Stockfish SMALL scalar/incremental evaluation and the BIG loader are separate packages without search wiring. The latest integrated milestone is **9f38b62**, including the eight-worker test extension at **89bdc27**. Final combined-head validation remains pending.

**Two playing-strength results are accepted:** HCE8 beat HCE1 under the predeclared width test, and Counter1 beat HCE1 under the predeclared evaluator test. The combined Counter8 result, remaining 2/4-thread strength results, external calibration and deployment are still required. The historical native-Windows rating of 2884.14 is not a current WSL rating, and internal gains cannot be added to it.

## Accepted results and remaining work

| Area | Evidence accepted | Remaining |
|---|---|---|
| Multicore search | Private worker state, synchronized shared TT, stop/join lifecycle; correctness/race gates; eight-thread NPS scaling 6.41–6.69x in the 128-sample matrix | Two- and four-thread strength runs; final combined release stability |
| HCE8 versus HCE1 | 102 games: 63 wins, 3 losses, 36 draws; first SPRT upper crossing at pair 51; all 14,485 played plies independently audited | The independently predeclared 2v1/4v1 runs use the same original SMP binary |
| Counter1 versus HCE1 | 166 games: 89 wins, 37 losses, 40 draws; first upper crossing at pair 83; all 26,074 played plies independently audited | Counter8 versus HCE8 at 60+0.6 with independent 16-ply openings |
| Counter transition speed | Exact cold/warm snapshots on six fixtures; 5.466876% lower cold 400,000-node time; integrated at 4b98888 after focused/full-short/race/vet/build checks | No steady-game speed or Elo claim from this timing |
| Counter AVX2 feature updates | Exact v1/v3 arithmetic/oracle/search checks; 19.8095% lower cold fixed-node time across 14 paired blocks and six fixtures; exact production source integrated at 9f38b62 | Final combined-head checks and real-clock confirmation; no separate SIMD Elo claim |
| Stockfish SMALL | Official loader and 145-position full-refresh parity; genuine upstream incremental parity across 703 states; incremental package integrated at 17d72ba | Search/score-policy wiring and BIG/SMALL selection remain separate work |
| Stockfish BIG | Exact official 108,919,594-byte model parsed by independent Python and Go loaders; focused/race/official/vet/build passed; integrated at 18e1c16 | Feature extraction and scalar/incremental inference are not implemented by this loader slice |
| Minimal trained NGN-v1 | Numerical export/scalar/incremental/optimized equivalence established | Selected 100k/1M/3M networks failed HCE screens; count-only training extensions stopped |
| Leading Go opponents | Counter 5.5, Zahak 10, Blunder 8.5.5 and Chess-3 v4.0 provisioned; Rodent V1.1 and V1.2 source/release/model identities pinned | Fresh admission under actual match mask, matched games and uncertainty reporting |
| Deployment | Original 53e4d1b preserved | Final combined build, required playing-strength gates, deployment verification and rollback rehearsal |

## Playing-strength evidence

Both accepted tests use the same binary for candidate and reference within their test, paired openings with reversed colors, 30+0.3, 128 MiB total Hash per engine, Move Overhead 100 ms, OwnBook false, one game at a time, no adjudication, and the predeclared normalized-Elo SPRT hypotheses 0/20 with alpha=beta=0.05 and a 400-game cap. These are hypothesis-test verdicts, not ordinary +20 Elo confidence claims.

HCE8/HCE1 used source **168385f** on the full physical-core mask 0,2,4,6,8,10,12,14 without fastchess affinity. Penta was [0,1,8,23,19]; the first upper crossing was pair 51 at LLR 3.024581092967444. Pair 50 remained below the actual boundary despite displaying 2.94. All six supervised stages passed and no children survived. Root acceptance:
`/home/ehrli/repos/ngn-m4c-sprt-runner/output/m4c-sprt-root-review-20260906/terminal-review-v1.json`,
SHA `353872594a7e4d93deed193cd28c444f73db0cf8a67964d5936a9ffdebe0e9eb`.

Counter1/HCE1 used the accepted **49eaf624** binary, exact Counter model and adapter, and physical CPU 12. Penta was [5,4,27,28,19]; pair 82 was below the upper boundary at 2.9426262166847152, and pair 83 crossed at 2.959195349009477. Root rehashed 96 files, reconstructed every pair/LLR prefix, and checked legal/terminal, operational and process results. Actual WSL outer exit was 0; a stale SSH relay later closed with local 255, which is preserved separately. Root acceptance:
`/home/ehrli/repos/ngn-counter55-sprt-capability/output/counter55-vs-hce-sprt-root-terminal-review-20260906-v1/root-review.json`,
SHA `fd1da7e0767d94a94da7768f880c460f0afb613d090e6bd3e306d943fd4c5029`.

The HCE2/4 manifests were narrowly amended to the accepted pairing auditor v5 before launch. Only auditor path/hash and approval subject/token changed; source, clock, openings, mask, widths and stopping rule remain fixed. The old approved manifests remain preserved.

## Numerical and performance evidence

Counter compatibility preserves the upstream float32 operation order. An actual AMD64-v3 compiler witness reproduced fused multiply-add output -2^-46 where the reference required +0. Explicit float32 product rounding fixed that difference; v1/v3 official reference tests passed. This floating-point issue does not explain the earlier integer NGN-v1 training losses.

The Counter transition optimization removes generic NGN-v1 move preparation while retaining checked board updates, private contexts and exact arithmetic. Its equal-weight median candidate/parent ratio was 0.9453312363677768 across six fixtures and 14 paired blocks. All fixture medians improved, both execution-order strata improved, and cold/warm correctness snapshots were identical. These are cold first searches: lazy TT initialization is inside the timer, accounting for roughly 135 MiB/op rather than per-node allocation. Root performance acceptance:
`/home/ehrli/repos/ngn-counter-transition-fastpath-v1/output/counter-transition-fastpath-v1-20260906/root-performance-terminal-review-v1.json`,
SHA `71d959e30c98351b52ddbd022edb1ee6caf48408c388930938d195b43441c42f`.

The accepted profile attributed 33.56% flat CPU to scalar evaluation and 25.28% to feature updates. The isolated AVX2 prototype addresses feature updates only; row order and scalar output-dot arithmetic remain unchanged. Actual v1/v3 tests covered every accumulator lane, explicit rounding/signed-zero cases, official full/incremental oracles and fixed search/full-refresh assertions. GNU disassembly confirms one eight-lane add/sub instruction in a 64-iteration loop. A Go disassembler decoding failure was preserved and checked against GNU output; production code did not change for that diagnostic.

Counter multicore ownership now has an accepted eight-worker witness using the exact accepted network, in addition to the existing three-worker fixture. Every worker reaches a committed move with a matching board and distinct context; stop joins every worker; warm/NewGame roots restore full-refresh values; model/backend replacement invalidates contexts, TT and history. Both normal and race executions passed. The extension is integrated at 89bdc27. Root acceptance: `/home/ehrli/repos/ngn-counter8-witness/output/counter8-witness-20260906/root-counter8-review-v1.json`, SHA `6fecaa1d6a8f43a5f18b0def313aab11fc4aaf1197ea397f20c894516eba858f`. An earlier direct grep missed the helper-based three-worker test; that review inference was corrected.

The AVX2 performance test is now accepted: all 34 measured stages completed successfully, and root independently recomputed all 84 paired fixture ratios from raw timing output. Equal-weight median AVX/scalar time was 0.8019046944948325; order medians were 0.800490409 and 0.802041782; every fixture median was below 0.813. Cold/warm correctness snapshots were byte-identical. The usable search profiles contained 32.09 scalar and 25.53 AVX CPU-sample seconds; feature-update flat share fell from 19.60% to 3.14%. This remains a cold-first-search measurement including lazy TT allocation. A postmeasurement shell error was preserved and repaired with a postprocess-only continuation, with no measurement reruns. Root acceptance: `/home/ehrli/repos/ngn-counter-feature-avx2/output/counter-feature-avx2-20260906/root-performance-review-v1.json`, SHA `d1b1bf8877e285a879244f6f1b0207be339d2b99f5fe5bc29d81a4411d90e2fc`.

The combined-match opening/runner validation is also accepted. The pinned official Stockfish 8moves_v3 corpus contained 34,700 source records; one terminal row (ordinal 16,703) was recorded and excluded before deterministic selection. All 200 selected 16-ply openings have exact independently checked PGN/UCI histories and unique legal, nonterminal final positions. All eight validation stages and nine focused fixture methods passed; an aggregation-only continuation reused their actual outputs. No candidate games informed selection. Root acceptance: `/home/ehrli/repos/ngn-nnue8-hce8-runner/output/root-combined-opening-validation-review-v1.json`, SHA `9f6dddebb61d92f497d8ab99feecd362ddf5ee791d72fa4a88380153132f6335`. The final binary and its actual runtime admission remain pending.

## Stockfish compatibility boundaries

SMALL's actual upstream incremental producer exercised reuse, incremental update and refresh paths across 438 core and 265 growth states, including 132 real plies followed by full unwind. The Go consumer compared both full-position and compact transitions, all recorded accumulator/PSQT/layer values and public evaluation results. Focused and race gates each passed nine tests with two intentional official-input skips; the explicit official gate passed both 703-state consumers. Vet/build and actual outer execution passed with no survivors. Root acceptance:
`output/sf18-small-incremental-oracle-20260906/root-go-terminal-review-v1.json`,
SHA `cf7ca83c139aa44b113ed8546b1d98eafdc401c4337523a2e7673f504c835e8e`.

BIG's separate strict loader passed an independent canonical grammar/EOF parse, ordered transformer and eight-stack spot checks, 13 focused tests and the same race tests, one actual official-model integration, vet and build. Root acceptance:
`output/sf18-big-plan-20260906/official-big-v1/spot-execution-v1/root-terminal-review-v1.json`,
SHA `67688fe7163ae372c09283f4d34a4fd73339867685a8a2cd9230ae3281b02e27`.

This does not yet reproduce Stockfish's general evaluation: BIG threat features and inference, dual-network selection, and NGN score/search integration remain separate. Arbitrary .nnue files are not supported. All failed mechanical launcher attempts and their successful continuations remain preserved.

## Next release decisions

1. Finish and review explicit evaluator startup flags and the versioned Counter launcher, then run the final combined-head short/concurrency/startup/build checks. Existing arithmetic oracles remain valid for unchanged source.
2. Complete HCE2v1 and HCE4v1 using the fixed original SMP binary and amended auditor bindings. HCE2 launched as `ngn-sprt-width-contract/output/m4c-hce2v1-sprt-20260906-attempt1`; reserve the brief integration/admission window after its actual terminal audits and before HCE4.
3. Freeze the final candidate binary and run Counter8/HCE8 at 60+0.6 with 200 independently selected, legally replayed 16-ply openings and the predeclared first-crossing rule.
4. Admit and play matched external opponents, led by Counter 5.5 and Rodent V1.2, retaining V1.1 as the published-rating anchor. CCRL does not identify the personality behind its V1.1 Blitz row; do not equate our testers artifact with that exact binary without evidence.
5. Build and verify the final WSL release, archive both candidate and rollback artifacts, deploy under the user's existing authorization, and verify the deployed behavior.

The first Counter standalone handshake ran under a one-CPU mask, so its advertised Threads max=1 did not establish an engine limit. Fresh external admission must use the actual match mask. No external opponent games or new absolute WSL rating have been accepted.

Detailed plans remain in `2026-09-05-next-stage-roadmap.md`, `2026-09-06-release-confirmation-plan.md`, `2026-09-06-sf18-big-dual-plan.md` and `2026-09-06-rodent-v1.1-release-confirmation.md`. NPS, training loss, format compatibility and favorable incomplete scores do not satisfy the remaining release gates.
