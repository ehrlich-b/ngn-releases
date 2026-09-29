# K4 static calibration dump

`ngnk4staticdump` verifies the finalized pilot manifest, calibration BF,
calibration label shards and exact selected K4/Rodent networks. It evaluates
every accepted calibration FEN through NGN's selected-evaluator API, using
halfmove-reset positions, and writes an immutable 100,000-row CSV plus a
SHA-256 receipt. It does no search and cannot select a checkpoint.

```text
ngnk4staticdump \
  -manifest FINALIZED_PILOT/manifest.json \
  -manifest-sha256 EXPECTED_SHA256 \
  -k4-model SELECTED_K4.nnue \
  -rodent-model RODENT_ANAND.bin \
  -output NEW_PREDICTIONS.csv \
  -receipt NEW_RECEIPT.json

python3 cmd/ngnk4staticdump/analyze.py \
  --csv NEW_PREDICTIONS.csv \
  --receipt NEW_RECEIPT.json \
  --output NEW_ANALYSIS.json
```

The default K4 SHA-256 pins the original 1,024-update pilot. For a newly
selected checkpoint, pass `-k4-sha256 EXPECTED_SHA256`; the dump verifies that
exact model and records its digest in the receipt.

The analysis script requires NumPy and SciPy. It reports score and WDL errors,
sign accuracy, rank correlation, Huber score slope/intercept, and results by
side, output head, king-bucket pair, piece count, source ply and teacher-score
band. Its 400-resample paired intervals use positions as the resampling unit;
neighboring positions can share an encoded chain, so those intervals are
descriptive and can understate uncertainty.
