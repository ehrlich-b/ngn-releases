# NGN K4 label packer

`cmd/ngnk4pack` is the independently maintained boundary between fresh
Stockfish labels and Bullet training records. It streams one completed
`ngn-k4-label-output-v2` shard, revalidates the frozen teacher/search contract,
the header-seeded per-record SHA-256 chain, every label identity, FEN, quiet
move, exact UCI info line, material inversion and sigmoid target, and the final
counts. Only accepted labels are emitted.

Each output is a sequence of 32-byte Bullet `ChessBoard` records. Pieces are
normalized to the side-to-move perspective, scores remain side-to-move
relative, and the unused WDL byte is the constant draw encoding `1` because the
trainer freezes `result_weight=0`. The receipt binds input and output hashes,
record ordering, teacher source/net identities and a digest over every accepted
ID, independently recomputed `ngn-k4-input-v1` identity, output ordinal and
score. The identity hashes both sorted king-bucketed/mirrored perspective row
lists plus the material output head; a sampler cannot substitute the older
plain-Chess768 key without rejection.

Both the BF file and receipt are published with no-clobber hard links only after
the entire input has verified and the temporary output has been synced. The
packer never overwrites prior evidence.
