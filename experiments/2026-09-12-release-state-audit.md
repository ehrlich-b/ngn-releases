# Release-state audit after NNUE/SMP integration

Date: 2026-09-12. This was a read-only audit of the clean WSL checkout and its
preserved result trees. It ran no engine, search, build or match command and did
not reinterpret failed or partial games.

## Source and deployment boundary

- WSL `/home/ehrli/repos/ngn` was clean at `314e1ba`; its latest behavior
  commit is `a51233b` (tree `622b3f8da8a1af9abfd7da3ee8942593373a0624`).
  The two later commits change `README.md` only.
- `a51233b` includes real Lazy SMP, Counter 5.5 evaluation, the accepted AVX2
  feature-update path and explicit startup evaluator selection.
- The installed `build/ngn` is still the classical binary produced from
  `53e4d1b`, SHA-256
  `0c067627404ab67a5f4a57fef63be024fd84e9248423286ef3dfc103041d266e`.
  The installed Counter launcher is absent. This runtime boundary is distinct
  from the clean source checkout, which has advanced to `314e1ba`.
- The prepared but uninstalled package is
  `/home/ehrli/repos/ngn/build/releases/ngn-a51233b-counter55-v1`. Its binary is
  `437981db2753e2e9b46fc7185754da63969f284792ed0a6b5182dc192565f910`,
  Counter model is `3488baed71f4d432d028e05b65e2ff13f8c9afe7f91974d1a3c34a0691d6670c`,
  and manifest is `0c52f487eb7ac4037bf39a6ddc8152f97312dbbf79ad3cf87b4fee364b5f20b6`.
  Preparation passed without changing the installed runtime. Deployment
  remains held. The preserved coherent-install runbook expected the source
  checkout itself at `53e4d1b`; that precondition is now false, so its command
  sequence must not be run unchanged even after the game gates close.
- The completed classical result remains 2884 [2834,2934] over 400 games at
  120+1/c8, with 55,349 legally replayed plies and 400 terminal outcomes passing.
  See [the original acceptance record](2026-09-05-2800-result.md).

## Accepted internal release gates

WSL artifact paths below are relative to `/home/ehrli/repos/` unless shown as
absolute paths.

All width tests used the original HCE SMP binary at 30+0.3, normalized-Elo
SPRT [0,20], alpha=beta=0.05 and a 400-game cap. Each stopped at its first
upper-bound crossing and passed the independent chess, trace and operational
audits.

| Test | W-D-L | Games | Penta 0..4 | First H1 crossing | Terminal evidence |
|---|---:|---:|---:|---:|---|
| HCE2 vs HCE1 | 59-83-20 | 162 | [0,11,29,32,9] | pair 81, LLR 3.0225 | `ngn-sprt-width-contract/output/m4c-hce2v1-sprt-20260906-attempt1/terminal.json` |
| HCE4 vs HCE1 | 58-51-5 | 114 | [0,0,16,29,12] | pair 57, LLR 2.9877 | `ngn-sprt-width-contract/output/m4c-hce4v1-sprt-20260906-attempt1/terminal.json` |
| HCE8 vs HCE1 | 63-36-3 | 102 | [0,1,8,23,19] | pair 51, LLR 3.0246 | `ngn-m4c-sprt-runner/output/m4c-sprt-root-review-20260906/terminal-review-v1.json` |
| Counter1 vs HCE1 | 89-40-37 | 166 | [5,4,27,28,19] | pair 83, LLR 2.9592 | `ngn-counter55-sprt-capability/output/counter55-vs-hce-sprt-root-terminal-review-20260906-v1/root-review.json` |
| Counter8 vs HCE8 | 66-27-5 | 98 | [0,1,6,22,20] | pair 49, LLR 2.9924 | `ngn-nnue8-hce8-runner/output/counter8-hce8-longclock-20260906-attempt1/terminal.json` |

The last row is the required longer-clock 60+0.6 interaction check. It confirms
the combined configuration under that test; it is not an absolute-rating result.

## External comparison state

The fixed cells use 50 independently selected 16-ply openings with reversed
colors, 100 games, one game at a time, 30+0.3, 128 MiB Hash per engine and no
adjudication. Counter results were recovered by offline audits after exact,
independently proved terminal-PV warning classifications; games were not rerun.

