# Owned K4 equal-node and equal-time search diagnostic

Status: **K4 has both a real model deficit and a severe search-speed handicap.**
The 20M training expansion remains held. This is a 32-position diagnostic, not
a strength estimate or checkpoint-selection gate.

## Frozen comparison

The [32-position panel](2026-09-21-owned-nnue-search-panel-32.json) was committed
before the searches ran. Its SHA-256 is
`96de37a99f6c7649a1bf297329e49ce20490aa1c0c7c1e935620ce67ad92e50f`.
Positions are the first 32 in the earlier identity-hash-selected 1,000-position
calibration sample; the selection does not inspect model predictions or moves.

The same frozen NGN executable (SHA-256
`3c3257bf14b43883ef7c88608fdb06eefe2379928119154ea6d38571c05e61b8`)
searched with HCE, the selected K4 net and the exact borrowed Rodent V1.1 Anand
net. A pinned SF18 teacher searched each position at 200,000 nodes for a deeper
move reference. NGN searches requested 100,000 nodes or 250 ms. Every process
used one thread; NGN used Hash 128 MiB and Move Overhead 100 ms. Searches ran
serially, with backend order rotating across positions, on WSL CPU 12 at nice
10. The Lean hopper was not stopped and the GPU was unused.

The [complete per-position result](2026-09-21-owned-nnue-search-panel-result.json)
has SHA-256 `4b571bed01a0230d66402064c7b162b1aea45a27195a51310546846f8fcc0f67`.
The WSL copy is
`/home/ehrli/nnue-owned-k4-20260920/runs/k4-search-panel-20260921/result.json`.
Runner SHA-256 is
`e5a32d1b61a39d1fff67b4c994a6876aa0116757505e6cb66efbc69f70164d25`.

## Result

| Budget and evaluator | SF18 best-move agreement | Mean depth | Last exact reported nodes | Mean elapsed |
| --- | ---: | ---: | ---: | ---: |
| 100k nodes requested, HCE | 15/32 | 10.09 | 70,076 | 50.9 ms |
| 100k nodes requested, K4 | 13/32 | 10.16 | 67,389 | 340.7 ms |
| 100k nodes requested, Rodent | 17/32 | 10.44 | 69,324 | 63.7 ms |
| 250 ms requested, HCE | 17/32 | 12.28 | 222,163 | 148.9 ms |
| 250 ms requested, K4 | 14/32 | 8.75 | 32,244 | 150.5 ms |
| 250 ms requested, Rodent | 15/32 | 12.25 | 184,685 | 150.2 ms |

At the same `go nodes 100000` request, K4 took 6.7 times HCE's mean wall time
and 5.3 times Rodent's. At the equal clock request, it reported about 6.9 times
fewer nodes than HCE and 5.7 times fewer than Rodent, and reached about 3.5
fewer plies than HCE. The shared Move Overhead setting explains why all three
NGN backends used roughly 150 ms of the nominal 250 ms budget.

UCI reports cumulative nodes at completed-depth info lines and may finish the
requested budget between those lines. The table therefore gives the **last
exact reported nodes**, not a claim that the engines stopped at 67–70k rather
than 100k. Wall time for the identical node request is the cleaner cost
comparison. The panel does not report evaluator-call counts because this frozen
binary does not expose them through UCI.

The move-agreement sample is small. Under the node request, HCE matched the
teacher while K4 did not on six positions, and K4 matched while HCE did not on
four. Under the clock request those counts were six and three. These margins
do not establish a reliable fixed-node or fixed-time playing-strength gap.
The deeper SF18 move is a reference, not proof of the only good chess move.

## Decision

The [100k static audit](2026-09-21-owned-nnue-static-calibration.md) found K4
slightly worse than HCE overall and especially noisy in low-material and
near-balanced positions. This search panel adds a large timing deficit. Both
can contribute to the pilot's 1.5/40 game result; neither is a sole proven
cause. The earlier estimate of roughly fourfold *evaluation* cost understated
the observed full-search cost of this frozen K4 engine.

The next cheap discriminator is the same-corpus **K4-S1 schedule-only training
probe** from the failure diagnosis: 16,384 updates on the existing 1M accepted
training BF. It asks whether model quality improves with more optimization,
without paying for another 19M labels. Keep that training result separate from
the inference-speed work. Before real-time strength promotion, profile and
reduce K4's full-search cost and repeat this panel on a frozen candidate.
