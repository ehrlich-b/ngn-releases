# The 1x projection is not a spending-preserving forecast

One additional **existing-log-only** scan took **2.673 seconds**, with a
30-second parser alarm. No engine, match, build, or test was launched and the
hopper was untouched. [Executed reader](clock_iteration_growth_audit.py),
[exact output](clock-iteration-growth-result.json). Input is the same run-003
candidate log as the [clock-tail audit](clock-tail-audit.md).

## Filters declared before reading

- Completed growth uses three consecutive completed-depth reports. The ending
  depth must be at least 8, all three scores nonmate, prior report-to-report
  iteration cost at least 2ms, and current cost positive. Current costs below
  2ms are **not** excluded, so measured shrinkage is retained.
- Report both all eligible transitions and only the last eligible transition
  per search. The latter is not necessarily the final completed transition if
  subsequent reports fail a filter.
- Final-tail multiples require last depth >=8 and <100, last and previous
  scores nonmate, consecutive depths, last completed cost >=2ms, tail >=5ms,
  and no explicit stop. Also report the subset finishing before both
  conservative hard/emergency bounds with the already defined 5ms margin.
- Costs use log report-to-report timestamps, not rounded engine-reported
  milliseconds. No multiplier fitting, source changes, or repeated scan.

## Observations

| Sample | Count | Median multiple | Above 2x | Above 4x |
|---|---:|---:|---:|---:|
| All eligible completed growth transitions | 128,132 | 1.547x | 38.94% | 16.37% |
| Last eligible completed growth per search | 30,205 | 2.114x | 52.20% | 26.09% |
| Final tail / last completed cost | 20,898 | 1.309x | 36.15% | 17.87% |
| Same tail ratio, below conservative deadlines | 17,252 | 1.242x | 34.31% | 16.20% |

The 75th percentile completed-growth ratios are 2.932x for all transitions and
4.164x for each search's last eligible transition. All 32,347 searches parsed;
the only recorded anomaly was four stop commands outside active searches.
There were no pending searches, unsupported go commands, bounded or unparsed
depth infos, or negative timestamps.

## What this means for the proposed experiment

Keeping the **1x previous-iteration projection** while removing recursive soft
cancellation is not a spending-preserving refactor. Many observed completed
iterations cost substantially more than the previous one; many final tails have
already consumed more than that previous cost without producing another
completed-depth report. An admitted iteration can therefore run beyond the
soft target or hit the hard ceiling. The separation experiment needs explicit
remaining-bank, deadline and completed-decision guardrails before strength-game
admission; preserving the hard/emergency protections alone does not preserve
clock distribution. This is a research recommendation, not a policy change or
an authorized experiment launch.

## Censoring and measurement limits

Completed pairs are survivors under the current clock policy. Slow unfinished
iterations are absent, while selecting each search's last eligible transition
introduces additional stop-selection effects. These distributions are not an
unbiased branching-factor estimate or an estimate of the unobserved next
iteration's completion time. They must not be used to tune a new multiplier from
this match.

Final tails are censored observations and include finalization/reporting. Their
multiples are **conditional lower-bound proxies** for unfinished-iteration cost
only to the extent the tail is unfinished search; they are not formal measured
counterfactual completion times. Even a perfect estimate on this logged path
would not predict a changed policy's later TT/history state or bank trajectory.
Report-to-report costs approximate the timer's iteration duration and include
boundary/report overhead; the 2ms filter reduces, but does not prove elimination
of, that measurement issue. The stored sums aggregate overlapping transitions
and concurrent searches; they are not session wall time or independent samples.
