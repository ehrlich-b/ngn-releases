# Counter correction-history interaction

Active thread goal: **raise the rating to 3300**. This experiment is a step
toward that objective, not a replacement completion condition. Use one-thread
external strength as the working configuration; absolute completion will require
fresh, configuration-matched rating evidence, not adding internal Elo estimates.
The earlier roughly 3000–3100 orientation remains uncertain and unchanged.

Immediate base: `270c73563139996d03f3c34b2840015b8e130df9`. Production retains
the accepted output AVX2 kernel; the separately archived copy/update fusion is
not included. The previous goal turn counts as progress because its completed
measurement rejected the next speed candidate under its declared rule and
changed the ranked next action. It produced no accepted engine gain.

## Bounded next step

Sol agents own source analysis, diagnostic instrumentation and independent
measurement review. All builds, engines and tests remain on authorized WSL.
No own-network training, deployment, broad tuning or new absolute calibration
is part of this initial diagnostic.

Inspect the existing three-term correction family with the Counter evaluator:
where it is applied, what targets update it, how TT refinement interacts, and
which pruning/reduction decisions consume it. Reuse the deterministic first
three accepted Counter1 game histories at plies 23 and 48, retaining one engine
across both searches per game. This is sparse-prefix retained-state replay,
not a reconstruction of every timed game search.

The diagnostic must preserve baseline search identity and distinguish raw
adapted eval, the three correction contributions, corrected static, TT-refined
static and downstream comparisons. Its purpose is to justify one coherent
Counter-only family experiment, not select a parameter by its sampled score.
Do not stack a correction change with TT, futility, extension or LMR changes.

Any candidate touching evaluation/search decisions requires a fresh identical-
configuration A/A and a predeclared real-clock paired game gate against this
immediate base. Freeze its source, inputs, environment, sample and decision rule
before candidate games. Reuse the audited runner where possible; preserve failed
attempts and never extend a sample after seeing its result. Diagnostic node
counts, choices and reference scores cannot establish playing strength.

## Status

The passive diagnostic is complete. Root admitted one Counter-only minor-
correction-off candidate for implementation and a separately frozen game gate.
Sol completed implementation and independent review. The released gate passed
A/A and completed all 400 candidate games, but its frozen operational audit
failed on benign concurrent startup-log interleaving. **Not adopted / shelved**;
no accepted Elo estimate, extension or game replay. The 3300 goal remains active
and unproved. See the terminal disposition below.

Source review finds no correctness defect or obviously excessive scale. Exact
integer saturation is ±48 cp per term, ±144 cp total; the three tables learn
the same residual against the already-corrected pre-TT static. This prevents
the simplistic claim that each independently learns the full raw error and
triples it. The narrower concern is overlap between minor-piece and non-pawn
keys; the classical minor-term gain was only about +3.7 Elo, with no proved
Counter-specific harm. An ordinary nonzero contribution alone does not establish
a reason to change production policy.

The passive diagnostic reports aggregate magnitudes and local comparison
changes on the baseline's reached nodes. These are not an alternate search
trajectory or evidence of stronger moves. Sol is separately implementing inactive
compatibility for the exact existing Rodent V1.1 Anand network in parallel.

The admitted candidate's reviewed game plan is the existing 10+0.1,
concurrency-four, one-thread-per-engine protocol:
fresh AA100 then fixed candidate400, adopting only with positive paired95
lower bound and all integrity checks. Root released exactly this gate after the
completed checks below.

## Completed diagnostic

The single monitored run passed in 4.317 seconds, with empty stderr, no survivors
and 329,728 KiB peak sampled process-tree RSS. All six control/instrumented
trajectories exactly match each other and the frozen untouched baseline.
Games 1 and 2 share the ply-23 position/trajectory: these are six declared
observations, not six independent positions. No sample was substituted.

Across 1,884,322 reached static observations, the minor term is nonzero on 83.3%
and has mean absolute magnitude 14.92 cp, versus 9.55 cp pawn and 3.53 cp
non-pawn. Aggregate correction mean magnitude is 27.10 cp, with no table entry
at its saturation limit. Holding the reached path, actual improving value and
other eligibility conditions fixed, minor removal changes 37,024 RFP, 16,879
NMP and 57,245 qsearch beta-stand-pat comparisons. These counts are local
comparison opportunities, including comparisons after which an earlier
pruning return could prevent execution—not actual alternate-branch counts.

The [frozen summary](2026-09-13-counter-correction-artifacts/diagnostic/summary.json)
changes the priority assessment: minor influence is not negligible in this
sample. It supplies no evidence of better moves or Elo direction. Across all
five distinct sampled root trajectories, mean absolute minor contribution is
7.74–23.86 cp and local qsearch beta-comparison changes span 3.16–10.92%.
The effect is not confined to the duplicated trajectory. Root judged this
materiality plus the structural key-overlap hypothesis sufficient to admit one
real-clock minor-only ablation. The Sol measurement agent recommended awaiting
directional evidence; root distinguishes admission to a falsifiable game
experiment from adoption of the change.

