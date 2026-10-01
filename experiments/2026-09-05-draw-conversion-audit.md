# Draw conversion and terminal audit

Read-only survey of frozen `r0904pin.pgn`, SHA-256
`a6b25b2af418f66c7c7cd10b9addaf48b5be4cea53e950d042caa6310848c077`.
The actual totals are **69 draws:49 repetition draws and20 max-ply draws**.
An intermediate working count of79/59/20 was incorrect. The original per-anchor
W/D/L and numeric rating report already used the correct totals.

## Proven measurement defect

Game70, NGN Black against v7.2.0, is recorded `D max-moves` after200 plies.
Its final FEN is `8/8/8/8/7r/8/5k1K/8 w - - 27 101`: Stockfish reports
check from h4 and zero legal moves. NGN delivered checkmate on the final allowed
ply. `internal/uci/uci.go` only checked terminal states inside
`for len(moves) < cfg.MaxMoves`, so it returned the cap draw without checking
the final board. The same post-move omission let early score adjudication
override actual mate/stalemate.

The frozen replay audit checked that a game *labeled* checkmate was checkmate,
but did not require a checkmated final board to have that label. Its PASS was
therefore insufficient to establish correct results. Root's new independent
final-board supplement checked **all200 games**, finding exactly game70 in
conflict. Raw records and the original2726 [2668,2784] report remain unchanged;
this is a disclosed measurement defect, not a retrospective rerating.

Isolated harness correction and red/green regressions are in
`output/harness-boundary-20260905/`. It checks actual terminal states before
the cap and before score adjudication, preserving time-forfeit ordering.
Both colors/result perspectives, final-ply mate/stalemate/repetition/fifty-move
draw, ordinary cap, cap-zero mate and adjudication precedence are covered.
Full internal/uci, short full suite, full race suite and vet pass.
Production patch SHA-256:
`dc36280779796a4b69bc7f29ef29d4493eee011dd22b55c9e766da4c45d2e3ef`.
The correction is now **integrated in main** after the completed pin's audit
and final integrity checks closed its source freeze. Root verified all159 Go
and module files against the tested archive: the only production change is
`internal/uci/uci.go`, with `boundary_terminal_test.go` added. The exact sources
retain the archived full short/race/vet passes, and all three boundary-test
parents also passed on native Windows before both subsequent match launches.
No local engine or test was executed for integration. Hash/parity receipts are
`main-source-parity-{before,after}-integration.json` in the archive directory.

The corrected versioned SPRT is already in use by the Windows mate-stop gate;
its completed200-game native A/A passed with zero errors/flags. Corrected
gauntlet SHA-256 is
`499946e16f9758f2dd52d9760e1752565d98438b221c24bb451e46cd8a58a247`.
Windows default gauntlet/SPRT copies have since been updated with rollback
siblings; the new rating run selects the corrected versioned binary explicitly.
This changes result bookkeeping,
not NGN's playing source, and carries no engine Elo claim.

The subsequent400-game pin retained its original binaries, labels and sample.
Its completed terminal audit found exactly two more mislabels, games211/216;
the [rating remains held](2026-09-05-accepted-t8-pin-result.md). An affected
run cannot establish the goal merely because the frozen collector reports
PASS. No score-dependent relabeling, extension or automatic rerun.

```text
python3 output/draw-endings-20260905/audit_terminal_precedence.py <completed.pgn> --games 400 --output <new-terminal-audit.json>
```

Supplement SHA-256:
`f8cf8f1aa73100ff8e7f8c65de351e9e31e874f390e3be8690929408ae42c1bf`.
It is a final-board check paired with the existing full legal-history replay.
Four real CLI mutation tests verify that it rejects the original erroneous
label, accepts the corrected label in a temporary fixture, rejects a wrong
winner, and rejects a nonterminal board labeled mate. No original file changes.

## Proven carried-state search failure

