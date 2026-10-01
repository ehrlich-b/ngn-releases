# Counter 5.5 multicore comparison and follow-on candidates

Source comparison only. No implementation, benchmark or strength acceptance follows from this note. Finish the active NGN 8v1 test and the pretrained-evaluator gates before another search-policy experiment.

The pinned Counter release is `63c487ca724c620f71c129d62129c6fb9109c872`. Its advertised Threads maximum comes from `runtime.NumCPU()` (`cmd/counter/main.go:55`), not a fixed one-worker ceiling. The saved one-CPU handshake is valid evidence of that restricted launch only. A future matched 8v8 comparison requires an actual full-mask handshake, resource checks and legal search preflight.

Counter's `pkg/engine/lazysmp.go` creates one worker goroutine per configured thread (lines 35–46), then distributes whole-depth searches through channels. Its coordinator starts searches at the deepest completed depth plus one, sending some workers one depth further after enough searches have been assigned at that depth (lines 58–69). Each task seeds the current best move and aspiration score. Any deeper completed result can replace the published main line (lines 84–96). This is source evidence of scheduling and result-selection mechanisms; it is not evidence that either mechanism improves NGN.

NGN's accepted search source `168385f` gives each helper the same independent iterative-deepening loop. Helpers affect shared TT contents and aggregate nodes; the primary owns time decisions, callbacks and the returned move/PV. `engine/search_smp.go` starts helper loops with no time manager or callback and collects their final results only after requesting stop. `engine/search.go` starts every worker at depth one. Private histories and TT timing can make their trees diverge, but there is no explicit depth staggering or helper result selection.

Two bounded future experiments are distinguishable:

1. Add deterministic helper depth staggering while preserving primary result/time ownership and all one-thread behavior. Predeclare the small schedule, keep bounds and stop checks intact, and measure useful paired-clock strength against the accepted SMP baseline. Aggregate NPS alone cannot establish improvement.
2. Consider completed-helper result selection separately. This has a larger correctness surface: publish an immutable completed-iteration snapshot, validate its legal PV at the current root, define deterministic depth/score/tie rules, keep one callback owner, and preserve mate/reporting/cancellation behavior. A deeper reported depth alone is insufficient grounds to select a result. Do not combine this with staggering in its first test.

These are candidate mechanisms from an identified successful Go engine, not prerequisites for accepting the current simpler SMP implementation. Counter's channel scheduler should not be transplanted wholesale without measuring NGN's ownership and timing costs.

Exact files read for this comparison:

- `/home/ehrli/repos/ngn-counter-pretrained-control/output/counter-pretrained-control-oracle-v1/upstream/cmd/counter/main.go`: `2330c668a5a7021c159d1825bbfd1c6984a4083399bee9c9a890842e430415ce`
- `/home/ehrli/repos/ngn-counter-pretrained-control/output/counter-pretrained-control-oracle-v1/upstream/pkg/engine/lazysmp.go`: `01fabc5cd22f09f8bd1269652e09473c27edb6df18a0c28f5b71bec88d1a9231`
- `/home/ehrli/repos/ngn-counter-pretrained-control/output/counter-pretrained-control-oracle-v1/upstream/pkg/engine/engine.go`: `601eacd494f7e9401d2df715ea96764fb21d1de626aba029dc95ef7962740bb8`
- `/home/ehrli/repos/ngn-next/engine/search_smp.go`: `80aca4a82563a0cc4832ac3a2a6f256a82229cfac38a4cb6bcbd50e17eb9bede`
- `/home/ehrli/repos/ngn-next/engine/search.go`: `717053489eddbf5fa19b5c009165972ec10e80aac23fe6d1c612b2a09ca30bb9`
