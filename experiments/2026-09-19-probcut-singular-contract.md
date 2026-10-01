# Singular-verification ProbCut contract repair

Date: 2026-09-19

Verdict: **accepted as a proved correctness repair**. Singular verification no
longer permits root ProbCut to traverse the move being excluded from that
verification. Ordinary searches and descendants outside the excluded ply retain
ProbCut. This is not an Elo claim and no deployment or installation was made.

## Scope

Accepted base: `a7fe3b6062c8b11b38eda1a713b69adfdf177475`

Isolated WSL worktree:
`/home/ehrli/repos/ngn-sol-probcut-f1-20260919`

Production change: the ProbCut eligibility guard in `alphaBetaPV` now also
requires `!inSingular`. No pruning margins, depths, move filters, terminal
scores, history policy, time policy, evaluator code, or UCI behavior changed.

Regression coverage in `engine/search_probcut_contract_test.go` proves:

- the excluded capture cannot be traversed by ProbCut at the excluded ply;
- excluding a quiet move also disables the whole root ProbCut mechanism;
- ordinary search still reaches the deterministic ProbCut cutoff;
- a descendant outside `ExcludedPly` still reaches that cutoff;
- the ordinary-position TT entry is not overwritten by the restricted search;
- normal and forced-stop returns restore the board/hash snapshot,
  `LastMovePlayed`, repetition/frame depths, live ancestor state and evaluator
  depth.

Validated file hashes, identical on the Mac checkout and WSL worktree:

```text
69fe9d21199e9024246ab20fc57b78129de830bcd0ca54f070925920568af2dc  engine/search.go
692a024b80ceca0cb16fdf28c33ba9871ad35f9eaf368a1a5c50b6c7432396df  engine/search_probcut_contract_test.go
```

## Failing-before proof

The new regression was copied into the otherwise unmodified accepted base and
run before the production edit:

```sh
timeout --signal=TERM 90s taskset -c 6 nice -n 10 \
  env CGO_ENABLED=0 GOAMD64=v3 GOMAXPROCS=1 GOFLAGS=-mod=readonly \
  /usr/local/go/bin/go -C /home/ehrli/repos/ngn-sol-probcut-f1-20260919 \
  test -p 1 -vet=off -run TestSingularVerificationSkipsProbcutAtExcludedPly \
  -count=1 -timeout=30s -v ./engine
```

It failed in 7.3 seconds with the intended witness:

```text
search_probcut_contract_test.go:84: singular verification traversed excluded move a1a7 through ProbCut
FAIL
```

After the one-condition production edit, all four focused tests and both
control subtests passed in 3.2 seconds.

## WSL validation

All commands ran on the authorized shared WSL host with the Lean hopper left
running. The user's earlier two-minute restriction was explicitly limited to
the preceding planning session; these remained bounded, low-concurrency gates.

```text
go test -short -p 2 -count=1 -timeout=8m ./engine
PASS  github.com/ehrlich-b/ngn/engine  7.795s

go test -short -race -p 2 -count=1 -timeout=12m ./engine
PASS  github.com/ehrlich-b/ngn/engine  38.538s

go test -short -p 2 -count=1 -timeout=12m ./...
PASS  all packages; engine 7.283s
```

The commands used `GOAMD64=v3`, `GOMAXPROCS=2`, Go 1.25.5, CPU affinity 6-7,
nice priority 10 and an outer timeout. No configured-network test was claimed,
no match ran, and the installed binaries were untouched.

Next: repair the independently proved live `MoveStack[ply]` omission in the
ProbCut real-move path as a separate correctness commit and regression.
