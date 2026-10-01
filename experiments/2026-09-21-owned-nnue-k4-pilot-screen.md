# Owned K4 NNUE pilot screen

Status: **HELD**. The 1M-position pilot candidate failed the predeclared
same-code HCE strength gate. Do not launch the 20M run from this recipe without
the written diagnosis required by the plan.

## Frozen candidate and runner

- Engine source commit: `81600cd1c03da2cf1c4bd3e9e8d3ff7ce5ef2b54`
- Linux amd64-v3 engine SHA-256:
  `3c3257bf14b43883ef7c88608fdb06eefe2379928119154ea6d38571c05e61b8`
- Candidate: update 1024, 16,777,216 presentations over 1,000,000 distinct
  training inputs
- Candidate model SHA-256:
  `034559653a83a7e64d4407badff334f66e3eae6a3a1f33147e3516b3c88c3e69`
- Frozen match-manifest SHA-256:
  `b85d7957a587cc6559ca478fb6d3886023268cb81fc162ac1a7940f7350148bb`
- Review-subject SHA-256:
  `14954a4c9c1f669101e143a838a8eddc873253e10572d6dca56650b610103596`
- Opening PGN SHA-256:
  `f16e6f6500b87cc68a08e7ce6e03cd00f74f3ee5be1b40151df4d4693f8bd2f1`

The candidate had already passed strict Bullet/raw/integer/Go parity, payload
round-trip, arithmetic-range, and checkpoint-eligibility gates. Checkpoint 1024
had validation integer MSE `0.011393196739740128`, down from
`0.02695743374931675` at initialization. This establishes that the frozen
trainer/export/inference path learned its declared target and transported it
consistently; it does not establish useful chess strength.

## Match

The screen ran on 2026-09-21 from 08:44 to 09:02 America/New_York. It used the
predeclared 40 complete games / 20 reversed-color pairs at 10+0.1, Threads 1,
Hash 128 MiB, Move Overhead 100 ms, concurrency 1, no book, tablebases,
resignation, adjudication, or recovery. The outer stage used separate physical
cores 12 and 14; fastchess pinned the game to CPU 12. The Lean hopper remained
available on the shared host.

Result from the HCE perspective: 37 wins, 3 draws, 0 losses, 38.5/40 (96.25%).
Therefore the K4 pilot scored 0 wins, 3 draws, 37 losses, 1.5/40 (**3.75%**).
HCE scored 19.0 points as White and 19.5 as Black. All decisive games ended in
natural mate; the draws were two fifty-move outcomes and one insufficient-
material outcome.

Both independent gates passed:

- complete/legal chess audit: PASS
- process/lifecycle/operational audit: PASS

No engine crash, hidden recovery, incomplete game, adjudication shortcut, or
color/opening imbalance explains the result. The plan requires a hold below
35%, so the 20M expansion was not launched.

## Evidence identities

The authoritative run directory is outside Git at:

`/home/ehrli/nnue-owned-k4-20260920/integration-v1/k4-pilot-screen-40-v1/run-001`

| Artifact | SHA-256 |
| --- | --- |
| `terminal.json` | `476d95a825a2b22619abb8d600f31ea78a5d30f4ad3014b62435810f2a93da85` |
| `audit.json` | `dc50913d8bc9bcb4fc91702c1968e4e25c62a181678fd8893b5cea3c9d385c2a` |
| `audit-operational.json` | `07598c7681a65a9d629fa7bf92b481566b1e1e177e3233401843a8ffc2f2cd60` |
| `games.pgn` | `301d492f65550f8f1b31fc9893b3fe2b9b9248459370480e6afb73b6f3d5fe4e` |
| `config.json` | `30be1c23f48df0edd9cce77116ead495c5ca81324f7cbf938089f3d9e9744a28` |
| final `STATE.json` | `a7572cc49f6b5c539f968cd6b556ce91a8b02c4b9c91cfeb47a598b7522adead` |

The runner's sealed final-files digest is
`eb8df9dc781f787441ac156436f4245bb2fb651a7b1190d9e3a250bd37d19efd`.

## Diagnosis boundary and next gate

This screen proves a large practical-strength deficit for this pilot. It does
not by itself prove the cause. Evidence presently weighs against transport,
crash, color, and harness explanations. The leading nonexclusive explanations
are insufficient unique coverage for a 2,372,360-parameter deployed network and
the approximately fourfold evaluator cost previously measured against HCE.
One million unique positions is less than one training example per deployed
parameter, while the architecture's published Rodent reference used orders of
magnitude more data. Cost alone is unlikely to explain the whole match deficit.

Before any 20M expenditure, use the frozen calibration data to test, in this
order:

1. prediction/teacher sign, scale, rank correlation, and error by side, output
   head, king bucket, phase, and material;
2. K4 versus HCE and teacher move agreement under equal-node and equal-time
   budgets, separating evaluator quality from search-speed cost;
3. representative losing-game blunders against the frozen teacher, including
   whether failure concentrates in sparse heads/buckets or unseen tactical
   states.

Only an evidence-backed diagnosis and one frozen recipe change may authorize a
new pilot. The failed screen must remain part of the record.
