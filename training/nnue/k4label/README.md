# NGN K4 fresh-label stage

`cmd/ngnk4label` labels one immutable sampler shard with the pinned official
Stockfish 18 teacher. It is intentionally single-process: the run supervisor may
place at most two independent shard processes on admitted CPUs without allowing
worker scheduling to change a shard's output order.

The JSONL input begins with one `ngn-k4-label-input-v2` header followed by the
declared number of position records. Each record retains its archive, encoded
chain, entry and K4-input identities, literal six-field FEN, and the source quiet
move used by the sampler's eligibility filter. It independently recomputes the
`ngn-k4-input-v1` architecture identity from both king-bucketed/mirrored
perspective feature lists and the material output head. The labeler rejects
malformed state, check positions, illegal moves, captures, promotions, castling
and en-passant source moves before starting the teacher.

The frozen teacher contract is:

- Stockfish source `cb3d4ee9b47d0c5aae855b12379378ea1439675c`;
- BIG network SHA-256 `c288c895ea924429ea9092e3f36b2b3c1f00f2a3a4c759ff7e57e79e3b43e4a7`;
- SMALL network SHA-256 `37f18f62d772f3107e1d6aaca3898c130c3c86f2ab63e6555fbbca20635a899d`;
- Threads 1, Hash 16 MiB, MultiPV 1, SyzygyPath `<empty>`;
- `ucinewgame` plus an `isready` barrier before every root;
- original castling/EP state, halfmove clock reset to zero, no root history;
- exactly `go nodes 5000`, a two-second whole-root deadline, depth at least 4;
- last exact non-bound CP line paired with the returned legal best move;
- the last completed-depth info line may report fewer than 5,000 nodes because
  Stockfish can reach the commanded root budget between depth reports;
- teacher best move must be non-capturing and non-promoting;
- SF18 material normalization inverted to NGN units, then the natural
  `sigmoid(score/400)` target; scores outside ±10,000 are rejected.

Every accepted or rejected input receives an explicit record. Exact teacher info
lines, executable/source/net identities, the input digest, rejection categories
and a per-record SHA-256 chain are retained. A fatal process/protocol failure
stops the shard and leaves a synced `.partial` journal. An identical invocation
verifies that journal and resumes at the next record. Completed output and its
receipt use atomic no-clobber publication.

This stage does not write Bullet data. A separately maintained packer verifies
the labeled JSONL and converts only accepted records; keeping the boundary
separate provides an independent perspective/score check.
