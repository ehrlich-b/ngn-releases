# Direct quiet-promotion candidate on accepted T8

**Locally validated, not installed or game-tested.** The external400-game pin
continues to measure accepted T8. This candidate follows the
[reproduced horizon witnesses](2026-09-05-quiet-promotion-witness.md) and may
only enter a separate game test after that pin completes and is reviewed.

## Exact scope and source

`output/qpromo-candidate-20260905/src` is an isolated archive with the accepted
T8 playing source. Root compared all158 tracked Go/module files against main;
only `engine/search.go` differs. Added `engine/qpromo_test.go` is test coverage.
The archive's documentation parent is `f8e1d2b`; accepted playing commit remains
`5cf301a870f6cd9218d8085166993f7ad5364aee`.

A direct pawn-bitboard helper appends Q/R/B/N when the moving pawn is on its
promotion rank and the forward square is empty. Qsearch retains its existing
specialized64-entry capture generation/order and appends the quiet promotions
into local96-entry move and score arrays. At most eight promotion-rank squares
times four variants add32 moves. SearchInfo's capture buffer is unchanged.
The helper is pseudo-legal; the existing make/check/unmake gate rejects pins.
Delta, SEE, TT, evaluation, search parameters and all other policies are unchanged.
It does not generate the full move list at every qnode, as the earlier diagnostic
did. Ordinary quiet checks remain outside the move class.

Frozen current artifacts, all under `output/qpromo-candidate-20260905/`:

- `quiet-promotion-qsearch.patch`:
  `e1e16ccd182edd6d62256b327f040fb6222d714f734af0540a1bf03e9a64b272`.
- Candidate `src/engine/search.go`:
  `e48892583a9ab0e947bf49601fbc41903584f7a7d0b25f923890fb31bdde3a22`.
- Mac `src/build/ngn_20260905_qpromo`:
  `05b83c143b630841f41ab621024bfcaa2cefb086b18f1c0b4347d3359cb7cd92`.
- Windows `src/build/ngn_20260905_qpromo.exe`:
  `8be9ba891b031f391ac0de738f65b814ce00eaf3bf04ffa8813f64fb45f96b0b`.
- Linux `src/build/ngn_20260905_qpromo_linux`:
  `a6fcf5481518f70598874c528c3f5b0e202cac4c185c29ea9099dfcb5737f2aa`.

Go1.26.2, CGO disabled, Mac arm64 and Windows/Linux amd64/v3. Earlier hashes
and full-test logs are preserved; the review follow-up changed a comment and
strengthened tests, without changing implementation semantics, then rebuilt
the three current binaries. Details: `metadata.json`, `FOLLOWUP-REVIEW.md` and
`SHA256SUMS-followup.txt`.

## Correctness validation and limits

Full short/race/vet passed. The independent Stockfish movegen oracle passed
2,293 positions,48,926 depth-two root divides and128 internal search roots.
UCI defaults and EP/castling/fifty-move regressions passed. Root read the actual
logs and verified all three current binary hashes and source scope.

Root additionally reran the strengthened tests. The direct quiet-promotion and
combined noisy classes match independently generated full pseudo/legal sets
for both colors, mixed capture/quiet promotions, blocked squares and deterministic
legal play. True diagonal pins with empty promotion squares assert four pseudo
variants and zero legal variants. Eight promotion-rank pawns exercise all32
variants. Both proven mate mirrors return exactly MATE_VALUE−1 and restore the
position, recursion frame and repetition stack. Root log:
`root-targeted-validation.log`.

These checks establish the new move class and preserve existing contracts on
the tested positions. They do not prove that generic delta/SEE pruning is ideal
for every checking promotion/underpromotion, or that the new tree wins more games.

## Mac performance check

Root froze both binary hashes and the six existing `scripts/ebfprobe.py` roots
before timing. Each fresh process verifies all14 T8 defaults, sets Hash64 and
searches a400,000-node budget. One warmup pair per position is excluded, then
eight alternating-order pairs per position yield **48 measured pairs**. No
other Mac engine task overlapped the timed probe. No Windows/WSL benchmark ran.

Candidate/base elapsed-time ratios:

| Existing root | Median ratio | Same completed-search output? |
| --- | --- | --- |
| Quiet QGD | 1.0031 | Same depth/score/PV/bestmove; nodes differ |
| Closed middlegame | 1.0058 | Exact after removing time/NPS |
| Late middlegame | 1.0007 | Exact after removing time/NPS |
| Najdorf | 0.9970 | Same depth/score/PV/bestmove; nodes differ |
| Rook endgame | 0.9931 | Same depth/score/PV/bestmove; nodes differ |
| Pawn endgame | 0.9424 | Same depth/score/PV/bestmove; nodes/seldepth differ |

Overall median ratio0.99777, geometric mean0.99195. All outputs repeat exactly
within each engine/position across the measured runs. The two identical-output
controls show small timing differences; this probe exposes no large speed cost.
The overall ratio is **not a pure speed gain** because added promotions change
the search tree on four roots. This is a small Mac sample, not native Windows
performance evidence or an Elo estimate. Reported nodes are from the last
completed iteration; the command uses the fixed400k budget.

Full method, frozen inputs and raw pairs: `root_speed_probe.py`,
`root-speed-inputs.json`, `root-speed-result.json`, `root-speed-summary.txt`.

## Prospective game decision

September5 priority update: the independently reproduced
[cached-mate stopping defect](2026-09-05-mate-stop-confirmation.md) is the next
correctness candidate after the live pin. Quiet promotions remain separate.
If that correction is accepted, rebuild this patch against the new immediate
base and freeze new artifacts before any promotion game gate.

The current external pin has priority and its candidate stays accepted T8.
After it completes, root first applies its original2800 decision rule. If the
goal remains unconfirmed and the measurement is valid, this single move-class
candidate is ready for a separate native startup check and prospective game
gate. No parameter alternatives, quiet-check additions, delta exemptions or
promotion-order changes are bundled. A completed game verdict is still required
before any installation or claimed strength gain.
