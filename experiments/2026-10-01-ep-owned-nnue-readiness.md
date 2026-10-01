# Legal-EP repetition repair: owned WDL25 NNUE readiness

Date: 2026-10-01 UTC

Branch: `task/ep-repetition-nnue-readiness-20261001`

Reviewed repair source: `042846d537732d075bb182e63d11f498f6eb7f1f`

Scope: test and evidence only; no production behavior change

## Question and answer

**Question.** With the released `ngn-k4-768-v1` WDL25 evaluator at
`K4EvalScale=60`, does the accepted legal-en-passant repetition repair preserve
the incremental evaluator and its lifecycle across the proved played histories,
copies, null descendants, search children and complete unwind? Does finite
search restore the caller's full position and game occurrence map?

**Answer for the bounded fixtures: yes.** The exact released model passed a
non-skipped integration test. At every committed real transition and every
inverse step, the retained K4 context had the exact engine board/side and the
same raw and scale-60 search evaluation as a separately constructed evaluator
reset from the whole current position. Raw EP, `PositionTag`, raw ordinary key,
`Hash()`, FEN, clock, board and the complete occurrence map restored exactly.
All tested worker contexts returned to depth zero. No new defect was observed.

This is correctness and lifecycle evidence, not a strength, timing or broader
search-quality result.

## Immutable inputs

- Source HEAD: `042846d537732d075bb182e63d11f498f6eb7f1f`
- Model:
  `/home/ehrli/nnue-owned-k4-20260920/night-20260927/runs/wdl25-e10.nnue`
- Model SHA-256:
  `1ec8fc1737ddfdd5f8b6ff0b4e26778085fb563f7c5e82c1fc5d3ea454b6ec29`
- Strictly loaded backend: `ngn-k4-768-v1`; engine scale: `60`
- Reused Stockfish fixture receipt:
  [`2026-10-01-ep-repetition-correctness/stockfish-probe.json`](2026-10-01-ep-repetition-correctness/stockfish-probe.json),
  SHA-256
  `897f7ebd0c3adc602839d7782673ddbecdd7c1bc72c58dc73fb424f8884b283e`
- Its pinned Stockfish 18 executable SHA-256 remains
  `6b087694916228c905a5e14db74cca8c7e5643602226af1fa5d42353c455b9f9`.
  The oracle and executable were not rerun or modified.

The exact-net test independently reads and hashes the model before strict load,
checks the loader metadata's file hash and architecture, and checks the selected
worker identity and scale. See `model-receipt.txt` and
`oracle-reuse-receipt.txt` in the evidence directory.

## Retained regression

`engine/enpassant_owned_nnue_integration_test.go` adds the environment-gated
`TestOwnedK4EnPassantRepetitionReadiness`. Ordinary suites skip it unless
`NGN_EP_OWNED_NNUE` is explicit; the first validation command supplied the
exact pinned path and did not skip.

The test covers:

1. The four existing legal nine-ply histories for pinned/unrelated-check EP,
   both colors, including a true third occurrence, plus the two existing
   legal-EP controls through their actual EP captures.
2. Every forward evaluator push and reverse pop in those histories. Each frame
   compares the incremental K4 raw score and scale-60 search score to a distinct
   freshly reset evaluator using the same immutable model.
3. The immutable Stockfish legal-root and perft-2 expectations for all six
   fixtures at real-push, EP-FEN and no-EP stages, plus the pinned Polyglot
   baseline values already separated from ordinary repetition identity.
4. `Position.Copy` equality and occurrence-map independence at every
   post-double-push root.
5. For all six fixtures: double push, null, a legal quiet non-pawn child, child
   pop/unmake, null pop/unmake and double-push pop/game-unmake. Exact raw EP,
   tag, raw key, computed key, FEN, board, clock and map snapshots are checked
   after each inverse boundary.
6. Depth-2 owned-evaluator search on a frozen four-root subset: illegal/pinned
   white and black roots and legal-EP white and black roots. An observer checks
   every committed incremental frame against full-position model evaluation;
   an independent full-refresh search produces the same comparable search
   result. The two pinned searches observed 33 committed frames each and the
   two legal-EP searches 27 each. Both caller positions/maps were byte-for-value
   restored, evaluator depth was zero, and root evaluation still matched a
   separately refreshed evaluator afterward.

