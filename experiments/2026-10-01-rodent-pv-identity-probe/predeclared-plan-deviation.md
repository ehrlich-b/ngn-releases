# Predeclared-plan deviation

The first seven searches completed and their raw evidence is preserved under
`evidence/attempt1/`, but the first classifier wrapper exited 1 after search.
It incorrectly required the direct promotion-node search's **best move** to be
one of `g2g1q/r/b/n`. The frozen node also has legal king moves, and the exact
binary legally chose `f3f2`. This was a probe assertion error, not an engine
failure and not a search timeout.

No search was discarded or repeated. The classifier was corrected to require:

- the exact four `g2g1q/r/b/n` alternatives are legal at the frozen node;
- the reported best move is independently legal; and
- the separate forced-promotion control, whose only legal moves are promotions,
  emits a suffixed promotion.

The preserved seven receipts are reused byte-for-byte. Because none reproduced
the suffix-less token, only the two conditional warm-TT searches declared as
steps 8–9 are run next. Thus the total remains the predeclared nine searches.
