# Independent clock-tail audit — 2026-09-19

Read-only independent parse of the already completed external Counter checkpoint.
The scan took **4.871 seconds** on the existing WSL host; no engine, match, build,
or test was launched. The hopper was not changed. Source examined locally was
`f75b578`; the match's own binary/protocol provenance is recorded in
[`2026-09-13-rodent-external-counter-checkpoint.md`](../2026-09-13-rodent-external-counter-checkpoint.md),
run-003. This audit did not independently rehash the remote log or binaries.

Input: `/home/ehrli/repos/ngn-external-counter-checkpoint-gate-20260913/run-003/candidate/fastchess.log`.
The parser was sent to the existing host using the read-only wrapper
`ssh -o ConnectTimeout=10 ehrli@192.168.4.108 'wsl -d Ubuntu -- python3 -'`.
[`clock_tail_audit.py`](clock_tail_audit.py) preserves the executed parser, with
formatting, documentation and an optional input-path argument added afterward.
It has not been rerun after preservation.

## Definition and checks

Each search is keyed by **(display name, fastchess thread)**. Its duration is the
log timestamp from inbound `go` to outbound `bestmove`; its tail is the duration
from the last non-bound `info depth` containing score, nodes and reported time to
that bestmove. These are sums over concurrent per-search durations, **not session
wall time**. Missing/duplicate starts, missing infos, pending searches, unparsed
depth infos, bounded infos and negative timestamps are counted. This parser does
not normalize midnight rollover: such a search would be reported as negative
and excluded. No negative timestamps occurred in this artifact.

There were zero duplicate/missing/pending searches, zero unparsed/bounded infos
and zero negative timestamps. All go directions were `<---`; all bestmove
directions were `--->`. Eight `stop` commands occurred **outside** active searches;
none of the included searches received one. Every bestmove equaled the first
move of its last reported PV for both engines.

## Observed results

| Role / subset | Searches | Summed duration, ms | Summed tail, ms | Weighted tail |
|---|---:|---:|---:|---:|
| NGN, all | 32,347 | 6,786,594.298 | 1,732,566.763 | 25.52925% |
| NGN, duration >=10ms | 31,410 | 6,784,618.058 | 1,732,556.281 | 25.53653% |
| NGN, duration >=100ms | 25,197 | 6,260,057.625 | 1,639,634.654 | 26.19201% |
| NGN, completed depth >=10 | 31,409 | 6,729,481.238 | 1,707,659.044 | 25.37579% |
| Counter 5.5, all | 32,335 | 6,695,092.547 | 893.759 | 0.01335% |

No search in either role lasted at least 1,000ms. NGN's median per-search tail
fraction was 17.83027% overall and 18.99135% for searches >=10ms; the corresponding
90th percentiles were 59.51247% and 60.01376%. The median difference between the
last info log timestamp minus go and the engine's reported elapsed time was
0.540ms for NGN and 0.603ms for Counter.

NGN had zero consecutive final-depth repeats. Counter had **3,572**; its final
reporting convention makes its near-zero tail **an invalid causal negative
control** for how much work it spends in unfinished iterations. The engines do
not publish comparable iteration boundaries at termination.

## Interpretation and limits

The NGN result is robust to removing short searches or shallow completed depths.
Its source only publishes new depth after a completed iteration
(`engine/search.go:958-967`). The soft-limit comment says not to start an
unaffordable iteration (`engine/time.go:442`), but the same soft-stop predicate
is sampled inside nodes (`engine/search.go:1355,1804`), and the boundary call
(`engine/search.go:686-691`) itself uses the rate-limited checker
(`engine/time.go:393-414`). Interrupted root results are correctly discarded
(`engine/search.go:871-879`).

**The tail is not measured waste, a reclaimable speedup, or an Elo estimate.**
It can retain useful TT/history and completed subtree work; logs do not identify
soft versus hard stops. Counter's reporting contrast must not be used to claim
zero corresponding work. This evidence motivates a bounded experiment separating
iteration admission from hard node cancellation while preserving safe unwind;
only real-clock games can establish whether that improves play. The older T1a
regression tested removing the projection, not this separation.