The candidate disables only the Counter backend's minor correction application
and learning. Pawn and non-pawn correction, other evaluator backends, table
storage/lifecycle, TT policy, pruning, timing, SMP and the output kernel remain
unchanged. No new UCI option or parameter sweep is admitted. All owned builds
and oracle checks must finish before the timed A/A begins. A failed or
inconclusive fixed gate shelves this candidate without extension.

Full raw observations remain WSL at
`/home/ehrli/repos/ngn-correction-profile-20260913/output/evidence-v1/correction-profile.json`,
SHA-256 `545462126185600889cedf50ebafc08915be5f1f829be2ca7edd80c6287cda66`.
The compact archive includes observer source, commands, summarizer, summary
and supervisor receipt. Its `wsl-SHA256SUMS` inventories the full WSL bundle;
the raw JSON and unchanged helper probes are intentionally not duplicated.

## Frozen candidate and validation

The candidate is game-ready but not adopted or integrated. Root and Sol reviewed
the exact production patch, including unchanged TT bound refinement and immutable
backend selection. Interior evaluation reuses its existing correction indices;
it does not add repeated hashing as a performance confound. Discriminating tests
cover poisoned root/interior/qsearch values, retained unequal pawn/non-pawn terms,
excluded minor learning, other-backend controls, TT boundaries and existing
transactional model replacement/private reusable SMP contexts.

All six checks pass: focused v3, short engine v3, race engine v3, full short v3,
and official Counter raw/transition/engine-boundary oracles at v1 and v3. The
initial race invocation used CGO_ENABLED=0 and was rejected before any tests;
that operational attempt is preserved/excluded. The argument-only correction
to CGO_ENABLED=1 passes the same race suite. Normal builds retain CGO_ENABLED=0.

Exact [build/test receipt](2026-09-13-counter-correction-artifacts/candidate/build-test-receipt.json):
`cbc3acbdbf414a73e116b88fa0e29e130be793708cb302d32d69e3c4b6c9fc47`.
The adjacent archive preserves the relative production/regression patches,
commands and complete validation logs. The earlier absolute-path/executable-mode
test patch is retained only on WSL; normalizing its mode/path changed no source
bytes or binaries.

Production patch: `3318c4b94306f1640d3beb803833bf2661b1edc6ebb8875e0127bbaf474b4af5`.
Regression patch: `4dd199ed546edc63301cf4d0f28eb7ca2b5529a91e8815dd47c3f2c443ded220`.
Base binary: `695c7aed5d15c4303d3d32f8cdc58c207b3705768e9e9d462970aa3ed1abc2df`.
Candidate binary: `a3e7a53ac5a9960348781c48d4c8469180fff88fe426b08adcc97f33be2742a7`.

Both instrument-free binaries use Go 1.25.5, GOAMD64=v3 and identical build
flags against the frozen immediate base. Later main commits add documentation
and the unselected standalone Rodent package only; they do not change these
playing inputs. WSL bundle:
`/home/ehrli/repos/ngn-counter-minor-off-20260913/output/counter-minor-off-20260913`.

## Released game gate — execution record

Independent Sol review recommended release after the frozen hash-only check
passed all 15 input artifacts and both binaries. Root read the entire driver,
manifest, patches and validation evidence. The [held manifest](2026-09-13-counter-correction-artifacts/gate/manifest-held-v3.json)
is preserved. The [released manifest](2026-09-13-counter-correction-artifacts/gate/manifest-released-v1.json)
changes only `release_state`, SHA-256
`d9cbf7056d8dda5a2002fb12555379756fa2764bfaa58ecae2107380c6640192`.
Driver SHA-256: `8aec98ffcd90001d1fd67c5a048dd4ef3e7b0aa68fcfd6d4ffcc46705f2688f2`.

WSL command:

```sh
/usr/bin/python3 /home/ehrli/repos/ngn-counter-policy-gate-20260913/driver-held-v2.py /home/ehrli/repos/ngn-counter-policy-gate-20260913/manifest-released-v1.json
```

Run root: `/home/ehrli/repos/ngn-counter-policy-gate-20260913/run-001`.
The first SSH attempt was rejected locally before connection; the authorized
retry started this single run, not a second game attempt. The 10-second prelaunch
sample observed 0.095 non-owned CPU cores. A/A started at
`2026-09-13T19:46:34.952141+00:00`, supervisor group/root PID 2992376, with the
declared four physical CPU representatives and 8 GiB process-tree limit.
The A/A phase completed and passed as detailed below. The candidate completed
its games and legal/terminal replay, then the operational audit failed as
recorded below. No canonical decision or terminal JSON was produced. Do not
restart or rewrite the frozen audit result.

