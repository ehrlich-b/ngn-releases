# SMP M1 worker-history ownership — 2026-09-05

Sol implemented this bounded slice in a fresh ngn-worker-history worktree at ba0b727; root reviewed and integrated its exact source with N1a.

SearchEngine now owns one persistent workerHistory value: main, continuation, follow-up and capture histories; pawn/non-pawn/minor corrections; killers; counters; and predecessor. SearchInfo receives an explicit pointer on iterative and fixed entry paths. UCI new-game clears and played-move replay use that receiver's history. Package compatibility wrappers retain the default instance. Model/parameter invalidation remains a later ownership step.

Root's mechanical review compared all 44 pre-existing move-ordering and correction function bodies: every body is unchanged apart from explicit owner qualification and formatting/comments. The table dimensions, arithmetic, statement order and warm lifetime are preserved. Each worker adds roughly 11 MiB for continuation/follow-up tables, plus smaller history structures.

New tests exercise real sequential searches after poisoning one receiver's corrections, with the shared TT normalized between searches. Clean receivers reproduce identical results while the poisoned receiver produces an observable difference. UCI tests verify predecessor, counter ordering and receiver-specific new-game clearing.

Integrated full short tests, full short race tests and vet pass. All eleven accepted-v2 cold/warm search fixtures match exactly in their recorded behavior, including full histories and the legacy 256/257 node-budget witness; only source metadata differs. Commands/logs/exits, source hashes, independent mechanical review, Sol receipts and root comparison are retained in output/m1-history-integration-20260905 with DONE_EXIT_0.

Different concurrent SearchEngine instances remain gated on shared TT, evaluation/pawn caches and configuration ownership. No helper workers, search-policy changes, multicore scaling or strength verdict are introduced by this slice.
