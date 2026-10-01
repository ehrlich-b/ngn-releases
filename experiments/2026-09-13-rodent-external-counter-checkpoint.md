# Optimized Rodent NGN versus external Counter 5.5

Status: accepted shared-host checkpoint. Fresh `run-003` completed the fixed
100-game A/A and 400-game candidate sample while the Lean hopper remained
running. NGN Rodent scored +28.73 paired Elo with a 95% paired-bootstrap interval
of [+6.08,+51.62] against exact Counter 5.5. This is not an absolute-rating or
3300 claim. The original candidate operational-audit failure and the bounded
post-hoc concurrency-key correction are both preserved.

The old external rating evidence predates the accepted Counter output speedup,
Rodent integration and its +96-Elo internal gate, and the subsequent 2.43x Rodent
search speedup. One fresh external comparison is now useful for prioritization.
This is not a repeat of the old broad calibration campaign, and no own-network
training or new search heuristic is part of it.

## Executed fixed protocol

- Latest accepted NGN `52ee629`, `rodent-v1.1-anand`, exact existing Anand weights.
- Opponent: the exact Counter 5.5 release artifact from the accepted historical
  one-thread cell, with its supported options and actual NNUE path verified.
- Fresh 100-game current-NGN/current-NGN A/A, then 400 games / 200 reversed-color
  opening pairs. 10+0.1, concurrency four, one thread each, Hash128, no book and
  no adjudication; preserve the established affinity and full interference
  telemetry. Per the September 18 user direction, the host is deliberately
  shared with the continuously running Lean hopper and background CPU alone is
  not a rejection condition.
  NGN Move Overhead is 100 throughout preflight and both phases; Counter has no
  such option and retains its native time policy.
- Freeze build/source, executable/model/opening hashes, supported options and
  the minimal reused harness before release. Do not apply NGN-only diagnostic
  expectations to the external engine or silently weaken its checks.
- Paired 100,000-resample bootstrap, seed 2026091201. A/A interval must include
  zero. Complete legal/terminal/operational/process checks are required.
- Fixed cap: no score-based extension or rerun. Report the NGN-relative WDL,
  pentanomial and interval irrespective of sign. This is a checkpoint, not a
  keep/shelve gate for already accepted pure-speed code.

## Interpretation boundary

The result measures the remaining gap to this exact external engine under these
local shared-host settings. Scheduler and time-management interference may
affect the estimate. It cannot establish a formal list rating or simply be added
to prior internal gains. Historical Counter rating labels are conditional
orientation only: clock, machine, opponent pool and artifact correspondence
remain separate uncertainties. Absolute 3300 is unverified until adequate fresh
configuration-matched evidence supports that claim.

All execution stays on the authorized WSL host. No installed binary, default
backend or external release contract changes are authorized by this checkpoint.

## Frozen release

Latest NGN source is `52ee629b316f89373513cf9ef3be819d4555c3f3`, tree
`b7dfd988f8c3f4ee1d8a4a46d68bd2764ed2241a`; binary SHA-256
`b73491e7cfbe927b28215a87d32102a275f6c7cff1cd06ac4d2bf79299ce1bfd`.
The exact external Counter binary is
`6c48fb52934d49d3774633e32f0b4fb4796b1e2c63925f24167ef0f0c0d761c8`,
release v1.55.0, source `63c487ca724c620f71c129d62129c6fb9109c872`.
It uses embedded NNUE with release AVX assembly and native Threads1/Hash128/
ExperimentSettings=false. No external EvalFile or overhead option is invented.

Reviewed driver SHA-256:
`74640c18d6c98c28bd8110e0515e243af8b4f35fa0d09c063af84efc424bd838`.
Held manifest SHA-256:
`1fe22dcd81ed3b6ee522eeedc217c8c31ec0370e7f7e867348333b2ae7c38785`.
Static checking passed all 21 frozen inputs and both executable hashes. Release
changes only the manifest's release-state field. Target is one immutable
`/home/ehrli/repos/ngn-external-counter-checkpoint-gate-20260913/run-001`.

The NGN phase is strict. The external phase retains the previously verified
Counter-specific fifty-move/threefold PV-warning allowlist; all other warnings
remain fatal. Exact startup, embedded-network and option evidence are checked,
with process attribution supplied by the independent process witness. Both
crash-log parsers handle valid interleaved startup records without relaxing
record counts, timestamps or paths.

