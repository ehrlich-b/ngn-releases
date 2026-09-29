# K4 teacher-quality diagnostic

`ngnk4teachercheck` verifies the frozen finalized-pilot manifest and label
receipts, chooses 1,000 accepted calibration positions by a domain-separated
hash of their IDs, and searches them with the pinned Stockfish 18 teacher at
20,000 nodes. It writes one new JSON receipt; it never produces training labels.

```text
ngnk4teachercheck \
  -manifest FINALIZED_PILOT/manifest.json \
  -manifest-sha256 EXPECTED_SHA256 \
  -teacher PINNED_STOCKFISH_BINARY \
  -output NEW_AUDIT.json
```

`PASS` means all 1,000 positions had comparable exact CP scores and mean
absolute target difference was at most 0.05. `PASS_BOUND` means protocol
exceptions occurred but the full-sample worst-case mean is still at most 0.05,
using the explicit mate-target convention in the receipt. `HOLD` means the
available evidence does not clear that threshold. The receipt retains every
sample ID, score, protocol exception and the bound calculation.
