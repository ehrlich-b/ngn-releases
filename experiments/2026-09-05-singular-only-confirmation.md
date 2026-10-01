# Coherent child depth with singular extensions only

**Superseded by the [T8-based confirmation](2026-09-05-singular-t8-confirmation.md).**
The qcap-based candidate below was never launched. T8 completed H1, and the
conditional controller stopped for root integration as intended. The remainder
records the original frozen artifacts/protocol; do not launch them on the new base.

Candidate built, root-reviewed and staged, **not launched or accepted**. This is the single candidate
selected from the [fixed four-arm diagnostic](2026-09-05-depth-policy.md).
It is a search heuristic experiment, not an automatic correctness keep.

## Behavior and prerequisite

Remove positive check and passed-pawn extensions. Preserve singular positive
and negative extensions, the extension budget, all legality/terminal/stop rules,
evaluation, ordering and the other pruning/reduction policies. Initial nonfirst
scouts use the already-computed child depth minus the reduction; remove the old
reclamp that discarded extensions. The diagnostic D variant combined these
changes explicitly. It produced six changed choices without a negative SF
child-score change on the23 frozen cases, but that selected sample does not
establish strength. Canonical tree cost increased on two of three roots.

The isolated candidate must contain no diagnostic switches, per-node logging,
extra counters or UCI changes. Require full node/score/PV/bestmove identity with
the D overlay on all three canonical roots and all23 full-history400k-node cases,
Hash64/Threads1, plus full short/race/vet and the independent legal-move oracle.

The classical fit was shelved at its completed inconclusive cap; the running
T17+T8 confirmation comes first. This
qcap-based candidate can launch as built only if qcap remains the accepted
immediate base. An accepted intervening change requires rebasing/rebuilding,
repeat validation and replacement frozen artifact identities before launch.
Do not silently test this old candidate against a newer base.

## Prospective game rule

Paired SPRT **[0,+10]**, alpha/beta .05, minimum200 games, maximum1600 games.
Accept only a completed H1 with zero operational errors and zero clock losses
on either side. H0 rejects; capped inconclusive shelves. No extension, pooling
with other variants, provisional keep on positive sign, or parameter changes
after results begin. No fitted evaluation or quiet-promotion patch is bundled.

Native Windows9800X3D,10+0.1, concurrency8/default affinity, lowpower=false.
Use `sprt_20260904.exe` SHA-256
`63ef86d9b986fdb2c260782149af259e278845d89595d010df3970e6698cc9ed`
and5000-line book `sprt_openings.txt` SHA-256
`974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222`.
Standard adjudication: resign900cp/5plies, draw10cp/10plies after80plies.
The cap is approximately100 minutes at recent throughput; no other native
Windows/WSL engine CPU work may overlap.

Require a completed A/A on the exact baseline/settings. The completed
`r0905qbaseaa` may be reused only if qcap, harness, book and machine configuration
remain unchanged. Any accepted different base requires its own A/A. Independently
check idle workers, remote hashes and native candidate UCI smoke before launch.
Record the final command/run name and all identities before the first game.

## Frozen isolated artifacts

Source archived from `98968fc823c91a501fc985cfad9cfda6a20b10a6` into
`output/singular-only-candidate-20260905/src`. Root compared every tracked
Go/module file with main: only `engine/search.go` differs. The patch removes
33 lines and adds8. It preserves the D diagnostic's nonnegative scout-depth
floor so a negative remaining-depth value cannot fall into the raw-eval branch.
No diagnostic data structure, switch or UCI edit remains.

- Normalized applicable diff `output/singular-only-candidate-20260905/singular-only-runtime.patch`:
  SHA-256 `efbafd010d1dcb552f6c3f3a401ba215b67000da4acc20c69a914f2497a15fc7`.
  Root `git apply --check` passes without modifying main. The original pathful
  `singular-only.patch` is retained separately.
- Mac `build/ngn_20260905_singularonly`: SHA-256
  `e9542e597ca327ee9bf0a539c906e61d84d187dcd779efd740c680374d581641`.
- Windows `build/ngn_20260905_singularonly.exe`: SHA-256
  `0a4a8ec0db97e321ec2e6e82cd73975cec98226416f22f627992d9961594740e`.
  Go1.26.2, Windows/amd64/v3, CGO disabled; root inspected build metadata.
