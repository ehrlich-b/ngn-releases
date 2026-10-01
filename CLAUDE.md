# CLAUDE.md

Agent guide for NGN. `TODO.md` is the live project state and ranked work queue. This file is the operating contract for agents.

## First Step

Before doing any substantial work:

1. Read `TODO.md`.
2. Read this file.
3. Run `git status --short`.
4. Preserve unrelated user/agent edits.
5. Pick the highest-ranked unblocked item from `TODO.md`.

If an older doc conflicts with `TODO.md`, follow `TODO.md` and treat the older doc as historical evidence only.

## Commands

```bash
make build
make test
make test-short
go test -short ./engine -count=1
go test -short -race ./engine -count=1
go test -short ./... -count=1
go test -short -race ./... -count=1
```

`make test` must fail when tests fail. Do not treat failure output as green.
Run `go test ./...` from a clean worktree: ignored research snapshots under
`output/` can contain partial third-party Go source trees and are not packages
of this module.

## Active Policy

- **September 20–23 user direction supersedes the September 13 NNUE deferral:**
  pursue an NGN-owned NNUE toward at least 3000 and allow justified training,
  data and pipeline work. The old deferral remains historical context. See
  `experiments/2026-09-23-owned-nnue-next-step-diagnosis.md` for the latest
  evidence and next gate. Do not infer absolute rating from relative matches.
- **Explicit user restriction, September5:** never run NGN on this Mac. It is a battery-powered user computer. Run engines, matches, search tests, benchmarks, tuning and heavy chess validation on the authorized Windows/WSL box only. Background scheduling, efficiency cores or reduced concurrency do not make local execution acceptable. Local editing and lightweight read-only inspection are fine. Do not restart any stopped Mac controller or allow a remote collector to launch local engine validation.
- **September29 user reminder:** use the WSL machine's GPU for NNUE training. Keep builds, tests, and playing-strength matches on WSL as above.

- The at-least-2800 target without NNUE is verified on the fixed historical calibration; see `experiments/2026-09-05-2800-result.md`.
- September5 user direction: the completed2800 goal is succeeded by verified NNUE and real multicore search, toward the strongest Go-based engine. NNUE is now allowed in principle. All implementation, training, builds, profiles, engines and matches run on WSL only. See experiments/2026-09-05-next-stage-roadmap.md. The Mac is a remote terminal; no local project computation.
- September5 planning direction: research NNUE and multicore before implementation; use the staged design in experiments/2026-09-05-nnue-smp/report-source.md. Sol performs major implementation; root coordinates, reviews contracts and controls acceptance. Start with the minimal verified pipeline and one-thread ownership, then named external backends and helper searches.
- Correctness and measurement integrity come before speculative Elo work.
- Optimize for Elo per confirmation hour. Default confirmation budget is 1-2 hours per change.
- One behavior change at a time, candidate vs immediate base.
- Do not stack changes unless the experiment is explicitly about an interaction.
- Do not keep speculative search/eval changes on positive sign at an inconclusive cap.
- Batch-certification path (2026-07-01, the only sanctioned alternative to shelving at cap): a capped-but-positive result (point est ≥ +1, LLR > 0, no regression signal) may enter main as a PROVISIONAL keep; after ≤4 provisionals or 2 weeks, one powered batch-vs-pre-batch-base real-clock SPRT [0,+6] certifies all of them or triggers a bisect.
- Pre-reset (before 2026-06-28) game verdicts are historical evidence only; they cannot close a lane or gate new work.
- Proxies (FMC/ebfprobe/fixed-nodes/ACPL/MSE) may REJECT only pure ordering/EBF-mechanism changes, and every proxy rejection records a reopen condition. Anything touching eval values, time, or TC behavior gets a real-clock games gate.
- Time-management and TC-sensitive changes are gated by real-clock games only.
- A SPSA run is evidence only if its params are verified live in code, it reaches ≥20k games, and the verdict is games on the converged vector — never mid-run plus_pct or param movement.
- Do not commit speculative changes before their verdict. Correctness/test/doc reset commits are allowed after tests pass.
- Do not create new sprawling memory/TODO narratives. Put detailed run records in `experiments/`; update `TODO.md` only when live state, policy, or ranked queue changes.

