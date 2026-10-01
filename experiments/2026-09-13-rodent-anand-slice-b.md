# Rodent V1.1 Anand incremental context — verification

Status: **accepted inactive compatibility**, after run-002 passed all required
checks. Slice A's loader and portable full refresh are accepted in `11ca24e`;
the exact three Slice B production/test files are now integrated.
No engine/UCI selection, bundled network, deployment or strength claim.

The Counter correction gate has terminated without adoption, and independent
cleanup found no surviving owned computation. Slice B runtime verification is
released below; no engine integration is accepted before it passes.

## Implemented scope

Sol implemented one worker-private `SearchContext` with immutable shared model,
separate position/accumulator frames, initial capacity 128 and explicit doubling.
Move pushes validate the semantic delta and expected post-position before
publishing state. Null pushes copy accumulator bits and flip side; pop restores
the parent frame. Crossing the own-king D/E mirror boundary refreshes only the
moving side's perspective. Other updates preserve exact int16 modular arithmetic.

Focused tests cover quiet moves, double pushes, captures, both en-passant
colors, four castles, all promotion pieces with/without capture, both mirror
directions, poisoned destination frames, model/parent immutability, transactional
errors, reset/growth, independent contexts and multi-ply sibling restoration.
Root and independent Sol review accepted this source scope. The completed
runtime evidence is recorded below; source review alone was not acceptance.

## Frozen oracle inputs

Source directory:
`/home/ehrli/rodent-v1.1-anand-sliceb-oracle-20260913/source-v2`.
The [integration contract](2026-09-13-rodent-anand-integration-contract.md)
defines the exact existing network, release configuration and arithmetic.

- `transition-fixtures.json`:
  `f9e26de0ed7a9663011f1fe8844ee0d8beb7a1bea4a2e4da50805fdbbf9579d4`.
- `rodent_sliceb_oracle_test.go`:
  `44fc5f8a0189d300d172641e6fc6364d84feac32023c946f783a5e17dbbd862b`.
- `release_raw_oracle_driver_v1.py`:
  `0ee2190d8d2ee1499748fcc10b0117c1e531342963c58f2aace701aca5480c9f`.
- `run_release_raw_oracle_v1.sh`:
  `ee0dd9874142cbb85d841d117140c45c725e933ee33f72e85ab915e392481d80`.

There are 22 sequences, 53 explicit steps and 75 root/checkpoint records.
Repeated null pushes are represented by one aggregate fixture step. They are
not 75 independent positions, and the tagged harness's null/pop/growth stack
is test machinery, not Rodent's production search-stack lifecycle.

The tagged harness calls Rodent's native pending-update helper and compares
all 1,024 incremental lanes with tagged full refresh at each checkpoint. It
emits little-endian int16 whole/half accumulator digests and raw scores. The
exact testers binary separately supplies two matching full-refresh raw passes
at the checkpoint FENs. It exposes no accumulator lanes.

NGN's opt-in consumer replays its actual context transitions, compares every
checkpoint lane with NGN refresh and tagged digests, and compares raw scores with
the exact release. Final static expectations use the independent test formula,
including the release's asymmetric material term. Independent review requested
per-push board/side/depth/lane-copy checks inside the 128-null growth sequence,
so intermediate NGN lifecycle errors cannot hide behind the final checkpoint.
That bounded source-only test strengthening is complete in consumer v3.

Final source-only bundle, beneath the context worktree's
`output/rodenteval-slice-b-source-20260913`:

- `slice-b-source-with-consumer-v3.patch`:
  `b30957db211b4fe627cc66da694aac46fd24b71a9d3e783c51c7f66ce9ec000f`.
- `source-only-receipt-v3.json`:
  `097fec9b206b83fcb0d7dc9763865069ebc55701ef26840a8f45b798edb65bf9`.
- Final consumer source:
  `a2c3389306f7ce55e7388cd3cb6e9ca837395488e0004a8ea3245d105ef63220`.

The combined patch includes an isolated draft at this report's path. At later
integration preserve exact production/test bytes and merge documentation
deliberately; do not overwrite this live record blindly with the draft.

## Historical execution preparation and stop rule

After the Counter verdict, pin the actual accepted playing base and final
context/consumer bytes. Run bounded serial tagged/release oracle generation,
NGN focused tests, configured parity (no skipped required oracle), package race
and full short suite on WSL. Inspect process cleanup and exact artifact hashes.
Stop on any unexplained lane or raw drift; do not fit a scale to the mismatch.

Held execution preparation lived at
`/home/ehrli/rodent-v1.1-anand-sliceb-gate-20260913`. Root
review found v1 lacked the tagged test's working directory and did not overlay
the uncommitted context files onto the clean accepted-base checkout. Driver v2
fixes both, records exact source assembly and excludes compiler caches from the
evidence inventory. Independent Sol review accepted the corrected stage wiring.
Runner v3 additionally rejects skipped required unit/oracle tests (including
subtests), without rejecting the baseline suite's intentional short-mode skips
or claiming all baseline tests executed.

