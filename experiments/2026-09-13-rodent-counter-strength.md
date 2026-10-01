# Optimized Rodent versus Counter: accepted

Status: **completed and accepted**. Optimized Rodent scored **187W / 134D / 79L**,
63.5%, against Counter in 400 paired games: **+96.19 Elo [69.53,123.02]** under
the predeclared paired bootstrap. Both configurations used the same accepted
binary. This establishes a relative gain at the tested settings, not absolute
3300, a wider-thread gain, or a default/deployment change.

Completed fixed protocol: fresh 100-game Counter/Counter A/A, then 400 paired
Rodent/Counter games, 10+0.1, concurrency 4, one thread per engine, Hash128,
OwnBook false, CPUs 0/2/4/6, and the first 200 frozen openings with color reversal.
The candidate is optimized `rodent-v1.1-anand`; the control is `counter-5.5`.
All other settings and the executable must match. The final manifest must pin
the actual binary, both existing networks, opening corpus and harness sources
before release. The actual freeze and attempted release are recorded below.

The rule was to keep the candidate configuration only if the paired 95% lower bound
is positive and all operational/legal/terminal audits pass. Otherwise shelve at
the fixed cap, with no extension. Do not reinterpret this as absolute Elo or pool
it with historical external cells. No own-network training or extra calibration.

## Harness preparation

The current UCI preflight had an obsolete HCE/NGN-v1-only backend allowlist and
diagnostic regex. The narrow repair admits the exact Counter and Rodent names
while retaining advertised-option membership checks and exact selected-backend
diagnostic equality. Tests prove both names and reject a mismatched diagnostic.
The generic HCE-vs-NGN manifest schema remains untouched; the established fixed
game driver does not use it.

The complete current candidate-match Python suite with these two file changes
passes on WSL: `python3 -m unittest discover -s tests -v`, 29 tests in 3.304s.
The [raw log](2026-09-13-rodent-anand-artifacts/strength-preflight/harness-tests.txt)
has SHA-256 `1398dddd29ddbd35f41cc4b7b53321e3c659834c30c03922b77c479ff2434644`.
An earlier narrower/stale discovery is not the acceptance evidence. No engines
or games were run for this harness-unit check. The previously repaired startup-log
auditor is retained, and the future game run still requires its fresh A/A.

## Frozen release and historical prelaunch hold

Root reconciled the actual clean build receipt, commit/tree and shared binary,
and independent source review accepted the orientation, bootstrap and runtime
selection/witness contracts. Both configured backend preflights pass. Final source
is `762377947c0907d1ce6be74cb71e01521610bd76`, tree
`59587882012a4bd0a64fdac2678a0b0cee01c143`; both roles use the same CGO-disabled
Linux/amd64 v3 binary, SHA-256
`5547e3481f63789df59f0ab5d059212bb6c61e2780c898f93ee7afb82c73dbf8`.

The released manifest is SHA-256
`0331be68c61106fc5858737e6d6f180071c5b2984a9df9bc2d0e0e7b7a2edd80`.
All fourteen frozen sources and the engine hash passed the held check. The actual
launch then failed closed before creating `run-001`: its ten-second prelaunch
sample measured 1.2472 non-owned CPU cores, above the declared maximum of one.
The leading process was an unrelated Python validation job; root confirmed it
was still live after a five-minute idle wait. It was not stopped or reprioritized.
An earlier WSL service timeout cleared on read-only checks; WSL was not restarted.

No games or run directory existed at that rejection. After a ten-minute backoff,
root verified the interfering PID had exited. One fresh prelaunch used a second
manifest filename with byte-identical contents, preserving the first rejected
sample. Its admission passed and the single controller completed both phases.
No service restart, parallel controller, result-based extension or game rerun occurred.

The [historical prelaunch status](2026-09-13-rodent-anand-artifacts/strength-preflight/status.json)
and adjacent release/build files remain unchanged. The completed WSL run is
`/home/ehrli/repos/ngn-rodent-counter-strength-gate-20260913/run-001`.

## Completed result and independent acceptance

- A/A: 100 games / 50 pairs, role A 31W / 43D / 26L, pentanomial
  `[2,10,22,13,3]`; +17.39 Elo, paired interval `[-27.85,+63.23]` includes zero.
  Legal replay passes all 15,167 plies; operational audit passes.
- Rodent A versus Counter B: 400 games / 200 color-swapped pairs, **187W / 134D /
  79L**, 254 points, pentanomial `[5,24,70,60,41]`. Legal replay passes all
  **60,198 plies**, and every terminal result is audited.
- Paired bootstrap: seed `2026091201`, 100,000 resamples, estimate
  `+96.19234433420041`, lower95 `+69.53219240387646`, upper95
  `+123.02434512917564`. Positive lower bound satisfies the frozen **ADOPT** rule.
- Both phases pass exact backend/option refresh checks, process witnesses and
  cleanup. Four instances per role share the frozen executable hash, GOMAXPROCS1
  and admitted CPU masks. No crashes, illegal moves, flag-outs, canonical errors,
  book signatures or surviving owned processes are reported.
- Interference passes the actual frozen rule: A/A has only one consecutive
  sample above one core, below the twelve-sample rejection threshold; candidate
  has none. Do not mistake A/A's isolated 1.927-core maximum for a sustained breach.

An independent Sol audit reconstructed each engine-A score from PGN player/color
tags, reproduced the bootstrap, checked all source/model/binary identities and
rebuilt the full inventory. The PGN result totals `153/134/113` describe White
wins/draws/Black wins, **not Rodent WDL**; the correct engine-relative counts above
are independently verified.

Terminal decision is `COMPLETE / ADOPT`, with `no_extension_after_observation=true`.
Decision SHA-256: `72eb68607041a51e5dc2520d6363bfaab42a16e3189085973786a248a385e285`.
Terminal SHA-256: `65b3485b9ceb37c5a511eed328f1398a3e33810601092ac5f253804f38a64efc`.
Inventory SHA-256: `e64ba129bb935c4a5348f264319ecfdb9973ffc47f400105ba7970aa68efa6ef`.

The [compact acceptance archive](2026-09-13-rodent-anand-artifacts/strength-result/results.json)
includes exact terminal/decision/freeze/inventory files, supervisor receipts,
explicitly labeled audit summaries, and paths/checksums for larger WSL evidence.

Rodent is now the strongest verified one-thread short-clock configuration in this
direct comparison. Keep its exact existing external network and optimized output
path as the next working reference. The next bounded task is a post-output profile
to verify the remaining hotspot before another exact speed candidate. No additional
absolute-rating calibration or owned-network training is needed yet.
