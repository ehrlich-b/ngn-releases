# Singular-only confirmation on accepted T17+T8

**Completed H0; rejected.** `r0905singulart8` finished794 games at paired
−12.26 [−27.56,+2.99], zero errors/flags. Accepted T8 remains deployed.
Frozen before games, the
[original D policy](2026-09-05-singular-only-confirmation.md)
is rebuilt on accepted T8 commit `5cf301a870f6cd9218d8085166993f7ad5364aee`.
T8 passed its completed734-game H1 at paired+38.02 [+20.77,+55.45] and is now
deployed on Mac/WSL/Windows. The old qcap-based D candidate was never launched;
its conditional controller stopped on T8 H1. No old games are pooled.

## Candidate scope and verified artifacts

Only `engine/search.go` differs from accepted T8: the same8 additions/33
deletions remove ordinary positive check/passed-pawn extensions and use
consistent scout child depth with a nonnegative floor. Singular extensions,
all14 T8 defaults/T17, evaluation and other policies remain unchanged. No
diagnostic counters or additional heuristic are bundled. This is a speculative
search interaction test, not an automatic correctness keep.

Isolated source: `output/singular-t8-candidate-20260905/src`. Root compared every
tracked Go/module file against accepted main and reviewed the exact normalized
diff. The source archive's old git HEAD is1387fd6 with the tested T8 working-tree
overlay; the actual accepted source content equals5cf301a. Hashes below identify
the candidate independently of that archival metadata.

- Normalized diff `output/singular-t8-candidate-20260905/normalized-t8-D.patch`:
  `a6da52bb6c6f5a855c4f344a39fc201c035c62ccf9b72985d66df15bc8b40508`.
- Mac `build/ngn_20260905_singulart8`:
  `b14c69d51a0574fdc82c1a5ec282eeaa7f927bd371d5939bf4bf08ff4b9d1ca7`.
- Windows `build/ngn_20260905_singulart8.exe`:
  `e5df3c210dfbc3f104204801b4ebfdbda2e536996ea0ca227f1f4ec9ea4730c7`.
- Linux `build/ngn_20260905_singulart8_linux`:
  `9b07553fffb56dc4d4be2efa9b2992b3a2f22512c5d4fa64ab2fa2ff4fd0b538`.
- Accepted Windows base `ngn_20260905_t8qcap.exe`:
  `43b2a2c93d9a773b0a08d8017d611edb7b39d63efe071cb7366c1ac4c11f92fc`.

Go1.26.2, Mac arm64 and Windows/Linux amd64/v3 with CGO disabled. Full short,
race and vet pass. The independent SF oracle ran2293 positions,48926 depth-2
root divides and128 internal search roots without mismatch. All14 T8 defaults
and the EP/castling/fifty-move-mate UCI smoke pass. Root read the logs and verified
the hashes after copying the isolated builds into the root build directory.

All three canonical fixed-depth roots and23 frozen full-history400k-node roots
repeat exact nodes, score, full PV and bestmove in fresh processes. Root checked
all26 commands/histories against the original frozen diagnostic input and the
raw repeated values. This establishes repeatability on the new base, not identity
with the obsolete qcap D outputs or a fresh strength result. No new scoring or
candidate selection was performed. Raw logs, metadata and repeats remain in
`output/singular-t8-candidate-20260905/`.

## New-base preflight: r0905t8baseaa

Native Windows Ryzen9800X3D, default affinity,10+0.1, concurrency8,
lowpower=false. Use the accepted T8 binary on both sides. Frozen harness
`sprt_20260904.exe` SHA-256
`63ef86d9b986fdb2c260782149af259e278845d89595d010df3970e6698cc9ed`;
5000-line opening book `sprt_openings.txt` SHA-256
`974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222`.

Fixed **200 games/100 pairs**, with SPRT early stopping disabled. Require a
completed successful log, zero operational errors/flag-outs and a paired95%
interval containing zero. Failure holds candidate games for investigation;
there is no automatic control replication or extension.

```text
BOXSPRT_BIN=sprt_20260904.exe scripts/boxsprt.sh launch r0905t8baseaa -- -new '.\ngn_20260905_t8qcap.exe' -base '.\ngn_20260905_t8qcap.exe' -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 -3 -elo1 3 -alpha 0.05 -beta 0.05 -maxgames 200 -mingames 201 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
```

The earlier qcap A/A is not reused after changing the accepted base. No competing
Windows/WSL engine CPU work may overlap this control or the following match.

## Candidate gate: r0905singulart8

Retain the original D rule: paired **SPRT[0,+10]**, alpha/beta.05,
minimum200 games and maximum1600. Same machine, clocks, concurrency, affinity,
harness, book and adjudication as the fresh A/A. **Accept only completed H1
with zero operational errors and zero flags on either side.** H0 rejects;
capped inconclusive shelves. No extension, parameter changes, provisional keep
on sign, or pooling with other variants. No playing-source/default change before
the completed verdict.

```text
BOXSPRT_BIN=sprt_20260904.exe scripts/boxsprt.sh launch r0905singulart8 -- -new '.\ngn_20260905_singulart8.exe' -base '.\ngn_20260905_t8qcap.exe' -tc 10+0.1 -concurrency 8 -lowpower=false -openings sprt_openings.txt -elo0 0 -elo1 10 -alpha 0.05 -beta 0.05 -maxgames 1600 -mingames 200 -resignscore 900 -resignplies 5 -drawscore 10 -drawplies 10 -drawminplies 80
```