K4's network input is board plus side-to-move; EP/tag/key/map correctness is
therefore asserted directly on the engine `Position`, while context position
and evaluation parity independently assert evaluator maintenance.

## Commands and results

All commands ran in WSL with `/usr/local/go/bin/go` 1.25.5, task-private cache,
`GOMAXPROCS=1` and `GOFLAGS=-p=1`. Exact command, start/finish, stdout, stderr
and numeric exit receipts are under
`experiments/2026-10-01-ep-owned-nnue-readiness/`.

| Gate | UTC interval | Result |
|---|---|---|
| Exact-net focused test, non-skipped | 02:52:18–02:52:24 | PASS, exit 0; package `0.366s` |
| `go test -short ./engine -count=1` | 02:52:35–02:52:47 | PASS, exit 0; `11.915s` |
| `go test -short -race ./engine -count=1` | 02:52:54–02:54:35 | PASS, exit 0; `94.985s` |
| `go test -short ./... -count=1` | 02:54:43–02:55:15 | PASS, exit 0; engine `12.819s`, all packages green |
| `gofmt` | before validation | PASS, exit 0 |

The exact-net command was:

```text
NGN_EP_OWNED_NNUE=/home/ehrli/nnue-owned-k4-20260920/night-20260927/runs/wdl25-e10.nnue GOCACHE=/home/ehrli/ngn-personal-correctness-20261001/.gocache-ep-repair GOMAXPROCS=1 GOFLAGS=-p=1 /usr/local/go/bin/go test -short ./engine -run ^TestOwnedK4EnPassantRepetitionReadiness -count=1 -v
```

No shared-state, concurrency, UCI or harness production path changed, so the
conditional full `-race ./...` gate did not apply.

## Resource and scope receipts

`environment.txt` records Linux/amd64 WSL, source/branch, Go binary hash,
service cgroup, CPU affinity `0,2`, `cpu.max=50000 100000`, nice `10`,
`memory.max=4294967296`, memory node `0`, serial Go and the private cache.

Only the optional integration test, this report and its evidence directory are
new in the checkout. The released model, historical EP proof/oracle, production
engine, owned benchmark/match files and other checkouts were not modified. No
games, benchmarks, downloads, training, tuning, external communication or
publication occurred.

## Limits and handoff

- This proves only the six curated EP fixtures and four small depth-2 searches.
  It is not a random campaign, exhaustive chess proof, performance measurement
  or Elo claim.
- The accepted production repair remains exactly the reviewed source commit;
  this task adds no engine behavior change.
- The environment-gated test requires this exact model hash. A missing or
  different file fails/skips according to whether the path was supplied; it is
  never silently substituted.
- The native `.git` mount was respected. After the native turn completed,
  the authorized controller independently reviewed and committed only this
  scoped test/report/evidence on the isolated task branch.

## Independent operator review

Review tightened `assertOwnedEnPassantRestored` to compare the raw position
snapshot before calling `Hash`; a cache refresh can no longer conceal a raw-key
or metadata restoration failure. No production source changed. The original
native test/report bytes and manifest are preserved by deterministic gzip and
`native-input-encodings.json`; original receipt hashes still verify against
those archived inputs.

After that test-only change, the controller ran the actual-net focused race
probe and all three required short suites again. Every command supplied the
exact released model path, so the optional integration test ran in these suites
as well. All passed with empty stderr. `review-*` receipts and
`operator-review.json` bind the final source SHA and actual process affinity,
cgroup quota, nice and memory limit.

The first operator verifier stopped before any Go check because it expected a
cpuset controller file that this WSL cgroup does not expose. Its failure log is
retained. The corrected verifier checks actual scheduler affinity `[0,2]` plus
the existing `cpu.max`, nice and memory limits; no resource or host setting was
changed. This setup failure is not an engine-test result.

The final local commit/bundle is recorded in external
`night-nnue-completion.json`. No owner files, default branches or published
artifacts changed. This evidence supplements the owner's separate benchmark
work and makes no new playing-strength claim.
