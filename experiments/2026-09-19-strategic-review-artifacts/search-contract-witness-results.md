# Search-contract research witnesses (2026-09-19)

Two deliberately defect-asserting witnesses reproduced current search-state
violations. These are research artifacts, **not fixes or passing regression
guards**. The `.go.txt` file is inert unless installed through the accompanying
Go overlay. No production source, live hopper, or engine match was changed.

## Result

```text
=== RUN   TestReviewSingularProbcutExcludedMoveWitness
    review_search_contract_witness_test.go:56: Excluded capture a1a7 was searched by ProbCut at excluded ply 0 and certified cutoff -200; nodes=0
--- PASS: TestReviewSingularProbcutExcludedMoveWitness (0.00s)
=== RUN   TestReviewProbcutMoveStackWitness
    review_search_contract_witness_test.go:69: ProbCut root capture a1a7 reached a grandchild while MoveStack[0]=e1d1; frames=3
--- PASS: TestReviewProbcutMoveStackWitness (0.00s)
PASS
ok  	github.com/ehrlich-b/ngn/engine	0.009s
```

Standard error was empty. The first test supplies a valid exact child TT result
to isolate the excluded-capture route; `nodes=0` reflects early-return node
accounting, not the absence of search transitions. Its evaluator observer
independently saw the forbidden child. The second stops through the observer
after the grandchild has read stale `prev2` and selected a move. Both verify
that evaluator state unwinds to depth zero.

These establish violations of intended contracts; neither estimates frequency
or Elo. The tests use zero-valued NGN-v1 tensors to remove evaluator variance,
not the current competitive Rodent network.

## Source contracts

- `engine/search.go:1420-1425` defines singular verification as search with one
  move excluded; TT cutoffs are guarded at line 1427 and the main move loop
  skips that move at line 1799. ProbCut's guard at line 1648 and capture loop
  at line 1663 omit both the singular guard and excluded-move check. A reduced
  verification at depth 5 or greater can therefore certify a cutoff using the
  very move whose absence it was supposed to test.
- ProbCut updates `LastMovePlayed` at line 1680 but does not set
  `MoveStack[ply]`. The ordinary move path sets both at lines 1892-1893 and the
  null path explicitly clears the slot at line 1610. At lines 1744-1749 a
  grandchild assumes `MoveStack[ply-2]` describes its live ancestor; ProbCut
  breaks this assumption and feeds stale follow-up-history context.

## Exact checkout and resource envelope

Local reviewed revision: `f75b578`. Existing WSL research checkout:
`/home/ehrli/repos/ngn-rodent-v12-slicec-20260919`, recorded base `f17114e` plus
its pre-existing Slice-C changes. A single compatibility check found the
following files byte-identical to local current source:

| File | SHA-256 |
| --- | --- |
| `engine/search.go` | `d20c51ecbc426ea4a13c23f7f2ec14e505197ab9cd1c398f38a3942786789054` |
| `engine/worker_evaluator.go` | `a9453da68ba4a2e449ae6b21c0028bb7a4c9f1d45cddb3a7bdaf00c368d1af80` |
| `engine/evaluator_selection_test.go` | `52b18f4c6a51db4856d36ab45d92c429cee7607933d6b8cd800ee314ccc32fd7` |
| `go.mod` | `93691f7adb3ed0ed7e0ebc88b5aa433707b38f7565373ce0853d6a572ed52d46` |

The run happened once, on WSL only, with a 90-second supervisor cap including
compilation, CPU affinity 6, nice priority 10, `GOMAXPROCS=1`, one Go build
worker, and a 2-GiB process-tree RSS cap. No retries or parameter sweep.

Supervisor receipt summary:

```text
stage: strategic-search-contract-witnesses
status: COMPLETE
command_returncode: 0
supervisor_returncode: 0
elapsed_seconds: 4.026784809073433
started_unix: 1789819975.1923902
ended_unix: 1789819979.219175
peak_process_count: 3
peak_rss_kib: 253200
valid_samples: 4
root_pid / pgid: 2625589
timeout / memory / orphan / termination events: none
survivors: none
```

Raw remote outputs and receipt remain under
`/tmp/ngn-search-contract-review-ykC2C4/` as `test.stdout`, `test.stderr`,
`test.time`, `test.samples.jsonl`, and `test.receipt.json`. Temporary files may
eventually be reclaimed by the host; the small source and result are retained
here intentionally.

## Exact launch

After copying only the two inert artifacts into the isolated remote directory:

```sh
ssh ehrli@192.168.4.108 'wsl -d Ubuntu -- python3 /home/ehrli/repos/ngn-rodent-v12-slicec-20260919/scripts/candidate-match/process_supervisor.py --label strategic-search-contract-witnesses --limit-seconds 90 --term-grace-seconds 2 --sample-interval-seconds 1 --memory-limit-kib 2097152 --cpu-list 6 --stdout /tmp/ngn-search-contract-review-ykC2C4/test.stdout --stderr /tmp/ngn-search-contract-review-ykC2C4/test.stderr --time-output /tmp/ngn-search-contract-review-ykC2C4/test.time --samples /tmp/ngn-search-contract-review-ykC2C4/test.samples.jsonl --receipt /tmp/ngn-search-contract-review-ykC2C4/test.receipt.json -- nice -n 10 env CGO_ENABLED=0 GOAMD64=v3 GOMAXPROCS=1 GOFLAGS=-mod=readonly /usr/local/go/bin/go -C /home/ehrli/repos/ngn-rodent-v12-slicec-20260919 test -p 1 -vet=off -overlay /tmp/ngn-search-contract-review-ykC2C4/search-contract-overlay.json.txt -run TestReview -count=1 -timeout=20s -v ./engine'
```

This command contains historical temporary paths and is a provenance record,
not an instruction to rerun automatically.