Other owned engines/builds/tests remained quiet throughout the timed gate.
Independent post-run cleanup found no surviving owned engines or match-stage
processes. The computation hold can now be released for the next isolated
Rodent verification step; it does not depend on accepting this candidate.

### Completed A/A preflight

All 100 games / 50 pairs and 15,214 legally replayed plies pass the legal and
terminal audit. Engine A scored 26W/52D/22L (52%); pentanomial counts are
`[2,13,17,15,3]`. These are engine-role results, not White/Black results.
The controller admitted the candidate phase only after the predeclared paired
bootstrap interval included zero; its exact interval will be archived from the
terminal decision, not substituted with fastchess's normal-approximation interval.

The supervisor completed with return code 0, no termination/error/survivors,
826.818 seconds elapsed and 5,426,592 KiB peak sampled RSS against the 8 GiB
limit. The operational audit passes, with no canonical errors or warning lines,
four normal process exits per role and all expected option sequences. The
non-owned-CPU monitor did not reject the run. Independent Sol review agrees.

Final A/A PGN SHA-256:
`0f01072ea3250885c3729514aa4fc7ef2d4d0fd2f08e885b56062db4613ca159`.
Final EPD SHA-256:
`b0ade6e1b9828f3b50064965f94646fcba6ccf34ee0217ad9122306c22940972`.
The candidate phase began automatically after these checks. No partial candidate
score was used as a verdict.

## Terminal disposition — not adopted

The fixed candidate sample completed in 54:32 match time: **98W/207D/95L**,
201.5 points / 400 games, pentanomial `[9,44,87,55,5]`. These are raw completed
game counts, not an accepted Elo result. Legal and terminal replay passes all
400 games, 200 pairs and 61,755 plies.

Candidate supervisor completed with both return codes 0, 3,275.460767 seconds
elapsed, peak sampled RSS 5,872,380 KiB, no timeout/memory/orphan/monitor errors
and no survivors. The process witness completed with fastchess return code 0,
no violations and exactly four observed instances per role. Independent Sol
review's one-shot executable check found no owned engines or match-stage process.

The frozen trace auditor then rejected the shared role-A crash log because its
first two lines both say `=== NGN Crash Handler Initialized ===`. The complete
eight-line file contains exactly four initialization notices and four correct
log-path notices, with no panic, stack trace or extra content. Two processes
interleaved their separate writes as `init, init, path, path`; role B has four
ordinary adjacent pairs. The auditor assumes every process's two writes remain
adjacent, which is not guaranteed for a shared log. This is a false-positive
crash-log classification, not evidence of an engine crash.

The original operational audit remains **FAILED**, and the controller exited 1
before producing `decision.json`, `terminal.json` or the declared bootstrap
decision. There is no canonical ADOPT verdict. Root therefore shelves the
candidate: no production patch, no accepted Elo, no game replay or extension.
The separate forensic closure does not replace the original failed audit or
claim the remaining full trace checks passed. Correct this narrow auditor bug
with discriminating tests before using it for a future game gate.

Candidate PGN SHA-256:
`8ae5c595dbe80e221556a899e40486ac21b013a4f9d53ab822fadecc2c5ef822`.
Final EPD SHA-256:
`023c63ba55350b5384e25d9f12bcf0cd6af3868b5cbdb8ec2ba51d8caba53c28`.
The separate [forensic closure](2026-09-13-counter-correction-artifacts/gate/forensic-closure.json)
is SHA-256 `f7147fd3861b4af724ee8bfed58c489328ac00d371650165f06dc0c134b9f63a`.
All raw artifacts remain in the original WSL run directory. The accepted
playing source is unchanged; next work is the existing Rodent network path.

## Future-run auditor repair

The repository auditor now validates exactly the expected initialization and
exact-path record counts without assuming adjacency. Unknown content, wrong
paths, count drift and invalid timestamps remain fatal. Logs establish record
counts, not per-process attribution; the process witness remains authoritative.
The regression fixture reproduces the preserved role-A interleaving byte-for-byte.
All 27 existing candidate-match Python tests pass on WSL with the two-file
overlay on base `203830a`: `python3 -m unittest discover -s tests -p test_*.py -v`,
3.196 seconds. The [test log](2026-09-13-counter-correction-artifacts/gate/future-auditor-tests.txt)
has SHA-256 `9d3af608814e46bdc2b31696b6bff5f301d1c33c4bf198cc4071d3860eed03e7`.
No historical run inputs or results were changed, and no old audit or games
were rerun. A future gate must freeze the repaired auditor and run its ordinary
A/A preflight before candidate games.
