# Predeclared Rodent V1.2 promotion-PV probe plan

Written before any execution of the frozen Rodent binary on 2026-10-01 UTC.

## Fixed scope and inputs

- Execute only the exact read-only binary whose required SHA-256 is
  `9cfb8195207ee5695c1973a89664ab73b34b5bcbc10ad3bc0f0afe28b9713cbc`
  and size is 7,311,522 bytes.
- Verify before execution that bytes `[2538080, 2538080+4744768)` hash to
  `c35a1abc1b8c1cb1d5f4221454d494c1a6da1ed9088fd51ab27038bfa74b5053`.
- Run the binary directly, never through the historical launcher, in a new
  private empty directory under this task root. Inherit the service's CPU 0/2,
  50% CPU, nice 10 and 4 GiB controls; do not set affinity or change resources.
- Configure `Threads=1`, `Hash=128`, `UCI_Chess960=false`,
  `UCI_LimitStrength=false`, `UCI_Elo=3000`, with `isready` after each option.
  Require the UCI banner/options and every `readyok`. Do not invent OwnBook;
  record that it is unadvertised and that the working directory is empty.

## Search order and controls

Maximum: 9 searches, each no deeper than 18 and killed with its owned process
group after 10 wall seconds; aggregate ceilings are 120 CPU seconds and 180
search wall seconds. Stop the conditional warm-TT pair if a runtime witness and
the positive/negative controls already answer the question.

1. Fresh process/hash, frozen root FEN, `go depth 17`.
2. Fresh process/hash, frozen root FEN, `go depth 18`.
3. Fresh process/hash, exact full frozen `position startpos moves ...`, depth 17.
4. Fresh process/hash, exact full frozen history, depth 18.
5. Forced-promotion positive renderer control, depth 4: require a legal
   last-rank pawn move and a UCI promotion suffix.
6. Ordinary legal-PV negative control from startpos, depth 2: require a legal
   ordinary PV and no false promotion classification.
7. Direct search from the saved nine-ply prefix's promotion node, depth 4:
   independently require each legal root move to be one of `g2g1q/r/b/n`.
8–9. Conditional only if 1–4 do not emit the saved defect: one process searches
   the frozen root at depth 18 and repeats it at depth 17 without clearing the
   TT, to bound a small warm-TT mechanism. No wider history/search campaign.

The classifier will walk every reported PV against an independently verified
python-chess 1.11.2 dependency if its installed bytes match the pinned
`chess` package SHA-256
`1fde6a8e932508d14b31f6a584497b49c2969fc3c7cdc675347f819f0d667a5b`.
It will keep historical lines separate from new lines, report the exact first
illegal token and legal alternatives, and never fill in or infer a promotion
suffix.

## Interpretation limits

- A newly emitted suffix-less pawn last-rank token is exact-binary runtime
  evidence. A failure to reproduce is inconclusive and leaves the gate closed.
- Pinned official-tag source can prove how its move bits, renderer, PV walk and
  TT operate. It cannot prove that the release binary was compiled from that
  exact commit because the binary has no VCS revision.
- A copied dependency-free source model, if used, is source-derived evidence
  only—not binary evidence.
- No game, strength conclusion, compatibility exception, guessed promotion,
  rescoring, adapter/harness change, production change or publication is in
  scope.
