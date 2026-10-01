# Teacher-score provenance follow-up

This bounded follow-up does not establish the exact producer of the selected test80-2022-08-aug-16tb7p.v6-dd.min.binpack archive. Its existing raw_binpack_score_unit contract remains in force; division by400 is a declared plumbing assumption, not a claim about current Stockfish UCI centipawns.

The published [converter](https://github.com/linrock/lc0-data-converter/blob/a283d1ad0b55/Dockerfile) builds an unpinned Tilps/lc0 rescore_tb branch and an unpinned Stockfish tools branch. Its history and published rescore script identify best-score/best-move and deblunder options, but provide no archive-specific binary or commit receipt.

The current official [Leela rescorer](https://github.com/LeelaChessZero/lc0/blob/3c99ccb1282ba2ed4f061109ab5d12b4cf842c64/src/trainingdata/rescorer.cc#L444) serializes scores as round(660.6*q/(1-0.9751875*q^10)); its comment attributes this to PR1477 adjusted for SF PawnValueEg. It selects best_q or played_q according to flags and emits the rounded result_q. The source SHA256 is736f28e0ca6404c1bfe5d8f7ac1629fa482fe365c561df465a9e3dcd028ddaa8.

That is a concrete conversion mechanism worth checking against historical production, not proof it produced this particular archive. A dated official rescorer-path query through March2023 returned no source entry. Neither a current implementation nor a filename proves the original transform. Do not silently relabel scores or change pilot labels from this evidence. Calibration against frozen validation or a proven producer transform belongs before a strength candidate.

Public API/source snapshots and byte hashes are in output/nnue-score-provenance-20260906/files.json. No dataset payload was downloaded and no training was run.
