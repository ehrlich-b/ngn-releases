# Independent Polyglot special-move oracle

`polyglot_special_move_oracle.json` is an exact byte-for-byte copy of the
root-frozen `../polyglot-independent-oracle.json` created before this task's
model admission.

- Fixture SHA-256: `3e7acd53328562ab5fb6943441d586f8bb3723e269c1904c4f128ecee63d05fc`
- Schema: `ngn-independent-polyglot-special-move-oracle-v1`
- Generator dependency: python-chess 1.11.2
- `chess/__init__.py` SHA-256:
  `1fde6a8e932508d14b31f6a584497b49c2969fc3c7cdc675347f819f0d667a5b`
- `chess/polyglot.py` SHA-256:
  `8dc20733bdc1297a9e8993a76737ba4f328a99185ae94b311ddbb5342fce32fc`
- Reused immutable Stockfish legal/perft oracle SHA-256:
  `897f7ebd0c3adc602839d7782673ddbecdd7c1bc72c58dc73fb424f8884b283e`
- Dimensions: 29 targeted histories, 99 states, 10,836 aggregate depth-2
  nodes, including four null-plus-quiet paths.
- Standard start-position control: `463b96181691fc9c`.

The histories cover the four proved illegal-EP repetition sequences, both
legal-EP controls, castling on both sides and wings, rook capture/rights loss,
and quiet/capture promotion to every piece for both colors. There is no random
campaign. Null comparisons intentionally cover only the first four FEN fields
and the Polyglot key because null-move clocks are hypothetical search state.
The pre-freeze setup's rejected invalid bare-rook castling continuation is not
part of this JSON and is not an engine result; the frozen castling vectors use
the independently validated blocker position.

The fixture is immutable evidence. Do not regenerate or update it in place;
replace it only through a separately reviewed oracle-provenance task.