Launch requires fresh completed A/A acceptance, unchanged local/remote frozen
identities and installed T8 base, idle native chess workers, an unused run name,
and passing native candidate UCI smoke. A reviewed bounded controller may perform
this handoff after those gates; it must stop on any failed prerequisite and
must not retry an ambiguous launch. It collects the final candidate result for
root review, never deploys/commits and never extends or restarts a match merely
because observation timed out. Controller arming and hashes are recorded below
before it is authorized to launch the successor.

An accepted relative gain still requires the separate
[external2800 confirmation](2026-09-05-2800-pin-plan.md).

## Preflight launch and reviewed controller

Root reverified base/harness/book hashes and idle workers after deployment.
The staged singular-T8 native smoke also passed all14 defaults and the three
protocol regressions while idle. The exact fixed A/A command launched at
04:04:54 local September5; root observed one versioned harness and16 T8 workers,
with completed games flowing. No speculative engine is installed.

Root reviewed and independently ran the new controller's six offline integration
tests, covering the successful handoff, a biased/incomplete control, observation
failure, changed local/remote identities, busy workers, smoke failure, duplicate
claiming and ambiguous launch failure. Strict fakes verify the full frozen
command and harness selector. Fixtures go through the real completion auditor.
The control's `minimum200` and disabled `stop_minimum201` are distinct: a valid
fixed A/A must not be rejected merely for crossing a deliberately disabled SPRT
boundary while its paired interval contains zero. A coherent low-variance fixture
exercises that case. The candidate's original stop rule is unchanged.

The controller is authorized for the already declared A/A-to-candidate handoff
after all gates pass. Frozen artifacts under `output/recovery-2026-09-04/`:

- `collect_and_run_singular_t8.py`:
  `22f6ac53e565a537b0ceaf808717f5b3454e7954fa1edbc635d4ba422389af62`.
- `audit_singular_t8_sprt.py`:
  `b0c388e694f30cbdda69eda7d6334a67d04cce9168cae38cd8cf134719627a10`.
- `singular-t8-games-inputs.json` (327 local files, five remote identities):
  `3c22d25f014430857d8f62cb2aae587bef57764a16a234d1aa68e6501866d472`.
- `test_collect_and_run_singular_t8.py`:
  `5ef1c2f1d390d5bc13afaea3fe360f84b90c0806629322af76dcede312448ce4`.
- Native smoke script:
  `951591fcd0fd1f9f4a0580f96407b7839ba00339c26ed9693349bc914b48ef4c`.

The reviewed controller is now armed, waiting on the live A/A. State is
`singular-t8-games-controller.json`; it uses an exclusive claim and
bounded polling. It never deploys or restarts/extends games. Root validation log:
`singular-t8-controller-root-validation.txt`. Older controllers and audits retain
their original bytes and completed states.

## Completed A/A and candidate launch

`r0905t8baseaa` completed200 games in12m42s:49W/108D/43L,
penta3/27/40/21/9. Paired+10.426 [−22.908,+43.954], zero flags/errors,
single final `DONE_EXIT_0`. The interval contains zero, so the predeclared
fixed-control rule **passes**. The harness's generic INCONCLUSIVE footer is
expected because early SPRT stopping was disabled; this is not a failed control.
Root independently recomputed the interval and reconciled the final counts/hash.
Raw SHA-256: `00f20f569623b85310cc12541fd91e0681b51f94a8656a9ea900f2efb32cd9d7`.
Control verdict: `output/recovery-2026-09-04/r0905t8baseaa-root-control.json`.

After fresh local/remote identity, installed-base, idle-worker, unused-name and
native UCI checks, the controller launched the exact frozen candidate command
at04:18:24 local September5. Root observed one expected versioned harness,
eight singular-T8 workers and eight accepted-T8 workers, with games flowing.
The candidate is not installed. No additional Windows/WSL engine work may run
alongside it. The controller collects completion for root review without any
automatic promotion, extension or restart.

## Completed rejection and root review

The match completed in50m21s with186W/394D/214L, penta21/97/181/85/13
over397 pairs, single terminal `DONE_EXIT_0`, and idle workers. Root reviewed
the full log, reconciled counts and independently recomputed the paired estimate
and interval. First printed H0 crossing was G786 at pLLR−3.02; already assigned
games drained to794, leaving final pLLR−2.85009. The final value's movement
inside the boundary does not undo the completed stopping decision.

The predeclared outcome is **REJECT_H0**. No singular-only patch is installed,
and the match will not be extended or pooled. Raw SHA-256:
`aa34a4679d55fe6b1407b26ae2b2ba1860e54315435ecdfad6ca507c4ff6c690`.
Root verdict: `output/recovery-2026-09-04/r0905singulart8-root-verdict.json`.

The external rating handoff selected already accepted T8. Its startup preflight
stopped before any rating launch on a WSL ps terminal-size warning. Root fixed
that in a new verifier version; the original collector, scripts and failure
record remain intact. See the [pin manifest](2026-09-05-2800-pin-plan.md) for
the selected release, passing final startup and launch record.
