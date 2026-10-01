# Owned K4 versus Rodent at equal search nodes

Status at 2026-09-23 10:43 UTC: the frozen 200-game fixed-node match completed.
The [terminal receipt](2026-09-23-owned-nnue-k4-vs-rodent200-fixednodes-terminal.json),
[chess audit](2026-09-23-owned-nnue-k4-vs-rodent200-fixednodes-audit.json)
and [operational audit](2026-09-23-owned-nnue-k4-vs-rodent200-fixednodes-audit-operational.json)
all passed. The independent chess audit checked 200 games, 100 reversed-color
opening pairs and 28,046 legal plies. It found zero probable embedded-book
signature plies; the trace audit found zero warning lines. The frozen
[Fastchess command](2026-09-23-owned-nnue-k4-vs-rodent200-fixednodes-command.json)
used `nodes=160000` for both roles and no time-control argument.

Owned K4 scored **37/200 (18.5%)**, from **12 wins, 50 draws and 138 losses**.
The predeclared [opening-pair report](2026-09-23-owned-nnue-k4-vs-rodent200-fixednodes-report.json)
gave a 95% score interval of **14.75%–22.5%**, or relative Elo **−258
[−305, −215]** against borrowed Rodent V1.1 Anand at this node budget.
The previous same-opening [10+0.1 match](2026-09-23-owned-nnue-k4-vs-rodent200-result.md)
scored 17.25%. Pairing by each exact six-ply opening, fixed nodes improved
K4's score by **1.25 percentage points**, with a 95% paired-bootstrap
interval of **−3.75 to +6.5 percentage points**. The result is compatible
with little or moderate speed contribution; the large disadvantage persists
when both evaluators receive the same node command. It does not measure an
exact speed-caused Elo fraction or an absolute rating.

The archived PGN reports roughly 1.407 billion completed-depth nodes in
1,836.7 summed search seconds for K4, versus 1.387 billion in 1,485.4 seconds
for Rodent; both have median reported depth 12. PGN node counts are from the
last completed-depth info line and can fall short of the 160k `go nodes`
request. The near-equal reported node totals and longer K4 search time are
consistent with the [real-clock throughput diagnostic](2026-09-23-owned-nnue-k4-vs-rodent200-throughput.json),
but model evaluation and tree shape remain intertwined.

The complete run, including PGN, trace, audits, receipts and frozen inputs,
has a verified off-worker backup at
`/Users/ehrlich/repos/ngn/output/nnue-owned-k4-backup-20260922/k4-vs-rodent200-fixednodes-full-run.tar.gz`.
Its SHA-256 is
`b973e74c2afd9ebe6eab8e57d6b63aef6907974064acbd144d72f4bdcd49f68c`.
All 87 files listed in the runner's
[final-files receipt](2026-09-23-owned-nnue-k4-vs-rodent200-fixednodes-final-files.sha256)
were found in the archive and individually matched their hashes. That
receipt's SHA-256 `2ad034c2e92b13fb2c6693d4a9631132b304487b3d1f01ddca5913f41a1bbb72`
matches the [final state](2026-09-23-owned-nnue-k4-vs-rodent200-fixednodes-state.json).

Decision: keep the original owned model as baseline, without claiming it
approaches the borrowed evaluator or 3000 absolute rating. The next useful
test is a frozen analysis of errors and sibling move ranking on positions
reached in these games. A longer training run over unchanged data is not
supported by the current loss-to-game evidence.
