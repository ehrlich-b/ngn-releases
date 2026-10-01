# SF18 BIG direct Counter 5.5 runner

This directory is a frozen copy of the accepted `external-opponents-source-v15`
candidate-match runner used by the 2026-09-08 Counter 5.5 anchor. N1i changes
only the candidate side of that protocol:

- the manifest admits the exact SF18 BIG model SHA-256
  `c288c895ea924429ea9092e3f36b2b3c1f00f2a3a4c759ff7e57e79e3b43e4a7`;
- candidate options select `EvalBackend=sf18-big`;
- the candidate preflight accepts and proves the `sf18-big` diagnostic identity;
- the cell is restricted to one worker, 100 games, 50 complete opening pairs,
  16 opening plies, and `30+0.3`.

The pinned Counter 5.5 binary, admission receipt, opening corpus, fastchess,
Stockfish auditor, CPU mask, resource limits, independent chess audit, trace
audit, zero-survivor requirement, and no-adjudication policy are unchanged.
