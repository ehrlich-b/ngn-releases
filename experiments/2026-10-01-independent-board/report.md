# Independent board replacement — 2026-10-01

Baseline: 25a8397c7aed95ca1f5800e135b2752e631fb238 on the isolated HCE repair
branch. Existing benchmark ownership, match adapters, default branches and
released source/assets are preserved. No strength games ran.

Fresh personal native Luna max thread 01a0f8ef-6aa2-7580-8fa9-c344aa18fe61
received a typed board behavior contract, independently generated primitives
and synthetic evaluation-value adapters. It received no previous board/position
implementation, real evaluation tables or hidden controller oracle. It completed
at 19:36:32 UTC, after about nine minutes, reporting 124,933/300,000 goal tokens.
That token bound is not a credit/spending allowance. Personal Pro identity and
included weekly usage 75% were verified before model calls; the live catalog
offered 5.6 Luna max and no 6.1 model. No paid fallback or quota denial occurred.
This records a fresh-context generation process, not formal legal certification.

Standalone serial tests, race checks and vet passed. Root reviewed its ordinary
Go implementation. The independent controller probe covers 5,000 deterministic
board mutations, all square/scan controls, starting/render state, captures with
overlapping masks, all four castling cases, mailbox/occupancy and additive
accumulator consistency, copy isolation and rebuild/restoration. Mutation paths
also pass zero-heap-allocation probes. Its 2,326,306-byte output matches the
immediate baseline exactly, SHA-256
d428770d7c354d3b9df4bd001f5a17f4b72a6260f14198a6d55b2ebe80cead8a.

The existing 229,574-byte hash/move/null/EP/restoration and Polyglot proof remains
identical, SHA-256
c21d87035a2b7023b323067ac3744c5339302ab7d991f816301cbfc87f242b64.
Six HCE score/move/node/PV/perft scenarios remain identical, SHA-256
4b9dafa94087ae370725438b14ad53bd648f4b1a942d6add7dc27f80c5fed596.
Required engine/all-package short and race checks are recorded in checks.json;
commit admission requires every exit code zero and the proof PASSED marker.

Historical Zahak notices remain. Position is the remaining credited Zahak core
implementation after this stage; PeSTO-derived values remain. The precise PeSTO
permission and other recorded source/data questions remain gates. A comment
claiming PeSTO public domain is corrected to reflect the actual unresolved audit.

The documented classical-tuning mirror KierenP/ChessTrainingSets has also been
pinned at ef85c314b9f0c0ff814a0d870f6b0b7c7c7f4b87. Its complete MIT license
Copyright (c) 2020 Kieren Pearson and README contributor Alexandru Moșoi are
retained. This is the mirror's declared evidence; exact original training-file
match and original contributor permission chain remain unverified. The mirror
notice does not license the separately borrowed PeSTO tables.

All heavy work uses the coordinated shared CPU0/2 50% NGN quota, nice10, 4GiB
and serial Go. No timing/Elo claim, release, merge, tag or CCRL resubmission
follows from this stage.