The [held manifest](2026-09-13-rodent-anand-artifacts/incremental-preflight/manifest-held-v3.json)
and adjacent driver/runner are archived byte-exactly:

- Manifest v3: `50c86813ce56910c9edc151e65d25e54e2d1a50a50656a1c3e597039a9b0d609`.
- Driver v2: `7602932f9297008292717ffdbed3702bbeb38a354cca68b49b851c31c65d5d2c`.
- Runner v3: `5c5e5cc6be76eb0b010ab6ee2b4fa954038f001df467ac29b225b939abdb4725`.

Static syntax/hash checks passed with 19 fixed inputs before runtime release.
The source root/commit/tree were then pending. Formatting, source freeze and
runtime results supersede that held state below. Do not launch any held manifest
or mutate archived inputs. The three bounded serial stages are tagged mechanism, exact-release raw,
and NGN checks; their time caps are 10, 1 and 30 minutes, respectively.

Only passing runtime verification permits integrating this inactive context.
Named engine/UCI integration follows separately, retaining worker ownership,
backend switching and current search policy. Measure realistic portable cost
before admitting a SIMD speed candidate; a separately frozen real-clock gate
must establish strength. None of these compatibility milestones proves 3300.

## Runtime release

The final [release manifest](2026-09-13-rodent-anand-artifacts/incremental-verification/manifest-release-v1.json)
is SHA-256 `7c89f5f61e9069901df92cc5efcca97347c472ebc04e1e6f56cbeda3bb8ba74a`.
It binds accepted base `0a78fd861b9f8081fb10f350f531fa8f0bd53249`, tree
`fbd9ab73663c01a42fc35f543441d71ef74500a2`, and 21 fixed inputs. WSL main
fast-forwarded cleanly to that exact base. No Counter candidate code was applied.

`gofmt` changed only consumer-field alignment; production and focused-test bytes
are unchanged. Root reviewed the whitespace-only diff. The adjacent formatting
receipt is `4ad70387e78ba1aac75a525bad5410cede486a0ea97887e6f3b7e0f7c4fc5f8b`.
Final overlay SHA-256s:

- `context.go`: `f10c6ef9d7413878398ab080aa2895b966095e708120061d6f4d8b5169174ce3`.
- `context_test.go`: `d46db3ab881e599836f74bd1574a6a33c840fcf26be7db13931b22a40a70af6e`.
- `context_oracle_test.go`: `3235b2643d0aa9c4d2da0b136c78602d0528106366c94aaaf09efc7acbc7a9da`.

Final code/tests-only patch, excluding the isolated draft report:
`116593a42cd094aeb6d88cffafa5151ce1a6fae3ffacbbd818a487e10334422f`.
The raw source-only v3 artifacts remain preserved. Root released the reviewed
serial driver with this manifest; Sol owns execution and reports completion
or failure without active progress polling. Any unexplained mismatch stops the
run rather than changing evaluation arithmetic to fit the oracle.

## Completed verification and acceptance

Run-001 passed tagged and release oracles, and its ten unit tests passed, but
the runner falsely counted child `=== RUN` lines as duplicate parent tests.
It stopped before NGN oracle/race/full-short checks. The original failure and
green unit log are preserved; no engine arithmetic changed to address it.

Runner v4 matches complete parent RUN/PASS lines. Its regression check accepts
the saved ten parents and rejects missing, duplicate, child-only and required-skip
cases. Root reviewed this narrow fix and released one correctness rerun with
[manifest v2](2026-09-13-rodent-anand-artifacts/incremental-verification/manifest-release-v2.json),
SHA-256 `dfa502129b851be543e81aa12c984535cfa84f877ccd58083c9c7665d242f969`.
Engine sources, network, oracle inputs and resource bounds were unchanged.

WSL `run-002` completed successfully:

- Tagged native-update/full-refresh mechanism: 75 checkpoints, all lane digests
  and scores matched; 9.874 seconds.
- Exact release: 57 distinct checkpoint FENs, two identical raw-score passes;
  0.058 seconds. No claim of observed release accumulator lanes.
- NGN: all ten focused parent tests and all 22 transition-oracle sequences
  passed without required skips; package race and full short suite passed;
  38.259 seconds. The suite's ordinary short-mode exclusions remain exclusions.
- All three supervisors returned zero, with no timeout, memory violation,
  orphan, monitor error or surviving owned process.

The [check summary](2026-09-13-rodent-anand-artifacts/incremental-verification/check-summary.json)
records exact commands; adjacent logs, source assembly, oracles and cleanup
receipts retain the evidence. The terminal receipt SHA-256 is
`3d0460d8a142b6bc65e05a6bc47bc3dd0bb4c903b42ea55458358c6f7d3451b8`.
Its canonicalized manifest hash differs from the source JSON's byte hash by
serialization, not runtime inputs. Integrated file hashes match the three frozen
overlays above. No bundled model, default selection or playing-policy change.

Next is Slice C: named engine/UCI selection and worker/search lifecycle parity,
then realistic portable speed measurement and a separate strength gate. Slice B
does not establish a rating gain or completion of the 3300 goal.
