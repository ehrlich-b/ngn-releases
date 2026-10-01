# Conservative hard/emergency deadline exclusion

One additional read-only scan of the **existing** run-003 candidate log took
**2.942 seconds**. The parser has a 30-second alarm. No engine, match, build, or
test was launched and the hopper was not touched. The files retain the
[executed parser](clock_hard_bound_audit.py) and its [exact JSON output](clock-hard-bound-result.json).
The original [tail audit](clock-tail-audit.md) defines timestamps and scope.

## Source and protocol preconditions

`git rev-parse 52ee629:engine/time.go f75b578:engine/time.go` returned the same
Git blob for the match and review source:
`3cb408b6fae48d4d7098c0f9216a0df8c418714d`. The relevant UCI clock admission code
is also unchanged; the only `engine/uci.go` diff between these revisions adds
V1.2 to the advertised backend option.

Source establishes:

- Emergency reserve is 100ms (`engine/time.go:109-117`).
- Tournament limits use the selected side's bank/increment and bypass the
  non-tournament postprocessing (`:128-168`).
- Usable bank is `max(bank - emergency - overhead, 0)`; soft is
  `usable/moves + .8*increment`; hard is `max(soft,min(4*soft,.3*usable))`
  (`:192-215`).
- Without explicit moves-to-go, the estimate is capped at 40 (`:218-240`).
- Hard interruption requires `elapsed >= hard`; emergency interruption requires
  `bank-elapsed <=100ms` (`:426-435`, also setup checks at `:575-587`).
- The move clock begins when NGN receives go, before setup
  (`engine/uci.go:889`; `engine/time.go:126-129`).
- The implicit maximum completed depth is 100 (`engine/global.go:3`,
  `engine/uci.go:927-932`). Completed mate iterations may terminate early
  (`engine/search.go:970-978`).

The scan observed **400 Move Overhead settings, all 100ms**. All **32,347** NGN
go commands consisted only of `wtime`, `btime`, `winc`, and `binc`; no unsupported
or duplicate key, explicit movestogo/depth/nodes/movetime, infinite or ponder
command entered the analysis. Both-side minimum increment was always 100ms.
The parser would exclude those alternative command shapes rather than assume
the tournament formula. There were no unmatched, duplicate, missing-info,
negative-time, bounded-info, unparsed-info or pending records. Four NGN stop
commands occurred outside active searches; none interrupted an included search.

## Lower-bound derivation

Let `B=min(wtime,btime)`, `I=min(winc,binc)`, with all units milliseconds:

```
U = max(B - 100 - 100, 0)
Smin = U/40 + 0.8*I
Hmin = max(Smin, min(4*Smin, 0.3*U))
Emin = B - 100
```

Whichever side NGN plays, its actual bank and increment are at least B and I,
and its estimated remaining moves are at most 40. Thus its actual soft budget
is at least Smin. The hard-budget expression is monotone in soft and usable
bank, so Hmin is a lower bound on its hard budget. Emin is a lower bound on its
emergency elapsed-time deadline.

A search with logged `go-to-bestmove duration +5ms < Hmin` **and**
`duration +5ms < Emin` completed before either deadline even under these
conservative inputs. The 5ms safety margin is retained rather than treating
sub-millisecond log/engine timestamp alignment as exact. As with the original
audit, conclusions rely on those recorded timestamps; no new timestamp
instrumentation or independent logging-latency bound was run.

## Results with the 5ms safety margin

| Population | Searches | Before both conservative deadlines | Share of that population's tail time before both deadlines |
|---|---:|---:|---:|
| All parsed searches | 32,347 | 26,922 (83.2287%) | 90.4116% |
| Tail >=5ms | 21,192 | 17,388 (82.0498%) | 90.4462% |
| Tail >=5ms, last score nonmate, last depth <100, no external stop | 21,029 | 17,326 (82.3910%) | 90.6093% |

The last subset contains 1,723,047.357ms of tail; 1,561,241.132ms belongs to
searches that completed before both conservative deadlines. Without the 5ms
margin, the corresponding share is 91.3662%; the conclusion is not dependent
on a zero-margin boundary. No >=5ms-tail search ended at completed depth 100.

This substantially reduces the concern that most observed tail simply reflects
unavoidable hard/emergency interruption. It strengthens admission of the
clock-boundary experiment. It **does not prove** that all remaining termination
is a soft abort, that all measured tail is search rather than finalization, or
that an alternate clock policy would produce better moves. Incomplete work can
still contribute useful TT/history. Changes in the remaining-clock trajectory
remain a real risk, and only a prospective real-clock match can establish gain.