| Cell | Accepted state | NGN W-D-L | Score | Evidence |
|---|---|---:|---:|---|
| Counter 5.5, width 1 | accepted recovered terminal | 1-25-74 | 0.135 | phase acceptance `1a8700e89bc134ae64e37e7e9bbc6d05f3ff39cae17912b7dcc54212a0c9b819` |
| Counter 5.5, width 8 | accepted recovered terminal | 1-49-50 | 0.255 | phase acceptance `6b0d2d22e88a96ed799145fc2f82149781034a5e2a7cd78646fceec53b4eac95` |
| Rodent V1.1 Anand/testers, width 1 | accepted terminal | 1-15-84 | 0.085 | `external-fixed-comparisons-20260908-attempt4/.../terminal.json`, SHA-bound inventory `6444196d888b44483b39c93a6ef5f407e21c38ef0359fbb419edb0a993fd2e0c` |
| Rodent V1.1 Anand/testers, width 8 | unavailable | -- | -- | exact full-mask transcript exposes no `Threads` option; substitution forbidden |

These results support the provisional range discussed in `README.md`, but do
not establish a transferable CCRL rating or a world-best claim.

## Failed boundary and remaining cells

The sequential queue stopped in
`external-sequential-queue-20260908-attempt5` at Rodent V1.2 width 1. Fifty
games completed (NGN 0-7-43), but game 52 produced a strict Fastchess warning:
Rodent's PV contained `g2g1` where a promotion suffix was required. The match
stage returned 1, the supervisor found no survivors, and the queue recorded
`FAILED` with failure SHA-256
`399e970ee3e0f27c93a3f19bd87e486786122b23f1da546147a3bf18a46bde5b`.
The partial score is neither accepted nor a basis for changing the test.

Consequently these predeclared cells remain incomplete:

- Rodent V1.2 non-Tal/testers width 1, fresh complete 30+0.3 cell;
- Rodent V1.2 non-Tal/testers width 8 at 30+0.3;
- Rodent V1.2 width 1 and width 8 focused 60+0.6 cells.

The exact failure should first be replayed as a reporting-boundary diagnostic.
Any compatibility rule must be fixed from mechanism evidence, preserve the
original failed run, leave best-move/game behavior untouched, add positive and
negative fixtures, and precede a fresh full schedule. If that cannot be proved,
the exact V1.2 artifact is unavailable under the release contract.

## Reproducibility debt

The external runner that produced these cells is not in `main`. WSL worktree
`/home/ehrli/repos/ngn-external-opponents-v1` remains dirty on `ac00e33`, with
roughly 2,000 changed lines plus additive external-runner modules. Frozen V17
records 54 source entries and four passing delta tests, but its full-suite
comparison retains 27 inherited failures. Treat the frozen source and result
trees as evidence, not as a ready commit. The next source task is to port it
onto current `main`, repair the full test baseline, verify frozen-result import,
and only then commit the runner.

The ordinary baseline command also needs a clean-worktree caveat. From the main
WSL checkout, `go test -short ./... -count=1` descended into ignored Go source
snapshots under `output/next-stage-20260905` and failed while trying to compile
incomplete CounterGo and Zahak copies. The NGN packages reached by that run
passed. From clean worktree `/home/ehrli/repos/ngn-evaluator-startup` at the
same behavior commit `a51233b`, both of these passed:

```text
go test -short -p 2 ./... -count=1
go test -short -race -p 2 ./... -count=1
```

Future baseline instructions should require a clean worktree rather than
treating ignored research trees as packages in the project module.

## Release decision order

1. Close the Rodent V1.2 reporting boundary without result-driven policy drift.
2. Complete the remaining fixed external cells and one root aggregate, or
   explicitly hold the release if the frozen gate cannot be satisfied.
3. Regenerate or explicitly amend the deployment transition for the current
   clean `314e1ba` source checkout; the old `53e4d1b` precondition is stale.
4. Rehearse the resulting immutable package and, under an exclusive idle
   window, run its coherent install/rollback/reinstall sequence while verifying
   both HCE and Counter launchers.
5. Resume strength work only from the deployed, reproducible base.

At audit time WSL had no NGN, Fastchess, Counter or Rodent match process. Other
unrelated WSL workloads were left untouched.