## Recovered execution disposition

The September 14 execution did not produce an accepted checkpoint result.

- `run-001` stopped before any game because the first driver revision expected
  NGN's startup log in the wrong directory. The directory is preserved. No
  engine or match result was involved.
- `run-002` used the corrected role layout. Its A/A phase completed 100 games / 50
  pairs, scored exactly 50%, and passed its legal match audit. The candidate
  supervisor then completed normally after 3,359.073 seconds; fastchess exited
  zero, its process witness recorded 478,456 observations across the exact two
  frozen binary hashes on CPUs 0, 2, 4 and 6, found no violations, and found no
  survivors.
- The candidate timing evidence nevertheless failed the frozen interference
  rule. Across 672 five-second samples, 509 exceeded one non-owned CPU core and
  the peak was 2.1869 cores. Separate `python` and `opencode` work dominated the
  load. This is sustained interference, not a marginal sample.
- The driver rejected the phase before candidate legal/result audit or paired
  Elo calculation. The candidate PGN is frozen but must not be scored, audited
  for a rating result, pooled, extended or used to choose whether to rerun. No
  external result or rating increment is accepted from either directory.

The exact `run-002` evidence remains on the authorized host at
`/home/ehrli/repos/ngn-external-counter-checkpoint-gate-20260913/run-002`.
Key SHA-256s are:

- frozen manifest: `c2b707223e173d84e26ac5ad3ecb1f3fdbc44cd04759635cb3dacf9ee956c52f`;
- A/A match audit: `fea84ee74c0cd565f4294dea004534092a0a9dd819b938c1ad7a0779275e83b8`;
- candidate CPU receipt: `d1526e80427f0ec77507ed5fade8c6c77163830527d5c4017bd6df9d1caedab9`;
- candidate supervisor: `6e5bd45597b950b2815451819f3ffc5ae8dd83c0aa33bc06ddb6c54702081d5a`;
- candidate process witness: `d27d94bc91cf835b469ff90284cae73422c53f8426f9fa6bf8fd2394e2031299`;
- frozen, unaccepted candidate PGN: `026956e2f95f397f5c60e6aab974cb5a2d6b91588d1f3759aee7bc5ea5116bcd`.

At that point one operational replacement remained scientifically admissible
because the rejection had been declared without reading or computing the
candidate score. The first replacement package, `held-v3`, retained the
exclusive-host rule and was not launched. Its byte-identical driver, all 21
frozen inputs and both engine binaries hash-checked, and the intended `run-003`
root did not exist. These preparation hashes remain part of the history:

- held-v3 driver:
  `0d204c66d49f930bcba1b09a88a60530ef260f51a659d88e37333b26d15f6344`;
- held-v3 manifest:
  `7d4c8cb12ad4616ad108182807bbcc4a2292a1e2a82647e22fb009eb0ed09ce0`;
- held-v3 replacement receipt:
  `8355e7c4b78977e41afc7f2c264e7994fb5183a4742c7bf9cdaee3000d252452`;
- prior v2 layout-preflight receipt:
  `f01c64c69d3990880e8c4b9dcfcc966a738860191d353e91e81b90a227e1e995`.

## Shared-host amendment and release

On September 18 the user explicitly directed that the hopper not be paused and
that the machine be shared. Before any fresh game result, `held-v4` amended only
the interference disposition:

- the Lean hopper stays running and may vary during both phases;
- the ten-second prelaunch sample and five-second continuous CPU samples remain;
- the old one-core-for-twelve-samples test remains recorded as counterfactual
  telemetry, but background CPU alone does not invalidate the sample;
- engines, network, openings, options, 10+0.1 time control, concurrency four,
  CPU masks 0/2/4/6, 100+400 game counts, seeds, audits and the no-extension rule
  remain unchanged;
- the result is explicitly shared-host observational evidence, susceptible to
  scheduler/time-management interference and not an absolute-rating claim.

The release records an ordinary root static diff/hash/syntax audit, not an
independent review. That distinction is explicit because no independent agent
was authorized for this turn. Static checking again hash-checked all 21 frozen
inputs and both engines without executing them. `run-003` was absent and the
hopper controller was alive immediately before launch.