- `plain-D-identity.json`: SHA-256
  `c7bee2ce98a60655e9ec1a5553f10168a11d68e757f2de0c1873811ca86b7d75`.
  All26 node/score/full-PV/bestmove comparisons pass; root checked the assertions
  and raw records. Logs in the same isolated directory show full short/race,
  vet and independent oracle passes. The oracle ran rather than skipped:
  2,293 positions,48,926 depth-2 root divides,128 raw search roots, all matching
  with state restoration.

Root staged the Windows binary and reverified its remote SHA-256 above. It is
not installed or running. Fresh Mac EP/castling/fifty-move-mate smoke passes;
all14 advertised defaults retain the qcap values. A matching native script is
staged but must run only after T8 completes and native workers are idle.

## Conditional unchanged-base handoff

The prospective run name is **`r0905singularqcap`**. Its complete command,
frozen before launch, is:

```text
BOXSPRT_BIN=sprt_20260904.exe scripts/boxsprt.sh launch r0905singularqcap -- -new '.\ngn_20260905_singularonly.exe' -base '.\ngn_20260904_qcap.exe' -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 0 -elo1 10 -alpha 0.05 -beta 0.05 -maxgames 1600 -mingames 200 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
```

To avoid repeated manual polling, root prepared a bounded controller for the
already authorized negative-result branch. It may launch this exact candidate
only after a fresh fetch and full audit establishes that T8 completed cleanly
with H0 or its inconclusive cap. It stops on T8 H1 for root integration/rebuild;
it never accepts an engine into main, changes defaults or starts an external pin.
This changes the handoff mechanism, not either frozen game decision rule.

Before the successor launch it verifies frozen local source/artifact hashes,
the original A/A log and interval, remote binary/harness/book/script hashes,
the installed Windows default still being qcap, idle native chess workers,
an unused run name, and the native candidate's bounded UCI smoke. Any failed
prerequisite stops the handoff. Exclusive local claim prevents duplicate
controllers; an ambiguous launch error is never retried automatically.

The controller then collects the singular-only result and stops for root review.
Collector timeout or lost observation is not a terminal engine verdict and
does not authorize a restart. The live T8 collector remains responsible for
its existing match. No Windows/WSL engine CPU is added while that match runs.

Controller and offline integration tests are under
`output/recovery-2026-09-04/{conditional_singular_handoff.py,test_conditional_singular_handoff.py}`.
Frozen inputs and state are `singular-handoff-inputs.json` and
`singular-handoff-controller.json`.

**Armed after independent validation and root review.** Nine offline integration
tests use temporary fixture directories and the real completed-log auditor.
They verify exactly one frozen launch after cap, no launch after H1, invalid
completion, changed local/remote hashes, active workers or failed native smoke,
exclusive claiming, and no retry after ambiguous launch failure. The actual
baseline A/A is byte-copied and SHA-verified; synthetic candidate fixtures are
explicitly test data. Fakes reject unknown commands and assert complete launch
arguments plus the versioned harness selector. Root reran all nine tests and
reviewed their assertions; log `singular-handoff-root-validation.txt`.

Review caught and repaired the native guard's missing PowerShell terminating-error
setting. A read-only native probe then verified the installed qcap hash and unused
run name and rejected the active T8 workers as required. Native candidate searches
remain deferred until idle. The controller now waits for T8; no successor is live.

Frozen SHA-256s:

- Controller: `be7b720f090646a5a6ca3e2840d346d1b48f368eac2e4a47429b2bb17ae017b9`.
- Inputs (326 local source/artifact files, five remote files):
  `70b3079687b8913d6163fd5d0a03b178a4cf0e7fab40d438ec8719ac4db56b90`.
- Integration tests: `c37b3bbf21e91858790ce5d439564a2d0853b355c69d745a787c109e330ed728`.
- Native smoke script: `18ba3f7635c9967e2fc75e42e25f887df04cdd1dd88f6df31e578fe3321c05fb`.

The candidate's self-play gate cannot establish the absolute2800 goal. An
accepted engine gain proceeds to the [prospective external confirmation](2026-09-05-2800-pin-plan.md).
