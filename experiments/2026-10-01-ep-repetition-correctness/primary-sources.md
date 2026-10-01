Primary sources retrieved October1,2026 by coordinator:

- FIDE Laws of Chess, https://handbook.fide.com/chapter/E012023, article9.2.3: repetition depends on the same player to move, same piece placement and same available legal moves; article9.2.3.1 specifically distinguishes an available en-passant capture. Read together with article3.9's king-safety rule. This suggests an illegal pinned EP capture should not distinguish repetition, but the actual engine dataflow and played-sequence reproduction must establish any defect.
- Official Stockfish18 source: https://raw.githubusercontent.com/official-stockfish/Stockfish/sf_18/src/position.cpp. The coordinator fetched this primary source. Verify actual legal-EP normalization using the pinned local SF18 binary and retain its SHA256. Do not assume the existing NGN test's claimed Stockfish equivalence is true. Polyglot's adjacency-based book key is a distinct contract and must be preserved.

This is a bounded rules/implementation audit, not a new strength claim. Neither a passing perft check nor an issue proof establishes Elo.