Seven repetition draws ended with NGN holding a rook or multiple major pieces
against a bare king: games6,45,46,58,80,105,118. The first reading incorrectly
treated the queen/rook mating helpers in `engine/eval.go` as active. A complete
call-site check shows `evaluateBasicEndgames` is called only by the old component
diagnostic test, and `evaluateBasicMatePattern` only by that dormant helper.
Commit c6675ff removed the production call in the April23 PeSTO replacement.
The live evaluator does still call king activity below phase6. These distinctions
are recorded in the [live-feature comparison](2026-09-05-live-evaluation-features.md);
the proved cached-mate control-flow failure does not depend on those helpers.

Root tested the last NGN decision in each game with the accepted Mac build,
SHA-256 `0a42c12d287269251946f4bcbd016a497f200965a445b9806c22d57a982b4345`.
Fresh processes, Hash64,400k nodes, both full history and FEN-only: all seven
choose a different move from the archived draw-causing move and report mate.
This alone does not establish a repair or validate the reported mating lines.

Then one process per game searched every NGN turn at400k nodes while the
archived moves were forced between searches. This is a controlled state-warming
experiment, not a reconstruction of the old binary's exact clocks or TT.
It reproduces **two actual draw-causing choices in the current release**:

| Game | Root FEN | Warm choice | Search report | Opponent drawing reply |
|---:|---|---|---|---|
|46|`8/3K4/5R2/4Q3/6k1/8/8/8 w - - 13 96`|`e5e8`|depth1, mate15,57 nodes|`g4g5`|
|118|`8/8/8/8/5K2/8/1k2r3/3q4 b - - 9 94`|`e2f2`|depth1, mate13,59 nodes|`f4e5`|

Both drawing replies are the archived legal moves and produce the final
threefolds independently inventoried above. Root also replayed every prefix
through Stockfish and independently counted the final FEN's third occurrence.
After each bad NGN choice, Stockfish selects that exact drawing reply at100k
nodes and reports0cp; see `root-warm-draw-proof.json` and its transcript.
Cold full-history choices were
`e5h2` and `d1f1`, respectively. The other five warm probes avoid the particular
archived draw move; this is not a claim that their complete continuations win.

NGN's iterative search stops immediately on *any* mate-range score, including
a cached distant mate at depth1. Qsearch can return an exact TT mate without
expanding the opponent's drawing reply. This is a control-flow/history problem;
changing evaluation weights does not repair that mechanism.
The unconditional mate stop entered in `cadce3c5c97c2f82245fbba5cfe260ed5959778e`
on May23,2026, as an optimization for repeated maximum-depth mate searches.

Pinned Counter3.8 offers a concrete comparison: `engine/timemanager.go`,
`OnIterationComplete`, stops for mate only when the completed depth reaches
the reported mate distance **plus five plies** (`winIn(depth-5)` /
`lossIn(depth-5)`). Root selected and locally validated that isolated guard:
both reproduced bad choices change, and both complete actual five-ply mates
against Stockfish in root's carried-state continuation checks. Full short/race,
vet and UCI checks pass. [Frozen candidate and native gate](2026-09-05-mate-stop-confirmation.md).
No playing patch is integrated or deployed, and no Elo claim is made.
The pinned Blunder8.5.5 `Search` and Chess-3v4 `iterativeDeepen` loops also
continue through ordinary depth/node/time limits instead of NGN's unconditional
mate-score exit. This comparison concerns their source behavior, not equivalent
search trees or a cross-list rating claim.

## Evidence

`output/draw-endings-20260905/` contains the full69-ending inventory, complete
histories, synchronized Stockfish terminal transcripts, archived repetition
counts, root final-board audit, cold and warm UCI transcripts, reproducible
probe scripts and `root-evidence-sha256.json`. Warm result SHA-256:
`50fdd2f2ef778ccb52b434a49aef560668055a19d9936aeb5068fa8ffa353ca6`.
The first cold probe's JSON reused `fen` and `history` keys for results,
overwriting its redundant input fields. Its transcript was intact; v2 fixes
the field names and reruns all seven cases. Use the v2 artifacts.
