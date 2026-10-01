# ProbCut live-MoveStack contract repair

Status: **ACCEPTED AS A PROVED CORRECTNESS REPAIR**

ProbCut now records its actual capture in the live `MoveStack[ply]` slot before
either quiescence or the reduced main search can consume ancestry. A clean
failing-before run reproduced stale ancestry in the grandchild, qsearch-stop and
sibling paths; the one-line lifecycle repair made every focused and baseline
gate pass. This is a correctness result, not an Elo claim.

## Scope

Immediate accepted base: `f1f6b7e` (`Fix singular verification ProbCut contract`)

Validation WSL worktree:
`/home/ehrli/repos/ngn-sol-probcut-f2-20260919`

Production diff: after a legal ProbCut capture is made and its
evaluator frame is pushed, assign that actual move to `info.MoveStack[ply]`
before entering quiescence or the reduced main search. This mirrors the normal
real-move lifecycle. It deliberately adds no slot restoration, history-policy
change, pruning change, or evaluator change.

Validated hashes, identical in the local checkout and isolated WSL worktree:

```text
d26651a27ad0c2f355c03817881c613e42a892c0e36d49f18d8023f3d42b63eb  engine/search.go
491ee5d223c84e917834b18b16a8453dea394aed51b1fdc08c47095bea287257  engine/search_probcut_contract_test.go
```

## Regression contract

The three new tests are designed so clearing the slot, retaining the stale move,
or updating it only after child search all fail:

1. `TestProbcutMoveStackFeedsGrandchild` seeds `MoveStack[0]` with `e1d1`,
   proves the root ProbCut capture is `a1a7`, reaches a reduced-search
   grandchild, and requires its live two-ply ancestor context to be `a1a7`.
   The observer requests stop independently of the value read, then checks the
   stopped sentinel and complete board/history/frame/evaluator unwind.
2. `TestProbcutMoveStackIsLiveBeforeQuiescence` requests cancellation as the
   evaluator publishes the root capture. The assignment occurs at the ordinary
   lifecycle boundary immediately afterward; quiescence must observe the stop,
   unwind, and leave evidence that the traversed capture—not the stale seed—was
   the active slot.
3. `TestProbcutMoveStackTracksCompletedSiblings` gives the root two legal,
   non-losing captures. Because the evaluator observer fires just before each
   assignment, the next ProbCut sibling must see the prior sibling in the slot;
   the first normal root move must see the final ProbCut sibling. The search then
   completes normally and must restore the board, predecessor history and frame
   depths.

The local review explicitly accounted for `PushMove` invoking the transition
observer before `alphaBetaPV` assigns `LastMovePlayed` and `MoveStack`. Do not
move the production assignment before `mustPushMadeMove`; that would diverge
from the normal real-move transaction and could publish a move whose evaluator
push failed.

## Failing-before and passing-after evidence

All commands ran in the isolated WSL worktree on the authorized shared host.
The hopper remained live and outside every process group. Go was 1.25.5 on
linux/amd64; focused runs used `GOAMD64=v3`, one CPU and one Go worker.

The first attempted failing-before run correctly exposed all three stale-slot
paths but also exposed a test-only assertion mistake: the stopped-search throwaway
score is numerically zero, so a normally completed zero score cannot be rejected
unless `info.Stopped` is true. That assertion was corrected before the production
patch. The clean rerun against byte-unchanged accepted source then failed only on
the intended stale ancestry:

```text
TestProbcutMoveStackFeedsGrandchild:
  read root move e1d1, want active capture a1a7
TestProbcutMoveStackIsLiveBeforeQuiescence:
  root move e1d1, want traversed capture a1a7
TestProbcutMoveStackTracksCompletedSiblings:
  completed siblings left MoveStack[0]=b2b3
FAIL (2.6 seconds outer command time)
```

Exact focused command:

```sh
timeout --signal=TERM 120s taskset -c 6 nice -n 10 \
  env CGO_ENABLED=0 GOAMD64=v3 GOMAXPROCS=1 GOFLAGS=-mod=readonly \
  /usr/local/go/bin/go -C /home/ehrli/repos/ngn-sol-probcut-f2-20260919 \
  test -p 1 -vet=off -run TestProbcutMoveStack -count=1 -timeout=60s -v ./engine
```

After copying only the one-line production candidate, the same three tests all
passed in 2.7 seconds. The combined F1+F2 `Probcut` group then passed all seven
tests plus both descendant-control subtests in 0.6 seconds:

```sh
timeout --signal=TERM 120s taskset -c 6 nice -n 10 \
  env CGO_ENABLED=0 GOAMD64=v3 GOMAXPROCS=1 GOFLAGS=-mod=readonly \
  /usr/local/go/bin/go -C /home/ehrli/repos/ngn-sol-probcut-f2-20260919 \
  test -p 1 -vet=off -run Probcut -count=1 -timeout=60s -v ./engine
```

## Baseline validation

The normal low-concurrency ladder passed:

```sh
timeout --signal=TERM 600s taskset -c 6,7 nice -n 10 \
  env GOAMD64=v3 GOMAXPROCS=2 GOFLAGS=-mod=readonly \
  /usr/local/go/bin/go -C /home/ehrli/repos/ngn-sol-probcut-f2-20260919 \
  test -short -p 2 -count=1 -timeout=8m ./engine

PASS  github.com/ehrlich-b/ngn/engine  7.351s

timeout --signal=TERM 900s taskset -c 6,7 nice -n 10 \
  env GOAMD64=v3 GOMAXPROCS=2 GOFLAGS=-mod=readonly \
  /usr/local/go/bin/go -C /home/ehrli/repos/ngn-sol-probcut-f2-20260919 \
  test -short -race -p 2 -count=1 -timeout=12m ./engine

PASS  github.com/ehrlich-b/ngn/engine  37.085s

timeout --signal=TERM 900s taskset -c 6,7 nice -n 10 \
  env GOAMD64=v3 GOMAXPROCS=2 GOFLAGS=-mod=readonly \
  /usr/local/go/bin/go -C /home/ehrli/repos/ngn-sol-probcut-f2-20260919 \
  test -short -p 2 -count=1 -timeout=12m ./...

PASS  all packages; engine 7.735s
```

No match, engine installation, release action or hopper change occurred. The
accepted result closes the second proved ProbCut contract defect. Next work is
the paired C0 clock-mechanism and V0 Rodent V1.2 cost preflight from the Sol plan.
