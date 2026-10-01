# Owned K4 versus borrowed Rodent V1.1 Anand

Status at 2026-09-23 09:36 UTC: the fixed 200-game same-code diagnostic
completed and both independent audits passed. It is a direct borrowed-evaluator
comparison, not an absolute rating calibration.

The pre-result [HELD manifest](2026-09-23-owned-nnue-k4-vs-rodent200-held.json)
and [setup receipt](2026-09-23-owned-nnue-k4-vs-rodent200-setup-receipt.json)
bound 100 disjoint opening pairs 4241–4340, reversed colors, 10+0.1 clocks,
two concurrent pinned single-thread games, the exact same optimized NGN AVX2
engine binary for both roles, owned K4 model SHA-256
`cebec29cbab676af6a2ade2b67da5c9a01ddb7882ed604238c0e7d67893f72d6`,
and borrowed Rodent V1.1 Anand model SHA-256
`5f7480b56538e9e64ee02fb5b4842bf0394ee44b707dae3a63ec7f902abd0afb`.
The narrowly versioned runner passed all 32 WSL tests before launch. No book,
tablebase or adjudication was used.

The [terminal receipt](2026-09-23-owned-nnue-k4-vs-rodent200-terminal.json),
[chess audit](2026-09-23-owned-nnue-k4-vs-rodent200-audit.json),
[operational audit](2026-09-23-owned-nnue-k4-vs-rodent200-audit-operational.json)
and [final state](2026-09-23-owned-nnue-k4-vs-rodent200-state.json) passed for
all 200 games, 100 pairs and 27,971 legal plies, with zero probable embedded
book signature plies and no operational warning lines.

Owned K4 scored **34.5/200 (17.25%)**, from **9 wins, 51 draws and 140
losses**. The [frozen paired report](2026-09-23-owned-nnue-k4-vs-rodent200-report.json)
gave a **13.75%–21.00%** 95% score interval using 100,000 opening-pair
bootstrap resamples and predeclared seed 2026092301. Its relative Elo was
**−272 [−319, −230]** under these match conditions. This is a large,
well-resolved gap to the borrowed evaluator. It does not assign an absolute
rating to either engine or network.

This result is compatible with K4's earlier 80.875% HCE score: HCE and Rodent
are different comparators. It also shows the limit of treating Stockfish-score
validation MSE or overall static MAE as a playing-strength proxy. K4's 100k
static MAE was better than borrowed Rodent's, yet its game score was far worse.
The existing 32-position search panel and the near-equal score band are too
small or indirect to explain the full gap. The next work should diagnose
K4-versus-Rodent score behavior on positions actually reached in these games,
including decisive errors, score scale, king/output buckets and throughput,
before paying for another large label expansion.

The PGN, fastchess trace and final EPD remain on WSL and in a hash-verified
off-host archive at
`/Users/ehrlich/repos/ngn/output/nnue-owned-k4-backup-20260922/k4-vs-rodent200-games-trace.tar.gz`,
SHA-256 `7262da90d008d39456778628b7aaeddbfc829d7955c9f7f5995d463ca19c4507`.
All three contents matched the independent audit hashes. The
[final-files digest](2026-09-23-owned-nnue-k4-vs-rodent200-final-files.sha256)
matches the final state's SHA-256.
