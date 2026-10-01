# C1 iteration-admission preflight

**Verdict: shelved before games.** The exact planned admission/abort split moved
11 of 12 representative stops to the new-depth boundary, but the remaining
search ran an admitted iteration all the way to the 2.955 s hard ceiling and
discarded 2.571 s after its last completed result. The candidate increased total
time across the frozen sequence from 7.839 s to 12.317 s (+57.1%). That is the
material hard-abort shift the plan required us to resolve before launch; the
known weakness of a 1x last-iteration forecast explains it but does not make it
safe or productive. No constants rescue, game gate or source adoption follows.

## Exact candidate

Base `5f33622` gained one unthrottled `ShouldStartIteration(completedDepth)` call
at each new-depth boundary. It retained the existing soft allocation, composed
stability/effort factor and strict 1x prior-iteration projection. After admission,
Tournament calls in aspiration, root move, main search and qsearch observed only
hard/emergency bounds. Fixed depth, fixed nodes, infinite, fixed time, external
stop, hard/emergency formulas and published-result guards were unchanged.

Deterministic tests passed the prescribed 799/800/801 ms strict boundary,
depth-one exception, zero-duration fallback, unthrottled admission, recursive
soft immunity, scheduled hard stop, independent emergency stop, first-cause
latching and NewIteration ordering. The complete engine short suite passed in
7.039 s. The exact unadopted change, including its tests, is preserved as
`candidate.patch` (SHA-256
`fd3feadc29827570f1a0bf60f93a3861802ea18818fb8dccd682ad7a3ed7a982`).

## Same frozen warm diagnostic

The C0 probe was rerun unchanged with exact Rodent V1.1 Anand, Threads 1,
Hash 128 MiB, Linux amd64.v3 and 10+0.1 over three passes of four retained game
prefixes. It used one persistent searcher so TT/history remained warm.

| Metric | Baseline C0 | C1 candidate |
|---|---:|---:|
| Soft admission | 0 / 12 | 11 / 12 |
| Soft active abort | 12 / 12 | 0 / 12 |
| Hard deadline | 0 / 12 | 1 / 12 |
| Total elapsed | 7.839050 s | 12.316675 s |
| Post-completion tail | 2.017880 s | 2.571027 s |
| Aggregate tail fraction | 25.74% | 20.87% |

The candidate did prove the mechanism: ordinary soft tails became zero because
the next iteration was never entered. But the bad cell is decisive. On
`accepted_game1_ply48`, pass 2, a 101.329 ms completed iteration admitted its
successor; that successor ran to 2.955135 s, searched 4,165,516 nodes, and was
discarded at the unchanged hard deadline. This is not recovered work. It also
shows why the earlier last-cost growth evidence warned against treating one
completed duration as a dependable next-cost forecast.

The candidate generally completed one additional depth, with a few volatile
larger changes due to the warm TT/history trajectory. Nominal depth and removal
of soft tails were explicitly insufficient acceptance criteria. A game launch
would therefore spend hundreds of games on a candidate whose required safety
preflight already exposed material hard-abort waste and substantially greater
clock consumption.

Exact observations, source hashes and aggregate arithmetic are in `result.json`.
The accepted C0 observer remains; only the C1 policy change is shelved. Work now
advances to the V1.2 output and update SIMD program.
