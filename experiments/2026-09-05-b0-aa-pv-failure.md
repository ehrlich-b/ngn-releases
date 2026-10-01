# B0 strict A/A failure

The first fixed 200-game control stopped with a nonzero runner exit. It produced no accepted games and no strength verdict. The original run remains unchanged; a replacement control requires a new frozen binary, manifest, and run directory.

The existing independent Stockfish replay verified all 43 played plies and all reported PV movements. The PV continued after a terminal threefold repetition.

- Root FEN: `3q1r2/rp4bk/2npbpp1/2pN3p/2PpPB1P/1Q1P4/P3BPP1/1R2R1K1 b - - 6 22`.
- Root occurrence count: 2.
- Reported PV: `d8d7 d5b6 d7d8 b6d5 d8d7`.
- Occurrence counts after successive PV moves: 2, 2, 2, 3, 3.
- Correctly terminated PV: `d8d7 d5b6 d7d8 b6d5`.

The first move was legal. The failure concerns the reported continuation: it crossed the draw boundary. The reporting-only repair and its independent regressions are being prepared separately.

WSL evidence:

- Original run: `/home/ehrli/repos/ngn-next/output/nnue-smp-b0-20260905/aa-200-tc30`.
- Independent oracle: `/home/ehrli/repos/ngn-next/output/nnue-smp-b0-20260905/aa-200-tc30-forensics/root-independent-pv-oracle.json`, SHA-256 `871ad97fb2bc36781fe73cb0240c8c8945659f836257433e3470c6e52cef0e36`.
- On-host derived receipt: `/home/ehrli/repos/ngn-next/output/nnue-smp-b0-20260905/aa-200-tc30-forensics/on-host-failure-receipt.json`.

This report was generated on WSL from artifacts already there. No forensic helper was uploaded and no original run file was modified.
