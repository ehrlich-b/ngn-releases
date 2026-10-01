# Independent standard Polyglot book-move oracle

`polyglot_book_move_oracle.json` is an exact byte-for-byte copy of the
root-frozen `../book-move-independent-oracle.json` created before this task's
model admission.

- Fixture SHA-256: `8289efed4fd50a58a6bff4a816c9b0ecdc7cea7d7d7acc7dcf0a17d26de993ba`
- Schema: `ngn-independent-polyglot-book-move-oracle-v1`
- Generator dependency: python-chess 1.11.2
- `chess/__init__.py` SHA-256:
  `1fde6a8e932508d14b31f6a584497b49c2969fc3c7cdc675347f819f0d667a5b`
- `chess/polyglot.py` SHA-256:
  `8dc20733bdc1297a9e8993a76737ba4f328a99185ae94b311ddbb5342fce32fc`
- Reused immutable 99-state source fixture SHA-256:
  `3e7acd53328562ab5fb6943441d586f8bb3723e269c1904c4f128ecee63d05fc`
- Historical generator SHA-256:
  `c76f0fd4e9bbafbebd4af23f6e668c23412be995f8a9a1a1ac8f8ae3d5b20e3b`
- Historical generation receipt SHA-256:
  `5ba656c3d0bef99ad5e2234dae33d3f4824e68d0af539a33dcacdfd26a2eb838`
- Dimensions: 23 cases and 7,651 aggregate before/after depth-2 nodes.

The cases contain literal independently generated 16-byte standard Polyglot
entries for all four orthodox castles, sixteen quiet/capture promotions across
both colors and all four promotion types, two legal en-passant captures, and
`e2e4`. The pinned python-chess reader loaded every literal entry and normalized
it to the recorded legal move. No key or move encoding was produced by NGN.

The fixture is immutable evidence. Do not regenerate or update it in place;
replace it only through a separately reviewed oracle-provenance task.