- v4 driver: `d426dbaa603e577473222d9e58726eebc220fa7fc053c7c84a3c6585b645df61`;
- v4 manifest: `2a330ce56cbab969ea5c919991924ac797291c78ba940da107bfce4db26eb135`;
- static check: `978896aa2c92472c66bcaa899d6543bf4c4638240faf79199c29edc6474b226f`;
- pre-result release receipt:
  `268686d8f6d9739541add0da434256b5175a54120192214d6e03eb9432def3f2`.

## Run-003 result

The fresh sample completed on the authorized WSL host while the hopper remained
running.

- A/A: 100 games / 50 pairs, 52.0/100, pentanomial `[2,9,24,13,2]`,
  +13.90 paired Elo, paired-bootstrap 95% interval [−27.85,+56.07]. The
  interval includes zero. The supervisor exited cleanly; the witness recorded
  124,703 observations across four instances per role with no violation or
  survivor.
- Candidate: 400 games / 200 pairs, 135 wins / 163 draws / 102 losses,
  216.5/400 (54.125%), pentanomial `[8,37,86,52,17]`. NGN-relative paired Elo
  is **+28.73**, normal 95% interval [+5.60,+52.12], and the predeclared
  100,000-resample paired-bootstrap interval is **[+6.08,+51.62]**. The legal
  audit passed all 400 games, 200 reversed-color pairs and 67,082 plies.
- Candidate supervision was clean after 3,402.408 seconds. Fastchess returned
  zero. The process witness recorded 495,536 observations across four exact
  instances per role, no violations and no survivors.
- Shared-host CPU telemetry did not trigger even the old counterfactual gate.
  A/A used 171 samples, mean 0.1225 and peak 0.5217 non-owned cores. Candidate
  used 681 samples, mean 0.2591 and peak 1.0185 non-owned cores; the peak was not
  sustained for twelve samples. This does not remove the declared shared-host
  limitation.

## Preserved audit correction

The frozen external trace auditor failed after the games and legal audit had
completed. Its refresh state used `(fastchess thread, display name)`, but its
pending `go`/width/`bestmove` state used only display name. Four legitimate
simultaneous games therefore appeared as duplicate `go` commands and orphaned
`bestmove` responses. The original failed receipt is preserved at SHA-256
`5f2a72bae806d8fea179613d7830a30597c88e36571157b26498a7c6cf7b19cb`.

Because the score was visible by then, no match rerun, extension, game filter or
result-dependent protocol change was allowed. A separately frozen post-hoc
auditor changed only those pending-state keys to `(trace thread, display name)`
and reran every original operational check over the unchanged trace. It passed:
all 400 option refreshes per role, exact process-exit counts, Counter startup
records, warning allowlist, clock commands, width receipts, crash logs and
terminal format remained enforced.

- corrected auditor:
  `b890943cab479612cae46d9c502c6d778b5bcfa06a1a16803ae436534fa5e290`;
- correction manifest:
  `6e5079ac4647017087c32e21f032b2ebc97282409ecb5e4fac10c688634be3c2`;
- corrected audit receipt:
  `2f837c8a38e1c1c24f752ea9c1a8b7803f4daa188a7381f8624abe0ada9b9f90`;
- unchanged candidate trace:
  `7cef8f01d612d0673c18056dce04aa25fdf12978180d9d122cbfaf6ab9824277`.

The no-game finalizer hash-bound the original and corrected receipts, recomputed
the predeclared paired statistics, and wrote a post-hoc provenance-aware terminal
chain. The original driver is correctly recorded as not having reached its own
terminal step; the game sample is complete and accepted through the preserved
correction chain.

- frozen run manifest:
  `1388cd74030c9202e04feca00dad12a8b315cc94d639187482b9886338e5ed80`;
- finalization manifest:
  `4b07a94a09a4f8c1624b58a65ebbf5ba1bba532fefab14efdc64538c0f130b46`;
- result:
  `8cd41334e71f731dfd829a9ab6316be2ed446e73a9bae868ff7aeaf558c0b7a1`;
- inventory:
  `f63eb345349563ad76cf87b8e8adf99fe2d16ef7117cecf696fcab088a2ba8bd`;
- terminal:
  `90ddf9648e460fe598441bb05c57ec10beebaa5064a5ac10d8dae71d818eebd5`.

The immutable run root is
`/home/ehrli/repos/ngn-external-counter-checkpoint-gate-20260913/run-003`.
This result establishes only that current NGN Rodent outperformed this exact
Counter 5.5 artifact at these local shared-host settings. It does not establish
a formal list rating or verify the active 3300 goal.
