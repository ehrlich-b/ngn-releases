# Owned K4 20M lower-rate result and resume point

Status at 2026-09-23 08:29 UTC: the 20M label, training, static calibration,
search panel, disjoint 40-game HCE screen, and fixed 400-game confirmation
completed. The owned-network milestone passed its paired lower-bound gate.
No absolute rating claim has been made.

## Preserved run

The [20M label receipt](2026-09-22-owned-nnue-main20m-label-run-receipt.txt)
records 19,123,414 accepted expansion positions available, above the 19M
target. The [training receipt](2026-09-22-owned-nnue-main-m1-lr1-run-receipt.txt)
binds the exact finalized 20M corpus manifest
`ef6aca46d496be0fd876eb063cda3650a85ba7ae92c683105bf433a29565ad8e`
and BF `55e8d7bf37bf774f748b36ec75a96e3dfd9970e1dc100da65191352869aa76e0`.
The [completion receipt](2026-09-22-owned-nnue-main-m1-lr1-training-completion.json)
records all 131,072 updates and 2,147,483,648 presentations.

The frozen [selection](2026-09-22-owned-nnue-main-m1-lr1-selection.json)
chose eligible update **20,480** by validation integer MSE **0.007170**. Its
retrospective stop point was update 45,056. The selected model is SHA-256
`cebec29cbab676af6a2ade2b67da5c9a01ddb7882ed604238c0e7d67893f72d6`
at `/home/ehrli/nnue-owned-k4-20260920/runs/k4-main-m1-lr1-20260922/model-selected.nnue`
on WSL. Its hash was verified after the screen.

The [static analysis](2026-09-22-owned-nnue-main-m1-lr1-static-analysis.json)
of the inherited 100k positions reports MAE **130.18 cp** for K4, **152.70**
for HCE and **133.13** for borrowed Rodent. The
[32-position panel](2026-09-22-owned-nnue-main-m1-lr1-search-panel-result.json)
reports SF18 move agreement at equal nodes of **17/32** for K4 versus **15/32**
for HCE, and at equal time **21/32** versus **17/32**. These are diagnostic
measurements, not rating estimates.

## Real-clock screen

The [setup receipt](2026-09-22-owned-nnue-main-m1-lr1-screen-setup-receipt.json)
and [HELD manifest](2026-09-22-owned-nnue-main-m1-lr1-screen-held.json)
bind opening pairs 4021–4040, disjoint from the earlier K4 screens. The
[launcher](2026-09-22-owned-nnue-main-m1-lr1-screen-launch.sh) ran as user
systemd unit `ngn-k4-main20m-lr1-screen-40-20260922.service`, with one pinned
worker, 10+0.1 clocks, the same AVX2 engine for both evaluator roles, and no
embedded book or adjudication. The approval token and approved manifest remain
only on WSL and are not in Git.

The [terminal receipt](2026-09-22-owned-nnue-main-m1-lr1-screen-terminal.json)
closed **40 games in 20 pairs** at 20:27 UTC. The
[independent chess audit](2026-09-22-owned-nnue-main-m1-lr1-screen-audit.json)
passed 6,033 legal plies and recorded zero probable embedded-book signature
plies. The [operational audit](2026-09-22-owned-nnue-main-m1-lr1-screen-audit-operational.json)
passed. The [final state](2026-09-22-owned-nnue-main-m1-lr1-screen-state.json)
binds the [file digest](2026-09-22-owned-nnue-main-m1-lr1-screen-final-files.sha256).
The PGN remains on WSL at
`/home/ehrli/nnue-owned-k4-20260920/integration-v1/k4-main20m-lr1-screen-40-v1/run-001/games.pgn`,
SHA-256 `bce751ff7491021504382f8f8e99bda240ec98cb54cdcff46de493a1c78e26e4`.

Against same-code HCE, the owned 20M K4 model scored **33/40 (82.5%)**:
**30 wins, 6 draws and 4 losses**. This clears the prospectively stated 50%
screen gate. The next planned strength step is the 400-game paired 30+0.3
confirmation, requiring a paired-bootstrap 95% lower bound above 50%, followed
by borrowed-evaluator and actual-engine comparisons. The 40-game HCE result
does not establish a 3k absolute rating.

## Fixed 400-game confirmation

At 2026-09-23 03:14 UTC, the separate fixed 400-game 30+0.3 confirmation
started as WSL user unit `ngn-k4-main20m-lr1-confirm-400-20260922.service`.
The [HELD manifest](2026-09-22-owned-nnue-main-m1-lr1-confirm400-held.json)
and [setup receipt](2026-09-22-owned-nnue-main-m1-lr1-confirm400-setup-receipt.json)
bind 200 new opening pairs 4041–4240, two concurrent pinned one-thread games,
the original selected model SHA above, and the unchanged same-code HCE role.
The unit was verified active with four engine processes after UCI preflight and
finished at 2026-09-23 08:29 UTC. The [terminal receipt](2026-09-22-owned-nnue-main-m1-lr1-confirm400-terminal.json),
[independent chess audit](2026-09-22-owned-nnue-main-m1-lr1-confirm400-audit.json),
[operational audit](2026-09-22-owned-nnue-main-m1-lr1-confirm400-audit-operational.json)
and [final state](2026-09-22-owned-nnue-main-m1-lr1-confirm400-state.json)
passed for all 400 games, 200 pairs and 56,891 legal plies, with zero probable
embedded-book signature plies and no operational warning lines.

K4 scored **323.5/400 (80.875%)**, from **293 wins, 61 draws and 46 losses**.
The [pre-result paired-bootstrap rule](2026-09-22-owned-nnue-main-m1-lr1-confirm400-report.py)
gave a [95% paired score interval](2026-09-22-owned-nnue-main-m1-lr1-confirm400-report.json)
of **77.75%–83.875%** (100,000 resamples, frozen seed 2026092201), with its
lower bound above the predeclared 50% gate. The owned-network milestone
therefore passes. The implied +250 relative Elo [217, 286] is only against
this same-code HCE at these conditions. It is not an absolute 3k rating.

The 400-game PGN, fastchess trace and final EPD are retained on WSL and in a
hash-verified off-host archive at
`/Users/ehrlich/repos/ngn/output/nnue-owned-k4-backup-20260922/confirm400-games-trace.tar.gz`,
SHA-256 `3b97947713792af0cb6b373f58852aadf214835864528e5f3daad742c4874ae9`.
Its three contents each matched the independent audit's SHA-256. The
[final-files digest](2026-09-22-owned-nnue-main-m1-lr1-confirm400-final-files.sha256)
matches the terminal state's SHA-256. Next: direct comparisons against the
borrowed evaluators and actual external engines under matched conditions.
