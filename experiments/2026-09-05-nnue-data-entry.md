# NNUE pilot data entry — 2026-09-05

The first supervised-data candidate is the official Stockfish T80 binpack, pinned through the dataset API rather than a mutable download URL:

- Dataset: official-stockfish/master-binpacks.
- Dataset revision: 1e095a758c630bc58d0b6dac4da44fcd38ac89c2.
- File: test80-2022-08-aug-16tb7p.v6-dd.min.binpack.
- Declared file size: 10,809,713,086 bytes.
- Declared LFS SHA256: 0d22957b8d4f0f8e6f2913be7b744b2dab5178c3563e0f916312d0d94c28b92b.
- Dataset card license: ODbL.

Metadata and the official format specification are retained in output/nnue-data-entry-20260905. No payload has been downloaded or validated yet. The complete download must match the declared digest before entering a corpus.

The format stores position chains with STM scores, result/ply and rule-50 metadata. It has no explicit original game ID field. A decoded chain ID alone cannot prove original-game-disjoint splits; the converter and data provenance must establish the actual grouping contract before splitting. If original game identity is unavailable, state that limitation and use a documented holdout/dedup strategy without claiming game-disjointness.

This is a pilot candidate, not a data-quality or strength verdict. Sol's exporter/trainer work follows the researched N2 stages; timed matches retain exclusive compute windows.

Sources: [pinned dataset card](https://huggingface.co/datasets/official-stockfish/master-binpacks/blob/1e095a758c630bc58d0b6dac4da44fcd38ac89c2/README.md), [official binpack format](https://github.com/nodchip/Stockfish/blob/17946c5954780ce4490dd4d11585d132cec8190f/docs/binpack.md).
