# Rodent portable profile: output kernel first

The accepted opt-in backend (`637043a`) has a concentrated runtime bottleneck:
the portable output calculation accounts for **46.11%** of sampled CPU, and
incremental updates another **33.69%**. The first admitted speed experiment is
therefore an exact-integer AVX2 output kernel only, retaining scalar fallback.
This is a diagnostic finding, not a speedup or Elo result.

## Completed diagnostic

One WSL process used the existing six Counter-profile FENs in unchanged order,
Hash128, Threads1 and 400,000 nodes per cold search. Five reported iterations
per fixture gave 30 measured searches, with six additional one-operation Go
benchmark calibrations. Every search passed legal-best-move, root restoration,
context unwind and incremental/full-refresh root-score assertions.

The process completed in 44.59 seconds, peak sampled RSS 307 MiB, with no timeout,
memory violation, orphan, monitor error or survivors. Per-fixture reported search
times were 1.106–1.418 seconds; they are descriptive and not directly comparable
to Counter because changing evaluation changes the search tree.

CPU attribution uses `scope=search` labels (44.25 of 44.29 sampled seconds):

| Mechanism | Cumulative sampled CPU |
| --- | ---: |
| Output calculation, including clipping/squaring | 46.11% |
| Incremental updates | 33.69% |
| King-half perspective refresh | 1.58% |

The roughly 135 MB/op cold benchmark allocation is dominated by lazy 128 MiB TT
allocation at first search admission, not per-node evaluator churn. The global
allocation profile additionally includes untimed construction; it does not measure
retained memory or search-only allocation.

Exact inputs, commands, raw stdout, cleanup and profile source listings are in
the [compact archive](2026-09-13-rodent-anand-artifacts/portable-profile/results.json).
Binary and raw profile files remain on WSL with hashes recorded. The test-only
probe is archived as inert `.go.txt`, not installed in the engine package.

## Next experiment

Sol is implementing only the output kernel in isolation. It must preserve int16
clipping and int32 modular sums, side ordering, truncating divisions and final
release scaling bit-for-bit, including extreme synthetic lanes/weights and
overflow witnesses. Existing-network oracle and search identity must pass before
a separately frozen paired speed gate. No new training, default selection,
search-policy changes or game launch is admitted by this profile.
