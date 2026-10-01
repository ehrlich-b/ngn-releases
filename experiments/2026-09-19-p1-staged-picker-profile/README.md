# P1 staged-picker admission profile

This is the prospective admission contract for the deferred P1 staged move
picker. It was frozen before collecting a profile from the accepted H1 base.
The purpose is to decide whether whole-list preparation is still a material
search cost, not to use profiling as strength evidence.

## Frozen inputs

- Source commit: `383e75745dfffcc47bf1cdd60411d297f456563d`.
- Platform: authorized WSL Linux on the Ryzen 7 9800X3D, Go 1.25.5,
  `GOAMD64=v3`, `GOMAXPROCS=1`, engine Threads 1, Hash 128 MiB.
- Model: released Rodent V1.2 default network, 4,744,768 bytes, SHA-256
  `c35a1abc1b8c1cb1d5f4221454d494c1a6da1ed9088fd51ab27038bfa74b5053`.
- Workload: the six frozen V1.2 cold positions at 400,000 nodes per search,
  three repetitions each (18 completed searches). Reuse
  `../2026-09-19-v1-rodent-v12-output/rodent_v12_output_gate_test.go.txt`
  byte-for-byte; its SHA-256 is
  `4113bcb783807e77f143210e39ae20731b652db1009830f4cf37249df6e1a307`.
- Profile filter: `scope=search`. Setup and result validation remain outside
  the labeled region.

The run is invalid if the source/model/harness identity differs, any search
does not reach its node floor, any result or restored root is invalid, the
hopper is disturbed, or the filtered profile contains less than 5.00 seconds
of CPU samples.

## Primary decision rule

Read cumulative samples at the two main-search call sites in `alphaBetaPV`:

1. `GenerateMovesIntoBuffer(pos, frame.moveBuffer[:])` immediately before the
   ordinary move loop; and
2. `info.history.scoreMovesIntoBuffer(...)` immediately after that generation.

These are sequential, non-overlapping siblings. Their combined share is the
conservative whole-list-preparation opportunity. It deliberately excludes
root, ProbCut and quiescence generation, `materializeCaptureScores`, and
`selectNextMove`; those costs may be reported separately but cannot make the
primary gate pass.

- **ADMIT P1** when the combined share is at least 3.00% of filtered CPU.
- **SHELVE P1** when it is below 3.00%; advance directly to N1.

Sampling is reported at 10 ms resolution. The saved source-line listing and
full `pprof -top -nodefraction=0` output are authoritative; a rounded table in
the result receipt is not.

If admitted, freeze the one-candidate prototype and game gate before changing
production search. The intended first mechanism is TT-first staging: validate
and try a real TT move before ordinary whole-list generation, then fall back to
capture/quiet stages without duplicates. Deferred history reads make this a
search candidate, not a node-identical refactor.

## Result

The run is valid and **P1 is admitted**. All six fixtures completed three
400,000-node searches, stderr was empty, and the filtered profile contained
8.68 seconds of CPU samples. The ordinary `alphaBetaPV` generation call used
330 ms and its immediately following whole-list scoring call used 270 ms. Their
non-overlapping total is 600 ms, or **6.91%**, clearing the frozen 3.00% gate.

This does not revive the June TT-first prototype. Commit `6bf83e9` records that
its generate-everything fallback read history after the TT subtree and inflated
fixed-depth nodes by 85% on Kiwipete, 29% in the middlegame and 12% in the
endgame. The admitted candidate is the materially different reference-style
stage machine frozen in `prototype-plan-v1.md`, with an explicit node-inflation
falsifier before games.

Full artifacts remain under
`/home/ehrli/p1-staged-picker-profile-20260919/run-001` on the authorized WSL
host. Their identities and the decision calculation are in `result-v1.json`.

