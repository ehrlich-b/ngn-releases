# Stockfish 18 SMALL full-refresh oracle result

The full-refresh reference implementation was accepted on 2026-09-06 against
Stockfish commit `cb3d4ee9b47d0c5aae855b12379378ea1439675c` and the official SMALL
network `nn-37f18f62d772.nnue` (SHA-256
`37f18f62d772f3107e1d6aaca3898c130c3c86f2ab63e6555fbbca20635a899d`).
The actual upstream scalar oracle emitted 145 rows, and
`TestOfficialAllLaneUpstreamOracle` matched both transformer perspectives, all
eight PSQT accumulators, transformed lanes, every affine-layer intermediate,
and all eight raw component pairs. Package, explicit-official, race, vet and
build gates passed. The root acceptance receipt is
`output/sf18-small-fullrefresh-20260906/root-terminal-review-v1.json`, SHA-256
`fbd7879065c413a6e5accf51d7a9bb9b7596dc76ed1ae72752f530a1101afedb`.

The initially reviewed `instrumentation.patch` had malformed hand-authored hunk
counts and was never used for accepted arithmetic. This directory contains the
exact corrected patch that passed `git apply --check`, built the accepted
Stockfish oracle, and generated the accepted 145-row fixture. The Go production
and test files are byte-identical to the executed gate source; only this
diagnostic patch and this result note differ from that preserved gate tree.
