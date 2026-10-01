# Safe-check relevance in the frozen game sample

Protocol written before new searches. This is a targeted diagnostic, not an
Elo gate or a population estimate. No playing evaluator is changed. The native
`r0905texelv2` confirmation remains frozen and uses no competing box CPU work.

Reuse all 24 games from the September 4 loss-study sample, including draw/win
controls. The old maximum-drop locator often selected late endgames. For each
game, consider its existing usable scan records with pre-move SF score at least
−300 cp. Require PeSTO material phase at least 12 (N/B=1, R=2, Q=4, capped24),
using independently replayed Stockfish FENs. Select the largest old shallow
drop, breaking ties at the earlier ply. Retain a game with no qualifying
position as unavailable; do not replace it. Freeze selected prefixes before
running any new analysis searches.

For each selected prefix, obtain fresh accepted NGN qcap at400000 nodes and
fresh Stockfish at completed depth16, Hash64/Threads1. Score the recorded move,
the current NGN move and SF's choice by fresh depth16 SF searches after each
move (deduplicate identical choices). Preserve full history for all searches.
Bounded, missing or incomplete-depth scores remain unavailable. Per-search
timeout60s; total analysis cap20minutes. No timed NGN-game reconstruction claim.

At every resulting opponent-to-move position, instrument the actual pinned
Chess-3 evaluator's safe/unsafe checking destinations and NGN's king-zone
component using Go overlays. Independently enumerate legal quiet checks with
Stockfish perft/checker output. Distinguish destination masks from legal moves;
exclude captures, pawn moves and promotions from this piece-check diagnostic.
Classify SF's best reply as checking/quiet/safe only when independently verified.

Report feature presence, legal relevance, and cases where a move loses at least
50cp relative to the scored SF alternative and SF's best reply is a legal quiet
check in the reference safe mask. This threshold identifies follow-up witnesses,
not evaluation causality. Existing search can see those checks; a feature's
presence and a bad move do not prove adding its coefficient fixes the choice.
Retain all controls and negative cases. Do not tune on these positions.

## Completed result

All24 original game identities and1668 scanned prefixes match the audited
200-game capture. One draw has no eligible position;23 games remain (12 losses,
5 draws,6 wins). All23 root and49 distinct child SF searches produce usable
completed-depth16 scores. Local analysis completed in21.5seconds. The original
sample is still a deliberately stratified and shallow-drop-selected sample.

- Nine continuations offer a legal quiet piece check. Only one offers a check
  in Chess-3's safe mask: two legal moves to a8, Ra8+ and Qa8+.
- That one position also meets the50cp diagnostic threshold. No other game
  supplies recurrence under this particular selection and test.
- GAME188, against Counter3.8, after95 plies: a fresh repaired NGN400k search
  chooses **Rd7** (`d8d7`), allowing **Ra8+**. SF scores that choice −271cp for
  NGN, versus −183cp after its preferred **Ng7** (`h5g7`), a diagnostic88cp gap.
  NGN's king-zone penalty for Black is zero after both choices, while the actual
  reference evaluator and SF legal checks identify the a8 opportunity after Rd7.
  The original timed game played **Qd7**, not Rd7; this is not attribution of the
  recorded loss to that fresh-search choice.

```text
Before choice: 3r2k1/2q2p1p/6pb/2p1P2n/2P2P2/1P1p1QPP/R5K1/2BB4 b - - 0 48
After Rd7:    6k1/2qr1p1p/6pb/2p1P2n/2P2P2/1P1p1QPP/R5K1/2BB4 w - - 1 49
```

This is a concrete follow-up witness, but insufficient evidence to prioritize a
broad king-danger transplant. No term was added or tuned. Next use the existing
search-family toggles and a larger node budget on **all** current-NGN choices
with at least50cp reference disagreement, including this case. This distinguishes
some search effects without assuming evaluation is the cause.

Raw files under `output/recovery-2026-09-04/`:

- `safe-study-selection.json` SHA-256
  `2f6f9e755580599ba990335bb140d78d33b81661fb4c76ae02ca4c2c2b7cd234`.
- `safe-study-result.json` SHA-256
  `95926b6cc5dd4aa66615b071ead937f413d9c9f7ed4b3957835d1a6afcf82737`.
- `safe-study-provenance-audit.json`: root's separate full-prefix/source audit.
- `safe-study-{reference,ngn}-features.json` and overlay sources/logs: actual
  evaluator outputs; both overlays pass. All feature positions come from legal
  replay and legal selected continuations verified with SF.

The executed driver hash is recorded in the result. The committed reproducer
additionally performs the successful provenance audit before any new searches
and records source/Go identities; these checks were independently run after the
original result and do not change its selection or chess calculations.