## Decision Table

| Change class | Required evidence | If inconclusive |
|---|---|---|
| Provable correctness fix | Mechanism proof, regression test, short non-regression check if behavior changes | Keep only if the correctness claim is proved |
| Node-identical refactor | Exact node/score identity plus tests | Keep if identity is proved and the benefit is real |
| Pure speed change | Behavior identity plus repeatable wall-time/NPS gain | Shelve if speed gain is not repeatable |
| Search/eval heuristic | Completed predeclared paired game test against immediate base | Shelve; do not keep on sign |
| Tune | Holdout/proxy improvement plus paired games | Shelve or redesign |
| Lane closure | Multiple falsification attempts using the right instrument | Mark uncertain if evidence is proxy-only |

There is no "correctness-flavored" class.

## Work Loop

Use this loop unless the user gives a narrower instruction:

1. Prove the bug or missing behavior from code before editing.
2. Make the smallest scoped patch.
3. Add a regression test that would have failed before the patch when feasible.
4. Run the narrow test, then the baseline tests required by `TODO.md`.
5. If games are required, write a manifest before launching the run.
6. Do not begin a second behavior change while a verdict for the first is pending.
7. End with a clean account of what changed, what passed, and what remains.

When uncertain, prefer a small correctness fix with a direct regression test over a speculative Elo idea.

## Current Reset Facts

The 2026-06-28 reset patch fixed known P0 search/UCI issues that were corrupting measurement:

- stopped child searches no longer update parent TT/history/best-score state;
- qsearch handles in-check nodes at the depth cap;
- UCI `searching` is atomic and the stop race repro passes;
- `Hash` UCI option resizes the TT and persists through `ucinewgame`;
- `SearchFixed` sets `RootDepth` and uses root game make/unmake;
- rejected aux correction-history code is not active.

Required baseline before new work:

```bash
go test -short ./engine -count=1
go test -short -race ./engine -count=1
go test -short ./... -count=1
```

Use `go test -short -race ./... -count=1` when touching UCI, harness, shared state, or concurrency.

## Documentation Contract

- `TODO.md`: live state, active policy, ranked queue, compact facts needed to continue.
- `CLAUDE.md`: this operating contract.
- `experiments/`: manifests, run logs, result summaries, and negative results.
- `docs/`: stable architecture/spec notes, not live run instructions.
- Review files: evidence and checklists, not policy.

When updating `TODO.md`, keep it self-building but not narrative-heavy. Add enough context for the next agent to continue; move detailed evidence to `experiments/`.

## Harness Guardrails

- Do not edit `scripts/cloudsprt.sh` while using it for a verdict.
- Any cloud harness edit requires a separate validation cycle and A/A run.
- Never verdict a mid-run or killed SPRT.
- Never pool cloud and local games in one verdict.
- Never mix machine classes or instance types inside one pooled run.
- Every cloud session ends with `scripts/cloudsprt.sh ps` showing no unnamed live workers.
- If AWS denies an action, stop; do not broaden IAM or work around credentials.

## Run Records

Every game run must record:

- base and candidate commit/patch;
- binary SHA-256s;
- exact command and environment;
- TC, concurrency, machine, OS, Go version, GOARCH/GOAMD64;
- opening/corpus checksum;
- game count and penta or W/D/L;
- flag-outs, crashes, illegal moves, no-move results;
- predeclared decision rule and verdict.

Record it with `experiments/RUN_RECORD_TEMPLATE.md`, and pin corpus checksums via `experiments/corpus_manifest.md`. A real-clock concurrent self-play verdict requires an A/A preflight (same binary vs itself at the exact TC/concurrency/affinity/machine) before the candidate run.

Default local opening file, when used:

```text
output/sprt_openings.txt
sha256 974e4b5ab871a9e106d0c766bfa39fc83676222337fb7702f61782e2ac5b3222
5000 lines
```

## Reanchor

When asked to reanchor, read `README.md`, `TODO.md`, and this file. Treat `TODO.md` as the live authority and older review files as historical evidence.
