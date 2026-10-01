# 2026-07-01 M4 — parallelize cmd/spsa (K perturbations/iter)

Revamp mill-item M4 (`2026-07-01-method-revamp.md`). `cmd/spsa` was fully
sequential: one iteration = one game-pair on a single (plus,minus) process pair,
blocking on real-clock games (~40s/iter at 10+0.1 → 12000 iters ≈ 5 days). That
cost is why the T8 SPSA round was "multi-day background."

## Change (`cmd/spsa/main.go`)

New `-pairs K` flag (default 1). Each iteration now draws K INDEPENDENT
perturbations and plays them CONCURRENTLY on K process pairs (2K engine
processes), then updates theta with the mean of the K `R·delta` gradient
estimates — the fishtest mini-batch: wall-clock drops ~K×, gradient variance drops
~1/K, the expected step direction is unchanged. `c` is common to a step so `(a/c)`
factors out of the mean.

Determinism preserved: all rng draws (perturbation signs + opening indices) happen
in the single main goroutine in `runCampaign` before dispatch; only the games race.
At **K=1 the rng stream and update are byte-identical to the old code** (sign draws
then one opening draw per iter), so `-pairs 1` = the original sequential SPSA.
`runCampaign`/`runSelfTest` were generalized to a `playBatch(plusV, minusV, opIdx)
[]float64` objective; `-selftest` maps the synthetic quadratic over the K
perturbations, exercising the same averaging path. Backward compatible.

## Evidence

- `-selftest -iters 5000 -pairs 1`: PASS, max normalized err 6.5% (reproduces the
  original convergence).
- `-selftest -iters 5000 -pairs 4`: PASS, max normalized err 6.6% — the averaged
  gradient converges; several per-param errors are tighter (lower variance).
- Real-games wiring smoke `-engine ngn -iters 2 -pairs 2 -tc 3+0.03`: header shows
  `2/iter`, each iter reports 4 games / 0 forfeits, both iters complete, exit 0, no
  deadlock. Concurrent play on distinct process pairs is clean.
- `go build ./...` clean; full short suite unaffected (no engine/shared change).

## Next

Unblocks **T8** (SPSA round 1): after M1 (make SingularMargin/NullMoveR live — they
are currently emitted by the SPSA setoption loop but hardcoded in the engine, so
2/11 dims are dead), run `-iters ≥3000 -pairs 4 -tc 5+0.05` on the box; the RESULT
vector is gated on GAMES (gauntlet/SPRT), never this driver's own plus_pct. On the
9800X3D (8c/16t) K=4 = 8 processes fits comfortably; watch the forfeit counter as
the real-clock-under-load health signal.
